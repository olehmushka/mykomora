package auth

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/olehmushka/mykomora/core-api/internal/config"
)

const testSecret = "a-test-secret-that-is-long-enough-000000"

func testConfig() config.Config {
	return config.Config{
		Env:             "test",
		AppSecret:       testSecret,
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 30 * 24 * time.Hour,
	}
}

func testPrincipal() Principal {
	return Principal{
		UserID:   uuid.MustParse("11111111-1111-4111-8111-111111111111"),
		FamilyID: uuid.MustParse("22222222-2222-4222-8222-222222222222"),
		Role:     RoleOwner,
	}
}

func TestAccessTokenRoundTrip(t *testing.T) {
	t.Parallel()

	issuer := newTokenIssuer(testConfig())
	want := testPrincipal()

	token, expires, err := issuer.issueAccess(want)
	if err != nil {
		t.Fatalf("issueAccess: %v", err)
	}

	if delta := time.Until(expires); delta > 15*time.Minute || delta < 14*time.Minute {
		t.Errorf("token expires in %s, want about 15m", delta)
	}

	got, err := issuer.parseAccess(token)
	if err != nil {
		t.Fatalf("parseAccess: %v", err)
	}

	if got != want {
		t.Errorf("principal = %+v, want %+v", got, want)
	}
}

func TestAccessTokenRejections(t *testing.T) {
	t.Parallel()

	issuer := newTokenIssuer(testConfig())

	valid, _, err := issuer.issueAccess(testPrincipal())
	if err != nil {
		t.Fatalf("issueAccess: %v", err)
	}

	// Same claims, signed with a different key: the shape is right and the
	// signature is not.
	other := newTokenIssuer(config.Config{
		AppSecret:      strings.Repeat("b", 40),
		AccessTokenTTL: 15 * time.Minute,
	})

	foreign, _, err := other.issueAccess(testPrincipal())
	if err != nil {
		t.Fatalf("issueAccess with other key: %v", err)
	}

	expired := newTokenIssuer(testConfig())
	expired.now = func() time.Time { return time.Now().Add(-2 * time.Hour) }

	stale, _, err := expired.issueAccess(testPrincipal())
	if err != nil {
		t.Fatalf("issueAccess in the past: %v", err)
	}

	cases := map[string]string{
		"empty":            "",
		"not a jwt":        "nonsense",
		"signed elsewhere": foreign,
		"expired":          stale,
		"algorithm none":   unsignedToken(t, testPrincipal()),
		"payload edited":   tamper(valid),
	}

	for name, token := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if _, err := issuer.parseAccess(token); !errors.Is(err, ErrInvalidToken) {
				t.Errorf("parseAccess(%q) error = %v, want ErrInvalidToken", name, err)
			}
		})
	}
}

func TestAccessTokenRejectsUnknownRole(t *testing.T) {
	t.Parallel()

	issuer := newTokenIssuer(testConfig())

	p := testPrincipal()
	p.Role = "administrator"

	token, _, err := issuer.issueAccess(p)
	if err != nil {
		t.Fatalf("issueAccess: %v", err)
	}

	// A role that is not in the enum must not survive a round trip, or a
	// future authorisation check could be fooled by a claim nobody grants.
	if _, err := issuer.parseAccess(token); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("parseAccess error = %v, want ErrInvalidToken", err)
	}
}

// unsignedToken builds an `alg: none` token — the classic JWT forgery, and the
// reason parseAccess pins the algorithm.
func unsignedToken(t *testing.T, p Principal) string {
	t.Helper()

	encode := func(v any) string {
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}

		return base64.RawURLEncoding.EncodeToString(raw)
	}

	header := encode(map[string]string{"alg": "none", "typ": "JWT"})
	claims := encode(map[string]any{
		"sub":  p.UserID.String(),
		"fam":  p.FamilyID.String(),
		"role": string(p.Role),
		"exp":  time.Now().Add(time.Hour).Unix(),
	})

	return header + "." + claims + "."
}

// tamper flips a character in the payload, leaving the signature stale.
func tamper(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[1] == "" {
		return token
	}

	body := []byte(parts[1])
	if body[0] == 'A' {
		body[0] = 'B'
	} else {
		body[0] = 'A'
	}

	parts[1] = string(body)

	return strings.Join(parts, ".")
}

func TestNewTokenIsUnpredictable(t *testing.T) {
	t.Parallel()

	seen := make(map[string]struct{}, 100)

	for range 100 {
		token, err := NewToken()
		if err != nil {
			t.Fatalf("NewToken: %v", err)
		}

		if len(token) < 40 {
			t.Fatalf("token %q is shorter than 256 bits of base64", token)
		}

		if _, duplicate := seen[token]; duplicate {
			t.Fatalf("NewToken returned %q twice", token)
		}

		seen[token] = struct{}{}
	}
}

func TestHashTokenIsStableAndOneWay(t *testing.T) {
	t.Parallel()

	const raw = "a-refresh-token"

	first := HashToken(raw)
	if len(first) != 32 {
		t.Fatalf("digest is %d bytes, want 32", len(first))
	}

	if !bytes.Equal(first, HashToken(raw)) {
		t.Error("HashToken is not deterministic; stored digests would never match")
	}

	if string(first) == raw || strings.Contains(string(first), raw) {
		t.Error("the digest contains the token it is meant to hide")
	}
}
