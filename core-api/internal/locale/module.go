package locale

import (
	"go.uber.org/fx"

	"github.com/olehmushka/mykomora/core-api/internal/db"
)

// Module provides the locale resolver.
var Module = fx.Module("locale",
	fx.Provide(
		NewResolver,
		func(q db.Querier) Store { return q },
	),
)
