package catalog

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"ecommerce/internal/platform"

	"github.com/jackc/pgx/v5"
)

const maxCSVBytes = 1 << 20
const maxCSVRows = 5000

var csvHeader = []string{"name", "sku", "description", "category", "price", "stock", "weight_kg"}

type importIssue struct {
	Line    int    `json:"line"`
	SKU     string `json:"sku"`
	Message string `json:"message"`
}

type importResult struct {
	Imported int           `json:"imported"`
	Skipped  int           `json:"skipped"`
	Issues   []importIssue `json:"issues"`
}

func (r *importResult) skip(line int, sku string, reason string) {
	r.Skipped++
	r.Issues = append(r.Issues, importIssue{Line: line, SKU: sku, Message: reason})
}

type importRow struct {
	line   int
	values productValues
}

func parseCSV(body []byte) ([]importRow, importResult, error) {
	result := importResult{Issues: []importIssue{}}
	if !utf8.Valid(body) {
		return nil, result, errors.New("CSV must be UTF-8")
	}
	reader := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(body, []byte("\xef\xbb\xbf"))))
	reader.FieldsPerRecord = len(csvHeader)
	header, err := reader.Read()
	if err != nil || !slices.Equal(header, csvHeader) {
		return nil, result, errors.New("CSV must begin with name,sku,description,category,price,stock,weight_kg")
	}

	var rows []importRow
	known := make(map[string]int)
	for count := 0; ; count++ {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, result, fmt.Errorf("Invalid CSV: %w", err)
		}
		if count >= maxCSVRows {
			return nil, result, fmt.Errorf("CSV may contain at most %d data rows", maxCSVRows)
		}
		line, _ := reader.FieldPos(0)
		blank := true
		for i, field := range record {
			record[i] = strings.TrimSpace(field)
			blank = blank && record[i] == ""
		}
		if blank {
			result.Skipped++
			continue
		}
		sku := record[1]
		price := strings.TrimSpace(strings.TrimPrefix(record[4], "$"))
		if strings.EqualFold(record[4], "free") {
			price = "0"
		}
		stock, err := strconv.ParseInt(record[5], 10, 64)
		if err != nil {
			result.skip(line, sku, "Stock must be a whole number between 0 and 1000000")
			continue
		}
		values, err := validate(productInput{
			Name: record[0], SKU: sku, Description: record[2], Category: record[3],
			Price: price, Stock: &stock, WeightKG: record[6],
		})
		if err != nil {
			result.skip(line, sku, err.Error())
			continue
		}
		if first, duplicate := known[values.sku]; duplicate {
			result.skip(line, values.sku, fmt.Sprintf("Duplicate SKU first accepted on line %d", first))
			continue
		}
		known[values.sku] = line
		rows = append(rows, importRow{line: line, values: values})
	}
	return rows, result, nil
}

func (s service) importCSV(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxCSVBytes))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			platform.Error(w, http.StatusRequestEntityTooLarge, "CSV must be at most 1 MiB")
		} else {
			platform.Error(w, http.StatusBadRequest, "Could not read CSV")
		}
		return
	}
	rows, result, err := parseCSV(body)
	if err != nil {
		platform.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx := r.Context()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback(ctx)
	sort.Slice(rows, func(i, j int) bool { return rows[i].values.sku < rows[j].values.sku })
	for _, row := range rows {
		product, err := insertProduct(ctx, tx, row.values)
		if errors.Is(err, pgx.ErrNoRows) {
			result.skip(row.line, row.values.sku, "SKU already exists")
			continue
		}
		if err == nil {
			err = publishProduct(ctx, tx, product, false)
		}
		if err != nil {
			fail(w, err)
			return
		}
		result.Imported++
	}
	if err := tx.Commit(ctx); err != nil {
		fail(w, err)
		return
	}
	sort.Slice(result.Issues, func(i, j int) bool { return result.Issues[i].Line < result.Issues[j].Line })
	platform.JSON(w, http.StatusOK, result)
}
