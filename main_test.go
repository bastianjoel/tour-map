package main

import (
	"testing"
	"time"
)

func TestPruneWaypoints(t *testing.T) {
	tests := []struct {
		name            string
		waypoints       []Waypoint
		expectedCount   int
		description     string
	}{
		{
			name:            "empty slice",
			waypoints:       []Waypoint{},
			expectedCount:   0,
			description:     "empty slice should return empty slice",
		},
		{
			name: "single waypoint",
			waypoints: []Waypoint{
				{
					Location:  &GPSCoords{Latitude: 40.7128, Longitude: -74.0060},
					Timestamp: time.Now(),
				},
			},
			expectedCount: 1,
			description:   "single waypoint should be retained",
		},
		{
			name: "two waypoints far apart",
			waypoints: []Waypoint{
				{
					Location:  &GPSCoords{Latitude: 40.7128, Longitude: -74.0060}, // NYC
					Timestamp: time.Now(),
				},
				{
					Location:  &GPSCoords{Latitude: 34.0522, Longitude: -118.2437}, // LA
					Timestamp: time.Now().Add(time.Hour),
				},
			},
			expectedCount: 2,
			description:   "waypoints far apart should both be retained",
		},
		{
			name: "consecutive waypoints less than 5 meters apart",
			waypoints: []Waypoint{
				{
					Location:  &GPSCoords{Latitude: 40.7128, Longitude: -74.0060},
					Timestamp: time.Now(),
				},
				{
					Location:  &GPSCoords{Latitude: 40.7128001, Longitude: -74.0060001}, // ~1 meter away
					Timestamp: time.Now().Add(time.Minute),
				},
				{
					Location:  &GPSCoords{Latitude: 40.7128002, Longitude: -74.0060002}, // ~2 meters from first
					Timestamp: time.Now().Add(2 * time.Minute),
				},
			},
			expectedCount: 1,
			description:   "close consecutive waypoints should be pruned to keep only the first",
		},
		{
			name: "mixed distances with cluster and distant point",
			waypoints: []Waypoint{
				{
					Location:  &GPSCoords{Latitude: 40.7128, Longitude: -74.0060},
					Timestamp: time.Now(),
				},
				{
					Location:  &GPSCoords{Latitude: 40.7128001, Longitude: -74.0060001}, // ~1 meter away
					Timestamp: time.Now().Add(time.Minute),
				},
				{
					Location:  &GPSCoords{Latitude: 40.7128002, Longitude: -74.0060002}, // ~2 meters from first
					Timestamp: time.Now().Add(2 * time.Minute),
				},
				{
					Location:  &GPSCoords{Latitude: 40.7200, Longitude: -74.0060}, // ~800 meters away
					Timestamp: time.Now().Add(3 * time.Minute),
				},
			},
			expectedCount: 2,
			description:   "should keep first waypoint of cluster and distant waypoint",
		},
		{
			name: "waypoints exactly 5 meters apart",
			waypoints: []Waypoint{
				{
					Location:  &GPSCoords{Latitude: 40.7128, Longitude: -74.0060},
					Timestamp: time.Now(),
				},
				{
					Location:  &GPSCoords{Latitude: 40.712845, Longitude: -74.0060}, // approximately 5 meters north
					Timestamp: time.Now().Add(time.Minute),
				},
			},
			expectedCount: 2,
			description:   "waypoints exactly at 5 meter threshold should be retained",
		},
		{
			name: "multiple clusters",
			waypoints: []Waypoint{
				// First cluster
				{
					Location:  &GPSCoords{Latitude: 40.7128, Longitude: -74.0060},
					Timestamp: time.Now(),
				},
				{
					Location:  &GPSCoords{Latitude: 40.7128001, Longitude: -74.0060001}, // ~1 meter
					Timestamp: time.Now().Add(time.Minute),
				},
				{
					Location:  &GPSCoords{Latitude: 40.7128002, Longitude: -74.0060002}, // ~2 meters from first
					Timestamp: time.Now().Add(2 * time.Minute),
				},
				// Distant point
				{
					Location:  &GPSCoords{Latitude: 40.7200, Longitude: -74.0060}, // ~800 meters away
					Timestamp: time.Now().Add(3 * time.Minute),
				},
				// Second cluster
				{
					Location:  &GPSCoords{Latitude: 40.7200001, Longitude: -74.0060001}, // ~1 meter from previous
					Timestamp: time.Now().Add(4 * time.Minute),
				},
				{
					Location:  &GPSCoords{Latitude: 40.7200002, Longitude: -74.0060002}, // ~2 meters from previous kept
					Timestamp: time.Now().Add(5 * time.Minute),
				},
			},
			expectedCount: 2,
			description:   "should keep one waypoint from each cluster",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pruneWaypoints(tt.waypoints)
			if len(result) != tt.expectedCount {
				t.Errorf("pruneWaypoints() returned %d waypoints, expected %d. %s", len(result), tt.expectedCount, tt.description)
			}

			// Verify that the first waypoint is always retained (if any waypoints exist)
			if len(tt.waypoints) > 0 && len(result) > 0 {
				if result[0].Location.Latitude != tt.waypoints[0].Location.Latitude ||
					result[0].Location.Longitude != tt.waypoints[0].Location.Longitude {
					t.Errorf("pruneWaypoints() did not retain the first waypoint")
				}
			}
		})
	}
}

func TestDistanceKm(t *testing.T) {
	tests := []struct {
		name      string
		lat1      float64
		lon1      float64
		lat2      float64
		lon2      float64
		expected  float64
		tolerance float64
	}{
		{
			name:      "same point",
			lat1:      40.7128,
			lon1:      -74.0060,
			lat2:      40.7128,
			lon2:      -74.0060,
			expected:  0.0,
			tolerance: 0.001,
		},
		{
			name:      "approximately 1 meter",
			lat1:      40.7128,
			lon1:      -74.0060,
			lat2:      40.7128001,
			lon2:      -74.0060001,
			expected:  0.000157, // approximately 0.157 meters
			tolerance: 0.001,
		},
		{
			name:      "approximately 5 meters",
			lat1:      40.7128,
			lon1:      -74.0060,
			lat2:      40.712845,
			lon2:      -74.0060,
			expected:  0.005, // 5 meters
			tolerance: 0.001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := distanceKm(tt.lat1, tt.lon1, tt.lat2, tt.lon2)
			if abs(result-tt.expected) > tt.tolerance {
				t.Errorf("distanceKm() = %v, expected %v (±%v)", result, tt.expected, tt.tolerance)
			}
		})
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}