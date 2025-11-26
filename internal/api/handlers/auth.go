package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/ignacio/lumo/internal/api/auth"
	"github.com/ignacio/lumo/internal/api/middleware"
	"github.com/ignacio/lumo/internal/api/response"
	"github.com/ignacio/lumo/internal/database/repository"
)

// AuthHandler handles authentication requests
type AuthHandler struct {
	apiKeyRepo *repository.APIKeyRepository
	jwtManager *auth.JWTManager
	logger     *logrus.Logger
}

// NewAuthHandler creates a new authentication handler
func NewAuthHandler(apiKeyRepo *repository.APIKeyRepository, jwtManager *auth.JWTManager, logger *logrus.Logger) *AuthHandler {
	return &AuthHandler{
		apiKeyRepo: apiKeyRepo,
		jwtManager: jwtManager,
		logger:     logger,
	}
}

// TokenRequest represents a token generation request
type TokenRequest struct {
	APIKey string `json:"api_key"` // API key for authentication
}

// TokenResponse represents a token generation response
type TokenResponse struct {
	Token     string    `json:"token"`
	Type      string    `json:"type"`
	ExpiresAt time.Time `json:"expires_at"`
	ExpiresIn int64     `json:"expires_in"` // Seconds until expiration
}

// RefreshResponse represents a token refresh response
type RefreshResponse struct {
	Token     string    `json:"token"`
	Type      string    `json:"type"`
	ExpiresAt time.Time `json:"expires_at"`
	ExpiresIn int64     `json:"expires_in"`
}

// GenerateToken handles POST /api/v1/auth/token
// Exchanges an API key for a JWT token
func (h *AuthHandler) GenerateToken(w http.ResponseWriter, r *http.Request) {
	// Parse request
	var req TokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "Invalid request body")
		return
	}

	if req.APIKey == "" {
		response.BadRequest(w, "API key is required")
		return
	}

	// Validate API key
	apiKey, err := h.apiKeyRepo.ValidateAndGet(r.Context(), req.APIKey)
	if err != nil {
		h.logger.WithError(err).Debug("API key validation failed")
		response.Unauthorized(w, "Invalid API key")
		return
	}

	// Update last used timestamp
	if err := h.apiKeyRepo.UpdateLastUsed(r.Context(), apiKey.ID); err != nil {
		h.logger.WithError(err).Warn("Failed to update API key last used timestamp")
	}

	// Generate JWT token
	token, err := h.jwtManager.GenerateToken(
		apiKey.ID.String(),
		apiKey.Name,
		apiKey.Scopes,
	)
	if err != nil {
		h.logger.WithError(err).Error("Failed to generate JWT token")
		response.InternalServerError(w, "Failed to generate token")
		return
	}

	// Calculate expiration
	expiresIn := 24 * time.Hour // Default, should match JWTManager config
	expiresAt := time.Now().Add(expiresIn)

	resp := TokenResponse{
		Token:     token,
		Type:      "Bearer",
		ExpiresAt: expiresAt,
		ExpiresIn: int64(expiresIn.Seconds()),
	}

	h.logger.WithFields(logrus.Fields{
		"api_key_name": apiKey.Name,
		"user_id":      apiKey.ID.String(),
	}).Info("JWT token generated successfully")

	response.Success(w, resp)
}

// RefreshToken handles POST /api/v1/auth/refresh
// Refreshes an existing JWT token
func (h *AuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	// Get current JWT claims from context
	claims, ok := middleware.GetJWTClaimsFromContext(r.Context())
	if !ok {
		response.Unauthorized(w, "No valid token to refresh")
		return
	}

	// Extract old token from Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		response.Unauthorized(w, "Missing authorization token")
		return
	}

	// Parse Bearer token
	var oldToken string
	if _, err := fmt.Sscanf(authHeader, "Bearer %s", &oldToken); err != nil {
		response.BadRequest(w, "Invalid authorization header")
		return
	}

	// Generate new token
	newToken, err := h.jwtManager.RefreshToken(oldToken)
	if err != nil {
		h.logger.WithError(err).Debug("Failed to refresh token")
		response.Unauthorized(w, "Unable to refresh token")
		return
	}

	// Calculate expiration
	expiresIn := 24 * time.Hour // Default
	expiresAt := time.Now().Add(expiresIn)

	resp := RefreshResponse{
		Token:     newToken,
		Type:      "Bearer",
		ExpiresAt: expiresAt,
		ExpiresIn: int64(expiresIn.Seconds()),
	}

	h.logger.WithFields(logrus.Fields{
		"user_id":  claims.UserID,
		"username": claims.Username,
	}).Info("JWT token refreshed successfully")

	response.Success(w, resp)
}

// ValidateToken handles GET /api/v1/auth/validate
// Validates the current JWT token and returns user info
func (h *AuthHandler) ValidateToken(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetJWTClaimsFromContext(r.Context())
	if !ok {
		response.Unauthorized(w, "No valid token")
		return
	}

	// Return token info
	resp := map[string]interface{}{
		"valid":      true,
		"user_id":    claims.UserID,
		"username":   claims.Username,
		"scopes":     claims.Scopes,
		"issuer":     claims.Issuer,
		"issued_at":  claims.IssuedAt.Time,
		"expires_at": claims.ExpiresAt.Time,
		"not_before": claims.NotBefore.Time,
	}

	response.Success(w, resp)
}
