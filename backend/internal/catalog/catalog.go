package catalog

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"ecommerce/internal/contract"
	"ecommerce/internal/platform"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const Schema = `
CREATE TABLE IF NOT EXISTS products (
	id uuid PRIMARY KEY,
	version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
	name text NOT NULL,
	sku text NOT NULL,
	description text NOT NULL,
	category text NOT NULL,
	price_cents bigint NOT NULL CHECK (price_cents BETWEEN 0 AND 99999999999),
	stock bigint NOT NULL CHECK (stock >= 0),
	weight_grams bigint NOT NULL CHECK (weight_grams BETWEEN 0 AND 999999999999),
	reserved_stock bigint NOT NULL DEFAULT 0 CHECK (reserved_stock >= 0),
	deleted boolean NOT NULL DEFAULT false,
	CONSTRAINT product_capacity CHECK (stock + reserved_stock <= 1000000)
);
CREATE UNIQUE INDEX IF NOT EXISTS products_active_sku ON products (sku) WHERE NOT deleted;
CREATE INDEX IF NOT EXISTS products_active_name ON products (lower(name), id) WHERE NOT deleted;
CREATE TABLE IF NOT EXISTS inventory_reservations (
	order_id uuid PRIMARY KEY,
	state text NOT NULL CHECK (state IN ('held', 'rejected', 'committed', 'released'))
);
CREATE TABLE IF NOT EXISTS reservation_items (
	order_id uuid NOT NULL REFERENCES inventory_reservations (order_id),
	product_id uuid NOT NULL REFERENCES products (id),
	quantity bigint NOT NULL CHECK (quantity BETWEEN 1 AND 1000000),
	PRIMARY KEY (order_id, product_id)
);
`

const productColumns = "id, version, name, sku, description, category, price_cents, stock, weight_grams"
const maxStock = 1_000_000

type service struct{ db *pgxpool.Pool }

func Setup(db *pgxpool.Pool, _ *platform.Cache) (http.Handler, platform.Handler) {
	s := service{db: db}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/catalog/products", s.list)
	mux.HandleFunc("POST /api/catalog/products", s.create)
	mux.HandleFunc("GET /api/catalog/products/{id}", s.get)
	mux.HandleFunc("PUT /api/catalog/products/{id}", s.update)
	mux.HandleFunc("DELETE /api/catalog/products/{id}", s.delete)
	mux.HandleFunc("POST /api/catalog/import", s.importCSV)
	return mux, handleEvent
}

type productInput struct {
	Name        string `json:"name"`
	SKU         string `json:"sku"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Price       string `json:"price"`
	Stock       *int64 `json:"stock"`
	WeightKG    string `json:"weight_kg"`
}

type productValues struct {
	name, sku, description, category string
	price, stock, weight             int64
}

var skuPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

func validate(input productInput) (productValues, error) {
	var values productValues
	values.name = strings.TrimSpace(input.Name)
	values.sku = strings.TrimSpace(input.SKU)
	values.description = strings.TrimSpace(input.Description)
	values.category = strings.TrimSpace(input.Category)

	for _, field := range []struct {
		name, value string
		min, max    int
	}{
		{"Name", values.name, 1, 160},
		{"Category", values.category, 1, 100},
		{"Description", values.description, 0, 4000},
	} {
		length := utf8.RuneCountInString(field.value)
		if length < field.min || length > field.max || strings.ContainsRune(field.value, 0) {
			return values, fmt.Errorf("%s must contain %d to %d valid characters", field.name, field.min, field.max)
		}
	}

	if !skuPattern.MatchString(values.sku) {
		return values, errors.New("SKU must start with a letter or digit and use at most 64 letters, digits, dots, underscores or hyphens")
	}
	values.sku = strings.ToUpper(values.sku)

	if input.Stock == nil || *input.Stock < 0 || *input.Stock > maxStock {
		return values, fmt.Errorf("Stock must be a whole number between 0 and %d", maxStock)
	}
	values.stock = *input.Stock

	var err error
	values.price, err = contract.ParseMoney(strings.TrimSpace(input.Price))
	if err != nil {
		return values, fmt.Errorf("Price: %w", err)
	}
	values.weight, err = contract.ParseWeight(strings.TrimSpace(input.WeightKG))
	if err != nil {
		return values, fmt.Errorf("Weight: %w", err)
	}
	return values, nil
}

type scanner interface{ Scan(...any) error }

func scanProduct(row scanner, extra ...any) (contract.Product, error) {
	var product contract.Product
	var price, weight int64
	err := row.Scan(append([]any{
		&product.ID, &product.Version, &product.Name, &product.SKU, &product.Description,
		&product.Category, &price, &product.Stock, &weight,
	}, extra...)...)
	product.Price = contract.Money(price)
	product.WeightKG = contract.Weight(weight)
	return product, err
}

func insertProduct(ctx context.Context, tx pgx.Tx, values productValues) (contract.Product, error) {
	return scanProduct(tx.QueryRow(ctx, `
		INSERT INTO products (id, name, sku, description, category, price_cents, stock, weight_grams)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (sku) WHERE NOT deleted DO NOTHING
		RETURNING `+productColumns,
		uuid.New(), values.name, values.sku, values.description, values.category, values.price, values.stock, values.weight))
}

func publishProduct(ctx context.Context, tx pgx.Tx, product contract.Product, deleted bool) error {
	return platform.Enqueue(ctx, tx, contract.ProductChanged, contract.Event{Product: &product, Deleted: deleted})
}

type productPage struct {
	Items []contract.Product `json:"items"`
	Total int64              `json:"total"`
	Page  int                `json:"page"`
	Limit int                `json:"limit"`
}

func (s service) list(w http.ResponseWriter, r *http.Request) {
	page, limit, err := platform.Page(r)
	if err != nil {
		platform.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if utf8.RuneCountInString(query) > 120 || strings.ContainsRune(query, 0) || !utf8.ValidString(query) {
		platform.Error(w, http.StatusBadRequest, "Search terms must contain at most 120 valid characters")
		return
	}

	ctx := r.Context()
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback(ctx)
	result := productPage{Items: []contract.Product{}, Page: page, Limit: limit}
	const matching = `FROM products WHERE NOT deleted AND
		($1 = '' OR strpos(lower(name), lower($1)) > 0 OR strpos(lower(sku), lower($1)) > 0
		OR strpos(lower(description), lower($1)) > 0 OR strpos(lower(category), lower($1)) > 0)`
	if err := tx.QueryRow(ctx, "SELECT count(*) "+matching, query).Scan(&result.Total); err != nil {
		fail(w, err)
		return
	}
	rows, err := tx.Query(ctx, "SELECT "+productColumns+" "+matching+
		" ORDER BY lower(name), id LIMIT $2 OFFSET $3", query, limit, (page-1)*limit)
	if err != nil {
		fail(w, err)
		return
	}
	for rows.Next() {
		product, err := scanProduct(rows)
		if err != nil {
			rows.Close()
			fail(w, err)
			return
		}
		result.Items = append(result.Items, product)
	}
	if err := rows.Err(); err != nil {
		fail(w, err)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		fail(w, err)
		return
	}
	platform.JSON(w, http.StatusOK, result)
}

func (s service) get(w http.ResponseWriter, r *http.Request) {
	id, ok := productID(w, r)
	if !ok {
		return
	}
	product, err := scanProduct(s.db.QueryRow(r.Context(),
		"SELECT "+productColumns+" FROM products WHERE id=$1 AND NOT deleted", id))
	if errors.Is(err, pgx.ErrNoRows) {
		platform.Error(w, http.StatusNotFound, "Product not found")
		return
	}
	if err != nil {
		fail(w, err)
		return
	}
	sendProduct(w, http.StatusOK, product)
}

func (s service) create(w http.ResponseWriter, r *http.Request) {
	values, ok := decodeProduct(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback(ctx)

	product, err := insertProduct(ctx, tx, values)
	if errors.Is(err, pgx.ErrNoRows) {
		platform.Error(w, http.StatusConflict, "SKU already exists")
		return
	}
	if err == nil {
		err = saveProduct(ctx, tx, product, false)
	}
	if err != nil {
		fail(w, err)
		return
	}
	w.Header().Set("Location", "/api/catalog/products/"+product.ID)
	sendProduct(w, http.StatusCreated, product)
}

func (s service) update(w http.ResponseWriter, r *http.Request) {
	id, version, ok := editTarget(w, r)
	if !ok {
		return
	}
	values, ok := decodeProduct(w, r)
	if !ok {
		return
	}
	s.mutate(w, r, id, version, false, `
		UPDATE products SET name=$3, sku=$4, description=$5, category=$6, price_cents=$7, stock=$8,
		weight_grams=$9, version=version+1`,
		values.name, values.sku, values.description, values.category, values.price, values.stock, values.weight)
}

func (s service) delete(w http.ResponseWriter, r *http.Request) {
	id, version, ok := editTarget(w, r)
	if ok {
		s.mutate(w, r, id, version, true, "UPDATE products SET deleted=true, version=version+1")
	}
}

func (s service) mutate(w http.ResponseWriter, r *http.Request, id uuid.UUID, version int64, deleted bool, query string, args ...any) {
	ctx := r.Context()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback(ctx)

	product, err := scanProduct(tx.QueryRow(ctx, query+
		" WHERE id=$1 AND version=$2 AND NOT deleted RETURNING "+productColumns,
		append([]any{id, version}, args...)...))
	if errors.Is(err, pgx.ErrNoRows) {
		var found bool
		err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM products WHERE id=$1 AND NOT deleted)", id).Scan(&found)
		switch {
		case err != nil:
			fail(w, err)
		case found:
			platform.Error(w, http.StatusPreconditionFailed, "Product changed. Reload it before editing")
		default:
			platform.Error(w, http.StatusNotFound, "Product not found")
		}
		return
	}
	if err != nil {
		fail(w, err)
		return
	}
	if err := saveProduct(ctx, tx, product, deleted); err != nil {
		fail(w, err)
		return
	}
	if deleted {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	sendProduct(w, http.StatusOK, product)
}

func saveProduct(ctx context.Context, tx pgx.Tx, product contract.Product, deleted bool) error {
	if err := publishProduct(ctx, tx, product, deleted); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func sendProduct(w http.ResponseWriter, status int, product contract.Product) {
	w.Header().Set("ETag", strconv.Quote(strconv.FormatInt(product.Version, 10)))
	platform.JSON(w, status, product)
}

func productID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil || id == uuid.Nil {
		platform.Error(w, http.StatusBadRequest, "Product ID must be a UUID")
		return uuid.Nil, false
	}
	return id, true
}

func editTarget(w http.ResponseWriter, r *http.Request) (uuid.UUID, int64, bool) {
	id, ok := productID(w, r)
	if !ok {
		return id, 0, false
	}
	raw := strings.TrimSpace(r.Header.Get("If-Match"))
	if raw == "" {
		platform.Error(w, http.StatusPreconditionRequired, "Send the product version in If-Match")
		return id, 0, false
	}
	if unquoted, err := strconv.Unquote(raw); err == nil {
		raw = unquoted
	}
	version, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || version < 1 {
		platform.Error(w, http.StatusBadRequest, "If-Match must contain a positive product version")
		return id, 0, false
	}
	return id, version, true
}

func decodeProduct(w http.ResponseWriter, r *http.Request) (productValues, bool) {
	var input productInput
	if !platform.Decode(w, r, &input) {
		return productValues{}, false
	}
	values, err := validate(input)
	if err != nil {
		platform.Error(w, http.StatusBadRequest, err.Error())
	}
	return values, err == nil
}

func fail(w http.ResponseWriter, err error) {
	var dbError *pgconn.PgError
	if errors.As(err, &dbError) {
		switch dbError.ConstraintName {
		case "products_active_sku":
			platform.Error(w, http.StatusConflict, "SKU already exists")
			return
		case "product_capacity":
			platform.Error(w, http.StatusConflict, "Available and reserved stock cannot exceed 1000000 units")
			return
		}
	}
	slog.Error("catalog request failed", "error", err)
	platform.Error(w, http.StatusServiceUnavailable, "Catalog is temporarily unavailable")
}
