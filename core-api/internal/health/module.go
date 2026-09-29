package health

import (
	"go.uber.org/fx"

	"github.com/olehmushka/mykomora/core-api/internal/db"
)

// Module provides the system endpoint handler.
//
// It no longer provides the generated strict interface directly: from M1 that
// interface is satisfied by internal/httpapi, which composes this handler with
// the domain ones.
var Module = fx.Module("health",
	fx.Provide(
		New,
		func(q db.Querier) Store { return q },
	),
)
