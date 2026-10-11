//go:build integration

// Integration tests against a real Postgres. The one thing worth proving
// here against a real database rather than a mock: rooms have no family_id
// column of their own, so every room query scopes through locations instead
// (see db/queries/rooms.sql's own header) — get that join wrong and a room
// leaks across families with no complaint from the sqlc vet rule.
package places_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/olehmushka/mykomora/core-api/internal/api"
	"github.com/olehmushka/mykomora/core-api/internal/auth"
	"github.com/olehmushka/mykomora/core-api/internal/config"
	"github.com/olehmushka/mykomora/core-api/internal/db"
	"github.com/olehmushka/mykomora/core-api/internal/places"
	"github.com/olehmushka/mykomora/core-api/internal/postgres"
	"github.com/olehmushka/mykomora/core-api/internal/testdb"
)

func newHandler(t *testing.T) (*places.Handler, func(sub string) auth.Principal) {
	t.Helper()

	database := testdb.Start(t)
	queries := db.New(database.Pool)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	service := auth.NewService(queries, postgres.NewTxRunner(database.Pool), config.Config{
		Env:             "test",
		AppSecret:       "an-integration-test-secret-0000000000000",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 30 * 24 * time.Hour,
		InviteTTL:       7 * 24 * time.Hour,
	}, log)

	signIn := func(sub string) auth.Principal {
		session, err := service.SignIn(context.Background(),
			auth.Identity{Subject: sub, Email: sub + "@example.com", DisplayName: sub}, "", auth.RequestMeta{})
		if err != nil {
			t.Fatalf("SignIn(%s): %v", sub, err)
		}

		return session.Principal
	}

	return places.NewHandler(queries, log), signIn
}

// fixture creates one location, owned by `one`, with one room in it, and
// returns both ids.
func fixture(t *testing.T, h *places.Handler, ctx context.Context) (locationID, roomID openapi_types.UUID) {
	t.Helper()

	locationResp, err := h.CreateLocation(ctx, api.CreateLocationRequestObject{
		Body: &api.CreateLocationRequest{Name: "Flat", Kind: api.Home},
	})
	if err != nil {
		t.Fatalf("CreateLocation: %v", err)
	}

	location, ok := locationResp.(api.CreateLocation201JSONResponse)
	if !ok {
		t.Fatalf("CreateLocation returned %T, want a 201", locationResp)
	}

	roomResp, err := h.CreateRoom(ctx, api.CreateRoomRequestObject{
		LocationId: location.Id,
		Body:       &api.CreateRoomRequest{Name: "Bedroom"},
	})
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}

	room, ok := roomResp.(api.CreateRoom201JSONResponse)
	if !ok {
		t.Fatalf("CreateRoom returned %T, want a 201", roomResp)
	}

	return location.Id, room.Id
}

// The milestone's own risk, exercised directly: a room created under one
// family's location is invisible to, and uncreatable for, another family —
// even though rooms carry no family_id to filter on.
func TestRoomsAreScopedThroughTheirLocation(t *testing.T) {
	h, signIn := newHandler(t)

	one := signIn("google-1")
	two := signIn("google-2")

	ctxOne := auth.WithPrincipal(context.Background(), one)
	ctxTwo := auth.WithPrincipal(context.Background(), two)

	locationID, roomID := fixture(t, h, ctxOne)

	// The other family cannot create a room under a location it does not own.
	createResp, err := h.CreateRoom(ctxTwo, api.CreateRoomRequestObject{
		LocationId: locationID,
		Body:       &api.CreateRoomRequest{Name: "Trespassing room"},
	})
	if err != nil {
		t.Fatalf("CreateRoom as the other family: %v", err)
	}

	if _, ok := createResp.(api.CreateRoom404JSONResponse); !ok {
		t.Fatalf("CreateRoom as the other family returned %T, want a 404", createResp)
	}

	// Nor can it read the room once it exists.
	getResp, err := h.GetRoom(ctxTwo, api.GetRoomRequestObject{RoomId: roomID})
	if err != nil {
		t.Fatalf("GetRoom as the other family: %v", err)
	}

	if _, ok := getResp.(api.GetRoom404JSONResponse); !ok {
		t.Fatalf("GetRoom as the other family returned %T, want a 404", getResp)
	}

	// Nor list it.
	listResp, err := h.ListRooms(ctxTwo, api.ListRoomsRequestObject{LocationId: locationID})
	if err != nil {
		t.Fatalf("ListRooms as the other family: %v", err)
	}

	if rooms, ok := listResp.(api.ListRooms200JSONResponse); ok && len(rooms) != 0 {
		t.Errorf("the other family sees %d rooms, want 0", len(rooms))
	}

	// The owning family still can read it.
	if _, err := h.GetRoom(ctxOne, api.GetRoomRequestObject{RoomId: roomID}); err != nil {
		t.Fatalf("GetRoom as the owning family: %v", err)
	}
}

// Archiving does not reach into another family's rows either, through the
// same location join.
func TestArchiveRoomDoesNotReachAnotherFamilysRoom(t *testing.T) {
	h, signIn := newHandler(t)

	one := signIn("google-1")
	two := signIn("google-2")

	ctxOne := auth.WithPrincipal(context.Background(), one)
	ctxTwo := auth.WithPrincipal(context.Background(), two)

	_, roomID := fixture(t, h, ctxOne)

	archived, err := h.ArchiveRoom(ctxTwo, api.ArchiveRoomRequestObject{RoomId: roomID})
	if err != nil {
		t.Fatalf("ArchiveRoom as the other family: %v", err)
	}

	if _, ok := archived.(api.ArchiveRoom404JSONResponse); !ok {
		t.Fatalf("ArchiveRoom as the other family returned %T, want a 404", archived)
	}

	// Confirmed still live: the owning family can still read it.
	if _, err := h.GetRoom(ctxOne, api.GetRoomRequestObject{RoomId: roomID}); err != nil {
		t.Fatalf("GetRoom as the owning family: %v", err)
	}
}
