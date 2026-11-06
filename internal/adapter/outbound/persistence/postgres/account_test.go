package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/raulaguila/authenticity/internal/core/domain"
	"github.com/raulaguila/authenticity/internal/core/port/input"
	"github.com/raulaguila/authenticity/internal/core/usecase/admin"
	"github.com/raulaguila/authenticity/pkg/apperror"
)

func TestProfileHasLivePermissionsAndDefaults(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	addr := e.activeUser(t)
	id := e.userID(t, addr)

	profile, err := e.account().Profile(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if profile.IsSuperuser || profile.User.Email != addr {
		t.Fatalf("got %+v", profile)
	}
	if profile.Preferences.Theme != domain.ThemeSystem || profile.Preferences.Language != domain.DefaultLanguage {
		t.Fatalf("defaults: %+v", profile.Preferences)
	}

	if err := admin.New(e.store, e.clock).Promote(ctx, addr); err != nil {
		t.Fatal(err)
	}
	if profile, err = e.account().Profile(ctx, id); err != nil || !profile.IsSuperuser {
		t.Fatalf("promotion not seen: %+v %v", profile, err)
	}
}

func TestPreferencesSaveAndValidate(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	id := e.userID(t, e.activeUser(t))
	svc := e.account()

	_, err := svc.UpdatePreferences(ctx, domain.Preferences{UserID: id, Theme: "neon", Language: "pt-BR"})
	wantCode(t, err, apperror.CodeInvalidInput)
	_, err = svc.UpdatePreferences(ctx, domain.Preferences{UserID: id, Theme: "dark", Language: "portuguese"})
	wantCode(t, err, apperror.CodeInvalidInput)

	for _, want := range []domain.Preferences{
		{UserID: id, Theme: domain.ThemeDark, Language: "en"},
		{UserID: id, Theme: domain.ThemeLight, Language: "en-US"},
	} {
		if _, err := svc.UpdatePreferences(ctx, want); err != nil {
			t.Fatal(err)
		}
		if got, err := svc.Preferences(ctx, id); err != nil || *got != want {
			t.Fatalf("got %+v, %v", got, err)
		}
	}
}

func TestNotificationsReportSecurityEvents(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	addr := e.activeUser(t)
	id := e.userID(t, addr)

	first := e.loginAt(t, addr, "203.0.113.1")
	e.loginAt(t, addr, "203.0.113.1")
	if got := e.titles(t, id); len(got) != 0 {
		t.Fatalf("first and repeated client: %v", got)
	}
	e.loginAt(t, addr, "198.51.100.7")
	if got := e.titles(t, id); len(got) != 1 || got[0] != "New login" {
		t.Fatalf("new client: %v", got)
	}

	if _, err := e.svc.Refresh(ctx, input.RefreshCommand{RefreshToken: first.RefreshToken}); err != nil {
		t.Fatal(err)
	}
	e.clock.now = e.clock.now.Add(time.Minute)
	if _, err := e.svc.Refresh(ctx, input.RefreshCommand{RefreshToken: first.RefreshToken}); err == nil {
		t.Fatal("replay accepted")
	}
	if got := e.titles(t, id); len(got) != 2 || got[1] != "Session ended" {
		t.Fatalf("reuse: %v", got)
	}

	for range e.svc.Cfg.MaxFailedLoginAttempts {
		_, _ = e.login(addr, "wrong password")
	}
	if got := e.titles(t, id); len(got) != 3 || got[0] != "Account locked" {
		t.Fatalf("lock: %v", got)
	}

	reset, err := e.svc.RequestPasswordReset(ctx, input.RequestPasswordResetCommand{Email: addr})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.svc.ConfirmPasswordReset(ctx, input.ConfirmPasswordResetCommand{Token: reset.ResetToken, Password: strongPassword + "2"}); err != nil {
		t.Fatal(err)
	}
	if got := e.titles(t, id); len(got) != 4 || got[2] != "Password changed" {
		t.Fatalf("reset: %v", got)
	}
}

func TestNotificationsStayWithTheirOwner(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	svc := e.account()
	id, strangerID := e.userID(t, e.activeUser(t)), e.userID(t, e.activeUser(t))
	for range 3 {
		n := &domain.Notification{UserID: id, Level: domain.NotificationInfo, Title: "t", Message: "m"}
		if err := e.store.Notifications().Create(ctx, n); err != nil {
			t.Fatal(err)
		}
	}
	items, total, err := svc.Notifications(ctx, id, domain.NewPage(2, 2))
	if err != nil || total != 3 || len(items) != 1 {
		t.Fatalf("page 2: %d of %d, %v", len(items), total, err)
	}

	wantCode(t, svc.MarkRead(ctx, strangerID, items[0].ID), apperror.CodeNotFound)
	for range 2 { // idempotent
		if err := svc.MarkRead(ctx, id, items[0].ID); err != nil {
			t.Fatal(err)
		}
	}
	if unread, err := svc.CountUnread(ctx, id); err != nil || unread != 2 {
		t.Fatalf("unread %d, %v", unread, err)
	}
	if unread, err := svc.CountUnread(ctx, strangerID); err != nil || unread != 0 {
		t.Fatalf("stranger unread %d, %v", unread, err)
	}
}
