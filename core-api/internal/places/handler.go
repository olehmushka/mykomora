// Package places serves locations and rooms: where things physically are
// (SPEC 3, "Concepts" and SPEC 4, "Places").
//
// Locations are scoped directly on family_id, the same shape as people.
// Rooms have no family_id column of their own — their tenant key is one hop
// away, through the location they belong to — so every room query joins or
// EXISTS-checks locations instead (see db/queries/rooms.sql's own header).
package places

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/olehmushka/mykomora/core-api/internal/api"
	"github.com/olehmushka/mykomora/core-api/internal/apimap"
	"github.com/olehmushka/mykomora/core-api/internal/auth"
	"github.com/olehmushka/mykomora/core-api/internal/db"
	"github.com/olehmushka/mykomora/core-api/internal/pgconv"
)

const maxNameLen = 120

// Store is the slice of the generated queries this package needs.
type Store interface {
	CreateLocation(ctx context.Context, arg db.CreateLocationParams) (db.Location, error)
	GetLocation(ctx context.Context, arg db.GetLocationParams) (db.Location, error)
	ListLocations(ctx context.Context, arg db.ListLocationsParams) ([]db.Location, error)
	UpdateLocation(ctx context.Context, arg db.UpdateLocationParams) (db.Location, error)
	ArchiveLocation(ctx context.Context, arg db.ArchiveLocationParams) (int64, error)

	CreateRoom(ctx context.Context, arg db.CreateRoomParams) (db.Room, error)
	GetRoom(ctx context.Context, arg db.GetRoomParams) (db.Room, error)
	ListRoomsByLocation(ctx context.Context, arg db.ListRoomsByLocationParams) ([]db.Room, error)
	UpdateRoom(ctx context.Context, arg db.UpdateRoomParams) (db.Room, error)
	ArchiveRoom(ctx context.Context, arg db.ArchiveRoomParams) (int64, error)
}

// Handler serves the places operations of the generated contract.
type Handler struct {
	store Store
	log   *slog.Logger
}

// NewHandler builds the places handler.
func NewHandler(store Store, log *slog.Logger) *Handler {
	return &Handler{store: store, log: log.With(slog.String("component", "places"))}
}

// ListLocations returns the family's locations.
func (h *Handler) ListLocations(
	ctx context.Context,
	request api.ListLocationsRequestObject,
) (api.ListLocationsResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.ListLocations401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	includeArchived := request.Params.IncludeArchived != nil && *request.Params.IncludeArchived

	rows, err := h.store.ListLocations(ctx, db.ListLocationsParams{
		FamilyID:        pgconv.UUID(principal.FamilyID),
		IncludeArchived: includeArchived,
	})
	if err != nil {
		return nil, fmt.Errorf("list locations: %w", err)
	}

	locations := make([]api.Location, 0, len(rows))
	for _, row := range rows {
		locations = append(locations, apimap.Location(row))
	}

	return api.ListLocations200JSONResponse(locations), nil
}

// CreateLocation adds a new place.
func (h *Handler) CreateLocation(
	ctx context.Context,
	request api.CreateLocationRequestObject,
) (api.CreateLocationResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.CreateLocation401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	if request.Body == nil {
		return api.CreateLocation400JSONResponse{BadRequestJSONResponse: invalidLocation("a location needs a name and kind")}, nil
	}

	name, problem := validName(request.Body.Name)
	if problem != "" {
		return api.CreateLocation400JSONResponse{BadRequestJSONResponse: invalidLocation(problem)}, nil
	}

	sortOrder := int32(0)
	if request.Body.SortOrder != nil {
		sortOrder = *request.Body.SortOrder
	}

	row, err := h.store.CreateLocation(ctx, db.CreateLocationParams{
		FamilyID:  pgconv.UUID(principal.FamilyID),
		Name:      name,
		Kind:      db.LocationKind(request.Body.Kind),
		Address:   pgconv.TextPtr(trimmed(request.Body.Address)),
		SortOrder: sortOrder,
	})
	if err != nil {
		return nil, fmt.Errorf("create location: %w", err)
	}

	return api.CreateLocation201JSONResponse(apimap.Location(row)), nil
}

// GetLocation returns one location.
//
//nolint:dupl // only the generated request/response types differ from GetRoom
func (h *Handler) GetLocation(
	ctx context.Context,
	request api.GetLocationRequestObject,
) (api.GetLocationResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.GetLocation401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	row, err := h.store.GetLocation(ctx, db.GetLocationParams{
		ID:       pgconv.UUID(request.LocationId),
		FamilyID: pgconv.UUID(principal.FamilyID),
	})

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return api.GetLocation404JSONResponse{NotFoundJSONResponse: notFound("location")}, nil
	case err != nil:
		return nil, fmt.Errorf("load location: %w", err)
	}

	return api.GetLocation200JSONResponse(apimap.Location(row)), nil
}

// UpdateLocation changes a location. Omitted fields are left alone.
func (h *Handler) UpdateLocation(
	ctx context.Context,
	request api.UpdateLocationRequestObject,
) (api.UpdateLocationResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.UpdateLocation401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	params := db.UpdateLocationParams{
		ID:       pgconv.UUID(request.LocationId),
		FamilyID: pgconv.UUID(principal.FamilyID),
	}

	if request.Body != nil {
		if request.Body.Name != nil {
			name, problem := validName(*request.Body.Name)
			if problem != "" {
				return api.UpdateLocation400JSONResponse{BadRequestJSONResponse: invalidLocation(problem)}, nil
			}

			params.Name = pgconv.Text(name)
		}

		if request.Body.Kind != nil {
			params.Kind = db.NullLocationKind{LocationKind: db.LocationKind(*request.Body.Kind), Valid: true}
		}

		params.Address = pgconv.TextPtr(trimmed(request.Body.Address))

		if request.Body.SortOrder != nil {
			params.SortOrder = pgconv.Int4(*request.Body.SortOrder)
		}
	}

	row, err := h.store.UpdateLocation(ctx, params)

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return api.UpdateLocation404JSONResponse{NotFoundJSONResponse: notFound("location")}, nil
	case err != nil:
		return nil, fmt.Errorf("update location: %w", err)
	}

	return api.UpdateLocation200JSONResponse(apimap.Location(row)), nil
}

// ArchiveLocation retires a location.
func (h *Handler) ArchiveLocation(
	ctx context.Context,
	request api.ArchiveLocationRequestObject,
) (api.ArchiveLocationResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.ArchiveLocation401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	affected, err := h.store.ArchiveLocation(ctx, db.ArchiveLocationParams{
		ID:       pgconv.UUID(request.LocationId),
		FamilyID: pgconv.UUID(principal.FamilyID),
	})
	if err != nil {
		return nil, fmt.Errorf("archive location: %w", err)
	}

	if affected == 0 {
		return api.ArchiveLocation404JSONResponse{NotFoundJSONResponse: notFound("location")}, nil
	}

	return api.ArchiveLocation204Response{}, nil
}

// ListRooms returns the rooms of one location.
func (h *Handler) ListRooms(
	ctx context.Context,
	request api.ListRoomsRequestObject,
) (api.ListRoomsResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.ListRooms401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	includeArchived := request.Params.IncludeArchived != nil && *request.Params.IncludeArchived

	rows, err := h.store.ListRoomsByLocation(ctx, db.ListRoomsByLocationParams{
		LocationID:      pgconv.UUID(request.LocationId),
		FamilyID:        pgconv.UUID(principal.FamilyID),
		IncludeArchived: includeArchived,
	})
	if err != nil {
		return nil, fmt.Errorf("list rooms: %w", err)
	}

	rooms := make([]api.Room, 0, len(rows))
	for _, row := range rows {
		rooms = append(rooms, apimap.Room(row))
	}

	return api.ListRooms200JSONResponse(rooms), nil
}

// CreateRoom adds a room to a location.
func (h *Handler) CreateRoom(
	ctx context.Context,
	request api.CreateRoomRequestObject,
) (api.CreateRoomResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.CreateRoom401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	if request.Body == nil {
		return api.CreateRoom400JSONResponse{BadRequestJSONResponse: invalidRoom("a room needs a name")}, nil
	}

	name, problem := validName(request.Body.Name)
	if problem != "" {
		return api.CreateRoom400JSONResponse{BadRequestJSONResponse: invalidRoom(problem)}, nil
	}

	sortOrder := int32(0)
	if request.Body.SortOrder != nil {
		sortOrder = *request.Body.SortOrder
	}

	row, err := h.store.CreateRoom(ctx, db.CreateRoomParams{
		LocationID: pgconv.UUID(request.LocationId),
		FamilyID:   pgconv.UUID(principal.FamilyID),
		Name:       name,
		SortOrder:  sortOrder,
	})

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		// The WHERE EXISTS found no matching location in this family: the id
		// was wrong, archived, or someone else's. None of those is a 500.
		return api.CreateRoom404JSONResponse{NotFoundJSONResponse: notFound("location")}, nil
	case err != nil:
		return nil, fmt.Errorf("create room: %w", err)
	}

	return api.CreateRoom201JSONResponse(apimap.Room(row)), nil
}

// GetRoom returns one room.
//
//nolint:dupl // only the generated request/response types differ from GetLocation
func (h *Handler) GetRoom(
	ctx context.Context,
	request api.GetRoomRequestObject,
) (api.GetRoomResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.GetRoom401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	row, err := h.store.GetRoom(ctx, db.GetRoomParams{
		ID:       pgconv.UUID(request.RoomId),
		FamilyID: pgconv.UUID(principal.FamilyID),
	})

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return api.GetRoom404JSONResponse{NotFoundJSONResponse: notFound("room")}, nil
	case err != nil:
		return nil, fmt.Errorf("load room: %w", err)
	}

	return api.GetRoom200JSONResponse(apimap.Room(row)), nil
}

// UpdateRoom changes a room. Omitted fields are left alone.
func (h *Handler) UpdateRoom(
	ctx context.Context,
	request api.UpdateRoomRequestObject,
) (api.UpdateRoomResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.UpdateRoom401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	params := db.UpdateRoomParams{
		ID:       pgconv.UUID(request.RoomId),
		FamilyID: pgconv.UUID(principal.FamilyID),
	}

	if request.Body != nil {
		if request.Body.Name != nil {
			name, problem := validName(*request.Body.Name)
			if problem != "" {
				return api.UpdateRoom400JSONResponse{BadRequestJSONResponse: invalidRoom(problem)}, nil
			}

			params.Name = pgconv.Text(name)
		}

		if request.Body.SortOrder != nil {
			params.SortOrder = pgconv.Int4(*request.Body.SortOrder)
		}
	}

	row, err := h.store.UpdateRoom(ctx, params)

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return api.UpdateRoom404JSONResponse{NotFoundJSONResponse: notFound("room")}, nil
	case err != nil:
		return nil, fmt.Errorf("update room: %w", err)
	}

	return api.UpdateRoom200JSONResponse(apimap.Room(row)), nil
}

// ArchiveRoom retires a room.
func (h *Handler) ArchiveRoom(
	ctx context.Context,
	request api.ArchiveRoomRequestObject,
) (api.ArchiveRoomResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.ArchiveRoom401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	affected, err := h.store.ArchiveRoom(ctx, db.ArchiveRoomParams{
		ID:       pgconv.UUID(request.RoomId),
		FamilyID: pgconv.UUID(principal.FamilyID),
	})
	if err != nil {
		return nil, fmt.Errorf("archive room: %w", err)
	}

	if affected == 0 {
		return api.ArchiveRoom404JSONResponse{NotFoundJSONResponse: notFound("room")}, nil
	}

	return api.ArchiveRoom204Response{}, nil
}

func validName(raw string) (name, problem string) {
	name = strings.TrimSpace(raw)
	if name == "" || len([]rune(name)) > maxNameLen {
		return "", "a name must be between 1 and 120 characters"
	}

	return name, ""
}

func trimmed(s *string) *string {
	if s == nil {
		return nil
	}

	v := strings.TrimSpace(*s)
	if v == "" {
		return nil
	}

	return &v
}

func unauthorized() api.UnauthorizedJSONResponse {
	return api.UnauthorizedJSONResponse{Code: "no_session", Message: "sign in to continue"}
}

func notFound(what string) api.NotFoundJSONResponse {
	return api.NotFoundJSONResponse{Code: "not_found", Message: "no such " + what + " in this family"}
}

func invalidLocation(message string) api.BadRequestJSONResponse {
	return api.BadRequestJSONResponse{Code: "invalid_location", Message: message}
}

func invalidRoom(message string) api.BadRequestJSONResponse {
	return api.BadRequestJSONResponse{Code: "invalid_room", Message: message}
}
