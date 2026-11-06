package policy

import (
	"strings"
	"testing"
)

func TestIdentityRules(t *testing.T) {
	name, email, err := ValidateIdentity(" Ana ", " ANA@example.org ")
	if err != nil || name != "Ana" || email != "ana@example.org" {
		t.Fatalf("%q %q %v", name, email, err)
	}
	for _, bad := range []string{"a", strings.Repeat("x", 81), "Ana\nMaria"} {
		if _, _, err := ValidateIdentity(bad, "a@acme.io"); err == nil {
			t.Fatalf("accepted name %q", bad)
		}
	}
	for _, bad := range []string{"bad", "Name <a@acme.io>", "a\x00@acme.io"} {
		if _, _, err := ValidateIdentity("Ana", bad); err == nil {
			t.Fatalf("accepted email %q", bad)
		}
	}
	if ValidatePassword("1234567") == nil || ValidatePassword(strings.Repeat("x", 129)) == nil {
		t.Fatal("bad password length accepted")
	}
	if ValidatePassword("12345678") != nil {
		t.Fatal("valid password rejected")
	}
}
