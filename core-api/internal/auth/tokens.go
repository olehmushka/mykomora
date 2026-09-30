package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/olehmushka/mykomora/core-api/internal/config"
)

// ErrInvalidToken is returned for any access token that cannot be trusted:
// wrong signature, wrong algorithm, expired, or malformed. Callers get one
// error rather than a taxonomy, because the correct response to all of them is
// the same and a more specific message only helps an attacker.
var ErrInvalidToken = errors.New("invalid access token")

// tokenIssuer is this API's own token authority. Google authenticates the
// human; everything after that is signed here.
type tokenIssuer struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
	now        func() time.Time
}

// newTokenIssuer builds the issuer from configuration.
func newTokenIssuer(cfg config.Config) *tokenIssuer {
	return &tokenIssuer{
		secret:     []byte(cfg.AppSecret),
		accessTTL:  cfg.AccessTokenTTL,
		refreshTTL: cfg.RefreshTokenTTL,
		now:        time.Now,
	}
}

// accessClaims is the access token's payload.
//
// The tenant travels in the token: a request's family is decided at sign-in
// and verified on every call, never read from the path or the body.
type accessClaims struct {
	jwt.RegisteredClaims

	FamilyID string `json:"fam"`
	Role     string `json:"role"`
}

// issueAccess mints a short-lived access token for the principal.
func (t *tokenIssuer) issueAccess(p Principal) (string, time.Time, error) {
	issued := t.now().UTC()
	expires := issued.Add(t.accessTTL)

	claims := accessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   p.UserID.String(),
			ID:        uuid.NewString(),
			IssuedAt:  jwt.NewNumericDate(issued),
			ExpiresAt: jwt.NewNumericDate(expires),
		},
		FamilyID: p.FamilyID.String(),
		Role:     string(p.Role),
	}

	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(t.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign access token: %w", err)
	}

	return signed, expires, nil
}

// parseAccess verifies an access token and recovers the principal it carries.
func (t *tokenIssuer) parseAccess(raw string) (Principal, error) {
	var claims accessClaims

	// The algorithm is pinned: without this, a token signed with "none" or with
	// an asymmetric algorithm would be accepted by a permissive parser.
	_, err := jwt.ParseWithClaims(raw, &claims,
		func(*jwt.Token) (any, error) { return t.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithTimeFunc(t.now),
	)
	if err != nil {
		return Principal{}, fmt.Errorf("%w: %s", ErrInvalidToken, err)
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return Principal{}, fmt.Errorf("%w: subject is not a uuid", ErrInvalidToken)
	}

	familyID, err := uuid.Parse(claims.FamilyID)
	if err != nil {
		return Principal{}, fmt.Errorf("%w: family claim is not a uuid", ErrInvalidToken)
	}

	role := Role(claims.Role)
	if role != RoleOwner && role != RoleMember {
		return Principal{}, fmt.Errorf("%w: unknown role %q", ErrInvalidToken, claims.Role)
	}

	return Principal{UserID: userID, FamilyID: familyID, Role: role}, nil
}

// opaqueTokenBytes is 256 bits of entropy — enough that guessing is not a
// threat model, and short enough to sit comfortably in a cookie.
const opaqueTokenBytes = 32

// NewToken returns a random, URL-safe secret.
//
// Refresh tokens, CSRF tokens and invite tokens are opaque rather than signed:
// they are looked up or compared, never parsed, so there is nothing for a
// forger to work with. Exported because invites are issued by the family
// package but must be minted the same way as everything else here.
func NewToken() (string, error) {
	buf := make([]byte, opaqueTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// HashToken is what gets stored. Refresh and invite tokens live in the
// database only as digests, so a database dump yields nothing replayable.
//
// Exported alongside NewToken, and for the same reason: every secret in this
// system is hashed by this one function.
//
// A plain SHA-256 is right here, and a password hash would not be: these are
// full-entropy random values, not user-chosen secrets, so there is no
// dictionary to slow an attacker down with.
func HashToken(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))

	return sum[:]
}
