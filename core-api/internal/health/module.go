package health

import (
	"go.uber.org/fx"

	"github.com/olehmushka/mykomora/core-api/internal/api"
)

// Module provides the system endpoint handler as the generated strict server
// interface. When real domain handlers arrive they are composed into a single
// implementation of that interface alongside this one.
var Module = fx.Module("health",
	fx.Provide(
		fx.Annotate(New, fx.As(new(api.StrictServerInterface))),
	),
)
