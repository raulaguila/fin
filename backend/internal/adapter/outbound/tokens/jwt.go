package tokens

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/raulaguila/authenticity/internal/core/port/output"
)

type accessClaims struct {
	Sid string `json:"sid"`
	jwt.RegisteredClaims
}

type refreshClaims struct {
	Sid string `json:"sid"`
	jwt.RegisteredClaims
}

// Issuer signs and parses EdDSA (Ed25519) JWTs: small signatures and a cheap
// verification, which runs on every authenticated request.
type Issuer struct {
	AccessKey     ed25519.PrivateKey
	RefreshKey    ed25519.PrivateKey
	AccessExpire  time.Duration
	RefreshExpire time.Duration
}

var _ output.TokenIssuer = (*Issuer)(nil)

func (i *Issuer) AccessTTL() time.Duration  { return i.AccessExpire }
func (i *Issuer) RefreshTTL() time.Duration { return i.RefreshExpire }

func (i *Issuer) IssueAccess(userID, sessionID string, now time.Time) (string, error) {
	claims := accessClaims{
		Sid: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			ID:        uuid.NewString(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(i.AccessExpire)),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	return t.SignedString(i.AccessKey)
}

func (i *Issuer) IssueRefresh(userID, sessionID string, now time.Time) (token string, jti string, err error) {
	jti = uuid.NewString()
	claims := refreshClaims{
		Sid: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(i.RefreshExpire)),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	token, err = t.SignedString(i.RefreshKey)
	return token, jti, err
}

func (i *Issuer) ParseAccess(token string) (*output.AccessClaims, error) {
	parsed, err := jwt.ParseWithClaims(token, &accessClaims{}, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodEdDSA {
			return nil, fmt.Errorf("unexpected method")
		}
		return i.AccessKey.Public(), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := parsed.Claims.(*accessClaims)
	if !ok || !parsed.Valid {
		return nil, errors.New("invalid access token")
	}
	out := &output.AccessClaims{UserID: claims.Subject, SessionID: claims.Sid}
	if claims.ExpiresAt != nil {
		out.ExpiresAt = claims.ExpiresAt.Time
	}
	return out, nil
}

func (i *Issuer) ParseRefresh(token string) (*output.RefreshClaims, error) {
	parsed, err := jwt.ParseWithClaims(token, &refreshClaims{}, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodEdDSA {
			return nil, fmt.Errorf("unexpected method")
		}
		return i.RefreshKey.Public(), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := parsed.Claims.(*refreshClaims)
	if !ok || !parsed.Valid {
		return nil, errors.New("invalid refresh token")
	}
	out := &output.RefreshClaims{UserID: claims.Subject, SessionID: claims.Sid, JTI: claims.ID}
	if claims.ExpiresAt != nil {
		out.ExpiresAt = claims.ExpiresAt.Time
	}
	return out, nil
}
