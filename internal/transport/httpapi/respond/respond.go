// Package respond is the single JSON envelope for every /api endpoint.
//
//	success: {"success":true,"data":...,"meta":null}
//	error:   {"success":false,"error":{"code":"...","message":"...","details":[]}}
//
// It only formats responses; business logic stays in the service layer.
package respond

import (
	"encoding/json"
	"net/http"
)

// Stable error codes. Clients branch on these, never on message text.
const (
	CodeValidation   = "VALIDATION_ERROR"
	CodeForbidden    = "FORBIDDEN"
	CodeNotFound     = "NOT_FOUND"
	CodeConflict     = "CONFLICT"
	CodePrecondition = "PRECONDITION_FAILED"
	CodeInternal     = "INTERNAL_ERROR"
)

type Pagination struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	Total      int `json:"total"`
	TotalPages int `json:"totalPages"`
}

type Detail struct {
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

type successBody struct {
	Success bool `json:"success"`
	Data    any  `json:"data"`
	Meta    any  `json:"meta"`
}

type errorBody struct {
	Success bool      `json:"success"`
	Error   errorInfo `json:"error"`
}

type errorInfo struct {
	Code    string   `json:"code"`
	Message string   `json:"message"`
	Details []Detail `json:"details"`
}

func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// Success writes 200 with data and optional meta (nil → "meta":null).
func Success(w http.ResponseWriter, data any, meta ...any) {
	b := successBody{Success: true, Data: data}
	if len(meta) > 0 {
		b.Meta = meta[0]
	}
	write(w, http.StatusOK, b)
}

// Created writes 201.
func Created(w http.ResponseWriter, data any) {
	write(w, http.StatusCreated, successBody{Success: true, Data: data})
}

// NoContent writes 204 with no body.
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// Paginated writes 200 with pagination metadata. TotalPages is derived from
// Total and Limit; a non-positive Limit yields 0 pages.
func Paginated(w http.ResponseWriter, data any, p Pagination) {
	if p.Limit > 0 {
		p.TotalPages = (p.Total + p.Limit - 1) / p.Limit
	} else {
		p.TotalPages = 0
	}
	write(w, http.StatusOK, successBody{Success: true, Data: data, Meta: p})
}

// Fail writes an error envelope. message must be safe for clients: never an
// internal exception, SQL, path, or stack trace.
func Fail(w http.ResponseWriter, code, message string, status int, details ...Detail) {
	if details == nil {
		details = []Detail{}
	}
	write(w, status, errorBody{Error: errorInfo{Code: code, Message: message, Details: details}})
}

// CodeFor maps an HTTP status to its stable error code.
func CodeFor(status int) string {
	switch status {
	case http.StatusForbidden:
		return CodeForbidden
	case http.StatusNotFound:
		return CodeNotFound
	case http.StatusConflict:
		return CodeConflict
	case http.StatusPreconditionFailed:
		return CodePrecondition
	case http.StatusBadRequest:
		return CodeValidation
	}
	return CodeInternal
}
