package taxonomy

import (
	"go.uber.org/fx"

	"github.com/olehmushka/mykomora/core-api/internal/db"
)

// Module provides the taxonomy handler.
var Module = fx.Module("taxonomy",
	fx.Provide(
		NewHandler,
		func(q db.Querier) Store { return q },
	),
)
