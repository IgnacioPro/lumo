package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestParseUUIDParam(t *testing.T) {
	validUUID := uuid.New()

	tests := []struct {
		name         string
		paramValue   string
		expectedOK   bool
		expectedUUID uuid.UUID
		expectBadReq bool
	}{
		{
			name:         "valid UUID",
			paramValue:   validUUID.String(),
			expectedOK:   true,
			expectedUUID: validUUID,
		},
		{
			name:         "invalid UUID",
			paramValue:   "not-a-uuid",
			expectedOK:   false,
			expectBadReq: true,
		},
		{
			name:         "malformed UUID",
			paramValue:   "123-456-789",
			expectedOK:   false,
			expectBadReq: true,
		},
		{
			name:         "partial UUID",
			paramValue:   "00000000-0000-0000-0000",
			expectedOK:   false,
			expectBadReq: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()

			// Set up chi router to inject URL param
			r := chi.NewRouter()
			var gotID uuid.UUID
			var gotOK bool

			r.Get("/test/{id}", func(w http.ResponseWriter, r *http.Request) {
				gotID, gotOK = parseUUIDParam(w, r, "id")
			})

			req := httptest.NewRequest("GET", "/test/"+tt.paramValue, nil)
			r.ServeHTTP(w, req)

			if gotOK != tt.expectedOK {
				t.Errorf("expected OK=%v, got %v", tt.expectedOK, gotOK)
			}

			if tt.expectedOK && gotID != tt.expectedUUID {
				t.Errorf("expected UUID %v, got %v", tt.expectedUUID, gotID)
			}

			if tt.expectBadReq && w.Code != http.StatusBadRequest {
				t.Errorf("expected status 400, got %d", w.Code)
			}
		})
	}
}
