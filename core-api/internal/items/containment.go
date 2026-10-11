package items

import (
	"github.com/google/uuid"

	"github.com/olehmushka/mykomora/core-api/internal/db"
	"github.com/olehmushka/mykomora/core-api/internal/pgconv"
)

// maxDepth is SPEC 4's "Containment" limit: an item may be nested at most
// this many containers deep.
const maxDepth = 5

// newChildDepth reports the depth a new child of chain's subject would sit
// at. chain is the subject's own ancestor chain (self at depth 0, root at
// the last row), so its length already equals the depth one level below the
// subject — self's ancestor count (len-1) plus one more hop for the child.
// Bounded by maxDepth+1 in practice, nowhere near int32's range.
func newChildDepth(chain []db.GetItemAncestorChainRow) int32 {
	return int32(len(chain)) //nolint:gosec // chain length is bounded by maxDepth
}

// chainContains reports whether id appears anywhere in chain — the subject
// itself or one of its ancestors. Used for cycle detection: walking a
// candidate parent's own chain and finding the item being moved there means
// that candidate is the item itself or one of its descendants.
func chainContains(chain []db.GetItemAncestorChainRow, id uuid.UUID) bool {
	for i := range chain {
		if pgconv.ToUUID(chain[i].ID) == id {
			return true
		}
	}

	return false
}

// maxRelativeDepth reports the deepest existing descendant's depth relative
// to the subject (0 if it has none), used to check that re-parenting a
// container does not push any of its contents past maxDepth.
func maxRelativeDepth(rows []db.GetItemDescendantDepthsRow) int32 {
	var deepest int32

	for _, row := range rows {
		if row.RelativeDepth > deepest {
			deepest = row.RelativeDepth
		}
	}

	return deepest
}

// rootPlace resolves the effective location/room/spot from an ancestor
// chain: the last row (greatest depth) is the root, and a root's own place
// columns are always authoritative (SPEC 4, "Containment").
func rootPlace(chain []db.GetItemAncestorChainRow) (locationID uuid.UUID, roomID *uuid.UUID, spot *string) {
	root := chain[len(chain)-1]

	return pgconv.ToUUID(root.LocationID), pgconv.ToUUIDPtr(root.RoomID), pgconv.ToTextPtr(root.Spot)
}
