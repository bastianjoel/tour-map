package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/rwcarlsen/goexif/exif"
)

const dataDir = "./data"
const imagesDir = "./images"
const trackingTokenFile = "./tracking_token.txt"

//go:embed index.html
var tmpl string

// GPS coordinates structure
type GPSCoords struct {
	Latitude  float64 `json:"lat"`
	Longitude float64 `json:"lng"`
}

// Karoo Live tracking entry
type Waypoint struct {
	Location  *GPSCoords `json:"location,omitempty"`
	Timestamp time.Time  `json:"updatedAt"`
}

// Application state
type App struct {
	latestWaypoint *time.Time
	waypoints      []Waypoint
	imageLocations map[string]GPSCoords
	wpMutex        sync.RWMutex
	imagesMutex    sync.RWMutex
}

func main() {
	app := &App{
		waypoints:      make([]Waypoint, 0),
		imageLocations: make(map[string]GPSCoords),
	}

	// Create data dir if not exists
	os.MkdirAll(dataDir, 0755)

	// Initial data load
	app.loadWaypoints()
	app.scanImages()

	// Start periodic updates
	go app.periodicImageScan()
	go app.periodicWaypointScan()

	// Setup HTTP server
	app.setupHTTPServer()

	// Start server
	fmt.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// Load all JSON files from /data directory
func (app *App) loadWaypoints() {
	nextPathData := make([]Waypoint, 0)

	err := filepath.WalkDir(dataDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && strings.HasSuffix(strings.ToLower(path), ".json") {
			data, err := os.ReadFile(path)
			if err != nil {
				log.Printf("Error reading JSON file %s: %v", path, err)
				return nil
			}

			var wp Waypoint
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
	}

	slices.SortFunc(nextPathData, func(a, b Waypoint) int {
		return a.Timestamp.Compare(b.Timestamp)
	})

	if (len(nextPathData) > 0) {
		latest := nextPathData[len(nextPathData)-1].Timestamp
		app.latestWaypoint = &latest
	}

	log.Printf("Loaded %d JSON files", len(nextPathData))

	app.wpMutex.Lock()
	defer app.wpMutex.Unlock()

	app.waypoints = nextPathData
}

// Scan images directory for GPS coordinates
func (app *App) scanImages() {
	newGPSData := make(map[string]GPSCoords)

	err := filepath.WalkDir(imagesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && app.isImageFile(path) {
			coords, err := app.extractGPSCoords(path)
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
		return
	}

	app.imagesMutex.Lock()
	defer app.imagesMutex.Unlock()

	app.imageLocations = newGPSData
	log.Printf("Updated GPS data for %d images", len(app.imageLocations))
}

// Check if file is an image
func (app *App) isImageFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return ext == ".jpg" || ext == ".jpeg" || ext == ".tiff" || ext == ".tif"
}

// Extract GPS coordinates from image EXIF data
func (app *App) extractGPSCoords(imagePath string) (*GPSCoords, error) {
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

	return &GPSCoords{
		Latitude:  lat,
		Longitude: lon,
	}, nil
}

// Periodic image scanning
func (app *App) periodicImageScan() {
	ticker := time.NewTicker(300 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		log.Println("Performing periodic image scan...")
		app.scanImages()
	}
}

// Periodic image scanning
func (app *App) periodicWaypointScan() {
	tokenDeleted := false
	lastToken := ""
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// Call http endpoint defined in tracking_token.txt
		data, err := os.ReadFile(trackingTokenFile)
		if err != nil {
			log.Printf("Error reading tracking token file %s: %v", trackingTokenFile, err)
			continue
		}

		token := strings.TrimSpace(string(data))
		if token == "" {
			log.Printf("Tracking token file %s is empty", trackingTokenFile)
			continue
		}

		if token != lastToken {
			log.Printf("Using new tracking token: %s", token)
			lastToken = token
		} else if tokenDeleted {
			continue
		}

		processedURL := fmt.Sprintf("https://dashboard.hammerhead.io/v1/shares/tracking/%s", token)
		resp, err := http.Get(processedURL)
		if err != nil {
			log.Printf("Error fetching tracking data: %v", err)
			continue
		}

		if resp.StatusCode == http.StatusNotFound {
			log.Printf("Tracking token %s not found, stopping further requests", token)
			tokenDeleted = true
			resp.Body.Close()
			continue
		} else if resp.StatusCode != http.StatusOK {
			log.Printf("Non-OK HTTP status: %s", resp.Status)
			resp.Body.Close()
			continue
		}

		// Read as string
		dataRaw, err := io.ReadAll(resp.Body); 
		if err != nil {
			log.Printf("Error reading tracking response body: %v", err)
			resp.Body.Close()
			continue
		}

		var fetchedWaypoints Waypoint
		if err := json.Unmarshal(dataRaw, &fetchedWaypoints); err != nil {
			log.Printf("Error decoding tracking JSON: %v", err)
			resp.Body.Close()
			continue
		}
		resp.Body.Close()

		if fetchedWaypoints.Location != nil {
			app.wpMutex.Lock()
			if app.latestWaypoint == nil || fetchedWaypoints.Timestamp.After(*app.latestWaypoint) {
				app.waypoints = append(app.waypoints, fetchedWaypoints)
				app.latestWaypoint = &fetchedWaypoints.Timestamp
				log.Printf("Added new waypoint at %s", fetchedWaypoints.Timestamp)
			}
			app.wpMutex.Unlock()

			filename := fmt.Sprintf("%s/tracking_%s.json", dataDir, fetchedWaypoints.Timestamp.Format("20060102_150405"))
			os.WriteFile(filename, dataRaw, 0644)
		}
	}
}

// Setup HTTP server routes
func (app *App) setupHTTPServer() {
	// Serve static files from /images
	http.Handle("/images/", http.StripPrefix("/images/", http.FileServer(http.Dir(imagesDir))))

	// Main index page
	http.HandleFunc("/", app.handleIndex)
}

// Handle main index page
func (app *App) handleIndex(w http.ResponseWriter, r *http.Request) {
	app.wpMutex.RLock()
	app.imagesMutex.RLock()
	images := make(map[string]GPSCoords)
	maps.Copy(images, app.imageLocations)
	waypoints := make([][]float64, 0, len(app.waypoints))
	for _, wp := range app.waypoints {
		waypoints = append(waypoints, []float64{wp.Location.Latitude, wp.Location.Longitude})
	}

	app.imagesMutex.RUnlock()
	app.wpMutex.RUnlock()

	t, err := template.New("index").Parse(tmpl)
	if err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}

	data := struct {
		Images    map[string]GPSCoords
		Waypoints [][]float64
	}{
		Images:    images,
		Waypoints: waypoints,
	}

	w.Header().Set("Content-Type", "text/html")
	t.Execute(w, data)
}
