package config

import "testing"

func TestDefaultsAndValidate(t *testing.T) {
	c := fromGetenv(func(string) string { return "" })
	if c.Port != "8080" || c.JWTAudience != "authenticated" {
		t.Fatalf("unexpected defaults: %+v", c)
	}
	if c.ValidateAuth() == nil {
		t.Fatal("expected error without secret/JWKS")
	}
	c.JWTSecret = "s"
	if c.ValidateAuth() == nil {
		t.Fatal("expected error without issuer")
	}
	c.JWTIssuer = "https://x.supabase.co/auth/v1"
	if err := c.ValidateAuth(); err != nil {
		t.Fatal(err)
	}
}
