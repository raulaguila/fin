package postgres_test

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/raulaguila/authenticity/internal/core/domain"
	"github.com/raulaguila/authenticity/internal/core/usecase/admin"
	"github.com/raulaguila/authenticity/pkg/apperror"
)

func TestPromoteGivesAdminRoleOnlyToActiveUsers(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	svc := admin.New(e.store, e.clock)

	pending := email()
	e.register(t, pending)
	wantCode(t, svc.Promote(ctx, pending), apperror.CodeConflict)
	wantCode(t, svc.Promote(ctx, email()), apperror.CodeNotFound)

	addr := e.activeUser(t)
	if err := svc.Promote(ctx, "  "+strings.ToUpper(addr)); err != nil {
		t.Fatal(err)
	}
	var n int64
	err := testDB.Table("usr.usr_user_role ur").
		Joins("JOIN usr.usr_role r ON r.id = ur.role_id JOIN usr.usr_user u ON u.id = ur.user_id").
		Where("u.email = ? AND r.name = 'admin'", addr).Count(&n).Error
	if err != nil || n != 1 {
		t.Fatalf("admin roles: %d %v", n, err)
	}
}

func TestDisableEndsSessionsAtOnce(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	addr := e.activeUser(t)
	pair, err := e.login(addr, strongPassword)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := e.svc.Tokens.ParseAccess(pair.AccessToken)
	if err != nil {
		t.Fatal(err)
	}

	if err := admin.New(e.store, e.clock).Disable(ctx, addr); err != nil {
		t.Fatal(err)
	}
	sess, err := e.store.Sessions().FindByID(ctx, claims.SessionID)
	if err != nil || sess.RevokedAt == nil {
		t.Fatalf("session alive: %+v %v", sess, err)
	}
	_, err = e.login(addr, strongPassword)
	wantCode(t, err, apperror.CodeInvalidCredentials)
}

func TestBootstrapCreatesOneWorkingAdmin(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	if err := testDB.Exec(`DELETE FROM usr.usr_user_role WHERE role_id = (SELECT id FROM usr.usr_role WHERE name = 'admin')`).Error; err != nil {
		t.Fatal(err)
	}
	svc := admin.New(e.store, e.clock)

	taken := e.activeUser(t)
	_, err := svc.Bootstrap(ctx, admin.BootstrapCommand{Name: "Root", Email: taken, Password: strongPassword})
	wantCode(t, err, apperror.CodeConflict)
	_, err = svc.Bootstrap(ctx, admin.BootstrapCommand{Name: "Root", Email: email(), Password: "short"})
	wantCode(t, err, apperror.CodeInvalidInput)

	const runs = 4
	addrs := make([]string, runs)
	created := make([]bool, runs)
	var wg sync.WaitGroup
	for i := range runs {
		addrs[i] = email()
		wg.Go(func() {
			var err error
			created[i], err = svc.Bootstrap(ctx, admin.BootstrapCommand{Name: "Root", Email: addrs[i], Password: strongPassword})
			if err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()

	winner := ""
	for i, ok := range created {
		if ok && winner != "" {
			t.Fatal("two runs each created an admin")
		}
		if ok {
			winner = addrs[i]
		}
	}
	if winner == "" {
		t.Fatal("no run created the admin")
	}
	pair, err := e.login(winner, strongPassword)
	if err != nil {
		t.Fatal(err)
	}
	profile, err := e.account().Profile(ctx, pair.UserID)
	if err != nil {
		t.Fatal(err)
	}
	if !profile.IsSuperuser || profile.User.Status != domain.UserStatusActive {
		t.Fatalf("got %+v", profile)
	}
}
