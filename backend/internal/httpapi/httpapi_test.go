package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/XLap-nattan0n/calitrack-3d/backend/internal/auth"
	"github.com/XLap-nattan0n/calitrack-3d/backend/internal/db"
	"github.com/XLap-nattan0n/calitrack-3d/backend/internal/profile"
)

const iss = "https://proj.supabase.co/auth/v1"

var secret = []byte("unit-test-secret-not-real-0123456789")

type fakeRepo struct{ rows map[string]profile.Profile }

func (f *fakeRepo) Get(_ context.Context, id string) (profile.Profile, error) {
	p, ok := f.rows[id]
	if !ok {
		return profile.Profile{}, profile.ErrNotFound
	}
	return p, nil
}

func (f *fakeRepo) Update(_ context.Context, id string, u profile.Update) (profile.Profile, error) {
	p, ok := f.rows[id]
	if !ok {
		return profile.Profile{}, profile.ErrNotFound
	}
	if u.DisplayName != nil {
		p.DisplayName = u.DisplayName
	}
	if u.Locale != nil {
		p.Locale = *u.Locale
	}
	f.rows[id] = p
	return p, nil
}

func newTestRouter(t *testing.T) http.Handler {
	t.Helper()
	v, err := auth.NewVerifier(secret, nil, iss, "authenticated")
	if err != nil {
		t.Fatal(err)
	}
	repo := &fakeRepo{rows: map[string]profile.Profile{
		"user-a": {ID: "user-a", Locale: "en", CreatedAt: time.Unix(0, 0).UTC()},
		"user-b": {ID: "user-b", Locale: "th", CreatedAt: time.Unix(0, 0).UTC()},
	}}
	return Build(db.New(""), v, profile.NewHandler(profile.NewService(repo)))
}

func tokenFor(t *testing.T, sub string) string {
	t.Helper()
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": sub, "iss": iss, "aud": "authenticated", "exp": time.Now().Add(time.Hour).Unix(),
	}).SignedString(secret)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func do(h http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHealthWithoutDatabaseURLReturns503(t *testing.T) {
	rec := do(newTestRouter(t), http.MethodGet, "/api/v1/health", "", "") // public: no token needed
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestProtectedRoutesRequireBearer(t *testing.T) {
	h := newTestRouter(t)
	for _, m := range []string{http.MethodGet, http.MethodPatch} {
		if rec := do(h, m, "/api/v1/me", "", "{}"); rec.Code != 401 {
			t.Fatalf("%s /me without token = %d", m, rec.Code)
		}
	}
}

func TestMeIsBoundToTokenSubject(t *testing.T) {
	h := newTestRouter(t)
	rec := do(h, http.MethodGet, "/api/v1/me", tokenFor(t, "user-a"), "")
	var p profile.Profile
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &p) != nil || p.ID != "user-a" {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}

	// A body cannot target another user: unknown fields (e.g. id) are rejected.
	if rec := do(h, http.MethodPatch, "/api/v1/me", tokenFor(t, "user-a"), `{"id":"user-b","display_name":"x"}`); rec.Code != 400 {
		t.Fatalf("patch with id = %d", rec.Code)
	}
	rec = do(h, http.MethodPatch, "/api/v1/me", tokenFor(t, "user-a"), `{"display_name":"  Alice "}`)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"display_name":"Alice"`) {
		t.Fatalf("patch = %d %s", rec.Code, rec.Body)
	}
	// user-b untouched
	rec = do(h, http.MethodGet, "/api/v1/me", tokenFor(t, "user-b"), "")
	if strings.Contains(rec.Body.String(), "Alice") {
		t.Fatalf("user-b saw user-a data: %s", rec.Body)
	}
}

func TestPatchValidationAndErrorFormat(t *testing.T) {
	h := newTestRouter(t)
	tok := tokenFor(t, "user-a")
	for _, body := range []string{`not json`, `{"display_name":"` + strings.Repeat("a", 81) + `"}`, `{"locale":""}`} {
		rec := do(h, http.MethodPatch, "/api/v1/me", tok, body)
		var e struct {
			Error struct{ Code, Message string }
		}
		if rec.Code != 400 || json.Unmarshal(rec.Body.Bytes(), &e) != nil || e.Error.Code != "bad_request" || e.Error.Message == "" {
			t.Fatalf("body %q: %d %s", body, rec.Code, rec.Body)
		}
	}
	if rec := do(h, http.MethodGet, "/api/v1/me", tokenFor(t, "ghost"), ""); rec.Code != 404 {
		t.Fatalf("unknown profile = %d", rec.Code)
	}
}
