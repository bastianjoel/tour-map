package tracker

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"tour-map/pkg/geo"
	"tour-map/pkg/images"
)

func TestStore_LoadWaypointsAndCodes(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tracker-store-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dataDir := filepath.Join(tmpDir, "data")
	fitDir := filepath.Join(tmpDir, "fit")
	codesFile := filepath.Join(tmpDir, "codes.txt")

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("failed to create data dir: %v", err)
	}
	if err := os.MkdirAll(fitDir, 0755); err != nil {
		t.Fatalf("failed to create fit dir: %v", err)
	}

	// 1. Write sample JSON tracking files
	wp1 := geo.Waypoint{
		Location:  &geo.GPSCoords{Latitude: 52.5200, Longitude: 13.4050},
		Timestamp: time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC),
	}
	wp2 := geo.Waypoint{
		Location:  &geo.GPSCoords{Latitude: 52.5250, Longitude: 13.4100},
		Timestamp: time.Date(2026, 8, 1, 10, 10, 0, 0, time.UTC),
	}

	d1, _ := json.Marshal(wp1)
	d2, _ := json.Marshal(wp2)
	os.WriteFile(filepath.Join(dataDir, "tracking_20260801_100000.json"), d1, 0644)
	os.WriteFile(filepath.Join(dataDir, "tracking_20260801_101000.json"), d2, 0644)

	// 2. Write sample codes file
	codesContent := "secret123\n  mycode  \n\n"
	os.WriteFile(codesFile, []byte(codesContent), 0644)

	store := NewStore(dataDir, fitDir, codesFile)

	// Test code loading
	if err := store.LoadCodes(); err != nil {
		t.Fatalf("LoadCodes() failed: %v", err)
	}

	if !store.IsAuthorized("secret123") {
		t.Errorf("expected secret123 to be authorized")
	}
	if !store.IsAuthorized("mycode") {
		t.Errorf("expected mycode to be authorized")
	}
	if store.IsAuthorized("invalid") {
		t.Errorf("expected invalid to NOT be authorized")
	}

	// Test waypoints loading
	if err := store.LoadWaypoints(); err != nil {
		t.Fatalf("LoadWaypoints() failed: %v", err)
	}

	wps := store.GetWaypoints()
	if len(wps) != 2 {
		t.Fatalf("expected 2 waypoints, got %d", len(wps))
	}
	if wps[0].Location.Latitude != 52.5200 || wps[1].Location.Latitude != 52.5250 {
		t.Errorf("unexpected waypoints data: %v", wps)
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
	store := NewStore(dataDir, "", "")

	t1 := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 8, 1, 10, 5, 0, 0, time.UTC)

	wp1 := geo.Waypoint{Location: &geo.GPSCoords{Latitude: 52.5200, Longitude: 13.4050}, Timestamp: t1}
	wp2 := geo.Waypoint{Location: &geo.GPSCoords{Latitude: 52.5300, Longitude: 13.4150}, Timestamp: t2}
	raw1, _ := json.Marshal(wp1)
	raw2, _ := json.Marshal(wp2)

	// Add first waypoint
	if added := store.AddWaypoint(wp1, raw1); !added {
		t.Errorf("expected wp1 to be added")
	}

	// Add older waypoint (should be rejected)
	olderWp := geo.Waypoint{Location: &geo.GPSCoords{Latitude: 52.5100, Longitude: 13.4000}, Timestamp: t1.Add(-10 * time.Minute)}
	if added := store.AddWaypoint(olderWp, nil); added {
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

func TestStore_GetUpdates(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tracker-updates-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dataDir := filepath.Join(tmpDir, "data")
	fitDir := filepath.Join(tmpDir, "fit")
	codesFile := filepath.Join(tmpDir, "codes.txt")
	os.MkdirAll(dataDir, 0755)

	os.WriteFile(codesFile, []byte("pass123\n"), 0644)
	store := NewStore(dataDir, fitDir, codesFile)
	store.LoadCodes()

	t0 := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	wp1 := geo.Waypoint{Location: &geo.GPSCoords{Latitude: 52.5200, Longitude: 13.4050}, Timestamp: t0}
	wp2 := geo.Waypoint{Location: &geo.GPSCoords{Latitude: 52.5210, Longitude: 13.4060}, Timestamp: t0.Add(1 * time.Hour)}
	wp3 := geo.Waypoint{Location: &geo.GPSCoords{Latitude: 52.5220, Longitude: 13.4070}, Timestamp: t0.Add(2 * time.Hour)}

	store.AddWaypoint(wp1, nil)
	store.AddWaypoint(wp2, nil)
	store.AddWaypoint(wp3, nil)

	// Query with since = t0 (should return wp2 and wp3)
	segs, lastMod := store.GetUpdates(t0, "pass123")
	if len(segs) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(segs))
	}
	if len(segs[0].Coords) != 2 {
		t.Errorf("expected 2 waypoints in update, got %d", len(segs[0].Coords))
	}
	if !lastMod.Equal(t0.Add(2 * time.Hour)) {
		t.Errorf("expected lastModified = %v, got %v", t0.Add(2*time.Hour), lastMod)
	}
}

func TestStore_InterpolateImageLocations(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tracker-interpolate-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dataDir := filepath.Join(tmpDir, "data")
	store := NewStore(dataDir, "", "")

	t0 := time.Date(2026, 8, 15, 10, 0, 0, 0, time.UTC)
	wp1 := geo.Waypoint{Location: &geo.GPSCoords{Latitude: 52.5200, Longitude: 13.4050}, Timestamp: t0}
	wp2 := geo.Waypoint{Location: &geo.GPSCoords{Latitude: 52.5300, Longitude: 13.4150}, Timestamp: t0.Add(10 * time.Minute)}

	store.AddWaypoint(wp1, nil)
	store.AddWaypoint(wp2, nil)

	// Photo 1: Has GPS -> should remain unchanged
	origGPS := &geo.GPSCoords{Latitude: 40.0, Longitude: 10.0}
	imgWithGPS := images.ImageInfo{
		Filename:  "has_gps.jpg",
		Location:  origGPS,
		Timestamp: t0.Add(5 * time.Minute),
	}

	// Photo 2: No GPS, taken at 5 minutes into the 10 min track -> should be interpolated at 50%
	imgNoGPS := images.ImageInfo{
		Filename:  "no_gps.jpg",
		Location:  nil,
		Timestamp: t0.Add(5 * time.Minute),
	}

	// Photo 3: No GPS, taken on another day -> should remain nil
	imgOtherDay := images.ImageInfo{
		Filename:  "other_day.jpg",
		Location:  nil,
		Timestamp: t0.Add(48 * time.Hour),
	}

	inputImgs := []images.ImageInfo{imgWithGPS, imgNoGPS, imgOtherDay}
	outputImgs := store.InterpolateImageLocations(inputImgs)

	if len(outputImgs) != 3 {
		t.Fatalf("expected 3 images, got %d", len(outputImgs))
	}

	// Image 1: Unchanged
	if outputImgs[0].Location.Latitude != origGPS.Latitude {
		t.Errorf("expected image with GPS to remain unchanged")
	}

	// Image 2: Interpolated to 52.5250, 13.4100
	if outputImgs[1].Location == nil {
		t.Fatalf("expected interpolated location for image 2, got nil")
	}
	if math.Abs(outputImgs[1].Location.Latitude-52.5250) > 1e-5 || math.Abs(outputImgs[1].Location.Longitude-13.4100) > 1e-5 {
		t.Errorf("expected ~52.5250, 13.4100, got %v", outputImgs[1].Location)
	}

	// Image 3: Remains nil
	if outputImgs[2].Location != nil {
		t.Errorf("expected image on other day to have nil location, got %v", outputImgs[2].Location)
	}
}
