// Package config loads runtime configuration from environment variables.
package config

import (
	"errors"
	"os"
	"strings"
)

type Config struct {
	Port        string
	DatabaseURL string

	// JWT verification. At least one of JWTSecret (HS256) or JWKSURL (asymmetric) is required.
	JWTSecret   string
	JWKSURL     string
	JWTIssuer   string
	JWTAudience string
}

func Load() Config {
	return fromGetenv(os.Getenv)
}

func fromGetenv(get func(string) string) Config {
	v := func(k string) string { return strings.TrimSpace(get(k)) }
	c := Config{
		Port:        v("PORT"),
		DatabaseURL: v("DATABASE_URL"),
		JWTSecret:   v("SUPABASE_JWT_SECRET"),
		JWKSURL:     v("SUPABASE_JWKS_URL"),
		JWTIssuer:   v("SUPABASE_JWT_ISSUER"),
		JWTAudience: v("SUPABASE_JWT_AUDIENCE"),
	}
	if c.Port == "" {
		c.Port = "8080"
	}
	if c.JWTAudience == "" {
		c.JWTAudience = "authenticated"
	}
	return c
}

// ValidateAuth reports whether JWT verification is configured well enough to serve protected routes.
func (c Config) ValidateAuth() error {
	if c.JWTSecret == "" && c.JWKSURL == "" {
		return errors.New("set SUPABASE_JWT_SECRET (HS256) or SUPABASE_JWKS_URL")
	}
	if c.JWTIssuer == "" {
		return errors.New("SUPABASE_JWT_ISSUER is not set")
	}
	return nil
}
