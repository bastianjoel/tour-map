package geo

import (
	"slices"
	"time"
)

const (
	// DefaultMaxInterpolationBuffer is the maximum time difference allowed between a photo
	// and a tour start/end point to clamp its position to that endpoint.
	DefaultMaxInterpolationBuffer = 1 * time.Hour
)

// InterpolateLocation estimates the GPS location for a target timestamp based on a slice of waypoints.
// If the target time falls between two consecutive waypoints on the same trip segment,
// it computes linear interpolation based on time.
// If the target time is within maxTimeBuffer before the start or after the end of a segment,
// it clamps to the corresponding endpoint location.
// Returns nil if no valid location can be determined within maxTimeBuffer.
func InterpolateLocation(waypoints []Waypoint, targetTime time.Time, maxTimeBuffer time.Duration) *GPSCoords {
	if targetTime.IsZero() || len(waypoints) == 0 {
		return nil
	}

	if maxTimeBuffer <= 0 {
		maxTimeBuffer = DefaultMaxInterpolationBuffer
	}

	// Filter waypoints with valid locations
	var validWps []Waypoint
	for _, wp := range waypoints {
		if wp.Location != nil {
			validWps = append(validWps, wp)
		}
	}

	if len(validWps) == 0 {
		return nil
	}

	// Ensure waypoints are sorted chronologically
	slices.SortFunc(validWps, func(a, b Waypoint) int {
		return a.Timestamp.Compare(b.Timestamp)
	})

	n := len(validWps)

	// Case 1: Target time is before the first waypoint
	if targetTime.Before(validWps[0].Timestamp) {
		if validWps[0].Timestamp.Sub(targetTime) <= maxTimeBuffer {
			loc := *validWps[0].Location
			return &loc
		}
		return nil
	}

	// Case 2: Target time is after the last waypoint
	if targetTime.After(validWps[n-1].Timestamp) {
		if targetTime.Sub(validWps[n-1].Timestamp) <= maxTimeBuffer {
			loc := *validWps[n-1].Location
			return &loc
		}
		return nil
	}

	// Case 3: Target time falls within the waypoints range
	for i := 0; i < n-1; i++ {
		w1 := validWps[i]
		w2 := validWps[i+1]

		if targetTime.Equal(w1.Timestamp) {
			loc := *w1.Location
			return &loc
		}
		if targetTime.Equal(w2.Timestamp) {
			loc := *w2.Location
			return &loc
		}

		if targetTime.After(w1.Timestamp) && targetTime.Before(w2.Timestamp) {
			timeDiff := w2.Timestamp.Sub(w1.Timestamp)

			// If points are within maxTimeBuffer*2 (or same activity), interpolate linearly
			if timeDiff <= 2*maxTimeBuffer || (w1.ActivityID != "" && w1.ActivityID == w2.ActivityID) {
				ratio := float64(targetTime.Sub(w1.Timestamp)) / float64(timeDiff)
				lat := w1.Location.Latitude + ratio*(w2.Location.Latitude-w1.Location.Latitude)
				lng := w1.Location.Longitude + ratio*(w2.Location.Longitude-w1.Location.Longitude)

				return &GPSCoords{
					Latitude:  lat,
					Longitude: lng,
				}
			}

			// In a larger gap between separate trips: check if within buffer of w1 or w2
			if targetTime.Sub(w1.Timestamp) <= maxTimeBuffer {
				loc := *w1.Location
				return &loc
			}
			if w2.Timestamp.Sub(targetTime) <= maxTimeBuffer {
				loc := *w2.Location
				return &loc
			}

			return nil
		}
	}

	return nil
}
