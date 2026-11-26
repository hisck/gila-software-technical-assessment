package orders

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const (
	firstProduct  = "5b0dc634-e3a2-4ed6-a303-fac94e716929"
	secondProduct = "52348ab4-42d2-491c-8410-c0ac98addc10"
)

func TestCanonicalCheckoutTreatsEquivalentRetriesAlike(t *testing.T) {
	original := checkout{Items: []checkoutItem{
		{ProductID: firstProduct, Quantity: 2, UnitPrice: "12.50"},
		{ProductID: secondProduct, Quantity: 1, UnitPrice: "0.00"},
	}}
	retry := checkout{Items: []checkoutItem{
		{ProductID: strings.ToUpper(secondProduct), Quantity: 1, UnitPrice: "0"},
		{ProductID: strings.ToUpper(firstProduct), Quantity: 2, UnitPrice: "12.5"},
	}}
	items, first, err := original.canonical()
	if err != nil {
		t.Fatal(err)
	}
	_, second, err := retry.canonical()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].ProductID != secondProduct || items[0].UnitPrice != "0.00" ||
		items[1].ProductID != firstProduct || items[1].UnitPrice != "12.50" || first != second {
		t.Fatalf("equivalent checkout mismatch: items=%+v, digests=%q, %q", items, first, second)
	}

	retry.Items[1].Quantity = 3
	_, differentQuantity, err := retry.canonical()
	if err != nil || differentQuantity == first {
		t.Fatalf("changing a quantity should change the fingerprint: %q, %v", differentQuantity, err)
	}
}

func TestCanonicalCheckoutRejectsInvalidItems(t *testing.T) {
	valid := checkoutItem{ProductID: firstProduct, Quantity: 1, UnitPrice: "0"}
	for _, test := range []struct {
		name  string
		items []checkoutItem
	}{
		{name: "empty cart"},
		{name: "duplicate id", items: []checkoutItem{
			valid, {ProductID: strings.ToUpper(firstProduct), Quantity: 1, UnitPrice: "0"},
		}},
		{name: "invalid UUID", items: []checkoutItem{
			{ProductID: "not-an-id", Quantity: 1, UnitPrice: "0"},
		}},
		{name: "zero quantity", items: []checkoutItem{
			{ProductID: firstProduct, Quantity: 0, UnitPrice: "0"},
		}},
		{name: "excess quantity", items: []checkoutItem{
			{ProductID: firstProduct, Quantity: 1_000_001, UnitPrice: "0"},
		}},
		{name: "fractional cent", items: []checkoutItem{
			{ProductID: firstProduct, Quantity: 1, UnitPrice: "3.001"},
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := checkout{Items: test.items}
			if _, _, err := input.canonical(); err == nil {
				t.Fatal("accepted invalid checkout")
			}
		})
	}
}

func TestPaymentValidatesOutcomeBeforeStartingOrder(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/orders/"+firstProduct+"/payment", strings.NewReader(`{"outcome":"other"}`))
	request.SetPathValue("id", firstProduct)
	response := httptest.NewRecorder()

	service{}.pay(response, request)

	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "Payment outcome must be approve or decline.") {
		t.Fatalf("unexpected payment response: %d %s", response.Code, response.Body.String())
	}
}
