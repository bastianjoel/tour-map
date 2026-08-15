package tracker

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"tour-map/pkg/geo"
)

const defaultTrackingBaseURL = "https://dashboard.hammerhead.io/v1/shares/tracking"

// Poller periodically fetches live tracking data from Hammerhead.
type Poller struct {
	store             *Store
	trackingTokenFile string
	baseURL           string
	client            *http.Client
	lastToken         string
	tokenDeleted      bool
}

// NewPoller creates a new Poller instance.
func NewPoller(store *Store, trackingTokenFile string, baseURL string, client *http.Client) *Poller {
	if baseURL == "" {
		baseURL = defaultTrackingBaseURL
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &Poller{
		store:             store,
		trackingTokenFile: trackingTokenFile,
		baseURL:           baseURL,
		client:            client,
	}
}

// PollOnce executes a single poll iteration: reloading codes, checking token, and querying live tracking.
func (p *Poller) PollOnce() error {
	// Reload codes
	p.store.LoadCodes()

	// Read tracking token
	data, err := os.ReadFile(p.trackingTokenFile)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("Error reading tracking token file %s: %v", p.trackingTokenFile, err)
		}
		return err
	}

	token := strings.TrimSpace(string(data))
	if token != p.lastToken {
		log.Printf("Using new tracking token: %s", token)
		p.lastToken = token
		p.tokenDeleted = false
	} else if p.tokenDeleted {
		return nil
	} else if token == "" {
		log.Printf("Tracking token file %s is empty", p.trackingTokenFile)
		p.lastToken = token
		p.tokenDeleted = true
		return nil
	}

	processedURL := fmt.Sprintf("%s/%s", strings.TrimRight(p.baseURL, "/"), token)
	resp, err := p.client.Get(processedURL)
	if err != nil {
		log.Printf("Error fetching tracking data: %v", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		log.Printf("Tracking token %s not found, stopping further requests", token)
		p.tokenDeleted = true
		return nil
	} else if resp.StatusCode != http.StatusOK {
		log.Printf("Non-OK HTTP status from tracking API: %s", resp.Status)
		return fmt.Errorf("unexpected status: %s", resp.Status)
	}

	dataRaw, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading tracking response body: %v", err)
		return err
	}

	var fetchedWaypoint geo.Waypoint
	if err := json.Unmarshal(dataRaw, &fetchedWaypoint); err != nil {
		log.Printf("Error decoding tracking JSON: %v", err)
		return err
	}

	if fetchedWaypoint.Location != nil {
		p.store.AddWaypoint(fetchedWaypoint, dataRaw)
	}

	return nil
}

// StartPeriodic runs PollOnce periodically until the stop channel receives a signal.
func (p *Poller) StartPeriodic(interval time.Duration, stopCh <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.PollOnce()
		case <-stopCh:
			return
		}
	}
}
