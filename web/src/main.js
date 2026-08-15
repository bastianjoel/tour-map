import maplibregl from 'maplibre-gl';
import 'maplibre-gl/dist/maplibre-gl.css';
import './style.css';
import { openGallery, setGalleryMap, initGalleryEvents } from './gallery.js';
import { formatDate, applyStaticTranslations } from './i18n.js';

let segments = JSON.parse(document.getElementById('tour-data')?.textContent || '[]');
let allImages = JSON.parse(document.getElementById('image-data')?.textContent || '[]');
const imageMarkers = {};
let lastUpdateTime = new Date().toISOString();

if (!Array.isArray(allImages)) allImages = [];
if (!Array.isArray(segments)) segments = [];

applyStaticTranslations();

const map = new maplibregl.Map({
  container: 'map',
  style: 'https://tiles.openfreemap.org/styles/liberty',
  center: [13.405, 52.52],
  zoom: 10,
});

map.addControl(new maplibregl.NavigationControl(), 'top-right');
setGalleryMap(map);
initGalleryEvents();

// 1 hour buffer window around tour timeframe
const TIME_BUFFER_MS = 60 * 60 * 1000;

function isImageInSegment(img, seg) {
  if (!img.timestamp || !seg || !seg.startTime || !seg.endTime) return false;
  const t = new Date(img.timestamp).getTime();
  const start = new Date(seg.startTime).getTime() - TIME_BUFFER_MS;
  const end = new Date(seg.endTime).getTime() + TIME_BUFFER_MS;
  return t >= start && t <= end;
}

function getImagesForSegment(seg) {
  if (!seg) return [];
  return allImages.filter((img) => isImageInSegment(img, seg));
}

function findSegmentForImage(img) {
  return segments.find((seg) => isImageInSegment(img, seg));
}

function segmentsToGeoJSON(segs) {
  const features = [];
  if (Array.isArray(segs)) {
    for (const segment of segs) {
      const segId = segment.id !== undefined ? segment.id : 0;
      if (Array.isArray(segment.lines) && segment.lines.length > 0) {
        for (const line of segment.lines) {
          if (Array.isArray(line.coords) && line.coords.length >= 2) {
            const coordinates = line.coords.map((pt) => [pt[1], pt[0]]);
            features.push({
              type: 'Feature',
              properties: {
                segmentId: segId,
                lineType: line.type || 'solid',
              },
              geometry: {
                type: 'LineString',
                coordinates: coordinates,
              },
            });
          }
        }
      } else {
        const coords = segment.coords || segment;
        if (Array.isArray(coords) && coords.length >= 2) {
          const coordinates = coords.map((pt) => [pt[1], pt[0]]);
          features.push({
            type: 'Feature',
            properties: {
              segmentId: segId,
              lineType: 'solid',
            },
            geometry: {
              type: 'LineString',
              coordinates: coordinates,
            },
          });
        }
      }
    }
  }
  return {
    type: 'FeatureCollection',
    features: features,
  };
}

function fitMapBounds(segs) {
  const bounds = new maplibregl.LngLatBounds();
  let count = 0;

  if (Array.isArray(segs)) {
    for (const segment of segs) {
      const coords = segment.coords || segment;
      if (Array.isArray(coords)) {
        for (const pt of coords) {
          bounds.extend([pt[1], pt[0]]);
          count++;
        }
      }
    }
  }

  if (count > 0) {
    map.fitBounds(bounds, {
      padding: 40,
      maxZoom: 15,
      animate: false,
    });
  }
}

function drawImageMarkers(imgs) {
  if (!Array.isArray(imgs)) return;
  for (const img of imgs) {
    if (img.location && img.location.lat && img.location.lng) {
      if (!imageMarkers[img.filename]) {
        const marker = new maplibregl.Marker({ color: '#2AAD27' })
          .setLngLat([img.location.lng, img.location.lat])
          .addTo(map);

        marker.getElement().style.cursor = 'pointer';

        const seg = findSegmentForImage(img);
        if (seg) {
          marker.getElement().addEventListener('click', (e) => {
            e.stopPropagation();
            const tourImgs = getImagesForSegment(seg);
            const idx = tourImgs.findIndex((item) => item.filename === img.filename);
            openGallery(tourImgs, idx >= 0 ? idx : 0);
          });
        } else {
          const popup = new maplibregl.Popup({ offset: 25, maxWidth: 'none' }).setHTML(`
            <div style="text-align: center; color: #0f172a; padding: 4px;">
              <a href="/images/${encodeURI(img.filename)}" target="_blank">
                <img src="/images/${encodeURI(img.filename)}" style="max-width: 45vh; max-height: 45vw; display: block; border-radius: 4px; margin-bottom: 6px;" />
              </a>
              <div style="font-size: 12px; color: #64748b; font-weight: 500;">${formatDate(img.timestamp)}</div>
            </div>
          `);
          marker.setPopup(popup);
        }

        imageMarkers[img.filename] = marker;
      }
    }
  }
}

map.on('load', () => {
  // 1. Add Satellite Raster Source and Layer
  map.addSource('satellite-source', {
    type: 'raster',
    tiles: [
      'https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}',
    ],
    tileSize: 256,
    attribution: '&copy; <a href="https://www.esri.com/">Esri</a>, Earthstar Geographics',
  });

  map.addLayer({
    id: 'satellite-layer',
    type: 'raster',
    source: 'satellite-source',
    layout: {
      visibility: 'none',
    },
    paint: {
      'raster-opacity': 1,
    },
  });

  // 2. Add Tour Tracks Source
  map.addSource('tour-tracks', {
    type: 'geojson',
    data: segmentsToGeoJSON(segments),
  });

  // 3. Add Solid Track Lines
  map.addLayer({
    id: 'tour-tracks-solid',
    type: 'line',
    source: 'tour-tracks',
    filter: ['==', ['get', 'lineType'], 'solid'],
    layout: {
      'line-join': 'round',
      'line-cap': 'round',
    },
    paint: {
      'line-color': '#e53e3e',
      'line-width': 5,
      'line-opacity': 0.85,
    },
  });

  // 4. Add Dotted Track Lines for pauses >2km
  map.addLayer({
    id: 'tour-tracks-dotted',
    type: 'line',
    source: 'tour-tracks',
    filter: ['==', ['get', 'lineType'], 'dotted'],
    layout: {
      'line-join': 'round',
      'line-cap': 'round',
    },
    paint: {
      'line-color': '#e53e3e',
      'line-width': 4,
      'line-dasharray': [2, 2],
      'line-opacity': 0.75,
    },
  });

  // Interactive click and hover handlers on both solid and dotted track segments
  const trackLayers = ['tour-tracks-solid', 'tour-tracks-dotted'];
  trackLayers.forEach((layerId) => {
    map.on('click', layerId, (e) => {
      if (e.features && e.features.length > 0) {
        const segId = e.features[0].properties.segmentId;
        const seg = segments.find((s) => s.id === segId) || segments[0];
        const tourImgs = getImagesForSegment(seg);
        openGallery(tourImgs, 0);
      }
    });

    map.on('mouseenter', layerId, () => {
      map.getCanvas().style.cursor = 'pointer';
    });
    map.on('mouseleave', layerId, () => {
      map.getCanvas().style.cursor = '';
    });
  });

  fitMapBounds(segments);
  drawImageMarkers(allImages);
});

// Setup map style switcher buttons (Map vs Satellite)
function initStyleSwitcher() {
  const libertyBtn = document.getElementById('style-liberty-btn');
  const satelliteBtn = document.getElementById('style-satellite-btn');

  if (libertyBtn) {
    libertyBtn.addEventListener('click', () => {
      libertyBtn.classList.add('active');
      if (satelliteBtn) satelliteBtn.classList.remove('active');
      if (map.getLayer('satellite-layer')) {
        map.setLayoutProperty('satellite-layer', 'visibility', 'none');
      }
    });
  }

  if (satelliteBtn) {
    satelliteBtn.addEventListener('click', () => {
      satelliteBtn.classList.add('active');
      if (libertyBtn) libertyBtn.classList.remove('active');
      if (map.getLayer('satellite-layer')) {
        map.setLayoutProperty('satellite-layer', 'visibility', 'visible');
      }
    });
  }
}

initStyleSwitcher();

function updateMap() {
  const urlParams = new URLSearchParams(window.location.search);
  const code = urlParams.get('code') || '';
  const apiUrl = `/api/updates?since=${encodeURIComponent(lastUpdateTime)}&code=${encodeURIComponent(code)}`;

  fetch(apiUrl)
    .then((response) => {
      if (!response.ok) {
        throw new Error(`HTTP ${response.status}: ${response.statusText}`);
      }
      return response.json();
    })
    .then((data) => {
      if (data.waypoints && data.waypoints.length > 0) {
        fetch(`/?code=${encodeURIComponent(code)}`)
          .then((res) => res.text())
          .then((html) => {
            const parser = new DOMParser();
            const doc = parser.parseFromString(html, 'text/html');
            const tourEl = doc.getElementById('tour-data');
            if (tourEl) {
              segments = JSON.parse(tourEl.textContent || '[]');
              const source = map.getSource('tour-tracks');
              if (source) {
                source.setData(segmentsToGeoJSON(segments));
              }
            }
          })
          .catch((err) => console.error('Error refreshing path:', err));
      }

      if (data.images && Array.isArray(data.images)) {
        allImages = data.images;
        drawImageMarkers(allImages);
      }

      if (data.lastModified) {
        lastUpdateTime = data.lastModified;
      }
    })
    .catch((error) => {
      console.error('Error fetching updates:', error);
    });
}

setInterval(updateMap, 30000);
