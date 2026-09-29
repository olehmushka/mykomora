package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/olehmushka/mykomora/core-api/internal/config"
)

// Google is never contacted from a test. This file stands up a real OIDC
// issuer on localhost — discovery document, JWKS, token endpoint — and points
// the provider at it, so the code exchange and the ID-token verification are
// exercised for real against a key the test controls.

const testKeyID = "test-key"

// fakeIssuer is a minimal OpenID provider.
type fakeIssuer struct {
	server *httptest.Server
	key    *rsa.PrivateKey

	// claims is what the next token endpoint call returns in its ID token.
	claims map[string]any
}

func newFakeIssuer(t *testing.T, clientID string) *fakeIssuer {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	issuer := &fakeIssuer{key: key}
	mux := http.NewServeMux()
	issuer.server = httptest.NewServer(mux)
	t.Cleanup(issuer.server.Close)

	issuer.claims = map[string]any{
		"sub":            "google-subject-1",
		"email":          "person@example.com",
		"email_verified": true,
		"name":           "Олег Мушка",
		"picture":        "https://example.com/avatar.png",
	}

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, map[string]any{
			"issuer":                                issuer.server.URL,
			"authorization_endpoint":                issuer.server.URL + "/authorize",
			"token_endpoint":                        issuer.server.URL + "/token",
			"jwks_uri":                              issuer.server.URL + "/jwks",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})

	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, map[string]any{"keys": []map[string]string{{
			"kty": "RSA",
			"alg": "RS256",
			"use": "sig",
			"kid": testKeyID,
			"n":   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
			"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
		}}})
	})

	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, map[string]any{
			"access_token": "an-access-token-we-never-use",
			"token_type":   "Bearer",
			"expires_in":   3600,
			"id_token":     issuer.idToken(t, clientID),
		})
	})

	return issuer
}

func (f *fakeIssuer) idToken(t *testing.T, audience string) string {
	t.Helper()

	claims := jwt.MapClaims{
		"iss": f.server.URL,
		"aud": audience,
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}

	for k, v := range f.claims {
		claims[k] = v
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = testKeyID

	signed, err := token.SignedString(f.key)
	if err != nil {
		t.Fatalf("sign id token: %v", err)
	}

	return signed
}

func writeJSON(t *testing.T, w http.ResponseWriter, body any) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(body); err != nil {
		t.Errorf("write response: %v", err)
	}
}

func googleConfig(issuerURL string) config.Config {
	cfg := testConfig()
	cfg.GoogleClientID = "test-client-id"
	cfg.GoogleClientSecret = "test-client-secret"
	cfg.GoogleRedirectURL = "http://localhost:8080/api/v1/auth/google/callback"
	cfg.GoogleIssuerURL = issuerURL

	return cfg
}

func TestAuthCodeURLCarriesStateAndPKCE(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	issuer := newFakeIssuer(t, "test-client-id")
	provider := NewGoogleProvider(googleConfig(issuer.server.URL))

	flow, err := testCodec().newFlow("", "/")
	if err != nil {
		t.Fatalf("newFlow: %v", err)
	}

	raw, err := provider.AuthCodeURL(ctx, flow)
	if err != nil {
		t.Fatalf("AuthCodeURL: %v", err)
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse consent url: %v", err)
	}

	query := parsed.Query()

	if got := query.Get("state"); got != flow.State {
		t.Errorf("state = %q, want %q", got, flow.State)
	}

	if got := query.Get("code_challenge_method"); got != "S256" {
		t.Errorf("code_challenge_method = %q, want S256", got)
	}

	// The challenge travels; the verifier must not.
	if query.Get("code_challenge") == "" {
		t.Error("no code_challenge: the exchange would not be PKCE-protected")
	}

	if strings.Contains(raw, flow.Verifier) {
		t.Error("the PKCE verifier leaked into the consent URL")
	}
}

func TestExchangeVerifiesTheIDToken(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	issuer := newFakeIssuer(t, "test-client-id")
	provider := NewGoogleProvider(googleConfig(issuer.server.URL))

	identity, err := provider.Exchange(ctx, "an-authorization-code", oauthVerifier)
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}

	want := Identity{
		Subject:     "google-subject-1",
		Email:       "person@example.com",
		DisplayName: "Олег Мушка",
		PictureURL:  "https://example.com/avatar.png",
	}

	if identity != want {
		t.Errorf("identity = %+v, want %+v", identity, want)
	}
}

func TestExchangeRejectsUnverifiedEmail(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	issuer := newFakeIssuer(t, "test-client-id")
	issuer.claims["email_verified"] = false

	provider := NewGoogleProvider(googleConfig(issuer.server.URL))

	// Accepting an unverified address would let anyone sign in as the owner of
	// an address they do not control.
	if _, err := provider.Exchange(ctx, "code", oauthVerifier); !errors.Is(err, ErrUnverifiedEmail) {
		t.Errorf("error = %v, want ErrUnverifiedEmail", err)
	}
}

func TestExchangeRejectsATokenForAnotherClient(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// The issuer mints tokens for a different audience than the one we are
	// configured as. A token meant for someone else's application must not
	// sign anyone into ours.
	issuer := newFakeIssuer(t, "some-other-application")
	provider := NewGoogleProvider(googleConfig(issuer.server.URL))

	if _, err := provider.Exchange(ctx, "code", oauthVerifier); err == nil {
		t.Error("Exchange accepted an id token issued for another client")
	}
}

func TestUnconfiguredProviderReportsItself(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	provider := NewGoogleProvider(testConfig())

	if provider.Configured() {
		t.Fatal("a config with no client id reports itself configured")
	}

	if _, err := provider.AuthCodeURL(ctx, flowState{}); !errors.Is(err, ErrGoogleNotConfigured) {
		t.Errorf("AuthCodeURL error = %v, want ErrGoogleNotConfigured", err)
	}

	if _, err := provider.Exchange(ctx, "code", "verifier"); !errors.Is(err, ErrGoogleNotConfigured) {
		t.Errorf("Exchange error = %v, want ErrGoogleNotConfigured", err)
	}
}

// oauthVerifier is any syntactically valid PKCE verifier; the fake token
// endpoint does not check it, because what these tests are about is the ID
// token that comes back.
const oauthVerifier = "0123456789012345678901234567890123456789012c"
