package postgres_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"gorm.io/gorm"

	"github.com/raulaguila/authenticity/internal/adapter/outbound/persistence/postgres"
	"github.com/raulaguila/authenticity/internal/core/domain"
	"github.com/raulaguila/authenticity/internal/core/port/input"
	"github.com/raulaguila/authenticity/internal/core/usecase/admin"
	"github.com/raulaguila/authenticity/internal/core/usecase/auth"
	"github.com/raulaguila/authenticity/pkg/aescrypt"
	"github.com/raulaguila/authenticity/pkg/apperror"
	"github.com/raulaguila/authenticity/pkg/password"
	"github.com/raulaguila/authenticity/pkg/tokens"
)

const strongPassword = "correct horse battery"

var (
	testDB  *gorm.DB
	testKey = []byte(strings.Repeat("k", 32))
	keyPair *rsa.PrivateKey
)

// TestMain needs TEST_POSTGRES_URL pointing at an empty database; CI provides one.
func TestMain(m *testing.M) {
	url := os.Getenv("TEST_POSTGRES_URL")
	if url == "" {
		fmt.Println("TEST_POSTGRES_URL not set: skipping Postgres tests")
		return
	}
	if err := migrate(url); err != nil {
		fmt.Println("migrate:", err)
		os.Exit(1)
	}
	var err error
	if testDB, err = postgres.Connect(&postgres.Config{Url: url}); err != nil {
		fmt.Println("connect:", err)
		os.Exit(1)
	}
	if keyPair, err = rsa.GenerateKey(rand.Reader, 2048); err != nil {
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func migrate(url string) error {
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)
	files, err := filepath.Glob("../../../../../deploy/db/migrations/*.up.sql")
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, f := range files {
		sql, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		if _, err := conn.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("%s: %w", f, err)
		}
	}
	return nil
}

type clock struct{ now time.Time }

func (c *clock) Now() time.Time { return c.now }

type env struct {
	svc   *auth.Service
	store *postgres.Store
	clock *clock
}

func newEnv(t *testing.T) *env {
	t.Helper()
	if err := testDB.Exec("DELETE FROM sys.outbox").Error; err != nil {
		t.Fatal(err)
	}
	store := postgres.NewStore(testDB)
	c := &clock{now: time.Now()}
	issuer := &tokens.Issuer{AccessKey: keyPair, RefreshKey: keyPair, AccessExpire: time.Minute, RefreshExpire: time.Hour}
	svc := auth.New(store, store, issuer, c, auth.Config{
		RegistrationTokenTTL:         24 * time.Hour,
		PasswordResetTokenTTL:        2 * time.Hour,
		MaxFailedLoginAttempts:       3,
		AccountLockDuration:          15 * time.Minute,
		AllowManualTokenDelivery:     true,
		LoginAttemptEmailWindow:      15 * time.Minute,
		LoginAttemptEmailMaxFailures: 5,
		LoginAttemptIPWindow:         time.Minute,
		LoginAttemptIPMaxFailures:    20,
		OutboxKey:                    testKey,
	}, nil)
	return &env{svc: svc, store: store, clock: c}
}

func email() string { return "u" + uuid.NewString()[:8] + "@acme.io" }

func (e *env) register(t *testing.T, addr string) *input.RegisterResult {
	t.Helper()
	res, err := e.svc.Register(context.Background(), input.RegisterCommand{Name: "Ana", Email: addr})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func (e *env) activeUser(t *testing.T) string {
	t.Helper()
	addr := email()
	res := e.register(t, addr)
	if err := e.svc.Activate(context.Background(), input.ActivateCommand{Token: res.ActivationToken, Password: strongPassword}); err != nil {
		t.Fatal(err)
	}
	return addr
}

func (e *env) login(addr, pass string) (*domain.TokenPair, error) {
	return e.svc.Login(context.Background(), input.LoginCommand{Email: addr, Password: pass})
}

func (e *env) userStatus(t *testing.T, addr string) string {
	t.Helper()
	u, err := e.store.Users().FindByEmail(context.Background(), addr)
	if err != nil {
		t.Fatal(err)
	}
	return u.Status
}

func wantCode(t *testing.T, err error, code apperror.Code) {
	t.Helper()
	if !apperror.IsCode(err, code) {
		t.Fatalf("want %s, got %v", code, err)
	}
}

func TestExpiredActivationIsRecordedAndRegisterRecovers(t *testing.T) {
	e := newEnv(t)
	addr := email()
	first := e.register(t, addr)

	e.clock.now = e.clock.now.Add(25 * time.Hour)
	err := e.svc.Activate(context.Background(), input.ActivateCommand{Token: first.ActivationToken, Password: strongPassword})
	wantCode(t, err, apperror.CodeTokenExpired)
	if got := e.userStatus(t, addr); got != domain.UserStatusExpired {
		t.Fatalf("expiry rolled back: status %s", got)
	}

	second := e.register(t, addr)
	if second.ActivationToken == "" || second.ActivationToken == first.ActivationToken {
		t.Fatal("no fresh token issued")
	}
	if err := e.svc.Activate(context.Background(), input.ActivateCommand{Token: second.ActivationToken, Password: strongPassword}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.login(addr, strongPassword); err != nil {
		t.Fatal(err)
	}
}

func TestRegisterOfActiveEmailLooksLikeNewAndSendsNothing(t *testing.T) {
	e := newEnv(t)
	addr := e.activeUser(t)
	if err := testDB.Exec("DELETE FROM sys.outbox").Error; err != nil {
		t.Fatal(err)
	}
	res := e.register(t, addr)
	if res.Status != domain.UserStatusPending || res.UserID == "" {
		t.Fatalf("distinguishable answer: %+v", res)
	}
	var queued int64
	if err := testDB.Table("sys.outbox").Count(&queued).Error; err != nil {
		t.Fatal(err)
	}
	if queued != 0 {
		t.Fatalf("%d emails queued for an existing account", queued)
	}
}

func TestLoginGivesSameAnswerForUnusableAccounts(t *testing.T) {
	e := newEnv(t)
	pending := email()
	e.register(t, pending)
	for _, addr := range []string{pending, email()} {
		_, err := e.login(addr, "whatever-password")
		wantCode(t, err, apperror.CodeInvalidCredentials)
	}
}

func TestOutboxHoldsSealedTokenUntilDone(t *testing.T) {
	e := newEnv(t)
	res := e.register(t, email())
	ctx := context.Background()

	msgs, err := e.store.Outbox().ClaimPending(ctx, 10, e.clock.now.Add(time.Second), time.Minute)
	if err != nil || len(msgs) != 1 {
		t.Fatalf("claim: %d %v", len(msgs), err)
	}
	if strings.Contains(string(msgs[0].Payload), res.ActivationToken) {
		t.Fatal("plaintext token in outbox")
	}
	var p struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(msgs[0].Payload, &p); err != nil {
		t.Fatal(err)
	}
	if plain, err := aescrypt.Decrypt(testKey, p.Token); err != nil || plain != res.ActivationToken {
		t.Fatalf("decrypt: %q %v", plain, err)
	}

	if err := e.store.Outbox().MarkSent(ctx, msgs[0].ID, e.clock.now); err != nil {
		t.Fatal(err)
	}
	var left int64
	if err := testDB.Table("sys.outbox").Where("id = ? AND payload ? 'token'", msgs[0].ID).Count(&left).Error; err != nil {
		t.Fatal(err)
	}
	if left != 0 {
		t.Fatal("sealed token kept after send")
	}
}

func TestCrashedClaimReturnsAfterLease(t *testing.T) {
	e := newEnv(t)
	e.register(t, email())
	ctx := context.Background()
	now := e.clock.now.Add(time.Second)

	first, err := e.store.Outbox().ClaimPending(ctx, 10, now, time.Minute)
	if err != nil || len(first) != 1 {
		t.Fatalf("claim: %d %v", len(first), err)
	}
	again, err := e.store.Outbox().ClaimPending(ctx, 10, now.Add(30*time.Second), time.Minute)
	if err != nil || len(again) != 0 {
		t.Fatalf("double claim: %d %v", len(again), err)
	}
	back, err := e.store.Outbox().ClaimPending(ctx, 10, now.Add(2*time.Minute), time.Minute)
	if err != nil || len(back) != 1 {
		t.Fatalf("lost after crash: %d %v", len(back), err)
	}
}

func TestLoginCannotRestoreOldPassword(t *testing.T) {
	e := newEnv(t)
	addr := e.activeUser(t)
	ctx := context.Background()
	u, err := e.store.Users().FindByEmail(ctx, addr)
	if err != nil {
		t.Fatal(err)
	}
	before, err := e.store.Credentials().FindByUserID(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}

	// The interleaving: a login verified the old hash, then a reset committed.
	newHash, err := password.Hash("a brand new password")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.store.Credentials().SetPassword(ctx, u.ID, newHash, domain.PasswordAlgorithmArgon2id); err != nil {
		t.Fatal(err)
	}

	applied, err := e.store.Credentials().RecordSuccess(ctx, u.ID, *before.PasswordHash, e.clock.now)
	if err != nil || applied {
		t.Fatalf("stale login accepted: %v %v", applied, err)
	}
	if _, err := e.store.Credentials().RecordFailure(ctx, u.ID, 3, e.clock.now.Add(time.Hour), e.clock.now); err != nil {
		t.Fatal(err)
	}
	after, err := e.store.Credentials().FindByUserID(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if *after.PasswordHash != newHash {
		t.Fatal("old password restored")
	}
}

func TestConcurrentFailuresAreAllCounted(t *testing.T) {
	e := newEnv(t)
	addr := e.activeUser(t)
	ctx := context.Background()
	u, err := e.store.Users().FindByEmail(ctx, addr)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			if _, err := e.store.Credentials().RecordFailure(ctx, u.ID, 100, e.clock.now.Add(time.Hour), e.clock.now); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	cred, err := e.store.Credentials().FindByUserID(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if cred.FailedLoginAttempts != 10 {
		t.Fatalf("counted %d of 10", cred.FailedLoginAttempts)
	}
}

func TestBlockedRetriesDoNotExtendTheBlock(t *testing.T) {
	e := newEnv(t)
	addr := email()
	for range 5 {
		_, err := e.login(addr, "wrong-password")
		wantCode(t, err, apperror.CodeInvalidCredentials)
	}
	_, err := e.login(addr, "wrong-password") // blocked; used to count as a new failure
	wantCode(t, err, apperror.CodeTooManyRequests)

	if err := testDB.Exec(`UPDATE usr.usr_login_attempt SET created_at = now() - INTERVAL '16 minutes'
		WHERE email = ? AND failure_kind = 'user_not_found'`, addr).Error; err != nil {
		t.Fatal(err)
	}
	_, err = e.login(addr, "wrong-password")
	wantCode(t, err, apperror.CodeInvalidCredentials)
}

func TestLockLapsesAndCounterRestarts(t *testing.T) {
	e := newEnv(t)
	addr := e.activeUser(t)

	for range 2 {
		_, err := e.login(addr, "wrong-password")
		wantCode(t, err, apperror.CodeInvalidCredentials)
	}
	_, err := e.login(addr, "wrong-password") // the lock itself must look like any other failure
	wantCode(t, err, apperror.CodeInvalidCredentials)
	if got := e.userStatus(t, addr); got != domain.UserStatusLocked {
		t.Fatalf("status %s", got)
	}
	_, err = e.login(addr, strongPassword)
	wantCode(t, err, apperror.CodeInvalidCredentials)

	e.clock.now = e.clock.now.Add(16 * time.Minute)
	_, err = e.login(addr, "wrong-password") // a typo after the lock must not relock at once
	wantCode(t, err, apperror.CodeInvalidCredentials)
	if got := e.userStatus(t, addr); got != domain.UserStatusActive {
		t.Fatalf("status %s", got)
	}
	if _, err := e.login(addr, strongPassword); err != nil {
		t.Fatal(err)
	}
}

func TestLockedAccountCanResetPassword(t *testing.T) {
	e := newEnv(t)
	addr := e.activeUser(t)
	ctx := context.Background()
	for range 3 {
		_, _ = e.login(addr, "wrong-password")
	}

	res, err := e.svc.RequestPasswordReset(ctx, input.RequestPasswordResetCommand{Email: addr})
	if err != nil || res.ResetToken == "" {
		t.Fatalf("no reset for locked account: %+v %v", res, err)
	}
	const newPassword = "a brand new password"
	if err := e.svc.ConfirmPasswordReset(ctx, input.ConfirmPasswordResetCommand{Token: res.ResetToken, Password: newPassword}); err != nil {
		t.Fatal(err)
	}
	if got := e.userStatus(t, addr); got != domain.UserStatusActive {
		t.Fatalf("status %s", got)
	}
	if _, err := e.login(addr, newPassword); err != nil {
		t.Fatal(err)
	}
}

func TestResetRevokesSessionsAndTokenIsSingleUse(t *testing.T) {
	e := newEnv(t)
	addr := e.activeUser(t)
	ctx := context.Background()
	pair, err := e.login(addr, strongPassword)
	if err != nil {
		t.Fatal(err)
	}
	res, err := e.svc.RequestPasswordReset(ctx, input.RequestPasswordResetCommand{Email: addr})
	if err != nil {
		t.Fatal(err)
	}
	cmd := input.ConfirmPasswordResetCommand{Token: res.ResetToken, Password: "a brand new password"}
	if err := e.svc.ConfirmPasswordReset(ctx, cmd); err != nil {
		t.Fatal(err)
	}
	wantCode(t, e.svc.ConfirmPasswordReset(ctx, cmd), apperror.CodeTokenExpired)

	_, err = e.svc.Refresh(ctx, input.RefreshCommand{RefreshToken: pair.RefreshToken})
	wantCode(t, err, apperror.CodeUnauthorized)
}

func TestOldRefreshTokenRevokesSessionAtAnyAge(t *testing.T) {
	e := newEnv(t)
	addr := e.activeUser(t)
	ctx := context.Background()
	gen0, err := e.login(addr, strongPassword)
	if err != nil {
		t.Fatal(err)
	}
	gen1, err := e.svc.Refresh(ctx, input.RefreshCommand{RefreshToken: gen0.RefreshToken})
	if err != nil {
		t.Fatal(err)
	}
	gen2, err := e.svc.Refresh(ctx, input.RefreshCommand{RefreshToken: gen1.RefreshToken})
	if err != nil {
		t.Fatal(err)
	}

	_, err = e.svc.Refresh(ctx, input.RefreshCommand{RefreshToken: gen0.RefreshToken}) // two generations old
	wantCode(t, err, apperror.CodeUnauthorized)
	_, err = e.svc.Refresh(ctx, input.RefreshCommand{RefreshToken: gen2.RefreshToken})
	wantCode(t, err, apperror.CodeUnauthorized)
}

func TestLockedAccountKeepsItsSession(t *testing.T) {
	e := newEnv(t)
	addr := e.activeUser(t)
	pair, err := e.login(addr, strongPassword)
	if err != nil {
		t.Fatal(err)
	}
	for range 3 {
		_, _ = e.login(addr, "wrong-password")
	}
	if got := e.userStatus(t, addr); got != domain.UserStatusLocked {
		t.Fatalf("status %s", got)
	}
	if _, err := e.svc.Refresh(context.Background(), input.RefreshCommand{RefreshToken: pair.RefreshToken}); err != nil {
		t.Fatalf("attacker logged the owner out: %v", err)
	}
}

func TestRefreshRaceLoserKeepsSessionButReplayLaterRevokes(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	gen0, err := e.login(e.activeUser(t), strongPassword)
	if err != nil {
		t.Fatal(err)
	}
	gen1, err := e.svc.Refresh(ctx, input.RefreshCommand{RefreshToken: gen0.RefreshToken})
	if err != nil {
		t.Fatal(err)
	}

	_, err = e.svc.Refresh(ctx, input.RefreshCommand{RefreshToken: gen0.RefreshToken}) // the other tab
	wantCode(t, err, apperror.CodeConflict)
	gen2, err := e.svc.Refresh(ctx, input.RefreshCommand{RefreshToken: gen1.RefreshToken})
	if err != nil {
		t.Fatalf("race cost the session: %v", err)
	}

	e.clock.now = e.clock.now.Add(11 * time.Second)
	_, err = e.svc.Refresh(ctx, input.RefreshCommand{RefreshToken: gen1.RefreshToken}) // replay outside the window
	wantCode(t, err, apperror.CodeUnauthorized)
	_, err = e.svc.Refresh(ctx, input.RefreshCommand{RefreshToken: gen2.RefreshToken})
	wantCode(t, err, apperror.CodeUnauthorized)
}

func TestEmailCooldownLimitsTokensPerAccount(t *testing.T) {
	e := newEnv(t)
	e.svc.Cfg.EmailTokenCooldown = time.Minute
	ctx := context.Background()
	addr := email()
	first := e.register(t, addr)

	if again := e.register(t, addr); again.ActivationToken != "" {
		t.Fatal("second token inside the cooldown")
	}
	if err := testDB.Exec(`UPDATE usr.usr_token SET created_at = now() - INTERVAL '2 minutes'`).Error; err != nil {
		t.Fatal(err)
	}
	if later := e.register(t, addr); later.ActivationToken == "" {
		t.Fatal("no token after the cooldown")
	}
	err := e.svc.Activate(ctx, input.ActivateCommand{Token: first.ActivationToken, Password: strongPassword})
	if err == nil {
		t.Fatal("replaced token still works")
	}

	active := e.activeUser(t)
	res, err := e.svc.RequestPasswordReset(ctx, input.RequestPasswordResetCommand{Email: active})
	if err != nil || res.ResetToken == "" {
		t.Fatalf("first reset: %+v %v", res, err)
	}
	if res, err = e.svc.RequestPasswordReset(ctx, input.RequestPasswordResetCommand{Email: active}); err != nil || res.ResetToken != "" {
		t.Fatalf("second reset inside the cooldown: %+v %v", res, err)
	}
}

type noCache struct{ input.Authorizer }

func (noCache) Invalidate(context.Context, string) error { return nil }

func TestPromoteGivesAdminRoleOnlyToActiveUsers(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	svc := admin.New(e.store, e.clock, noCache{})

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

	if err := admin.New(e.store, e.clock, noCache{}).Disable(ctx, addr); err != nil {
		t.Fatal(err)
	}
	sess, err := e.store.Sessions().FindByID(ctx, claims.SessionID)
	if err != nil || sess.RevokedAt == nil {
		t.Fatalf("session alive: %+v %v", sess, err)
	}
	_, err = e.login(addr, strongPassword)
	wantCode(t, err, apperror.CodeInvalidCredentials)
}

func TestConcurrentRefreshHasOneWinnerAtMost(t *testing.T) {
	e := newEnv(t)
	addr := e.activeUser(t)
	pair, err := e.login(addr, strongPassword)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	wins := 0
	for range 8 {
		wg.Go(func() {
			if _, err := e.svc.Refresh(context.Background(), input.RefreshCommand{RefreshToken: pair.RefreshToken}); err == nil {
				mu.Lock()
				wins++
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	if wins > 1 {
		t.Fatalf("%d refreshes won with one token", wins)
	}
}

func TestCleanupRemovesAbandonedSignups(t *testing.T) {
	e := newEnv(t)
	abandoned, resumed, activated, fresh := email(), email(), e.activeUser(t), email()
	for _, addr := range []string{abandoned, resumed, fresh} {
		e.register(t, addr)
	}
	err := testDB.Exec(`UPDATE usr.usr_user SET created_at = now() - INTERVAL '31 days' WHERE email IN (?)`,
		[]string{abandoned, resumed, activated}).Error
	if err != nil {
		t.Fatal(err)
	}
	err = testDB.Exec(`UPDATE usr.usr_token SET expires_at = now() - INTERVAL '1 hour'
		WHERE user_id = (SELECT id FROM usr.usr_user WHERE email = ?)`, abandoned).Error
	if err != nil {
		t.Fatal(err)
	}
	if err := testDB.Exec(`SELECT sys.fn_cleanup()`).Error; err != nil {
		t.Fatal(err)
	}
	var left []string
	all := []string{abandoned, resumed, activated, fresh}
	if err := testDB.Table("usr.usr_user").Where("email IN ?", all).Pluck("email", &left).Error; err != nil {
		t.Fatal(err)
	}
	sort.Strings(left)
	want := []string{resumed, activated, fresh}
	sort.Strings(want)
	if strings.Join(left, ",") != strings.Join(want, ",") {
		t.Fatalf("left %v, want %v", left, want)
	}
}

func TestLogoutIsIdempotent(t *testing.T) {
	e := newEnv(t)
	pair, err := e.login(e.activeUser(t), strongPassword)
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := e.svc.Logout(context.Background(), input.LogoutCommand{AccessToken: pair.AccessToken}); err != nil {
			t.Fatal(err)
		}
	}
	_, err = e.svc.Refresh(context.Background(), input.RefreshCommand{RefreshToken: pair.RefreshToken})
	wantCode(t, err, apperror.CodeUnauthorized)
}

func TestCleanupRemovesOnlyOldHistory(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	old, recent := email(), email()
	for _, addr := range []string{old, recent} {
		if err := e.store.Attempts().Create(ctx, &domain.LoginAttempt{Email: addr, Success: true}); err != nil {
			t.Fatal(err)
		}
	}
	if err := testDB.Exec(`UPDATE usr.usr_login_attempt SET created_at = now() - INTERVAL '31 days' WHERE email = ?`, old).Error; err != nil {
		t.Fatal(err)
	}
	if err := testDB.Exec(`SELECT sys.fn_cleanup()`).Error; err != nil {
		t.Fatal(err)
	}
	var left []string
	if err := testDB.Table("usr.usr_login_attempt").Where("email IN ?", []string{old, recent}).Pluck("email", &left).Error; err != nil {
		t.Fatal(err)
	}
	if len(left) != 1 || left[0] != recent {
		t.Fatalf("left %v", left)
	}
}
