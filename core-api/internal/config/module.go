package config

import "go.uber.org/fx"

// Module provides the runtime configuration.
var Module = fx.Module("config", fx.Provide(Load))
