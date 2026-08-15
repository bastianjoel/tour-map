package tracker

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"tour-map/pkg/geo"
)

func TestPoller_PollOnce(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "poller-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dataDir := filepath.Join(tmpDir, "data")
	os.MkdirAll(dataDir, 0755)
	codesFile := filepath.Join(tmpDir, "codes.txt")
	tokenFile := filepath.Join(tmpDir, "tracking_token.txt")

	// Write token
	os.WriteFile(tokenFile, []byte("valid-token-123\n"), 0644)

	wpTime := time.Date(2026, 8, 1, 15, 0, 0, 0, time.UTC)
	mockWaypoint := geo.Waypoint{
		Location:  &geo.GPSCoords{Latitude: 52.5200, Longitude: 13.4050},
		Timestamp: wpTime,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/valid-token-123" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(mockWaypoint)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	store := NewStore(dataDir, codesFile)
	poller := NewPoller(store, tokenFile, server.URL, server.Client())

	if err := poller.PollOnce(); err != nil {
		t.Fatalf("PollOnce() failed: %v", err)
	}

	wps := store.GetWaypoints()
	if len(wps) != 1 {
		t.Fatalf("expected 1 waypoint in store, got %d", len(wps))
	}
	if wps[0].Location.Latitude != 52.5200 {
		t.Errorf("unexpected latitude: %v", wps[0].Location.Latitude)
	}
}

func TestPoller_Token404(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "poller-404-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dataDir := filepath.Join(tmpDir, "data")
	os.MkdirAll(dataDir, 0755)
	codesFile := filepath.Join(tmpDir, "codes.txt")
	tokenFile := filepath.Join(tmpDir, "tracking_token.txt")

	os.WriteFile(tokenFile, []byte("invalid-token\n"), 0644)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	store := NewStore(dataDir, codesFile)
	poller := NewPoller(store, tokenFile, server.URL, server.Client())

	if err := poller.PollOnce(); err != nil {
		t.Fatalf("PollOnce() with 404 should return nil error, got %v", err)
	}

	if !poller.tokenDeleted {
		t.Errorf("expected tokenDeleted to be true after 404")
	}
}
