package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/olehmushka/mykomora/core-api/internal/config"
	"github.com/olehmushka/mykomora/core-api/internal/db"
	"github.com/olehmushka/mykomora/core-api/internal/pgconv"
	"github.com/olehmushka/mykomora/core-api/internal/postgres"
)

// Session errors. They are deliberately coarse at the boundary: a caller with
// no valid session gets one answer, whatever the underlying reason.
var (
	// ErrNoSession means there is no usable session: absent, expired, revoked
	// or unknown.
	ErrNoSession = errors.New("no valid session")

	// ErrTokenReuse means a refresh token was presented after it had already
	// been exchanged. It is handled separately from ErrNoSession because it is
	// evidence of theft, not of ordinary expiry.
	ErrTokenReuse = errors.New("refresh token reuse detected")
)

// Store is the slice of the generated queries this package needs.
//
// It is declared here, narrowly, rather than taken as db.Querier: that
// interface grows with every query in the repo, and a handler that can reach
// any query is a handler that can reach an unscoped one.
type Store interface {
	GetUserByID(ctx context.Context, id pgtype.UUID) (db.User, error)
	GetFamily(ctx context.Context, arg db.GetFamilyParams) (db.Family, error)
	GetFamilyMembership(ctx context.Context, arg db.GetFamilyMembershipParams) (db.FamilyMember, error)
	GetRefreshTokenByHash(ctx context.Context, tokenHash []byte) (db.GetRefreshTokenByHashRow, error)
	CreateRefreshToken(ctx context.Context, arg db.CreateRefreshTokenParams) (db.CreateRefreshTokenRow, error)
	RevokeRefreshToken(ctx context.Context, arg db.RevokeRefreshTokenParams) (int64, error)
	RevokeAllUserRefreshTokens(ctx context.Context, userID pgtype.UUID) (int64, error)
}

// tokenWriter is the one query the issuing path needs, so that it can run
// either against the pool or inside a transaction.
type tokenWriter interface {
	CreateRefreshToken(ctx context.Context, arg db.CreateRefreshTokenParams) (db.CreateRefreshTokenRow, error)
}

// Session is a freshly issued pair of tokens and the principal they carry.
type Session struct {
	Principal     Principal
	AccessToken   string
	AccessExpiry  time.Time
	RefreshToken  string
	RefreshExpiry time.Time
	CSRFToken     string

	// refreshID is the database row of the refresh token, needed to record a
	// rotation. It does not leave this package.
	refreshID pgtype.UUID
}

// Service implements sign-in, session rotation and sign-out.
type Service struct {
	store  Store
	tx     *postgres.TxRunner
	tokens *tokenIssuer
	log    *slog.Logger
}

// NewService builds the identity service.
func NewService(store Store, tx *postgres.TxRunner, cfg config.Config, log *slog.Logger) *Service {
	return &Service{
		store:  store,
		tx:     tx,
		tokens: newTokenIssuer(cfg),
		log:    log.With(slog.String("component", "auth")),
	}
}

// SignIn turns a verified Google identity into a session.
//
// Everything that decides *which tenant* the user lands in happens in one
// transaction: upserting the account, accepting an invite, and creating a
// family with its first owner. A half-applied sign-in would leave an account
// with no family or a family with no owner, and both are unrecoverable from
// the UI.
func (s *Service) SignIn(ctx context.Context, ident Identity, invite string, meta RequestMeta) (Session, error) {
	var principal Principal

	err := s.tx.InTx(ctx, func(q *db.Queries) error {
		user, err := q.UpsertUserByGoogleSub(ctx, db.UpsertUserByGoogleSubParams{
			GoogleSub:   ident.Subject,
			Email:       ident.Email,
			DisplayName: ident.DisplayName,
			AvatarUrl:   optionalText(ident.PictureURL),
		})
		if err != nil {
			return fmt.Errorf("upsert user: %w", err)
		}

		familyID, role, err := resolveFamily(ctx, q, user, invite)
		if err != nil {
			return err
		}

		principal = Principal{
			UserID:   pgconv.ToUUID(user.ID),
			FamilyID: familyID,
			Role:     role,
		}

		return nil
	})
	if err != nil {
		return Session{}, err
	}

	return s.issue(ctx, s.store, principal, meta)
}

// resolveFamily decides which tenant a sign-in lands in.
//
// The order matters: an invite wins over an existing membership, so following
// a link always does what the link says. A dead link is not an error — the
// user still signs in, and the join page explains why it did not work, which
// is far better than an authentication failure they cannot act on.
func resolveFamily(
	ctx context.Context,
	q *db.Queries,
	user db.User,
	invite string,
) (uuid.UUID, Role, error) {
	if invite != "" {
		accepted, err := q.AcceptInvite(ctx, db.AcceptInviteParams{
			TokenHash:        HashToken(invite),
			AcceptedByUserID: user.ID,
		})

		switch {
		case err == nil:
			if _, err := q.AddFamilyMember(ctx, db.AddFamilyMemberParams{
				FamilyID: accepted.FamilyID,
				UserID:   user.ID,
				Role:     db.FamilyRoleMember,
			}); err != nil {
				return uuid.Nil, "", fmt.Errorf("join family: %w", err)
			}

			return pgconv.ToUUID(accepted.FamilyID), RoleMember, nil
		case !errors.Is(err, pgx.ErrNoRows):
			return uuid.Nil, "", fmt.Errorf("accept invite: %w", err)
		}
	}

	existing, err := q.GetFirstFamilyForUser(ctx, user.ID)

	switch {
	case err == nil:
		return pgconv.ToUUID(existing.FamilyID), Role(existing.Role), nil
	case errors.Is(err, pgx.ErrNoRows):
		return createOwnFamily(ctx, q, user)
	default:
		return uuid.Nil, "", fmt.Errorf("look up membership: %w", err)
	}
}

// createOwnFamily gives a brand-new account a tenant of its own.
//
// The family is named after the person, with no word appended: any English or
// Ukrainian suffix would be a user-visible string minted by the server, and
// the name is editable in settings the moment they care.
func createOwnFamily(ctx context.Context, q *db.Queries, user db.User) (uuid.UUID, Role, error) {
	family, err := q.CreateFamily(ctx, user.DisplayName)
	if err != nil {
		return uuid.Nil, "", fmt.Errorf("create family: %w", err)
	}

	if _, err := q.AddFamilyMember(ctx, db.AddFamilyMemberParams{
		FamilyID: family.FamilyID,
		UserID:   user.ID,
		Role:     db.FamilyRoleOwner,
	}); err != nil {
		return uuid.Nil, "", fmt.Errorf("add first owner: %w", err)
	}

	return pgconv.ToUUID(family.FamilyID), RoleOwner, nil
}

// Refresh rotates a session.
//
// A token presented after it was already exchanged can only have been
// captured: the legitimate client replaced it at the previous refresh and no
// longer has it. That is the one case that escalates — every refresh token of
// the account is revoked, across every family it belongs to.
func (s *Service) Refresh(ctx context.Context, raw string, meta RequestMeta) (Session, error) {
	if raw == "" {
		return Session{}, ErrNoSession
	}

	row, err := s.store.GetRefreshTokenByHash(ctx, HashToken(raw))

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return Session{}, ErrNoSession
	case err != nil:
		return Session{}, fmt.Errorf("look up refresh token: %w", err)
	}

	if row.RotatedTo.Valid {
		return Session{}, s.handleReuse(ctx, row)
	}

	if row.RevokedAt.Valid || time.Now().After(pgconv.ToTime(row.ExpiresAt)) {
		return Session{}, ErrNoSession
	}

	member, err := s.store.GetFamilyMembership(ctx, db.GetFamilyMembershipParams{
		FamilyID: row.FamilyID,
		UserID:   row.UserID,
	})

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		// Removed from the family since the token was issued. The token is
		// still cryptographically fine and must still stop working.
		return Session{}, ErrNoSession
	case err != nil:
		return Session{}, fmt.Errorf("look up membership: %w", err)
	}

	principal := Principal{
		UserID:   pgconv.ToUUID(row.UserID),
		FamilyID: pgconv.ToUUID(row.FamilyID),
		Role:     Role(member.Role),
	}

	return s.rotate(ctx, row, principal, meta)
}

// handleReuse revokes the account's sessions after a replayed token.
func (s *Service) handleReuse(ctx context.Context, row db.GetRefreshTokenByHashRow) error {
	revoked, err := s.store.RevokeAllUserRefreshTokens(ctx, row.UserID)
	if err != nil {
		// Still reported as reuse: failing to revoke must not turn a detected
		// theft into an ordinary 401 the attacker can retry past.
		s.log.ErrorContext(ctx, "could not revoke sessions after token reuse",
			slog.String("user_id", pgconv.ToUUID(row.UserID).String()),
			slog.String("error", err.Error()),
		)

		return ErrTokenReuse
	}

	s.log.WarnContext(ctx, "refresh token reuse detected; revoked every session of the account",
		slog.String("user_id", pgconv.ToUUID(row.UserID).String()),
		slog.Int64("revoked", revoked),
	)

	return ErrTokenReuse
}

// rotate issues the successor and retires the presented token, atomically.
func (s *Service) rotate(
	ctx context.Context,
	row db.GetRefreshTokenByHashRow,
	principal Principal,
	meta RequestMeta,
) (Session, error) {
	var session Session

	err := s.tx.InTx(ctx, func(q *db.Queries) error {
		issued, err := s.issue(ctx, q, principal, meta)
		if err != nil {
			return err
		}

		affected, err := q.RotateRefreshToken(ctx, db.RotateRefreshTokenParams{
			ID:        row.ID,
			RotatedTo: issued.refreshID,
			FamilyID:  row.FamilyID,
		})
		if err != nil {
			return fmt.Errorf("retire refresh token: %w", err)
		}

		if affected == 0 {
			// Another refresh won the race and already retired this token.
			return ErrNoSession
		}

		session = issued

		return nil
	})
	if err != nil {
		if errors.Is(err, ErrNoSession) {
			return Session{}, ErrNoSession
		}

		return Session{}, err
	}

	return session, nil
}

// Logout revokes the presented refresh token.
//
// An unknown or absent token is not an error: sign-out must always succeed
// from the user's point of view, and the cookies are cleared either way.
func (s *Service) Logout(ctx context.Context, p Principal, raw string) error {
	if raw == "" {
		return nil
	}

	row, err := s.store.GetRefreshTokenByHash(ctx, HashToken(raw))

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil
	case err != nil:
		return fmt.Errorf("look up refresh token: %w", err)
	}

	if _, err := s.store.RevokeRefreshToken(ctx, db.RevokeRefreshTokenParams{
		ID:       row.ID,
		FamilyID: pgconv.UUID(p.FamilyID),
	}); err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}

	return nil
}

// issue mints an access token, a refresh token and a CSRF token, and records
// the refresh token's digest.
func (s *Service) issue(
	ctx context.Context,
	w tokenWriter,
	p Principal,
	meta RequestMeta,
) (Session, error) {
	access, accessExpiry, err := s.tokens.issueAccess(p)
	if err != nil {
		return Session{}, err
	}

	refresh, err := NewToken()
	if err != nil {
		return Session{}, err
	}

	csrf, err := NewToken()
	if err != nil {
		return Session{}, err
	}

	refreshExpiry := time.Now().Add(s.tokens.refreshTTL)

	row, err := w.CreateRefreshToken(ctx, db.CreateRefreshTokenParams{
		UserID:    pgconv.UUID(p.UserID),
		FamilyID:  pgconv.UUID(p.FamilyID),
		TokenHash: HashToken(refresh),
		ExpiresAt: pgconv.Timestamptz(refreshExpiry),
		UserAgent: optionalText(meta.UserAgent),
		Ip:        meta.IP,
	})
	if err != nil {
		return Session{}, fmt.Errorf("store refresh token: %w", err)
	}

	return Session{
		Principal:     p,
		AccessToken:   access,
		AccessExpiry:  accessExpiry,
		RefreshToken:  refresh,
		RefreshExpiry: pgconv.ToTime(row.ExpiresAt),
		CSRFToken:     csrf,
		refreshID:     row.ID,
	}, nil
}

// optionalText treats an empty string as absent, which is what the schema
// means by a nullable text column.
func optionalText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}

	return pgconv.Text(s)
}
