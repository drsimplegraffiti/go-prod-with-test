package utils

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TokenType distinguishes access tokens from refresh tokens so a refresh
// token can never be used to authenticate an API call, and vice versa.
type TokenType string

const (
	AccessToken  TokenType = "access"
	RefreshToken TokenType = "refresh"
)

// ErrInvalidToken is returned for any malformed, expired, or wrong-type token.
var ErrInvalidToken = errors.New("invalid or expired token")

// Claims is the JWT payload used across the service.
type Claims struct {
	UserID string    `json:"uid"`
	Role   string    `json:"role"`
	Type   TokenType `json:"typ"`
	jwt.RegisteredClaims
}

// JWTManager issues and validates HS256 JWTs.
type JWTManager struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
	issuer     string
}

// NewJWTManager builds a manager bound to a secret and token lifetimes.
func NewJWTManager(secret string, accessTTL, refreshTTL time.Duration) *JWTManager {
	return &JWTManager{
		secret:     []byte(secret),
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
		issuer:     "goapi",
	}
}

// GenerateAccessToken issues a short-lived token used to authenticate API requests.
func (m *JWTManager) GenerateAccessToken(userID, role string) (string, time.Time, error) {
	return m.generate(userID, role, AccessToken, m.accessTTL)
}

// GenerateRefreshToken issues a long-lived token used solely to obtain new access tokens.
func (m *JWTManager) GenerateRefreshToken(userID, role string) (string, time.Time, error) {
	return m.generate(userID, role, RefreshToken, m.refreshTTL)
}

func (m *JWTManager) generate(userID, role string, typ TokenType, ttl time.Duration) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(ttl)

	claims := Claims{
		UserID: userID,
		Role:   role,
		Type:   typ,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign token: %w", err)
	}
	return signed, expiresAt, nil
}

// Verify parses and validates a token, ensuring its signature, expiry, and
// declared type all match expectations.
func (m *JWTManager) Verify(tokenString string, expectedType TokenType) (*Claims, error) {
	claims := &Claims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}))

	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}

	if claims.Type != expectedType {
		return nil, ErrInvalidToken
	}

	return claims, nil
}
