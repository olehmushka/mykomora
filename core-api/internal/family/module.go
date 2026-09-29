package family

import (
	"go.uber.org/fx"

	"github.com/olehmushka/mykomora/core-api/internal/db"
)

// Module provides the family handler.
var Module = fx.Module("family",
	fx.Provide(
		NewHandler,
		func(q db.Querier) Store { return q },
	),
)
