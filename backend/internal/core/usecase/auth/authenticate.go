package auth

import (
	"context"

	"github.com/raulaguila/authenticity/internal/core/domain"
	"github.com/raulaguila/authenticity/pkg/apperror"
)

func (s *Service) Logout(ctx context.Context, sessionID string) error {
	return s.UOW.Sessions().Revoke(ctx, sessionID, s.Clock.Now(), domain.RevokeUserLogout)
}

// Authenticate checks the token and its session on every request, so a revoked
// session stops at once instead of when the token expires.
func (s *Service) Authenticate(ctx context.Context, accessToken string) (*domain.Principal, error) {
	claims, err := s.Tokens.ParseAccess(accessToken)
	if err != nil {
		return nil, apperror.New(apperror.CodeUnauthorized, "invalid access token").WithCause(err)
	}
	if claims.UserID == "" || claims.SessionID == "" {
		return nil, apperror.New(apperror.CodeUnauthorized, "invalid access token")
	}
	now := s.Clock.Now()
	if !claims.ExpiresAt.IsZero() && !claims.ExpiresAt.After(now) {
		return nil, apperror.New(apperror.CodeTokenExpired, "access token expired")
	}
	sess, err := s.UOW.Sessions().FindLive(ctx, claims.SessionID)
	if apperror.IsNotFound(err) {
		return nil, apperror.New(apperror.CodeUnauthorized, "session not found")
	}
	if err != nil {
		return nil, err
	}
	switch {
	case sess.UserID != claims.UserID:
		return nil, apperror.New(apperror.CodeUnauthorized, "session mismatch")
	case sess.RevokedAt != nil:
		return nil, apperror.New(apperror.CodeUnauthorized, "session revoked")
	case !sess.ExpiresAt.After(now):
		return nil, apperror.New(apperror.CodeTokenExpired, "session expired")
	}
	return &domain.Principal{UserID: claims.UserID, SessionID: claims.SessionID}, nil
}
