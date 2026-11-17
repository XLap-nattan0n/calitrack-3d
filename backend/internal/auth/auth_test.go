package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	iss = "https://proj.supabase.co/auth/v1"
	aud = "authenticated"
	sub = "11111111-1111-1111-1111-111111111111"
)

var secret = []byte("unit-test-secret-not-real-0123456789")

func hsToken(t *testing.T, key []byte, mut func(jwt.MapClaims)) string {
	t.Helper()
	c := jwt.MapClaims{"sub": sub, "iss": iss, "aud": aud, "exp": time.Now().Add(time.Hour).Unix()}
	if mut != nil {
		mut(c)
	}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func call(t *testing.T, v *Verifier, authz string) (*httptest.ResponseRecorder, string) {
	t.Helper()
	var got string
	h := v.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = UserID(r.Context())
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if authz != "" {
		req.Header.Set("Authorization", authz)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec, got
}

func TestMiddlewareHS256(t *testing.T) {
	v, err := NewVerifier(secret, nil, iss, aud)
	if err != nil {
		t.Fatal(err)
	}
	none, _ := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{"sub": sub, "iss": iss, "aud": aud, "exp": time.Now().Add(time.Hour).Unix()}).SignedString(jwt.UnsafeAllowNoneSignatureType)

	cases := []struct {
		name  string
		authz string
		want  int
	}{
		{"valid", "Bearer " + hsToken(t, secret, nil), 200},
		{"missing header", "", 401},
		{"not bearer", "Basic abc", 401},
		{"expired", "Bearer " + hsToken(t, secret, func(c jwt.MapClaims) { c["exp"] = time.Now().Add(-time.Minute).Unix() }), 401},
		{"no exp", "Bearer " + hsToken(t, secret, func(c jwt.MapClaims) { delete(c, "exp") }), 401},
		{"wrong aud", "Bearer " + hsToken(t, secret, func(c jwt.MapClaims) { c["aud"] = "anon" }), 401},
		{"wrong iss", "Bearer " + hsToken(t, secret, func(c jwt.MapClaims) { c["iss"] = "https://evil" }), 401},
		{"bad signature", "Bearer " + hsToken(t, []byte("another-secret-another-secret-123456"), nil), 401},
		{"missing sub", "Bearer " + hsToken(t, secret, func(c jwt.MapClaims) { delete(c, "sub") }), 401},
		{"alg none", "Bearer " + none, 401},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec, got := call(t, v, c.authz)
			if rec.Code != c.want {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, c.want, rec.Body)
			}
			if c.want == 200 && got != sub {
				t.Fatalf("user id = %q, want %q", got, sub)
			}
			if c.want == 401 {
				var b struct{ Error struct{ Code string } }
				if json.Unmarshal(rec.Body.Bytes(), &b) != nil || b.Error.Code != "unauthorized" {
					t.Fatalf("bad error body: %s", rec.Body)
				}
			}
		})
	}
}

func TestMiddlewareJWKSES256(t *testing.T) {
	priv, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	enc := func(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }
	set := map[string]any{"keys": []map[string]string{{
		"kty": "EC", "crv": "P-256", "kid": "k1",
		"x": enc(priv.X.FillBytes(make([]byte, 32))), "y": enc(priv.Y.FillBytes(make([]byte, 32))),
	}}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _ = json.NewEncoder(w).Encode(set) }))
	defer srv.Close()

	v, err := NewVerifier(nil, NewJWKS(srv.URL), iss, aud)
	if err != nil {
		t.Fatal(err)
	}
	mk := func(kid string) string {
		tok := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{"sub": sub, "iss": iss, "aud": aud, "exp": time.Now().Add(time.Hour).Unix()})
		tok.Header["kid"] = kid
		s, _ := tok.SignedString(priv)
		return s
	}
	if rec, got := call(t, v, "Bearer "+mk("k1")); rec.Code != 200 || got != sub {
		t.Fatalf("valid ES256: %d %s", rec.Code, rec.Body)
	}
	if rec, _ := call(t, v, "Bearer "+mk("unknown")); rec.Code != 401 {
		t.Fatalf("unknown kid: %d", rec.Code)
	}
	// HS256 must not be accepted when only JWKS is configured.
	if rec, _ := call(t, v, "Bearer "+hsToken(t, secret, nil)); rec.Code != 401 {
		t.Fatalf("hs256 with jwks only: %d", rec.Code)
	}
}
