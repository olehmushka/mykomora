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

	"github.com/google/uuid"
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
	if value != api.UserLocaleEn && value != api.UserLocaleUk {
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

// Location renders one of the family's physical places.
func Location(l db.Location) api.Location {
	return api.Location{
		Id:         pgconv.ToUUID(l.ID),
		Name:       l.Name,
		Kind:       api.LocationKind(l.Kind),
		Address:    pgconv.ToTextPtr(l.Address),
		SortOrder:  l.SortOrder,
		ArchivedAt: pgconv.ToTimePtr(l.ArchivedAt),
		CreatedAt:  pgconv.ToTime(l.CreatedAt),
		UpdatedAt:  pgconv.ToTime(l.UpdatedAt),
	}
}

// Room renders a room inside a location.
func Room(r db.Room) api.Room {
	return api.Room{
		Id:         pgconv.ToUUID(r.ID),
		LocationId: pgconv.ToUUID(r.LocationID),
		Name:       r.Name,
		SortOrder:  r.SortOrder,
		ArchivedAt: pgconv.ToTimePtr(r.ArchivedAt),
		CreatedAt:  pgconv.ToTime(r.CreatedAt),
		UpdatedAt:  pgconv.ToTime(r.UpdatedAt),
	}
}

// Item renders one possession. effectiveLocationID/effectiveRoomID/
// effectiveSpot are resolved by the caller (internal/items walks the
// ancestor chain for a nested item) rather than read off the row directly,
// because the row's own place columns are cosmetic once it has a parent
// (SPEC 4, "Containment").
func Item(it *db.Item, effectiveLocationID uuid.UUID, effectiveRoomID *uuid.UUID, effectiveSpot *string) api.Item {
	return api.Item{
		Id:                  pgconv.ToUUID(it.ID),
		Name:                it.Name,
		CategoryId:          pgconv.ToUUID(it.CategoryID),
		LocationId:          pgconv.ToUUID(it.LocationID),
		RoomId:              pgconv.ToUUIDPtr(it.RoomID),
		Spot:                pgconv.ToTextPtr(it.Spot),
		ParentItemId:        pgconv.ToUUIDPtr(it.ParentItemID),
		IsContainer:         it.IsContainer,
		Status:              api.ItemStatus(it.Status),
		Quantity:            float32(pgconv.ToNumeric(it.Quantity)),
		Unit:                pgconv.ToTextPtr(it.Unit),
		MinQuantity:         numericPtr32(pgconv.ToNumericPtr(it.MinQuantity)),
		ExpiresAt:           date(pgconv.ToDatePtr(it.ExpiresAt)),
		OpenedAt:            date(pgconv.ToDatePtr(it.OpenedAt)),
		PaoMonths:           pgconv.ToInt4Ptr(it.PaoMonths),
		Brand:               pgconv.ToTextPtr(it.Brand),
		Condition:           itemCondition(it.Condition),
		Season:              itemSeason(it.Season),
		SizeSystem:          sizeSystem(it.SizeSystem),
		SizeValue:           pgconv.ToTextPtr(it.SizeValue),
		ForPersonId:         pgconv.ToUUIDPtr(it.ForPersonID),
		SerialNumber:        pgconv.ToTextPtr(it.SerialNumber),
		WarrantyUntil:       date(pgconv.ToDatePtr(it.WarrantyUntil)),
		PurchaseDate:        date(pgconv.ToDatePtr(it.PurchaseDate)),
		PurchasePrice:       numericPtr32(pgconv.ToNumericPtr(it.PurchasePrice)),
		PurchaseCurrency:    pgconv.ToTextPtr(it.PurchaseCurrency),
		Notes:               pgconv.ToTextPtr(it.Notes),
		Traits:              pgconv.ToJSONB(it.Traits),
		EffectiveLocationId: effectiveLocationID,
		EffectiveRoomId:     effectiveRoomID,
		EffectiveSpot:       effectiveSpot,
		CreatedBy:           pgconv.ToUUID(it.CreatedBy),
		CreatedAt:           pgconv.ToTime(it.CreatedAt),
		UpdatedAt:           pgconv.ToTime(it.UpdatedAt),
		ArchivedAt:          pgconv.ToTimePtr(it.ArchivedAt),
	}
}

// ItemEvent renders one row of an item's audit trail.
func ItemEvent(e db.ItemEvent) api.ItemEvent {
	return api.ItemEvent{
		Id:          pgconv.ToUUID(e.ID),
		ItemId:      pgconv.ToUUID(e.ItemID),
		ActorUserId: pgconv.ToUUID(e.ActorUserID),
		Type:        e.Type,
		Payload:     pgconv.ToJSONB(e.Payload),
		CreatedAt:   pgconv.ToTime(e.CreatedAt),
	}
}

// Tag renders a family-defined label.
func Tag(t db.Tag) api.Tag {
	return api.Tag{
		Id:    pgconv.ToUUID(t.ID),
		Name:  t.Name,
		Color: pgconv.ToTextPtr(t.Color),
	}
}

func numericPtr32(v *float64) *float32 {
	if v == nil {
		return nil
	}

	f := float32(*v)

	return &f
}

func itemCondition(v db.NullItemCondition) *api.ItemCondition {
	if !v.Valid {
		return nil
	}

	c := api.ItemCondition(v.ItemCondition)

	return &c
}

func itemSeason(v db.NullItemSeason) *api.ItemSeason {
	if !v.Valid {
		return nil
	}

	s := api.ItemSeason(v.ItemSeason)

	return &s
}

func sizeSystem(v db.NullSizeSystem) *api.SizeSystem {
	if !v.Valid {
		return nil
	}

	s := api.SizeSystem(v.SizeSystem)

	return &s
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
