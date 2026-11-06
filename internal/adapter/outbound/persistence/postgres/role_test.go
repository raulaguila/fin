package postgres_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/raulaguila/authenticity/internal/adapter/outbound/persistence/postgres/rbac"
	"github.com/raulaguila/authenticity/internal/core/domain"
	"github.com/raulaguila/authenticity/internal/core/usecase/admin"
	"github.com/raulaguila/authenticity/internal/core/usecase/authorize"
	"github.com/raulaguila/authenticity/internal/core/usecase/role"
	"github.com/raulaguila/authenticity/pkg/apperror"
)

func findRole(t *testing.T, svc *role.Service, name string) domain.Role {
	t.Helper()
	roles, err := svc.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range roles {
		if r.Name == name {
			return r
		}
	}
	t.Fatalf("role %s missing", name)
	return domain.Role{}
}

func TestAdminRoleCannotBeChangedThroughTheAPI(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	svc := role.New(e.store, rbac.NewRepository(testDB))
	before := findRole(t, svc, "admin")

	addr := e.activeUser(t)
	if err := admin.New(e.store, e.clock).Promote(ctx, addr); err != nil {
		t.Fatal(err)
	}
	userID := e.userID(t, addr)

	_, err := svc.SetPermission(ctx, before.ID, domain.ResourceProducts, domain.ActionRead)
	wantCode(t, err, apperror.CodeForbidden)
	_, err = svc.SetPermission(ctx, before.ID, domain.ResourceProducts, 0)
	wantCode(t, err, apperror.CodeForbidden)
	wantCode(t, svc.Delete(ctx, before.ID), apperror.CodeForbidden)
	wantCode(t, svc.AddMember(ctx, before.ID, e.userID(t, e.activeUser(t))), apperror.CodeForbidden)
	wantCode(t, svc.RemoveMember(ctx, before.ID, userID), apperror.CodeForbidden)

	after := findRole(t, svc, "admin")
	if !after.IsSuperuser || len(after.Permissions) != 1 || after.Permissions[domain.ResourceAll] != before.Permissions[domain.ResourceAll] {
		t.Fatalf("admin changed: %+v", after)
	}
	eff, err := rbac.NewRepository(testDB).LoadEffective(ctx, userID)
	if err != nil || !eff.IsSuperuser {
		t.Fatalf("the member must stay admin: %+v %v", eff, err)
	}
}

func TestSentinelGrantMakesARoleProtected(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	svc := role.New(e.store, rbac.NewRepository(testDB))
	created, err := svc.Create(ctx, "t"+uuid.NewString()[:8])
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = testDB.Exec("DELETE FROM usr.usr_role WHERE id = ?", created.ID).Error })

	_, err = svc.SetPermission(ctx, created.ID, domain.ResourceAll, domain.ActionRead)
	wantCode(t, err, apperror.CodeNotFound) // the API never grants "*"

	if err := testDB.Exec(`
		INSERT INTO usr.usr_role_permission (role_id, resource, actions) VALUES (?, '*', 1)
	`, created.ID).Error; err != nil {
		t.Fatal(err)
	}
	wantCode(t, svc.AddMember(ctx, created.ID, e.userID(t, e.activeUser(t))), apperror.CodeForbidden)
}

func TestRolePermissionsApplyAtOnce(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	perms := rbac.NewRepository(testDB)
	svc := role.New(e.store, perms)
	authz := authorize.New(perms)
	userID := e.userID(t, e.activeUser(t))

	created, err := svc.Create(ctx, "t"+uuid.NewString()[:8])
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = testDB.Exec("DELETE FROM usr.usr_role WHERE id = ?", created.ID).Error })
	_, err = svc.Create(ctx, created.Name)
	wantCode(t, err, apperror.CodeAlreadyExists)
	_, err = svc.Create(ctx, "Not Valid")
	wantCode(t, err, apperror.CodeInvalidInput)

	wantCode(t, authz.Authorize(ctx, userID, domain.ResourceProducts, domain.ActionRead), apperror.CodeForbidden)

	updated, err := svc.SetPermission(ctx, created.ID, domain.ResourceProducts, domain.ActionRead|domain.ActionUpdate)
	if err != nil {
		t.Fatal(err)
	}
	if got := updated.Permissions[domain.ResourceProducts]; got != domain.ActionRead|domain.ActionUpdate {
		t.Fatalf("actions %d", got)
	}
	if err := svc.AddMember(ctx, created.ID, userID); err != nil {
		t.Fatal(err)
	}
	if err := authz.Authorize(ctx, userID, domain.ResourceProducts, domain.ActionUpdate); err != nil {
		t.Fatal(err)
	}
	wantCode(t, authz.Authorize(ctx, userID, domain.ResourceProducts, domain.ActionDelete), apperror.CodeForbidden)

	members, total, err := svc.Members(ctx, created.ID, domain.NewPage(1, 20))
	if err != nil || total != 1 || len(members) != 1 || members[0].ID != userID {
		t.Fatalf("members %+v %d %v", members, total, err)
	}

	wantCode(t, svc.Delete(ctx, created.ID), apperror.CodeResourceInUse)
	updated, err = svc.SetPermission(ctx, created.ID, domain.ResourceProducts, 0)
	if err != nil || len(updated.Permissions) != 0 {
		t.Fatalf("permissions %+v %v", updated, err)
	}
	wantCode(t, authz.Authorize(ctx, userID, domain.ResourceProducts, domain.ActionRead), apperror.CodeForbidden)

	_, err = svc.SetPermission(ctx, created.ID, "unknown", domain.ActionRead)
	wantCode(t, err, apperror.CodeNotFound)
	wantCode(t, svc.AddMember(ctx, created.ID, uuid.NewString()), apperror.CodeNotFound)

	if err := svc.RemoveMember(ctx, created.ID, userID); err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	wantCode(t, svc.Delete(ctx, created.ID), apperror.CodeNotFound)
}

func TestDefaultRoleCanChangeButNotLeave(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	svc := role.New(e.store, rbac.NewRepository(testDB))
	def := findRole(t, svc, "user")

	wantCode(t, svc.Delete(ctx, def.ID), apperror.CodeConflict)
	if _, err := svc.SetPermission(ctx, def.ID, domain.ResourceProducts, domain.ActionRead); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetPermission(ctx, def.ID, domain.ResourceProducts, 0); err != nil {
		t.Fatal(err)
	}
}
