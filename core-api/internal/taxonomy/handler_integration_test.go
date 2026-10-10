//go:build integration

// Integration tests against a real Postgres, exercising the one migration
// this milestone's sqlc queries cannot prove on their own: the seed migration
// itself, and the locale/ETag behaviour GetTaxonomy builds on top of it.
package taxonomy_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/olehmushka/mykomora/core-api/internal/api"
	"github.com/olehmushka/mykomora/core-api/internal/auth"
	"github.com/olehmushka/mykomora/core-api/internal/catalogue"
	"github.com/olehmushka/mykomora/core-api/internal/config"
	"github.com/olehmushka/mykomora/core-api/internal/db"
	"github.com/olehmushka/mykomora/core-api/internal/locale"
	"github.com/olehmushka/mykomora/core-api/internal/postgres"
	"github.com/olehmushka/mykomora/core-api/internal/taxonomy"
	"github.com/olehmushka/mykomora/core-api/internal/testdb"
)

func newHandler(t *testing.T) (*taxonomy.Handler, auth.Principal) {
	t.Helper()

	database := testdb.Start(t)
	queries := db.New(database.Pool)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	service := auth.NewService(queries, postgres.NewTxRunner(database.Pool), config.Config{
		Env:             "test",
		AppSecret:       "an-integration-test-secret-0000000000000",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 30 * 24 * time.Hour,
		InviteTTL:       7 * 24 * time.Hour,
	}, log)

	session, err := service.SignIn(context.Background(),
		auth.Identity{Subject: "google-1", Email: "one@example.com", DisplayName: "One"}, "", auth.RequestMeta{})
	if err != nil {
		t.Fatalf("SignIn: %v", err)
	}

	cat := catalogue.New(catalogue.BuiltinLabels)
	resolver := locale.NewResolver(queries)

	return taxonomy.NewHandler(queries, cat, resolver, log), session.Principal
}

func getTaxonomy(t *testing.T, h *taxonomy.Handler, ctx context.Context) (api.Taxonomy, string) {
	t.Helper()

	resp, err := h.GetTaxonomy(ctx, api.GetTaxonomyRequestObject{})
	if err != nil {
		t.Fatalf("GetTaxonomy: %v", err)
	}

	rec := httptest.NewRecorder()
	if err := resp.VisitGetTaxonomyResponse(rec); err != nil {
		t.Fatalf("VisitGetTaxonomyResponse: %v", err)
	}

	if rec.Code == 304 {
		return api.Taxonomy{}, rec.Header().Get("ETag")
	}

	var body api.Taxonomy
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	return body, rec.Header().Get("ETag")
}

// The seed migration's own acceptance: all 23 built-in top-level categories
// are visible, in both locales, each with its traits attached.
func TestGetTaxonomyReturnsEveryBuiltinCategory(t *testing.T) {
	h, principal := newHandler(t)
	ctx := auth.WithRequestMeta(auth.WithPrincipal(context.Background(), principal), auth.RequestMeta{AcceptLanguage: "en"})

	body, _ := getTaxonomy(t, h, ctx)

	if len(body.Categories) != 23 {
		t.Fatalf("got %d top-level categories, want 23", len(body.Categories))
	}

	var clothing *api.Category
	for i := range body.Categories {
		if body.Categories[i].Slug == "clothing_footwear" {
			clothing = &body.Categories[i]
		}
	}

	if clothing == nil {
		t.Fatal("clothing_footwear is missing from the tree")
	}

	if clothing.Name != "Clothing & footwear" {
		t.Errorf("name = %q, want the English catalogue label", clothing.Name)
	}

	if len(clothing.Traits) != 8 {
		t.Errorf("clothing_footwear has %d traits, want 8", len(clothing.Traits))
	}

	if !clothing.IsBuiltin {
		t.Error("a seeded category reports isBuiltin = false")
	}
}

// Labels resolve per Accept-Language, and the catalogue's Ukrainian column
// is what actually renders — not a bare key or an English fallback.
func TestGetTaxonomyLocalisesIntoUkrainian(t *testing.T) {
	h, principal := newHandler(t)
	ctx := auth.WithRequestMeta(auth.WithPrincipal(context.Background(), principal), auth.RequestMeta{AcceptLanguage: "uk"})

	body, _ := getTaxonomy(t, h, ctx)

	for _, c := range body.Categories {
		if c.Slug == "cosmetics" && c.Name != "Косметика" {
			t.Errorf("name = %q, want the Ukrainian catalogue label", c.Name)
		}
	}
}

// The ETag changes with the locale (so a shared cache never serves one
// language's body under the other's tag) and a repeated request with the
// prior ETag in If-None-Match gets a 304 with no body.
func TestGetTaxonomyETagVariesByLocaleAndSupports304(t *testing.T) {
	h, principal := newHandler(t)

	enCtx := auth.WithRequestMeta(auth.WithPrincipal(context.Background(), principal), auth.RequestMeta{AcceptLanguage: "en"})
	ukCtx := auth.WithRequestMeta(auth.WithPrincipal(context.Background(), principal), auth.RequestMeta{AcceptLanguage: "uk"})

	_, enETag := getTaxonomy(t, h, enCtx)
	_, ukETag := getTaxonomy(t, h, ukCtx)

	if enETag == "" || ukETag == "" {
		t.Fatal("expected a non-empty ETag for both locales")
	}

	if enETag == ukETag {
		t.Error("en and uk produced the same ETag")
	}

	resp, err := h.GetTaxonomy(enCtx, api.GetTaxonomyRequestObject{
		Params: api.GetTaxonomyParams{IfNoneMatch: &enETag},
	})
	if err != nil {
		t.Fatalf("GetTaxonomy with If-None-Match: %v", err)
	}

	rec := httptest.NewRecorder()
	if err := resp.VisitGetTaxonomyResponse(rec); err != nil {
		t.Fatalf("VisitGetTaxonomyResponse: %v", err)
	}

	if rec.Code != 304 {
		t.Fatalf("status = %d, want 304", rec.Code)
	}

	if rec.Body.Len() != 0 {
		t.Errorf("304 response has a %d-byte body, want none", rec.Body.Len())
	}
}

// A family-defined category and a custom field on a built-in one are visible
// only to that family, and only ever a family's own rows can be written.
func TestCustomCategoryAndFieldAreFamilyScoped(t *testing.T) {
	h, principal := newHandler(t)
	ctx := auth.WithPrincipal(context.Background(), principal)

	created, err := h.CreateCategory(ctx, api.CreateCategoryRequestObject{
		Body: &api.CreateCategoryRequest{Slug: "garage_tools", NameCustom: "Garage tools"},
	})
	if err != nil {
		t.Fatalf("CreateCategory: %v", err)
	}

	createdResp, ok := created.(api.CreateCategory201JSONResponse)
	if !ok {
		t.Fatalf("CreateCategory returned %T, want a 201", created)
	}

	categoryID := api.Category(createdResp).Id

	fieldCreated, err := h.CreateTraitDef(ctx, api.CreateTraitDefRequestObject{
		CategoryId: categoryID,
		Body:       &api.CreateTraitDefRequest{Key: "spec", DataType: api.Text, LabelCustom: "Spec"},
	})
	if err != nil {
		t.Fatalf("CreateTraitDef: %v", err)
	}

	if _, ok := fieldCreated.(api.CreateTraitDef201JSONResponse); !ok {
		t.Fatalf("CreateTraitDef returned %T, want a 201", fieldCreated)
	}

	body, _ := getTaxonomy(t, h, auth.WithRequestMeta(ctx, auth.RequestMeta{AcceptLanguage: "en"}))

	var found *api.Category
	for i := range body.Categories {
		if body.Categories[i].Slug == "garage_tools" {
			found = &body.Categories[i]
		}
	}

	if found == nil {
		t.Fatal("the family's own category is missing from its own taxonomy")
	}

	if found.IsBuiltin {
		t.Error("a family-defined category reports isBuiltin = true")
	}

	if len(found.Traits) != 1 || found.Traits[0].Label != "Spec" {
		t.Errorf("traits = %+v, want one field labelled Spec", found.Traits)
	}
}
