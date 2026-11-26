package contract

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const (
	ProductChanged    = "product.changed.v1"
	InventoryReserve  = "inventory.reserve.v1"
	InventoryReserved = "inventory.reserved.v1"
	InventoryRejected = "inventory.rejected.v1"
	InventoryCommit   = "inventory.commit.v1"
	InventoryRelease  = "inventory.release.v1"
	InventoryReleased = "inventory.released.v1"
	PaymentRequested  = "payment.requested.v1"
	PaymentApproved   = "payment.approved.v1"
	PaymentDeclined   = "payment.declined.v1"
)

type Product struct {
	ID          string `json:"id"`
	Version     int64  `json:"version"`
	Name        string `json:"name"`
	SKU         string `json:"sku"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Price       string `json:"price"`
	Stock       int64  `json:"stock"`
	WeightKG    string `json:"weight_kg"`
}

type Item struct {
	ProductID string `json:"product_id"`
	Quantity  int64  `json:"quantity"`
	UnitPrice string `json:"unit_price"`
	SKU       string `json:"sku,omitempty"`
	Name      string `json:"name,omitempty"`
}

type Event struct {
	Product        *Product `json:"product,omitempty"`
	Deleted        bool     `json:"deleted,omitempty"`
	OrderID        string   `json:"order_id,omitempty"`
	Items          []Item   `json:"items,omitempty"`
	TotalCents     int64    `json:"total_cents,omitempty"`
	PaymentOutcome string   `json:"payment_outcome,omitempty"`
	Reason         string   `json:"reason,omitempty"`
}

var unsignedDecimal = regexp.MustCompile(`^[0-9]{1,9}(\.[0-9]+)?$`)

func ParseMoney(value string) (int64, error)  { return parseDecimal(value, 2) }
func ParseWeight(value string) (int64, error) { return parseDecimal(value, 3) }

func parseDecimal(value string, precision int) (int64, error) {
	if !unsignedDecimal.MatchString(value) {
		return 0, errors.New("use a nonnegative decimal with up to nine integer digits")
	}
	whole, fraction, _ := strings.Cut(value, ".")
	if len(fraction) > precision {
		return 0, fmt.Errorf("use at most %d decimal places", precision)
	}
	return strconv.ParseInt(whole+fraction+strings.Repeat("0", precision-len(fraction)), 10, 64)
}

func Money(cents int64) string  { return fmt.Sprintf("%d.%02d", cents/100, cents%100) }
func Weight(grams int64) string { return fmt.Sprintf("%d.%03d", grams/1000, grams%1000) }
