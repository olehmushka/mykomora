package taxonomy

import (
	"encoding/json"

	"github.com/olehmushka/mykomora/core-api/internal/api"
	"github.com/olehmushka/mykomora/core-api/internal/catalogue"
	"github.com/olehmushka/mykomora/core-api/internal/db"
	"github.com/olehmushka/mykomora/core-api/internal/locale"
	"github.com/olehmushka/mykomora/core-api/internal/pgconv"
)

// buildTree nests a flat, family-scoped category list under their parents.
//
// Nesting a few dozen rows in Go is simpler than a recursive query for a
// shape this small; ListCategories' own ORDER BY (parent_id NULLS FIRST,
// sort_order, slug) is enough to keep each parent's children in the right
// relative order without this function sorting anything itself.
func buildTree(categories []db.Category, traitRows []db.TraitDef, cat *catalogue.Catalogue, loc locale.Locale) []api.Category {
	traitsByCategory := make(map[string][]db.TraitDef, len(categories))
	for _, t := range traitRows {
		key := pgconv.ToUUID(t.CategoryID).String()
		traitsByCategory[key] = append(traitsByCategory[key], t)
	}

	rendered := make(map[string]api.Category, len(categories))
	childrenOf := make(map[string][]string, len(categories))

	for _, c := range categories {
		id := pgconv.ToUUID(c.ID).String()
		rendered[id] = renderCategory(c, traitsByCategory[id], cat, loc)

		parentKey := ""
		if parentID := pgconv.ToUUIDPtr(c.ParentID); parentID != nil {
			parentKey = parentID.String()
		}

		childrenOf[parentKey] = append(childrenOf[parentKey], id)
	}

	var attach func(id string) api.Category

	attach = func(id string) api.Category {
		node := rendered[id]
		for _, childID := range childrenOf[id] {
			node.Children = append(node.Children, attach(childID))
		}

		return node
	}

	top := make([]api.Category, 0, len(childrenOf[""]))
	for _, id := range childrenOf[""] {
		top = append(top, attach(id))
	}

	return top
}

// renderCategory resolves one category's label (built-in via the catalogue,
// family-defined via its own text) and attaches its already-resolved traits.
// Children are left empty here; buildTree fills them for the tree endpoint.
func renderCategory(c db.Category, traits []db.TraitDef, cat *catalogue.Catalogue, loc locale.Locale) api.Category {
	isBuiltin := !c.FamilyID.Valid

	name := c.NameCustom.String
	if isBuiltin {
		name = cat.Text(c.NameI18nKey.String, loc)
	}

	rendered := make([]api.TraitDef, 0, len(traits))
	for _, t := range traits {
		rendered = append(rendered, renderTraitDef(t, cat, loc))
	}

	return api.Category{
		Id:         pgconv.ToUUID(c.ID),
		ParentId:   pgconv.ToUUIDPtr(c.ParentID),
		Slug:       c.Slug,
		Icon:       pgconv.ToTextPtr(c.Icon),
		Name:       name,
		SortOrder:  c.SortOrder,
		IsBuiltin:  isBuiltin,
		ArchivedAt: pgconv.ToTimePtr(c.ArchivedAt),
		Traits:     rendered,
		Children:   []api.Category{},
	}
}

// renderTraitDef resolves one field's label the same way renderCategory
// resolves a category's.
func renderTraitDef(t db.TraitDef, cat *catalogue.Catalogue, loc locale.Locale) api.TraitDef {
	isBuiltin := !t.FamilyID.Valid

	label := t.LabelCustom.String
	if isBuiltin {
		label = cat.Text(t.LabelI18nKey.String, loc)
	}

	return api.TraitDef{
		Id:          pgconv.ToUUID(t.ID),
		CategoryId:  pgconv.ToUUID(t.CategoryID),
		Key:         t.Key,
		DataType:    api.TraitDataType(t.DataType),
		Unit:        pgconv.ToTextPtr(t.Unit),
		EnumOptions: decodeEnumOptions(t.EnumOptions),
		Label:       label,
		Required:    t.Required,
		Filterable:  t.Filterable,
		SortOrder:   t.SortOrder,
		IsBuiltin:   isBuiltin,
		ArchivedAt:  pgconv.ToTimePtr(t.ArchivedAt),
	}
}

// encodeEnumOptions validates and marshals the options an "enum" field needs
// one or more of, and refuses them on any other data type — the enum-shape
// invariant trait_defs' own CHECK constraint enforces, checked earlier so a
// bad request reads as a 400 rather than a constraint-violation 500.
func encodeEnumOptions(dataType db.TraitDataType, options *[]string) (value []byte, problem string) {
	if dataType != db.TraitDataTypeEnum {
		if options != nil && len(*options) > 0 {
			return nil, "only an enum field takes enum options"
		}

		return nil, ""
	}

	if options == nil || len(*options) == 0 {
		return nil, "an enum field needs at least one option"
	}

	encoded, err := json.Marshal(*options)
	if err != nil {
		return nil, "invalid enum options"
	}

	return encoded, ""
}

// decodeEnumOptions is encodeEnumOptions' read-side inverse. A malformed
// column (which nothing in this package writes) is treated as absent rather
// than surfaced as an error a client could not act on.
func decodeEnumOptions(raw []byte) *[]string {
	if len(raw) == 0 {
		return nil
	}

	var options []string
	if err := json.Unmarshal(raw, &options); err != nil {
		return nil
	}

	return &options
}
