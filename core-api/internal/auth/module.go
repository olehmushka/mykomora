package auth

import (
	"go.uber.org/fx"

	"github.com/olehmushka/mykomora/core-api/internal/db"
)

// Module provides identity: the Google provider, the token service, the
// session middleware and the auth handlers.
var Module = fx.Module("auth",
	fx.Provide(
		NewGoogleProvider,
		NewService,
		NewMiddleware,
		NewHandler,
		// The generated Querier satisfies this package's narrow Store, but
		// only Store is injected: a handler that cannot name a query cannot
		// accidentally call an unscoped one.
		func(q db.Querier) Store { return q },
	),
)
