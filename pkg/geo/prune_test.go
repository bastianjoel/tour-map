package geo

import (
	"testing"
	"time"
)

func TestPruneWaypoints(t *testing.T) {
	now := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name          string
		waypoints     []Waypoint
		minDistanceKm float64
		expectedCount int
	}{
		{
			name:          "empty slice",
			waypoints:     []Waypoint{},
			minDistanceKm: 0.005,
			expectedCount: 0,
		},
		{
			name: "single waypoint",
			waypoints: []Waypoint{
				{Location: &GPSCoords{Latitude: 40.7128, Longitude: -74.0060}, Timestamp: now},
			},
			minDistanceKm: 0.005,
			expectedCount: 1,
		},
		{
			name: "two waypoints far apart",
			waypoints: []Waypoint{
				{Location: &GPSCoords{Latitude: 40.7128, Longitude: -74.0060}, Timestamp: now},
				{Location: &GPSCoords{Latitude: 34.0522, Longitude: -118.2437}, Timestamp: now.Add(time.Hour)},
			},
			minDistanceKm: 0.005,
			expectedCount: 2,
		},
		{
			name: "consecutive waypoints less than 5 meters apart",
			waypoints: []Waypoint{
				{Location: &GPSCoords{Latitude: 40.7128, Longitude: -74.0060}, Timestamp: now},
				{Location: &GPSCoords{Latitude: 40.7128001, Longitude: -74.0060001}, Timestamp: now.Add(time.Minute)},
				{Location: &GPSCoords{Latitude: 40.7128002, Longitude: -74.0060002}, Timestamp: now.Add(2 * time.Minute)},
			},
			minDistanceKm: 0.005,
			expectedCount: 1,
		},
		{
			name: "mixed cluster and distant point",
			waypoints: []Waypoint{
				{Location: &GPSCoords{Latitude: 40.7128, Longitude: -74.0060}, Timestamp: now},
				{Location: &GPSCoords{Latitude: 40.7128001, Longitude: -74.0060001}, Timestamp: now.Add(time.Minute)},
				{Location: &GPSCoords{Latitude: 40.7128002, Longitude: -74.0060002}, Timestamp: now.Add(2 * time.Minute)},
				{Location: &GPSCoords{Latitude: 40.7200, Longitude: -74.0060}, Timestamp: now.Add(3 * time.Minute)},
			},
			minDistanceKm: 0.005,
			expectedCount: 2,
		},
		{
			name: "multiple clusters",
			waypoints: []Waypoint{
				// Cluster 1
				{Location: &GPSCoords{Latitude: 40.7128, Longitude: -74.0060}, Timestamp: now},
				{Location: &GPSCoords{Latitude: 40.7128001, Longitude: -74.0060001}, Timestamp: now.Add(time.Minute)},
				{Location: &GPSCoords{Latitude: 40.7128002, Longitude: -74.0060002}, Timestamp: now.Add(2 * time.Minute)},
				// Distant point / Cluster 2
				{Location: &GPSCoords{Latitude: 40.7200, Longitude: -74.0060}, Timestamp: now.Add(3 * time.Minute)},
				{Location: &GPSCoords{Latitude: 40.7200001, Longitude: -74.0060001}, Timestamp: now.Add(4 * time.Minute)},
				{Location: &GPSCoords{Latitude: 40.7200002, Longitude: -74.0060002}, Timestamp: now.Add(5 * time.Minute)},
			},
			minDistanceKm: 0.005,
			expectedCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PruneWaypoints(tt.waypoints, tt.minDistanceKm)
			if len(result) != tt.expectedCount {
				t.Errorf("PruneWaypoints() returned %d waypoints, expected %d", len(result), tt.expectedCount)
			}
			if len(tt.waypoints) > 0 && len(result) > 0 {
				if result[0].Location.Latitude != tt.waypoints[0].Location.Latitude ||
					result[0].Location.Longitude != tt.waypoints[0].Location.Longitude {
					t.Errorf("PruneWaypoints() did not retain the first waypoint")
				}
			}
		})
	}
}
