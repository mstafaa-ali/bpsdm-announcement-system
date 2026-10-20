package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"announcer/internal/database"
	"announcer/internal/paths"
)

func setupServerTest(t *testing.T) (*Server, func()) {
	tmpDir, err := os.MkdirTemp("", "announcer-server-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	oldDB := paths.DBFile
	oldMedia := paths.MediaDir
	paths.DBFile = filepath.Join(tmpDir, "test.db")
	paths.MediaDir = filepath.Join(tmpDir, "media")
	_ = os.MkdirAll(paths.MediaDir, 0755)

	if err := database.InitDB(); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	srv := NewServer(nil)

	return srv, func() {
		paths.DBFile = oldDB
		paths.MediaDir = oldMedia
		_ = os.RemoveAll(tmpDir)
	}
}

func TestServerStatusAndSettings(t *testing.T) {
	srv, cleanup := setupServerTest(t)
	defer cleanup()

	// 1. GET /api/status
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var statusResp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &statusResp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	if statusResp["status"] != "success" {
		t.Errorf("Expected status 'success', got %v", statusResp["status"])
	}

	// 2. GET /api/settings
	req = httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 for /api/settings, got %d", w.Code)
	}

	// 3. POST /api/schedules
	payload := map[string]any{
		"name":       "Test Schedule",
		"time":       "08:00",
		"days":       []string{"monday", "tuesday"},
		"audio_file": "chime.mp3",
		"enabled":    true,
	}
	body, _ := json.Marshal(payload)
	req = httptest.NewRequest(http.MethodPost, "/api/schedules", bytes.NewReader(body))
	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 for creating schedule, got %d. Body: %s", w.Code, w.Body.String())
	}
}
