package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

// ErrInvalidFlow is returned when the OAuth flow cookie is missing, tampered
// with, expired, or does not match the `state` Google handed back.
var ErrInvalidFlow = errors.New("invalid sign-in flow")

// flowTTL bounds how long a half-finished sign-in stays valid. Long enough to
// read a consent screen, short enough that an abandoned flow is not a
// lingering credential.
const flowTTL = 10 * time.Minute

// flowState is everything the callback needs to remember about a sign-in that
// is still in progress.
//
// It travels in a signed cookie rather than a table. The alternative — an
// `oauth_states` row per click — buys nothing here: the value is already bound
// to one browser, it expires on its own, and a table would need sweeping.
type flowState struct {
	// State is echoed by Google and compared against this cookie, which is
	// what makes a forged callback useless.
	State string `json:"state"`
	// Verifier is the PKCE secret. It never leaves this cookie until the code
	// exchange, so an intercepted authorization code cannot be redeemed.
	Verifier string `json:"verifier"`
	// Invite carries a `/join/{token}` link through the round trip.
	Invite string `json:"invite,omitempty"`
	// Next is the in-app path to land on afterwards.
	Next string `json:"next,omitempty"`

	ExpiresAt int64 `json:"exp"`
}

// flowCodec signs and verifies the flow cookie.
type flowCodec struct {
	secret []byte
	now    func() time.Time
}

// newFlow starts a sign-in, generating the CSRF `state` and the PKCE verifier.
func (c flowCodec) newFlow(invite, next string) (flowState, error) {
	state, err := NewToken()
	if err != nil {
		return flowState{}, fmt.Errorf("generate state: %w", err)
	}

	return flowState{
		State:     state,
		Verifier:  oauth2.GenerateVerifier(),
		Invite:    invite,
		Next:      next,
		ExpiresAt: c.now().Add(flowTTL).Unix(),
	}, nil
}

// encode renders the flow as a signed cookie value.
//
// Signed, not encrypted: the contents are already private to this browser
// (the cookie is HttpOnly and same-site), and what actually matters is that a
// caller cannot mint or edit one.
func (c flowCodec) encode(f flowState) (string, error) {
	payload, err := json.Marshal(f)
	if err != nil {
		return "", fmt.Errorf("encode flow state: %w", err)
	}

	body := base64.RawURLEncoding.EncodeToString(payload)

	return body + "." + c.sign(body), nil
}

// decode verifies a cookie value and returns the flow it carries.
func (c flowCodec) decode(raw string) (flowState, error) {
	body, signature, found := strings.Cut(raw, ".")
	if !found {
		return flowState{}, ErrInvalidFlow
	}

	// Constant time: a signature check that returns early leaks how much of a
	// forgery was correct.
	if !hmac.Equal([]byte(signature), []byte(c.sign(body))) {
		return flowState{}, ErrInvalidFlow
	}

	payload, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return flowState{}, ErrInvalidFlow
	}

	var f flowState
	if err := json.Unmarshal(payload, &f); err != nil {
		return flowState{}, ErrInvalidFlow
	}

	if c.now().Unix() > f.ExpiresAt {
		return flowState{}, ErrInvalidFlow
	}

	return f, nil
}

func (c flowCodec) sign(body string) string {
	mac := hmac.New(sha256.New, c.secret)
	mac.Write([]byte(body))

	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// safeNextPath keeps the post-sign-in redirect on this origin.
//
// An unsafe value is replaced rather than rejected: a tampered link should
// still sign the user in and land them somewhere sensible, not show them an
// error they cannot act on. What it must never do is bounce them to another
// site with a fresh session in hand.
func safeNextPath(next string) string {
	const home = "/"

	if next == "" {
		return home
	}

	// "//evil.example" and "/\evil.example" are both protocol-relative URLs in
	// a browser, despite looking like local paths.
	if !strings.HasPrefix(next, "/") ||
		strings.HasPrefix(next, "//") ||
		strings.HasPrefix(next, `/\`) ||
		strings.Contains(next, "\\") {
		return home
	}

	return next
}
