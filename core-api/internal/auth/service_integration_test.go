//go:build integration

// Integration tests for identity, against a real Postgres.
//
// These are the ones that prove M1's "Done when": two Google accounts share
// one family. Everything here runs the real queries against the real schema,
// because the parts most likely to be wrong — a single-use invite that is not
// actually single-use, a rotation that leaves two live tokens — are only wrong
// in the presence of a database.
//
// Google is never contacted: sign-in takes a verified Identity, and verifying
// it is the one step these tests skip.
package auth_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/olehmushka/mykomora/core-api/internal/auth"
	"github.com/olehmushka/mykomora/core-api/internal/config"
	"github.com/olehmushka/mykomora/core-api/internal/db"
	"github.com/olehmushka/mykomora/core-api/internal/pgconv"
	"github.com/olehmushka/mykomora/core-api/internal/postgres"
	"github.com/olehmushka/mykomora/core-api/internal/testdb"
)

type harness struct {
	service *auth.Service
	queries *db.Queries
	tx      *postgres.TxRunner
	cfg     config.Config
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	database := testdb.Start(t)
	queries := db.New(database.Pool)
	tx := postgres.NewTxRunner(database.Pool)

	cfg := config.Config{
		Env:             "test",
		AppSecret:       "an-integration-test-secret-0000000000000",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 30 * 24 * time.Hour,
		InviteTTL:       7 * 24 * time.Hour,
	}

	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	return &harness{
		service: auth.NewService(queries, tx, cfg, log),
		queries: queries,
		tx:      tx,
		cfg:     cfg,
	}
}

func identity(sub, email, name string) auth.Identity {
	return auth.Identity{Subject: sub, Email: email, DisplayName: name}
}

// The first half of the milestone: signing in with a brand-new Google account
// produces a family with that account as its owner.
func TestFirstSignInCreatesAFamilyOwnedByTheUser(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)

	session, err := h.service.SignIn(ctx, identity("google-1", "one@example.com", "Олег"), "", auth.RequestMeta{})
	if err != nil {
		t.Fatalf("SignIn: %v", err)
	}

	if session.Principal.Role != auth.RoleOwner {
		t.Errorf("role = %q, want owner", session.Principal.Role)
	}

	if session.Principal.FamilyID == uuid.Nil {
		t.Fatal("the session carries no family")
	}

	family, err := h.queries.GetFamily(ctx, db.GetFamilyParams{
		FamilyID: pgconv.UUID(session.Principal.FamilyID),
		UserID:   pgconv.UUID(session.Principal.UserID),
	})
	if err != nil {
		t.Fatalf("GetFamily: %v", err)
	}

	// Named after the person, with nothing appended: a server-minted suffix
	// would be an untranslated user-visible string.
	if family.Name != "Олег" {
		t.Errorf("family name = %q, want the display name", family.Name)
	}

	if family.DefaultCurrency != "UAH" {
		t.Errorf("default currency = %q, want UAH", family.DefaultCurrency)
	}
}

// Signing in again must land in the same family rather than minting another.
func TestSecondSignInReusesTheSameFamily(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)

	first, err := h.service.SignIn(ctx, identity("google-1", "one@example.com", "One"), "", auth.RequestMeta{})
	if err != nil {
		t.Fatalf("first SignIn: %v", err)
	}

	second, err := h.service.SignIn(ctx, identity("google-1", "one@example.com", "One"), "", auth.RequestMeta{})
	if err != nil {
		t.Fatalf("second SignIn: %v", err)
	}

	if first.Principal.UserID != second.Principal.UserID {
		t.Error("the same Google subject produced two accounts")
	}

	if first.Principal.FamilyID != second.Principal.FamilyID {
		t.Error("signing in again created a second family")
	}
}

// The milestone's own acceptance sentence: two Google accounts, one family.
func TestInviteJoinsASecondAccountToTheSameFamily(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)

	owner, err := h.service.SignIn(ctx, identity("google-1", "one@example.com", "One"), "", auth.RequestMeta{})
	if err != nil {
		t.Fatalf("owner SignIn: %v", err)
	}

	token := h.issueInvite(t, owner.Principal)

	guest, err := h.service.SignIn(ctx, identity("google-2", "two@example.com", "Two"), token, auth.RequestMeta{})
	if err != nil {
		t.Fatalf("guest SignIn: %v", err)
	}

	if guest.Principal.FamilyID != owner.Principal.FamilyID {
		t.Fatalf("guest family = %s, want the owner's %s",
			guest.Principal.FamilyID, owner.Principal.FamilyID)
	}

	if guest.Principal.Role != auth.RoleMember {
		t.Errorf("guest role = %q, want member", guest.Principal.Role)
	}

	members, err := h.queries.ListFamilyMembers(ctx, pgconv.UUID(owner.Principal.FamilyID))
	if err != nil {
		t.Fatalf("ListFamilyMembers: %v", err)
	}

	if len(members) != 2 {
		t.Fatalf("family has %d members, want 2", len(members))
	}
}

// An invite is single-use. The second person to follow the same link gets a
// family of their own rather than a seat in someone else's.
func TestInviteCannotBeUsedTwice(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)

	owner, err := h.service.SignIn(ctx, identity("google-1", "one@example.com", "One"), "", auth.RequestMeta{})
	if err != nil {
		t.Fatalf("owner SignIn: %v", err)
	}

	token := h.issueInvite(t, owner.Principal)

	if _, err := h.service.SignIn(ctx, identity("google-2", "two@example.com", "Two"), token, auth.RequestMeta{}); err != nil {
		t.Fatalf("first use of the invite: %v", err)
	}

	third, err := h.service.SignIn(ctx, identity("google-3", "three@example.com", "Three"), token, auth.RequestMeta{})
	if err != nil {
		t.Fatalf("second use of the invite: %v", err)
	}

	if third.Principal.FamilyID == owner.Principal.FamilyID {
		t.Error("a spent invite still admitted someone")
	}

	if third.Principal.Role != auth.RoleOwner {
		t.Errorf("role = %q; a dead invite should fall through to a family of one's own",
			third.Principal.Role)
	}
}

func TestRefreshRotatesTheTokenPair(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)

	first, err := h.service.SignIn(ctx, identity("google-1", "one@example.com", "One"), "", auth.RequestMeta{})
	if err != nil {
		t.Fatalf("SignIn: %v", err)
	}

	second, err := h.service.Refresh(ctx, first.RefreshToken, auth.RequestMeta{})
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	if second.RefreshToken == first.RefreshToken {
		t.Error("refresh returned the same token; nothing was rotated")
	}

	if second.Principal != first.Principal {
		t.Errorf("principal changed across a refresh: %+v then %+v", first.Principal, second.Principal)
	}

	// The retired token must be dead immediately, not merely superseded.
	if _, err := h.service.Refresh(ctx, first.RefreshToken, auth.RequestMeta{}); !errors.Is(err, auth.ErrTokenReuse) {
		t.Fatalf("replaying the old token: error = %v, want ErrTokenReuse", err)
	}
}

// Reuse detection, and its deliberate blast radius: a replayed token is
// evidence that the account is compromised, so every session it has goes.
func TestTokenReuseRevokesEverySessionOfTheAccount(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)

	laptop, err := h.service.SignIn(ctx, identity("google-1", "one@example.com", "One"), "", auth.RequestMeta{})
	if err != nil {
		t.Fatalf("first SignIn: %v", err)
	}

	phone, err := h.service.SignIn(ctx, identity("google-1", "one@example.com", "One"), "", auth.RequestMeta{})
	if err != nil {
		t.Fatalf("second SignIn: %v", err)
	}

	rotated, err := h.service.Refresh(ctx, laptop.RefreshToken, auth.RequestMeta{})
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	if _, err := h.service.Refresh(ctx, laptop.RefreshToken, auth.RequestMeta{}); !errors.Is(err, auth.ErrTokenReuse) {
		t.Fatalf("replay: error = %v, want ErrTokenReuse", err)
	}

	for name, token := range map[string]string{
		"the other device":      phone.RefreshToken,
		"the rotated successor": rotated.RefreshToken,
	} {
		if _, err := h.service.Refresh(ctx, token, auth.RequestMeta{}); !errors.Is(err, auth.ErrNoSession) {
			t.Errorf("%s: error = %v, want ErrNoSession — reuse must revoke it too", name, err)
		}
	}
}

func TestLogoutRevokesOnlyThePresentedToken(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)

	laptop, err := h.service.SignIn(ctx, identity("google-1", "one@example.com", "One"), "", auth.RequestMeta{})
	if err != nil {
		t.Fatalf("first SignIn: %v", err)
	}

	phone, err := h.service.SignIn(ctx, identity("google-1", "one@example.com", "One"), "", auth.RequestMeta{})
	if err != nil {
		t.Fatalf("second SignIn: %v", err)
	}

	if err := h.service.Logout(ctx, laptop.Principal, laptop.RefreshToken); err != nil {
		t.Fatalf("Logout: %v", err)
	}

	if _, err := h.service.Refresh(ctx, laptop.RefreshToken, auth.RequestMeta{}); !errors.Is(err, auth.ErrNoSession) {
		t.Errorf("signed-out token: error = %v, want ErrNoSession", err)
	}

	// Signing out of one browser must not sign you out of the other.
	if _, err := h.service.Refresh(ctx, phone.RefreshToken, auth.RequestMeta{}); err != nil {
		t.Errorf("the other session: error = %v, want it still valid", err)
	}
}

// The tenancy invariant, exercised rather than asserted: one family's rows are
// not reachable with another family's id.
func TestFamilyScopingHidesAnotherFamilysPeople(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)

	one, err := h.service.SignIn(ctx, identity("google-1", "one@example.com", "One"), "", auth.RequestMeta{})
	if err != nil {
		t.Fatalf("SignIn one: %v", err)
	}

	two, err := h.service.SignIn(ctx, identity("google-2", "two@example.com", "Two"), "", auth.RequestMeta{})
	if err != nil {
		t.Fatalf("SignIn two: %v", err)
	}

	person, err := h.queries.CreatePerson(ctx, db.CreatePersonParams{
		FamilyID: pgconv.UUID(one.Principal.FamilyID),
		Name:     "Дитина",
		IsChild:  true,
	})
	if err != nil {
		t.Fatalf("CreatePerson: %v", err)
	}

	visible, err := h.queries.ListPeople(ctx, db.ListPeopleParams{
		FamilyID: pgconv.UUID(two.Principal.FamilyID),
	})
	if err != nil {
		t.Fatalf("ListPeople: %v", err)
	}

	if len(visible) != 0 {
		t.Errorf("the other family sees %d people, want 0", len(visible))
	}

	_, err = h.queries.GetPerson(ctx, db.GetPersonParams{
		ID:       person.ID,
		FamilyID: pgconv.UUID(two.Principal.FamilyID),
	})

	if !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("GetPerson across families: error = %v, want no rows", err)
	}
}

// `families` has no family_id column of its own, so the two queries that read
// and write it scope through membership instead. This is what that buys: a
// token claiming someone else's family is not enough to read it.
func TestReadingAFamilyRequiresMembership(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)

	one, err := h.service.SignIn(ctx, identity("google-1", "one@example.com", "One"), "", auth.RequestMeta{})
	if err != nil {
		t.Fatalf("SignIn one: %v", err)
	}

	two, err := h.service.SignIn(ctx, identity("google-2", "two@example.com", "Two"), "", auth.RequestMeta{})
	if err != nil {
		t.Fatalf("SignIn two: %v", err)
	}

	if _, err := h.queries.GetFamily(ctx, db.GetFamilyParams{
		FamilyID: pgconv.UUID(one.Principal.FamilyID),
		UserID:   pgconv.UUID(two.Principal.UserID),
	}); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("GetFamily as a non-member: error = %v, want no rows", err)
	}

	if _, err := h.queries.UpdateFamily(ctx, db.UpdateFamilyParams{
		FamilyID: pgconv.UUID(one.Principal.FamilyID),
		UserID:   pgconv.UUID(two.Principal.UserID),
		Name:     pgconv.Text("taken over"),
	}); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("UpdateFamily as a non-member: error = %v, want no rows", err)
	}
}

// issueInvite creates an invite the way the family handler does, and returns
// the raw token that goes in the link.
func (h *harness) issueInvite(t *testing.T, owner auth.Principal) string {
	t.Helper()

	token, err := auth.NewToken()
	if err != nil {
		t.Fatalf("NewToken: %v", err)
	}

	if _, err := h.queries.CreateFamilyInvite(context.Background(), db.CreateFamilyInviteParams{
		FamilyID:  pgconv.UUID(owner.FamilyID),
		TokenHash: auth.HashToken(token),
		CreatedBy: pgconv.UUID(owner.UserID),
		ExpiresAt: pgconv.Timestamptz(time.Now().Add(h.cfg.InviteTTL)),
	}); err != nil {
		t.Fatalf("CreateFamilyInvite: %v", err)
	}

	return token
}
