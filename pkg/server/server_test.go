package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tour-map/pkg/geo"
	"tour-map/pkg/images"
	"tour-map/pkg/tracker"
)

const testTemplate = `<!doctype html><html><script id="tour-data">{{.Waypoints}}</script><script id="image-data">{{.Images}}</script></html>`

func TestServer_HandleIndex(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "server-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dataDir := filepath.Join(tmpDir, "data")
	fitDir := filepath.Join(tmpDir, "fit")
	imagesDir := filepath.Join(tmpDir, "images")
	codesFile := filepath.Join(tmpDir, "codes.txt")
	os.MkdirAll(dataDir, 0755)
	os.MkdirAll(fitDir, 0755)
	os.MkdirAll(imagesDir, 0755)

	store := tracker.NewStore(dataDir, fitDir, codesFile)
	scanner := images.NewScanner(imagesDir)

	baseTime := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	// Add 2 waypoints in Trip 1, and 1 waypoint in Trip 2 (>10km away)
	wp1 := geo.Waypoint{Location: &geo.GPSCoords{Latitude: 52.5200, Longitude: 13.4050}, Timestamp: baseTime}
	wp2 := geo.Waypoint{Location: &geo.GPSCoords{Latitude: 52.5210, Longitude: 13.4060}, Timestamp: baseTime.Add(10 * time.Minute)}
	wp3 := geo.Waypoint{Location: &geo.GPSCoords{Latitude: 48.8566, Longitude: 2.3522}, Timestamp: baseTime.Add(2 * time.Hour)}

	store.AddWaypoint(wp1, nil)
	store.AddWaypoint(wp2, nil)
	store.AddWaypoint(wp3, nil)

	srv, err := NewServer(store, scanner, imagesDir, testTemplate)
	if err != nil {
		t.Fatalf("NewServer() failed: %v", err)
	}

	handler := srv.Handler()

	// 1. Request "/"
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "tour-data") {
		t.Errorf("response body does not contain tour-data element")
	}

	// 2. Request 404
	req404 := httptest.NewRequest("GET", "/not-found", nil)
	rr404 := httptest.NewRecorder()
	handler.ServeHTTP(rr404, req404)
	if rr404.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr404.Code)
	}
}

func TestServer_HandleUpdates(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "server-updates-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dataDir := filepath.Join(tmpDir, "data")
	fitDir := filepath.Join(tmpDir, "fit")
	imagesDir := filepath.Join(tmpDir, "images")
	codesFile := filepath.Join(tmpDir, "codes.txt")
	os.MkdirAll(dataDir, 0755)
	os.MkdirAll(fitDir, 0755)
	os.MkdirAll(imagesDir, 0755)

	os.WriteFile(codesFile, []byte("valid-code\n"), 0644)

	store := tracker.NewStore(dataDir, fitDir, codesFile)
	store.LoadCodes()
	scanner := images.NewScanner(imagesDir)

	baseTime := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	wp1 := geo.Waypoint{Location: &geo.GPSCoords{Latitude: 40.7128, Longitude: -74.0060}, Timestamp: baseTime}
	wp2 := geo.Waypoint{Location: &geo.GPSCoords{Latitude: 40.7200, Longitude: -74.0070}, Timestamp: baseTime.Add(time.Hour)}

	store.AddWaypoint(wp1, nil)
	store.AddWaypoint(wp2, nil)

	srv, err := NewServer(store, scanner, imagesDir, testTemplate)
	if err != nil {
		t.Fatalf("NewServer() failed: %v", err)
	}

	handler := srv.Handler()

	// 1. Query updates with valid code and since timestamp between wp1 and wp2
	since := baseTime.Add(30 * time.Minute).Format(time.RFC3339)
	req := httptest.NewRequest("GET", "/api/updates?since="+since+"&code=valid-code", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rr.Code)
	}

	var resp UpdateResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode JSON response: %v", err)
	}

	if len(resp.Waypoints) != 1 || len(resp.Waypoints[0].Coords) != 1 {
		t.Errorf("expected 1 segment with 1 waypoint, got %v", resp.Waypoints)
	}

	// 2. Query updates with invalid timestamp format
	reqInvalid := httptest.NewRequest("GET", "/api/updates?since=invalid-date", nil)
	rrInvalid := httptest.NewRecorder()
	handler.ServeHTTP(rrInvalid, reqInvalid)

	if rrInvalid.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d", rrInvalid.Code)
	}
}

func TestServer_ImagesStatic(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "server-images-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dataDir := filepath.Join(tmpDir, "data")
	fitDir := filepath.Join(tmpDir, "fit")
	imagesDir := filepath.Join(tmpDir, "images")
	codesFile := filepath.Join(tmpDir, "codes.txt")
	os.MkdirAll(dataDir, 0755)
	os.MkdirAll(fitDir, 0755)
	os.MkdirAll(imagesDir, 0755)

	// Create a dummy image file
	testImagePath := filepath.Join(imagesDir, "sample.jpg")
	os.WriteFile(testImagePath, []byte("fake image content"), 0644)

	store := tracker.NewStore(dataDir, fitDir, codesFile)
	scanner := images.NewScanner(imagesDir)

	srv, err := NewServer(store, scanner, imagesDir, testTemplate)
	if err != nil {
		t.Fatalf("NewServer() failed: %v", err)
	}

	handler := srv.Handler()

	req := httptest.NewRequest("GET", "/images/sample.jpg", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for image, got %d", rr.Code)
	}

	cacheControl := rr.Header().Get("Cache-Control")
	if !strings.Contains(cacheControl, "max-age=259200") {
		t.Errorf("expected Cache-Control header, got %q", cacheControl)
	}

	if rr.Body.String() != "fake image content" {
		t.Errorf("unexpected body content: %q", rr.Body.String())
	}
}

func TestServer_MultiSegmentOutput(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "server-segments-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dataDir := filepath.Join(tmpDir, "data")
	fitDir := filepath.Join(tmpDir, "fit")
	imagesDir := filepath.Join(tmpDir, "images")
	codesFile := filepath.Join(tmpDir, "codes.txt")
	os.MkdirAll(dataDir, 0755)
	os.MkdirAll(fitDir, 0755)
	os.MkdirAll(imagesDir, 0755)

	os.WriteFile(codesFile, []byte("auth123\n"), 0644)

	store := tracker.NewStore(dataDir, fitDir, codesFile)
	store.LoadCodes()
	scanner := images.NewScanner(imagesDir)

	baseTime := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	// Trip 1 (Berlin)
	wp1 := geo.Waypoint{Location: &geo.GPSCoords{Latitude: 52.5200, Longitude: 13.4050}, Timestamp: baseTime}
	wp2 := geo.Waypoint{Location: &geo.GPSCoords{Latitude: 52.5210, Longitude: 13.4060}, Timestamp: baseTime.Add(10 * time.Minute)}
	// Trip 2 (Paris, >10km away)
	wp3 := geo.Waypoint{Location: &geo.GPSCoords{Latitude: 48.8566, Longitude: 2.3522}, Timestamp: baseTime.Add(2 * time.Hour)}
	wp4 := geo.Waypoint{Location: &geo.GPSCoords{Latitude: 48.8570, Longitude: 2.3530}, Timestamp: baseTime.Add(2*time.Hour + 5*time.Minute)}

	store.AddWaypoint(wp1, nil)
	store.AddWaypoint(wp2, nil)
	store.AddWaypoint(wp3, nil)
	store.AddWaypoint(wp4, nil)

	srv, err := NewServer(store, scanner, imagesDir, testTemplate)
	if err != nil {
		t.Fatalf("NewServer() failed: %v", err)
	}

	handler := srv.Handler()

	req := httptest.NewRequest("GET", "/?code=auth123", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rr.Code)
	}

	// Extract tour-data JSON
	body := rr.Body.String()
	prefix := `<script id="tour-data">`
	suffix := `</script>`
	start := strings.Index(body, prefix) + len(prefix)
	end := strings.Index(body, suffix)
	jsonData := body[start:end]

	var segments []geo.Segment
	if err := json.Unmarshal([]byte(jsonData), &segments); err != nil {
		t.Fatalf("failed to parse segments JSON: %v, body was %q", err, jsonData)
	}

	if len(segments) != 2 {
		t.Fatalf("expected 2 distinct trip segments, got %d", len(segments))
	}
	if len(segments[0].Coords) != 2 || len(segments[1].Coords) != 2 {
		t.Errorf("unexpected points per segment: %v", segments)
	}
}
