package auth

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/raulaguila/authenticity/internal/core/domain"
	"github.com/raulaguila/authenticity/internal/core/port/output"
	"github.com/raulaguila/authenticity/pkg/apperror"
	"github.com/raulaguila/authenticity/pkg/tokenhash"
)

func (s *Service) issueSession(ctx context.Context, userID, verifiedHash string, ip, ua *string, now time.Time) (*domain.TokenPair, error) {
	// Pre-generate session id so JWT sid matches the row on insert.
	sessionID := uuid.NewString()
	refresh, jti, err := s.Tokens.IssueRefresh(userID, sessionID, now)
	if err != nil {
		return nil, apperror.New(apperror.CodeInternal, "token issue failed").WithCause(err)
	}
	access, err := s.Tokens.IssueAccess(userID, sessionID, now)
	if err != nil {
		return nil, apperror.New(apperror.CodeInternal, "token issue failed").WithCause(err)
	}

	expiresAt := now.Add(min(s.Tokens.RefreshTTL(), s.Cfg.SessionMaxLifetime))
	sess := &domain.Session{
		ID:               sessionID,
		UserID:           userID,
		ExpiresAt:        expiresAt,
		RefreshTokenHash: tokenhash.SHA256Bytes(jti),
		IPAddress:        ip,
		UserAgent:        ua,
		CreatedAt:        now,
		LastSeenAt:       now,
	}
	// RecordSuccess locks the credential row until session insertion commits.
	// Reset updates that same row before revoking sessions, so it either wins
	// first (the hash check fails) or revokes this new session after we commit.
	if err := s.Tx.WithinTx(ctx, func(ctx context.Context, uow output.UnitOfWork) error {
		applied, err := uow.Credentials().RecordSuccess(ctx, userID, verifiedHash, now)
		if err != nil {
			return err
		}
		if !applied {
			return invalidCredentials()
		}
		return uow.Sessions().Create(ctx, sess)
	}); err != nil {
		return nil, err
	}

	// Disable may have run during the slow verify and missed this session. Read
	// after the insert, so whichever of the two comes last sees the other.
	user, err := s.UOW.Users().FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user.Status == domain.UserStatusDisabled {
		s.warn("Session not revoked", s.UOW.Sessions().Revoke(ctx, sessionID, now, domain.RevokeAccountDisabled))
		return nil, invalidCredentials()
	}

	return &domain.TokenPair{
		UserID:       userID,
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    int64(s.Tokens.AccessTTL().Seconds()),
		TokenType:    "Bearer",
	}, nil
}
