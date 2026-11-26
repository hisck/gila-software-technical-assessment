package orders

import (
	"context"
	"fmt"

	"ecommerce/internal/contract"
	"ecommerce/internal/platform"

	"github.com/jackc/pgx/v5"
)

func handleEvent(ctx context.Context, tx pgx.Tx, topic string, event contract.Event) error {
	var status, outcome string
	err := tx.QueryRow(ctx, `
		SELECT status, COALESCE(payment_outcome, '') FROM orders WHERE id = $1 FOR UPDATE`, event.OrderID,
	).Scan(&status, &outcome)
	if err != nil {
		return fmt.Errorf("load order for %s: %w", topic, err)
	}

	next, command := orderTransition(status, topic)
	if next == "" {
		return nil
	}

	outgoing := contract.Event{OrderID: event.OrderID}
	switch topic {
	case contract.InventoryReserved:
		_, err = tx.Exec(ctx, `
			UPDATE orders SET status = $2, items = $3, total_cents = $4 WHERE id = $1`,
			event.OrderID, next, event.Items, event.TotalCents,
		)
		outgoing.TotalCents = event.TotalCents
		outgoing.PaymentOutcome = outcome
	case contract.InventoryReleased:
		_, err = tx.Exec(ctx, `UPDATE orders SET status = $2 WHERE id = $1`, event.OrderID, next)
	default:
		var reason string
		if topic == contract.InventoryRejected || topic == contract.PaymentDeclined {
			reason = event.Reason
		}
		_, err = tx.Exec(ctx, `UPDATE orders SET status = $2, reason = $3 WHERE id = $1`, event.OrderID, next, reason)
	}
	if err != nil || command == "" {
		return err
	}
	return platform.Enqueue(ctx, tx, command, outgoing)
}

func orderTransition(status, topic string) (next, command string) {
	switch {
	case topic == contract.InventoryReserved && status == pendingStock:
		return pendingPayment, contract.PaymentRequested
	case topic == contract.InventoryRejected && status == pendingStock:
		return rejected, ""
	case topic == contract.PaymentApproved && status == pendingPayment:
		return paid, contract.InventoryCommit
	case topic == contract.PaymentDeclined && status == pendingPayment:
		return releasingStock, contract.InventoryRelease
	case topic == contract.InventoryReleased && status == releasingStock:
		return rejected, ""
	default:
		return "", ""
	}
}
