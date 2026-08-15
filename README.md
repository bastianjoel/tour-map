# Tour Map

A self-hosted, interactive GPS tour mapping application with live tracking, FIT file support, privacy controls, and a tour-specific photo gallery with automatic image compression.

---

## Features

- **Vector Maps with MapLibre GL JS**: OpenStreetMap raster tiles rendered smoothly with self-contained local assets (no external CDN requests).
- **Tour-Specific Photo Gallery**:
  - Automatically associates photos (both GPS-tagged and date-only) with tour dates.
  - Interactive "Focus on Map" button in the gallery header that centers the map on the photo's coordinate and closes the lightbox.
  - Standalone photos (taken outside tour timeframes) appear as individual pins on the map without being mixed into tour galleries.
- **Smart Image Compression**:
  - Automatically resizes and compresses raw photos from `./images` into `./data/images-compressed`.
  - Fixes EXIF orientation (e.g. mobile portrait photos) and uses bilinear resampling.
  - Serves compressed images with browser cache headers, with transparent fallback to raw originals.
- **Garmin FIT File Support**:
  - Automatically parses `.fit` activity files recursively from `./fit` using [`github.com/muktihari/fit`](https://github.com/muktihari/fit).
  - Preserves single activity continuity: pauses or gaps $>2\text{ km}$ are rendered as dotted lines, while normal riding is solid.
  - Different activities or live tracking trips $>10\text{ km}$ or $>7\text{ days}$ apart remain distinct, disconnected trips.
- **Live Tracking Poller**:
  - Continuously polls Hammerhead live tracking updates and persists waypoints to `./data/`.
  - Prunes waypoints closer than 5 meters to prevent dense point clutter.
- **Privacy Controls**:
  - Conceals waypoints within a 10 km radius of the latest position unless an authorized access code (`?code=<your-code>`) is provided.
- **Internationalization (i18n)**:
  - Client-side translations for English (`en`) and German (`de`) based on browser language (or `?lang=de` / `?lang=en`).
  - Localized date formatting matching the active language.

---

## Architecture & Package Structure

```
tour-map/
├── Dockerfile                  # Multi-stage container build (Node.js + Go + Alpine)
├── main.go                     # Application entrypoint & background worker initialization
├── pkg/
│   ├── geo/                    # GPS calculations, distance, pruning, privacy & trip segmentation
│   ├── images/                 # Recursive scanner, EXIF parser, orientation & JPEG compression
│   ├── server/                 # HTTP server, /api/updates endpoint & static image serving
│   └── tracker/                # Waypoint store, FIT parser, access codes & Hammerhead poller
└── web/                        # Frontend source files & singlefile bundler
    ├── index.html              # HTML template
    ├── package.json            # MapLibre GL JS & Vite dependencies
    ├── vite.config.js          # Vite singlefile bundler configuration
    └── src/
        ├── gallery.js          # Lightbox modal, navigation & map focus
        ├── i18n.js             # German / English translation dictionary & localized dates
        ├── main.js             # MapLibre map setup, track rendering & update polling
        └── style.css           # UI styles
```

---

## Directory Configuration

| Directory / File       | Description                                                                                    |
| :--------------------- | :--------------------------------------------------------------------------------------------- |
| `./data/`              | Stores incremental live tracking JSON files and `./data/images-compressed/`.                   |
| `./fit/`               | Folder containing Garmin/FIT activity files (`.fit`), scanned recursively.                     |
| `./images/`            | Folder containing original raw photos (`.jpg`, `.jpeg`, `.png`, `.tiff`), scanned recursively. |
| `./codes.txt`          | Text file containing authorized access codes (one per line).                                   |
| `./tracking_token.txt` | Text file containing the Hammerhead live tracking share token.                                 |

---

## Getting Started

### Prerequisites

- **Go**: 1.25 or later
- **Node.js**: 22 or later (and npm)

### 1. Build the Frontend

```bash
cd web
npm install
npm run build
cd ..
```

### 2. Run the Application

```bash
go run .
```

The server will start on port `8080` (accessible at `http://localhost:8080`).

---

## Docker Deployment

Build and run using the optimized multi-stage Dockerfile:

```bash
# Build container image
docker build -t tour-map .

# Run container with mounted data and image volumes
docker run -d \
  -p 8080:8080 \
  -v $(pwd)/data:/app/data \
  -v $(pwd)/fit:/app/fit \
  -v $(pwd)/images:/app/images \
  -v $(pwd)/codes.txt:/app/codes.txt:ro \
  -v $(pwd)/tracking_token.txt:/app/tracking_token.txt:ro \
  --name tour-map tour-map
```

---

## Testing & CI

Run the automated test suite with race detector:

```bash
go test -v -race ./...
```

The project includes a GitHub Actions CI workflow in `.github/workflows/ci.yml` that validates:

- Frontend bundling (`npm run build`)
- Go dependency verification (`go mod verify`, `go mod tidy`)
- Formatting (`gofmt`) and linting (`go vet`)
- Full unit test execution with race detector (`go test -v -race`)
- Binary compilation (`go build`)
