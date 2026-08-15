package geo

import (
	"testing"
	"time"
)

func TestSegmentWaypoints_WithinLimits(t *testing.T) {
	baseTime := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)

	// Points within 1-2 km and a few minutes apart
	waypoints := []Waypoint{
		{Location: &GPSCoords{Latitude: 52.5200, Longitude: 13.4050}, Timestamp: baseTime},
		{Location: &GPSCoords{Latitude: 52.5210, Longitude: 13.4060}, Timestamp: baseTime.Add(10 * time.Minute)},
		{Location: &GPSCoords{Latitude: 52.5220, Longitude: 13.4070}, Timestamp: baseTime.Add(20 * time.Minute)},
	}

	segments := SegmentWaypoints(waypoints, 10.0, 7*24*time.Hour)
	if len(segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(segments))
	}
	if len(segments[0]) != 3 {
		t.Fatalf("expected 3 points in segment 0, got %d", len(segments[0]))
	}
}

func TestSegmentWaypoints_DistanceSplit(t *testing.T) {
	baseTime := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)

	// Point 1 and 2 are close in Berlin. Point 3 is in Munich (far > 10 km).
	waypoints := []Waypoint{
		{Location: &GPSCoords{Latitude: 52.5200, Longitude: 13.4050}, Timestamp: baseTime},
		{Location: &GPSCoords{Latitude: 52.5210, Longitude: 13.4060}, Timestamp: baseTime.Add(1 * time.Hour)},
		{Location: &GPSCoords{Latitude: 48.1351, Longitude: 11.5820}, Timestamp: baseTime.Add(2 * time.Hour)},
		{Location: &GPSCoords{Latitude: 48.1360, Longitude: 11.5830}, Timestamp: baseTime.Add(3 * time.Hour)},
	}

	segments := SegmentWaypoints(waypoints, 10.0, 7*24*time.Hour)
	if len(segments) != 2 {
		t.Fatalf("expected 2 segments due to >10km distance, got %d", len(segments))
	}
	if len(segments[0]) != 2 {
		t.Errorf("expected 2 points in trip 1, got %d", len(segments[0]))
	}
	if len(segments[1]) != 2 {
		t.Errorf("expected 2 points in trip 2, got %d", len(segments[1]))
	}
}

func TestSegmentWaypoints_TimeSplit(t *testing.T) {
	baseTime := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)

	// Points are close geographically (in same city), but 8 days apart
	waypoints := []Waypoint{
		{Location: &GPSCoords{Latitude: 52.5200, Longitude: 13.4050}, Timestamp: baseTime},
		{Location: &GPSCoords{Latitude: 52.5210, Longitude: 13.4060}, Timestamp: baseTime.Add(1 * time.Hour)},
		{Location: &GPSCoords{Latitude: 52.5220, Longitude: 13.4070}, Timestamp: baseTime.Add(8 * 24 * time.Hour)},
		{Location: &GPSCoords{Latitude: 52.5230, Longitude: 13.4080}, Timestamp: baseTime.Add(8*24*time.Hour + 1*time.Hour)},
	}

	segments := SegmentWaypoints(waypoints, 10.0, 7*24*time.Hour)
	if len(segments) != 2 {
		t.Fatalf("expected 2 segments due to >7 days time gap, got %d", len(segments))
	}
	if len(segments[0]) != 2 {
		t.Errorf("expected 2 points in trip 1, got %d", len(segments[0]))
	}
	if len(segments[1]) != 2 {
		t.Errorf("expected 2 points in trip 2, got %d", len(segments[1]))
	}
}

func TestSegmentWaypoints_BoundaryTests(t *testing.T) {
	baseTime := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)

	// 1. Time boundary test: 6 days 23 hours (should not split) vs 7 days 1 hour (should split)
	wp1 := Waypoint{Location: &GPSCoords{Latitude: 52.5200, Longitude: 13.4050}, Timestamp: baseTime}
	wp2 := Waypoint{Location: &GPSCoords{Latitude: 52.5205, Longitude: 13.4055}, Timestamp: baseTime.Add(6*24*time.Hour + 23*time.Hour)}
	wp3 := Waypoint{Location: &GPSCoords{Latitude: 52.5210, Longitude: 13.4060}, Timestamp: baseTime.Add(14*24*time.Hour + 1*time.Hour)} // 7 days 2 hours after wp2

	segments := SegmentWaypoints([]Waypoint{wp1, wp2, wp3}, 10.0, 7*24*time.Hour)
	if len(segments) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(segments))
	}
	if len(segments[0]) != 2 || len(segments[1]) != 1 {
		t.Errorf("unexpected segment lengths: %v", segments)
	}
}

func TestSegmentWaypoints_EdgeCases(t *testing.T) {
	// Empty slice
	if res := SegmentWaypoints(nil, 10.0, 7*24*time.Hour); len(res) != 0 {
		t.Errorf("expected 0 segments for nil waypoints, got %d", len(res))
	}
	if res := SegmentWaypoints([]Waypoint{}, 10.0, 7*24*time.Hour); len(res) != 0 {
		t.Errorf("expected 0 segments for empty waypoints, got %d", len(res))
	}

	// Single point
	single := []Waypoint{
		{Location: &GPSCoords{Latitude: 52.5200, Longitude: 13.4050}, Timestamp: time.Now()},
	}
	res := SegmentWaypoints(single, 10.0, 7*24*time.Hour)
	if len(res) != 1 || len(res[0]) != 1 {
		t.Errorf("expected 1 segment with 1 point for single waypoint, got %v", res)
	}

	// Waypoints with nil Location should be ignored safely
	withNil := []Waypoint{
		{Location: nil, Timestamp: time.Now()},
		{Location: &GPSCoords{Latitude: 52.5200, Longitude: 13.4050}, Timestamp: time.Now()},
		{Location: nil, Timestamp: time.Now()},
	}
	resNil := SegmentWaypoints(withNil, 10.0, 7*24*time.Hour)
	if len(resNil) != 1 || len(resNil[0]) != 1 {
		t.Errorf("expected 1 segment with 1 point, got %v", resNil)
	}
}

func TestFilterPrivacy(t *testing.T) {
	baseTime := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)

	// Points:
	// p0: Paris (far away from Munich, > 600km)
	// p1: Munich City Center (last waypoint)
	// p2: Munich 2km away from p1
	// p3: Munich 5km away from p1
	// p4: Munich center (same as p1)
	p0 := Waypoint{Location: &GPSCoords{Latitude: 48.8566, Longitude: 2.3522}, Timestamp: baseTime}
	p1 := Waypoint{Location: &GPSCoords{Latitude: 48.1351, Longitude: 11.5820}, Timestamp: baseTime.Add(1 * time.Hour)}
	p2 := Waypoint{Location: &GPSCoords{Latitude: 48.1400, Longitude: 11.5850}, Timestamp: baseTime.Add(2 * time.Hour)}
	p3 := Waypoint{Location: &GPSCoords{Latitude: 48.1450, Longitude: 11.5900}, Timestamp: baseTime.Add(3 * time.Hour)}

	// Test 1: All points close to the last one (p1, p2, p3)
	allClose := []Waypoint{p1, p2, p3}
	filteredAllClose := FilterPrivacy(allClose, 10.0)
	if len(filteredAllClose) != 0 {
		t.Errorf("expected 0 points remaining when all points are within 10km of last point, got %d", len(filteredAllClose))
	}

	// Test 2: Point p0 is far away (>10km), p1, p2, p3 are within 10km of p3
	mixed := []Waypoint{p0, p1, p2, p3}
	filteredMixed := FilterPrivacy(mixed, 10.0)
	if len(filteredMixed) != 1 {
		t.Fatalf("expected 1 point (p0) remaining, got %d", len(filteredMixed))
	}
	if filteredMixed[0].Location.Latitude != p0.Location.Latitude {
		t.Errorf("expected p0, got %v", filteredMixed[0])
	}

	// Test 3: Empty slice
	if res := FilterPrivacy(nil, 10.0); len(res) != 0 {
		t.Errorf("expected empty result for nil input")
	}
}
