package places

import (
	"go.uber.org/fx"

	"github.com/olehmushka/mykomora/core-api/internal/db"
)

// Module provides the places handler.
var Module = fx.Module("places",
	fx.Provide(
		NewHandler,
		func(q db.Querier) Store { return q },
	),
)
