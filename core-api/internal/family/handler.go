// Package family serves the tenant itself: its settings, its members, and the
// invite links that let a second Google account join it.
//
// Every operation here resolves its family from the session and passes that id
// to the query. Nothing takes a family id from the path or the body, which is
// the tenancy invariant in practice rather than in principle (SPEC 4).
package family

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/olehmushka/mykomora/core-api/internal/api"
	"github.com/olehmushka/mykomora/core-api/internal/apimap"
	"github.com/olehmushka/mykomora/core-api/internal/auth"
	"github.com/olehmushka/mykomora/core-api/internal/config"
	"github.com/olehmushka/mykomora/core-api/internal/db"
	"github.com/olehmushka/mykomora/core-api/internal/pgconv"
)

// Store is the slice of the generated queries this package needs.
type Store interface {
	GetFamily(ctx context.Context, arg db.GetFamilyParams) (db.Family, error)
	UpdateFamily(ctx context.Context, arg db.UpdateFamilyParams) (db.Family, error)
	ListFamilyMembers(ctx context.Context, familyID pgtype.UUID) ([]db.ListFamilyMembersRow, error)
	RemoveFamilyMember(ctx context.Context, arg db.RemoveFamilyMemberParams) (int64, error)
	CreateFamilyInvite(ctx context.Context, arg db.CreateFamilyInviteParams) (db.CreateFamilyInviteRow, error)
	ListFamilyInvites(ctx context.Context, familyID pgtype.UUID) ([]db.ListFamilyInvitesRow, error)
	RevokeFamilyInvite(ctx context.Context, arg db.RevokeFamilyInviteParams) (int64, error)
	GetInviteByTokenHash(ctx context.Context, tokenHash []byte) (db.GetInviteByTokenHashRow, error)
}

// Handler serves the family operations of the generated contract.
type Handler struct {
	store     Store
	baseURL   string
	inviteTTL time.Duration
	log       *slog.Logger
}

// NewHandler builds the family handler.
func NewHandler(store Store, cfg config.Config, log *slog.Logger) *Handler {
	return &Handler{
		store:     store,
		baseURL:   strings.TrimSuffix(cfg.PublicBaseURL, "/"),
		inviteTTL: cfg.InviteTTL,
		log:       log.With(slog.String("component", "family")),
	}
}

// GetFamily returns the current family's settings.
func (h *Handler) GetFamily(
	ctx context.Context,
	_ api.GetFamilyRequestObject,
) (api.GetFamilyResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.GetFamily401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	row, err := h.store.GetFamily(ctx, db.GetFamilyParams{
		FamilyID: pgconv.UUID(principal.FamilyID),
		UserID:   pgconv.UUID(principal.UserID),
	})

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return api.GetFamily404JSONResponse{NotFoundJSONResponse: notFound("family")}, nil
	case err != nil:
		return nil, fmt.Errorf("load family: %w", err)
	}

	return api.GetFamily200JSONResponse(apimap.Family(row)), nil
}

// currencyPattern is ISO 4217: three letters, and nothing clever.
var currencyPattern = regexp.MustCompile(`^[A-Za-z]{3}$`)

// maxFamilyNameLen mirrors the contract's own limit.
const maxFamilyNameLen = 120

// UpdateFamily changes the family's settings. Owners only.
func (h *Handler) UpdateFamily(
	ctx context.Context,
	request api.UpdateFamilyRequestObject,
) (api.UpdateFamilyResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.UpdateFamily401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	if !principal.IsOwner() {
		return api.UpdateFamily403JSONResponse{ForbiddenJSONResponse: ownersOnly()}, nil
	}

	params, problem := updateParams(request.Body, principal)
	if problem != "" {
		return api.UpdateFamily400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
			Code:    "invalid_family",
			Message: problem,
		}}, nil
	}

	row, err := h.store.UpdateFamily(ctx, params)

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return api.UpdateFamily404JSONResponse{NotFoundJSONResponse: notFound("family")}, nil
	case err != nil:
		return nil, fmt.Errorf("update family: %w", err)
	}

	return api.UpdateFamily200JSONResponse(apimap.Family(row)), nil
}

// updateParams validates the request and builds the query arguments. An empty
// problem string means the request is acceptable.
func updateParams(
	body *api.UpdateFamilyRequest,
	principal auth.Principal,
) (params db.UpdateFamilyParams, problem string) {
	params = db.UpdateFamilyParams{
		FamilyID: pgconv.UUID(principal.FamilyID),
		UserID:   pgconv.UUID(principal.UserID),
	}

	if body == nil {
		return params, ""
	}

	if body.Name != nil {
		name := strings.TrimSpace(*body.Name)
		if name == "" || len([]rune(name)) > maxFamilyNameLen {
			return params, "the family name must be between 1 and 120 characters"
		}

		params.Name = pgconv.Text(name)
	}

	if body.DefaultCurrency != nil {
		currency := strings.TrimSpace(*body.DefaultCurrency)
		if !currencyPattern.MatchString(currency) {
			return params, "the currency must be a three-letter ISO 4217 code"
		}

		params.DefaultCurrency = pgconv.Text(strings.ToUpper(currency))
	}

	return params, ""
}

// ListFamilyMembers returns everyone in the family.
func (h *Handler) ListFamilyMembers(
	ctx context.Context,
	_ api.ListFamilyMembersRequestObject,
) (api.ListFamilyMembersResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.ListFamilyMembers401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	rows, err := h.store.ListFamilyMembers(ctx, pgconv.UUID(principal.FamilyID))
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}

	members := make([]api.FamilyMember, 0, len(rows))
	for _, row := range rows {
		members = append(members, apimap.Member(row))
	}

	return api.ListFamilyMembers200JSONResponse(members), nil
}

// RemoveFamilyMember removes a member. Owners only, and owners are not
// removable: there is no role-change endpoint yet, so losing the last owner
// would leave the family with nobody able to administer it.
//
// The body below is the same shape as the other owner-only mutation in this
// package, and deliberately stays that way. Everything that could be shared
// already is, in asOwner; what is left is the mapping from an outcome to this
// operation's own generated response types, which Go cannot abstract over —
// a composite literal of a type parameter is not a thing. Collapsing it
// further would mean inventing indirection to satisfy a duplication detector.
//
//nolint:dupl // only the generated response types differ; see above
func (h *Handler) RemoveFamilyMember(
	ctx context.Context,
	request api.RemoveFamilyMemberRequestObject,
) (api.RemoveFamilyMemberResponseObject, error) {
	outcome, err := asOwner(ctx, func(p auth.Principal) (int64, error) {
		return h.store.RemoveFamilyMember(ctx, db.RemoveFamilyMemberParams{
			FamilyID: pgconv.UUID(p.FamilyID),
			UserID:   pgconv.UUID(request.UserId),
		})
	})
	if err != nil {
		return nil, fmt.Errorf("remove member: %w", err)
	}

	switch outcome {
	case outcomeUnauthenticated:
		return api.RemoveFamilyMember401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	case outcomeForbidden:
		return api.RemoveFamilyMember403JSONResponse{ForbiddenJSONResponse: ownersOnly()}, nil
	case outcomeMissing:
		return api.RemoveFamilyMember404JSONResponse{NotFoundJSONResponse: notFound("member")}, nil
	case outcomeDone:
	}

	return api.RemoveFamilyMember204Response{}, nil
}

// outcome is the result of an owner-only mutation that reports itself as a
// row count.
//
// Removing a member and revoking an invite are the same five decisions in a
// row — is there a session, is it an owner, did the statement match anything —
// and only the generated response types differ. The decisions live here once;
// each operation keeps its own typed answers, because those are what the
// contract promises.
type outcome int

const (
	outcomeDone outcome = iota
	outcomeUnauthenticated
	outcomeForbidden
	outcomeMissing
)

func asOwner(ctx context.Context, exec func(auth.Principal) (int64, error)) (outcome, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return outcomeUnauthenticated, nil
	}

	if !principal.IsOwner() {
		return outcomeForbidden, nil
	}

	affected, err := exec(principal)
	if err != nil {
		return outcomeDone, err
	}

	if affected == 0 {
		return outcomeMissing, nil
	}

	return outcomeDone, nil
}

func unauthorized() api.UnauthorizedJSONResponse {
	return api.UnauthorizedJSONResponse{Code: "no_session", Message: "sign in to continue"}
}

func ownersOnly() api.ForbiddenJSONResponse {
	return api.ForbiddenJSONResponse{
		Code:    "owner_required",
		Message: "only the family owner can do this",
	}
}

func notFound(what string) api.NotFoundJSONResponse {
	return api.NotFoundJSONResponse{
		Code:    "not_found",
		Message: "no such " + what + " in this family",
	}
}
