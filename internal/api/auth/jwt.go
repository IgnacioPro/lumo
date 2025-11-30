package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// TokenType represents the type of JWT token
type TokenType string

const (
	TokenTypeUser  TokenType = "user"
	TokenTypeAgent TokenType = "agent"
)

// Claims represents JWT claims for Lumo API
type Claims struct {
	UserID   string   `json:"user_id,omitempty"`
	Username string   `json:"username,omitempty"`
	Scopes   []string `json:"scopes,omitempty"`

	// Multi-tenant fields
	TenantID   string    `json:"tenant_id,omitempty"`
	TenantSlug string    `json:"tenant_slug,omitempty"`
	AgentID    string    `json:"agent_id,omitempty"`
	TokenType  TokenType `json:"token_type,omitempty"`

	jwt.RegisteredClaims
}

// IsTenantScoped returns true if the token is scoped to a specific tenant
func (c *Claims) IsTenantScoped() bool {
	return c.TenantID != ""
}

// IsAgentToken returns true if the token is for an agent
func (c *Claims) IsAgentToken() bool {
	return c.TokenType == TokenTypeAgent || c.AgentID != ""
}

// IsUserToken returns true if the token is for a user
func (c *Claims) IsUserToken() bool {
	return c.TokenType == TokenTypeUser || c.UserID != ""
}

// HasScope returns true if the token has the specified scope
func (c *Claims) HasScope(scope string) bool {
	for _, s := range c.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// JWTManager handles JWT token generation and validation
type JWTManager struct {
	secretKey  string
	expiration time.Duration
	issuer     string
}

// NewJWTManager creates a new JWT manager
func NewJWTManager(secretKey string, expiration time.Duration, issuer string) (*JWTManager, error) {
	if secretKey == "" {
		return nil, fmt.Errorf("JWT secret key is required")
	}
	if expiration <= 0 {
		expiration = 24 * time.Hour // Default to 24 hours
	}
	if issuer == "" {
		issuer = "lumo-api"
	}

	return &JWTManager{
		secretKey:  secretKey,
		expiration: expiration,
		issuer:     issuer,
	}, nil
}

// GenerateToken generates a new JWT token for a user
func (m *JWTManager) GenerateToken(userID, username string, scopes []string) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:    userID,
		Username:  username,
		Scopes:    scopes,
		TokenType: TokenTypeUser,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.expiration)),
			NotBefore: jwt.NewNumericDate(now),
			ID:        uuid.New().String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(m.secretKey))
}

// GenerateTenantUserToken generates a JWT token for a user within a tenant
func (m *JWTManager) GenerateTenantUserToken(userID, username, tenantID, tenantSlug string, scopes []string) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:     userID,
		Username:   username,
		TenantID:   tenantID,
		TenantSlug: tenantSlug,
		Scopes:     scopes,
		TokenType:  TokenTypeUser,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.expiration)),
			NotBefore: jwt.NewNumericDate(now),
			ID:        uuid.New().String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(m.secretKey))
}

// GenerateAgentToken generates a JWT token for an agent
func (m *JWTManager) GenerateAgentToken(tenantID, tenantSlug, agentID string, scopes []string) (string, error) {
	now := time.Now()
	claims := Claims{
		TenantID:   tenantID,
		TenantSlug: tenantSlug,
		AgentID:    agentID,
		Scopes:     scopes,
		TokenType:  TokenTypeAgent,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   agentID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.expiration)),
			NotBefore: jwt.NewNumericDate(now),
			ID:        uuid.New().String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(m.secretKey))
}

// ValidateToken validates a JWT token and returns the claims
func (m *JWTManager) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(m.secretKey), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return claims, nil
}

// RefreshToken generates a new token with extended expiration
func (m *JWTManager) RefreshToken(oldToken string) (string, error) {
	claims, err := m.ValidateToken(oldToken)
	if err != nil {
		return "", fmt.Errorf("cannot refresh invalid token: %w", err)
	}

	// Generate new token based on token type
	if claims.IsAgentToken() {
		return m.GenerateAgentToken(claims.TenantID, claims.TenantSlug, claims.AgentID, claims.Scopes)
	}

	if claims.IsTenantScoped() {
		return m.GenerateTenantUserToken(claims.UserID, claims.Username, claims.TenantID, claims.TenantSlug, claims.Scopes)
	}

	return m.GenerateToken(claims.UserID, claims.Username, claims.Scopes)
}

// Expiration returns the configured token expiration duration
func (m *JWTManager) Expiration() time.Duration {
	return m.expiration
}

// GenerateSecureSecret generates a cryptographically secure random secret key.
// Useful for generating JWT secrets. Returns a base64-encoded string.
func GenerateSecureSecret(length int) (string, error) {
	if length < 32 {
		length = 32 // Minimum recommended length
	}

	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	return base64.StdEncoding.EncodeToString(bytes), nil
}
