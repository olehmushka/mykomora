package auth

import (
	"slices"
	"strings"
	"testing"

	"github.com/olehmushka/mykomora/core-api/internal/api"
)

// These two tests are the authentication counterpart of the `family-scoped`
// vet rule in sqlc.yaml: the contract and the code each say which operations
// are public, and neither is allowed to say it alone.
//
// They read the OpenAPI document embedded in the generated code, so they
// compare the served surface against the published one rather than against a
// copy of it.

// operationGoName converts an operationId from the spec into the method name
// oapi-codegen generates for it, which is what the middleware sees.
func operationGoName(operationID string) string {
	if operationID == "" {
		return ""
	}

	return strings.ToUpper(operationID[:1]) + operationID[1:]
}

func TestEveryOperationDeclaresSecurity(t *testing.T) {
	t.Parallel()

	doc, err := api.GetSpec()
	if err != nil {
		t.Fatalf("GetSpec: %v", err)
	}

	for path, item := range doc.Paths.Map() {
		for method, operation := range item.Operations() {
			if operation.Security == nil {
				t.Errorf("%s %s (%s) declares no `security`; there is no document-level "+
					"default, so it would be served without a session",
					method, path, operation.OperationID)
			}
		}
	}
}

func TestPublicOperationsMatchTheSpec(t *testing.T) {
	t.Parallel()

	doc, err := api.GetSpec()
	if err != nil {
		t.Fatalf("GetSpec: %v", err)
	}

	var declaredPublic []string

	for _, item := range doc.Paths.Map() {
		for _, operation := range item.Operations() {
			if operation.Security != nil && len(*operation.Security) == 0 {
				declaredPublic = append(declaredPublic, operationGoName(operation.OperationID))
			}
		}
	}

	slices.Sort(declaredPublic)

	enforced := PublicOperations()

	if !slices.Equal(declaredPublic, enforced) {
		t.Errorf("the contract and the middleware disagree about which operations are public.\n"+
			"  api/openapi.yaml says: %v\n"+
			"  publicOperations says:  %v\n"+
			"Fix whichever is wrong — an operation that is public in only one of them is a hole.",
			declaredPublic, enforced)
	}
}
