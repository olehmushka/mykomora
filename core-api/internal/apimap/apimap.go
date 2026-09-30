// Package apimap converts database rows into the types generated from
// api/openapi.yaml.
//
// It is the one place where the storage shape meets the wire shape. Keeping
// the translation here means a column rename is a compile error in a single
// file, and that three handler packages do not each grow their own
// near-identical copy of the same twenty lines.
package apimap

import (
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/olehmushka/mykomora/core-api/internal/api"
	"github.com/olehmushka/mykomora/core-api/internal/db"
	"github.com/olehmushka/mykomora/core-api/internal/pgconv"
)

// User renders an account.
func User(u db.User) api.User {
	return api.User{
		Id:          pgconv.ToUUID(u.ID),
		Email:       openapi_types.Email(u.Email),
		DisplayName: u.DisplayName,
		AvatarUrl:   pgconv.ToTextPtr(u.AvatarUrl),
		Locale:      locale(pgconv.ToTextPtr(u.Locale)),
		CreatedAt:   pgconv.ToTime(u.CreatedAt),
		LastSeenAt:  pgconv.ToTime(u.LastSeenAt),
	}
}

// locale narrows a stored preference to the locales the contract admits.
// Anything else is reported as "not chosen" rather than echoed back, so a
// stray value in the column cannot become a value in the API.
func locale(stored *string) *api.UserLocale {
	if stored == nil {
		return nil
	}

	value := api.UserLocale(*stored)
	if value != api.En && value != api.Uk {
		return nil
	}

	return &value
}

// Family renders a tenant's settings.
func Family(f db.Family) api.Family {
	return api.Family{
		Id:              pgconv.ToUUID(f.ID),
		Name:            f.Name,
		DefaultCurrency: f.DefaultCurrency,
		CreatedAt:       pgconv.ToTime(f.CreatedAt),
	}
}

// Member renders one row of the member list.
func Member(m db.ListFamilyMembersRow) api.FamilyMember {
	return api.FamilyMember{
		UserId:      pgconv.ToUUID(m.UserID),
		Role:        api.FamilyRole(m.Role),
		JoinedAt:    pgconv.ToTime(m.JoinedAt),
		Email:       openapi_types.Email(m.Email),
		DisplayName: m.DisplayName,
		AvatarUrl:   pgconv.ToTextPtr(m.AvatarUrl),
		LastSeenAt:  pgconv.ToTimePtr(m.LastSeenAt),
	}
}

// Person renders someone the family tracks things for.
func Person(p db.Person) api.Person {
	return api.Person{
		Id:         pgconv.ToUUID(p.ID),
		Name:       p.Name,
		Birthdate:  date(pgconv.ToDatePtr(p.Birthdate)),
		IsChild:    p.IsChild,
		AvatarUrl:  pgconv.ToTextPtr(p.AvatarUrl),
		UserId:     pgconv.ToUUIDPtr(p.UserID),
		ArchivedAt: pgconv.ToTimePtr(p.ArchivedAt),
		CreatedAt:  pgconv.ToTime(p.CreatedAt),
		UpdatedAt:  pgconv.ToTime(p.UpdatedAt),
	}
}

func date(t *time.Time) *openapi_types.Date {
	if t == nil {
		return nil
	}

	return &openapi_types.Date{Time: *t}
}

// InviteState is the subset of an invite row that decides its status. Both
// the list query and the single-token lookup produce these four values, under
// different row types, so the status rule is written once against this.
type InviteState struct {
	ExpiresAt  time.Time
	AcceptedAt *time.Time
	RevokedAt  *time.Time
}

// InviteStatus collapses the invite's timestamps into the contract's enum.
//
// The order is the order in which the outcomes matter: an accepted invite is
// spent whatever else is true of it, and a revoked one was cancelled on
// purpose, which is worth distinguishing from one that merely ran out.
func InviteStatus(s InviteState, now time.Time) api.InviteStatus {
	switch {
	case s.AcceptedAt != nil:
		return api.Accepted
	case s.RevokedAt != nil:
		return api.Revoked
	case now.After(s.ExpiresAt):
		return api.Expired
	default:
		return api.Pending
	}
}
