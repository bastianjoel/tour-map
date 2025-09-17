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
	"math"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/rwcarlsen/goexif/exif"
	"github.com/tormoder/fit"
)

const dataDir = "./data"
const imagesDir = "./images"
const fitDir = "./fit"
const trackingTokenFile = "./tracking_token.txt"
const codesFile = "./codes.txt"

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
	codesMutex     sync.RWMutex
	codes          map[string]struct{}
}

func main() {
	app := &App{
		waypoints:      make([]Waypoint, 0),
		imageLocations: make(map[string]GPSCoords),
		codes:          make(map[string]struct{}),
	}

	// Create directories if they don't exist
	os.MkdirAll(dataDir, 0755)
	os.MkdirAll(fitDir, 0755)

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

// Load all JSON files from /data directory and FIT files from /fit directory
func (app *App) loadWaypoints() {
	// Load JSON waypoints from data directory
	jsonWaypoints := make([]Waypoint, 0)

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
				jsonWaypoints = append(jsonWaypoints, wp)
			}
		}

		return nil
	})

	if err != nil {
		log.Printf("Error walking data directory: %v", err)
	}

	// Load FIT waypoints from fit directory
	fitWaypoints := make([]Waypoint, 0)

	// Check if fit directory exists first
	if _, err := os.Stat(fitDir); !os.IsNotExist(err) {
		err := filepath.WalkDir(fitDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if !d.IsDir() && strings.HasSuffix(strings.ToLower(path), ".fit") {
				waypoints, err := app.parseFitFile(path)
				if err != nil {
					log.Printf("Error parsing FIT file %s: %v", path, err)
					return nil
				}

				fitWaypoints = append(fitWaypoints, waypoints...)
			}

			return nil
		})

		if err != nil {
			log.Printf("Error walking fit directory: %v", err)
		}
	}

	// Combine and filter waypoints
	allWaypoints := make([]Waypoint, 0, len(jsonWaypoints)+len(fitWaypoints))
	
	// If FIT waypoints exist, filter out JSON waypoints that are before or equal to the latest FIT waypoint
	if len(fitWaypoints) > 0 {
		// Sort FIT waypoints first to find latest
		slices.SortFunc(fitWaypoints, func(a, b Waypoint) int {
			return a.Timestamp.Compare(b.Timestamp)
		})
		
		latestFitTime := fitWaypoints[len(fitWaypoints)-1].Timestamp
		
		// Only include JSON waypoints that are after the latest FIT waypoint
		for _, wp := range jsonWaypoints {
			if wp.Timestamp.After(latestFitTime) {
				allWaypoints = append(allWaypoints, wp)
			}
		}
		allWaypoints = append(allWaypoints, fitWaypoints...)
	} else {
		allWaypoints = append(allWaypoints, jsonWaypoints...)
	}

	// Sort all waypoints by timestamp
	slices.SortFunc(allWaypoints, func(a, b Waypoint) int {
		return a.Timestamp.Compare(b.Timestamp)
	})

	// Prune waypoints to remove consecutive waypoints less than 5 meters apart
	prunedWaypoints := pruneWaypoints(allWaypoints)

	if len(prunedWaypoints) > 0 {
		latest := prunedWaypoints[len(prunedWaypoints)-1].Timestamp
		app.latestWaypoint = &latest
	}

	log.Printf("Loaded %d JSON files, %d FIT waypoints, %d total waypoints, %d after pruning", len(jsonWaypoints), len(fitWaypoints), len(allWaypoints), len(prunedWaypoints))

	app.wpMutex.Lock()
	defer app.wpMutex.Unlock()

	app.waypoints = prunedWaypoints
}

// Parse a single FIT file and extract waypoints
func (app *App) parseFitFile(path string) ([]Waypoint, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	fitFile, err := fit.Decode(file)
	if err != nil {
		return nil, err
	}

	waypoints := make([]Waypoint, 0)

	// Extract track points from the FIT file
	activity, err := fitFile.Activity()
	if err != nil {
		return nil, err
	}

	// Process all record messages directly from the activity file
	for _, record := range activity.Records {
		if !record.PositionLat.Invalid() && !record.PositionLong.Invalid() {
			lat := record.PositionLat.Degrees()
			lng := record.PositionLong.Degrees()

			waypoint := Waypoint{
				Location: &GPSCoords{
					Latitude:  lat,
					Longitude: lng,
				},
				Timestamp: record.Timestamp,
			}
			waypoints = append(waypoints, waypoint)
		}
	}

	return waypoints, nil
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
		{
			data, err := os.ReadFile(codesFile)
			if err != nil {
				log.Printf("Error reading codes file %s: %v", codesFile, err)
			} else {
				codes := strings.TrimSpace(string(data))
				if codes != "" {
					newCodes := strings.Split(codes, "\n")
					app.codesMutex.Lock()
					if app.codes == nil {
						app.codes = make(map[string]struct{})
					}
					for _, code := range newCodes {
						code = strings.TrimSpace(code)
						if code != "" {
							app.codes[code] = struct{}{}
						}
					}
					app.codesMutex.Unlock()
				}
			}
		}

		// Call http endpoint defined in tracking_token.txt
		data, err := os.ReadFile(trackingTokenFile)
		if err != nil {
			log.Printf("Error reading tracking token file %s: %v", trackingTokenFile, err)
			continue
		}

		token := strings.TrimSpace(string(data))
		if token != lastToken {
			log.Printf("Using new tracking token: %s", token)
			lastToken = token
			tokenDeleted = false
		} else if tokenDeleted {
			continue
		} else if token == "" {
			log.Printf("Tracking token file %s is empty", trackingTokenFile)
			lastToken = token
			tokenDeleted = true
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
		dataRaw, err := io.ReadAll(resp.Body)
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
			}
			app.wpMutex.Unlock()

			filename := fmt.Sprintf("%s/tracking_%s.json", dataDir, fetchedWaypoints.Timestamp.Format("20060102_150405"))
			os.WriteFile(filename, dataRaw, 0644)
		}
	}
}

// UpdateResponse represents the API response for incremental updates
type UpdateResponse struct {
	Waypoints    [][]float64            `json:"waypoints"`
	Images       map[string][]float64   `json:"images"`
	LastModified time.Time              `json:"lastModified"`
}

// Setup HTTP server routes
func (app *App) setupHTTPServer() {
	// Serve static files from /images with cache control headers
	imageHandler := http.StripPrefix("/images/", http.FileServer(http.Dir(imagesDir)))
	http.Handle("/images/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=259200")
		imageHandler.ServeHTTP(w, r)
	}))

	// API endpoint for incremental updates
	http.HandleFunc("/api/updates", app.handleUpdates)

	// Main index page
	http.HandleFunc("/", app.handleIndex)
}

func distanceKm(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371.0

	lat1Rad := lat1 * math.Pi / 180
	lon1Rad := lon1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	lon2Rad := lon2 * math.Pi / 180

	dLat := lat2Rad - lat1Rad
	dLon := lon2Rad - lon1Rad

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*
			math.Sin(dLon/2)*math.Sin(dLon/2)

	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return R * c
}

// pruneWaypoints removes consecutive waypoints that are less than 5 meters apart
// while ensuring at least one waypoint is retained in each closely clustered group
func pruneWaypoints(waypoints []Waypoint) []Waypoint {
	if len(waypoints) <= 1 {
		return waypoints
	}

	const minDistanceKm = 0.02 // 20 meters in kilometers

	prunedWaypoints := make([]Waypoint, 0, len(waypoints))
	prunedWaypoints = append(prunedWaypoints, waypoints[0]) // Always keep the first waypoint

	for i := 1; i < len(waypoints); i++ {
		currentWaypoint := waypoints[i]
		lastKeptWaypoint := prunedWaypoints[len(prunedWaypoints)-1]

		// Calculate distance between current waypoint and the last kept waypoint
		distance := distanceKm(
			lastKeptWaypoint.Location.Latitude,
			lastKeptWaypoint.Location.Longitude,
			currentWaypoint.Location.Latitude,
			currentWaypoint.Location.Longitude,
		)

		// Keep waypoint if distance is 5 meters or more
		if distance >= minDistanceKm {
			prunedWaypoints = append(prunedWaypoints, currentWaypoint)
		}
	}

	return prunedWaypoints
}

// Handle API endpoint for incremental updates
func (app *App) handleUpdates(w http.ResponseWriter, r *http.Request) {
	// Parse 'since' timestamp parameter
	sinceParam := r.URL.Query().Get("since")
	var since time.Time
	var err error
	
	if sinceParam != "" {
		since, err = time.Parse(time.RFC3339, sinceParam)
		if err != nil {
			http.Error(w, "Invalid 'since' timestamp format, use RFC3339", http.StatusBadRequest)
			return
		}
	}

	// Get code for access control (same logic as main page)
	code := r.URL.Query().Get("code")
	app.codesMutex.RLock()
	_, hasValidCode := app.codes[code]
	app.codesMutex.RUnlock()

	app.wpMutex.RLock()
	var waypoints [][]float64
	var lastModified time.Time
	
	// Apply 10km restriction if user doesn't have a valid code (same logic as main page)
	if hasValidCode {
		// Return all waypoints if user has valid access code
		waypoints = make([][]float64, 0, len(app.waypoints))
		for _, wp := range app.waypoints {
			if sinceParam == "" || wp.Timestamp.After(since) {
				waypoints = append(waypoints, []float64{wp.Location.Latitude, wp.Location.Longitude})
			}
		}
		if len(app.waypoints) > 0 {
			lastModified = app.waypoints[len(app.waypoints)-1].Timestamp
		}
	} else {
		// Apply 10km restriction for users without valid access code
		allWaypoints := make([][]float64, 0, len(app.waypoints))
		for _, wp := range app.waypoints {
			allWaypoints = append(allWaypoints, []float64{wp.Location.Latitude, wp.Location.Longitude})
		}
		
		if len(allWaypoints) > 0 {
			lastWaypoint := allWaypoints[len(allWaypoints)-1]
			i := len(allWaypoints) - 1
			for ; i >= 0; i-- {
				if distanceKm(lastWaypoint[0], lastWaypoint[1], allWaypoints[i][0], allWaypoints[i][1]) > 10.0 {
					break
				}
			}
			// i+1 is the count of waypoints to keep (all waypoints within 10km from the end)
			keepCount := i + 1
			if keepCount <= 0 {
				// All waypoints are within 10km, keep them all
				keepCount = len(allWaypoints)
			}
			
			restrictedWaypoints := allWaypoints[:keepCount]
			
			// Filter by 'since' timestamp
			for j, wp := range app.waypoints[:keepCount] {
				if sinceParam == "" || wp.Timestamp.After(since) {
					waypoints = append(waypoints, restrictedWaypoints[j])
				}
			}
			if len(app.waypoints) > 0 && keepCount > 0 {
				lastModified = app.waypoints[keepCount-1].Timestamp
			}
		}
	}
	app.wpMutex.RUnlock()

	// Get images (images don't have timestamps, so return all new ones based on comparison)
	app.imagesMutex.RLock()
	imageData := make(map[string][]float64)
	for filename, coords := range app.imageLocations {
		imageData[filename] = []float64{coords.Latitude, coords.Longitude}
	}
	app.imagesMutex.RUnlock()

	response := UpdateResponse{
		Waypoints:    waypoints,
		Images:       imageData,
		LastModified: lastModified,
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding JSON response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// Handle main index page
func (app *App) handleIndex(w http.ResponseWriter, r *http.Request) {
	app.imagesMutex.RLock()
	images := make(map[string]GPSCoords)
	maps.Copy(images, app.imageLocations)
	app.imagesMutex.RUnlock()
	
	app.wpMutex.RLock()
	waypoints := make([][]float64, 0, len(app.waypoints))
	for _, wp := range app.waypoints {
		waypoints = append(waypoints, []float64{wp.Location.Latitude, wp.Location.Longitude})
	}
	app.wpMutex.RUnlock()

	code := r.URL.Query().Get("code")
	app.codesMutex.RLock()
	if _, exists := app.codes[code]; !exists && len(waypoints) > 0 {
		lastWaypoint := waypoints[len(waypoints)-1]
		i := len(waypoints) - 1
		for ; i >= 0; i-- {
			// if distance is more than 10km, break
			if distanceKm(lastWaypoint[0], lastWaypoint[1], waypoints[i][0], waypoints[i][1]) > 10.0 {
				break
			}
		}

		waypoints = waypoints[:i+1]
	}
	app.codesMutex.RUnlock()

	t, err := template.New("index").Parse(tmpl)
	if err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}

	imageData := make(map[string][]float64)
	for filename, coords := range images {
		imageData[filename] = []float64{coords.Latitude, coords.Longitude}
	}

	imageDataJson, err := json.Marshal(imageData)
	if err != nil {
		http.Error(w, "JSON encoding error", http.StatusInternalServerError)
		return
	}

	waypointsJson, err := json.Marshal(waypoints)
	if err != nil {
		http.Error(w, "JSON encoding error", http.StatusInternalServerError)
		return
	}

	data := struct {
		Images    template.JS
		Waypoints template.JS
	}{
		Images:    template.JS(string(imageDataJson)),
		Waypoints: template.JS(string(waypointsJson)),
	}

	w.Header().Set("Content-Type", "text/html")
	t.Execute(w, data)
}
