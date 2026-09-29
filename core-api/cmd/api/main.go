// Command api is core-api's entrypoint.
//
// It does nothing but compose fx modules: every unit of behaviour lives in
// internal/, and startup ordering, dependency wiring and graceful shutdown are
// fx's responsibility rather than this file's.
package main

import (
	"go.uber.org/fx"

	"github.com/olehmushka/mykomora/core-api/internal/config"
	"github.com/olehmushka/mykomora/core-api/internal/health"
	"github.com/olehmushka/mykomora/core-api/internal/httpserver"
	"github.com/olehmushka/mykomora/core-api/internal/logging"
	"github.com/olehmushka/mykomora/core-api/internal/postgres"
)

func main() {
	fx.New(
		config.Module,
		logging.Module,
		postgres.Module,
		health.Module,
		httpserver.Module,
	).Run()
}
