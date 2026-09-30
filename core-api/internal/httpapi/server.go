// Package httpapi composes the per-domain handlers into the single interface
// oapi-codegen generates from the contract.
//
// The generated StrictServerInterface has one method per operation, so exactly
// one value has to implement all of them. Rather than let that become a
// god-object, each domain keeps its own handler and this struct embeds them:
// Go promotes the methods, the compiler checks the set is complete, and adding
// an operation to api/openapi.yaml fails to build until some package actually
// serves it.
package httpapi

import (
	"github.com/olehmushka/mykomora/core-api/internal/api"
	"github.com/olehmushka/mykomora/core-api/internal/auth"
	"github.com/olehmushka/mykomora/core-api/internal/family"
	"github.com/olehmushka/mykomora/core-api/internal/health"
	"github.com/olehmushka/mykomora/core-api/internal/people"
)

// Every package calls its handler `Handler`, and four embedded fields cannot
// share a name. These aliases give each one a distinct field name while
// keeping the methods promoted, which a named field would not.
type (
	healthHandler = health.Handler
	authHandler   = auth.Handler
	familyHandler = family.Handler
	peopleHandler = people.Handler
)

// Server is the whole served surface.
type Server struct {
	*healthHandler
	*authHandler
	*familyHandler
	*peopleHandler
}

// The contract and the implementation agree, checked at compile time.
var _ api.StrictServerInterface = (*Server)(nil)

// NewServer assembles the served surface from the domain handlers.
func NewServer(
	h *health.Handler,
	a *auth.Handler,
	f *family.Handler,
	p *people.Handler,
) api.StrictServerInterface {
	return &Server{
		healthHandler: h,
		authHandler:   a,
		familyHandler: f,
		peopleHandler: p,
	}
}
