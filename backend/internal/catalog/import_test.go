package catalog

import (
	"os"
	"reflect"
	"testing"
)

func TestCSVFixture(t *testing.T) {
	body, err := os.ReadFile("../../../Code Challenge E-Commerce.csv")
	if err != nil {
		t.Fatal(err)
	}
	rows, report, err := parseCSV(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 87 || report.Skipped != 10 {
		t.Fatalf("got %d valid rows and %d skips, want 87 and 10", len(rows), report.Skipped)
	}
	lines := make([]int, 0, len(report.Issues))
	for _, issue := range report.Issues {
		lines = append(lines, issue.Line)
	}
	if !reflect.DeepEqual(lines, []int{16, 25, 36, 41, 50, 52, 56, 89}) {
		t.Fatalf("unexpected CSV issue lines: %v", lines)
	}

	prices := make(map[string]int64, len(rows))
	for _, row := range rows {
		prices[row.values.sku] = row.values.price
	}
	for sku, expected := range map[string]int64{"WM-042": 2999, "YM-015": 0, "RS-001": 8999, "BS-021": 5999} {
		got, found := prices[sku]
		if !found || got != expected {
			t.Errorf("price of %s: got %d (present: %v), want %d", sku, got, found, expected)
		}
	}
}

func TestCSVReportsFirstLineAndAllowsFirstValidSKU(t *testing.T) {
	input := "\xef\xbb\xbfname,sku,description,category,price,stock,weight_kg\r\n" +
		",test-1,\"two\nlines\",Category,5,2,0.1\r\n" +
		"Correct,TEST-1,,Category,5,2,0.1\r\n" +
		"Duplicate,test-1,,Category,8,3,0.2\r\n"
	rows, report, err := parseCSV([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].line != 4 || rows[0].values.sku != "TEST-1" {
		t.Fatalf("unexpected CSV rows: %+v", rows)
	}
	if len(report.Issues) != 2 || report.Issues[0].Line != 2 || report.Issues[1].Line != 5 {
		t.Fatalf("unexpected CSV report: %+v", report)
	}
}

func TestMalformedCSVDiscardsPreviouslyParsedRows(t *testing.T) {
	input := "name,sku,description,category,price,stock,weight_kg\n" +
		"Correct,TEST-1,,Category,5,2,0.1\n" +
		"Bad,TEST-2,\"Lacks closing quote\n"
	rows, _, err := parseCSV([]byte(input))
	if err == nil || len(rows) != 0 {
		t.Fatalf("malformed upload retained candidates: rows=%+v, err=%v", rows, err)
	}
}
