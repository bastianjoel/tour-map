package images

import (
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/rwcarlsen/goexif/exif"
	"tour-map/pkg/geo"
)

// ImageInfo holds metadata about an image, including filename, optional GPS location, and timestamp.
type ImageInfo struct {
	Filename  string         `json:"filename"`
	Location  *geo.GPSCoords `json:"location,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
}

// Scanner scans an images directory and extracts EXIF metadata and GPS coordinates.
type Scanner struct {
	imagesDir string
	images    []ImageInfo
	mu        sync.RWMutex
}

// NewScanner creates a new Scanner for the given directory.
func NewScanner(imagesDir string) *Scanner {
	return &Scanner{
		imagesDir: imagesDir,
		images:    make([]ImageInfo, 0),
	}
}

// IsImageFile checks if the filename has a supported image extension.
func IsImageFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return ext == ".jpg" || ext == ".jpeg" || ext == ".tiff" || ext == ".tif"
}

// ExtractImageInfo extracts GPS coordinates and timestamp from an image file and its EXIF data.
func ExtractImageInfo(imagePath string, info fs.FileInfo) ImageInfo {
	filename := filepath.Base(imagePath)
	var timestamp time.Time
	if info != nil {
		timestamp = info.ModTime()
	}

	file, err := os.Open(imagePath)
	if err != nil {
		return ImageInfo{
			Filename:  filename,
			Timestamp: timestamp,
		}
	}
	defer file.Close()

	x, err := exif.Decode(file)
	if err != nil {
		return ImageInfo{
			Filename:  filename,
			Timestamp: timestamp,
		}
	}

	// Extract timestamp from EXIF if available
	if exifTime, err := x.DateTime(); err == nil && !exifTime.IsZero() {
		timestamp = exifTime
	}

	// Extract GPS coordinates if available
	var loc *geo.GPSCoords
	if lat, lon, err := x.LatLong(); err == nil {
		loc = &geo.GPSCoords{
			Latitude:  lat,
			Longitude: lon,
		}
	}

	return ImageInfo{
		Filename:  filename,
		Location:  loc,
		Timestamp: timestamp,
	}
}

// Scan walks the images directory, extracts metadata for all images, and sorts them by date.
func (s *Scanner) Scan() error {
	newImages := make([]ImageInfo, 0)

	if _, err := os.Stat(s.imagesDir); os.IsNotExist(err) {
		return nil
	}

	err := filepath.WalkDir(s.imagesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && IsImageFile(path) {
			info, err := d.Info()
			if err != nil {
				log.Printf("Error getting file info for %s: %v", path, err)
			}

			imgInfo := ExtractImageInfo(path, info)
			newImages = append(newImages, imgInfo)
		}

		return nil
	})

	if err != nil {
		log.Printf("Error walking images directory: %v", err)
		return err
	}

	// Sort images chronologically by date
	slices.SortFunc(newImages, func(a, b ImageInfo) int {
		return a.Timestamp.Compare(b.Timestamp)
	})

	s.mu.Lock()
	defer s.mu.Unlock()
	s.images = newImages
	return nil
}

// GetImages returns a sorted copy of all discovered images.
func (s *Scanner) GetImages() []ImageInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res := make([]ImageInfo, len(s.images))
	copy(res, s.images)
	return res
}

// GetLocations returns a map of filename to GPSCoords for images that have GPS data.
func (s *Scanner) GetLocations() map[string]geo.GPSCoords {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res := make(map[string]geo.GPSCoords)
	for _, img := range s.images {
		if img.Location != nil {
			res[img.Filename] = *img.Location
		}
	}
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
