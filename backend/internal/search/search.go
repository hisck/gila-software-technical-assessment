package search

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"ecommerce/internal/contract"
	"ecommerce/internal/platform"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const Schema = `
CREATE TABLE IF NOT EXISTS product_projection (
	id uuid PRIMARY KEY,
	version bigint NOT NULL CHECK (version > 0),
	deleted boolean NOT NULL,
	product jsonb NOT NULL
);
CREATE INDEX IF NOT EXISTS projection_active_name ON product_projection (lower(product->>'name'), id) WHERE NOT deleted;
CREATE INDEX IF NOT EXISTS projection_active_category ON product_projection ((product->>'category')) WHERE NOT deleted;
CREATE TABLE IF NOT EXISTS search_revision (
	singleton boolean PRIMARY KEY CHECK (singleton),
	epoch uuid NOT NULL DEFAULT gen_random_uuid(),
	revision bigint NOT NULL DEFAULT 0
);
INSERT INTO search_revision (singleton) VALUES (true) ON CONFLICT DO NOTHING;
`

const queryLifetime = 90 * time.Second

type service struct {
	db    *pgxpool.Pool
	cache *platform.Cache
}

func Setup(db *pgxpool.Pool, cache *platform.Cache) (http.Handler, platform.Handler) {
	s := service{db: db, cache: cache}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/search/products", s.list)
	return mux, applyProduct
}

func applyProduct(ctx context.Context, tx pgx.Tx, topic string, event contract.Event) error {
	if topic != contract.ProductChanged || event.Product == nil || event.Product.Version < 1 {
		return errors.New("search requires a versioned product snapshot")
	}
	product := *event.Product
	id, err := uuid.Parse(product.ID)
	if err != nil || id == uuid.Nil {
		return errors.New("search snapshot requires a product UUID")
	}
	product.ID = id.String()
	applied, err := tx.Exec(ctx, `
		INSERT INTO product_projection (id, version, deleted, product) VALUES ($1, $2, $3, $4)
		ON CONFLICT (id) DO UPDATE
		SET version=EXCLUDED.version, deleted=EXCLUDED.deleted, product=EXCLUDED.product
		WHERE EXCLUDED.version > product_projection.version`,
		id, product.Version, event.Deleted, product)
	if err != nil || applied.RowsAffected() == 0 {
		return err
	}
	_, err = tx.Exec(ctx, "UPDATE search_revision SET revision=revision+1 WHERE singleton")
	return err
}

type results struct {
	Items      []contract.Product `json:"items"`
	Categories []string           `json:"categories"`
	Total      int64              `json:"total"`
	Page       int                `json:"page"`
	Limit      int                `json:"limit"`
}

type filters struct {
	query, category string
	page, limit     int
}

func (s service) list(w http.ResponseWriter, r *http.Request) {
	page, limit, err := platform.Page(r)
	if err != nil {
		platform.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	values := r.URL.Query()
	f := filters{
		query: strings.TrimSpace(values.Get("q")), category: strings.TrimSpace(values.Get("category")),
		page: page, limit: limit,
	}
	if !validFilter(f.query, 120) || !validFilter(f.category, 100) {
		platform.Error(w, http.StatusBadRequest, "Search terms and categories must contain at most 120 and 100 valid characters, respectively")
		return
	}
	result, err := s.find(r.Context(), f)
	if err != nil {
		slog.Error("search request failed", "error", err)
		platform.Error(w, http.StatusServiceUnavailable, "Search is temporarily unavailable")
		return
	}
	platform.JSON(w, http.StatusOK, result)
}

func validFilter(term string, limit int) bool {
	return utf8.ValidString(term) && utf8.RuneCountInString(term) <= limit && !strings.ContainsRune(term, 0)
}

func (s service) find(ctx context.Context, f filters) (results, error) {
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return results{}, err
	}
	defer tx.Rollback(ctx)

	var epoch string
	var revision int64
	err = tx.QueryRow(ctx, "SELECT epoch::text, revision, lower($1::text) FROM search_revision WHERE singleton", f.query).
		Scan(&epoch, &revision, &f.query)
	if err != nil {
		return results{}, err
	}
	key := cacheKey(epoch, revision, f)
	if cached, ok := s.cache.Get(ctx, key, queryLifetime); ok {
		var result results
		if json.Unmarshal(cached, &result) == nil && result.Items != nil && result.Categories != nil {
			if err := tx.Commit(ctx); err != nil {
				return results{}, err
			}
			return result, nil
		}
	}

	result, err := queryProducts(ctx, tx, f)
	if err != nil {
		return results{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return results{}, err
	}
	if encoded, err := json.Marshal(result); err == nil {
		s.cache.Put(ctx, key, encoded, queryLifetime)
	}
	return result, nil
}

func cacheKey(epoch string, revision int64, f filters) string {
	input := fmt.Sprintf("%s:%d:%d:%d:%d:%s:%s", epoch, revision, f.page, f.limit, len(f.category), f.category, f.query)
	return fmt.Sprintf("search:products:v1:%x", sha256.Sum256([]byte(input)))
}

func queryProducts(ctx context.Context, tx pgx.Tx, f filters) (results, error) {
	result := results{Page: f.page, Limit: f.limit}
	const matching = `FROM product_projection WHERE NOT deleted
		AND ($1 = '' OR strpos(lower(product->>'name'), $1) > 0
		OR strpos(lower(product->>'sku'), $1) > 0 OR strpos(lower(product->>'description'), $1) > 0
		OR strpos(lower(product->>'category'), $1) > 0)
		AND ($2 = '' OR product->>'category' = $2)`

	if err := tx.QueryRow(ctx, "SELECT count(*) "+matching, f.query, f.category).Scan(&result.Total); err != nil {
		return result, err
	}
	rows, err := tx.Query(ctx, "SELECT product "+matching+
		" ORDER BY lower(product->>'name'), id LIMIT $3 OFFSET $4", f.query, f.category, f.limit, (f.page-1)*f.limit)
	if err != nil {
		return result, err
	}
	result.Items, err = pgx.AppendRows([]contract.Product{}, rows, pgx.RowTo[contract.Product])
	if err != nil {
		return result, err
	}
	rows, err = tx.Query(ctx, `SELECT DISTINCT product->>'category' AS category
		FROM product_projection WHERE NOT deleted ORDER BY category`)
	if err != nil {
		return result, err
	}
	result.Categories, err = pgx.AppendRows([]string{}, rows, pgx.RowTo[string])
	return result, err
}
