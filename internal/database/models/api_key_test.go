package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestAPIKey_Basic(t *testing.T) {
	key := &APIKey{
		ID:        uuid.New(),
		Name:      "test-api-key",
		KeyHash:   "hashed-key-value",
		Scopes:    []string{"diagnostics:read", "diagnostics:write"},
		CreatedAt: time.Now(),
	}

	assert.NotEqual(t, uuid.Nil, key.ID)
	assert.Equal(t, "test-api-key", key.Name)
	assert.Equal(t, "hashed-key-value", key.KeyHash)
	assert.Len(t, key.Scopes, 2)
}

func TestAPIKey_WithExpiry(t *testing.T) {
	now := time.Now()
	future := now.Add(30 * 24 * time.Hour) // 30 days

	key := &APIKey{
		ID:        uuid.New(),
		Name:      "expiring-key",
		KeyHash:   "hash",
		ExpiresAt: &future,
		CreatedAt: now,
	}

	assert.NotNil(t, key.ExpiresAt)
	assert.True(t, key.ExpiresAt.After(now))
}

func TestAPIKey_LastUsed(t *testing.T) {
	now := time.Now()
	key := &APIKey{
		ID:         uuid.New(),
		Name:       "active-key",
		KeyHash:    "hash",
		LastUsedAt: &now,
		CreatedAt:  now.Add(-7 * 24 * time.Hour),
	}

	assert.NotNil(t, key.LastUsedAt)
	assert.True(t, key.LastUsedAt.After(key.CreatedAt))
}

func TestAPIKey_Scopes(t *testing.T) {
	tests := []struct {
		name   string
		scopes []string
	}{
		{
			name:   "ReadOnly",
			scopes: []string{"diagnostics:read", "agents:read"},
		},
		{
			name:   "FullAccess",
			scopes: []string{"diagnostics:read", "diagnostics:write", "agents:read", "agents:write", "admin"},
		},
		{
			name:   "Empty",
			scopes: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := &APIKey{
				ID:      uuid.New(),
				Name:    tt.name,
				KeyHash: "hash",
				Scopes:  tt.scopes,
			}

			assert.Equal(t, len(tt.scopes), len(key.Scopes))
			if len(tt.scopes) > 0 {
				assert.Equal(t, tt.scopes, key.Scopes)
			}
		})
	}
}

func TestAPIKey_IsExpired(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		expiresAt *time.Time
		expected  bool
	}{
		{
			name:      "NotExpired",
			expiresAt: timePtr(now.Add(24 * time.Hour)),
			expected:  false,
		},
		{
			name:      "Expired",
			expiresAt: timePtr(now.Add(-24 * time.Hour)),
			expected:  true,
		},
		{
			name:      "NoExpiry",
			expiresAt: nil,
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := &APIKey{
				ExpiresAt: tt.expiresAt,
			}

			// Manual expiry check
			if key.ExpiresAt != nil {
				isExpired := key.ExpiresAt.Before(now)
				assert.Equal(t, tt.expected, isExpired)
			} else {
				assert.False(t, tt.expected)
			}
		})
	}
}
