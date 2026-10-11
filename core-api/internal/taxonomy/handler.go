// Package taxonomy serves the category tree and its trait schema: the
// built-in catalogue plus each family's own categories and custom fields
// (SPEC 3, "Concepts" and SPEC 4, "Taxonomy").
//
// Built-in rows (family_id NULL) are seeded once, by db/migrations'
// 00006_taxonomy_seed.go, and are read-only here — there is no
// create/update/archive path for them, only for a family's own rows.
package taxonomy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/olehmushka/mykomora/core-api/internal/api"
	"github.com/olehmushka/mykomora/core-api/internal/auth"
	"github.com/olehmushka/mykomora/core-api/internal/catalogue"
	"github.com/olehmushka/mykomora/core-api/internal/db"
	"github.com/olehmushka/mykomora/core-api/internal/locale"
	"github.com/olehmushka/mykomora/core-api/internal/pgconv"
)

const (
	maxSlugLen  = 64
	maxLabelLen = 120
)

var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

// Store is the slice of the generated queries this package needs.
type Store interface {
	ListCategories(ctx context.Context, arg db.ListCategoriesParams) ([]db.Category, error)
	GetCategory(ctx context.Context, arg db.GetCategoryParams) (db.Category, error)
	CreateCategory(ctx context.Context, arg db.CreateCategoryParams) (db.Category, error)
	UpdateCategory(ctx context.Context, arg db.UpdateCategoryParams) (db.Category, error)
	ArchiveCategory(ctx context.Context, arg db.ArchiveCategoryParams) (int64, error)

	ListTraitDefsForCategories(ctx context.Context, arg db.ListTraitDefsForCategoriesParams) ([]db.TraitDef, error)
	GetTraitDef(ctx context.Context, arg db.GetTraitDefParams) (db.TraitDef, error)
	CreateTraitDef(ctx context.Context, arg db.CreateTraitDefParams) (db.TraitDef, error)
	UpdateTraitDef(ctx context.Context, arg db.UpdateTraitDefParams) (db.TraitDef, error)
	ArchiveTraitDef(ctx context.Context, arg db.ArchiveTraitDefParams) (int64, error)
}

// Handler serves the taxonomy operations of the generated contract.
type Handler struct {
	store     Store
	catalogue *catalogue.Catalogue
	resolver  *locale.Resolver
	log       *slog.Logger
}

// NewHandler builds the taxonomy handler.
func NewHandler(store Store, cat *catalogue.Catalogue, resolver *locale.Resolver, log *slog.Logger) *Handler {
	return &Handler{
		store:     store,
		catalogue: cat,
		resolver:  resolver,
		log:       log.With(slog.String("component", "taxonomy")),
	}
}

// GetTaxonomy returns the localised category tree, ETag-cached.
//
// The tree is built here rather than in SQL: categories.sql's ListCategories
// returns a flat family-scoped set (built-in ∪ this family's own), and
// nesting a handful of rows in Go is simpler than a recursive query for a
// shape this small.
func (h *Handler) GetTaxonomy(
	ctx context.Context,
	request api.GetTaxonomyRequestObject,
) (api.GetTaxonomyResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.GetTaxonomy401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	loc := h.resolver.Resolve(ctx, pgconv.UUID(principal.UserID), auth.RequestMetaFrom(ctx).AcceptLanguage)

	categories, err := h.store.ListCategories(ctx, db.ListCategoriesParams{
		FamilyID:        pgconv.UUID(principal.FamilyID),
		IncludeArchived: false,
	})
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}

	ids := make([]pgtype.UUID, 0, len(categories))
	for _, c := range categories {
		ids = append(ids, c.ID)
	}

	traits, err := h.store.ListTraitDefsForCategories(ctx, db.ListTraitDefsForCategoriesParams{
		CategoryIds:     ids,
		FamilyID:        pgconv.UUID(principal.FamilyID),
		IncludeArchived: false,
	})
	if err != nil {
		return nil, fmt.Errorf("list trait defs: %w", err)
	}

	tree := buildTree(categories, traits, h.catalogue, loc)

	body := api.Taxonomy{Categories: tree}

	etag, err := computeETag(loc, body)
	if err != nil {
		return nil, fmt.Errorf("compute taxonomy etag: %w", err)
	}

	if request.Params.IfNoneMatch != nil && *request.Params.IfNoneMatch == etag {
		return taxonomyResponse{etag: etag, notModified: true}, nil
	}

	return taxonomyResponse{etag: etag, body: body}, nil
}

// CreateCategory adds a family-defined category. Built-in categories exist
// only via the seed migration; there is no create path for them.
func (h *Handler) CreateCategory(
	ctx context.Context,
	request api.CreateCategoryRequestObject,
) (api.CreateCategoryResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.CreateCategory401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	if request.Body == nil {
		return api.CreateCategory400JSONResponse{BadRequestJSONResponse: invalidCategory("a category needs a slug and a name")}, nil
	}

	slug, problem := validSlug(request.Body.Slug)
	if problem != "" {
		return api.CreateCategory400JSONResponse{BadRequestJSONResponse: invalidCategory(problem)}, nil
	}

	name, problem := validLabel(request.Body.NameCustom)
	if problem != "" {
		return api.CreateCategory400JSONResponse{BadRequestJSONResponse: invalidCategory(problem)}, nil
	}

	sortOrder := int32(0)
	if request.Body.SortOrder != nil {
		sortOrder = *request.Body.SortOrder
	}

	row, err := h.store.CreateCategory(ctx, db.CreateCategoryParams{
		FamilyID:   pgconv.UUID(principal.FamilyID),
		ParentID:   pgconv.UUIDPtr(request.Body.ParentId),
		Slug:       slug,
		Icon:       pgconv.TextPtr(trimmed(request.Body.Icon)),
		NameCustom: pgconv.Text(name),
		SortOrder:  sortOrder,
	})

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return api.CreateCategory400JSONResponse{BadRequestJSONResponse: invalidCategory("no such parent category in this family")}, nil
	case isUniqueViolation(err):
		return api.CreateCategory400JSONResponse{BadRequestJSONResponse: invalidCategory("a category with this slug already exists")}, nil
	case err != nil:
		return nil, fmt.Errorf("create category: %w", err)
	}

	return api.CreateCategory201JSONResponse(renderCategory(row, nil, h.catalogue, locale.En)), nil
}

// UpdateCategory changes a family-defined category. Omitted fields are left
// alone; a built-in category can never match, because the query always
// filters on family_id.
func (h *Handler) UpdateCategory(
	ctx context.Context,
	request api.UpdateCategoryRequestObject,
) (api.UpdateCategoryResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.UpdateCategory401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	params := db.UpdateCategoryParams{
		ID:       pgconv.UUID(request.CategoryId),
		FamilyID: pgconv.UUID(principal.FamilyID),
	}

	if request.Body != nil {
		if request.Body.NameCustom != nil {
			name, problem := validLabel(*request.Body.NameCustom)
			if problem != "" {
				return api.UpdateCategory400JSONResponse{BadRequestJSONResponse: invalidCategory(problem)}, nil
			}

			params.NameCustom = pgconv.Text(name)
		}

		params.Icon = pgconv.TextPtr(trimmed(request.Body.Icon))

		if request.Body.SortOrder != nil {
			params.SortOrder = pgconv.Int4(*request.Body.SortOrder)
		}
	}

	row, err := h.store.UpdateCategory(ctx, params)

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return api.UpdateCategory404JSONResponse{NotFoundJSONResponse: notFound("category")}, nil
	case err != nil:
		return nil, fmt.Errorf("update category: %w", err)
	}

	return api.UpdateCategory200JSONResponse(renderCategory(row, nil, h.catalogue, locale.En)), nil
}

// ArchiveCategory retires a family-defined category.
func (h *Handler) ArchiveCategory(
	ctx context.Context,
	request api.ArchiveCategoryRequestObject,
) (api.ArchiveCategoryResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.ArchiveCategory401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	affected, err := h.store.ArchiveCategory(ctx, db.ArchiveCategoryParams{
		ID:       pgconv.UUID(request.CategoryId),
		FamilyID: pgconv.UUID(principal.FamilyID),
	})
	if err != nil {
		return nil, fmt.Errorf("archive category: %w", err)
	}

	if affected == 0 {
		// Either no such category in this family, or it exists but is
		// built-in — both report the same way: there is nothing this family
		// is allowed to archive at that id.
		return api.ArchiveCategory404JSONResponse{NotFoundJSONResponse: notFound("category")}, nil
	}

	return api.ArchiveCategory204Response{}, nil
}

// CreateTraitDef adds a custom field to a category — built-in or the
// family's own; SPEC 4's "one code path, one validator, one renderer"
// applies per field, not per category.
func (h *Handler) CreateTraitDef(
	ctx context.Context,
	request api.CreateTraitDefRequestObject,
) (api.CreateTraitDefResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.CreateTraitDef401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	if request.Body == nil {
		return api.CreateTraitDef400JSONResponse{BadRequestJSONResponse: invalidTraitDef("a field needs a key, a type and a label")}, nil
	}

	key, problem := validSlug(request.Body.Key)
	if problem != "" {
		return api.CreateTraitDef400JSONResponse{BadRequestJSONResponse: invalidTraitDef(problem)}, nil
	}

	label, problem := validLabel(request.Body.LabelCustom)
	if problem != "" {
		return api.CreateTraitDef400JSONResponse{BadRequestJSONResponse: invalidTraitDef(problem)}, nil
	}

	enumOptions, problem := encodeEnumOptions(db.TraitDataType(request.Body.DataType), request.Body.EnumOptions)
	if problem != "" {
		return api.CreateTraitDef400JSONResponse{BadRequestJSONResponse: invalidTraitDef(problem)}, nil
	}

	row, err := h.store.CreateTraitDef(ctx, db.CreateTraitDefParams{
		CategoryID:  pgconv.UUID(request.CategoryId),
		FamilyID:    pgconv.UUID(principal.FamilyID),
		Key:         key,
		DataType:    db.TraitDataType(request.Body.DataType),
		Unit:        pgconv.TextPtr(trimmed(request.Body.Unit)),
		EnumOptions: enumOptions,
		LabelCustom: pgconv.Text(label),
		Required:    request.Body.Required != nil && *request.Body.Required,
		Filterable:  request.Body.Filterable != nil && *request.Body.Filterable,
		SortOrder:   sortOrderOf(request.Body.SortOrder),
	})

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return api.CreateTraitDef404JSONResponse{NotFoundJSONResponse: notFound("category")}, nil
	case isUniqueViolation(err):
		return api.CreateTraitDef400JSONResponse{BadRequestJSONResponse: invalidTraitDef("this category already has a field with this key")}, nil
	case err != nil:
		return nil, fmt.Errorf("create trait def: %w", err)
	}

	return api.CreateTraitDef201JSONResponse(renderTraitDef(row, h.catalogue, locale.En)), nil
}

// UpdateTraitDef changes a custom field's label, requiredness, filterability
// or order. Its key and type are fixed at creation.
func (h *Handler) UpdateTraitDef(
	ctx context.Context,
	request api.UpdateTraitDefRequestObject,
) (api.UpdateTraitDefResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.UpdateTraitDef401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	params := db.UpdateTraitDefParams{
		ID:       pgconv.UUID(request.TraitDefId),
		FamilyID: pgconv.UUID(principal.FamilyID),
	}

	if request.Body != nil {
		if request.Body.LabelCustom != nil {
			label, problem := validLabel(*request.Body.LabelCustom)
			if problem != "" {
				return api.UpdateTraitDef400JSONResponse{BadRequestJSONResponse: invalidTraitDef(problem)}, nil
			}

			params.LabelCustom = pgconv.Text(label)
		}

		params.Required = pgconv.Bool(request.Body.Required)
		params.Filterable = pgconv.Bool(request.Body.Filterable)

		if request.Body.SortOrder != nil {
			params.SortOrder = pgconv.Int4(*request.Body.SortOrder)
		}
	}

	row, err := h.store.UpdateTraitDef(ctx, params)

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return api.UpdateTraitDef404JSONResponse{NotFoundJSONResponse: notFound("field")}, nil
	case err != nil:
		return nil, fmt.Errorf("update trait def: %w", err)
	}

	return api.UpdateTraitDef200JSONResponse(renderTraitDef(row, h.catalogue, locale.En)), nil
}

// ArchiveTraitDef retires a custom field.
func (h *Handler) ArchiveTraitDef(
	ctx context.Context,
	request api.ArchiveTraitDefRequestObject,
) (api.ArchiveTraitDefResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.ArchiveTraitDef401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	affected, err := h.store.ArchiveTraitDef(ctx, db.ArchiveTraitDefParams{
		ID:       pgconv.UUID(request.TraitDefId),
		FamilyID: pgconv.UUID(principal.FamilyID),
	})
	if err != nil {
		return nil, fmt.Errorf("archive trait def: %w", err)
	}

	if affected == 0 {
		return api.ArchiveTraitDef404JSONResponse{NotFoundJSONResponse: notFound("field")}, nil
	}

	return api.ArchiveTraitDef204Response{}, nil
}

func sortOrderOf(v *int32) int32 {
	if v == nil {
		return 0
	}

	return *v
}

func validSlug(raw string) (slug, problem string) {
	slug = strings.ToLower(strings.TrimSpace(raw))
	if !slugPattern.MatchString(slug) {
		return "", "a key must be lowercase letters, digits, underscores or hyphens, up to 64 characters"
	}

	return slug, ""
}

func validLabel(raw string) (label, problem string) {
	label = strings.TrimSpace(raw)
	if label == "" || len([]rune(label)) > maxLabelLen {
		return "", "a label must be between 1 and 120 characters"
	}

	return label, ""
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

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError

	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func unauthorized() api.UnauthorizedJSONResponse {
	return api.UnauthorizedJSONResponse{Code: "no_session", Message: "sign in to continue"}
}

func notFound(what string) api.NotFoundJSONResponse {
	return api.NotFoundJSONResponse{Code: "not_found", Message: "no such " + what + " in this family"}
}

func invalidCategory(message string) api.BadRequestJSONResponse {
	return api.BadRequestJSONResponse{Code: "invalid_category", Message: message}
}

func invalidTraitDef(message string) api.BadRequestJSONResponse {
	return api.BadRequestJSONResponse{Code: "invalid_trait_def", Message: message}
}
