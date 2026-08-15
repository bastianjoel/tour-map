package geo

import (
	"time"
)

const (
	// DefaultMaxDistanceKm is the default maximum distance between points before splitting into a new segment (10km).
	DefaultMaxDistanceKm = 10.0

	// DefaultDottedPauseDistanceKm is the distance threshold (>2.0km) within a single FIT activity
	// where pauses/gaps are connected via dotted lines.
	DefaultDottedPauseDistanceKm = 2.0

	// DefaultMaxTimeGap is the default maximum time duration between points before splitting into a new segment (7 days).
	DefaultMaxTimeGap = 7 * 24 * time.Hour

	// DefaultPrivacyRadiusKm is the radius (10km) around the latest live location trimmed when unauthorized.
	DefaultPrivacyRadiusKm = 10.0
)

// PathLine represents a contiguous line of coordinates with a render type ("solid" or "dotted").
type PathLine struct {
	Type   string       `json:"type"`   // "solid" or "dotted"
	Coords [][2]float64 `json:"coords"` // [lat, lng] pairs
}

// Segment represents a distinct trip segment with start/end time and rendering path lines.
type Segment struct {
	ID        int          `json:"id"`
	StartTime time.Time    `json:"startTime"`
	EndTime   time.Time    `json:"endTime"`
	Lines     []PathLine   `json:"lines,omitempty"`
	Coords    [][2]float64 `json:"coords"` // Flattened coordinates for bounding box calculation
}

// SegmentWaypoints partitions a chronologically sorted slice of waypoints into distinct trip segments.
// - Points from the same FIT activity are connected, rendering gaps > 2km as dotted lines.
// - Points from different activities or live tracking that are > maxDistanceKm or > maxTimeGap apart start a new segment.
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
	var currentStart, currentEnd time.Time
	segmentID := 0

	flushCurrentSegment := func() {
		if len(currentCoords) > 0 {
			if len(currentSolidLine) >= 2 {
				currentLines = append(currentLines, PathLine{
					Type:   "solid",
					Coords: currentSolidLine,
				})
			} else if len(currentSolidLine) == 1 && len(currentLines) == 0 {
				currentLines = append(currentLines, PathLine{
					Type:   "solid",
					Coords: [][2]float64{currentSolidLine[0], currentSolidLine[0]},
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

// FilterPrivacy trims live tracking waypoints within radiusKm of the last recorded live tracking position
// to protect real-time or home location privacy when no access code is provided.
// Historical activity files (such as FIT files with ActivityID != "") are preserved and never trimmed.
func FilterPrivacy(waypoints []Waypoint, radiusKm float64) []Waypoint {
	if len(waypoints) == 0 {
		return waypoints
	}

	if radiusKm <= 0 {
		radiusKm = DefaultPrivacyRadiusKm
	}

	// 1. Find the last live tracking waypoint (ActivityID == "") with a valid location
	var lastLiveWp *Waypoint
	for i := len(waypoints) - 1; i >= 0; i-- {
		if waypoints[i].ActivityID == "" && waypoints[i].Location != nil {
			lastLiveWp = &waypoints[i]
			break
		}
	}

	// If there are no live tracking waypoints, do not trim anything
	if lastLiveWp == nil {
		return waypoints
	}

	// 2. Filter out live tracking points that are within radiusKm of the last live location
	var filtered []Waypoint
	for _, wp := range waypoints {
		// Non-live waypoints (e.g. FIT files) are always kept intact
		if wp.ActivityID != "" {
			filtered = append(filtered, wp)
			continue
		}

		// For live tracking: trim points within privacy radius of the latest live position
		if wp.Location != nil {
			dist := DistanceKm(
				lastLiveWp.Location.Latitude, lastLiveWp.Location.Longitude,
				wp.Location.Latitude, wp.Location.Longitude,
			)
			if dist <= radiusKm {
				continue
			}
		}

		filtered = append(filtered, wp)
	}

	return filtered
}
