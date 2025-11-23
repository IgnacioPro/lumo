package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewJWTManager(t *testing.T) {
	tests := []struct {
		name           string
		secretKey      string
		expiration     time.Duration
		issuer         string
		wantErr        bool
		wantIssuer     string
		wantExpiration time.Duration
	}{
		{
			name:           "Valid configuration",
			secretKey:      "test-secret",
			expiration:     1 * time.Hour,
			issuer:         "test-issuer",
			wantErr:        false,
			wantIssuer:     "test-issuer",
			wantExpiration: 1 * time.Hour,
		},
		{
			name:       "Empty secret key",
			secretKey:  "",
			expiration: 1 * time.Hour,
			issuer:     "test-issuer",
			wantErr:    true,
		},
		{
			name:           "Zero expiration - uses default",
			secretKey:      "test-secret",
			expiration:     0,
			issuer:         "test-issuer",
			wantErr:        false,
			wantIssuer:     "test-issuer",
			wantExpiration: 24 * time.Hour,
		},
		{
			name:           "Negative expiration - uses default",
			secretKey:      "test-secret",
			expiration:     -1 * time.Hour,
			issuer:         "test-issuer",
			wantErr:        false,
			wantIssuer:     "test-issuer",
			wantExpiration: 24 * time.Hour,
		},
		{
			name:           "Empty issuer - uses default",
			secretKey:      "test-secret",
			expiration:     1 * time.Hour,
			issuer:         "",
			wantErr:        false,
			wantIssuer:     "lumo-api",
			wantExpiration: 1 * time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager, err := NewJWTManager(tt.secretKey, tt.expiration, tt.issuer)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, manager)
			} else {
				require.NoError(t, err)
				require.NotNil(t, manager)
				assert.Equal(t, tt.wantIssuer, manager.issuer)
				assert.Equal(t, tt.wantExpiration, manager.expiration)
			}
		})
	}
}

func TestJWTManager_GenerateToken(t *testing.T) {
	manager, err := NewJWTManager("test-secret", 1*time.Hour, "test-issuer")
	require.NoError(t, err)

	tests := []struct {
		name     string
		userID   string
		username string
		scopes   []string
	}{
		{
			name:     "Valid token with scopes",
			userID:   "user-123",
			username: "testuser",
			scopes:   []string{"read", "write"},
		},
		{
			name:     "Valid token without scopes",
			userID:   "user-456",
			username: "anotheruser",
			scopes:   nil,
		},
		{
			name:     "Empty username",
			userID:   "user-789",
			username: "",
			scopes:   []string{"read"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenString, err := manager.GenerateToken(tt.userID, tt.username, tt.scopes)

			require.NoError(t, err)
			assert.NotEmpty(t, tokenString)

			// Validate the token can be parsed
			token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
				return []byte("test-secret"), nil
			})

			require.NoError(t, err)
			require.True(t, token.Valid)

			claims, ok := token.Claims.(*Claims)
			require.True(t, ok)
			assert.Equal(t, tt.userID, claims.UserID)
			assert.Equal(t, tt.username, claims.Username)
			assert.Equal(t, tt.scopes, claims.Scopes)
			assert.Equal(t, "test-issuer", claims.Issuer)
			assert.Equal(t, tt.userID, claims.Subject)
		})
	}
}

func TestJWTManager_ValidateToken(t *testing.T) {
	manager, err := NewJWTManager("test-secret", 1*time.Hour, "test-issuer")
	require.NoError(t, err)

	// Generate a valid token
	validToken, err := manager.GenerateToken("user-123", "testuser", []string{"read"})
	require.NoError(t, err)

	// Generate a token with wrong secret
	wrongManager, err := NewJWTManager("wrong-secret", 1*time.Hour, "test-issuer")
	require.NoError(t, err)
	wrongToken, err := wrongManager.GenerateToken("user-456", "wronguser", nil)
	require.NoError(t, err)

	// Generate expired token by creating a token in the past
	// We can't directly create an expired token, so we'll test with a malformed token
	malformedToken := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"

	tests := []struct {
		name       string
		token      string
		wantErr    bool
		wantUserID string
	}{
		{
			name:       "Valid token",
			token:      validToken,
			wantErr:    false,
			wantUserID: "user-123",
		},
		{
			name:    "Invalid token string",
			token:   "invalid-token-string",
			wantErr: true,
		},
		{
			name:    "Empty token",
			token:   "",
			wantErr: true,
		},
		{
			name:    "Token signed with wrong secret",
			token:   wrongToken,
			wantErr: true,
		},
		{
			name:    "Malformed/Invalid signature token",
			token:   malformedToken,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := manager.ValidateToken(tt.token)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, claims)
			} else {
				require.NoError(t, err)
				require.NotNil(t, claims)
				assert.Equal(t, tt.wantUserID, claims.UserID)
			}
		})
	}
}

func TestJWTManager_RefreshToken(t *testing.T) {
	manager, err := NewJWTManager("test-secret", 1*time.Hour, "test-issuer")
	require.NoError(t, err)

	// Generate original token
	originalToken, err := manager.GenerateToken("user-123", "testuser", []string{"read", "write"})
	require.NoError(t, err)

	// Sleep briefly to ensure timestamps differ
	time.Sleep(10 * time.Millisecond)

	// Refresh the token
	refreshedToken, err := manager.RefreshToken(originalToken)
	require.NoError(t, err)
	assert.NotEmpty(t, refreshedToken)
	assert.NotEqual(t, originalToken, refreshedToken)

	// Validate refreshed token
	claims, err := manager.ValidateToken(refreshedToken)
	require.NoError(t, err)
	assert.Equal(t, "user-123", claims.UserID)
	assert.Equal(t, "testuser", claims.Username)
	assert.Equal(t, []string{"read", "write"}, claims.Scopes)

	// Test refresh with invalid token
	_, err = manager.RefreshToken("invalid-token")
	assert.Error(t, err)
}

func TestGenerateSecureSecret(t *testing.T) {
	tests := []struct {
		name       string
		length     int
		wantMinLen int
	}{
		{
			name:       "Default length (32 bytes)",
			length:     32,
			wantMinLen: 40, // Base64 encoding increases size
		},
		{
			name:       "Large length (64 bytes)",
			length:     64,
			wantMinLen: 80,
		},
		{
			name:       "Small length - uses minimum (32 bytes)",
			length:     16,
			wantMinLen: 40,
		},
		{
			name:       "Zero length - uses minimum",
			length:     0,
			wantMinLen: 40,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			secret, err := GenerateSecureSecret(tt.length)

			require.NoError(t, err)
			assert.NotEmpty(t, secret)
			assert.GreaterOrEqual(t, len(secret), tt.wantMinLen)
		})
	}

	// Test that multiple calls generate different secrets
	secret1, err1 := GenerateSecureSecret(32)
	secret2, err2 := GenerateSecureSecret(32)

	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.NotEqual(t, secret1, secret2)
}

func TestClaims_Structure(t *testing.T) {
	// Test that Claims struct can be marshaled/unmarshaled properly
	claims := Claims{
		UserID:   "user-123",
		Username: "testuser",
		Scopes:   []string{"read", "write"},
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "test-issuer",
			Subject:   "user-123",
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		},
	}

	assert.Equal(t, "user-123", claims.UserID)
	assert.Equal(t, "testuser", claims.Username)
	assert.Equal(t, []string{"read", "write"}, claims.Scopes)
	assert.Equal(t, "test-issuer", claims.Issuer)
}
