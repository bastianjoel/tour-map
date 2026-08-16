package server

import (
	"encoding/json"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"tour-map/pkg/geo"
	"tour-map/pkg/images"
	"tour-map/pkg/tracker"
)

// UpdateResponse represents the JSON payload for incremental map updates.
type UpdateResponse struct {
	Waypoints    []geo.Segment      `json:"waypoints"`
	Images       []images.ImageInfo `json:"images"`
	LastModified time.Time          `json:"lastModified"`
}

// Server handles HTTP requests for the tour map.
type Server struct {
	store               *tracker.Store
	imageScanner        *images.Scanner
	compressedImagesDir string
	rawImagesDir        string
	tmpl                *template.Template
}

// NewServer creates a new HTTP Server instance.
func NewServer(store *tracker.Store, imageScanner *images.Scanner, compressedImagesDir, rawImagesDir string, tmplContent string) (*Server, error) {
	tmpl, err := template.New("index").Parse(tmplContent)
	if err != nil {
		return nil, err
	}

	return &Server{
		store:               store,
		imageScanner:        imageScanner,
		compressedImagesDir: compressedImagesDir,
		rawImagesDir:        rawImagesDir,
		tmpl:                tmpl,
	}, nil
}

// Handler returns an http.Handler configured with all routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Serve static image files with priority on compressed images and fallback to raw
	mux.HandleFunc("/images/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=259200")
		relPath := strings.TrimPrefix(r.URL.Path, "/images/")

		// 1. Try serving from compressed images directory
		if s.compressedImagesDir != "" {
			compFile := filepath.Join(s.compressedImagesDir, filepath.FromSlash(relPath))
			if info, err := os.Stat(compFile); err == nil && !info.IsDir() {
				http.ServeFile(w, r, compFile)
				return
			}
		}

		// 2. Fallback to raw images directory
		if s.rawImagesDir != "" {
			rawFile := filepath.Join(s.rawImagesDir, filepath.FromSlash(relPath))
			if info, err := os.Stat(rawFile); err == nil && !info.IsDir() {
				http.ServeFile(w, r, rawFile)
				return
			}
		}

		http.NotFound(w, r)
	})

	// API endpoint for incremental updates
	mux.HandleFunc("/api/updates", s.handleUpdates)

	// Main map page
	mux.HandleFunc("/", s.handleIndex)

	return mux
}

// handleUpdates returns JSON data for incremental updates since a given timestamp.
func (s *Server) handleUpdates(w http.ResponseWriter, r *http.Request) {
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

	code := r.URL.Query().Get("code")
	segments, lastModified := s.store.GetUpdates(since, code)
	allImages := s.store.InterpolateImageLocations(s.imageScanner.GetImages())

	response := UpdateResponse{
		Waypoints:    segments,
		Images:       allImages,
		LastModified: lastModified,
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// handleIndex processes requests for the main map view.
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	code := r.URL.Query().Get("code")
	segments := s.store.GetTripSegments(code)
	allImages := s.store.InterpolateImageLocations(s.imageScanner.GetImages())

	imagesJSON, err := json.Marshal(allImages)
	if err != nil {
		http.Error(w, "JSON encoding error", http.StatusInternalServerError)
		return
	}

	waypointsJSON, err := json.Marshal(segments)
	if err != nil {
		http.Error(w, "JSON encoding error", http.StatusInternalServerError)
		return
	}

	data := struct {
		Images    template.JS
		Waypoints template.JS
	}{
		Images:    template.JS(string(imagesJSON)),
		Waypoints: template.JS(string(waypointsJSON)),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.Execute(w, data); err != nil {
		http.Error(w, "Template execution error", http.StatusInternalServerError)
	}
}
