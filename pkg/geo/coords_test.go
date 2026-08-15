package geo

import (
	"math"
	"testing"
)

func TestDistanceKm(t *testing.T) {
	tests := []struct {
		name        string
		lat1, lon1  float64
		lat2, lon2  float64
		expectedKm  float64
		toleranceKm float64
	}{
		{
			name:        "Same location",
			lat1:        52.5200,
			lon1:        13.4050,
			lat2:        52.5200,
			lon2:        13.4050,
			expectedKm:  0.0,
			toleranceKm: 0.001,
		},
		{
			name:        "Berlin to Paris",
			lat1:        52.5200,
			lon1:        13.4050,
			lat2:        48.8566,
			lon2:        2.3522,
			expectedKm:  878.0,
			toleranceKm: 5.0,
		},
		{
			name:        "London to Paris",
			lat1:        51.5074,
			lon1:        -0.1278,
			lat2:        48.8566,
			lon2:        2.3522,
			expectedKm:  343.0,
			toleranceKm: 3.0,
		},
		{
			name:        "Small distance (approx 1 km)",
			lat1:        52.5200,
			lon1:        13.4050,
			lat2:        52.5290,
			lon2:        13.4050,
			expectedKm:  1.0,
			toleranceKm: 0.05,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := DistanceKm(tt.lat1, tt.lon1, tt.lat2, tt.lon2)
			if math.Abs(actual-tt.expectedKm) > tt.toleranceKm {
				t.Errorf("DistanceKm() = %v, expected %v (+/- %v)", actual, tt.expectedKm, tt.toleranceKm)
			}
		})
	}
}
