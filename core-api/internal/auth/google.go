package auth

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/olehmushka/mykomora/core-api/internal/config"
)

// ErrGoogleNotConfigured is returned when this deployment has no Google OAuth
// client. It is a supported state, not a bug: a fresh clone runs the whole
// stack without anyone provisioning credentials, and only sign-in reports
// itself unavailable.
var ErrGoogleNotConfigured = errors.New("google sign-in is not configured")

// ErrUnverifiedEmail is returned when Google asserts an address it has not
// verified. Accepting one would let anyone claim an address they do not own.
var ErrUnverifiedEmail = errors.New("google account has no verified email")

// Identity is what Google tells us about the human, reduced to the four
// things this application stores.
type Identity struct {
	// Subject is Google's stable identifier. It, and not the email address, is
	// the account key: people change addresses.
	Subject     string
	Email       string
	DisplayName string
	PictureURL  string
}

// GoogleProvider performs the authorization-code exchange and verifies the ID
// token against Google's published keys.
type GoogleProvider struct {
	cfg config.Config

	// Discovery is lazy so that core-api starts even when Google is
	// unreachable. Sign-in is the only thing that needs the network, and it
	// should be the only thing that fails without it.
	once     sync.Once
	oauth    *oauth2.Config
	verifier *oidc.IDTokenVerifier
	initErr  error
}

// NewGoogleProvider builds the provider. It performs no network calls.
func NewGoogleProvider(cfg config.Config) *GoogleProvider {
	return &GoogleProvider{cfg: cfg}
}

// Configured reports whether Google sign-in can be attempted at all.
func (g *GoogleProvider) Configured() bool { return g.cfg.GoogleConfigured() }

// discover resolves Google's OIDC endpoints and key set, once.
func (g *GoogleProvider) discover(ctx context.Context) error {
	if !g.Configured() {
		return ErrGoogleNotConfigured
	}

	g.once.Do(func() {
		provider, err := oidc.NewProvider(ctx, g.cfg.GoogleIssuerURL)
		if err != nil {
			g.initErr = fmt.Errorf("discover openid configuration: %w", err)

			return
		}

		g.oauth = &oauth2.Config{
			ClientID:     g.cfg.GoogleClientID,
			ClientSecret: g.cfg.GoogleClientSecret,
			RedirectURL:  g.cfg.GoogleRedirectURL,
			Endpoint:     provider.Endpoint(),
			Scopes:       []string{oidc.ScopeOpenID, "email", "profile"},
		}
		g.verifier = provider.Verifier(&oidc.Config{ClientID: g.cfg.GoogleClientID})
	})

	return g.initErr
}

// AuthCodeURL builds the consent-screen URL for a sign-in in progress.
func (g *GoogleProvider) AuthCodeURL(ctx context.Context, f flowState) (string, error) {
	if err := g.discover(ctx); err != nil {
		return "", err
	}

	return g.oauth.AuthCodeURL(f.State,
		oauth2.AccessTypeOffline,
		oauth2.S256ChallengeOption(f.Verifier),
	), nil
}

// Exchange redeems the authorization code and verifies the resulting ID token.
func (g *GoogleProvider) Exchange(ctx context.Context, code, verifier string) (Identity, error) {
	if err := g.discover(ctx); err != nil {
		return Identity{}, err
	}

	token, err := g.oauth.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return Identity{}, fmt.Errorf("exchange authorization code: %w", err)
	}

	rawID, ok := token.Extra("id_token").(string)
	if !ok {
		return Identity{}, errors.New("token response carried no id_token")
	}

	// The access token is deliberately discarded: this application never calls
	// a Google API on the user's behalf. The ID token is the whole point of
	// the exchange, and it is verified against the published key set rather
	// than trusted because it arrived over TLS.
	idToken, err := g.verifier.Verify(ctx, rawID)
	if err != nil {
		return Identity{}, fmt.Errorf("verify id token: %w", err)
	}

	return identityFromClaims(idToken)
}

// idTokenClaims is the subset of the ID token this application reads.
type idTokenClaims struct {
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

func identityFromClaims(idToken *oidc.IDToken) (Identity, error) {
	var claims idTokenClaims
	if err := idToken.Claims(&claims); err != nil {
		return Identity{}, fmt.Errorf("read id token claims: %w", err)
	}

	if claims.Email == "" || !claims.EmailVerified {
		return Identity{}, ErrUnverifiedEmail
	}

	name := claims.Name
	if name == "" {
		// Never empty: the display name seeds the first family's name, and an
		// unnamed family is worse than an awkward one.
		name = claims.Email
	}

	return Identity{
		Subject:     idToken.Subject,
		Email:       claims.Email,
		DisplayName: name,
		PictureURL:  claims.Picture,
	}, nil
}
