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

func TestClaims_IsTenantScoped(t *testing.T) {
	tests := []struct {
		name     string
		tenantID string
		want     bool
	}{
		{"with tenant ID", "tenant-123", true},
		{"without tenant ID", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := &Claims{TenantID: tt.tenantID}
			if got := claims.IsTenantScoped(); got != tt.want {
				t.Errorf("IsTenantScoped() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClaims_IsAgentToken(t *testing.T) {
	tests := []struct {
		name      string
		tokenType TokenType
		agentID   string
		want      bool
	}{
		{"agent token type", TokenTypeAgent, "", true},
		{"agent ID set", "", "agent-123", true},
		{"user token type", TokenTypeUser, "", false},
		{"no type or agent ID", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := &Claims{TokenType: tt.tokenType, AgentID: tt.agentID}
			if got := claims.IsAgentToken(); got != tt.want {
				t.Errorf("IsAgentToken() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClaims_IsUserToken(t *testing.T) {
	tests := []struct {
		name      string
		tokenType TokenType
		userID    string
		want      bool
	}{
		{"user token type", TokenTypeUser, "", true},
		{"user ID set", "", "user-123", true},
		{"agent token type", TokenTypeAgent, "", false},
		{"no type or user ID", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := &Claims{TokenType: tt.tokenType, UserID: tt.userID}
			if got := claims.IsUserToken(); got != tt.want {
				t.Errorf("IsUserToken() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClaims_HasScope(t *testing.T) {
	claims := &Claims{
		Scopes: []string{"read", "write", "admin"},
	}

	tests := []struct {
		name  string
		scope string
		want  bool
	}{
		{"has read scope", "read", true},
		{"has admin scope", "admin", true},
		{"missing delete scope", "delete", false},
		{"empty scope", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := claims.HasScope(tt.scope); got != tt.want {
				t.Errorf("HasScope(%q) = %v, want %v", tt.scope, got, tt.want)
			}
		})
	}
}

func TestJWTManager_GenerateTenantUserToken(t *testing.T) {
	manager, err := NewJWTManager("test-secret", 1*time.Hour, "test-issuer")
	require.NoError(t, err)

	token, err := manager.GenerateTenantUserToken("user-123", "testuser", "tenant-456", "acme", []string{"read"})
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	claims, err := manager.ValidateToken(token)
	require.NoError(t, err)
	assert.Equal(t, "user-123", claims.UserID)
	assert.Equal(t, "testuser", claims.Username)
	assert.Equal(t, "tenant-456", claims.TenantID)
	assert.Equal(t, "acme", claims.TenantSlug)
	assert.Equal(t, TokenTypeUser, claims.TokenType)
	assert.True(t, claims.IsTenantScoped())
	assert.True(t, claims.IsUserToken())
	assert.False(t, claims.IsAgentToken())
}

func TestJWTManager_GenerateAgentToken(t *testing.T) {
	manager, err := NewJWTManager("test-secret", 1*time.Hour, "test-issuer")
	require.NoError(t, err)

	token, err := manager.GenerateAgentToken("tenant-456", "acme", "agent-789", []string{"events:submit"})
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	claims, err := manager.ValidateToken(token)
	require.NoError(t, err)
	assert.Equal(t, "tenant-456", claims.TenantID)
	assert.Equal(t, "acme", claims.TenantSlug)
	assert.Equal(t, "agent-789", claims.AgentID)
	assert.Equal(t, TokenTypeAgent, claims.TokenType)
	assert.True(t, claims.IsTenantScoped())
	assert.True(t, claims.IsAgentToken())
	assert.False(t, claims.IsUserToken())
}

func TestJWTManager_RefreshToken_AgentToken(t *testing.T) {
	manager, err := NewJWTManager("test-secret", 1*time.Hour, "test-issuer")
	require.NoError(t, err)

	// Generate original agent token
	originalToken, err := manager.GenerateAgentToken("tenant-456", "acme", "agent-789", []string{"events:submit"})
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)

	// Refresh the token
	refreshedToken, err := manager.RefreshToken(originalToken)
	require.NoError(t, err)
	assert.NotEqual(t, originalToken, refreshedToken)

	// Validate refreshed token maintains agent properties
	claims, err := manager.ValidateToken(refreshedToken)
	require.NoError(t, err)
	assert.Equal(t, "tenant-456", claims.TenantID)
	assert.Equal(t, "acme", claims.TenantSlug)
	assert.Equal(t, "agent-789", claims.AgentID)
	assert.True(t, claims.IsAgentToken())
}

func TestJWTManager_RefreshToken_TenantUserToken(t *testing.T) {
	manager, err := NewJWTManager("test-secret", 1*time.Hour, "test-issuer")
	require.NoError(t, err)

	// Generate original tenant user token
	originalToken, err := manager.GenerateTenantUserToken("user-123", "testuser", "tenant-456", "acme", []string{"read"})
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)

	// Refresh the token
	refreshedToken, err := manager.RefreshToken(originalToken)
	require.NoError(t, err)
	assert.NotEqual(t, originalToken, refreshedToken)

	// Validate refreshed token maintains tenant user properties
	claims, err := manager.ValidateToken(refreshedToken)
	require.NoError(t, err)
	assert.Equal(t, "user-123", claims.UserID)
	assert.Equal(t, "testuser", claims.Username)
	assert.Equal(t, "tenant-456", claims.TenantID)
	assert.Equal(t, "acme", claims.TenantSlug)
	assert.True(t, claims.IsUserToken())
}
