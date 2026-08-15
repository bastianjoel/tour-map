package server

import (
	"encoding/json"
	"html/template"
	"net/http"

	"tour-map/pkg/images"
	"tour-map/pkg/tracker"
)

// Server handles HTTP requests for the tour map.
type Server struct {
	store        *tracker.Store
	imageScanner *images.Scanner
	imagesDir    string
	tmpl         *template.Template
}

// NewServer creates a new HTTP Server instance.
func NewServer(store *tracker.Store, imageScanner *images.Scanner, imagesDir string, tmplContent string) (*Server, error) {
	tmpl, err := template.New("index").Parse(tmplContent)
	if err != nil {
		return nil, err
	}

	return &Server{
		store:        store,
		imageScanner: imageScanner,
		imagesDir:    imagesDir,
		tmpl:         tmpl,
	}, nil
}

// Handler returns an http.Handler configured with all routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Serve static files from imagesDir with cache headers
	imageHandler := http.StripPrefix("/images/", http.FileServer(http.Dir(s.imagesDir)))
	mux.HandleFunc("/images/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=259200")
		imageHandler.ServeHTTP(w, r)
	})

	// Main map page
	mux.HandleFunc("/", s.handleIndex)

	return mux
}

// handleIndex processes requests for the main map view.
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	code := r.URL.Query().Get("code")
	segments := s.store.GetTripSegments(code)

	imagesMap := s.imageScanner.GetLocations()
	imageData := make(map[string][]float64, len(imagesMap))
	for filename, coords := range imagesMap {
		imageData[filename] = []float64{coords.Latitude, coords.Longitude}
	}

	imageDataJSON, err := json.Marshal(imageData)
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
		Images:    template.JS(string(imageDataJSON)),
		Waypoints: template.JS(string(waypointsJSON)),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.Execute(w, data); err != nil {
		http.Error(w, "Template execution error", http.StatusInternalServerError)
	}
}
