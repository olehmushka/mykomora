// Package people serves the persons a family tracks things for.
//
// A person is not a member: children will not have Google accounts, and the
// second-priority job of this application is knowing what fits them (SPEC 3,
// "Concepts"). People are archived rather than deleted, so the items recorded
// against someone keep their history.
package people

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/olehmushka/mykomora/core-api/internal/api"
	"github.com/olehmushka/mykomora/core-api/internal/apimap"
	"github.com/olehmushka/mykomora/core-api/internal/auth"
	"github.com/olehmushka/mykomora/core-api/internal/db"
	"github.com/olehmushka/mykomora/core-api/internal/pgconv"
)

// maxNameLen mirrors the contract's own limit.
const maxNameLen = 120

// Store is the slice of the generated queries this package needs.
type Store interface {
	CreatePerson(ctx context.Context, arg db.CreatePersonParams) (db.Person, error)
	GetPerson(ctx context.Context, arg db.GetPersonParams) (db.Person, error)
	ListPeople(ctx context.Context, arg db.ListPeopleParams) ([]db.Person, error)
	UpdatePerson(ctx context.Context, arg db.UpdatePersonParams) (db.Person, error)
	ArchivePerson(ctx context.Context, arg db.ArchivePersonParams) (int64, error)
}

// Handler serves the people operations of the generated contract.
type Handler struct {
	store Store
	log   *slog.Logger
}

// NewHandler builds the people handler.
func NewHandler(store Store, log *slog.Logger) *Handler {
	return &Handler{store: store, log: log.With(slog.String("component", "people"))}
}

// ListPeople returns the family's people, children first.
func (h *Handler) ListPeople(
	ctx context.Context,
	request api.ListPeopleRequestObject,
) (api.ListPeopleResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.ListPeople401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	includeArchived := request.Params.IncludeArchived != nil && *request.Params.IncludeArchived

	rows, err := h.store.ListPeople(ctx, db.ListPeopleParams{
		FamilyID:        pgconv.UUID(principal.FamilyID),
		IncludeArchived: includeArchived,
	})
	if err != nil {
		return nil, fmt.Errorf("list people: %w", err)
	}

	people := make([]api.Person, 0, len(rows))
	for _, row := range rows {
		people = append(people, apimap.Person(row))
	}

	return api.ListPeople200JSONResponse(people), nil
}

// CreatePerson adds someone the family tracks things for.
func (h *Handler) CreatePerson(
	ctx context.Context,
	request api.CreatePersonRequestObject,
) (api.CreatePersonResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.CreatePerson401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	if request.Body == nil {
		return api.CreatePerson400JSONResponse{BadRequestJSONResponse: invalidPerson("a person needs at least a name")}, nil
	}

	name, problem := validName(request.Body.Name)
	if problem != "" {
		return api.CreatePerson400JSONResponse{BadRequestJSONResponse: invalidPerson(problem)}, nil
	}

	row, err := h.store.CreatePerson(ctx, db.CreatePersonParams{
		FamilyID:  pgconv.UUID(principal.FamilyID),
		Name:      name,
		Birthdate: pgconv.Date(dateValue(request.Body.Birthdate)),
		UserID:    pgconv.UUIDPtr(request.Body.UserId),
		IsChild:   request.Body.IsChild != nil && *request.Body.IsChild,
		AvatarUrl: pgconv.TextPtr(trimmed(request.Body.AvatarUrl)),
	})
	if err != nil {
		return nil, fmt.Errorf("create person: %w", err)
	}

	return api.CreatePerson201JSONResponse(apimap.Person(row)), nil
}

// GetPerson returns one person.
func (h *Handler) GetPerson(
	ctx context.Context,
	request api.GetPersonRequestObject,
) (api.GetPersonResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.GetPerson401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	row, err := h.store.GetPerson(ctx, db.GetPersonParams{
		ID:       pgconv.UUID(request.PersonId),
		FamilyID: pgconv.UUID(principal.FamilyID),
	})

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return api.GetPerson404JSONResponse{NotFoundJSONResponse: notFound()}, nil
	case err != nil:
		return nil, fmt.Errorf("load person: %w", err)
	}

	return api.GetPerson200JSONResponse(apimap.Person(row)), nil
}

// UpdatePerson changes a person. Omitted fields are left alone.
func (h *Handler) UpdatePerson(
	ctx context.Context,
	request api.UpdatePersonRequestObject,
) (api.UpdatePersonResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.UpdatePerson401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	params := db.UpdatePersonParams{
		ID:       pgconv.UUID(request.PersonId),
		FamilyID: pgconv.UUID(principal.FamilyID),
	}

	if request.Body != nil {
		if request.Body.Name != nil {
			name, problem := validName(*request.Body.Name)
			if problem != "" {
				return api.UpdatePerson400JSONResponse{BadRequestJSONResponse: invalidPerson(problem)}, nil
			}

			params.Name = pgconv.Text(name)
		}

		params.Birthdate = pgconv.Date(dateValue(request.Body.Birthdate))
		params.UserID = pgconv.UUIDPtr(request.Body.UserId)
		params.IsChild = pgconv.Bool(request.Body.IsChild)
		params.AvatarUrl = pgconv.TextPtr(trimmed(request.Body.AvatarUrl))
	}

	row, err := h.store.UpdatePerson(ctx, params)

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		// Either no such person in this family, or they are already archived.
		return api.UpdatePerson404JSONResponse{NotFoundJSONResponse: notFound()}, nil
	case err != nil:
		return nil, fmt.Errorf("update person: %w", err)
	}

	return api.UpdatePerson200JSONResponse(apimap.Person(row)), nil
}

// ArchivePerson retires a person without losing what is recorded against them.
func (h *Handler) ArchivePerson(
	ctx context.Context,
	request api.ArchivePersonRequestObject,
) (api.ArchivePersonResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.ArchivePerson401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	affected, err := h.store.ArchivePerson(ctx, db.ArchivePersonParams{
		ID:       pgconv.UUID(request.PersonId),
		FamilyID: pgconv.UUID(principal.FamilyID),
	})
	if err != nil {
		return nil, fmt.Errorf("archive person: %w", err)
	}

	if affected == 0 {
		return api.ArchivePerson404JSONResponse{NotFoundJSONResponse: notFound()}, nil
	}

	return api.ArchivePerson204Response{}, nil
}

// validName trims and bounds a name. An empty problem means it is acceptable.
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

func dateValue(d *openapi_types.Date) *time.Time {
	if d == nil {
		return nil
	}

	t := d.Time

	return &t
}

func unauthorized() api.UnauthorizedJSONResponse {
	return api.UnauthorizedJSONResponse{Code: "no_session", Message: "sign in to continue"}
}

func notFound() api.NotFoundJSONResponse {
	return api.NotFoundJSONResponse{Code: "not_found", Message: "no such person in this family"}
}

// invalidPerson is the standard 400 body; each operation wraps it in its own
// response type.
func invalidPerson(message string) api.BadRequestJSONResponse {
	return api.BadRequestJSONResponse{Code: "invalid_person", Message: message}
}
