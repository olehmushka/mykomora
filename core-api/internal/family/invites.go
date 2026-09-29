package family

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/olehmushka/mykomora/core-api/internal/api"
	"github.com/olehmushka/mykomora/core-api/internal/apimap"
	"github.com/olehmushka/mykomora/core-api/internal/auth"
	"github.com/olehmushka/mykomora/core-api/internal/db"
	"github.com/olehmushka/mykomora/core-api/internal/pgconv"
)

// joinPath is the web route an invite link points at. The page there previews
// the invite and then hands the token back to the sign-in flow.
const joinPath = "/join/"

// CreateFamilyInvite issues a single-use link. Owners only.
//
// The raw token is returned exactly once, in this response. Only its digest is
// stored, so nothing — not a later API call, not a database dump — can produce
// the link again.
func (h *Handler) CreateFamilyInvite(
	ctx context.Context,
	_ api.CreateFamilyInviteRequestObject,
) (api.CreateFamilyInviteResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.CreateFamilyInvite401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	if !principal.IsOwner() {
		return api.CreateFamilyInvite403JSONResponse{ForbiddenJSONResponse: ownersOnly()}, nil
	}

	token, err := auth.NewToken()
	if err != nil {
		return nil, fmt.Errorf("generate invite token: %w", err)
	}

	row, err := h.store.CreateFamilyInvite(ctx, db.CreateFamilyInviteParams{
		FamilyID:  pgconv.UUID(principal.FamilyID),
		TokenHash: auth.HashToken(token),
		CreatedBy: pgconv.UUID(principal.UserID),
		ExpiresAt: pgconv.Timestamptz(time.Now().Add(h.inviteTTL)),
	})
	if err != nil {
		return nil, fmt.Errorf("create invite: %w", err)
	}

	return api.CreateFamilyInvite201JSONResponse{
		Invite: api.FamilyInvite{
			Id:        pgconv.ToUUID(row.ID),
			Status:    api.Pending,
			CreatedAt: pgconv.ToTime(row.CreatedAt),
			ExpiresAt: pgconv.ToTime(row.ExpiresAt),
		},
		Url: h.baseURL + joinPath + token,
	}, nil
}

// ListFamilyInvites reports the invites issued for this family, without their
// tokens: the status is administrable, the link is not recoverable.
func (h *Handler) ListFamilyInvites(
	ctx context.Context,
	_ api.ListFamilyInvitesRequestObject,
) (api.ListFamilyInvitesResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.ListFamilyInvites401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	if !principal.IsOwner() {
		return api.ListFamilyInvites403JSONResponse{ForbiddenJSONResponse: ownersOnly()}, nil
	}

	rows, err := h.store.ListFamilyInvites(ctx, pgconv.UUID(principal.FamilyID))
	if err != nil {
		return nil, fmt.Errorf("list invites: %w", err)
	}

	now := time.Now()
	invites := make([]api.FamilyInvite, 0, len(rows))

	for _, row := range rows {
		invites = append(invites, api.FamilyInvite{
			Id:         pgconv.ToUUID(row.ID),
			CreatedAt:  pgconv.ToTime(row.CreatedAt),
			ExpiresAt:  pgconv.ToTime(row.ExpiresAt),
			AcceptedAt: pgconv.ToTimePtr(row.AcceptedAt),
			Status: apimap.InviteStatus(apimap.InviteState{
				ExpiresAt:  pgconv.ToTime(row.ExpiresAt),
				AcceptedAt: pgconv.ToTimePtr(row.AcceptedAt),
				RevokedAt:  pgconv.ToTimePtr(row.RevokedAt),
			}, now),
		})
	}

	return api.ListFamilyInvites200JSONResponse(invites), nil
}

// RevokeFamilyInvite cancels an invite that has not been used yet.
//
// The body below is the same shape as the other owner-only mutation in this
// package, and deliberately stays that way. Everything that could be shared
// already is, in asOwner; what is left is the mapping from an outcome to this
// operation's own generated response types, which Go cannot abstract over —
// a composite literal of a type parameter is not a thing. Collapsing it
// further would mean inventing indirection to satisfy a duplication detector.
//
//nolint:dupl // see above: only the generated response types differ
func (h *Handler) RevokeFamilyInvite(
	ctx context.Context,
	request api.RevokeFamilyInviteRequestObject,
) (api.RevokeFamilyInviteResponseObject, error) {
	result, err := asOwner(ctx, func(p auth.Principal) (int64, error) {
		return h.store.RevokeFamilyInvite(ctx, db.RevokeFamilyInviteParams{
			ID:       pgconv.UUID(request.InviteId),
			FamilyID: pgconv.UUID(p.FamilyID),
		})
	})
	if err != nil {
		return nil, fmt.Errorf("revoke invite: %w", err)
	}

	switch result {
	case outcomeUnauthenticated:
		return api.RevokeFamilyInvite401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	case outcomeForbidden:
		return api.RevokeFamilyInvite403JSONResponse{ForbiddenJSONResponse: ownersOnly()}, nil
	case outcomeMissing:
		// Either it is not this family's invite, or it was already accepted or
		// revoked. All three are "nothing here to cancel".
		return api.RevokeFamilyInvite404JSONResponse{NotFoundJSONResponse: notFound("invite")}, nil
	case outcomeDone:
	}

	return api.RevokeFamilyInvite204Response{}, nil
}

// GetInvitePreview tells an invitee what a link leads to, before they have an
// account.
//
// It is the only unauthenticated endpoint that reads tenant data, and it
// returns exactly one field of it: the family's name. An invite link is enough
// to see who invited you, and never enough to see what they own.
func (h *Handler) GetInvitePreview(
	ctx context.Context,
	request api.GetInvitePreviewRequestObject,
) (api.GetInvitePreviewResponseObject, error) {
	row, err := h.store.GetInviteByTokenHash(ctx, auth.HashToken(request.Token))

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		// A token that was never issued. There is no family to name, so there
		// is nothing to preview.
		return api.GetInvitePreview404JSONResponse{NotFoundJSONResponse: notFound("invite")}, nil
	case err != nil:
		return nil, fmt.Errorf("load invite: %w", err)
	}

	status := apimap.InviteStatus(apimap.InviteState{
		ExpiresAt:  pgconv.ToTime(row.ExpiresAt),
		AcceptedAt: pgconv.ToTimePtr(row.AcceptedAt),
		RevokedAt:  pgconv.ToTimePtr(row.RevokedAt),
	}, time.Now())

	return api.GetInvitePreview200JSONResponse{
		Usable:     status == api.Pending,
		FamilyName: row.FamilyName,
		Status:     &status,
	}, nil
}
