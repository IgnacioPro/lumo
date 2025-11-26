package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ignacio/lumo/internal/api/response"
)

// parsePagination extracts limit and offset from query parameters
// Default limit: 50, max limit: 100, default offset: 0
func parsePagination(r *http.Request) (limit, offset int) {
	limit = 50 // default
	offset = 0 // default

	// Parse limit
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	// Parse offset
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	return limit, offset
}

// parseUUIDParam extracts and validates a UUID from URL parameters.
// Returns the parsed UUID and true if successful, or writes an error response and returns false.
func parseUUIDParam(w http.ResponseWriter, r *http.Request, param string) (uuid.UUID, bool) {
	idStr := chi.URLParam(r, param)
	if idStr == "" {
		response.BadRequest(w, fmt.Sprintf("%s is required", param))
		return uuid.Nil, false
	}

	id, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(w, fmt.Sprintf("Invalid %s format", param))
		return uuid.Nil, false
	}

	return id, true
}
