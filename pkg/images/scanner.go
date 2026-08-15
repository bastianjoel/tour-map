package images

import (
	"io/fs"
	"log"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rwcarlsen/goexif/exif"
	"tour-map/pkg/geo"
)

// Scanner scans an images directory and extracts GPS coordinates from image EXIF metadata.
type Scanner struct {
	imagesDir string
	locations map[string]geo.GPSCoords
	mu        sync.RWMutex
}

// NewScanner creates a new Scanner for the given directory.
func NewScanner(imagesDir string) *Scanner {
	return &Scanner{
		imagesDir: imagesDir,
		locations: make(map[string]geo.GPSCoords),
	}
}

// IsImageFile checks if the filename has a supported image extension.
func IsImageFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return ext == ".jpg" || ext == ".jpeg" || ext == ".tiff" || ext == ".tif"
}

// ExtractGPSCoords extracts GPS coordinates from an image's EXIF data.
func ExtractGPSCoords(imagePath string) (*geo.GPSCoords, error) {
	file, err := os.Open(imagePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Decode EXIF data
	x, err := exif.Decode(file)
	if err != nil {
		return nil, err // No EXIF data or corrupted
	}

	// Get GPS coordinates
	lat, lon, err := x.LatLong()
	if err != nil {
		return nil, err // No GPS data
	}

	return &geo.GPSCoords{
		Latitude:  lat,
		Longitude: lon,
	}, nil
}

// Scan walks the images directory and updates the GPS locations map.
func (s *Scanner) Scan() error {
	newGPSData := make(map[string]geo.GPSCoords)

	if _, err := os.Stat(s.imagesDir); os.IsNotExist(err) {
		return nil
	}

	err := filepath.WalkDir(s.imagesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && IsImageFile(path) {
			coords, err := ExtractGPSCoords(path)
			if err != nil {
				log.Printf("Error extracting GPS from %s: %v", filepath.Base(path), err)
				return nil
			}

			if coords != nil {
				filename := filepath.Base(path)
				newGPSData[filename] = *coords
			}
		}

		return nil
	})

	if err != nil {
		log.Printf("Error walking images directory: %v", err)
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.locations = newGPSData
	return nil
}

// GetLocations returns a copy of the detected image GPS coordinates.
func (s *Scanner) GetLocations() map[string]geo.GPSCoords {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res := make(map[string]geo.GPSCoords, len(s.locations))
	maps.Copy(res, s.locations)
	return res
}

// StartPeriodicScan runs Scan periodically until the stop channel receives a signal.
func (s *Scanner) StartPeriodicScan(interval time.Duration, stopCh <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.Scan()
		case <-stopCh:
			return
		}
	}
}
