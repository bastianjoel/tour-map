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
)

// Store manages the in-memory cache of waypoints, access codes, and background updates.
type Store struct {
	dataDir        string
	fitDir         string
	codesFile      string
	waypoints      []geo.Waypoint
	latestWaypoint *time.Time
	codes          map[string]struct{}
	wpMutex        sync.RWMutex
	codesMutex     sync.RWMutex
}

// NewStore creates a new waypoint Store.
func NewStore(dataDir, fitDir, codesFile string) *Store {
	return &Store{
		dataDir:   dataDir,
		fitDir:    fitDir,
		codesFile: codesFile,
		waypoints: make([]geo.Waypoint, 0),
		codes:     make(map[string]struct{}),
	}
}

// LoadWaypoints reads JSON files from dataDir and FIT files from fitDir, deduplicating and pruning them.
func (s *Store) LoadWaypoints() error {
	jsonWaypoints := make([]geo.Waypoint, 0)

	// 1. Load JSON files from data directory
	if _, err := os.Stat(s.dataDir); err == nil {
		err := filepath.WalkDir(s.dataDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if !d.IsDir() && strings.HasSuffix(strings.ToLower(path), ".json") {
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

	// 2. Load FIT files from fit directory
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

	// 3. Combine and deduplicate: keep JSON waypoints only if strictly newer than the latest FIT waypoint
	allWaypoints := make([]geo.Waypoint, 0, len(jsonWaypoints)+len(fitWaypoints))
	if len(fitWaypoints) > 0 {
		slices.SortFunc(fitWaypoints, func(a, b geo.Waypoint) int {
			return a.Timestamp.Compare(b.Timestamp)
		})

		latestFitTime := fitWaypoints[len(fitWaypoints)-1].Timestamp
		for _, wp := range jsonWaypoints {
			if wp.Timestamp.After(latestFitTime) {
				allWaypoints = append(allWaypoints, wp)
			}
		}
		allWaypoints = append(allWaypoints, fitWaypoints...)
	} else {
		allWaypoints = append(allWaypoints, jsonWaypoints...)
	}

	// 4. Sort all waypoints chronologically
	slices.SortFunc(allWaypoints, func(a, b geo.Waypoint) int {
		return a.Timestamp.Compare(b.Timestamp)
	})

	// 5. Prune points closer than 5 meters to prevent dense clutter
	prunedWaypoints := geo.PruneWaypoints(allWaypoints, geo.DefaultMinPruneDistanceKm)

	s.wpMutex.Lock()
	defer s.wpMutex.Unlock()

	s.waypoints = prunedWaypoints
	if len(prunedWaypoints) > 0 {
		latest := prunedWaypoints[len(prunedWaypoints)-1].Timestamp
		s.latestWaypoint = &latest
	}

	log.Printf("Loaded %d JSON waypoints, %d FIT waypoints -> %d total waypoints (%d after pruning)",
		len(jsonWaypoints), len(fitWaypoints), len(allWaypoints), len(prunedWaypoints))
	return nil
}

// LoadCodes reads authorized codes from the codes file.
func (s *Store) LoadCodes() error {
	data, err := os.ReadFile(s.codesFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		log.Printf("Error reading codes file %s: %v", s.codesFile, err)
		return err
	}

	codesText := strings.TrimSpace(string(data))
	newCodes := make(map[string]struct{})
	if codesText != "" {
		for _, line := range strings.Split(codesText, "\n") {
			code := strings.TrimSpace(line)
			if code != "" {
				newCodes[code] = struct{}{}
			}
		}
	}

	s.codesMutex.Lock()
	defer s.codesMutex.Unlock()
	s.codes = newCodes
	return nil
}

// IsAuthorized checks if the given code is present in the authorized codes list.
func (s *Store) IsAuthorized(code string) bool {
	if code == "" {
		return false
	}
	s.codesMutex.RLock()
	defer s.codesMutex.RUnlock()

	_, exists := s.codes[code]
	return exists
}

// AddWaypoint appends a new waypoint if it is newer than the latest recorded waypoint.
// It also persists the waypoint as a JSON file in the data directory.
func (s *Store) AddWaypoint(wp geo.Waypoint, rawData []byte) bool {
	if wp.Location == nil {
		return false
	}

	s.wpMutex.Lock()
	defer s.wpMutex.Unlock()

	if s.latestWaypoint != nil && !wp.Timestamp.After(*s.latestWaypoint) {
		return false
	}

	// Check if this new waypoint should be pruned relative to the last kept waypoint
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
