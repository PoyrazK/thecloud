package services

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
)

type JWTTokenClaims struct {
	UserID    uuid.UUID `json:"user_id"`
	TenantID  uuid.UUID `json:"tenant_id,omitempty"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	SessionID string    `json:"session_id"`
	jwt.RegisteredClaims
}

type JWTService struct {
	signingKey []byte
	expiresIn  time.Duration
}

func NewJWTService(signingKey string) *JWTService {
	if signingKey == "" {
		b := make([]byte, 32)
		rand.Read(b)
		signingKey = hex.EncodeToString(b)
	}
	return &JWTService{
		signingKey: []byte(signingKey),
		expiresIn:  24 * time.Hour,
	}
}

func (s *JWTService) GenerateToken(userID uuid.UUID, tenantID uuid.UUID, email, role string) (string, error) {
	sessionID := uuid.New().String()
	now := time.Now()

	claims := JWTTokenClaims{
		UserID:    userID,
		TenantID:  tenantID,
		Email:     email,
		Role:      role,
		SessionID: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(s.expiresIn)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "thecloud",
			Subject:   userID.String(),
			ID:        sessionID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.signingKey)
}

func (s *JWTService) ValidateToken(tokenString string) (*JWTTokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTTokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return s.signingKey, nil
	})

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*JWTTokenClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}