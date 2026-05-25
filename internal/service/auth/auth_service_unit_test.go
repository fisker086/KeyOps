package auth

import "testing"

func TestParseAdminWhitelist(t *testing.T) {
	got := parseAdminWhitelist(" admin@example.com,invalid,OPS@EXAMPLE.COM, ,user@x.io")
	if len(got) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(got))
	}
	if got[0] != "admin@example.com" || got[1] != "ops@example.com" || got[2] != "user@x.io" {
		t.Fatalf("unexpected whitelist: %#v", got)
	}
}

func TestIsAdminEmailWhitelisted(t *testing.T) {
	svc := &AuthService{adminWhitelist: []string{"ops@example.com"}}
	if !svc.isAdminEmailWhitelisted(" OPS@EXAMPLE.COM ") {
		t.Fatal("expected email to be whitelisted")
	}
	if svc.isAdminEmailWhitelisted("dev@example.com") {
		t.Fatal("did not expect email to be whitelisted")
	}
}

func TestExtract32BytesForAES(t *testing.T) {
	long := []byte("abcdefghijklmnopqrstuvwxyz0123456789")
	k := extract32BytesForAES(long)
	if len(k) != 32 {
		t.Fatalf("expected 32-byte key, got %d", len(k))
	}
	if string(k) != string(long[:32]) {
		t.Fatal("expected first 32 bytes for long key")
	}

	short := []byte("short-key")
	k2 := extract32BytesForAES(short)
	if len(k2) != 32 {
		t.Fatalf("expected 32-byte hash key, got %d", len(k2))
	}
}
