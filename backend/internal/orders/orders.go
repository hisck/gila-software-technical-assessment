package orders

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"ecommerce/internal/contract"
	"ecommerce/internal/platform"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const schema = `
CREATE TABLE IF NOT EXISTS orders (
	id uuid PRIMARY KEY,
	idempotency_key uuid NOT NULL UNIQUE,
	request_fingerprint text NOT NULL,
	payment_outcome text,
	status text NOT NULL,
	items jsonb NOT NULL CHECK (jsonb_typeof(items) = 'array'),
	total_cents bigint CHECK (total_cents >= 0),
	reason text NOT NULL DEFAULT ''
);
DO $$
BEGIN
	IF NOT EXISTS (
		SELECT 1 FROM pg_constraint
		WHERE conrelid = 'orders'::regclass AND conname = 'orders_payment_phase_check'
	) THEN
		ALTER TABLE orders
			ADD COLUMN order_number bigint GENERATED ALWAYS AS IDENTITY,
			ADD COLUMN created_at timestamptz NOT NULL DEFAULT now(),
			ALTER COLUMN payment_outcome DROP NOT NULL,
			DROP CONSTRAINT IF EXISTS orders_payment_outcome_check,
			DROP CONSTRAINT IF EXISTS orders_status_check,
			ADD CONSTRAINT orders_payment_phase_check CHECK (
				(status = 'awaiting_payment' AND payment_outcome IS NULL) OR
				(status IN ('pending_inventory', 'pending_payment', 'releasing_inventory', 'paid', 'rejected')
					AND payment_outcome IS NOT NULL AND payment_outcome IN ('approve', 'decline'))
			);
	END IF;
END
$$;
CREATE UNIQUE INDEX IF NOT EXISTS orders_order_number ON orders (order_number);`

const (
	maxLines        = 25
	maxQuantity     = 1_000_000
	idempotencyTTL  = 24 * time.Hour
	awaitingPayment = "awaiting_payment"
	pendingStock    = "pending_inventory"
	pendingPayment  = "pending_payment"
	releasingStock  = "releasing_inventory"
	paid            = "paid"
	rejected        = "rejected"
)

var errIdempotencyConflict = errors.New("This idempotency key was already used for a different checkout.")

type service struct {
	db    *pgxpool.Pool
	cache *platform.Cache
}

type checkoutItem struct {
	ProductID string `json:"product_id"`
	Quantity  int64  `json:"quantity"`
	UnitPrice string `json:"unit_price"`
}

type checkout struct {
	Items []checkoutItem `json:"items"`
}

type checkoutResponse struct {
	ID string `json:"id"`
}

type order struct {
	ID        string          `json:"id"`
	Number    string          `json:"number"`
	CreatedAt time.Time       `json:"created_at"`
	Status    string          `json:"status"`
	Reason    string          `json:"reason,omitempty"`
	Total     string          `json:"total,omitempty"`
	Items     []contract.Item `json:"items"`
}

func Run() {
	platform.Run("orders", schema, func(db *pgxpool.Pool, cache *platform.Cache) (http.Handler, platform.Handler) {
		s := service{db: db, cache: cache}
		mux := http.NewServeMux()
		mux.HandleFunc("POST /api/orders", s.submit)
		mux.HandleFunc("GET /api/orders", s.list)
		mux.HandleFunc("GET /api/orders/{id}", s.get)
		mux.HandleFunc("POST /api/orders/{id}/payment", s.pay)
		return mux, handleEvent
	})
}

func (s service) submit(w http.ResponseWriter, r *http.Request) {
	key, ok := parseID(r.Header.Get("Idempotency-Key"))
	if !ok {
		platform.Error(w, http.StatusBadRequest, "Idempotency-Key must be a UUID.")
		return
	}
	var request checkout
	if !platform.Decode(w, r, &request) {
		return
	}
	items, fingerprint, err := request.canonical()
	if err != nil {
		platform.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx := r.Context()
	cacheKey := "orders:idempotency:v1:" + key + ":" + fingerprint
	if value, found := s.cache.Get(ctx, cacheKey, 0); found {
		if id, valid := parseID(string(value)); valid {
			var persisted bool
			err := s.db.QueryRow(ctx, `
				SELECT EXISTS (SELECT 1 FROM orders
				WHERE id = $1 AND idempotency_key = $2 AND request_fingerprint = $3)`,
				id, key, fingerprint).Scan(&persisted)
			if err != nil {
				slog.Error("check cached order", "error", err)
				platform.Error(w, http.StatusServiceUnavailable, "Checkout is temporarily unavailable. Retry with the same idempotency key.")
				return
			}
			if persisted {
				accepted(w, id)
				return
			}
		}
	}

	id, err := s.create(ctx, key, fingerprint, items)
	switch {
	case errors.Is(err, errIdempotencyConflict):
		platform.Error(w, http.StatusConflict, err.Error())
	case err != nil:
		slog.Error("create order", "error", err)
		platform.Error(w, http.StatusServiceUnavailable, "Checkout is temporarily unavailable. Retry with the same idempotency key.")
	default:
		s.cache.Put(ctx, cacheKey, []byte(id), idempotencyTTL)
		accepted(w, id)
	}
}

func accepted(w http.ResponseWriter, id string) {
	w.Header().Set("Location", "/api/orders/"+id)
	platform.JSON(w, http.StatusAccepted, checkoutResponse{ID: id})
}

func (s service) create(ctx context.Context, key, fingerprint string, items []contract.Item) (string, error) {
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	var id string
	err = tx.QueryRow(ctx, `
		INSERT INTO orders (id, idempotency_key, request_fingerprint, status, items)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (idempotency_key) DO NOTHING
		RETURNING id::text`,
		uuid.NewString(), key, fingerprint, awaitingPayment, items,
	).Scan(&id)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		var storedFingerprint string
		err = tx.QueryRow(ctx, `
			SELECT id::text, request_fingerprint FROM orders WHERE idempotency_key = $1`, key,
		).Scan(&id, &storedFingerprint)
		if err != nil {
			return "", err
		}
		if storedFingerprint != fingerprint {
			return "", errIdempotencyConflict
		}
	case err != nil:
		return "", err
	}
	return id, tx.Commit(ctx)
}

func (s service) get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r.PathValue("id"))
	if !ok {
		platform.Error(w, http.StatusBadRequest, "Order ID must be a UUID.")
		return
	}

	result, err := scanOrder(s.db.QueryRow(r.Context(), `SELECT `+orderColumns+` FROM orders WHERE id = $1`, id))
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		platform.Error(w, http.StatusNotFound, "Order not found.")
	case err != nil:
		slog.Error("get order", "error", err)
		platform.Error(w, http.StatusServiceUnavailable, "Order status is temporarily unavailable.")
	default:
		platform.JSON(w, http.StatusOK, result)
	}
}

func (request checkout) canonical() ([]contract.Item, string, error) {
	if len(request.Items) == 0 || len(request.Items) > maxLines {
		return nil, "", fmt.Errorf("Provide between 1 and %d product lines.", maxLines)
	}

	items := make([]contract.Item, 0, len(request.Items))
	seen := make(map[string]bool, len(request.Items))
	for _, item := range request.Items {
		id, ok := parseID(item.ProductID)
		if !ok {
			return nil, "", errors.New("Each product ID must be a UUID.")
		}
		if seen[id] {
			return nil, "", errors.New("Each product can appear once in a checkout.")
		}
		if item.Quantity < 1 || item.Quantity > maxQuantity {
			return nil, "", fmt.Errorf("Each quantity must be between 1 and %d.", maxQuantity)
		}
		cents, err := contract.ParseMoney(item.UnitPrice)
		if err != nil {
			return nil, "", fmt.Errorf("Invalid unit price for product %s: %w.", id, err)
		}
		seen[id] = true
		items = append(items, contract.Item{
			ProductID: id,
			Quantity:  item.Quantity,
			UnitPrice: contract.Money(cents),
		})
	}
	slices.SortFunc(items, func(a, b contract.Item) int {
		return strings.Compare(a.ProductID, b.ProductID)
	})
	body, err := json.Marshal(items)
	if err != nil {
		return nil, "", err
	}
	digest := sha256.Sum256(body)
	return items, hex.EncodeToString(digest[:]), nil
}

func parseID(value string) (string, bool) {
	if len(value) != 36 {
		return "", false
	}
	id, err := uuid.Parse(value)
	if err != nil || id == uuid.Nil {
		return "", false
	}
	return id.String(), true
}
