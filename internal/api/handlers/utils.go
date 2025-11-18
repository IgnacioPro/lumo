package handlers

import (
	"net/http"
	"strconv"
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
