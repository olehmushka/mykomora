package people

import (
	"go.uber.org/fx"

	"github.com/olehmushka/mykomora/core-api/internal/db"
)

// Module provides the people handler.
var Module = fx.Module("people",
	fx.Provide(
		NewHandler,
		func(q db.Querier) Store { return q },
	),
)
