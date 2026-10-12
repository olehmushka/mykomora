// Package items serves possessions: creation, editing, containment, the
// lifecycle and its audit trail, plus the family's tags (SPEC 3, "Concepts"
// and SPEC 4, "Items").
//
// Service, not a flat Handler, is the template internal/auth's Service
// sets: every mutation here writes its item_events row in the same
// transaction as the item write it describes (SPEC 4, "Audit events
// written in the same transaction as every mutation"), which needs a real
// *postgres.TxRunner, not just the pooled Querier places/taxonomy's simpler
// CRUD gets away with.
//
// CreateItem/MoveItem validate their referenced category, location, room,
// parent and person with their own Go-level reads rather than folding the
// checks into the INSERT's WHERE EXISTS the way rooms.sql/trait_defs.sql
// do: those tables have one referential check each, items has up to four
// plus trait, depth and cycle validation that cannot be expressed as a
// single SQL guard anyway. The small TOCTOU window this opens (a reference
// archived between the check and the write) cannot leak across families —
// every write still carries its own family_id — so the trade is a plainer
// INSERT, not a safety gap.
package items

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/olehmushka/mykomora/core-api/internal/api"
	"github.com/olehmushka/mykomora/core-api/internal/apimap"
	"github.com/olehmushka/mykomora/core-api/internal/auth"
	"github.com/olehmushka/mykomora/core-api/internal/db"
	"github.com/olehmushka/mykomora/core-api/internal/pgconv"
	"github.com/olehmushka/mykomora/core-api/internal/postgres"
	"github.com/olehmushka/mykomora/core-api/internal/storage"
)

const (
	maxNameLen    = 120
	maxTagNameLen = 60
)

// NotFoundError names a referenced record — the item itself, or something
// it points at — that does not exist or does not belong to this family.
// Its message matches the "no such X in this family" shape every other
// handler package's notFound() helper already produces.
type NotFoundError struct{ What string }

func (e *NotFoundError) Error() string { return "no such " + e.What + " in this family" }

// ValidationError is a request the caller could fix themselves: a bad
// field, an illegal transition, a cycle, a depth limit.
type ValidationError struct{ Msg string }

func (e *ValidationError) Error() string { return e.Msg }

// wrapNotFound turns a store lookup's error into the one of three shapes
// every call site needs: nil (success), a *NotFoundError (no such row in
// this family), or a wrapped generic error. Collapsing this into one
// function is what keeps CreateItem/UpdateItem/MoveItem's own cyclomatic
// complexity down — each lookup becomes a single `if`, not an
// errors.Is/else-if pair.
func wrapNotFound(err error, what string) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, pgx.ErrNoRows):
		return &NotFoundError{What: what}
	default:
		return fmt.Errorf("%s: %w", what, err)
	}
}

// Service implements every items, tags, photos and attachments operation.
type Service struct {
	store   Store
	tx      *postgres.TxRunner
	storage storage.Client
	log     *slog.Logger
}

// NewService builds the items service.
func NewService(store Store, tx *postgres.TxRunner, storageClient storage.Client, log *slog.Logger) *Service {
	return &Service{store: store, tx: tx, storage: storageClient, log: log.With(slog.String("component", "items"))}
}

// present reports whether a hot field will have a value after this request:
// either the request supplies it directly, or the stored row already has
// one. Used so a required hot-column trait_def is satisfied without
// re-demanding a field a prior request already set.
func present[T any](override *T, storedValid bool) bool {
	return override != nil || storedValid
}

// --- reads -----------------------------------------------------------------

// GetItem returns one item, with its effective place resolved.
func (s *Service) GetItem(ctx context.Context, principal auth.Principal, itemID uuid.UUID) (api.Item, error) {
	row, err := s.store.GetItem(ctx, db.GetItemParams{ID: pgconv.UUID(itemID), FamilyID: pgconv.UUID(principal.FamilyID)})
	if wrapped := wrapNotFound(err, "item"); wrapped != nil {
		return api.Item{}, wrapped
	}

	return s.toAPIItem(ctx, principal.FamilyID, &row)
}

// ListContents returns an item's direct children. They all sit one level
// below the same parent, so the parent's own effective place (resolved
// once) is every child's effective place too — no per-child chain walk.
func (s *Service) ListContents(ctx context.Context, principal auth.Principal, itemID uuid.UUID) ([]api.Item, error) {
	familyID := pgconv.UUID(principal.FamilyID)

	parent, err := s.store.GetItem(ctx, db.GetItemParams{ID: pgconv.UUID(itemID), FamilyID: familyID})
	if wrapped := wrapNotFound(err, "item"); wrapped != nil {
		return nil, wrapped
	}

	effLocation, effRoom, effSpot, err := s.effectivePlace(ctx, principal.FamilyID, &parent)
	if err != nil {
		return nil, err
	}

	children, err := s.store.ListItemChildren(ctx, db.ListItemChildrenParams{
		FamilyID:        familyID,
		ParentItemID:    parent.ID,
		IncludeArchived: false,
	})
	if err != nil {
		return nil, fmt.Errorf("list item children: %w", err)
	}

	out := make([]api.Item, 0, len(children))

	for i := range children {
		out = append(out, apimap.Item(&children[i], effLocation, effRoom, effSpot))
	}

	return out, nil
}

// ListEvents returns an item's history, most recent first.
func (s *Service) ListEvents(
	ctx context.Context, principal auth.Principal, itemID uuid.UUID, params api.ListItemEventsParams,
) ([]api.ItemEvent, error) {
	familyID := pgconv.UUID(principal.FamilyID)

	_, err := s.store.GetItem(ctx, db.GetItemParams{ID: pgconv.UUID(itemID), FamilyID: familyID})
	if wrapped := wrapNotFound(err, "item"); wrapped != nil {
		return nil, wrapped
	}

	limit, offset := pageBounds(params.Limit, params.Offset)

	rows, err := s.store.ListItemEvents(ctx, db.ListItemEventsParams{
		ItemID:       pgconv.UUID(itemID),
		FamilyID:     familyID,
		ResultOffset: offset,
		ResultLimit:  limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list item events: %w", err)
	}

	out := make([]api.ItemEvent, 0, len(rows))
	for _, row := range rows {
		out = append(out, apimap.ItemEvent(row))
	}

	return out, nil
}

// --- create ------------------------------------------------------------

// CreateItem adds a possession, either placed directly or inside an
// existing container.
func (s *Service) CreateItem(ctx context.Context, principal auth.Principal, body api.CreateItemRequest) (api.Item, error) {
	familyID := pgconv.UUID(principal.FamilyID)

	params, tagIDs, err := s.prepareCreate(ctx, principal, &body)
	if err != nil {
		return api.Item{}, err
	}

	created, err := s.createItemTx(ctx, principal, familyID, params, tagIDs)
	if err != nil {
		return api.Item{}, err
	}

	return s.toAPIItem(ctx, principal.FamilyID, &created)
}

// prepareCreate validates a create request against the caller's family and
// turns it into the insert parameters plus the tags to attach. It writes
// nothing, so batch creation can prepare every row before opening its one
// transaction.
func (s *Service) prepareCreate(
	ctx context.Context, principal auth.Principal, body *api.CreateItemRequest,
) (*db.CreateItemParams, []uuid.UUID, error) {
	familyID := pgconv.UUID(principal.FamilyID)

	name, status, err := validateCreateBasics(*body)
	if err != nil {
		return nil, nil, err
	}

	category, err := s.store.GetCategory(ctx, db.GetCategoryParams{ID: pgconv.UUID(body.CategoryId), FamilyID: familyID})
	if wrapped := wrapNotFound(err, "category"); wrapped != nil {
		return nil, nil, wrapped
	}

	locationID, roomID, parentID, spot, err := s.resolveCreatePlace(ctx, familyID, *body)
	if err != nil {
		return nil, nil, err
	}

	if body.ForPersonId != nil {
		_, err := s.store.GetPerson(ctx, db.GetPersonParams{ID: pgconv.UUID(*body.ForPersonId), FamilyID: familyID})
		if wrapped := wrapNotFound(err, "person"); wrapped != nil {
			return nil, nil, wrapped
		}
	}

	traits := map[string]any{}
	if body.Traits != nil {
		traits = *body.Traits
	}

	longTail, err := s.validateTraits(ctx, familyID, category.ID, traits, hotFieldsFromCreate(*body))
	if err != nil {
		return nil, nil, err
	}

	tagIDs, err := s.checkTagIDs(ctx, familyID, body.TagIds)
	if err != nil {
		return nil, nil, err
	}

	return &db.CreateItemParams{
		FamilyID: familyID, Name: name, CategoryID: category.ID,
		LocationID: locationID, RoomID: roomID, Spot: spot, ParentItemID: parentID,
		IsContainer:      body.IsContainer,
		Status:           db.ItemStatus(status),
		Quantity:         pgconv.Numeric(float64(body.Quantity)),
		Unit:             pgconv.TextPtr(body.Unit),
		MinQuantity:      pgconv.NumericPtr(float64Ptr(body.MinQuantity)),
		ExpiresAt:        pgconv.Date(dateToTimePtr(body.ExpiresAt)),
		OpenedAt:         pgconv.Date(dateToTimePtr(body.OpenedAt)),
		PaoMonths:        pgconv.Int4Ptr(body.PaoMonths),
		Brand:            pgconv.TextPtr(body.Brand),
		Condition:        nullCondition(body.Condition),
		Season:           nullSeason(body.Season),
		SizeSystem:       nullSizeSystem(body.SizeSystem),
		SizeValue:        pgconv.TextPtr(body.SizeValue),
		ForPersonID:      pgconv.UUIDPtr(body.ForPersonId),
		SerialNumber:     pgconv.TextPtr(body.SerialNumber),
		WarrantyUntil:    pgconv.Date(dateToTimePtr(body.WarrantyUntil)),
		PurchaseDate:     pgconv.Date(dateToTimePtr(body.PurchaseDate)),
		PurchasePrice:    pgconv.NumericPtr(float64Ptr(body.PurchasePrice)),
		PurchaseCurrency: pgconv.TextPtr(body.PurchaseCurrency),
		Notes:            pgconv.TextPtr(body.Notes),
		Traits:           pgconv.JSONB(longTail),
		CreatedBy:        pgconv.UUID(principal.UserID),
	}, tagIDs, nil
}

// validateCreateBasics checks the fields CreateItem can validate with no
// database access: the name, that exactly one place target is given, that
// any caller-supplied status is active, and that quantity is sane.
func validateCreateBasics(body api.CreateItemRequest) (name string, status api.ItemStatus, err error) {
	name, problem := validName(body.Name)
	if problem != "" {
		return "", "", &ValidationError{Msg: problem}
	}

	if (body.LocationId != nil) == (body.ParentItemId != nil) {
		return "", "", &ValidationError{Msg: "exactly one of locationId or parentItemId must be given"}
	}

	status = api.InUse
	if body.Status != nil {
		status = *body.Status
	}

	if !IsActive(status) {
		return "", "", &ValidationError{Msg: "status must be an active status when creating an item"}
	}

	if body.Quantity < 0 {
		return "", "", &ValidationError{Msg: "quantity must not be negative"}
	}

	return name, status, nil
}

// resolveCreatePlace resolves the new item's place columns from exactly one
// of a location-target or a parent-target — validateCreateBasics already
// confirmed which.
func (s *Service) resolveCreatePlace(
	ctx context.Context, familyID pgtype.UUID, body api.CreateItemRequest,
) (locationID, roomID, parentID pgtype.UUID, spot pgtype.Text, err error) {
	if body.LocationId != nil {
		return s.resolveLocationPlace(ctx, familyID, *body.LocationId, body.RoomId, body.Spot)
	}

	return s.resolveParentPlace(ctx, familyID, *body.ParentItemId)
}

func (s *Service) resolveLocationPlace(
	ctx context.Context, familyID pgtype.UUID, locationID uuid.UUID, roomID *uuid.UUID, spot *string,
) (resolvedLocationID, resolvedRoomID, resolvedParentID pgtype.UUID, resolvedSpot pgtype.Text, err error) {
	location, err := s.store.GetLocation(ctx, db.GetLocationParams{ID: pgconv.UUID(locationID), FamilyID: familyID})
	if wrapped := wrapNotFound(err, "location"); wrapped != nil {
		return pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, pgtype.Text{}, wrapped
	}

	var room pgtype.UUID

	if roomID != nil {
		r, err := s.store.GetRoom(ctx, db.GetRoomParams{ID: pgconv.UUID(*roomID), FamilyID: familyID})
		if wrapped := wrapNotFound(err, "room"); wrapped != nil {
			return pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, pgtype.Text{}, wrapped
		}

		if pgconv.ToUUID(r.LocationID) != locationID {
			return pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, pgtype.Text{}, &ValidationError{Msg: "that room is not in the given location"}
		}

		room = r.ID
	}

	return location.ID, room, pgtype.UUID{}, pgconv.TextPtr(spot), nil
}

func (s *Service) resolveParentPlace(
	ctx context.Context, familyID pgtype.UUID, parentItemID uuid.UUID,
) (resolvedLocationID, resolvedRoomID, resolvedParentID pgtype.UUID, resolvedSpot pgtype.Text, err error) {
	parent, err := s.store.GetItem(ctx, db.GetItemParams{ID: pgconv.UUID(parentItemID), FamilyID: familyID})
	if wrapped := wrapNotFound(err, "item"); wrapped != nil {
		return pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, pgtype.Text{}, wrapped
	}

	if !parent.IsContainer {
		return pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, pgtype.Text{}, &ValidationError{Msg: "the target item is not a container"}
	}

	chain, err := s.store.GetItemAncestorChain(ctx, db.GetItemAncestorChainParams{ItemID: parent.ID, FamilyID: familyID})
	if err != nil {
		return pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, pgtype.Text{}, fmt.Errorf("get item ancestor chain: %w", err)
	}

	if newChildDepth(chain) > maxDepth {
		return pgtype.UUID{}, pgtype.UUID{}, pgtype.UUID{}, pgtype.Text{}, &ValidationError{Msg: "would exceed the maximum container depth"}
	}

	return parent.LocationID, parent.RoomID, parent.ID, parent.Spot, nil
}

func (s *Service) createItemTx(
	ctx context.Context, principal auth.Principal, familyID pgtype.UUID, params *db.CreateItemParams, tagIDs []uuid.UUID,
) (db.Item, error) {
	var created db.Item

	err := s.tx.InTx(ctx, func(q *db.Queries) error {
		row, err := insertItem(ctx, q, principal, familyID, newItem{params: params, tagIDs: tagIDs})
		created = row

		return err
	})

	return created, err
}

// newItem is one validated row ready to insert: its parameters, its tags,
// and anything extra the created event should record.
type newItem struct {
	params *db.CreateItemParams
	tagIDs []uuid.UUID
	extra  map[string]any
}

// insertItem writes one item, its tags and its created event through q, so
// that single and batch creation share the same audit trail and can run
// inside one transaction.
func insertItem(ctx context.Context, q *db.Queries, principal auth.Principal, familyID pgtype.UUID, n newItem) (db.Item, error) {
	row, err := q.CreateItem(ctx, *n.params)
	if err != nil {
		return db.Item{}, fmt.Errorf("create item: %w", err)
	}

	for _, tagID := range n.tagIDs {
		if err := q.AttachItemTag(ctx, db.AttachItemTagParams{ItemID: row.ID, TagID: pgconv.UUID(tagID), FamilyID: familyID}); err != nil {
			return db.Item{}, fmt.Errorf("attach tag: %w", err)
		}
	}

	payload := createdPayload(&row)
	for k, v := range n.extra {
		payload[k] = v
	}

	if _, err := q.CreateItemEvent(ctx, db.CreateItemEventParams{
		FamilyID: familyID, ItemID: row.ID, ActorUserID: pgconv.UUID(principal.UserID),
		Type: eventCreated, Payload: pgconv.JSONB(payload),
	}); err != nil {
		return db.Item{}, fmt.Errorf("create item event: %w", err)
	}

	return row, nil
}

// --- update ------------------------------------------------------------

// UpdateItem changes everything about an item except its place and status.
func (s *Service) UpdateItem(
	ctx context.Context, principal auth.Principal, itemID uuid.UUID, body api.UpdateItemRequest,
) (api.Item, error) {
	familyID := pgconv.UUID(principal.FamilyID)

	current, err := s.store.GetItem(ctx, db.GetItemParams{ID: pgconv.UUID(itemID), FamilyID: familyID})
	if wrapped := wrapNotFound(err, "item"); wrapped != nil {
		return api.Item{}, wrapped
	}

	if err := validateUpdateBasics(body); err != nil {
		return api.Item{}, err
	}

	categoryID := current.CategoryID

	if body.CategoryId != nil {
		category, err := s.store.GetCategory(ctx, db.GetCategoryParams{ID: pgconv.UUID(*body.CategoryId), FamilyID: familyID})
		if wrapped := wrapNotFound(err, "category"); wrapped != nil {
			return api.Item{}, wrapped
		}

		categoryID = category.ID
	}

	if body.ForPersonId != nil {
		_, err := s.store.GetPerson(ctx, db.GetPersonParams{ID: pgconv.UUID(*body.ForPersonId), FamilyID: familyID})
		if wrapped := wrapNotFound(err, "person"); wrapped != nil {
			return api.Item{}, wrapped
		}
	}

	traitsParam, err := s.resolveUpdateTraits(ctx, familyID, categoryID, body, &current)
	if err != nil {
		return api.Item{}, err
	}

	params := buildUpdateItemParams(itemID, familyID, body, traitsParam)
	changed := changedFields(body)

	updated, err := s.updateItemTx(ctx, principal, familyID, &params, changed)
	if err != nil {
		return api.Item{}, err
	}

	return s.toAPIItem(ctx, principal.FamilyID, &updated)
}

func validateUpdateBasics(body api.UpdateItemRequest) error {
	if body.Name != nil {
		if _, problem := validName(*body.Name); problem != "" {
			return &ValidationError{Msg: problem}
		}
	}

	switch {
	case body.Quantity != nil && *body.Quantity < 0:
		return &ValidationError{Msg: "quantity must not be negative"}
	case body.MinQuantity != nil && *body.MinQuantity < 0:
		return &ValidationError{Msg: "minQuantity must not be negative"}
	case body.PurchasePrice != nil && *body.PurchasePrice < 0:
		return &ValidationError{Msg: "purchasePrice must not be negative"}
	}

	return nil
}

// resolveUpdateTraits re-validates traits only when the request could
// invalidate the current set: a new category, or a new traits map.
func (s *Service) resolveUpdateTraits(
	ctx context.Context, familyID pgtype.UUID, categoryID pgtype.UUID, body api.UpdateItemRequest, current *db.Item,
) ([]byte, error) {
	if body.CategoryId == nil && body.Traits == nil {
		return nil, nil
	}

	traits := pgconv.ToJSONB(current.Traits)
	if body.Traits != nil {
		traits = *body.Traits
	}

	longTail, err := s.validateTraits(ctx, familyID, categoryID, traits, hotFieldsFromUpdate(body, current))
	if err != nil {
		return nil, err
	}

	return pgconv.JSONB(longTail), nil
}

func buildUpdateItemParams(itemID uuid.UUID, familyID pgtype.UUID, body api.UpdateItemRequest, traitsParam []byte) db.UpdateItemParams {
	return db.UpdateItemParams{
		ID: pgconv.UUID(itemID), FamilyID: familyID,
		Name:             pgconv.TextPtr(body.Name),
		CategoryID:       pgconv.UUIDPtr(body.CategoryId),
		Quantity:         pgconv.NumericPtr(float64Ptr(body.Quantity)),
		Unit:             pgconv.TextPtr(body.Unit),
		MinQuantity:      pgconv.NumericPtr(float64Ptr(body.MinQuantity)),
		ExpiresAt:        pgconv.Date(dateToTimePtr(body.ExpiresAt)),
		OpenedAt:         pgconv.Date(dateToTimePtr(body.OpenedAt)),
		PaoMonths:        pgconv.Int4Ptr(body.PaoMonths),
		Brand:            pgconv.TextPtr(body.Brand),
		Condition:        nullCondition(body.Condition),
		Season:           nullSeason(body.Season),
		SizeSystem:       nullSizeSystem(body.SizeSystem),
		SizeValue:        pgconv.TextPtr(body.SizeValue),
		ForPersonID:      pgconv.UUIDPtr(body.ForPersonId),
		SerialNumber:     pgconv.TextPtr(body.SerialNumber),
		WarrantyUntil:    pgconv.Date(dateToTimePtr(body.WarrantyUntil)),
		PurchaseDate:     pgconv.Date(dateToTimePtr(body.PurchaseDate)),
		PurchasePrice:    pgconv.NumericPtr(float64Ptr(body.PurchasePrice)),
		PurchaseCurrency: pgconv.TextPtr(body.PurchaseCurrency),
		Notes:            pgconv.TextPtr(body.Notes),
		Traits:           traitsParam,
	}
}

func (s *Service) updateItemTx(
	ctx context.Context, principal auth.Principal, familyID pgtype.UUID, params *db.UpdateItemParams, changed []string,
) (db.Item, error) {
	var updated db.Item

	err := s.tx.InTx(ctx, func(q *db.Queries) error {
		row, err := q.UpdateItem(ctx, *params)
		if wrapped := wrapNotFound(err, "item"); wrapped != nil {
			return wrapped
		}

		updated = row

		if len(changed) == 0 {
			return nil
		}

		if _, err := q.CreateItemEvent(ctx, db.CreateItemEventParams{
			FamilyID: familyID, ItemID: row.ID, ActorUserID: pgconv.UUID(principal.UserID),
			Type: eventUpdated, Payload: pgconv.JSONB(updatedPayload(changed)),
		}); err != nil {
			return fmt.Errorf("create item event: %w", err)
		}

		return nil
	})

	return updated, err
}

// --- move --------------------------------------------------------------

// MoveItem moves an item to a place or into a container, with its contents.
//
//nolint:dupl // only the generated request/response shapes differ from UpdateItem/SetItemStatus/UpdateTag
func (s *Service) MoveItem(
	ctx context.Context, principal auth.Principal, itemID uuid.UUID, body api.MoveItemRequest,
) (api.Item, error) {
	familyID := pgconv.UUID(principal.FamilyID)

	current, err := s.store.GetItem(ctx, db.GetItemParams{ID: pgconv.UUID(itemID), FamilyID: familyID})
	if wrapped := wrapNotFound(err, "item"); wrapped != nil {
		return api.Item{}, wrapped
	}

	if (body.LocationId != nil) == (body.ParentItemId != nil) {
		return api.Item{}, &ValidationError{Msg: "exactly one of locationId or parentItemId must be given"}
	}

	params, err := s.resolveMoveParams(ctx, familyID, itemID, &current, body)
	if err != nil {
		return api.Item{}, err
	}

	moved, err := s.moveItemTx(ctx, principal, familyID, params, &current)
	if err != nil {
		return api.Item{}, err
	}

	return s.toAPIItem(ctx, principal.FamilyID, &moved)
}

func (s *Service) resolveMoveParams(
	ctx context.Context, familyID pgtype.UUID, itemID uuid.UUID, current *db.Item, body api.MoveItemRequest,
) (db.MoveItemParams, error) {
	params := db.MoveItemParams{ID: pgconv.UUID(itemID), FamilyID: familyID}

	if body.LocationId != nil {
		return s.resolveMoveToLocation(ctx, familyID, params, *body.LocationId, body.RoomId, body.Spot)
	}

	return s.resolveMoveToParent(ctx, familyID, itemID, current, params, *body.ParentItemId)
}

func (s *Service) resolveMoveToLocation(
	ctx context.Context, familyID pgtype.UUID, params db.MoveItemParams,
	locationID uuid.UUID, roomID *uuid.UUID, spot *string,
) (db.MoveItemParams, error) {
	location, err := s.store.GetLocation(ctx, db.GetLocationParams{ID: pgconv.UUID(locationID), FamilyID: familyID})
	if wrapped := wrapNotFound(err, "location"); wrapped != nil {
		return params, wrapped
	}

	params.LocationID = location.ID

	if roomID != nil {
		room, err := s.store.GetRoom(ctx, db.GetRoomParams{ID: pgconv.UUID(*roomID), FamilyID: familyID})
		if wrapped := wrapNotFound(err, "room"); wrapped != nil {
			return params, wrapped
		}

		if pgconv.ToUUID(room.LocationID) != locationID {
			return params, &ValidationError{Msg: "that room is not in the given location"}
		}

		params.RoomID = room.ID
	}

	params.Spot = pgconv.TextPtr(spot)
	// ParentItemID is left zero-valued (Valid: false): this clears any
	// existing parent, making the item a root.

	return params, nil
}

func (s *Service) resolveMoveToParent(
	ctx context.Context, familyID pgtype.UUID, itemID uuid.UUID, current *db.Item,
	params db.MoveItemParams, parentItemID uuid.UUID,
) (db.MoveItemParams, error) {
	parent, err := s.store.GetItem(ctx, db.GetItemParams{ID: pgconv.UUID(parentItemID), FamilyID: familyID})
	if wrapped := wrapNotFound(err, "item"); wrapped != nil {
		return params, wrapped
	}

	if !parent.IsContainer {
		return params, &ValidationError{Msg: "the target item is not a container"}
	}

	chain, err := s.store.GetItemAncestorChain(ctx, db.GetItemAncestorChainParams{ItemID: parent.ID, FamilyID: familyID})
	if err != nil {
		return params, fmt.Errorf("get item ancestor chain: %w", err)
	}

	if chainContains(chain, itemID) {
		return params, &ValidationError{Msg: "would create a containment cycle"}
	}

	descendants, err := s.store.GetItemDescendantDepths(ctx, db.GetItemDescendantDepthsParams{ItemID: pgconv.UUID(itemID), FamilyID: familyID})
	if err != nil {
		return params, fmt.Errorf("get item descendant depths: %w", err)
	}

	if newChildDepth(chain)+maxRelativeDepth(descendants) > maxDepth {
		return params, &ValidationError{Msg: "would exceed the maximum container depth"}
	}

	// Place columns stay whatever they were — cosmetic once nested (SPEC
	// 4, "Containment") — only parent_item_id actually changes.
	params.LocationID, params.RoomID, params.Spot, params.ParentItemID = current.LocationID, current.RoomID, current.Spot, parent.ID

	return params, nil
}

func (s *Service) moveItemTx(
	ctx context.Context, principal auth.Principal, familyID pgtype.UUID, params db.MoveItemParams, before *db.Item,
) (db.Item, error) {
	var moved db.Item

	err := s.tx.InTx(ctx, func(q *db.Queries) error {
		row, err := q.MoveItem(ctx, params)
		if wrapped := wrapNotFound(err, "item"); wrapped != nil {
			return wrapped
		}

		moved = row

		if _, err := q.CreateItemEvent(ctx, db.CreateItemEventParams{
			FamilyID: familyID, ItemID: row.ID, ActorUserID: pgconv.UUID(principal.UserID),
			Type: eventMoved, Payload: pgconv.JSONB(movedPayload(before, &row)),
		}); err != nil {
			return fmt.Errorf("create item event: %w", err)
		}

		return nil
	})

	return moved, err
}

// --- status and archive ---------------------------------------------------

// SetStatus changes an item's lifecycle status, archiving it once a
// terminal status is reached.
//
//nolint:dupl // only the generated request/response shapes differ from UpdateItem/MoveItem/UpdateTag
func (s *Service) SetStatus(
	ctx context.Context, principal auth.Principal, itemID uuid.UUID, body api.SetItemStatusRequest,
) (api.Item, error) {
	familyID := pgconv.UUID(principal.FamilyID)

	current, err := s.store.GetItem(ctx, db.GetItemParams{ID: pgconv.UUID(itemID), FamilyID: familyID})
	if wrapped := wrapNotFound(err, "item"); wrapped != nil {
		return api.Item{}, wrapped
	}

	from := api.ItemStatus(current.Status)
	if !CanTransition(from, body.Status) {
		return api.Item{}, &ValidationError{Msg: "illegal status transition"}
	}

	var archivedAt pgtype.Timestamptz
	if IsTerminal(body.Status) {
		archivedAt = pgconv.Timestamptz(time.Now())
	}

	updated, err := s.setStatusTx(ctx, principal, familyID, itemID, from, body.Status, archivedAt)
	if err != nil {
		return api.Item{}, err
	}

	return s.toAPIItem(ctx, principal.FamilyID, &updated)
}

func (s *Service) setStatusTx(
	ctx context.Context, principal auth.Principal, familyID pgtype.UUID, itemID uuid.UUID,
	from, to api.ItemStatus, archivedAt pgtype.Timestamptz,
) (db.Item, error) {
	var updated db.Item

	err := s.tx.InTx(ctx, func(q *db.Queries) error {
		row, err := q.SetItemStatus(ctx, db.SetItemStatusParams{
			Status: db.ItemStatus(to), ArchivedAt: archivedAt, ID: pgconv.UUID(itemID), FamilyID: familyID,
		})
		if wrapped := wrapNotFound(err, "item"); wrapped != nil {
			return wrapped
		}

		updated = row

		if _, err := q.CreateItemEvent(ctx, db.CreateItemEventParams{
			FamilyID: familyID, ItemID: row.ID, ActorUserID: pgconv.UUID(principal.UserID),
			Type: eventStatusChanged, Payload: pgconv.JSONB(statusChangedPayload(from, to)),
		}); err != nil {
			return fmt.Errorf("create item event: %w", err)
		}

		return nil
	})

	return updated, err
}

// Archive retires an item outright, regardless of its current status.
func (s *Service) Archive(ctx context.Context, principal auth.Principal, itemID uuid.UUID) error {
	familyID := pgconv.UUID(principal.FamilyID)

	return s.tx.InTx(ctx, func(q *db.Queries) error {
		affected, err := q.ArchiveItem(ctx, db.ArchiveItemParams{ID: pgconv.UUID(itemID), FamilyID: familyID})
		if err != nil {
			return fmt.Errorf("archive item: %w", err)
		}

		if affected == 0 {
			return &NotFoundError{What: "item"}
		}

		if _, err := q.CreateItemEvent(ctx, db.CreateItemEventParams{
			FamilyID: familyID, ItemID: pgconv.UUID(itemID), ActorUserID: pgconv.UUID(principal.UserID),
			Type: eventArchived, Payload: pgconv.JSONB(map[string]any{}),
		}); err != nil {
			return fmt.Errorf("create item event: %w", err)
		}

		return nil
	})
}

// --- tags --------------------------------------------------------------

// ListTags returns the family's tags, alphabetically.
func (s *Service) ListTags(ctx context.Context, principal auth.Principal) ([]api.Tag, error) {
	rows, err := s.store.ListTags(ctx, pgconv.UUID(principal.FamilyID))
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}

	out := make([]api.Tag, 0, len(rows))
	for _, row := range rows {
		out = append(out, apimap.Tag(row))
	}

	return out, nil
}

// ListItemTags returns the tags attached to one item.
func (s *Service) ListItemTags(ctx context.Context, principal auth.Principal, itemID uuid.UUID) ([]api.Tag, error) {
	familyID := pgconv.UUID(principal.FamilyID)

	_, err := s.store.GetItem(ctx, db.GetItemParams{ID: pgconv.UUID(itemID), FamilyID: familyID})
	if wrapped := wrapNotFound(err, "item"); wrapped != nil {
		return nil, wrapped
	}

	rows, err := s.store.ListTagsForItem(ctx, db.ListTagsForItemParams{ItemID: pgconv.UUID(itemID), FamilyID: familyID})
	if err != nil {
		return nil, fmt.Errorf("list tags for item: %w", err)
	}

	out := make([]api.Tag, 0, len(rows))
	for _, row := range rows {
		out = append(out, apimap.Tag(row))
	}

	return out, nil
}

// CreateTag adds a family-defined label.
func (s *Service) CreateTag(ctx context.Context, principal auth.Principal, body api.CreateTagRequest) (api.Tag, error) {
	name, problem := validTagName(body.Name)
	if problem != "" {
		return api.Tag{}, &ValidationError{Msg: problem}
	}

	row, err := s.store.CreateTag(ctx, db.CreateTagParams{
		FamilyID: pgconv.UUID(principal.FamilyID), Name: name, Color: pgconv.TextPtr(body.Color),
	})

	switch {
	case isUniqueViolation(err):
		return api.Tag{}, &ValidationError{Msg: "a tag with that name already exists"}
	case err != nil:
		return api.Tag{}, fmt.Errorf("create tag: %w", err)
	}

	return apimap.Tag(row), nil
}

// UpdateTag renames or recolors a tag.
//
//nolint:dupl // only the generated request/response shapes differ from UpdateItem/MoveItem/SetItemStatus
func (s *Service) UpdateTag(
	ctx context.Context, principal auth.Principal, tagID uuid.UUID, body api.UpdateTagRequest,
) (api.Tag, error) {
	if body.Name != nil {
		if _, problem := validTagName(*body.Name); problem != "" {
			return api.Tag{}, &ValidationError{Msg: problem}
		}
	}

	row, err := s.store.UpdateTag(ctx, db.UpdateTagParams{
		Name: pgconv.TextPtr(body.Name), Color: pgconv.TextPtr(body.Color),
		ID: pgconv.UUID(tagID), FamilyID: pgconv.UUID(principal.FamilyID),
	})

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return api.Tag{}, &NotFoundError{What: "tag"}
	case isUniqueViolation(err):
		return api.Tag{}, &ValidationError{Msg: "a tag with that name already exists"}
	case err != nil:
		return api.Tag{}, fmt.Errorf("update tag: %w", err)
	}

	return apimap.Tag(row), nil
}

// DeleteTag removes a tag outright — a hard delete, since tags carry no
// history to preserve (00008_items.sql).
func (s *Service) DeleteTag(ctx context.Context, principal auth.Principal, tagID uuid.UUID) error {
	affected, err := s.store.DeleteTag(ctx, db.DeleteTagParams{ID: pgconv.UUID(tagID), FamilyID: pgconv.UUID(principal.FamilyID)})
	if err != nil {
		return fmt.Errorf("delete tag: %w", err)
	}

	if affected == 0 {
		return &NotFoundError{What: "tag"}
	}

	return nil
}

// AttachTag attaches an existing tag to an item.
func (s *Service) AttachTag(ctx context.Context, principal auth.Principal, itemID, tagID uuid.UUID) error {
	familyID := pgconv.UUID(principal.FamilyID)

	_, err := s.store.GetItem(ctx, db.GetItemParams{ID: pgconv.UUID(itemID), FamilyID: familyID})
	if wrapped := wrapNotFound(err, "item"); wrapped != nil {
		return wrapped
	}

	_, err = s.store.GetTag(ctx, db.GetTagParams{ID: pgconv.UUID(tagID), FamilyID: familyID})
	if wrapped := wrapNotFound(err, "tag"); wrapped != nil {
		return wrapped
	}

	if err := s.store.AttachItemTag(ctx, db.AttachItemTagParams{ItemID: pgconv.UUID(itemID), TagID: pgconv.UUID(tagID), FamilyID: familyID}); err != nil {
		return fmt.Errorf("attach item tag: %w", err)
	}

	return nil
}

// DetachTag removes a tag from an item. Detaching a tag that was never
// attached is not an error — the end state is the same either way.
func (s *Service) DetachTag(ctx context.Context, principal auth.Principal, itemID, tagID uuid.UUID) error {
	familyID := pgconv.UUID(principal.FamilyID)

	_, err := s.store.GetItem(ctx, db.GetItemParams{ID: pgconv.UUID(itemID), FamilyID: familyID})
	if wrapped := wrapNotFound(err, "item"); wrapped != nil {
		return wrapped
	}

	if _, err := s.store.DetachItemTag(ctx, db.DetachItemTagParams{ItemID: pgconv.UUID(itemID), TagID: pgconv.UUID(tagID), FamilyID: familyID}); err != nil {
		return fmt.Errorf("detach item tag: %w", err)
	}

	return nil
}

// --- shared helpers ---------------------------------------------------------

// validateTraits resolves a category's trait_defs and validates a request's
// long-tail traits map against them.
func (s *Service) validateTraits(
	ctx context.Context, familyID, categoryID pgtype.UUID, traits map[string]any, hotFieldsPresent map[string]bool,
) (map[string]any, error) {
	defs, err := s.store.ListTraitDefsForCategories(ctx, db.ListTraitDefsForCategoriesParams{
		CategoryIds: []pgtype.UUID{categoryID}, FamilyID: familyID, IncludeArchived: false,
	})
	if err != nil {
		return nil, fmt.Errorf("list trait defs: %w", err)
	}

	longTail, problem := validateLongTailTraits(defs, traits, hotFieldsPresent)
	if problem != "" {
		return nil, &ValidationError{Msg: problem}
	}

	return longTail, nil
}

// checkTagIDs confirms every requested tag id belongs to this family,
// before any of them reach a transaction.
func (s *Service) checkTagIDs(ctx context.Context, familyID pgtype.UUID, ids *[]openapi_types.UUID) ([]uuid.UUID, error) {
	if ids == nil || len(*ids) == 0 {
		return nil, nil
	}

	existing, err := s.store.ListTags(ctx, familyID)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}

	known := make(map[uuid.UUID]struct{}, len(existing))
	for _, tag := range existing {
		known[pgconv.ToUUID(tag.ID)] = struct{}{}
	}

	out := make([]uuid.UUID, 0, len(*ids))

	for _, id := range *ids {
		if _, ok := known[id]; !ok {
			return nil, &NotFoundError{What: "tag"}
		}

		out = append(out, id)
	}

	return out, nil
}

// hotFieldsFromCreate reports, for each hot-column trait key, whether a
// create request supplies it through its own dedicated field.
func hotFieldsFromCreate(body api.CreateItemRequest) map[string]bool {
	return map[string]bool{
		"size":           body.SizeValue != nil,
		"size_system":    body.SizeSystem != nil,
		"season":         body.Season != nil,
		"condition":      body.Condition != nil,
		"for_person":     body.ForPersonId != nil,
		"expires_at":     body.ExpiresAt != nil,
		"opened_at":      body.OpenedAt != nil,
		"pao_months":     body.PaoMonths != nil,
		"quantity":       true,
		"unit":           body.Unit != nil,
		"min_quantity":   body.MinQuantity != nil,
		"serial_number":  body.SerialNumber != nil,
		"warranty_until": body.WarrantyUntil != nil,
	}
}

// hotFieldsFromUpdate is hotFieldsFromCreate's update-side counterpart: a
// field already set on the stored row counts as present even when this
// particular request does not touch it (see present()).
func hotFieldsFromUpdate(body api.UpdateItemRequest, current *db.Item) map[string]bool {
	return map[string]bool{
		"size":           present(body.SizeValue, current.SizeValue.Valid),
		"size_system":    present(body.SizeSystem, current.SizeSystem.Valid),
		"season":         present(body.Season, current.Season.Valid),
		"condition":      present(body.Condition, current.Condition.Valid),
		"for_person":     present(body.ForPersonId, current.ForPersonID.Valid),
		"expires_at":     present(body.ExpiresAt, current.ExpiresAt.Valid),
		"opened_at":      present(body.OpenedAt, current.OpenedAt.Valid),
		"pao_months":     present(body.PaoMonths, current.PaoMonths.Valid),
		"quantity":       true,
		"unit":           present(body.Unit, current.Unit.Valid),
		"min_quantity":   present(body.MinQuantity, current.MinQuantity.Valid),
		"serial_number":  present(body.SerialNumber, current.SerialNumber.Valid),
		"warranty_until": present(body.WarrantyUntil, current.WarrantyUntil.Valid),
	}
}

// changedFields lists which of UpdateItemRequest's fields this request
// actually set, for the updated event's payload.
func changedFields(body api.UpdateItemRequest) []string {
	type field struct {
		name string
		set  bool
	}

	fields := []field{
		{"name", body.Name != nil},
		{"categoryId", body.CategoryId != nil},
		{"quantity", body.Quantity != nil},
		{"unit", body.Unit != nil},
		{"minQuantity", body.MinQuantity != nil},
		{"expiresAt", body.ExpiresAt != nil},
		{"openedAt", body.OpenedAt != nil},
		{"paoMonths", body.PaoMonths != nil},
		{"brand", body.Brand != nil},
		{"condition", body.Condition != nil},
		{"season", body.Season != nil},
		{"sizeSystem", body.SizeSystem != nil},
		{"sizeValue", body.SizeValue != nil},
		{"forPersonId", body.ForPersonId != nil},
		{"serialNumber", body.SerialNumber != nil},
		{"warrantyUntil", body.WarrantyUntil != nil},
		{"purchaseDate", body.PurchaseDate != nil},
		{"purchasePrice", body.PurchasePrice != nil},
		{"purchaseCurrency", body.PurchaseCurrency != nil},
		{"notes", body.Notes != nil},
		{"traits", body.Traits != nil},
	}

	out := make([]string, 0, len(fields))

	for _, f := range fields {
		if f.set {
			out = append(out, f.name)
		}
	}

	return out
}

func (s *Service) effectivePlace(ctx context.Context, familyID uuid.UUID, it *db.Item) (locationID uuid.UUID, roomID *uuid.UUID, spot *string, err error) {
	if !it.ParentItemID.Valid {
		return pgconv.ToUUID(it.LocationID), pgconv.ToUUIDPtr(it.RoomID), pgconv.ToTextPtr(it.Spot), nil
	}

	chain, err := s.store.GetItemAncestorChain(ctx, db.GetItemAncestorChainParams{ItemID: it.ID, FamilyID: pgconv.UUID(familyID)})
	if err != nil {
		return uuid.Nil, nil, nil, fmt.Errorf("get item ancestor chain: %w", err)
	}

	if len(chain) == 0 {
		return uuid.Nil, nil, nil, fmt.Errorf("item %s has no ancestor chain", pgconv.ToUUID(it.ID))
	}

	loc, room, spot := rootPlace(chain)

	return loc, room, spot, nil
}

func (s *Service) toAPIItem(ctx context.Context, familyID uuid.UUID, it *db.Item) (api.Item, error) {
	loc, room, spot, err := s.effectivePlace(ctx, familyID, it)
	if err != nil {
		return api.Item{}, err
	}

	return apimap.Item(it, loc, room, spot), nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError

	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func validName(raw string) (name, problem string) {
	name = strings.TrimSpace(raw)
	if name == "" || len([]rune(name)) > maxNameLen {
		return "", "a name must be between 1 and 120 characters"
	}

	return name, ""
}

func validTagName(raw string) (name, problem string) {
	name = strings.TrimSpace(raw)
	if name == "" || len([]rune(name)) > maxTagNameLen {
		return "", "a tag name must be between 1 and 60 characters"
	}

	return name, ""
}

func pageBounds(limit, offset *int32) (resultLimit, resultOffset int32) {
	const (
		defaultLimit = 50
		maxLimit     = 200
	)

	resultLimit = defaultLimit
	if limit != nil {
		resultLimit = *limit
	}

	if resultLimit <= 0 || resultLimit > maxLimit {
		resultLimit = defaultLimit
	}

	if offset != nil && *offset > 0 {
		resultOffset = *offset
	}

	return resultLimit, resultOffset
}

func float64Ptr(f *float32) *float64 {
	if f == nil {
		return nil
	}

	v := float64(*f)

	return &v
}

func dateToTimePtr(d *openapi_types.Date) *time.Time {
	if d == nil {
		return nil
	}

	t := d.Time

	return &t
}

func nullCondition(c *api.ItemCondition) db.NullItemCondition {
	if c == nil {
		return db.NullItemCondition{}
	}

	return db.NullItemCondition{ItemCondition: db.ItemCondition(*c), Valid: true}
}

func nullSeason(s *api.ItemSeason) db.NullItemSeason {
	if s == nil {
		return db.NullItemSeason{}
	}

	return db.NullItemSeason{ItemSeason: db.ItemSeason(*s), Valid: true}
}

func nullSizeSystem(s *api.SizeSystem) db.NullSizeSystem {
	if s == nil {
		return db.NullSizeSystem{}
	}

	return db.NullSizeSystem{SizeSystem: db.SizeSystem(*s), Valid: true}
}
