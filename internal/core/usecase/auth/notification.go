package auth

import (
	"context"

	"github.com/raulaguila/authenticity/internal/core/domain"
)

// notify is best effort: a lost notice must not cost the login or refresh it reports on.
func (s *Service) notify(ctx context.Context, userID, level, title, message string) {
	s.warn("Notification lost", s.UOW.Notifications().Create(ctx, &domain.Notification{UserID: userID, Level: level, Title: title, Message: message}))
}

// noticeNewClient warns about a login from a client this account never used.
func (s *Service) noticeNewClient(ctx context.Context, userID string, ip *string) {
	if ip == nil || *ip == "" {
		return
	}
	known, err := s.UOW.Sessions().KnownClient(ctx, userID, *ip)
	if err != nil {
		s.warn("New client check failed", err)
		return
	}
	if !known {
		s.notify(ctx, userID, domain.NotificationInfo, "New login", "New login from "+*ip+".")
	}
}
