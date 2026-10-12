//go:build integration

// M7's Done-when sentence against a real Postgres: what needs action today is
// listed truthfully, can be snoozed and un-snoozed, and every quantity change
// leaves an event behind.
package items_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/olehmushka/mykomora/core-api/internal/api"
)

func inDays(n int) *openapi_types.Date {
	return &openapi_types.Date{Time: time.Now().UTC().AddDate(0, 0, n)}
}

func listAlerts(t *testing.T, f fixture) api.AlertList {
	t.Helper()

	resp, err := f.h.ListAlerts(f.ctx, api.ListAlertsRequestObject{})
	if err != nil {
		t.Fatalf("ListAlerts: %v", err)
	}

	list, ok := resp.(api.ListAlerts200JSONResponse)
	if !ok {
		t.Fatalf("ListAlerts returned %T", resp)
	}

	return api.AlertList(list)
}

func adjust(t *testing.T, f fixture, id uuid.UUID, body api.AdjustQuantityRequest) api.AdjustItemQuantityResponseObject {
	t.Helper()

	resp, err := f.h.AdjustItemQuantity(f.ctx, api.AdjustItemQuantityRequestObject{ItemId: id, Body: &body})
	if err != nil {
		t.Fatalf("AdjustItemQuantity: %v", err)
	}

	return resp
}

func f32(v float32) *float32 { return &v }

func alertFor(list api.AlertList, id uuid.UUID, kind api.AlertKind) bool {
	for i := range list.Alerts {
		if list.Alerts[i].ItemId == id && list.Alerts[i].Kind == kind {
			return true
		}
	}

	return false
}

func TestAlertsListWhatNeedsActionAndNothingElse(t *testing.T) {
	f := newFixture(t)
	place := func(r api.CreateItemRequest) api.Item {
		r.CategoryId, r.LocationId = f.categoryID, &f.locationID

		return createItem(t, f, r)
	}

	expired := place(api.CreateItemRequest{Name: "Йогурт", Quantity: 1, ExpiresAt: inDays(-1)})
	soon := place(api.CreateItemRequest{Name: "Молоко", Quantity: 1, ExpiresAt: inDays(10)})
	later := place(api.CreateItemRequest{Name: "Консерви", Quantity: 1, ExpiresAt: inDays(90)})
	low := place(api.CreateItemRequest{Name: "Папір", Quantity: 1, MinQuantity: f32(3)})
	enough := place(api.CreateItemRequest{Name: "Мило", Quantity: 3, MinQuantity: f32(3)})
	warranty := place(api.CreateItemRequest{Name: "Пральна машина", Quantity: 1, WarrantyUntil: inDays(30)})

	opened := time.Now().UTC().AddDate(0, -7, 0)
	months := int32(6)
	pao := place(api.CreateItemRequest{
		Name: "Крем", Quantity: 1, ExpiresAt: inDays(400),
		OpenedAt: &openapi_types.Date{Time: opened}, PaoMonths: &months,
	})

	list := listAlerts(t, f)

	for _, tc := range []struct {
		id   uuid.UUID
		kind api.AlertKind
		want bool
	}{
		{expired.Id, api.AlertKindExpired, true},
		{soon.Id, api.AlertKindExpiring, true},
		{later.Id, api.AlertKindExpiring, false},
		{low.Id, api.AlertKindBelowMinimum, true},
		{enough.Id, api.AlertKindBelowMinimum, false},
		{warranty.Id, api.AlertKindWarrantyEnding, true},
		{pao.Id, api.AlertKindExpired, true}, // opened 7 months ago with a 6-month PAO
	} {
		if got := alertFor(list, tc.id, tc.kind); got != tc.want {
			t.Errorf("alert %s for %s = %v, want %v", tc.kind, tc.id, got, tc.want)
		}
	}

	if list.Counts["expired"] != 2 || list.Counts["expiring"] != 1 || list.Counts["below_minimum"] != 1 || list.Counts["warranty_ending"] != 1 {
		t.Errorf("counts = %v", list.Counts)
	}

	if list.Alerts[0].Kind != api.AlertKindExpired {
		t.Errorf("first alert is %s, want expired", list.Alerts[0].Kind)
	}
}

func TestSnoozeHidesAnAlertAndUndoBringsItBack(t *testing.T) {
	f := newFixture(t)
	item := createItem(t, f, api.CreateItemRequest{
		Name: "Йогурт", CategoryId: f.categoryID, LocationId: &f.locationID, Quantity: 1, ExpiresAt: inDays(-1),
	})

	dismissResp, err := f.h.DismissAlert(f.ctx, api.DismissAlertRequestObject{
		Body: &api.DismissAlertRequest{ItemId: item.Id, Kind: api.AlertKindExpired, SnoozeDays: 7},
	})
	if err != nil {
		t.Fatalf("DismissAlert: %v", err)
	}

	dismissal, ok := dismissResp.(api.DismissAlert200JSONResponse)
	if !ok {
		t.Fatalf("DismissAlert returned %T", dismissResp)
	}

	list := listAlerts(t, f)
	if alertFor(list, item.Id, api.AlertKindExpired) {
		t.Fatal("a snoozed alert is still listed")
	}

	if list.Snoozed == nil || len(*list.Snoozed) != 1 || (*list.Snoozed)[0].Id != dismissal.Id {
		t.Fatalf("snoozed = %+v, want the dismissal", list.Snoozed)
	}

	// Snoozing one kind leaves the same item's other kinds alone.
	if _, err := f.h.UndoAlertDismissal(f.ctx, api.UndoAlertDismissalRequestObject{DismissalId: dismissal.Id}); err != nil {
		t.Fatalf("UndoAlertDismissal: %v", err)
	}

	if !alertFor(listAlerts(t, f), item.Id, api.AlertKindExpired) {
		t.Fatal("the alert did not come back after undo")
	}

	// A second undo, and a dismissal of a made-up item, are both 404s.
	again, _ := f.h.UndoAlertDismissal(f.ctx, api.UndoAlertDismissalRequestObject{DismissalId: dismissal.Id})
	if _, ok := again.(api.UndoAlertDismissal404JSONResponse); !ok {
		t.Fatalf("second undo returned %T, want 404", again)
	}

	ghost, _ := f.h.DismissAlert(f.ctx, api.DismissAlertRequestObject{
		Body: &api.DismissAlertRequest{ItemId: uuid.New(), Kind: api.AlertKindExpired, SnoozeDays: 1},
	})
	if _, ok := ghost.(api.DismissAlert404JSONResponse); !ok {
		t.Fatalf("dismissing an unknown item returned %T, want 404", ghost)
	}
}

func TestQuantityAdjustmentClampsAndWritesOneEventPerChange(t *testing.T) {
	f := newFixture(t)
	item := createItem(t, f, api.CreateItemRequest{
		Name: "Папір", CategoryId: f.categoryID, LocationId: &f.locationID, Quantity: 2, MinQuantity: f32(2),
	})

	if got, ok := adjust(t, f, item.Id, api.AdjustQuantityRequest{Delta: f32(-1)}).(api.AdjustItemQuantity200JSONResponse); !ok || got.Quantity != 1 {
		t.Fatalf("-1 from 2 = %+v, want 1", got)
	}

	if !alertFor(listAlerts(t, f), item.Id, api.AlertKindBelowMinimum) {
		t.Fatal("dropping under the minimum raised no alert")
	}

	if got, ok := adjust(t, f, item.Id, api.AdjustQuantityRequest{Delta: f32(-5)}).(api.AdjustItemQuantity200JSONResponse); !ok || got.Quantity != 0 {
		t.Fatalf("-5 from 1 = %+v, want a clamp at 0", got)
	}

	// Decrementing at zero changes nothing and so records nothing.
	adjust(t, f, item.Id, api.AdjustQuantityRequest{Delta: f32(-1)})

	if got, ok := adjust(t, f, item.Id, api.AdjustQuantityRequest{Quantity: f32(4)}).(api.AdjustItemQuantity200JSONResponse); !ok || got.Quantity != 4 {
		t.Fatalf("set 4 = %+v", got)
	}

	if alertFor(listAlerts(t, f), item.Id, api.AlertKindBelowMinimum) {
		t.Fatal("restocking left the alert in place")
	}

	changes := 0

	for _, e := range listEvents(t, f, item.Id) {
		if e.Type == "quantity_changed" {
			changes++
		}
	}

	if changes != 3 {
		t.Errorf("%d quantity_changed events, want 3 (2→1, 1→0, 0→4)", changes)
	}

	for _, bad := range []api.AdjustQuantityRequest{{}, {Delta: f32(1), Quantity: f32(1)}, {Quantity: f32(-1)}} {
		if _, ok := adjust(t, f, item.Id, bad).(api.AdjustItemQuantity400JSONResponse); !ok {
			t.Errorf("request %+v was accepted, want 400", bad)
		}
	}

	if _, ok := adjust(t, f, uuid.New(), api.AdjustQuantityRequest{Delta: f32(1)}).(api.AdjustItemQuantity404JSONResponse); !ok {
		t.Error("adjusting an unknown item was not a 404")
	}
}

func TestShoppingListIsEverythingBelowMinimum(t *testing.T) {
	f := newFixture(t)
	create := func(name string, qty float32, minQty *float32) {
		createItem(t, f, api.CreateItemRequest{
			Name: name, CategoryId: f.categoryID, LocationId: &f.locationID, Quantity: qty, MinQuantity: minQty,
		})
	}

	create("Папір", 1, f32(4))
	create("Мило", 5, f32(2))
	create("Ложка", 1, nil)

	resp, err := f.h.GetShoppingList(f.ctx, api.GetShoppingListRequestObject{})
	if err != nil {
		t.Fatalf("GetShoppingList: %v", err)
	}

	list, ok := resp.(api.GetShoppingList200JSONResponse)
	if !ok || len(list.Entries) != 1 || list.Entries[0].Name != "Папір" || *list.Entries[0].Shortfall != 3 {
		t.Fatalf("shopping list = %+v", resp)
	}
}
