// Package httputil provides HTTP response helpers used across the API layer.
package httputil

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
)

// ErrorResponse is the standard error envelope returned by all API endpoints.
// Matches the OpenAPI Error schema.
type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

// JSON writes a JSON response with the given status code and body.
func JSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if body != nil {
		if err := json.NewEncoder(w).Encode(body); err != nil {
			slog.Error("failed to encode JSON response", "error", err)
		}
	}
}

// NoContent writes a 204 No Content response.
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// Error writes a structured error response.
func Error(w http.ResponseWriter, status int, code, message string) {
	JSON(w, status, ErrorResponse{
		Code:    code,
		Message: message,
	})
}

// Errorf writes a 500 Internal Server Error with a generic message.
// Use for unexpected errors; the caller should log the actual error.
func Errorf(w http.ResponseWriter, err error) {
	slog.Error("internal server error", "error", err)
	Error(w, http.StatusInternalServerError, "internal_error", "An internal error occurred")
}

// PaginatedResponse wraps a paginated list response.
type PaginatedResponse struct {
	Data       any        `json:"data"`
	Pagination Pagination `json:"pagination"`
}

// Pagination holds pagination metadata.
type Pagination struct {
	Page       int `json:"page"`
	PageSize   int `json:"pageSize"`
	Total      int `json:"total"`
	TotalPages int `json:"totalPages"`
}

// WritePaginated writes a paginated response with pagination metadata.
func WritePaginated(w http.ResponseWriter, data any, page, pageSize, total int) {
	totalPages := (total + pageSize - 1) / pageSize
	if totalPages < 1 {
		totalPages = 1
	}
	JSON(w, http.StatusOK, PaginatedResponse{
		Data: data,
		Pagination: Pagination{
			Page:       page,
			PageSize:   pageSize,
			Total:      total,
			TotalPages: totalPages,
		},
	})
}

const (
	DefaultPage     = 1
	DefaultPageSize = 20
	MaxPageSize     = 100
)

// Paginate extracts pagination parameters from the request query string.
// Returns normalized page, pageSize, and offset values.
func Paginate(r *http.Request) (page, pageSize, offset int) {
	page = intQuery(r, "page", DefaultPage)
	pageSize = intQuery(r, "pageSize", DefaultPageSize)

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = DefaultPageSize
	}
	if pageSize > MaxPageSize {
		pageSize = MaxPageSize
	}

	offset = (page - 1) * pageSize
	return
}

func intQuery(r *http.Request, key string, fallback int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return fallback
	}
	return n
}
