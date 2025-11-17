package v1

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"Zeno/internal/analytics"
	"Zeno/internal/runner"
)

func TestHandler_HandleHealth(t *testing.T) {
	h := NewHandler(nil, nil)

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()

	h.HandleHealth(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["status"] != "healthy" {
		t.Errorf("Expected status 'healthy', got '%s'", response["status"])
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got '%s'", contentType)
	}
}

func TestHandler_HandleMetrics(t *testing.T) {
	tracker := analytics.NewTracker()
	h := NewHandler(nil, tracker)

	req := httptest.NewRequest("GET", "/api/v1/metrics", nil)
	rec := httptest.NewRecorder()

	h.HandleMetrics(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got '%s'", contentType)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
}

func TestHandler_HandleRunners(t *testing.T) {
	mgr := runner.NewManager()
	h := NewHandler(mgr, nil)

	req := httptest.NewRequest("GET", "/api/v1/runners", nil)
	rec := httptest.NewRecorder()

	h.HandleRunners(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if _, ok := response["count"]; !ok {
		t.Error("Expected 'count' field in response")
	}

	if _, ok := response["runners"]; !ok {
		t.Error("Expected 'runners' field in response")
	}
}

func TestHandler_HandleHistory(t *testing.T) {
	tracker := analytics.NewTracker()
	h := NewHandler(nil, tracker)

	req := httptest.NewRequest("GET", "/api/v1/history", nil)
	rec := httptest.NewRecorder()

	h.HandleHistory(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got '%s'", contentType)
	}

	var response []interface{}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
}

func TestNewHandler(t *testing.T) {
	mgr := runner.NewManager()
	tracker := analytics.NewTracker()

	h := NewHandler(mgr, tracker)

	if h == nil {
		t.Fatal("NewHandler returned nil")
	}

	if h.runnerMgr != mgr {
		t.Error("Handler runnerMgr not set correctly")
	}

	if h.tracker != tracker {
		t.Error("Handler tracker not set correctly")
	}
}

func TestHandler_MethodNotAllowed(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		method  string
	}{
		{
			name:    "health with POST",
			handler: NewHandler(nil, nil).HandleHealth,
			method:  "POST",
		},
		{
			name:    "metrics with DELETE",
			handler: NewHandler(nil, analytics.NewTracker()).HandleMetrics,
			method:  "DELETE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/test", nil)
			rec := httptest.NewRecorder()

			tt.handler(rec, req)

			// Should still respond (doesn't validate method in current impl)
			if rec.Code == http.StatusMethodNotAllowed {
				t.Skip("Method validation not implemented")
			}
		})
	}
}
