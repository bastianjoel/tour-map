package tracker

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"tour-map/pkg/geo"
	"tour-map/pkg/images"
)

// Store manages all recorded GPS waypoints and access authorization codes in a thread-safe manner.
type Store struct {
	dataDir        string
	fitDir         string
	codesFile      string
	waypoints      []geo.Waypoint
	codes          map[string]bool
	latestWaypoint *time.Time
	wpMutex        sync.RWMutex
	codeMutex      sync.RWMutex
}

// NewStore creates a new Store instance.
func NewStore(dataDir, fitDir, codesFile string) *Store {
	return &Store{
		dataDir:   dataDir,
		fitDir:    fitDir,
		codesFile: codesFile,
		waypoints: make([]geo.Waypoint, 0),
		codes:     make(map[string]bool),
	}
}

// LoadCodes reads authorization codes from the codes file.
func (s *Store) LoadCodes() error {
	if s.codesFile == "" {
		return nil
	}

	data, err := os.ReadFile(s.codesFile)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	newCodes := make(map[string]bool)
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			newCodes[trimmed] = true
		}
	}

	s.codeMutex.Lock()
	defer s.codeMutex.Unlock()
	s.codes = newCodes
	return nil
}

// IsAuthorized checks if the given code is in the authorized codes list.
// If no codes are configured, access is restricted by default.
func (s *Store) IsAuthorized(code string) bool {
	s.codeMutex.RLock()
	defer s.codeMutex.RUnlock()

	if len(s.codes) == 0 {
		return false
	}
	return s.codes[code]
}

// LoadWaypoints reads all JSON tracking files from dataDir and FIT files from fitDir,
// parses and deduplicates/prunes them, and stores them chronologically.
func (s *Store) LoadWaypoints() error {
	// 1. Load JSON tracking files from data directory
	jsonWaypoints := make([]geo.Waypoint, 0)
	if s.dataDir != "" {
		err := filepath.WalkDir(s.dataDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if !d.IsDir() && strings.HasPrefix(d.Name(), "tracking_") && strings.HasSuffix(d.Name(), ".json") {
				data, err := os.ReadFile(path)
				if err != nil {
					log.Printf("Error reading JSON file %s: %v", path, err)
					return nil
				}

				var wp geo.Waypoint
				if err := json.Unmarshal(data, &wp); err != nil {
					log.Printf("Error parsing JSON file %s: %v", path, err)
					return nil
				}

				if wp.Location != nil {
					jsonWaypoints = append(jsonWaypoints, wp)
				}
			}

			return nil
		})

		if err != nil {
			log.Printf("Error walking data directory: %v", err)
		}
	}

	// 2. Load FIT files from fit directory recursively
	fitWaypoints := make([]geo.Waypoint, 0)
	if s.fitDir != "" {
		if _, err := os.Stat(s.fitDir); err == nil {
			err := filepath.WalkDir(s.fitDir, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}

				if !d.IsDir() && strings.HasSuffix(strings.ToLower(path), ".fit") {
					wps, err := ParseFitFile(path)
					if err != nil {
						log.Printf("Error parsing FIT file %s: %v", path, err)
						return nil
					}
					fitWaypoints = append(fitWaypoints, wps...)
				}

				return nil
			})

			if err != nil {
				log.Printf("Error walking fit directory: %v", err)
			}
		}
	}

	// Combine all loaded waypoints
	combined := append(jsonWaypoints, fitWaypoints...)

	// Sort chronologically by timestamp
	slices.SortFunc(combined, func(a, b geo.Waypoint) int {
		return a.Timestamp.Compare(b.Timestamp)
	})

	// Prune dense clusters closer than 5 meters
	pruned := geo.PruneWaypoints(combined, geo.DefaultMinPruneDistanceKm)

	s.wpMutex.Lock()
	defer s.wpMutex.Unlock()

	s.waypoints = pruned
	if len(pruned) > 0 {
		s.latestWaypoint = &pruned[len(pruned)-1].Timestamp
	}

	log.Printf("Loaded %d JSON waypoints, %d FIT waypoints -> %d total waypoints (%d after pruning)",
		len(jsonWaypoints), len(fitWaypoints), len(combined), len(pruned))
	return nil
}

// AddWaypoint appends a newly polled waypoint to the store and writes it to disk if appropriate.
// Returns false if the waypoint was ignored (e.g. invalid or redundant).
func (s *Store) AddWaypoint(wp geo.Waypoint, rawData []byte) bool {
	if wp.Location == nil {
		return false
	}

	s.wpMutex.Lock()
	defer s.wpMutex.Unlock()

	// Check if already present or older than latest
	if s.latestWaypoint != nil && (wp.Timestamp.Before(*s.latestWaypoint) || wp.Timestamp.Equal(*s.latestWaypoint)) {
		return false
	}

	// Check if point is closer than 5 meters to the last point
	if len(s.waypoints) > 0 {
		lastKept := s.waypoints[len(s.waypoints)-1]
		if lastKept.Location != nil {
			dist := geo.DistanceKm(
				lastKept.Location.Latitude, lastKept.Location.Longitude,
				wp.Location.Latitude, wp.Location.Longitude,
			)
			if dist < geo.DefaultMinPruneDistanceKm {
				s.latestWaypoint = &wp.Timestamp
				if len(rawData) > 0 && s.dataDir != "" {
					filename := fmt.Sprintf("%s/tracking_%s.json", s.dataDir, wp.Timestamp.Format("20060102_150405"))
					os.WriteFile(filename, rawData, 0644)
				}
				return true
			}
		}
	}

	s.waypoints = append(s.waypoints, wp)
	s.latestWaypoint = &wp.Timestamp

	if len(rawData) > 0 && s.dataDir != "" {
		filename := fmt.Sprintf("%s/tracking_%s.json", s.dataDir, wp.Timestamp.Format("20060102_150405"))
		if err := os.WriteFile(filename, rawData, 0644); err != nil {
			log.Printf("Error saving tracking file %s: %v", filename, err)
		}
	}

	return true
}

// GetWaypoints returns a copy of all loaded waypoints.
func (s *Store) GetWaypoints() []geo.Waypoint {
	s.wpMutex.RLock()
	defer s.wpMutex.RUnlock()

	res := make([]geo.Waypoint, len(s.waypoints))
	copy(res, s.waypoints)
	return res
}

// InterpolateImageLocations enriches images that lack GPS coordinates by calculating their
// estimated location along the route using their timestamps.
func (s *Store) InterpolateImageLocations(imageList []images.ImageInfo) []images.ImageInfo {
	wps := s.GetWaypoints()
	if len(wps) == 0 || len(imageList) == 0 {
		return imageList
	}

	res := make([]images.ImageInfo, len(imageList))
	for i, img := range imageList {
		res[i] = img
		if res[i].Location == nil && !res[i].Timestamp.IsZero() {
			if loc := geo.InterpolateLocation(wps, res[i].Timestamp, geo.DefaultMaxInterpolationBuffer); loc != nil {
				res[i].Location = loc
			}
		}
	}
	return res
}

// GetTripSegments returns waypoints partitioned into distinct trip segments.
// If the access code is not authorized, the 10km tail of the latest trip is filtered for privacy.
func (s *Store) GetTripSegments(code string) []geo.Segment {
	wps := s.GetWaypoints()

	if !s.IsAuthorized(code) {
		wps = geo.FilterPrivacy(wps, geo.DefaultPrivacyRadiusKm)
	}

	return geo.SegmentWaypoints(wps, geo.DefaultMaxDistanceKm, geo.DefaultMaxTimeGap)
}

// GetUpdates returns trip segments and the last modified timestamp for points after the given since timestamp.
func (s *Store) GetUpdates(since time.Time, code string) ([]geo.Segment, time.Time) {
	wps := s.GetWaypoints()

	if !s.IsAuthorized(code) {
		wps = geo.FilterPrivacy(wps, geo.DefaultPrivacyRadiusKm)
	}

	var lastModified time.Time
	if len(wps) > 0 {
		lastModified = wps[len(wps)-1].Timestamp
	}

	if !since.IsZero() {
		var filtered []geo.Waypoint
		for _, wp := range wps {
			if wp.Timestamp.After(since) {
				filtered = append(filtered, wp)
			}
		}
		wps = filtered
	}

	segments := geo.SegmentWaypoints(wps, geo.DefaultMaxDistanceKm, geo.DefaultMaxTimeGap)
	return segments, lastModified
}
