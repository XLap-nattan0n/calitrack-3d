package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"
)

// JWKS is a small cached JWKS client supporting RSA and P-256 EC keys.
type JWKS struct {
	url    string
	client *http.Client
	ttl    time.Duration

	mu        sync.Mutex
	keys      map[string]any
	fetchedAt time.Time
}

func NewJWKS(url string) *JWKS {
	return &JWKS{url: url, client: &http.Client{Timeout: 5 * time.Second}, ttl: 10 * time.Minute}
}

// Key returns the key for kid, refreshing the set when stale or when kid is unknown (rate-limited to 1/min).
func (j *JWKS) Key(ctx context.Context, kid string) (any, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	age := time.Since(j.fetchedAt)
	if k, ok := j.keys[kid]; ok && age < j.ttl {
		return k, nil
	}
	if j.keys == nil || age >= j.ttl || age >= time.Minute {
		if err := j.refresh(ctx); err != nil {
			if k, ok := j.keys[kid]; ok {
				return k, nil // serve stale on fetch failure
			}
			return nil, err
		}
	}
	if k, ok := j.keys[kid]; ok {
		return k, nil
	}
	return nil, fmt.Errorf("unknown kid %q", kid)
}

type jwk struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

func (j *JWKS) refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, j.url, nil)
	if err != nil {
		return err
	}
	resp, err := j.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("jwks: status %d", resp.StatusCode)
	}
	var set struct {
		Keys []jwk `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&set); err != nil {
		return err
	}
	keys := map[string]any{}
	for _, k := range set.Keys {
		if pub, err := k.public(); err == nil {
			keys[k.Kid] = pub
		}
	}
	j.keys, j.fetchedAt = keys, time.Now()
	return nil
}

func b64(s string) (*big.Int, error) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	return new(big.Int).SetBytes(b), nil
}

func (k jwk) public() (any, error) {
	switch k.Kty {
	case "RSA":
		n, err1 := b64(k.N)
		e, err2 := b64(k.E)
		if err1 != nil || err2 != nil {
			return nil, errors.New("bad rsa key")
		}
		return &rsa.PublicKey{N: n, E: int(e.Int64())}, nil
	case "EC":
		if k.Crv != "P-256" {
			return nil, errors.New("unsupported curve")
		}
		x, err1 := b64(k.X)
		y, err2 := b64(k.Y)
		if err1 != nil || err2 != nil {
			return nil, errors.New("bad ec key")
		}
		pub := &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}
		if !pub.Curve.IsOnCurve(x, y) {
			return nil, errors.New("point not on curve")
		}
		return pub, nil
	}
	return nil, errors.New("unsupported kty")
}
