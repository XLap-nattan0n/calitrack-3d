// Package apierr writes the API error envelope: {"error":{"code","message","details"}}.
package apierr

import (
	"encoding/json"
	"net/http"
)

type body struct {
	Error detail `json:"error"`
}

type detail struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

func Write(w http.ResponseWriter, status int, code, message string, details map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body{Error: detail{Code: code, Message: message, Details: details}})
}

func Unauthorized(w http.ResponseWriter, message string) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	Write(w, http.StatusUnauthorized, "unauthorized", message, nil)
}

func BadRequest(w http.ResponseWriter, message string, details map[string]any) {
	Write(w, http.StatusBadRequest, "bad_request", message, details)
}

func NotFound(w http.ResponseWriter, message string) {
	Write(w, http.StatusNotFound, "not_found", message, nil)
}

func Internal(w http.ResponseWriter) {
	Write(w, http.StatusInternalServerError, "internal", "internal server error", nil)
}
