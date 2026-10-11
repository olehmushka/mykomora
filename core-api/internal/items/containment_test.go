package items

import (
	"testing"

	"github.com/google/uuid"

	"github.com/olehmushka/mykomora/core-api/internal/db"
	"github.com/olehmushka/mykomora/core-api/internal/pgconv"
)

func ancestorRow(id, locationID uuid.UUID) db.GetItemAncestorChainRow {
	return db.GetItemAncestorChainRow{ID: pgconv.UUID(id), LocationID: pgconv.UUID(locationID)}
}

func TestNewChildDepth(t *testing.T) {
	cases := []struct {
		name  string
		chain []db.GetItemAncestorChainRow
		want  int32
	}{
		{"root parent (chain of 1) gives depth 1", []db.GetItemAncestorChainRow{{}}, 1},
		{"nested parent (chain of 3) gives depth 3", []db.GetItemAncestorChainRow{{}, {}, {}}, 3},
		{"at the depth-5 boundary", make([]db.GetItemAncestorChainRow, 5), 5},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := newChildDepth(tc.chain); got != tc.want {
				t.Errorf("newChildDepth(len=%d) = %d, want %d", len(tc.chain), got, tc.want)
			}
		})
	}
}

func TestChainContains(t *testing.T) {
	a, p, grandparent := uuid.New(), uuid.New(), uuid.New()
	chain := []db.GetItemAncestorChainRow{ancestorRow(p, uuid.New()), ancestorRow(grandparent, uuid.New())}

	if chainContains(chain, a) {
		t.Error("an unrelated id should not be found in the chain")
	}

	if !chainContains(chain, p) {
		t.Error("the chain's own subject should be found (self or direct target)")
	}

	if !chainContains(chain, grandparent) {
		t.Error("an ancestor further up the chain should be found")
	}
}

func TestMaxRelativeDepth(t *testing.T) {
	cases := []struct {
		name string
		rows []db.GetItemDescendantDepthsRow
		want int32
	}{
		{"no descendants", nil, 0},
		{"one descendant", []db.GetItemDescendantDepthsRow{{RelativeDepth: 1}}, 1},
		{"deepest wins", []db.GetItemDescendantDepthsRow{{RelativeDepth: 1}, {RelativeDepth: 3}, {RelativeDepth: 2}}, 3},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := maxRelativeDepth(tc.rows); got != tc.want {
				t.Errorf("maxRelativeDepth(%v) = %d, want %d", tc.rows, got, tc.want)
			}
		})
	}
}

func TestRootPlace(t *testing.T) {
	root := uuid.New()
	rootLocation := uuid.New()
	chain := []db.GetItemAncestorChainRow{
		ancestorRow(uuid.New(), uuid.New()),
		ancestorRow(root, rootLocation),
	}

	locationID, roomID, spot := rootPlace(chain)

	if locationID != rootLocation {
		t.Errorf("rootPlace location = %s, want the last row's (the root's) location %s", locationID, rootLocation)
	}

	if roomID != nil {
		t.Errorf("roomID = %v, want nil (not set on the fake root row)", roomID)
	}

	if spot != nil {
		t.Errorf("spot = %v, want nil (not set on the fake root row)", spot)
	}
}
