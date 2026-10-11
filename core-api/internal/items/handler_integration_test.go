//go:build integration

// Integration tests against a real Postgres. Unit tests already cover
// lifecycle, trait validation and the pure containment arithmetic in
// isolation; what only a real database can prove is the thing M3's own
// Done-when sentence is about: a container's move carries its contents
// with it with no edit to them, and every mutation leaves exactly one
// item_events row behind, in the same transaction as the write it
// describes.
package items_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/olehmushka/mykomora/core-api/internal/api"
	"github.com/olehmushka/mykomora/core-api/internal/auth"
	"github.com/olehmushka/mykomora/core-api/internal/config"
	"github.com/olehmushka/mykomora/core-api/internal/db"
	"github.com/olehmushka/mykomora/core-api/internal/items"
	"github.com/olehmushka/mykomora/core-api/internal/pgconv"
	"github.com/olehmushka/mykomora/core-api/internal/places"
	"github.com/olehmushka/mykomora/core-api/internal/postgres"
	"github.com/olehmushka/mykomora/core-api/internal/testdb"
)

// fixture is everything a test needs: a handler, a signed-in principal's
// context, a location with two rooms, and a built-in category id.
type fixture struct {
	h          *items.Handler
	ctx        context.Context
	locationID uuid.UUID
	roomAID    uuid.UUID
	roomBID    uuid.UUID
	categoryID uuid.UUID
}

func newFixture(t *testing.T) fixture {
	t.Helper()

	database := testdb.Start(t)
	queries := db.New(database.Pool)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	tx := postgres.NewTxRunner(database.Pool)

	authService := auth.NewService(queries, tx, config.Config{
		Env:             "test",
		AppSecret:       "an-integration-test-secret-0000000000000",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 30 * 24 * time.Hour,
		InviteTTL:       7 * 24 * time.Hour,
	}, log)

	session, err := authService.SignIn(context.Background(),
		auth.Identity{Subject: "google-1", Email: "one@example.com", DisplayName: "One"}, "", auth.RequestMeta{})
	if err != nil {
		t.Fatalf("SignIn: %v", err)
	}

	ctx := auth.WithPrincipal(context.Background(), session.Principal)

	placesHandler := places.NewHandler(queries, log)

	locationResp, err := placesHandler.CreateLocation(ctx, api.CreateLocationRequestObject{
		Body: &api.CreateLocationRequest{Name: "Flat", Kind: api.Home},
	})
	if err != nil {
		t.Fatalf("CreateLocation: %v", err)
	}

	location, ok := locationResp.(api.CreateLocation201JSONResponse)
	if !ok {
		t.Fatalf("CreateLocation returned %T, want a 201", locationResp)
	}

	roomA := createRoom(t, placesHandler, ctx, location.Id, "Bedroom")
	roomB := createRoom(t, placesHandler, ctx, location.Id, "Garage")

	categories, err := queries.ListCategories(ctx, db.ListCategoriesParams{
		FamilyID: pgconv.UUID(session.Principal.FamilyID), IncludeArchived: false,
	})
	if err != nil {
		t.Fatalf("ListCategories: %v", err)
	}

	var categoryID uuid.UUID

	for _, c := range categories {
		if c.Slug == "clothing_footwear" {
			categoryID = pgconv.ToUUID(c.ID)
		}
	}

	if categoryID == uuid.Nil {
		t.Fatal("seed migration's clothing_footwear category is missing")
	}

	return fixture{
		h:          items.NewHandler(items.NewService(queries, tx, log), log),
		ctx:        ctx,
		locationID: location.Id,
		roomAID:    roomA,
		roomBID:    roomB,
		categoryID: categoryID,
	}
}

func createRoom(t *testing.T, h *places.Handler, ctx context.Context, locationID uuid.UUID, name string) uuid.UUID {
	t.Helper()

	resp, err := h.CreateRoom(ctx, api.CreateRoomRequestObject{
		LocationId: locationID,
		Body:       &api.CreateRoomRequest{Name: name},
	})
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}

	room, ok := resp.(api.CreateRoom201JSONResponse)
	if !ok {
		t.Fatalf("CreateRoom returned %T, want a 201", resp)
	}

	return room.Id
}

func createItem(t *testing.T, f fixture, body api.CreateItemRequest) api.Item {
	t.Helper()

	resp, err := f.h.CreateItem(f.ctx, api.CreateItemRequestObject{Body: &body})
	if err != nil {
		t.Fatalf("CreateItem: %v", err)
	}

	item, ok := resp.(api.CreateItem201JSONResponse)
	if !ok {
		t.Fatalf("CreateItem returned %T, want a 201: %+v", resp, resp)
	}

	return api.Item(item)
}

func getItem(t *testing.T, f fixture, id uuid.UUID) api.Item {
	t.Helper()

	resp, err := f.h.GetItem(f.ctx, api.GetItemRequestObject{ItemId: id})
	if err != nil {
		t.Fatalf("GetItem: %v", err)
	}

	item, ok := resp.(api.GetItem200JSONResponse)
	if !ok {
		t.Fatalf("GetItem returned %T, want a 200: %+v", resp, resp)
	}

	return api.Item(item)
}

func listEvents(t *testing.T, f fixture, id uuid.UUID) []api.ItemEvent {
	t.Helper()

	resp, err := f.h.ListItemEvents(f.ctx, api.ListItemEventsRequestObject{ItemId: id})
	if err != nil {
		t.Fatalf("ListItemEvents: %v", err)
	}

	events, ok := resp.(api.ListItemEvents200JSONResponse)
	if !ok {
		t.Fatalf("ListItemEvents returned %T, want a 200", resp)
	}

	return events
}

// A box created in one room, with a sock created inside it, is the
// milestone's own worked example: moving the box must carry the sock with
// it — one row, one event — with no edit made to the sock itself.
func TestMovingAContainerCarriesItsContentsWithIt(t *testing.T) {
	f := newFixture(t)

	box := createItem(t, f, api.CreateItemRequest{
		Name: "Box of kids' winter clothes", CategoryId: f.categoryID,
		LocationId: &f.locationID, RoomId: &f.roomAID, IsContainer: true, Quantity: 1,
	})

	sock := createItem(t, f, api.CreateItemRequest{
		Name: "Wool sock", CategoryId: f.categoryID, ParentItemId: &box.Id, Quantity: 1,
	})

	if sock.EffectiveRoomId == nil || *sock.EffectiveRoomId != f.roomAID {
		t.Fatalf("sock's effective room = %v, want %s (the box's room)", sock.EffectiveRoomId, f.roomAID)
	}

	moveResp, err := f.h.MoveItem(f.ctx, api.MoveItemRequestObject{
		ItemId: box.Id, Body: &api.MoveItemRequest{LocationId: &f.locationID, RoomId: &f.roomBID},
	})
	if err != nil {
		t.Fatalf("MoveItem: %v", err)
	}

	if _, ok := moveResp.(api.MoveItem200JSONResponse); !ok {
		t.Fatalf("MoveItem returned %T, want a 200: %+v", moveResp, moveResp)
	}

	movedSock := getItem(t, f, sock.Id)
	if movedSock.EffectiveRoomId == nil || *movedSock.EffectiveRoomId != f.roomBID {
		t.Fatalf("sock's effective room after the box moved = %v, want %s — no edit to the sock should have been needed", movedSock.EffectiveRoomId, f.roomBID)
	}

	if movedSock.RoomId != nil && *movedSock.RoomId == f.roomBID {
		t.Error("the sock's own roomId column changed — it should stay cosmetic, only the box's own row should have been written")
	}

	boxEvents := listEvents(t, f, box.Id)
	if len(boxEvents) != 2 || boxEvents[0].Type != "moved" || boxEvents[1].Type != "created" {
		t.Errorf("box events = %+v, want exactly [moved, created]", boxEvents)
	}

	sockEvents := listEvents(t, f, sock.Id)
	if len(sockEvents) != 1 || sockEvents[0].Type != "created" {
		t.Errorf("sock events = %+v, want exactly [created] — moving its container must not write an event on the sock", sockEvents)
	}
}

// The full active set moves freely among itself, then a terminal status
// archives the item and locks it: no further status, move or edit succeeds.
func TestLifecycleWalkThenTerminalLocksTheItem(t *testing.T) {
	f := newFixture(t)

	item := createItem(t, f, api.CreateItemRequest{
		Name: "Drill", CategoryId: f.categoryID, LocationId: &f.locationID, Quantity: 1,
	})

	for _, status := range []api.ItemStatus{api.Stored, api.Lent, api.Stored} {
		resp, err := f.h.SetItemStatus(f.ctx, api.SetItemStatusRequestObject{
			ItemId: item.Id, Body: &api.SetItemStatusRequest{Status: status},
		})
		if err != nil {
			t.Fatalf("SetItemStatus(%s): %v", status, err)
		}

		if _, ok := resp.(api.SetItemStatus200JSONResponse); !ok {
			t.Fatalf("SetItemStatus(%s) returned %T, want a 200", status, resp)
		}
	}

	terminalResp, err := f.h.SetItemStatus(f.ctx, api.SetItemStatusRequestObject{
		ItemId: item.Id, Body: &api.SetItemStatusRequest{Status: api.GivenAway},
	})
	if err != nil {
		t.Fatalf("SetItemStatus(givenAway): %v", err)
	}

	terminal, ok := terminalResp.(api.SetItemStatus200JSONResponse)
	if !ok {
		t.Fatalf("SetItemStatus(givenAway) returned %T, want a 200", terminalResp)
	}

	if terminal.ArchivedAt == nil {
		t.Error("archivedAt is nil after a terminal status, want it set")
	}

	afterTerminalResp, err := f.h.SetItemStatus(f.ctx, api.SetItemStatusRequestObject{
		ItemId: item.Id, Body: &api.SetItemStatusRequest{Status: api.Stored},
	})
	if err != nil {
		t.Fatalf("SetItemStatus after terminal: %v", err)
	}

	if _, ok := afterTerminalResp.(api.SetItemStatus400JSONResponse); !ok {
		t.Errorf("SetItemStatus after terminal returned %T, want a 400 (illegal transition)", afterTerminalResp)
	}

	moveResp, err := f.h.MoveItem(f.ctx, api.MoveItemRequestObject{
		ItemId: item.Id, Body: &api.MoveItemRequest{LocationId: &f.locationID, RoomId: &f.roomBID},
	})
	if err != nil {
		t.Fatalf("MoveItem after terminal: %v", err)
	}

	if _, ok := moveResp.(api.MoveItem404JSONResponse); !ok {
		t.Errorf("MoveItem after terminal returned %T, want a 404 (the SQL guard, not the Go check, catches this one)", moveResp)
	}

	events := listEvents(t, f, item.Id)
	if len(events) != 5 {
		t.Errorf("got %d events, want 5 (created, stored, lent, stored, status_changed-to-givenAway) — none for the two rejected attempts", len(events))
	}
}

// A write that fails after the transaction has begun — here, a status
// change racing an already-archived row — leaves no orphan event: the
// same transaction that would have written it also rolled it back.
func TestFailedMutationWritesNoEvent(t *testing.T) {
	f := newFixture(t)

	item := createItem(t, f, api.CreateItemRequest{
		Name: "Lamp", CategoryId: f.categoryID, LocationId: &f.locationID, Quantity: 1,
	})

	archiveResp, err := f.h.ArchiveItem(f.ctx, api.ArchiveItemRequestObject{ItemId: item.Id})
	if err != nil {
		t.Fatalf("ArchiveItem: %v", err)
	}

	if _, ok := archiveResp.(api.ArchiveItem204Response); !ok {
		t.Fatalf("ArchiveItem returned %T, want a 204", archiveResp)
	}

	before := listEvents(t, f, item.Id)

	// The item's own `status` column is still "in_use" — only archivedAt was
	// touched — so CanTransition's Go-side check lets this through, and the
	// SQL's own `WHERE archived_at IS NULL` guard is what actually rejects
	// it, inside the transaction that would otherwise have written the event.
	statusResp, err := f.h.SetItemStatus(f.ctx, api.SetItemStatusRequestObject{
		ItemId: item.Id, Body: &api.SetItemStatusRequest{Status: api.Stored},
	})
	if err != nil {
		t.Fatalf("SetItemStatus on an archived item: %v", err)
	}

	if _, ok := statusResp.(api.SetItemStatus404JSONResponse); !ok {
		t.Fatalf("SetItemStatus on an archived item returned %T, want a 404", statusResp)
	}

	after := listEvents(t, f, item.Id)
	if len(after) != len(before) {
		t.Errorf("event count changed from %d to %d after a rejected write — the failed transaction left an orphan event", len(before), len(after))
	}
}
