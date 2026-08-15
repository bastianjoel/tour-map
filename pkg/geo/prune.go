package geo

const (
	// DefaultMinPruneDistanceKm is the default minimum distance between consecutive retained waypoints (5 meters / 0.005 km).
	DefaultMinPruneDistanceKm = 0.005
)

// PruneWaypoints removes consecutive waypoints that are less than minDistanceKm apart
// while ensuring that the first waypoint is retained in each closely clustered group.
func PruneWaypoints(waypoints []Waypoint, minDistanceKm float64) []Waypoint {
	if len(waypoints) <= 1 {
		return waypoints
	}

	if minDistanceKm <= 0 {
		minDistanceKm = DefaultMinPruneDistanceKm
	}

	pruned := make([]Waypoint, 0, len(waypoints))
	pruned = append(pruned, waypoints[0])

	for i := 1; i < len(waypoints); i++ {
		current := waypoints[i]
		if current.Location == nil {
			continue
		}

		lastKept := pruned[len(pruned)-1]
		if lastKept.Location == nil {
			pruned = append(pruned, current)
			continue
		}

		dist := DistanceKm(
			lastKept.Location.Latitude,
			lastKept.Location.Longitude,
			current.Location.Latitude,
			current.Location.Longitude,
		)

		if dist >= minDistanceKm {
			pruned = append(pruned, current)
		}
	}

	return pruned
}
