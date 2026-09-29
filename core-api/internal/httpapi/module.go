package httpapi

import "go.uber.org/fx"

// Module provides the composed strict server interface.
var Module = fx.Module("httpapi", fx.Provide(NewServer))
