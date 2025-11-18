package models

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// APIKey represents an API key for authentication
type APIKey struct {
	ID         uuid.UUID  `json:"id"`
	KeyHash    string     `json:"-"` // Never expose the hash in JSON
	Name       string     `json:"name"`
	Scopes     []string   `json:"scopes"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	Revoked    bool       `json:"revoked"`
	Metadata   JSONB      `json:"metadata,omitempty"`
}

// GenerateAPIKey generates a new random API key
// Returns the plain-text key (to show to user once) and the hash (to store in DB)
func GenerateAPIKey() (plainKey string, keyHash string, err error) {
	// Generate 32 random bytes (256 bits)
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return "", "", fmt.Errorf("failed to generate random key: %w", err)
	}

	// Encode as base64 for the plain key
	plainKey = "lumo_" + base64.URLEncoding.EncodeToString(keyBytes)

	// Hash the plain key with SHA-256
	hash := sha256.Sum256([]byte(plainKey))
	keyHash = fmt.Sprintf("%x", hash)

	return plainKey, keyHash, nil
}

// HashAPIKey hashes an API key for storage/comparison
func HashAPIKey(plainKey string) string {
	hash := sha256.Sum256([]byte(plainKey))
	return fmt.Sprintf("%x", hash)
}

// IsExpired returns true if the key has expired
func (k *APIKey) IsExpired() bool {
	if k.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*k.ExpiresAt)
}

// IsValid returns true if the key is valid (not revoked and not expired)
func (k *APIKey) IsValid() bool {
	return !k.Revoked && !k.IsExpired()
}

// HasScope checks if the API key has a specific scope
func (k *APIKey) HasScope(scope string) bool {
	for _, s := range k.Scopes {
		if s == "*" || s == scope {
			return true
		}
	}
	return false
}

// UpdateLastUsed updates the last_used_at timestamp
func (k *APIKey) UpdateLastUsed() {
	now := time.Now()
	k.LastUsedAt = &now
}
