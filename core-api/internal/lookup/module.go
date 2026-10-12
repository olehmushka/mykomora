package lookup

import (
	"net/http"
	"time"

	"go.uber.org/fx"

	"github.com/olehmushka/mykomora/core-api/internal/config"
	"github.com/olehmushka/mykomora/core-api/internal/db"
)

// httpTimeout bounds one catalogue call. The service applies a tighter
// overall deadline; this only stops a stuck connection outliving it.
const httpTimeout = 5 * time.Second

// Module provides the barcode lookup handler.
var Module = fx.Module("lookup",
	fx.Provide(
		NewHandler,
		NewService,
		func(q db.Querier) Store { return q },
		func(cfg config.Config) []Provider {
			client := &http.Client{Timeout: httpTimeout}

			return Providers(client, &cfg)
		},
	),
)
