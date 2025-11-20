package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewJWTManager(t *testing.T) {
	// Test case 1: Valid parameters
	manager, err := NewJWTManager("secret", time.Hour, "test-issuer")
	assert.NoError(t, err)
	assert.NotNil(t, manager)

	// Test case 2: Empty secret key
	manager, err = NewJWTManager("", time.Hour, "test-issuer")
	assert.Error(t, err)
	assert.Nil(t, manager)
	assert.Equal(t, "JWT secret key is required", err.Error())

	// Test case 3: Default expiration
	manager, err = NewJWTManager("secret", 0, "test-issuer")
	assert.NoError(t, err)
	assert.Equal(t, 24*time.Hour, manager.expiration)
}

func TestJWTManager_GenerateToken(t *testing.T) {
	manager, _ := NewJWTManager("secret", time.Hour, "test-issuer")

	token, err := manager.GenerateToken("user123", "testuser", []string{"read", "write"})
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	// Validate the generated token
	claims, err := manager.ValidateToken(token)
	assert.NoError(t, err)
	assert.Equal(t, "user123", claims.UserID)
	assert.Equal(t, "testuser", claims.Username)
	assert.Equal(t, []string{"read", "write"}, claims.Scopes)
	assert.Equal(t, "test-issuer", claims.Issuer)
}

func TestJWTManager_ValidateToken_Invalid(t *testing.T) {
	manager, _ := NewJWTManager("secret", time.Hour, "test-issuer")

	// Test invalid token string
	_, err := manager.ValidateToken("invalid-token")
	assert.Error(t, err)

	// Test token signed with different key
	otherManager, _ := NewJWTManager("other-secret", time.Hour, "test-issuer")
	token, _ := otherManager.GenerateToken("user1", "user", nil)
	_, err = manager.ValidateToken(token)
	assert.Error(t, err)
}

func TestGenerateSecureSecret(t *testing.T) {
	secret, err := GenerateSecureSecret(32)
	assert.NoError(t, err)
	assert.NotEmpty(t, secret)
	assert.True(t, len(secret) >= 32)
}
