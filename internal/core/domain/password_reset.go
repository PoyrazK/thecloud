package domain

import (
	"time"

	"github.com/google/uuid"
)

// PasswordResetToken represents a token used to reset a user's password
type PasswordResetToken struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	TokenHash string    `json:"-"` // Stored as hash, never returned
	ExpiresAt time.Time `json:"expires_at"`
	Used      bool      `json:"used"`
	CreatedAt time.Time `json:"created_at"`
}
