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

// Segment represents a distinct connected trip segment with its timeframe and coordinates.
type Segment struct {
	ID        int          `json:"id"`
	StartTime time.Time    `json:"startTime"`
	EndTime   time.Time    `json:"endTime"`
	Coords    [][2]float64 `json:"coords"`
}

// SegmentWaypoints splits a chronological slice of waypoints into separate trip segments
// whenever the distance between consecutive points exceeds maxDistanceKm or the time gap exceeds maxTimeGap.
func SegmentWaypoints(waypoints []Waypoint, maxDistanceKm float64, maxTimeGap time.Duration) []Segment {
	var validWaypoints []Waypoint
	for _, wp := range waypoints {
		if wp.Location != nil {
			validWaypoints = append(validWaypoints, wp)
		}
	}

	if len(validWaypoints) == 0 {
		return []Segment{}
	}

	if maxDistanceKm <= 0 {
		maxDistanceKm = DefaultMaxDistanceKm
	}
	if maxTimeGap <= 0 {
		maxTimeGap = DefaultMaxTimeGap
	}

	var segments []Segment
	var currentCoords [][2]float64
	var currentStart time.Time
	var currentEnd time.Time
	segmentID := 0

	for i, wp := range validWaypoints {
		coord := [2]float64{wp.Location.Latitude, wp.Location.Longitude}

		if i == 0 {
			currentCoords = append(currentCoords, coord)
			currentStart = wp.Timestamp
			currentEnd = wp.Timestamp
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
			// Disconnect: close current segment and start a new trip segment
			if len(currentCoords) > 0 {
				segments = append(segments, Segment{
					ID:        segmentID,
					StartTime: currentStart,
					EndTime:   currentEnd,
					Coords:    currentCoords,
				})
				segmentID++
			}
			currentCoords = [][2]float64{coord}
			currentStart = wp.Timestamp
			currentEnd = wp.Timestamp
		} else {
			currentCoords = append(currentCoords, coord)
			currentEnd = wp.Timestamp
		}
	}

	if len(currentCoords) > 0 {
		segments = append(segments, Segment{
			ID:        segmentID,
			StartTime: currentStart,
			EndTime:   currentEnd,
			Coords:    currentCoords,
		})
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
