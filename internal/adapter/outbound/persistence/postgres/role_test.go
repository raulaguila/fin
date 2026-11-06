package postgres_test

import (
	"context"
	"sync"
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
	roles, err := svc.UOW.Roles().List(context.Background(), 1000) // the repository also sees protected roles
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
	svc := role.New(e.store, e.store, rbac.NewRepository(testDB))
	before := findRole(t, svc, "admin")

	addr := e.activeUser(t)
	if err := admin.New(e.store, e.clock).Promote(ctx, addr); err != nil {
		t.Fatal(err)
	}
	userID := e.userID(t, addr)

	_, err := svc.SetPermissions(ctx, before.ID, map[string]domain.Action{domain.ResourceProducts: domain.ActionRead})
	wantCode(t, err, apperror.CodeForbidden)
	_, err = svc.SetPermissions(ctx, before.ID, map[string]domain.Action{})
	wantCode(t, err, apperror.CodeForbidden)
	name, enabled := "renamed", false
	_, err = svc.Update(ctx, before.ID, domain.RolePatch{Name: &name})
	wantCode(t, err, apperror.CodeForbidden)
	_, err = svc.Update(ctx, before.ID, domain.RolePatch{Enabled: &enabled})
	wantCode(t, err, apperror.CodeForbidden)
	wantCode(t, svc.Delete(ctx, before.ID), apperror.CodeForbidden)
	wantCode(t, svc.AddMember(ctx, before.ID, e.userID(t, e.activeUser(t))), apperror.CodeForbidden)
	wantCode(t, svc.RemoveMember(ctx, before.ID, userID), apperror.CodeForbidden)

	after := findRole(t, svc, "admin")
	if !after.IsSuperuser || !after.Enabled || len(after.Permissions) != 1 || after.Permissions[domain.ResourceAll] != before.Permissions[domain.ResourceAll] {
		t.Fatalf("admin changed: %+v", after)
	}
	eff, err := rbac.NewRepository(testDB).LoadEffective(ctx, userID)
	if err != nil || !eff.IsSuperuser {
		t.Fatalf("the member must stay admin: %+v %v", eff, err)
	}
}

func TestListHidesProtectedRoles(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	svc := role.New(e.store, e.store, rbac.NewRepository(testDB))
	sentinel := newRole(t, svc)
	if err := testDB.Exec(`
		INSERT INTO usr.usr_role_permission (role_id, resource, actions) VALUES (?, '*', 1)
	`, sentinel.ID).Error; err != nil {
		t.Fatal(err)
	}

	roles, err := svc.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, r := range roles {
		found[r.Name] = true
	}
	if !found["user"] || found["admin"] || found[sentinel.Name] {
		t.Fatalf("listed %v", found)
	}
}

func TestSentinelGrantMakesARoleProtected(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	svc := role.New(e.store, e.store, rbac.NewRepository(testDB))
	created, err := svc.Create(ctx, "t"+uuid.NewString()[:8])
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = testDB.Exec("DELETE FROM usr.usr_role WHERE id = ?", created.ID).Error })

	_, err = svc.SetPermissions(ctx, created.ID, map[string]domain.Action{domain.ResourceAll: domain.ActionRead})
	wantCode(t, err, apperror.CodeInvalidInput) // the API never grants "*"

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
	svc := role.New(e.store, e.store, perms)
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

	updated, err := svc.SetPermissions(ctx, created.ID, map[string]domain.Action{
		domain.ResourceProducts: domain.ActionRead | domain.ActionUpdate,
		domain.ResourceRoles:    0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.Permissions) != 1 || updated.Permissions[domain.ResourceProducts] != domain.ActionRead|domain.ActionUpdate {
		t.Fatalf("permissions %v", updated.Permissions)
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
	updated, err = svc.SetPermissions(ctx, created.ID, map[string]domain.Action{})
	if err != nil || len(updated.Permissions) != 0 {
		t.Fatalf("permissions %+v %v", updated, err)
	}
	wantCode(t, authz.Authorize(ctx, userID, domain.ResourceProducts, domain.ActionRead), apperror.CodeForbidden)

	_, err = svc.SetPermissions(ctx, created.ID, map[string]domain.Action{"unknown": domain.ActionRead})
	wantCode(t, err, apperror.CodeInvalidInput)
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
	svc := role.New(e.store, e.store, rbac.NewRepository(testDB))
	def := findRole(t, svc, "user")

	wantCode(t, svc.Delete(ctx, def.ID), apperror.CodeConflict)
	off := false
	_, err := svc.Update(ctx, def.ID, domain.RolePatch{Enabled: &off})
	wantCode(t, err, apperror.CodeConflict)
	if _, err := svc.SetPermissions(ctx, def.ID, map[string]domain.Action{domain.ResourceProducts: domain.ActionRead}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetPermissions(ctx, def.ID, map[string]domain.Action{}); err != nil {
		t.Fatal(err)
	}
}

func newRole(t *testing.T, svc *role.Service) *domain.Role {
	t.Helper()
	created, err := svc.Create(context.Background(), "t"+uuid.NewString()[:8])
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = testDB.Exec("DELETE FROM usr.usr_role WHERE id = ?", created.ID).Error })
	return created
}

func TestFailedSaveKeepsThePreviousPermissions(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	svc := role.New(e.store, e.store, rbac.NewRepository(testDB))
	created := newRole(t, svc)
	if _, err := svc.SetPermissions(ctx, created.ID, map[string]domain.Action{"users": domain.ActionRead}); err != nil {
		t.Fatal(err)
	}

	// A negative mask only fails at the database, after the old rows are gone.
	_, err := svc.SetPermissions(ctx, created.ID, map[string]domain.Action{
		domain.ResourceProducts: domain.ActionRead,
		domain.ResourceRoles:    domain.Action(-1),
	})
	if err == nil {
		t.Fatal("a negative mask must fail")
	}
	got := findRole(t, svc, created.Name)
	if len(got.Permissions) != 1 || got.Permissions["users"] != domain.ActionRead {
		t.Fatalf("permissions changed: %v", got.Permissions)
	}
}

func TestConcurrentSavesEndWithOneOfThem(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	svc := role.New(e.store, e.store, rbac.NewRepository(testDB))
	created := newRole(t, svc)

	var wg sync.WaitGroup
	errs := make([]error, 8)
	for i := range errs {
		wg.Go(func() {
			_, errs[i] = svc.SetPermissions(ctx, created.ID, map[string]domain.Action{
				domain.ResourceProducts: domain.ActionRead,
				domain.ResourceRoles:    domain.ActionRead,
			})
		})
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if got := findRole(t, svc, created.Name); len(got.Permissions) != 2 {
		t.Fatalf("permissions %v", got.Permissions)
	}
}

func TestUpdateChangesOnlyWhatItCarries(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	svc := role.New(e.store, e.store, rbac.NewRepository(testDB))
	created := newRole(t, svc)

	off := false
	updated, err := svc.Update(ctx, created.ID, domain.RolePatch{Enabled: &off})
	if err != nil || updated.Enabled || updated.Name != created.Name {
		t.Fatalf("%+v %v", updated, err)
	}
	name := created.Name + "-x"
	updated, err = svc.Update(ctx, created.ID, domain.RolePatch{Name: &name})
	if err != nil || updated.Name != name || updated.Enabled {
		t.Fatalf("%+v %v", updated, err)
	}

	bad, taken := "Not Valid", "user"
	_, err = svc.Update(ctx, created.ID, domain.RolePatch{Name: &bad})
	wantCode(t, err, apperror.CodeInvalidInput)
	_, err = svc.Update(ctx, created.ID, domain.RolePatch{Name: &taken})
	wantCode(t, err, apperror.CodeAlreadyExists)
	_, err = svc.Update(ctx, uuid.NewString(), domain.RolePatch{Name: &name})
	wantCode(t, err, apperror.CodeNotFound)
}
