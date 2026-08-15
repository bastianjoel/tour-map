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

// ImageInfo holds metadata about an image, including relative filename path, optional GPS location, and timestamp.
type ImageInfo struct {
	Filename  string         `json:"filename"`
	Location  *geo.GPSCoords `json:"location,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
}

// Scanner scans a raw images directory recursively, compresses images into a compressed directory,
// and extracts EXIF metadata and GPS coordinates.
type Scanner struct {
	rawDir        string
	compressedDir string
	images        []ImageInfo
	mu            sync.RWMutex
}

// NewScanner creates a new Scanner for the given raw and compressed image directories.
func NewScanner(rawDir, compressedDir string) *Scanner {
	return &Scanner{
		rawDir:        rawDir,
		compressedDir: compressedDir,
		images:        make([]ImageInfo, 0),
	}
}

// IsImageFile checks if the filename has a supported image extension.
func IsImageFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return ext == ".jpg" || ext == ".jpeg" || ext == ".tiff" || ext == ".tif" || ext == ".png"
}

// ExtractImageInfo extracts GPS coordinates and timestamp from an image file and its EXIF data.
func ExtractImageInfo(imagePath, relFilename string, info fs.FileInfo) ImageInfo {
	if relFilename == "" {
		relFilename = filepath.Base(imagePath)
	}

	var timestamp time.Time
	if info != nil {
		timestamp = info.ModTime()
	}

	file, err := os.Open(imagePath)
	if err != nil {
		return ImageInfo{
			Filename:  relFilename,
			Timestamp: timestamp,
		}
	}
	defer file.Close()

	x, err := exif.Decode(file)
	if err != nil {
		return ImageInfo{
			Filename:  relFilename,
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
		Filename:  relFilename,
		Location:  loc,
		Timestamp: timestamp,
	}
}

// Scan walks the raw images directory recursively, compresses new/updated images into the compressed directory,
// extracts metadata, and sorts images chronologically by date.
func (s *Scanner) Scan() error {
	newImages := make([]ImageInfo, 0)

	if _, err := os.Stat(s.rawDir); os.IsNotExist(err) {
		return nil
	}

	err := filepath.WalkDir(s.rawDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && IsImageFile(path) {
			info, err := d.Info()
			if err != nil {
				log.Printf("Error getting file info for %s: %v", path, err)
			}

			relPath, err := filepath.Rel(s.rawDir, path)
			if err != nil {
				relPath = filepath.Base(path)
			}
			relPath = filepath.ToSlash(relPath)

			imgInfo := ExtractImageInfo(path, relPath, info)

			// Perform compression if compressed directory is configured
			if s.compressedDir != "" {
				destPath := filepath.Join(s.compressedDir, filepath.FromSlash(relPath))
				needsCompression := true

				if destInfo, err := os.Stat(destPath); err == nil && info != nil {
					// Compressed file exists; only re-compress if raw file is newer
					if !info.ModTime().After(destInfo.ModTime()) {
						needsCompression = false
					}
				}

				if needsCompression {
					if err := CompressImage(path, destPath, DefaultMaxImageDimension, DefaultJPEGQuality); err != nil {
						log.Printf("Warning: failed to compress %s to %s: %v", path, destPath, err)
					}
				}
			}

			newImages = append(newImages, imgInfo)
		}

		return nil
	})

	if err != nil {
		log.Printf("Error walking raw images directory: %v", err)
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
