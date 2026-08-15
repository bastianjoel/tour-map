package images

import (
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strconv"
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

// parseGPSDateTime extracts UTC date and time from EXIF GPSDateStamp and GPSTimeStamp tags.
func parseGPSDateTime(x *exif.Exif) (time.Time, bool) {
	dateTag, err := x.Get(exif.GPSDateStamp)
	if err != nil {
		return time.Time{}, false
	}
	dateStr, err := dateTag.StringVal()
	if err != nil || dateStr == "" {
		return time.Time{}, false
	}

	dateStr = strings.Trim(dateStr, "\x00 \"'\n\r\t")
	dateStr = strings.ReplaceAll(dateStr, "-", ":")
	parts := strings.Split(dateStr, ":")
	if len(parts) != 3 {
		return time.Time{}, false
	}

	year, errY := strconv.Atoi(parts[0])
	month, errM := strconv.Atoi(parts[1])
	day, errD := strconv.Atoi(parts[2])
	if errY != nil || errM != nil || errD != nil || year == 0 {
		return time.Time{}, false
	}

	timeTag, err := x.Get(exif.GPSTimeStamp)
	if err != nil {
		return time.Time{}, false
	}

	var hour, min, sec, nsec int
	if timeTag.Count >= 3 {
		if hNum, hDen, err := timeTag.Rat2(0); err == nil && hDen > 0 {
			hour = int(hNum / hDen)
		}
		if mNum, mDen, err := timeTag.Rat2(1); err == nil && mDen > 0 {
			min = int(mNum / mDen)
		}
		if sNum, sDen, err := timeTag.Rat2(2); err == nil && sDen > 0 {
			secFloat := float64(sNum) / float64(sDen)
			sec = int(secFloat)
			nsec = int((secFloat - float64(sec)) * 1e9)
		}
	} else {
		return time.Time{}, false
	}

	// GPS timestamps are always recorded in UTC
	return time.Date(year, time.Month(month), day, hour, min, sec, nsec, time.UTC), true
}

// ExtractExifTimestamp attempts to extract the capture date from EXIF metadata.
// It prioritizes GPS Date/Time, followed by DateTimeOriginal, DateTimeDigitized, and DateTime.
func ExtractExifTimestamp(x *exif.Exif) (time.Time, bool) {
	if x == nil {
		return time.Time{}, false
	}

	// 1. Prefer GPS Date & Time (GPSDateStamp & GPSTimeStamp)
	if gpsTime, ok := parseGPSDateTime(x); ok {
		return gpsTime, true
	}

	dateFormats := []string{
		"2006:01:02 15:04:05",
		"2006-01-02 15:04:05",
		"2006:01:02T15:04:05",
		"2006-01-02T15:04:05",
		time.RFC3339,
	}

	// Helper to parse string values with multiple format attempts
	parseDateString := func(raw string) (time.Time, bool) {
		clean := strings.Trim(raw, "\x00 \"'\n\r\t")
		for _, layout := range dateFormats {
			if t, err := time.Parse(layout, clean); err == nil && !t.IsZero() {
				return t, true
			}
		}
		return time.Time{}, false
	}

	// 2. Try DateTimeOriginal (EXIF tag 0x9003) - Time when original photo was taken
	if tag, err := x.Get(exif.DateTimeOriginal); err == nil {
		if val, err := tag.StringVal(); err == nil {
			if t, ok := parseDateString(val); ok {
				return t, true
			}
		}
	}

	// 3. Try DateTimeDigitized (EXIF tag 0x9004) - Time when photo was stored/digitized
	if tag, err := x.Get(exif.DateTimeDigitized); err == nil {
		if val, err := tag.StringVal(); err == nil {
			if t, ok := parseDateString(val); ok {
				return t, true
			}
		}
	}

	// 4. Try standard DateTime (EXIF tag 0x0132) via goexif DateTime helper
	if exifTime, err := x.DateTime(); err == nil && !exifTime.IsZero() {
		return exifTime, true
	}

	// 5. Try DateTime (EXIF tag 0x0132) manually in case formatting differed
	if tag, err := x.Get(exif.DateTime); err == nil {
		if val, err := tag.StringVal(); err == nil {
			if t, ok := parseDateString(val); ok {
				return t, true
			}
		}
	}

	return time.Time{}, false
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

	// Extract timestamp from EXIF if available (preferring GPS Date/Time)
	if exifDate, ok := ExtractExifTimestamp(x); ok {
		timestamp = exifDate
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
