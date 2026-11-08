package tokens

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"
)

func issuer(t *testing.T) *Issuer {
	t.Helper()
	access, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	refresh, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return &Issuer{AccessKey: access, RefreshKey: refresh, AccessExpire: time.Minute, RefreshExpire: time.Hour}
}

func TestTokensRoundTripAndDoNotCross(t *testing.T) {
	i := issuer(t)
	now := time.Now()
	access, err := i.IssueAccess("u", "s", now)
	if err != nil {
		t.Fatal(err)
	}
	refresh, jti, err := i.IssueRefresh("u", "s", now)
	if err != nil {
		t.Fatal(err)
	}

	if c, err := i.ParseAccess(access); err != nil || c.UserID != "u" || c.SessionID != "s" {
		t.Fatalf("access: %+v %v", c, err)
	}
	if c, err := i.ParseRefresh(refresh); err != nil || c.JTI != jti {
		t.Fatalf("refresh: %+v %v", c, err)
	}
	if _, err := i.ParseAccess(refresh); err == nil {
		t.Fatal("refresh token accepted as access")
	}
	if _, err := i.ParseRefresh(access); err == nil {
		t.Fatal("access token accepted as refresh")
	}
}

func TestExpiredAndForeignTokensRefused(t *testing.T) {
	i := issuer(t)
	old, err := i.IssueAccess("u", "s", time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := i.ParseAccess(old); err == nil {
		t.Fatal("expired token accepted")
	}
	foreign, err := issuer(t).IssueAccess("u", "s", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := i.ParseAccess(foreign); err == nil {
		t.Fatal("foreign signature accepted")
	}
}
