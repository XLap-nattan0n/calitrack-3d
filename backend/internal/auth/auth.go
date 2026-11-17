// Package auth verifies Supabase-issued JWTs and exposes the caller's user id.
package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"github.com/XLap-nattan0n/calitrack-3d/backend/internal/apierr"
)

type ctxKey struct{}

// UserID returns the authenticated user id (JWT sub) set by Middleware.
func UserID(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(ctxKey{}).(string)
	return id, ok && id != ""
}

// WithUserID is mainly for tests of handlers behind the middleware.
func WithUserID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

type Verifier struct {
	parser *jwt.Parser
	keyfn  jwt.Keyfunc
}

// NewVerifier accepts HS256 tokens signed with secret and/or asymmetric tokens (RS256/ES256) via jwks.
// iss and aud are enforced; exp is required.
func NewVerifier(secret []byte, jwks *JWKS, issuer, audience string) (*Verifier, error) {
	if len(secret) == 0 && jwks == nil {
		return nil, errors.New("auth: need an HS256 secret or a JWKS source")
	}
	var methods []string
	if len(secret) > 0 {
		methods = append(methods, "HS256")
	}
	if jwks != nil {
		methods = append(methods, "RS256", "ES256")
	}
	p := jwt.NewParser(
		jwt.WithValidMethods(methods),
		jwt.WithIssuer(issuer),
		jwt.WithAudience(audience),
		jwt.WithExpirationRequired(),
	)
	keyfn := func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); ok {
			if len(secret) == 0 {
				return nil, errors.New("hs256 not enabled")
			}
			return secret, nil
		}
		if jwks == nil {
			return nil, errors.New("jwks not enabled")
		}
		kid, _ := t.Header["kid"].(string)
		return jwks.Key(context.Background(), kid)
	}
	return &Verifier{parser: p, keyfn: keyfn}, nil
}

// Verify returns the user id (sub) of a valid token.
func (v *Verifier) Verify(ctx context.Context, token string) (string, error) {
	t, err := v.parser.Parse(token, v.keyfn)
	if err != nil {
		return "", err
	}
	sub, err := t.Claims.GetSubject()
	if err != nil || sub == "" {
		return "", fmt.Errorf("missing sub claim")
	}
	return sub, nil
}

// Middleware rejects requests without a valid Bearer token with 401 and stores sub in the context.
func (v *Verifier) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		const prefix = "bearer "
		if len(h) <= len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
			apierr.Unauthorized(w, "missing bearer token")
			return
		}
		sub, err := v.Verify(r.Context(), strings.TrimSpace(h[len(prefix):]))
		if err != nil {
			apierr.Unauthorized(w, "invalid or expired token")
			return
		}
		next.ServeHTTP(w, r.WithContext(WithUserID(r.Context(), sub)))
	})
}
