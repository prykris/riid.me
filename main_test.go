package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
)

func TestCreateShortURL(t *testing.T) {
	// Setup
	router := mux.NewRouter()
	router.HandleFunc("/shorten", createShortURL).Methods("POST")

	tests := []struct {
		name       string
		payload    map[string]string
		wantStatus int
		wantErr    bool
	}{
		{
			name:       "valid url",
			payload:    map[string]string{"long_url": "https://example.com"},
			wantStatus: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "empty url",
			payload:    map[string]string{"long_url": ""},
			wantStatus: http.StatusBadRequest,
			wantErr:    true,
		},
		{
			name:       "missing url field",
			payload:    map[string]string{},
			wantStatus: http.StatusBadRequest,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest("POST", "/shorten", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code)
			
			if !tt.wantErr {
				var response URLResponse
				err := json.NewDecoder(rr.Body).Decode(&response)
				assert.NoError(t, err)
				assert.Contains(t, response.ShortURL, config.Domain)
			}
		})
	}
}

func TestRedirectToLongURL(t *testing.T) {
	// Setup
	router := mux.NewRouter()
	router.HandleFunc("/{shortcode}", redirectToLongURL).Methods("GET")

	// Create a test URL
	testURL := "https://example.com"
	testCode := "testcode123"
	ctx := context.Background()
	err := rdb.Set(ctx, testCode, testURL, time.Hour).Err()
	assert.NoError(t, err)

	tests := []struct {
		name       string
		shortcode  string
		wantStatus int
		wantURL    string
	}{
		{
			name:       "valid shortcode",
			shortcode:  testCode,
			wantStatus: http.StatusMovedPermanently,
			wantURL:    testURL,
		},
		{
			name:       "invalid shortcode",
			shortcode:  "nonexistent",
			wantStatus: http.StatusNotFound,
			wantURL:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/"+tt.shortcode, nil)
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code)
			if tt.wantURL != "" {
				assert.Equal(t, tt.wantURL, rr.Header().Get("Location"))
			}
		})
	}
}

func TestHealthCheck(t *testing.T) {
	router := mux.NewRouter()
	router.HandleFunc("/health", healthCheck).Methods("GET")

	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	
	var response map[string]interface{}
	err := json.NewDecoder(rr.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Equal(t, "ok", response["status"])
	assert.NotNil(t, response["redis"])
} 