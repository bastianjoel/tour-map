package geo

import (
	"math"
	"testing"
	"time"
)

func TestInterpolateLocation(t *testing.T) {
	baseTime := time.Date(2026, 8, 15, 10, 0, 0, 0, time.UTC)

	wps := []Waypoint{
		{Location: &GPSCoords{Latitude: 50.0, Longitude: 10.0}, Timestamp: baseTime},
		{Location: &GPSCoords{Latitude: 50.1, Longitude: 10.2}, Timestamp: baseTime.Add(10 * time.Minute)},
		{Location: &GPSCoords{Latitude: 50.2, Longitude: 10.4}, Timestamp: baseTime.Add(20 * time.Minute)},
	}

	// 1. Exact match on waypoint 0
	loc := InterpolateLocation(wps, baseTime, 1*time.Hour)
	if loc == nil || loc.Latitude != 50.0 || loc.Longitude != 10.0 {
		t.Errorf("expected exact match at 50.0, 10.0, got %v", loc)
	}

	// 2. Exact match at 50% between wps[0] and wps[1] (at 5 minutes)
	locMid := InterpolateLocation(wps, baseTime.Add(5*time.Minute), 1*time.Hour)
	if locMid == nil {
		t.Fatalf("expected interpolated location, got nil")
	}
	if math.Abs(locMid.Latitude-50.05) > 1e-6 || math.Abs(locMid.Longitude-10.1) > 1e-6 {
		t.Errorf("expected ~50.05, 10.1, got %v", locMid)
	}

	// 3. Exact match at 25% between wps[1] and wps[2] (at 12.5 minutes)
	locQuarter := InterpolateLocation(wps, baseTime.Add(12*time.Minute+30*time.Second), 1*time.Hour)
	if locQuarter == nil {
		t.Fatalf("expected interpolated location, got nil")
	}
	expectedLat := 50.1 + 0.25*(50.2-50.1)
	expectedLng := 10.2 + 0.25*(10.4-10.2)
	if math.Abs(locQuarter.Latitude-expectedLat) > 1e-6 || math.Abs(locQuarter.Longitude-expectedLng) > 1e-6 {
		t.Errorf("expected %f, %f, got %v", expectedLat, expectedLng, locQuarter)
	}

	// 4. Clamping before start within buffer (30 mins before start)
	locBefore := InterpolateLocation(wps, baseTime.Add(-30*time.Minute), 1*time.Hour)
	if locBefore == nil || locBefore.Latitude != 50.0 || locBefore.Longitude != 10.0 {
		t.Errorf("expected clamped to start (50.0, 10.0), got %v", locBefore)
	}

	// 5. Clamping after end within buffer (45 mins after end)
	locAfter := InterpolateLocation(wps, baseTime.Add(20*time.Minute+45*time.Minute), 1*time.Hour)
	if locAfter == nil || locAfter.Latitude != 50.2 || locAfter.Longitude != 10.4 {
		t.Errorf("expected clamped to end (50.2, 10.4), got %v", locAfter)
	}

	// 6. Outside buffer (> 1 hour before start or after end)
	locFarBefore := InterpolateLocation(wps, baseTime.Add(-2*time.Hour), 1*time.Hour)
	if locFarBefore != nil {
		t.Errorf("expected nil for time far before start, got %v", locFarBefore)
	}

	locFarAfter := InterpolateLocation(wps, baseTime.Add(5*time.Hour), 1*time.Hour)
	if locFarAfter != nil {
		t.Errorf("expected nil for time far after end, got %v", locFarAfter)
	}

	// 7. Empty waypoints or zero target time
	if res := InterpolateLocation(nil, baseTime, 1*time.Hour); res != nil {
		t.Errorf("expected nil for empty waypoints")
	}
	if res := InterpolateLocation(wps, time.Time{}, 1*time.Hour); res != nil {
		t.Errorf("expected nil for zero target time")
	}
}
