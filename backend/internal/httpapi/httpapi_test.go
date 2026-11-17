package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthWithoutDatabaseURLReturns503(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	rec := httptest.NewRecorder()
	NewRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}
