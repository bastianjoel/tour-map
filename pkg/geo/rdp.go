package geo

import (
	"math"
)

// DefaultRDPEpsilonKm is the default maximum perpendicular deviation threshold (2.5 meters / 0.0025 km)
// used to prune collinear/redundant points along paths while preserving full curve detail.
const DefaultRDPEpsilonKm = 0.0025

// perpendicularDistanceKm calculates the perpendicular distance in km from point p to line segment a-b
// using equirectangular projection (highly accurate for local GPS point scales).
func perpendicularDistanceKm(p, a, b [2]float64) float64 {
	const kmPerDegLat = 111.32
	midLatRad := ((a[0] + b[0]) / 2.0) * math.Pi / 180.0
	kmPerDegLng := 111.32 * math.Cos(midLatRad)

	// Convert lat/lng to local km offsets relative to point a
	x1 := 0.0
	y1 := 0.0
	x2 := (b[1] - a[1]) * kmPerDegLng
	y2 := (b[0] - a[0]) * kmPerDegLat
	x0 := (p[1] - a[1]) * kmPerDegLng
	y0 := (p[0] - a[0]) * kmPerDegLat

	dx := x2 - x1
	dy := y2 - y1

	lineLenSq := dx*dx + dy*dy
	if lineLenSq == 0 {
		return math.Hypot(x0-x1, y0-y1)
	}

	// Calculate perpendicular distance to line
	num := math.Abs(dy*x0 - dx*y0)
	return num / math.Sqrt(lineLenSq)
}

// SimplifyCoords applies the Ramer-Douglas-Peucker (RDP) algorithm to simplify a sequence of GPS coordinates.
// Epsilon is the maximum perpendicular deviation threshold in kilometers (e.g. 0.0025 km = 2.5 meters).
func SimplifyCoords(coords [][2]float64, epsilonKm float64) [][2]float64 {
	if len(coords) <= 2 {
		return coords
	}

	if epsilonKm <= 0 {
		epsilonKm = DefaultRDPEpsilonKm
	}

	// Find the point with the maximum distance from the line between first and last points
	maxDist := 0.0
	maxIndex := 0

	first := coords[0]
	last := coords[len(coords)-1]

	for i := 1; i < len(coords)-1; i++ {
		dist := perpendicularDistanceKm(coords[i], first, last)
		if dist > maxDist {
			maxDist = dist
			maxIndex = i
		}
	}

	// If max distance is greater than epsilon, recursively simplify
	if maxDist > epsilonKm {
		recResults1 := SimplifyCoords(coords[:maxIndex+1], epsilonKm)
		recResults2 := SimplifyCoords(coords[maxIndex:], epsilonKm)

		// Merge results, omitting the duplicate middle point
		result := make([][2]float64, 0, len(recResults1)+len(recResults2)-1)
		result = append(result, recResults1[:len(recResults1)-1]...)
		result = append(result, recResults2...)
		return result
	}

	// All intermediate points are within epsilon of the straight line
	return [][2]float64{first, last}
}
