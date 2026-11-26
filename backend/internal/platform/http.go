package platform

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
)

func JSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Debug("write response", "error", err)
	}
}

func Error(w http.ResponseWriter, status int, message string) {
	JSON(w, status, map[string]string{"error": message})
}

func Decode(w http.ResponseWriter, r *http.Request, dest any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dest); err != nil {
		var size *http.MaxBytesError
		if errors.As(err, &size) {
			Error(w, http.StatusRequestEntityTooLarge, "JSON body exceeds 64 KiB")
		} else {
			Error(w, http.StatusBadRequest, "Invalid JSON body: "+err.Error())
		}
		return false
	}
	if err := decoder.Decode(new(json.RawMessage)); !errors.Is(err, io.EOF) {
		Error(w, http.StatusBadRequest, "Send one JSON object per request")
		return false
	}
	return true
}

func Page(r *http.Request) (int, int, error) {
	query := r.URL.Query()
	page, err := boundedNumber(query.Get("page"), "page", 1, 10000)
	if err != nil {
		return 0, 0, err
	}
	limit, err := boundedNumber(query.Get("limit"), "limit", 12, 50)
	return page, limit, err
}

func boundedNumber(value, field string, defaultValue, maximum int) (int, error) {
	if value == "" {
		return defaultValue, nil
	}
	number, err := strconv.Atoi(value)
	if err != nil || number < 1 || number > maximum {
		return 0, fmt.Errorf("%s must be an integer from 1 to %d", field, maximum)
	}
	return number, nil
}
