package payments

import (
	"context"
	"fmt"
	"net/http"

	"ecommerce/internal/contract"
	"ecommerce/internal/platform"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const schema = `
CREATE TABLE IF NOT EXISTS payments (
	order_id uuid PRIMARY KEY,
	amount_cents bigint NOT NULL CHECK (amount_cents >= 0),
	status text NOT NULL CHECK (status IN ('approved', 'declined'))
);`

func Run() {
	platform.Run("payments", schema, func(_ *pgxpool.Pool, _ *platform.Cache) (http.Handler, platform.Handler) {
		return http.NewServeMux(), handleEvent
	})
}

func handleEvent(ctx context.Context, tx pgx.Tx, topic string, request contract.Event) error {
	if topic != contract.PaymentRequested {
		return fmt.Errorf("unexpected payment event %q", topic)
	}

	response := contract.Event{OrderID: request.OrderID}
	var status, responseTopic string
	switch request.PaymentOutcome {
	case "approve":
		status, responseTopic = "approved", contract.PaymentApproved
	case "decline":
		status, responseTopic = "declined", contract.PaymentDeclined
		response.Reason = "The simulated payment was declined."
	default:
		return fmt.Errorf("unexpected payment outcome %q", request.PaymentOutcome)
	}

	result, err := tx.Exec(ctx, `
		INSERT INTO payments (order_id, amount_cents, status) VALUES ($1, $2, $3)
		ON CONFLICT (order_id) DO NOTHING`, request.OrderID, request.TotalCents, status)
	if err != nil || result.RowsAffected() == 0 {
		return err
	}
	return platform.Enqueue(ctx, tx, responseTopic, response)
}
