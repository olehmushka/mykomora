package logging

import (
	"log/slog"

	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
)

// Module provides the application logger and routes fx's own lifecycle events
// through it, so startup and shutdown are visible in the same JSON stream.
var Module = fx.Module("logging",
	fx.Provide(New),
	fx.WithLogger(func(log *slog.Logger) fxevent.Logger {
		return &fxevent.SlogLogger{Logger: log.With(slog.String("component", "fx"))}
	}),
)
