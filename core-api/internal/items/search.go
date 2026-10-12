package items

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/olehmushka/mykomora/core-api/internal/api"
	"github.com/olehmushka/mykomora/core-api/internal/apimap"
	"github.com/olehmushka/mykomora/core-api/internal/auth"
	"github.com/olehmushka/mykomora/core-api/internal/db"
	"github.com/olehmushka/mykomora/core-api/internal/pgconv"
)

// errNoPlacement means a hit had no ancestor chain, which cannot happen for
// an item the same family just matched.
var errNoPlacement = errors.New("item has no placement")

const (
	sortRelevance = "relevance"
	sortCreated   = "created"
	sortName      = "name"
	sortExpires   = "expires"
	sortPrice     = "price"

	minSimilarNameLen = 2
	recentEventCount  = 10
)

// ListItems browses and searches the family's items (SPEC 6, "Search").
//
// A bare browse (no words, no filter beyond place) lists root items only: a
// nested item's own place is cosmetic and it is reached through its
// container. Any narrower question — words, a status, a tag — is asked of
// every item, nested or not, because the boot in the box is what the person
// is looking for.
func (s *Service) ListItems(ctx context.Context, principal auth.Principal, params api.ListItemsParams) (api.ItemPage, error) {
	args, sort, limit, err := s.buildSearch(ctx, principal, &params)
	if err != nil {
		return api.ItemPage{}, err
	}

	rows, err := s.store.SearchItems(ctx, args)
	if err != nil {
		return api.ItemPage{}, fmt.Errorf("search items: %w", err)
	}

	page := api.ItemPage{Items: make([]api.ItemHit, 0, len(rows))}
	if len(rows) == 0 {
		return page, nil
	}

	placements, err := s.placements(ctx, args.FamilyID, rows)
	if err != nil {
		return api.ItemPage{}, err
	}

	for i := range rows {
		hit, err := s.toHit(ctx, &rows[i], placements)
		if err != nil {
			return api.ItemPage{}, err
		}

		page.Items = append(page.Items, hit)
	}

	if int32(len(rows)) == limit { //nolint:gosec // len(rows) <= limit <= maxPageLimit
		last := rows[len(rows)-1]
		next := encodeCursor(cursor{Sort: sort, Key: last.SortKey, ID: pgconv.ToUUID(last.Item.ID)})
		page.NextCursor = &next
	}

	return page, nil
}

// buildSearch validates the request and turns it into query arguments. It
// also returns the effective sort and page size, which the caller needs to
// mint the next cursor.
func (s *Service) buildSearch(
	ctx context.Context, principal auth.Principal, params *api.ListItemsParams,
) (args db.SearchItemsParams, sort string, limit int32, err error) {
	var zero db.SearchItemsParams

	query := parseSearchQuery(deref(params.Q))

	sort, err = resolveSort(params.Sort, query)
	if err != nil {
		return zero, "", 0, err
	}

	limit, _ = pageBounds(params.Limit, nil)

	args = db.SearchItemsParams{
		FamilyID:        pgconv.UUID(principal.FamilyID),
		IncludeArchived: params.IncludeArchived != nil && *params.IncludeArchived,
		LocationID:      pgconv.UUIDPtr(params.LocationId),
		RoomID:          pgconv.UUIDPtr(params.RoomId),
		CategoryID:      pgconv.UUIDPtr(params.CategoryId),
		ParentItemID:    pgconv.UUIDPtr(params.ContainerId),
		ForPersonID:     pgconv.UUIDPtr(params.PersonId),
		TagID:           pgconv.UUIDPtr(params.TagId),
		ExpiresFrom:     pgconv.Date(dateToTimePtr(params.ExpiresFrom)),
		ExpiresTo:       pgconv.Date(dateToTimePtr(params.ExpiresTo)),
		WarrantyFrom:    pgconv.Date(dateToTimePtr(params.WarrantyFrom)),
		WarrantyTo:      pgconv.Date(dateToTimePtr(params.WarrantyTo)),
		PriceMin:        pgconv.NumericPtr(params.PriceMin),
		PriceMax:        pgconv.NumericPtr(params.PriceMax),
		SortMode:        sort,
		ResultLimit:     limit,
	}

	if params.Status != nil {
		args.Status = db.NullItemStatus{ItemStatus: db.ItemStatus(*params.Status), Valid: true}
	}

	if params.Season != nil {
		args.Season = db.NullItemSeason{ItemSeason: db.ItemSeason(*params.Season), Valid: true}
	}

	if params.Condition != nil {
		args.Condition = db.NullItemCondition{ItemCondition: db.ItemCondition(*params.Condition), Valid: true}
	}

	if !query.empty() {
		args.QOr = pgtype.Text{String: query.Or, Valid: true}
		args.QPlain = pgtype.Text{String: query.Plain, Valid: true}
		args.QTokens = query.Tokens
		args.QTokenCount = int32(len(query.Tokens)) //nolint:gosec // <= maxQueryTokens
		args.QMinMatch = query.MinMatch
	}

	args.RootsOnly = query.empty() && isPlainBrowse(params)

	args.TraitFilter, err = s.traitFilter(ctx, args.FamilyID, params)
	if err != nil {
		return zero, "", 0, err
	}

	if params.Cursor != nil && *params.Cursor != "" {
		c, cursorErr := decodeCursor(*params.Cursor, sort)
		if cursorErr != nil {
			return zero, "", 0, &ValidationError{Msg: "the cursor is invalid, or belongs to a different sort"}
		}

		args.CursorKey = pgtype.Text{String: c.Key, Valid: true}
		args.CursorID = pgconv.UUID(c.ID)
	}

	return args, sort, limit, nil
}

func resolveSort(requested *api.ListItemsParamsSort, query searchQuery) (string, error) {
	if requested == nil {
		if query.empty() {
			return sortCreated, nil
		}

		return sortRelevance, nil
	}

	switch sort := string(*requested); sort {
	case sortRelevance:
		if query.empty() {
			return "", &ValidationError{Msg: "sorting by relevance needs a search query"}
		}

		return sort, nil
	case sortCreated, sortName, sortExpires, sortPrice:
		return sort, nil
	default:
		return "", &ValidationError{Msg: "unknown sort \"" + sort + "\""}
	}
}

// isPlainBrowse reports whether the request narrows by place alone, which is
// the only narrowing that keeps the roots-only list.
func isPlainBrowse(p *api.ListItemsParams) bool {
	narrowing := []bool{
		p.ContainerId != nil, p.CategoryId != nil, p.PersonId != nil, p.TagId != nil,
		p.Status != nil, p.Season != nil, p.Condition != nil, p.Traits != nil,
		p.ExpiresFrom != nil, p.ExpiresTo != nil, p.WarrantyFrom != nil, p.WarrantyTo != nil,
		p.PriceMin != nil, p.PriceMax != nil,
	}

	return !slices.Contains(narrowing, true)
}

// traitFilter validates the `traits` parameter against the category's
// filterable trait definitions and returns it as the JSON the query matches
// with `@>`. Traits are category-specific, so one without a category is
// ambiguous and rejected.
func (s *Service) traitFilter(ctx context.Context, familyID pgtype.UUID, p *api.ListItemsParams) ([]byte, error) {
	if p.Traits == nil || strings.TrimSpace(*p.Traits) == "" {
		return nil, nil
	}

	if p.CategoryId == nil {
		return nil, &ValidationError{Msg: "filtering by traits needs a category"}
	}

	var wanted map[string]any
	if err := json.Unmarshal([]byte(*p.Traits), &wanted); err != nil || len(wanted) == 0 {
		return nil, &ValidationError{Msg: "traits must be a non-empty JSON object"}
	}

	defs, err := s.store.ListTraitDefsForCategories(ctx, db.ListTraitDefsForCategoriesParams{
		CategoryIds: []pgtype.UUID{pgconv.UUID(*p.CategoryId)}, FamilyID: familyID, IncludeArchived: false,
	})
	if err != nil {
		return nil, fmt.Errorf("list trait defs: %w", err)
	}

	if err := checkTraitFilter(defs, wanted); err != nil {
		return nil, err
	}

	raw, err := json.Marshal(wanted)
	if err != nil {
		return nil, fmt.Errorf("marshal trait filter: %w", err)
	}

	return raw, nil
}

// checkTraitFilter requires every key to be a filterable, long-tail trait of
// the category and every value to fit that trait's type.
func checkTraitFilter(defs []db.TraitDef, wanted map[string]any) error {
	byKey := make(map[string]db.TraitDef, len(defs))
	for _, def := range defs {
		byKey[def.Key] = def
	}

	for key, value := range wanted {
		def, ok := byKey[key]
		if !ok || !def.Filterable || isHotColumnKey(key) {
			return &ValidationError{Msg: "\"" + key + "\" is not a filterable trait of this category"}
		}

		if problem := coerceTraitValue(def, value); problem != "" {
			return &ValidationError{Msg: problem}
		}
	}

	return nil
}

// placementSet groups ListItemPlacements rows by the item they describe.
type placementSet map[pgtype.UUID][]db.ListItemPlacementsRow

func (s *Service) placements(ctx context.Context, familyID pgtype.UUID, rows []db.SearchItemsRow) (placementSet, error) {
	ids := make([]pgtype.UUID, len(rows))
	for i := range rows {
		ids[i] = rows[i].Item.ID
	}

	chain, err := s.store.ListItemPlacements(ctx, db.ListItemPlacementsParams{FamilyID: familyID, ItemIds: ids})
	if err != nil {
		return nil, fmt.Errorf("list item placements: %w", err)
	}

	out := make(placementSet, len(rows))
	for i := range chain {
		out[chain[i].OriginID] = append(out[chain[i].OriginID], chain[i])
	}

	return out, nil
}

func (s *Service) toHit(ctx context.Context, row *db.SearchItemsRow, placements placementSet) (api.ItemHit, error) {
	chain := placements[row.Item.ID]
	if len(chain) == 0 {
		return api.ItemHit{}, fmt.Errorf("item %s: %w", pgconv.ToUUID(row.Item.ID), errNoPlacement)
	}

	root := chain[len(chain)-1]
	placement := api.ItemPlacement{
		Location:      api.NamedRef{Id: pgconv.ToUUID(root.LocationID), Name: root.LocationName.String},
		Spot:          pgconv.ToTextPtr(root.Spot),
		ContainerPath: make([]api.NamedRef, 0, len(chain)-1),
	}

	var effRoom *uuid.UUID

	if root.RoomID.Valid {
		id := pgconv.ToUUID(root.RoomID)
		effRoom = &id
		placement.Room = &api.NamedRef{Id: id, Name: root.RoomName.String}
	}

	for i := len(chain) - 1; i >= 1; i-- {
		placement.ContainerPath = append(placement.ContainerPath, api.NamedRef{
			Id: pgconv.ToUUID(chain[i].AncestorID), Name: chain[i].AncestorName,
		})
	}

	hit := api.ItemHit{
		Item:      apimap.Item(&row.Item, placement.Location.Id, effRoom, placement.Spot),
		Placement: placement,
	}

	if row.PrimaryThumbKey.Valid {
		url, err := s.storage.PresignGet(ctx, row.PrimaryThumbKey.String)
		if err != nil {
			// A missing thumbnail must not fail the whole page.
			s.log.WarnContext(ctx, "presign thumbnail", "item_id", pgconv.ToUUID(row.Item.ID), "error", err)
		} else {
			hit.ThumbUrl = &url
		}
	}

	return hit, nil
}

// FindSimilar returns up to five items whose name is close to name.
func (s *Service) FindSimilar(ctx context.Context, principal auth.Principal, name string) ([]api.SimilarItem, error) {
	name = strings.TrimSpace(name)
	if len([]rune(name)) < minSimilarNameLen || len([]rune(name)) > maxNameLen {
		return nil, &ValidationError{Msg: "name must be between 2 and 120 characters"}
	}

	rows, err := s.store.FindSimilarItems(ctx, db.FindSimilarItemsParams{FamilyID: pgconv.UUID(principal.FamilyID), Name: name})
	if err != nil {
		return nil, fmt.Errorf("find similar items: %w", err)
	}

	out := make([]api.SimilarItem, 0, len(rows))
	for _, row := range rows {
		out = append(out, api.SimilarItem{
			Id:         pgconv.ToUUID(row.ID),
			Name:       row.Name,
			CategoryId: pgconv.ToUUID(row.CategoryID),
			Status:     api.ItemStatus(row.Status),
			Score:      row.Score,
		})
	}

	return out, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}
