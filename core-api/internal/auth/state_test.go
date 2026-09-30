package auth

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func testCodec() flowCodec {
	return flowCodec{secret: []byte(testSecret), now: time.Now}
}

func TestFlowCookieRoundTrip(t *testing.T) {
	t.Parallel()

	codec := testCodec()

	flow, err := codec.newFlow("invite-token", "/people")
	if err != nil {
		t.Fatalf("newFlow: %v", err)
	}

	if flow.State == "" || flow.Verifier == "" {
		t.Fatal("a new flow must carry both a state and a PKCE verifier")
	}

	cookie, err := codec.encode(flow)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	got, err := codec.decode(cookie)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if got != flow {
		t.Errorf("flow = %+v, want %+v", got, flow)
	}
}

func TestFlowCookieRejections(t *testing.T) {
	t.Parallel()

	codec := testCodec()

	flow, err := codec.newFlow("", "/")
	if err != nil {
		t.Fatalf("newFlow: %v", err)
	}

	valid, err := codec.encode(flow)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	body, signature, _ := strings.Cut(valid, ".")

	// Signed with a different key: a cookie minted by anyone else.
	foreign, err := flowCodec{secret: []byte(strings.Repeat("z", 40)), now: time.Now}.encode(flow)
	if err != nil {
		t.Fatalf("encode with other key: %v", err)
	}

	cases := map[string]string{
		"empty":            "",
		"no signature":     body,
		"edited payload":   body + "x." + signature,
		"edited signature": body + "." + signature + "x",
		"signed elsewhere": foreign,
		"not base64":       "!!!." + signature,
	}

	for name, cookie := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if _, err := codec.decode(cookie); !errors.Is(err, ErrInvalidFlow) {
				t.Errorf("decode(%q) error = %v, want ErrInvalidFlow", name, err)
			}
		})
	}
}

func TestFlowCookieExpires(t *testing.T) {
	t.Parallel()

	issued := flowCodec{
		secret: []byte(testSecret),
		now:    func() time.Time { return time.Now().Add(-time.Hour) },
	}

	flow, err := issued.newFlow("", "/")
	if err != nil {
		t.Fatalf("newFlow: %v", err)
	}

	cookie, err := issued.encode(flow)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	// The signature is still perfectly valid; only the clock has moved. An
	// abandoned sign-in must not stay redeemable.
	if _, err := testCodec().decode(cookie); !errors.Is(err, ErrInvalidFlow) {
		t.Errorf("decode error = %v, want ErrInvalidFlow", err)
	}
}

func TestSafeNextPathKeepsRedirectsLocal(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"":                           "/",
		"/":                          "/",
		"/people":                    "/people",
		"/settings/family?tab=join":  "/settings/family?tab=join",
		"//evil.example":             "/",
		`/\evil.example`:             "/",
		"https://evil.example":       "/",
		"http://evil.example/people": "/",
		`/people\..\..`:              "/",
		"people":                     "/",
	}

	for input, want := range cases {
		t.Run(input, func(t *testing.T) {
			t.Parallel()

			if got := safeNextPath(input); got != want {
				t.Errorf("safeNextPath(%q) = %q, want %q", input, got, want)
			}
		})
	}
}
