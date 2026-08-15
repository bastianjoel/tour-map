package geo

import (
	"math"
	"time"
)

// GPSCoords represents latitude and longitude coordinates.
type GPSCoords struct {
	Latitude  float64 `json:"lat"`
	Longitude float64 `json:"lng"`
}

// Waypoint represents a tracking location point with an update timestamp and optional ActivityID.
type Waypoint struct {
	Location   *GPSCoords `json:"location,omitempty"`
	Timestamp  time.Time  `json:"updatedAt"`
	ActivityID string     `json:"activityId,omitempty"`
}

// DistanceKm calculates the great-circle distance between two GPS coordinates in kilometers using the Haversine formula.
func DistanceKm(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusKm = 6371.0

	lat1Rad := lat1 * math.Pi / 180.0
	lon1Rad := lon1 * math.Pi / 180.0
	lat2Rad := lat2 * math.Pi / 180.0
	lon2Rad := lon2 * math.Pi / 180.0

	dLat := lat2Rad - lat1Rad
	dLon := lon2Rad - lon1Rad

	a := math.Sin(dLat/2.0)*math.Sin(dLat/2.0) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*
			math.Sin(dLon/2.0)*math.Sin(dLon/2.0)

	c := 2.0 * math.Atan2(math.Sqrt(a), math.Sqrt(1.0-a))
	return earthRadiusKm * c
}
