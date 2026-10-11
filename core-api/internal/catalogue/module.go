package catalogue

import "go.uber.org/fx"

// Module provides the built-in label catalogue.
var Module = fx.Module("catalogue",
	fx.Provide(func() *Catalogue { return New(BuiltinLabels) }),
)
