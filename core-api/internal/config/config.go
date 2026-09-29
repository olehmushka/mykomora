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
}

// IsDevelopment reports whether core-api is running in a developer's stack.
func (c Config) IsDevelopment() bool { return c.Env == "development" }

// Load reads and validates the configuration from the process environment.
func Load() (Config, error) {
	cfg, err := env.ParseAs[Config]()
	if err != nil {
		return Config{}, fmt.Errorf("parse environment: %w", err)
	}

	return cfg, nil
}
