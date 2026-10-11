package storage

import (
	"go.uber.org/fx"

	"github.com/olehmushka/mykomora/core-api/internal/db"
)

// Module provides the S3-compatible client and the orphaned-object
// cleanup sweeper that runs against it.
var Module = fx.Module("storage",
	fx.Provide(
		NewClient,
		NewSweeper,
		func(q db.Querier) Keys { return q },
	),
)
