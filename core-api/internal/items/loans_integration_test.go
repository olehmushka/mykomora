//go:build integration

// M9's Done-when sentence against a real Postgres: who has the drill, and
// when is it coming back, with item status and loan never disagreeing.
package items_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/olehmushka/mykomora/core-api/internal/api"
	"github.com/olehmushka/mykomora/core-api/internal/db"
	"github.com/olehmushka/mykomora/core-api/internal/pgconv"
)

func TestLendAndReturnRoundTrip(t *testing.T) {
	f := newFixture(t)
	drill := createItem(t, f, api.CreateItemRequest{
		Name: "Дриль", CategoryId: f.categoryID, LocationId: &f.locationID, Quantity: 1, Status: ptr(api.Stored),
	})

	person, err := db.New(f.pool).CreatePerson(f.ctx, db.CreatePersonParams{FamilyID: pgconv.UUID(f.familyID), Name: "Сашко"})
	if err != nil {
		t.Fatalf("CreatePerson: %v", err)
	}

	personID := pgconv.ToUUID(person.ID)

	resp, err := f.h.LendItem(f.ctx, api.LendItemRequestObject{
		ItemId: drill.Id, Body: &api.LendItemRequest{BorrowerPersonId: &personID, DueAt: inDays(-2)},
	})
	if err != nil {
		t.Fatalf("LendItem: %v", err)
	}

	loan, ok := resp.(api.LendItem201JSONResponse)
	if !ok {
		t.Fatalf("LendItem returned %T", resp)
	}

	if loan.BorrowerName == nil || *loan.BorrowerName != "Сашко" || !loan.Overdue {
		t.Errorf("loan = %+v, want Сашко and overdue", loan)
	}

	if got := getItem(t, f, drill.Id).Status; got != api.Lent {
		t.Errorf("status after lend = %s, want lent", got)
	}

	// One open loan per item.
	again, _ := f.h.LendItem(f.ctx, api.LendItemRequestObject{
		ItemId: drill.Id, Body: &api.LendItemRequest{BorrowerName: ptr("Сусід")},
	})
	if _, bad := again.(api.LendItem400JSONResponse); !bad {
		t.Errorf("second lend returned %T, want 400", again)
	}

	// The generic status path may neither clear nor set `lent`.
	cleared, _ := f.h.SetItemStatus(f.ctx, api.SetItemStatusRequestObject{ItemId: drill.Id, Body: &api.SetItemStatusRequest{Status: api.InUse}})
	if _, bad := cleared.(api.SetItemStatus400JSONResponse); !bad {
		t.Errorf("clearing lent via status returned %T, want 400", cleared)
	}

	open, _ := f.h.ListOpenLoans(f.ctx, api.ListOpenLoansRequestObject{})
	if l, ok := open.(api.ListOpenLoans200JSONResponse); !ok || len(l) != 1 || l[0].ItemName == nil || *l[0].ItemName != "Дриль" {
		t.Errorf("open loans = %+v", open)
	}

	if !alertFor(listAlerts(t, f), drill.Id, api.AlertKindLoanOverdue) {
		t.Error("overdue loan raised no alert")
	}

	if _, err := f.h.ReturnItem(f.ctx, api.ReturnItemRequestObject{ItemId: drill.Id}); err != nil {
		t.Fatalf("ReturnItem: %v", err)
	}

	if got := getItem(t, f, drill.Id).Status; got != api.Stored {
		t.Errorf("status after return = %s, want the pre-loan stored", got)
	}

	if alertFor(listAlerts(t, f), drill.Id, api.AlertKindLoanOverdue) {
		t.Error("returned loan still raises an alert")
	}

	events, _ := f.h.ListItemEvents(f.ctx, api.ListItemEventsRequestObject{ItemId: drill.Id})
	got := map[string]bool{}

	if l, ok := events.(api.ListItemEvents200JSONResponse); ok {
		for _, e := range l {
			got[e.Type] = true
		}
	}

	if !got["lent"] || !got["returned"] {
		t.Errorf("events = %v, want lent and returned", got)
	}
}

func TestLendingToAnotherFamilysPersonFails(t *testing.T) {
	f := newFixture(t)
	item := createItem(t, f, api.CreateItemRequest{Name: "Драбина", CategoryId: f.categoryID, LocationId: &f.locationID, Quantity: 1})

	stranger := uuid.New()

	resp, _ := f.h.LendItem(f.ctx, api.LendItemRequestObject{ItemId: item.Id, Body: &api.LendItemRequest{BorrowerPersonId: &stranger}})
	if _, bad := resp.(api.LendItem400JSONResponse); !bad {
		t.Errorf("lend to unknown person returned %T, want 400", resp)
	}

	if got := getItem(t, f, item.Id).Status; got == api.Lent {
		t.Error("a failed lend left the item lent")
	}
}

func ptr[T any](v T) *T { return &v }
