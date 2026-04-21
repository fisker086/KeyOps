package auth

import (
	"strings"
	"testing"
)

func TestNormalizeSSONextPath(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", "/"},
		{"/", "/"},
		{"/dashboard", "/dashboard"},
		{"//evil.com", "/"},
		{"https://evil.com/x", "/"},
		{"http://x", "/"},
		{"/a?b=1", "/a?b=1"},
	}
	for _, tc := range tests {
		got := normalizeSSONextPath(tc.in)
		if got != tc.want {
			t.Errorf("normalizeSSONextPath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestEncodeDecodeSSOStateRoundTrip(t *testing.T) {
	s, err := encodeSSOState("/reports")
	if err != nil {
		t.Fatal(err)
	}
	if got := decodeSSONextFromState(s); got != "/reports" {
		t.Errorf("got %q, want /reports", got)
	}
}

func TestDecodeSSONextFromState_LegacyUUID(t *testing.T) {
	u := "550e8400-e29b-41d4-a716-446655440000"
	if got := decodeSSONextFromState(u); got != "/" {
		t.Errorf("legacy uuid should map to default, got %q", got)
	}
}

func TestRedirectLoginWithSSOToken(t *testing.T) {
	r := redirectLoginWithSSOToken("/dash", "tok_x")
	if !strings.HasPrefix(r, "/login?") {
		t.Fatalf("unexpected: %s", r)
	}
	if !strings.Contains(r, "sso_token=tok_x") || !strings.Contains(r, "next=") {
		t.Fatalf("missing params: %s", r)
	}
}
