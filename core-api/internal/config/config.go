// Package config loads core-api's runtime configuration from the environment
// and exposes it as an fx module.
//
// The environment is the only source: no config files, no flags. That keeps the
// binary identical across local Compose, CI and the production VPS, where the
// only thing that differs is the injected environment.
package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
)

// Config is the complete runtime configuration of core-api.
type Config struct {
	// Env names the deployment: development, test or production. It gates
	// developer conveniences, never security decisions.
	Env string `env:"ENV" envDefault:"development"`

	// Port is the TCP port the HTTP server listens on.
	Port int `env:"PORT" envDefault:"8081"`

	// DatabaseURL is a libpq-style Postgres connection string.
	DatabaseURL string `env:"DATABASE_URL,required"`

	// LogLevel is one of debug, info, warn or error.
	LogLevel string `env:"LOG_LEVEL" envDefault:"info"`

	// ShutdownTimeout bounds how long in-flight requests may take to drain
	// before the server is closed anyway.
	ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT" envDefault:"10s"`

	// PublicBaseURL is the origin the browser sees — Caddy's, not core-api's.
	// Invite links and post-sign-in redirects are built from it, so getting it
	// wrong produces links that work for nobody but the developer.
	PublicBaseURL string `env:"PUBLIC_BASE_URL" envDefault:"http://localhost:8080"`

	// AppSecret signs the access JWTs and the OAuth flow cookie. Required, with
	// no default: a deployment that forgets it must fail to start rather than
	// quietly run on a guessable key.
	AppSecret string `env:"APP_SECRET,required"`

	// AccessTokenTTL is short on purpose. A stolen access token stays useful
	// only until it expires, and the refresh path is where revocation bites.
	AccessTokenTTL time.Duration `env:"ACCESS_TOKEN_TTL" envDefault:"15m"`

	// RefreshTokenTTL bounds how long a signed-in browser stays signed in
	// without any interaction at all.
	RefreshTokenTTL time.Duration `env:"REFRESH_TOKEN_TTL" envDefault:"720h"`

	// InviteTTL is the lifetime of a family invite link (SPEC: seven days).
	InviteTTL time.Duration `env:"INVITE_TTL" envDefault:"168h"`

	// Google OAuth2. Empty credentials are a supported state: the stack starts,
	// everything else works, and only sign-in reports itself unavailable. That
	// keeps a fresh clone runnable without anyone provisioning a client first.
	GoogleClientID     string `env:"GOOGLE_CLIENT_ID"`
	GoogleClientSecret string `env:"GOOGLE_CLIENT_SECRET"`
	GoogleRedirectURL  string `env:"GOOGLE_REDIRECT_URL" envDefault:"http://localhost:8080/api/v1/auth/google/callback"`

	// GoogleIssuerURL is the OIDC issuer to discover and verify ID tokens
	// against. It is configurable so tests can point it at a local issuer and
	// never touch the network.
	GoogleIssuerURL string `env:"GOOGLE_ISSUER_URL" envDefault:"https://accounts.google.com"`
}

// minAppSecretLen is 32 bytes of key material for HMAC-SHA256. Shorter is not
// a subtle weakness worth tolerating in a config that is set once.
const minAppSecretLen = 32

// ErrWeakAppSecret is returned when APP_SECRET is too short to sign with.
var ErrWeakAppSecret = fmt.Errorf("APP_SECRET must be at least %d characters", minAppSecretLen)

// GoogleConfigured reports whether this deployment can serve Google sign-in.
func (c Config) GoogleConfigured() bool {
	return c.GoogleClientID != "" && c.GoogleClientSecret != ""
}

// CookieSecure reports whether session cookies must carry the Secure flag.
// Only a developer's plain-HTTP stack is exempt; everything else is served
// over TLS by Caddy.
func (c Config) CookieSecure() bool { return !c.IsDevelopment() }

// IsDevelopment reports whether core-api is running in a developer's stack.
func (c Config) IsDevelopment() bool { return c.Env == "development" }

// Load reads and validates the configuration from the process environment.
func Load() (Config, error) {
	cfg, err := env.ParseAs[Config]()
	if err != nil {
		return Config{}, fmt.Errorf("parse environment: %w", err)
	}

	if len(cfg.AppSecret) < minAppSecretLen {
		return Config{}, ErrWeakAppSecret
	}

	return cfg, nil
}
