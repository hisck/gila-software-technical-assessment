package orders

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"ecommerce/internal/contract"
	"ecommerce/internal/platform"

	"github.com/jackc/pgx/v5"
)

var errPaymentConflict = errors.New("A different payment outcome was already selected for this order.")

func (s service) pay(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r.PathValue("id"))
	if !ok {
		platform.Error(w, http.StatusBadRequest, "Order ID must be a UUID.")
		return
	}

	var request struct {
		Outcome string `json:"outcome"`
	}
	if !platform.Decode(w, r, &request) {
		return
	}
	if request.Outcome != "approve" && request.Outcome != "decline" {
		platform.Error(w, http.StatusBadRequest, "Payment outcome must be approve or decline.")
		return
	}

	err := s.startPayment(r.Context(), id, request.Outcome)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		platform.Error(w, http.StatusNotFound, "Order not found.")
	case errors.Is(err, errPaymentConflict):
		platform.Error(w, http.StatusConflict, err.Error())
	case err != nil:
		slog.Error("start payment", "error", err)
		platform.Error(w, http.StatusServiceUnavailable, "Payment is temporarily unavailable. Retry with the same outcome.")
	default:
		accepted(w, id)
	}
}

func (s service) startPayment(ctx context.Context, id, outcome string) error {
	return pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		var prior *string
		var items []contract.Item
		err := tx.QueryRow(ctx, `
			SELECT payment_outcome, items FROM orders WHERE id = $1 FOR UPDATE`, id,
		).Scan(&prior, &items)
		if err != nil {
			return err
		}
		if prior != nil {
			if *prior == outcome {
				return nil
			}
			return errPaymentConflict
		}

		_, err = tx.Exec(ctx, `
			UPDATE orders SET payment_outcome = $2, status = $3 WHERE id = $1`,
			id, outcome, pendingStock,
		)
		if err != nil {
			return err
		}
		return platform.Enqueue(ctx, tx, contract.InventoryReserve, contract.Event{OrderID: id, Items: items})
	})
}
