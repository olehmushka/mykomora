//go:build integration

package items_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/olehmushka/mykomora/core-api/internal/api"
	"github.com/olehmushka/mykomora/core-api/internal/db"
	"github.com/olehmushka/mykomora/core-api/internal/pgconv"
)

func newChild(t *testing.T, f fixture, size string) uuid.UUID {
	t.Helper()

	person, err := db.New(f.pool).CreatePerson(f.ctx, db.CreatePersonParams{
		FamilyID: pgconv.UUID(f.familyID), Name: "Mia", IsChild: true,
	})
	if err != nil {
		t.Fatalf("CreatePerson: %v", err)
	}

	id := pgconv.ToUUID(person.ID)

	resp, err := f.h.RecordPersonSize(f.ctx, api.RecordPersonSizeRequestObject{
		PersonId: id,
		Body:     &api.RecordPersonSizeRequest{SizeType: api.Clothing, SizeSystem: api.KidsCm, SizeValue: size},
	})
	if err != nil {
		t.Fatalf("RecordPersonSize: %v", err)
	}

	if _, ok := resp.(api.RecordPersonSize201JSONResponse); !ok {
		t.Fatalf("RecordPersonSize returned %T, want a 201", resp)
	}

	return id
}

func sized(t *testing.T, f fixture, name string, system api.SizeSystem, value string) api.Item {
	t.Helper()

	return createItem(t, f, api.CreateItemRequest{
		Name: name, CategoryId: f.categoryID, LocationId: &f.locationID, Quantity: 1,
		SizeSystem: &system, SizeValue: &value,
	})
}

func fitNames(t *testing.T, f fixture, person uuid.UUID, offset int) []string {
	t.Helper()

	wire := api.GetPersonFitsParamsOffset(offset)

	resp, err := f.h.GetPersonFits(f.ctx, api.GetPersonFitsRequestObject{
		PersonId: person, Params: api.GetPersonFitsParams{Offset: &wire},
	})
	if err != nil {
		t.Fatalf("GetPersonFits: %v", err)
	}

	fits, ok := resp.(api.GetPersonFits200JSONResponse)
	if !ok {
		t.Fatalf("GetPersonFits returned %T, want a 200: %+v", resp, resp)
	}

	var names []string

	for _, set := range fits.Sets {
		for _, hit := range set.Items {
			names = append(names, hit.Item.Name)
		}
	}

	return names
}

func TestSizesNormaliseOnWriteAndMatchByRange(t *testing.T) {
	f := newFixture(t)
	mia := newChild(t, f, "110")

	sized(t, f, "Exact 110", api.KidsCm, "110")
	sized(t, f, "Span 104-110", api.KidsCm, "104–110")
	sized(t, f, "Age 5T", api.KidsEu, "5T")
	sized(t, f, "Too small 98", api.KidsCm, "98")
	sized(t, f, "Next 116", api.KidsCm, "116")
	sized(t, f, "Unmapped", api.KidsCm, "banana")

	now := fitNames(t, f, mia, 0)
	for _, want := range []string{"Exact 110", "Span 104-110", "Age 5T"} {
		if !contains(now, want) {
			t.Errorf("fits now = %v, missing %q", now, want)
		}
	}

	for _, unwanted := range []string{"Too small 98", "Next 116", "Unmapped"} {
		if contains(now, unwanted) {
			t.Errorf("fits now = %v, should not include %q", now, unwanted)
		}
	}

	next := fitNames(t, f, mia, 1)
	if !contains(next, "Next 116") || contains(next, "Exact 110") {
		t.Errorf("next size = %v, want the 116 and not the 110", next)
	}
}

func TestFitsLeaveOutOtherPeoplesItemsAndOtherFamilies(t *testing.T) {
	f := newFixture(t)
	mia := newChild(t, f, "110")

	other, err := db.New(f.pool).CreatePerson(f.ctx, db.CreatePersonParams{FamilyID: pgconv.UUID(f.familyID), Name: "Sam"})
	if err != nil {
		t.Fatalf("CreatePerson: %v", err)
	}

	otherID := pgconv.ToUUID(other.ID)
	system, value := api.KidsCm, "110"

	createItem(t, f, api.CreateItemRequest{
		Name: "Sam's coat", CategoryId: f.categoryID, LocationId: &f.locationID, Quantity: 1,
		SizeSystem: &system, SizeValue: &value, ForPersonId: &otherID,
	})

	if got := fitNames(t, f, mia, 0); contains(got, "Sam's coat") {
		t.Errorf("fits = %v, included an item earmarked for someone else", got)
	}

	// A person that is not in this family is simply not found.
	resp, err := f.h.GetPersonFits(f.ctx, api.GetPersonFitsRequestObject{PersonId: uuid.New()})
	if err != nil {
		t.Fatalf("GetPersonFits: %v", err)
	}

	if _, ok := resp.(api.GetPersonFits404JSONResponse); !ok {
		t.Errorf("GetPersonFits for a stranger returned %T, want a 404", resp)
	}
}

func TestBulkStatusAndUndo(t *testing.T) {
	f := newFixture(t)
	a := sized(t, f, "A", api.KidsCm, "110")
	b := sized(t, f, "B", api.KidsCm, "110")

	resp, err := f.h.BulkSetItemStatus(f.ctx, api.BulkSetItemStatusRequestObject{
		Body: &api.BulkSetItemStatusRequest{ItemIds: []uuid.UUID{a.Id, b.Id}, Status: api.ToGiveAway},
	})
	if err != nil {
		t.Fatalf("BulkSetItemStatus: %v", err)
	}

	done, ok := resp.(api.BulkSetItemStatus200JSONResponse)
	if !ok || done.Changed != 2 {
		t.Fatalf("BulkSetItemStatus = %T %+v, want 2 changed", resp, resp)
	}

	if got := getItem(t, f, a.Id).Status; got != api.ToGiveAway {
		t.Fatalf("status after the sweep = %q, want to_give_away", got)
	}

	// B moves on by itself; the undo must leave it alone.
	if _, err := f.h.SetItemStatus(f.ctx, api.SetItemStatusRequestObject{
		ItemId: b.Id, Body: &api.SetItemStatusRequest{Status: api.Stored},
	}); err != nil {
		t.Fatalf("SetItemStatus: %v", err)
	}

	undone, err := f.h.UndoItemBatch(f.ctx, api.UndoItemBatchRequestObject{BatchId: done.BatchId})
	if err != nil {
		t.Fatalf("UndoItemBatch: %v", err)
	}

	result, ok := undone.(api.UndoItemBatch200JSONResponse)
	if !ok || result.Changed != 1 || result.Skipped != 1 {
		t.Fatalf("UndoItemBatch = %T %+v, want 1 changed and 1 skipped", undone, undone)
	}

	if got := getItem(t, f, a.Id).Status; got != api.InUse {
		t.Errorf("A after undo = %q, want in_use", got)
	}
}

func TestSeasonFlipPacksOnlyInUseContentsAndUndoes(t *testing.T) {
	f := newFixture(t)
	box := createItem(t, f, api.CreateItemRequest{
		Name: "Box 7", CategoryId: f.categoryID, LocationId: &f.locationID, Quantity: 1, IsContainer: true,
	})

	inside := func(name string, status api.ItemStatus) api.Item {
		return createItem(t, f, api.CreateItemRequest{
			Name: name, CategoryId: f.categoryID, ParentItemId: &box.Id, Quantity: 1, Status: &status,
		})
	}

	coat := inside("Coat", api.InUse)
	hat := inside("Hat", api.InUse)
	boots := inside("Boots", api.ToGiveAway)

	resp, err := f.h.FlipItemSeason(f.ctx, api.FlipItemSeasonRequestObject{
		ItemId: box.Id, Body: &api.SeasonFlipRequest{Direction: api.Pack},
	})
	if err != nil {
		t.Fatalf("FlipItemSeason: %v", err)
	}

	packed, ok := resp.(api.FlipItemSeason200JSONResponse)
	if !ok || packed.Changed != 2 || packed.Skipped != 1 {
		t.Fatalf("FlipItemSeason = %T %+v, want 2 changed and 1 skipped", resp, resp)
	}

	for _, id := range []uuid.UUID{coat.Id, hat.Id} {
		if got := getItem(t, f, id).Status; got != api.Stored {
			t.Errorf("packed item status = %q, want stored", got)
		}
	}

	if got := getItem(t, f, boots.Id).Status; got != api.ToGiveAway {
		t.Errorf("boots = %q, want untouched", got)
	}

	if _, err := f.h.UndoItemBatch(f.ctx, api.UndoItemBatchRequestObject{BatchId: packed.BatchId}); err != nil {
		t.Fatalf("UndoItemBatch: %v", err)
	}

	if got := getItem(t, f, coat.Id).Status; got != api.InUse {
		t.Errorf("coat after undo = %q, want in_use", got)
	}

	// Not a container: rejected.
	plain := sized(t, f, "Plain", api.KidsCm, "110")

	bad, err := f.h.FlipItemSeason(f.ctx, api.FlipItemSeasonRequestObject{
		ItemId: plain.Id, Body: &api.SeasonFlipRequest{Direction: api.Pack},
	})
	if err != nil {
		t.Fatalf("FlipItemSeason: %v", err)
	}

	if _, ok := bad.(api.FlipItemSeason400JSONResponse); !ok {
		t.Errorf("flip on a non-container returned %T, want a 400", bad)
	}
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}

	return false
}
