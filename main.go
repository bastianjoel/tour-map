package main

import (
	_ "embed"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"tour-map/pkg/images"
	"tour-map/pkg/server"
	"tour-map/pkg/tracker"
)

const (
	dataDir           = "./data"
	fitDir            = "./fit"
	imagesDir         = "./images"
	trackingTokenFile = "./tracking_token.txt"
	codesFile         = "./codes.txt"
	serverPort        = ":8080"
)

//go:embed web/dist/index.html
var tmpl string

func main() {
	compressedImagesDir := filepath.Join(dataDir, "images-compressed")

	// Ensure directories exist
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("Failed to create data directory %s: %v", dataDir, err)
	}
	if err := os.MkdirAll(compressedImagesDir, 0755); err != nil {
		log.Fatalf("Failed to create compressed images directory %s: %v", compressedImagesDir, err)
	}
	if err := os.MkdirAll(fitDir, 0755); err != nil {
		log.Fatalf("Failed to create fit directory %s: %v", fitDir, err)
	}
	if err := os.MkdirAll(imagesDir, 0755); err != nil {
		log.Fatalf("Failed to create raw images directory %s: %v", imagesDir, err)
	}

	// Initialize waypoint store and image scanner
	store := tracker.NewStore(dataDir, fitDir, codesFile)
	if err := store.LoadWaypoints(); err != nil {
		log.Printf("Warning: initial load of waypoints failed: %v", err)
	}
	if err := store.LoadCodes(); err != nil {
		log.Printf("Warning: initial load of codes failed: %v", err)
	}

	imageScanner := images.NewScanner(imagesDir, compressedImagesDir)
	if err := imageScanner.Scan(); err != nil {
		log.Printf("Warning: initial image scan failed: %v", err)
	}

	// Initialize live tracker poller
	poller := tracker.NewPoller(store, trackingTokenFile, "", nil)

	// Start background workers
	go imageScanner.StartPeriodicScan(300*time.Second, nil)
	go poller.StartPeriodic(15*time.Second, nil)

	// Initialize HTTP server
	srv, err := server.NewServer(store, imageScanner, compressedImagesDir, imagesDir, tmpl)
	if err != nil {
		log.Fatalf("Failed to initialize server: %v", err)
	}

	fmt.Printf("Server starting on %s\n", serverPort)
	log.Fatal(http.ListenAndServe(serverPort, srv.Handler()))
}
