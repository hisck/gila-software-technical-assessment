package catalog

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"ecommerce/internal/contract"
	"ecommerce/internal/platform"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func handleEvent(ctx context.Context, tx pgx.Tx, topic string, event contract.Event) error {
	orderID, err := uuid.Parse(event.OrderID)
	if err != nil || orderID == uuid.Nil {
		return errors.New("inventory command requires an order UUID")
	}
	switch topic {
	case contract.InventoryReserve:
		return reserve(ctx, tx, orderID, event.Items)
	case contract.InventoryCommit:
		return settle(ctx, tx, orderID, false)
	case contract.InventoryRelease:
		return settle(ctx, tx, orderID, true)
	default:
		return fmt.Errorf("unsupported catalog topic %q", topic)
	}
}

type reservationLine struct {
	id       uuid.UUID
	quantity int64
	quote    int64
}

func reservationLines(items []contract.Item) ([]reservationLine, error) {
	if len(items) < 1 || len(items) > 25 {
		return nil, errors.New("inventory command requires 1 to 25 products")
	}
	lines := make([]reservationLine, len(items))
	seen := make(map[uuid.UUID]bool, len(items))
	for i, item := range items {
		id, err := uuid.Parse(item.ProductID)
		if err != nil || id == uuid.Nil || seen[id] || item.Quantity < 1 || item.Quantity > maxStock {
			return nil, errors.New("invalid or duplicate inventory line")
		}
		quote, err := contract.ParseMoney(item.UnitPrice)
		if err != nil {
			return nil, fmt.Errorf("invalid inventory quote: %w", err)
		}
		seen[id] = true
		lines[i] = reservationLine{id, item.Quantity, quote}
	}
	slices.SortFunc(lines, func(a, b reservationLine) int {
		return strings.Compare(a.id.String(), b.id.String())
	})
	return lines, nil
}

func reserve(ctx context.Context, tx pgx.Tx, orderID uuid.UUID, items []contract.Item) error {
	lines, err := reservationLines(items)
	if err != nil {
		return err
	}
	inserted, err := tx.Exec(ctx, `
		INSERT INTO inventory_reservations (order_id, state) VALUES ($1, 'held')
		ON CONFLICT (order_id) DO NOTHING`, orderID)
	if err != nil || inserted.RowsAffected() == 0 {
		return err
	}

	snapshots := make([]contract.Item, 0, len(lines))
	var total int64
	for _, line := range lines {
		var name, sku string
		var stock, cents int64
		err := tx.QueryRow(ctx, `SELECT name, sku, stock, price_cents FROM products
			WHERE id=$1 AND NOT deleted FOR UPDATE`, line.id).Scan(&name, &sku, &stock, &cents)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return reject(ctx, tx, orderID, "A product is no longer available")
		case err != nil:
			return err
		case stock < line.quantity:
			return reject(ctx, tx, orderID, fmt.Sprintf("Insufficient stock for %s", name))
		case cents != line.quote:
			return reject(ctx, tx, orderID, fmt.Sprintf("The price of %s changed. Refresh the storefront and add it to a new cart", name))
		}
		snapshots = append(snapshots, contract.Item{
			ProductID: line.id.String(), Quantity: line.quantity, UnitPrice: contract.Money(cents), SKU: sku, Name: name,
		})
		total += cents * line.quantity
	}

	for _, line := range lines {
		if _, err := tx.Exec(ctx, `INSERT INTO reservation_items (order_id, product_id, quantity)
			VALUES ($1, $2, $3)`, orderID, line.id, line.quantity); err != nil {
			return err
		}
		product, err := scanProduct(tx.QueryRow(ctx, `
			UPDATE products SET stock=stock-$2, reserved_stock=reserved_stock+$2, version=version+1
			WHERE id=$1 RETURNING `+productColumns, line.id, line.quantity))
		if err != nil {
			return err
		}
		if err := publishProduct(ctx, tx, product, false); err != nil {
			return err
		}
	}
	return platform.Enqueue(ctx, tx, contract.InventoryReserved, contract.Event{
		OrderID: orderID.String(), Items: snapshots, TotalCents: total,
	})
}

func reject(ctx context.Context, tx pgx.Tx, orderID uuid.UUID, reason string) error {
	if _, err := tx.Exec(ctx, "UPDATE inventory_reservations SET state='rejected' WHERE order_id=$1", orderID); err != nil {
		return err
	}
	return platform.Enqueue(ctx, tx, contract.InventoryRejected, contract.Event{OrderID: orderID.String(), Reason: reason})
}

func settle(ctx context.Context, tx pgx.Tx, orderID uuid.UUID, release bool) error {
	var state string
	if err := tx.QueryRow(ctx, `SELECT state FROM inventory_reservations WHERE order_id=$1 FOR UPDATE`,
		orderID).Scan(&state); err != nil {
		return err
	}
	if state != "held" {
		return nil
	}

	rows, err := tx.Query(ctx, `SELECT product_id, quantity FROM reservation_items
		WHERE order_id=$1 ORDER BY product_id`, orderID)
	if err != nil {
		return err
	}
	lines, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (reservationLine, error) {
		var line reservationLine
		err := row.Scan(&line.id, &line.quantity)
		return line, err
	})
	if err != nil {
		return err
	}
	for _, line := range lines {
		if release {
			if err := restore(ctx, tx, line); err != nil {
				return err
			}
		} else if _, err := tx.Exec(ctx, `UPDATE products SET reserved_stock=reserved_stock-$2 WHERE id=$1`,
			line.id, line.quantity); err != nil {
			return err
		}
	}

	state = "committed"
	if release {
		state = "released"
	}
	if _, err := tx.Exec(ctx, "UPDATE inventory_reservations SET state=$2 WHERE order_id=$1", orderID, state); err != nil {
		return err
	}
	if release {
		return platform.Enqueue(ctx, tx, contract.InventoryReleased, contract.Event{OrderID: orderID.String()})
	}
	return nil
}

func restore(ctx context.Context, tx pgx.Tx, line reservationLine) error {
	var deleted bool
	product, err := scanProduct(tx.QueryRow(ctx, `
		UPDATE products SET stock=stock+$2, reserved_stock=reserved_stock-$2, version=version+1
		WHERE id=$1 RETURNING `+productColumns+", deleted", line.id, line.quantity), &deleted)
	if err != nil {
		return err
	}
	return publishProduct(ctx, tx, product, deleted)
}
