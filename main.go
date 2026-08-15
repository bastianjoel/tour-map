package main

import (
	_ "embed"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"tour-map/pkg/images"
	"tour-map/pkg/server"
	"tour-map/pkg/tracker"
)

const (
	dataDir           = "./data"
	imagesDir         = "./images"
	trackingTokenFile = "./tracking_token.txt"
	codesFile         = "./codes.txt"
	serverPort        = ":8080"
)

//go:embed index.html
var tmpl string

func main() {
	// Ensure data directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("Failed to create data directory %s: %v", dataDir, err)
	}

	// Initialize waypoint store and image scanner
	store := tracker.NewStore(dataDir, codesFile)
	if err := store.LoadWaypoints(); err != nil {
		log.Printf("Warning: initial load of waypoints failed: %v", err)
	}
	if err := store.LoadCodes(); err != nil {
		log.Printf("Warning: initial load of codes failed: %v", err)
	}

	imageScanner := images.NewScanner(imagesDir)
	if err := imageScanner.Scan(); err != nil {
		log.Printf("Warning: initial image scan failed: %v", err)
	}

	// Initialize live tracker poller
	poller := tracker.NewPoller(store, trackingTokenFile, "", nil)

	// Start background workers
	go imageScanner.StartPeriodicScan(300*time.Second, nil)
	go poller.StartPeriodic(15*time.Second, nil)

	// Initialize HTTP server
	srv, err := server.NewServer(store, imageScanner, imagesDir, tmpl)
	if err != nil {
		log.Fatalf("Failed to initialize server: %v", err)
	}

	fmt.Printf("Server starting on %s\n", serverPort)
	log.Fatal(http.ListenAndServe(serverPort, srv.Handler()))
}
