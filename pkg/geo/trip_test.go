package geo

import (
	"testing"
	"time"
)

func TestSegmentWaypoints_WithinLimits(t *testing.T) {
	baseTime := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)

	// Points within 10km and 10 mins apart
	wps := []Waypoint{
		{Location: &GPSCoords{Latitude: 52.5200, Longitude: 13.4050}, Timestamp: baseTime},
		{Location: &GPSCoords{Latitude: 52.5210, Longitude: 13.4060}, Timestamp: baseTime.Add(10 * time.Minute)},
		{Location: &GPSCoords{Latitude: 52.5220, Longitude: 13.4070}, Timestamp: baseTime.Add(20 * time.Minute)},
	}

	segments := SegmentWaypoints(wps, 10.0, 7*24*time.Hour)

	if len(segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(segments))
	}
	if len(segments[0].Coords) != 3 {
		t.Errorf("expected 3 coordinates, got %d", len(segments[0].Coords))
	}
}

func TestSegmentWaypoints_DistanceSplit(t *testing.T) {
	baseTime := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)

	// Point 1: Berlin, Point 2: Paris (>10km away)
	wps := []Waypoint{
		{Location: &GPSCoords{Latitude: 52.5200, Longitude: 13.4050}, Timestamp: baseTime},
		{Location: &GPSCoords{Latitude: 48.8566, Longitude: 2.3522}, Timestamp: baseTime.Add(1 * time.Hour)},
	}

	segments := SegmentWaypoints(wps, 10.0, 7*24*time.Hour)

	if len(segments) != 2 {
		t.Fatalf("expected 2 segments due to distance > 10km, got %d", len(segments))
	}
}

func TestSegmentWaypoints_SingleFitFileDottedConnection_2km(t *testing.T) {
	baseTime := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)

	// Points from the same FIT file with a 3km gap (>2km dotted threshold)
	p1 := Waypoint{Location: &GPSCoords{Latitude: 52.5200, Longitude: 13.4050}, Timestamp: baseTime, ActivityID: "fit:tour1.fit"}
	p2 := Waypoint{Location: &GPSCoords{Latitude: 52.5210, Longitude: 13.4060}, Timestamp: baseTime.Add(5 * time.Minute), ActivityID: "fit:tour1.fit"}
	// Gap of ~3.3km
	p3 := Waypoint{Location: &GPSCoords{Latitude: 52.5500, Longitude: 13.4060}, Timestamp: baseTime.Add(30 * time.Minute), ActivityID: "fit:tour1.fit"}
	p4 := Waypoint{Location: &GPSCoords{Latitude: 52.5510, Longitude: 13.4070}, Timestamp: baseTime.Add(35 * time.Minute), ActivityID: "fit:tour1.fit"}

	wps := []Waypoint{p1, p2, p3, p4}
	segments := SegmentWaypoints(wps, 10.0, 7*24*time.Hour)

	if len(segments) != 1 {
		t.Fatalf("expected 1 segment for single FIT activity, got %d", len(segments))
	}

	lines := segments[0].Lines
	if len(lines) != 3 {
		t.Fatalf("expected 3 path lines (solid, dotted, solid), got %d", len(lines))
	}
	if lines[0].Type != "solid" || lines[1].Type != "dotted" || lines[2].Type != "solid" {
		t.Errorf("unexpected line types: %v, %v, %v", lines[0].Type, lines[1].Type, lines[2].Type)
	}
}

func TestSegmentWaypoints_DifferentFitFilesDisconnected(t *testing.T) {
	baseTime := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)

	// Two distinct FIT files 500m apart but 8 days apart (> 7 days)
	wps := []Waypoint{
		{Location: &GPSCoords{Latitude: 52.5200, Longitude: 13.4050}, Timestamp: baseTime, ActivityID: "fit:day1.fit"},
		{Location: &GPSCoords{Latitude: 52.5210, Longitude: 13.4060}, Timestamp: baseTime.Add(8 * 24 * time.Hour), ActivityID: "fit:day2.fit"},
	}

	segments := SegmentWaypoints(wps, 10.0, 7*24*time.Hour)

	if len(segments) != 2 {
		t.Fatalf("expected 2 distinct segments for different FIT files >7 days apart, got %d", len(segments))
	}
}

func TestSegmentWaypoints_TimeSplit(t *testing.T) {
	baseTime := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)

	// Points close in distance (500m) but 8 days apart (>7 days limit)
	wps := []Waypoint{
		{Location: &GPSCoords{Latitude: 52.5200, Longitude: 13.4050}, Timestamp: baseTime},
		{Location: &GPSCoords{Latitude: 52.5210, Longitude: 13.4060}, Timestamp: baseTime.Add(8 * 24 * time.Hour)},
	}

	segments := SegmentWaypoints(wps, 10.0, 7*24*time.Hour)

	if len(segments) != 2 {
		t.Fatalf("expected 2 segments due to time gap > 7 days, got %d", len(segments))
	}
}

func TestSegmentWaypoints_BoundaryTests(t *testing.T) {
	baseTime := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)

	// Test exact time boundary (7 days)
	wpsTimeBoundary := []Waypoint{
		{Location: &GPSCoords{Latitude: 52.5200, Longitude: 13.4050}, Timestamp: baseTime},
		{Location: &GPSCoords{Latitude: 52.5201, Longitude: 13.4051}, Timestamp: baseTime.Add(7 * 24 * time.Hour)},
	}
	segsTime := SegmentWaypoints(wpsTimeBoundary, 10.0, 7*24*time.Hour)
	if len(segsTime) != 1 {
		t.Errorf("expected 1 segment for exact 7 days boundary, got %d", len(segsTime))
	}

	// Just over time boundary (7 days + 1 second)
	wpsTimeOver := []Waypoint{
		{Location: &GPSCoords{Latitude: 52.5200, Longitude: 13.4050}, Timestamp: baseTime},
		{Location: &GPSCoords{Latitude: 52.5201, Longitude: 13.4051}, Timestamp: baseTime.Add(7*24*time.Hour + time.Second)},
	}
	segsTimeOver := SegmentWaypoints(wpsTimeOver, 10.0, 7*24*time.Hour)
	if len(segsTimeOver) != 2 {
		t.Errorf("expected 2 segments for 7 days + 1s, got %d", len(segsTimeOver))
	}
}

func TestSegmentWaypoints_EdgeCases(t *testing.T) {
	// Empty slice
	if res := SegmentWaypoints(nil, 10.0, 7*24*time.Hour); len(res) != 0 {
		t.Errorf("expected empty slice, got %v", res)
	}

	// Single point
	single := []Waypoint{
		{Location: &GPSCoords{Latitude: 52.5200, Longitude: 13.4050}, Timestamp: time.Now()},
	}
	resSingle := SegmentWaypoints(single, 10.0, 7*24*time.Hour)
	if len(resSingle) != 1 || len(resSingle[0].Coords) != 1 {
		t.Errorf("expected 1 segment with 1 point, got %v", resSingle)
	}

	// Points with nil locations filtered out
	withNil := []Waypoint{
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

	// Live tracking points (ActivityID == "")
	p0 := Waypoint{Location: &GPSCoords{Latitude: 48.8566, Longitude: 2.3522}, Timestamp: baseTime}
	p1 := Waypoint{Location: &GPSCoords{Latitude: 48.1351, Longitude: 11.5820}, Timestamp: baseTime.Add(1 * time.Hour)}
	p2 := Waypoint{Location: &GPSCoords{Latitude: 48.1400, Longitude: 11.5850}, Timestamp: baseTime.Add(2 * time.Hour)}
	p3 := Waypoint{Location: &GPSCoords{Latitude: 48.1450, Longitude: 11.5900}, Timestamp: baseTime.Add(3 * time.Hour)}

	// Test 1: All live points close to the last one (p1, p2, p3)
	allClose := []Waypoint{p1, p2, p3}
	filteredAllClose := FilterPrivacy(allClose, 10.0)
	if len(filteredAllClose) != 0 {
		t.Errorf("expected 0 points remaining when all live points are within 10km of last point, got %d", len(filteredAllClose))
	}

	// Test 2: Live point p0 is far away (>10km), p1, p2, p3 are within 10km of p3
	mixedLive := []Waypoint{p0, p1, p2, p3}
	filteredMixed := FilterPrivacy(mixedLive, 10.0)
	if len(filteredMixed) != 1 {
		t.Fatalf("expected 1 point (p0) remaining, got %d", len(filteredMixed))
	}
	if filteredMixed[0].Location.Latitude != p0.Location.Latitude {
		t.Errorf("expected p0, got %v", filteredMixed[0])
	}

	// Test 3: FIT activity waypoints (ActivityID != "") should NEVER be trimmed
	fit1 := Waypoint{Location: &GPSCoords{Latitude: 48.1400, Longitude: 11.5850}, Timestamp: baseTime.Add(1 * time.Hour), ActivityID: "fit:ride.fit"}
	fit2 := Waypoint{Location: &GPSCoords{Latitude: 48.1450, Longitude: 11.5900}, Timestamp: baseTime.Add(2 * time.Hour), ActivityID: "fit:ride.fit"}
	fitOnly := []Waypoint{fit1, fit2}
	filteredFit := FilterPrivacy(fitOnly, 10.0)
	if len(filteredFit) != 2 {
		t.Errorf("expected all FIT waypoints (2) to be preserved without trimming, got %d", len(filteredFit))
	}

	// Test 4: Mixed FIT and Live tracking: FIT kept completely, Live trimmed
	mixedFitAndLive := []Waypoint{fit1, fit2, p1, p2, p3}
	filteredMixedFitLive := FilterPrivacy(mixedFitAndLive, 10.0)
	if len(filteredMixedFitLive) != 2 {
		t.Fatalf("expected 2 FIT waypoints preserved and live points trimmed, got %d", len(filteredMixedFitLive))
	}
	if filteredMixedFitLive[0].ActivityID != "fit:ride.fit" || filteredMixedFitLive[1].ActivityID != "fit:ride.fit" {
		t.Errorf("expected only FIT waypoints, got %v", filteredMixedFitLive)
	}

	// Test 5: Empty slice
	if res := FilterPrivacy(nil, 10.0); len(res) != 0 {
		t.Errorf("expected empty result for nil input")
	}
}

func TestSegmentWaypoints_DayStagesAndDistances(t *testing.T) {
	day1 := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 8, 2, 9, 0, 0, 0, time.UTC)
	day3 := time.Date(2026, 8, 3, 9, 0, 0, 0, time.UTC)

	// Multi-day tour with 3 days, consecutive days within 10km and <7 days
	wps := []Waypoint{
		// Day 1
		{Location: &GPSCoords{Latitude: 52.5200, Longitude: 13.4050}, Timestamp: day1},
		{Location: &GPSCoords{Latitude: 52.5250, Longitude: 13.4100}, Timestamp: day1.Add(2 * time.Hour)},
		// Day 2 (starts near where Day 1 ended)
		{Location: &GPSCoords{Latitude: 52.5255, Longitude: 13.4105}, Timestamp: day2},
		{Location: &GPSCoords{Latitude: 52.5300, Longitude: 13.4150}, Timestamp: day2.Add(3 * time.Hour)},
		// Day 3
		{Location: &GPSCoords{Latitude: 52.5305, Longitude: 13.4155}, Timestamp: day3},
		{Location: &GPSCoords{Latitude: 52.5400, Longitude: 13.4250}, Timestamp: day3.Add(4 * time.Hour)},
	}

	segments := SegmentWaypoints(wps, 10.0, 7*24*time.Hour)

	if len(segments) != 1 {
		t.Fatalf("expected 1 segment for connected multi-day tour, got %d", len(segments))
	}

	seg := segments[0]
	if len(seg.Stages) != 3 {
		t.Fatalf("expected 3 stages for 3 days, got %d", len(seg.Stages))
	}

	// Verify Start and End coords for segment
	if seg.StartCoord[0] != 52.5200 || seg.StartCoord[1] != 13.4050 {
		t.Errorf("unexpected segment startCoord: %v", seg.StartCoord)
	}
	if seg.EndCoord[0] != 52.5400 || seg.EndCoord[1] != 13.4250 {
		t.Errorf("unexpected segment endCoord: %v", seg.EndCoord)
	}
	if seg.DistanceKm <= 0 {
		t.Errorf("expected positive segment distance, got %f", seg.DistanceKm)
	}

	// Stage 1
	if seg.Stages[0].StageNumber != 1 || seg.Stages[0].Date != "2026-08-01" {
		t.Errorf("unexpected stage 1: %+v", seg.Stages[0])
	}
	if seg.Stages[0].StartCoord[0] != 52.5200 || seg.Stages[0].StartCoord[1] != 13.4050 {
		t.Errorf("unexpected stage 1 startCoord: %v", seg.Stages[0].StartCoord)
	}

	// Stage 2
	if seg.Stages[1].StageNumber != 2 || seg.Stages[1].Date != "2026-08-02" {
		t.Errorf("unexpected stage 2: %+v", seg.Stages[1])
	}
	if seg.Stages[1].TotalDistanceKm <= seg.Stages[0].TotalDistanceKm {
		t.Errorf("expected cumulative distance to increase: stage 1 = %f, stage 2 = %f",
			seg.Stages[0].TotalDistanceKm, seg.Stages[1].TotalDistanceKm)
	}

	// Stage 3
	if seg.Stages[2].StageNumber != 3 || seg.Stages[2].Date != "2026-08-03" {
		t.Errorf("unexpected stage 3: %+v", seg.Stages[2])
	}
	if seg.Stages[2].EndCoord[0] != 52.5400 || seg.Stages[2].EndCoord[1] != 13.4250 {
		t.Errorf("unexpected stage 3 endCoord: %v", seg.Stages[2].EndCoord)
	}
}
