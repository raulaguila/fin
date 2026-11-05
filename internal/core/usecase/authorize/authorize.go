package authorize

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/raulaguila/authenticity/internal/core/domain"
	"github.com/raulaguila/authenticity/internal/core/port/input"
	"github.com/raulaguila/authenticity/internal/core/port/output"
	"github.com/raulaguila/authenticity/pkg/apperror"
)

const cacheKeyPrefix = "authz:effective:"

// UseCase loads effective permissions (cache → DB) and checks CRUD bits.
type UseCase struct {
	Reader output.PermissionReader
	Cache  output.CacheStore
	TTL    time.Duration
}

var _ input.Authorizer = (*UseCase)(nil)

func New(reader output.PermissionReader, cache output.CacheStore, ttl time.Duration) *UseCase {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	return &UseCase{Reader: reader, Cache: cache, TTL: ttl}
}

func (u *UseCase) cacheKey(userID string) string {
	return cacheKeyPrefix + userID
}

func (u *UseCase) LoadEffective(ctx context.Context, userID string) (*domain.EffectivePermissions, error) {
	if userID == "" {
		return nil, apperror.New(apperror.CodeInvalidInput, "user id required")
	}

	if u.Cache != nil {
		if raw, err := u.Cache.Get(ctx, u.cacheKey(userID)); err == nil && len(raw) > 0 {
			var cached cachedPermissions
			if json.Unmarshal(raw, &cached) == nil {
				return cached.toDomain(userID), nil
			}
		}
	}

	if u.Reader == nil {
		return nil, apperror.New(apperror.CodeInternal, "permission reader not configured")
	}

	eff, err := u.Reader.LoadEffective(ctx, userID)
	if err != nil {
		return nil, err
	}

	if u.Cache != nil && eff != nil {
		payload, err := json.Marshal(fromDomain(eff))
		if err == nil {
			_ = u.Cache.Set(ctx, u.cacheKey(userID), payload, u.TTL)
		}
	}

	return eff, nil
}

func (u *UseCase) Authorize(ctx context.Context, userID, resource string, need domain.Action) error {
	if resource == "" {
		return apperror.New(apperror.CodeInvalidInput, "resource required")
	}
	if need == 0 {
		return apperror.New(apperror.CodeInvalidInput, "action required")
	}

	eff, err := u.LoadEffective(ctx, userID)
	if err != nil {
		return err
	}
	if eff == nil {
		return apperror.New(apperror.CodeForbidden, "forbidden")
	}
	if eff.IsSuperuser {
		return nil
	}
	if _, ok := eff.ByResource[domain.ResourceAll]; ok {
		return nil
	}
	if have, ok := eff.ByResource[resource]; ok && domain.Allows(have, need) {
		return nil
	}
	return apperror.New(apperror.CodeForbidden, fmt.Sprintf("missing %s permission on %s", needLabel(need), resource))
}

func (u *UseCase) Invalidate(ctx context.Context, userID string) error {
	if u.Cache == nil || userID == "" {
		return nil
	}
	return u.Cache.Del(ctx, u.cacheKey(userID))
}

type cachedPermissions struct {
	IsSuperuser bool             `json:"is_superuser"`
	ByResource  map[string]int32 `json:"by_resource"`
}

func fromDomain(e *domain.EffectivePermissions) cachedPermissions {
	m := make(map[string]int32, len(e.ByResource))
	for k, v := range e.ByResource {
		m[k] = int32(v)
	}
	return cachedPermissions{IsSuperuser: e.IsSuperuser, ByResource: m}
}

func (c cachedPermissions) toDomain(userID string) *domain.EffectivePermissions {
	m := make(map[string]domain.Action, len(c.ByResource))
	for k, v := range c.ByResource {
		m[k] = domain.Action(v)
	}
	return &domain.EffectivePermissions{
		UserID:      userID,
		IsSuperuser: c.IsSuperuser,
		ByResource:  m,
	}
}

func needLabel(need domain.Action) string {
	switch {
	case domain.Allows(need, domain.ActionCreate):
		return "create"
	case domain.Allows(need, domain.ActionRead):
		return "read"
	case domain.Allows(need, domain.ActionUpdate):
		return "update"
	case domain.Allows(need, domain.ActionDelete):
		return "delete"
	default:
		return "action"
	}
}
