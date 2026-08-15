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

	// DefaultDottedPauseDistanceKm is the threshold (>2km) for pauses/gaps within a single FIT activity to be connected via dotted lines.
	DefaultDottedPauseDistanceKm = 2.0
)

// PathLine represents a continuous sub-path that is either "solid" (regular ride) or "dotted" (gaps/pauses > 2km within a FIT activity).
type PathLine struct {
	Type   string       `json:"type"` // "solid" or "dotted"
	Coords [][2]float64 `json:"coords"`
}

// Segment represents a distinct connected trip segment with its timeframe, sub-lines, and full coordinates list.
type Segment struct {
	ID        int          `json:"id"`
	StartTime time.Time    `json:"startTime"`
	EndTime   time.Time    `json:"endTime"`
	Lines     []PathLine   `json:"lines"`
	Coords    [][2]float64 `json:"coords"`
}

// SegmentWaypoints splits a chronological slice of waypoints into trip segments.
// - Points originating from the SAME single FIT activity (matching non-empty ActivityID) are always kept in the same segment.
//   If a pause/gap between consecutive points in that activity exceeds DefaultDottedPauseDistanceKm (2km), they are connected via a "dotted" line.
// - Points from different sources/activities separated by >maxDistanceKm or >maxTimeGap are split into separate trip segments.
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
	var currentLines []PathLine
	var currentSolidLine [][2]float64
	var currentStart time.Time
	var currentEnd time.Time
	segmentID := 0

	flushCurrentSegment := func() {
		if len(currentSolidLine) >= 2 {
			currentLines = append(currentLines, PathLine{
				Type:   "solid",
				Coords: currentSolidLine,
			})
		}
		if len(currentCoords) > 0 {
			// If there were points but no lines with >=2 points (e.g. single point), create a solid line
			if len(currentLines) == 0 && len(currentCoords) >= 2 {
				currentLines = append(currentLines, PathLine{
					Type:   "solid",
					Coords: currentCoords,
				})
			}
			segments = append(segments, Segment{
				ID:        segmentID,
				StartTime: currentStart,
				EndTime:   currentEnd,
				Lines:     currentLines,
				Coords:    currentCoords,
			})
			segmentID++
		}
		currentCoords = nil
		currentLines = nil
		currentSolidLine = nil
	}

	for i, wp := range validWaypoints {
		coord := [2]float64{wp.Location.Latitude, wp.Location.Longitude}

		if i == 0 {
			currentCoords = append(currentCoords, coord)
			currentSolidLine = append(currentSolidLine, coord)
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

		sameActivity := wp.ActivityID != "" && wp.ActivityID == prevWp.ActivityID

		if sameActivity {
			// Inside the SAME single FIT activity: always connect!
			// Pauses/gaps > 2km appear as dotted lines.
			if dist > DefaultDottedPauseDistanceKm {
				// Flush current solid line
				if len(currentSolidLine) >= 2 {
					currentLines = append(currentLines, PathLine{
						Type:   "solid",
						Coords: currentSolidLine,
					})
				}
				// Connect the gap with a dotted line between prev point and current point
				prevCoord := [2]float64{prevWp.Location.Latitude, prevWp.Location.Longitude}
				currentLines = append(currentLines, PathLine{
					Type:   "dotted",
					Coords: [][2]float64{prevCoord, coord},
				})
				// Start new solid line at current point
				currentSolidLine = [][2]float64{coord}
			} else {
				currentSolidLine = append(currentSolidLine, coord)
			}
			currentCoords = append(currentCoords, coord)
			currentEnd = wp.Timestamp
		} else {
			// Different activities or live tracking
			if dist > maxDistanceKm || timeDiff > maxTimeGap {
				// Disconnect: start a new trip segment
				flushCurrentSegment()
				currentCoords = [][2]float64{coord}
				currentSolidLine = [][2]float64{coord}
				currentStart = wp.Timestamp
				currentEnd = wp.Timestamp
			} else {
				currentSolidLine = append(currentSolidLine, coord)
				currentCoords = append(currentCoords, coord)
				currentEnd = wp.Timestamp
			}
		}
	}

	flushCurrentSegment()
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
