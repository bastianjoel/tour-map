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
	if len(segments[0].Coords) != 3 {
		t.Fatalf("expected 3 points in segment 0, got %d", len(segments[0].Coords))
	}
	if !segments[0].StartTime.Equal(baseTime) || !segments[0].EndTime.Equal(baseTime.Add(20*time.Minute)) {
		t.Errorf("unexpected segment timeframe: %v to %v", segments[0].StartTime, segments[0].EndTime)
	}
	if len(segments[0].Lines) != 1 || segments[0].Lines[0].Type != "solid" {
		t.Errorf("expected 1 solid line, got %v", segments[0].Lines)
	}
}

func TestSegmentWaypoints_DistanceSplit(t *testing.T) {
	baseTime := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)

	// Point 1 and 2 are close in Berlin. Point 3 is in Munich (far > 10 km, no common ActivityID).
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
	if len(segments[0].Coords) != 2 {
		t.Errorf("expected 2 points in trip 1, got %d", len(segments[0].Coords))
	}
	if len(segments[1].Coords) != 2 {
		t.Errorf("expected 2 points in trip 2, got %d", len(segments[1].Coords))
	}
}

func TestSegmentWaypoints_SingleFitFileDottedConnection_2km(t *testing.T) {
	baseTime := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)

	// All 4 points belong to the SAME single FIT activity ("fit:ride1.fit")
	// Points 1 & 2 are in Berlin (0.1km apart -> solid).
	// Point 3 is 3km away after a pause (> 2km threshold -> dotted).
	// Point 4 is close to Point 3 (0.1km apart -> solid).
	fitID := "fit:ride1.fit"
	waypoints := []Waypoint{
		{Location: &GPSCoords{Latitude: 52.5200, Longitude: 13.4050}, Timestamp: baseTime, ActivityID: fitID},
		{Location: &GPSCoords{Latitude: 52.5210, Longitude: 13.4060}, Timestamp: baseTime.Add(10 * time.Minute), ActivityID: fitID},
		{Location: &GPSCoords{Latitude: 52.5500, Longitude: 13.4300}, Timestamp: baseTime.Add(40 * time.Minute), ActivityID: fitID}, // ~3.7 km gap (>2km)
		{Location: &GPSCoords{Latitude: 52.5510, Longitude: 13.4310}, Timestamp: baseTime.Add(50 * time.Minute), ActivityID: fitID},
	}

	segments := SegmentWaypoints(waypoints, 10.0, 7*24*time.Hour)
	if len(segments) != 1 {
		t.Fatalf("expected 1 connected segment for single FIT activity, got %d", len(segments))
	}
	if len(segments[0].Coords) != 4 {
		t.Errorf("expected 4 total coordinates, got %d", len(segments[0].Coords))
	}
	if len(segments[0].Lines) != 3 {
		t.Fatalf("expected 3 sub-lines (solid, dotted, solid), got %d: %v", len(segments[0].Lines), segments[0].Lines)
	}
	if segments[0].Lines[0].Type != "solid" {
		t.Errorf("expected line 0 to be solid, got %s", segments[0].Lines[0].Type)
	}
	if segments[0].Lines[1].Type != "dotted" {
		t.Errorf("expected line 1 to be dotted for >2km pause, got %s", segments[0].Lines[1].Type)
	}
	if segments[0].Lines[2].Type != "solid" {
		t.Errorf("expected line 2 to be solid, got %s", segments[0].Lines[2].Type)
	}
}

func TestSegmentWaypoints_DifferentFitFilesDisconnected(t *testing.T) {
	baseTime := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)

	// Points from two DIFFERENT FIT activities (ride1 in Berlin, ride2 in Munich >10km away)
	waypoints := []Waypoint{
		{Location: &GPSCoords{Latitude: 52.5200, Longitude: 13.4050}, Timestamp: baseTime, ActivityID: "fit:ride1.fit"},
		{Location: &GPSCoords{Latitude: 52.5210, Longitude: 13.4060}, Timestamp: baseTime.Add(30 * time.Minute), ActivityID: "fit:ride1.fit"},
		{Location: &GPSCoords{Latitude: 48.1351, Longitude: 11.5820}, Timestamp: baseTime.Add(2 * time.Hour), ActivityID: "fit:ride2.fit"},
		{Location: &GPSCoords{Latitude: 48.1360, Longitude: 11.5830}, Timestamp: baseTime.Add(3 * time.Hour), ActivityID: "fit:ride2.fit"},
	}

	segments := SegmentWaypoints(waypoints, 10.0, 7*24*time.Hour)
	if len(segments) != 2 {
		t.Fatalf("expected 2 disconnected segments for different FIT activities, got %d", len(segments))
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
	if len(segments[0].Coords) != 2 {
		t.Errorf("expected 2 points in trip 1, got %d", len(segments[0].Coords))
	}
	if len(segments[1].Coords) != 2 {
		t.Errorf("expected 2 points in trip 2, got %d", len(segments[1].Coords))
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
	if len(segments[0].Coords) != 2 || len(segments[1].Coords) != 1 {
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
	if len(res) != 1 || len(res[0].Coords) != 1 {
		t.Errorf("expected 1 segment with 1 point for single waypoint, got %v", res)
	}

	// Waypoints with nil Location should be ignored safely
	withNil := []Waypoint{
		{Location: nil, Timestamp: time.Now()},
		{Location: &GPSCoords{Latitude: 52.5200, Longitude: 13.4050}, Timestamp: time.Now()},
		{Location: nil, Timestamp: time.Now()},
	}
	resNil := SegmentWaypoints(withNil, 10.0, 7*24*time.Hour)
	if len(resNil) != 1 || len(resNil[0].Coords) != 1 {
		t.Errorf("expected 1 segment with 1 point, got %v", resNil)
	}
}

func TestFilterPrivacy(t *testing.T) {
	baseTime := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)

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
