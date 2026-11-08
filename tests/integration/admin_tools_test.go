package integration_test

import (
	"context"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/raulaguila/authenticity/internal/adapter/outbound/persistence/postgres/rbac"
	"github.com/raulaguila/authenticity/internal/core/domain"
	"github.com/raulaguila/authenticity/internal/core/port/input"
	"github.com/raulaguila/authenticity/internal/core/usecase/audit"
	"github.com/raulaguila/authenticity/internal/core/usecase/role"
	"github.com/raulaguila/authenticity/pkg/apperror"
	"github.com/raulaguila/authenticity/pkg/reqctx"
)

func TestInviteActivatesAndShowsInTheAuditLog(t *testing.T) {
	e := newEnv(t)
	actor := e.userID(t, e.activeUser(t))
	ctx := reqctx.WithActor(context.Background(), actor)
	addr := email()

	res, err := e.svc.Invite(ctx, input.RegisterCommand{Name: "Bea", Email: addr})
	if err != nil {
		t.Fatal(err)
	}
	_, err = e.svc.Invite(ctx, input.RegisterCommand{Name: "Bea", Email: addr})
	wantCode(t, err, apperror.CodeAlreadyExists)
	if err := e.svc.Activate(ctx, input.ActivateCommand{Token: res.ActivationToken, Password: strongPassword}); err != nil {
		t.Fatal(err)
	}

	records, total, err := audit.New(e.store.Audit()).List(ctx, domain.AuditFilter{Page: domain.NewPage(1, 10), Action: "user.", UserID: res.UserID})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || records[0].Action != "user.invite" || records[0].ActorID != actor || records[0].TargetLabel != addr {
		t.Fatalf("audit %d %+v", total, records)
	}
}

func TestRevokeSessionsEndsAnotherUsersLogins(t *testing.T) {
	e := newEnv(t)
	me, other := e.activeUser(t), e.activeUser(t)
	meID, otherID := e.userID(t, me), e.userID(t, other)
	pair := e.loginAt(t, other, "10.0.0.1")

	wantCode(t, e.users().RevokeSessions(context.Background(), meID, meID), apperror.CodeForbidden)
	if err := e.users().RevokeSessions(context.Background(), meID, otherID); err != nil {
		t.Fatal(err)
	}
	_, err := e.svc.Refresh(context.Background(), input.RefreshCommand{RefreshToken: pair.RefreshToken})
	if err == nil {
		t.Fatal("refresh still works after the sessions were revoked")
	}
}

func TestUserListFiltersByRoleAndSorts(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	roles := role.New(e.store, e.store, rbac.NewRepository(testDB))
	r, err := roles.Create(ctx, "t"+uuid.NewString()[:8])
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = testDB.Exec(`DELETE FROM usr.usr_role WHERE id = ?`, r.ID).Error })
	var emails []string
	for range 3 {
		addr := e.activeUser(t)
		if err := roles.AddMember(ctx, r.ID, e.userID(t, addr)); err != nil {
			t.Fatal(err)
		}
		emails = append(emails, addr)
	}
	e.activeUser(t) // outside the role

	users, total, err := e.users().List(ctx, domain.UserFilter{Page: domain.NewPage(1, 10), RoleID: r.ID, Sort: "email", Order: "desc"})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, u := range users {
		got = append(got, u.Email)
	}
	slices.Sort(emails)
	slices.Reverse(emails)
	if total != 3 || !slices.Equal(got, emails) {
		t.Fatalf("got %d %v, want %v", total, got, emails)
	}
	_, _, err = e.users().List(ctx, domain.UserFilter{Sort: "password_hash"})
	wantCode(t, err, apperror.CodeInvalidInput)
}

func TestRoleListSortsByCreation(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	roles := role.New(e.store, e.store, rbac.NewRepository(testDB))
	prefix := "s" + uuid.NewString()[:6]
	var ids []string
	for _, suffix := range []string{"b", "a"} { // created b first, a second
		r, err := roles.Create(ctx, prefix+suffix)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, r.ID)
		t.Cleanup(func() { _ = testDB.Exec(`DELETE FROM usr.usr_role WHERE id = ?`, r.ID).Error })
	}
	list, _, err := roles.List(ctx, domain.RoleFilter{Page: domain.NewPage(1, 10), Search: prefix, Sort: "created_at", Order: "desc"})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].ID != ids[1] || list[1].ID != ids[0] {
		t.Fatalf("order %+v", list)
	}
}
