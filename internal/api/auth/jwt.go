package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Claims represents JWT claims for Lumo API
type Claims struct {
	UserID   string   `json:"user_id"`
	Username string   `json:"username"`
	Scopes   []string `json:"scopes,omitempty"`
	jwt.RegisteredClaims
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

// GenerateToken generates a new JWT token
func (m *JWTManager) GenerateToken(userID, username string, scopes []string) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:   userID,
		Username: username,
		Scopes:   scopes,
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

	// Generate new token with same user info but fresh expiration
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
