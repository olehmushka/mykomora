package taxonomy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"

	"github.com/olehmushka/mykomora/core-api/internal/api"
	"github.com/olehmushka/mykomora/core-api/internal/locale"
)

// taxonomyResponse writes GET /taxonomy's ETag and Vary headers alongside
// its body — oapi-codegen's generated strict-response types carry only what
// the spec declares as a schema'd header, and ETag/304 handling needs the
// raw ResponseWriter, the same reasoning auth/responses.go documents for its
// own custom Visit types.
type taxonomyResponse struct {
	etag        string
	body        api.Taxonomy
	notModified bool
}

// VisitGetTaxonomyResponse satisfies api.GetTaxonomyResponseObject.
func (t taxonomyResponse) VisitGetTaxonomyResponse(w http.ResponseWriter) error {
	w.Header().Set("ETag", t.etag)
	// The same bytes resolve to a different body per Accept-Language, so a
	// shared cache must not serve one locale's response to the other.
	w.Header().Set("Vary", "Accept-Language")

	if t.notModified {
		w.WriteHeader(http.StatusNotModified)

		return nil
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	return json.NewEncoder(w).Encode(t.body)
}

// computeETag hashes the resolved locale together with the serialised body,
// so "en" and "uk" responses never collide on the same tag even if their
// content happened to match.
//
// There is no updated_at column on categories/trait_defs to derive a cheaper
// tag from: built-in rows only ever change via a new migration (itself a
// deploy, which busts any cache on its own), and family-defined edits are
// rare enough that hashing the response is simpler than adding one.
func computeETag(loc locale.Locale, body api.Taxonomy) (string, error) {
	encoded, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(append([]byte(string(loc)+"|"), encoded...))

	return `"` + hex.EncodeToString(sum[:]) + `"`, nil
}
