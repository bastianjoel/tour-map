package tracker

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"tour-map/pkg/geo"
)

func TestStore_LoadWaypointsAndCodes(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tracker-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dataDir := filepath.Join(tmpDir, "data")
	os.MkdirAll(dataDir, 0755)
	codesFile := filepath.Join(tmpDir, "codes.txt")

	// Write codes file
	os.WriteFile(codesFile, []byte("secret123\nothercode\n"), 0644)

	// Write 2 waypoint JSON files with different timestamps
	t1 := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 8, 1, 12, 30, 0, 0, time.UTC)

	wp1 := geo.Waypoint{
		Location:  &geo.GPSCoords{Latitude: 52.5200, Longitude: 13.4050},
		Timestamp: t1,
	}
	wp2 := geo.Waypoint{
		Location:  &geo.GPSCoords{Latitude: 52.5220, Longitude: 13.4070},
		Timestamp: t2,
	}

	raw1, _ := json.Marshal(wp1)
	raw2, _ := json.Marshal(wp2)

	// Write out of chronological order to test sorting
	os.WriteFile(filepath.Join(dataDir, "tracking_2.json"), raw2, 0644)
	os.WriteFile(filepath.Join(dataDir, "tracking_1.json"), raw1, 0644)

	store := NewStore(dataDir, codesFile)
	if err := store.LoadWaypoints(); err != nil {
		t.Fatalf("LoadWaypoints() failed: %v", err)
	}
	if err := store.LoadCodes(); err != nil {
		t.Fatalf("LoadCodes() failed: %v", err)
	}

	wps := store.GetWaypoints()
	if len(wps) != 2 {
		t.Fatalf("expected 2 waypoints, got %d", len(wps))
	}
	if !wps[0].Timestamp.Equal(t1) || !wps[1].Timestamp.Equal(t2) {
		t.Errorf("waypoints were not properly sorted by timestamp: %v, %v", wps[0].Timestamp, wps[1].Timestamp)
	}

	// Test authorization
	if !store.IsAuthorized("secret123") {
		t.Errorf("expected secret123 to be authorized")
	}
	if !store.IsAuthorized("othercode") {
		t.Errorf("expected othercode to be authorized")
	}
	if store.IsAuthorized("wrongcode") {
		t.Errorf("expected wrongcode to be unauthorized")
	}
	if store.IsAuthorized("") {
		t.Errorf("expected empty string to be unauthorized")
	}
}

func TestStore_AddWaypoint(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tracker-add-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dataDir := filepath.Join(tmpDir, "data")
	os.MkdirAll(dataDir, 0755)
	codesFile := filepath.Join(tmpDir, "codes.txt")

	store := NewStore(dataDir, codesFile)
	t1 := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 8, 1, 11, 0, 0, 0, time.UTC)

	wp1 := geo.Waypoint{
		Location:  &geo.GPSCoords{Latitude: 52.5200, Longitude: 13.4050},
		Timestamp: t1,
	}
	wp2 := geo.Waypoint{
		Location:  &geo.GPSCoords{Latitude: 52.5210, Longitude: 13.4060},
		Timestamp: t2,
	}

	raw1, _ := json.Marshal(wp1)
	raw2, _ := json.Marshal(wp2)

	// Add first waypoint
	if added := store.AddWaypoint(wp1, raw1); !added {
		t.Errorf("expected wp1 to be added")
	}

	// Adding older waypoint should be rejected
	tOld := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	wpOld := geo.Waypoint{
		Location:  &geo.GPSCoords{Latitude: 52.5200, Longitude: 13.4050},
		Timestamp: tOld,
	}
	if added := store.AddWaypoint(wpOld, []byte{}); added {
		t.Errorf("expected older waypoint to be rejected")
	}

	// Add newer waypoint
	if added := store.AddWaypoint(wp2, raw2); !added {
		t.Errorf("expected wp2 to be added")
	}

	if len(store.GetWaypoints()) != 2 {
		t.Errorf("expected 2 waypoints, got %d", len(store.GetWaypoints()))
	}
}
