package geo

import (
	"time"
)

const (
	// DefaultMaxDistanceKm is the maximum distance between consecutive points before starting a new trip.
	DefaultMaxDistanceKm = 10.0

	// DefaultMaxTimeGap is the maximum time gap between consecutive points before starting a new trip (7 days).
	DefaultMaxTimeGap = 7 * 24 * time.Hour

	// DefaultPrivacyRadiusKm is the radius within the latest waypoint that is hidden without authorization.
	DefaultPrivacyRadiusKm = 10.0
)

// SegmentWaypoints splits a chronological slice of waypoints into separate trip segments
// whenever the distance between consecutive points exceeds maxDistanceKm or the time gap exceeds maxTimeGap.
// It returns a slice of coordinate paths formatted as [][][2]float64 for JSON serialization.
func SegmentWaypoints(waypoints []Waypoint, maxDistanceKm float64, maxTimeGap time.Duration) [][][2]float64 {
	var validWaypoints []Waypoint
	for _, wp := range waypoints {
		if wp.Location != nil {
			validWaypoints = append(validWaypoints, wp)
		}
	}

	if len(validWaypoints) == 0 {
		return [][][2]float64{}
	}

	if maxDistanceKm <= 0 {
		maxDistanceKm = DefaultMaxDistanceKm
	}
	if maxTimeGap <= 0 {
		maxTimeGap = DefaultMaxTimeGap
	}

	var segments [][][2]float64
	var currentSegment [][2]float64

	for i, wp := range validWaypoints {
		coord := [2]float64{wp.Location.Latitude, wp.Location.Longitude}

		if i == 0 {
			currentSegment = append(currentSegment, coord)
			continue
		}

		prevWp := validWaypoints[i-1]
		dist := DistanceKm(
			prevWp.Location.Latitude, prevWp.Location.Longitude,
			wp.Location.Latitude, wp.Location.Longitude,
		)

		timeDiff := wp.Timestamp.Sub(prevWp.Timestamp)
		if timeDiff < 0 {
			timeDiff = -timeDiff
		}

		if dist > maxDistanceKm || timeDiff > maxTimeGap {
			// Disconnect: start a new trip segment
			if len(currentSegment) > 0 {
				segments = append(segments, currentSegment)
			}
			currentSegment = [][2]float64{coord}
		} else {
			currentSegment = append(currentSegment, coord)
		}
	}

	if len(currentSegment) > 0 {
		segments = append(segments, currentSegment)
	}

	return segments
}

// FilterPrivacy trims waypoints within radiusKm of the last recorded waypoint
// to protect real-time or home location privacy when no access code is provided.
func FilterPrivacy(waypoints []Waypoint, radiusKm float64) []Waypoint {
	if len(waypoints) == 0 {
		return waypoints
	}

	if radiusKm <= 0 {
		radiusKm = DefaultPrivacyRadiusKm
	}

	// Find the last waypoint with a valid location
	var lastWp *Waypoint
	for i := len(waypoints) - 1; i >= 0; i-- {
		if waypoints[i].Location != nil {
			lastWp = &waypoints[i]
			break
		}
	}

	if lastWp == nil {
		return waypoints
	}

	i := len(waypoints) - 1
	for ; i >= 0; i-- {
		if waypoints[i].Location == nil {
			continue
		}
		dist := DistanceKm(
			lastWp.Location.Latitude, lastWp.Location.Longitude,
			waypoints[i].Location.Latitude, waypoints[i].Location.Longitude,
		)
		if dist > radiusKm {
			break
		}
	}

	if i < 0 {
		return []Waypoint{}
	}

	return waypoints[:i+1]
}
