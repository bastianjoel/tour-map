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

// Store manages the in-memory cache of waypoints and access codes.
type Store struct {
	dataDir        string
	codesFile      string
	waypoints      []geo.Waypoint
	latestWaypoint *time.Time
	codes          map[string]struct{}
	wpMutex        sync.RWMutex
	codesMutex     sync.RWMutex
}

// NewStore creates a new waypoint Store.
func NewStore(dataDir, codesFile string) *Store {
	return &Store{
		dataDir:   dataDir,
		codesFile: codesFile,
		waypoints: make([]geo.Waypoint, 0),
		codes:     make(map[string]struct{}),
	}
}

// LoadWaypoints reads all JSON files in the data directory and loads them into memory sorted by timestamp.
func (s *Store) LoadWaypoints() error {
	nextPathData := make([]geo.Waypoint, 0)

	if _, err := os.Stat(s.dataDir); os.IsNotExist(err) {
		return nil
	}

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
				nextPathData = append(nextPathData, wp)
			}
		}

		return nil
	})

	if err != nil {
		log.Printf("Error walking data directory: %v", err)
		return err
	}

	slices.SortFunc(nextPathData, func(a, b geo.Waypoint) int {
		return a.Timestamp.Compare(b.Timestamp)
	})

	s.wpMutex.Lock()
	defer s.wpMutex.Unlock()

	s.waypoints = nextPathData
	if len(nextPathData) > 0 {
		latest := nextPathData[len(nextPathData)-1].Timestamp
		s.latestWaypoint = &latest
	}

	log.Printf("Loaded %d JSON files from %s", len(nextPathData), s.dataDir)
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

	s.waypoints = append(s.waypoints, wp)
	s.latestWaypoint = &wp.Timestamp

	if len(rawData) > 0 {
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
func (s *Store) GetTripSegments(code string) [][][2]float64 {
	wps := s.GetWaypoints()

	if !s.IsAuthorized(code) {
		wps = geo.FilterPrivacy(wps, geo.DefaultPrivacyRadiusKm)
	}

	return geo.SegmentWaypoints(wps, geo.DefaultMaxDistanceKm, geo.DefaultMaxTimeGap)
}
