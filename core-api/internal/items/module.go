package items

import (
	"go.uber.org/fx"

	"github.com/olehmushka/mykomora/core-api/internal/db"
)

// Module provides the items handler and the service it dispatches to.
var Module = fx.Module("items",
	fx.Provide(
		NewHandler,
		NewService,
		func(q db.Querier) Store { return q },
	),
)
