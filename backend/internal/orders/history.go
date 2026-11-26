package orders

import (
	"log/slog"
	"net/http"

	"ecommerce/internal/contract"
	"ecommerce/internal/platform"

	"github.com/jackc/pgx/v5"
)

const orderColumns = "id::text, order_number::text, created_at, status, reason, total_cents, items"

type historyPage struct {
	Items []order `json:"items"`
	Total int     `json:"total"`
	Page  int     `json:"page"`
	Limit int     `json:"limit"`
}

func (s service) list(w http.ResponseWriter, r *http.Request) {
	page, limit, err := platform.Page(r)
	if err != nil {
		platform.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx := r.Context()
	result := historyPage{Items: []order{}, Page: page, Limit: limit}
	err = pgx.BeginTxFunc(ctx, s.db, pgx.TxOptions{
		IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly,
	}, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, "SELECT count(*) FROM orders").Scan(&result.Total); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT `+orderColumns+` FROM orders
			ORDER BY orders.order_number DESC LIMIT $1 OFFSET $2`, limit, (page-1)*limit,
		)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			item, err := scanOrder(rows)
			if err != nil {
				return err
			}
			result.Items = append(result.Items, item)
		}
		return rows.Err()
	})
	if err != nil {
		slog.Error("list orders", "error", err)
		platform.Error(w, http.StatusServiceUnavailable, "Order history is temporarily unavailable.")
		return
	}
	platform.JSON(w, http.StatusOK, result)
}

func scanOrder(row pgx.Row) (order, error) {
	var result order
	var total *int64
	err := row.Scan(&result.ID, &result.Number, &result.CreatedAt, &result.Status, &result.Reason, &total, &result.Items)
	if err != nil {
		return order{}, err
	}
	result.CreatedAt = result.CreatedAt.UTC()
	if total != nil {
		result.Total = contract.Money(*total)
	}
	return result, nil
}
