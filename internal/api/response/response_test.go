package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSON(t *testing.T) {
	tests := []struct {
		name            string
		statusCode      int
		data            interface{}
		expectedSuccess bool
	}{
		{
			name:            "Success response (200)",
			statusCode:      http.StatusOK,
			data:            map[string]string{"message": "success"},
			expectedSuccess: true,
		},
		{
			name:            "Created response (201)",
			statusCode:      http.StatusCreated,
			data:            map[string]string{"id": "123"},
			expectedSuccess: true,
		},
		{
			name:            "Bad Request (400)",
			statusCode:      http.StatusBadRequest,
			data:            map[string]string{"error": "bad request"},
			expectedSuccess: false,
		},
		{
			name:            "Internal Server Error (500)",
			statusCode:      http.StatusInternalServerError,
			data:            map[string]string{"error": "server error"},
			expectedSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()

			JSON(rec, tt.statusCode, tt.data)

			assert.Equal(t, tt.statusCode, rec.Code)
			assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

			var resp Response
			err := json.Unmarshal(rec.Body.Bytes(), &resp)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedSuccess, resp.Success)
			assert.NotNil(t, resp.Data)
		})
	}
}

func TestSuccess(t *testing.T) {
	rec := httptest.NewRecorder()
	data := map[string]string{"result": "ok"}

	Success(rec, data)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var resp Response
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.NotNil(t, resp.Data)
}

func TestCreated(t *testing.T) {
	rec := httptest.NewRecorder()
	data := map[string]string{"id": "new-resource"}

	Created(rec, data)

	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var resp Response
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp.Success)
}

func TestNoContent(t *testing.T) {
	rec := httptest.NewRecorder()

	NoContent(rec)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, rec.Body.String())
}

func TestError(t *testing.T) {
	rec := httptest.NewRecorder()

	Error(rec, http.StatusBadRequest, "TEST_ERROR", "Test error message")

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var resp Response
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.False(t, resp.Success)
	assert.Nil(t, resp.Data)
	require.NotNil(t, resp.Error)
	assert.Equal(t, "TEST_ERROR", resp.Error.Code)
	assert.Equal(t, "Test error message", resp.Error.Message)
	assert.Empty(t, resp.Error.Details)
}

func TestErrorWithDetails(t *testing.T) {
	rec := httptest.NewRecorder()

	ErrorWithDetails(rec, http.StatusInternalServerError, "DETAILED_ERROR", "Error message", "Additional details")

	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	var resp Response
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.False(t, resp.Success)
	require.NotNil(t, resp.Error)
	assert.Equal(t, "DETAILED_ERROR", resp.Error.Code)
	assert.Equal(t, "Error message", resp.Error.Message)
	assert.Equal(t, "Additional details", resp.Error.Details)
}

func TestBadRequest(t *testing.T) {
	rec := httptest.NewRecorder()

	BadRequest(rec, "Invalid input")

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var resp Response
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.False(t, resp.Success)
	require.NotNil(t, resp.Error)
	assert.Equal(t, "BAD_REQUEST", resp.Error.Code)
	assert.Equal(t, "Invalid input", resp.Error.Message)
}

func TestUnauthorized(t *testing.T) {
	rec := httptest.NewRecorder()

	Unauthorized(rec, "Authentication required")

	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	var resp Response
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.False(t, resp.Success)
	require.NotNil(t, resp.Error)
	assert.Equal(t, "UNAUTHORIZED", resp.Error.Code)
}

func TestForbidden(t *testing.T) {
	rec := httptest.NewRecorder()

	Forbidden(rec, "Access denied")

	assert.Equal(t, http.StatusForbidden, rec.Code)

	var resp Response
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.False(t, resp.Success)
	require.NotNil(t, resp.Error)
	assert.Equal(t, "FORBIDDEN", resp.Error.Code)
}

func TestNotFound(t *testing.T) {
	rec := httptest.NewRecorder()

	NotFound(rec, "Resource not found")

	assert.Equal(t, http.StatusNotFound, rec.Code)

	var resp Response
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.False(t, resp.Success)
	require.NotNil(t, resp.Error)
	assert.Equal(t, "NOT_FOUND", resp.Error.Code)
}

func TestConflict(t *testing.T) {
	rec := httptest.NewRecorder()

	Conflict(rec, "Resource already exists")

	assert.Equal(t, http.StatusConflict, rec.Code)

	var resp Response
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.False(t, resp.Success)
	require.NotNil(t, resp.Error)
	assert.Equal(t, "CONFLICT", resp.Error.Code)
}

func TestInternalServerError(t *testing.T) {
	rec := httptest.NewRecorder()

	InternalServerError(rec, "Something went wrong")

	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	var resp Response
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.False(t, resp.Success)
	require.NotNil(t, resp.Error)
	assert.Equal(t, "INTERNAL_ERROR", resp.Error.Code)
}

func TestServiceUnavailable(t *testing.T) {
	rec := httptest.NewRecorder()

	ServiceUnavailable(rec, "Service temporarily unavailable")

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)

	var resp Response
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.False(t, resp.Success)
	require.NotNil(t, resp.Error)
	assert.Equal(t, "SERVICE_UNAVAILABLE", resp.Error.Code)
}

func TestResponse_Structure(t *testing.T) {
	// Test Response struct marshaling/unmarshaling
	resp := Response{
		Success: true,
		Data:    map[string]string{"key": "value"},
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var decoded Response
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.True(t, decoded.Success)
	assert.NotNil(t, decoded.Data)
}

func TestErrorInfo_Structure(t *testing.T) {
	// Test ErrorInfo struct
	errInfo := ErrorInfo{
		Code:    "TEST_CODE",
		Message: "Test message",
		Details: "Test details",
	}

	data, err := json.Marshal(errInfo)
	require.NoError(t, err)

	var decoded ErrorInfo
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, "TEST_CODE", decoded.Code)
	assert.Equal(t, "Test message", decoded.Message)
	assert.Equal(t, "Test details", decoded.Details)
}
