import maplibregl from 'maplibre-gl';
import 'maplibre-gl/dist/maplibre-gl.css';
import './style.css';
import { openGallery, setGalleryMap, initGalleryEvents } from './gallery.js';
import {
  formatDate,
  formatShortDate,
  formatTime,
  formatDuration,
  t,
  applyStaticTranslations,
} from './i18n.js';

let segments = JSON.parse(document.getElementById('tour-data')?.textContent || '[]');
let allImages = JSON.parse(document.getElementById('image-data')?.textContent || '[]');
const imageMarkers = {};
let stageMarkers = [];
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
  const tTime = new Date(img.timestamp).getTime();
  const start = new Date(seg.startTime).getTime() - TIME_BUFFER_MS;
  const end = new Date(seg.endTime).getTime() + TIME_BUFFER_MS;
  return tTime >= start && tTime <= end;
}

function getImagesForSegment(seg) {
  if (!seg) return [];
  return allImages.filter((img) => isImageInSegment(img, seg));
}

function getImagesForStage(stage) {
  if (!stage || !stage.startTime || !stage.endTime) return [];
  const s = new Date(stage.startTime).getTime() - TIME_BUFFER_MS;
  const e = new Date(stage.endTime).getTime() + TIME_BUFFER_MS;
  return allImages.filter((img) => {
    if (!img.timestamp) return false;
    const time = new Date(img.timestamp).getTime();
    return time >= s && time <= e;
  });
}

function findSegmentForImage(img) {
  return segments.find((seg) => isImageInSegment(img, seg));
}

function isNear(c1, c2, thresholdDeg = 0.001) {
  if (!c1 || !c2) return false;
  return (
    Math.abs(c1[0] - c2[0]) < thresholdDeg &&
    Math.abs(c1[1] - c2[1]) < thresholdDeg
  );
}

const PIN_POPUP_OFFSET = {
  top: [0, 8],
  'top-left': [0, 8],
  'top-right': [0, 8],
  bottom: [0, -38],
  'bottom-left': [0, -38],
  'bottom-right': [0, -38],
  left: [16, -18],
  right: [-16, -18],
};

function createPopupElement(contentHtml, buttonId, onClick) {
  const popup = new maplibregl.Popup({
    offset: PIN_POPUP_OFFSET,
    maxWidth: '280px',
    closeButton: true,
    closeOnClick: true,
  }).setHTML(contentHtml);

  popup.on('open', () => {
    const popupEl = popup.getElement();
    if (popupEl) {
      ['click', 'mousedown', 'pointerdown', 'touchstart'].forEach((evtName) => {
        popupEl.addEventListener(evtName, (e) => {
          e.stopPropagation();
        });
      });
    }

    if (buttonId && onClick) {
      const btn = document.getElementById(buttonId);
      if (btn) {
        btn.onclick = (e) => {
          e.stopPropagation();
          onClick();
        };
      }
    }
  });

  return popup;
}

function clearStageMarkers() {
  for (const m of stageMarkers) {
    m.remove();
  }
  stageMarkers = [];
}

function createPinSvg(type, content = '') {
  let pinColor = '#3b82f6';
  let inner = '';

  if (type === 'start') {
    pinColor = '#10b981';
    inner = `<polygon points="13,10.5 21,15 13,19.5" fill="#059669"/>`;
  } else if (type === 'goal') {
    pinColor = '#ef4444';
    inner = `<text x="16" y="19" font-size="11" text-anchor="middle">🏁</text>`;
  } else if (type === 'startgoal') {
    pinColor = '#0d9488';
    inner = `<text x="16" y="19" font-size="11" text-anchor="middle">🏁</text>`;
  } else if (type === 'stage') {
    pinColor = '#3b82f6';
    inner = `<text x="16" y="19" font-size="11" font-weight="800" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif" text-anchor="middle" fill="#1d4ed8">${content}</text>`;
  }

  return `
    <svg viewBox="0 0 32 42" width="28" height="37" xmlns="http://www.w3.org/2000/svg" class="tour-pin-svg">
      <path d="M16 1 C7.7 1 1 7.7 1 16 C1 26.5 14.8 40.2 15.4 40.8 C15.7 41.1 16.3 41.1 16.6 40.8 C17.2 40.2 31 26.5 31 16 C31 7.7 24.3 1 16 1 Z"
            fill="${pinColor}" stroke="#ffffff" stroke-width="1.8" stroke-linejoin="round"/>
      <circle cx="16" cy="15" r="9" fill="#ffffff"/>
      ${inner}
    </svg>
  `;
}

function createPinElement(type, content = '', tooltip = '') {
  const el = document.createElement('div');
  el.className = `tour-pin tour-pin-${type}`;
  el.innerHTML = createPinSvg(type, content);
  if (tooltip) {
    el.title = tooltip;
  }
  return el;
}

function addPinMarker(el, coord, popup) {
  popup.setLngLat([coord[1], coord[0]]);
  const marker = new maplibregl.Marker({ element: el, anchor: 'bottom' })
    .setLngLat([coord[1], coord[0]])
    .setPopup(popup)
    .addTo(map);

  el.addEventListener('click', (e) => {
    e.stopPropagation();
    if (popup.isOpen()) {
      popup.remove();
    } else {
      popup.addTo(map);
    }
  });

  stageMarkers.push(marker);
  return marker;
}

function drawStageMarkers(segs) {
  clearStageMarkers();
  if (!Array.isArray(segs)) return;

  segs.forEach((segment, segIdx) => {
    const segId = segment.id !== undefined ? segment.id : segIdx;
    let stages = segment.stages;
    let startCoord = segment.startCoord;
    let endCoord = segment.endCoord;

    // Fallback if stages or coords are not provided
    if (!startCoord && segment.coords && segment.coords.length > 0) {
      startCoord = segment.coords[0];
    }
    if (!endCoord && segment.coords && segment.coords.length > 0) {
      endCoord = segment.coords[segment.coords.length - 1];
    }

    if (!stages || stages.length === 0) {
      if (startCoord && endCoord) {
        stages = [
          {
            stageNumber: 1,
            date: segment.startTime ? segment.startTime.split('T')[0] : '',
            startTime: segment.startTime,
            endTime: segment.endTime,
            startCoord: startCoord,
            endCoord: endCoord,
            distanceKm: segment.distanceKm || 0,
            totalDistanceKm: segment.distanceKm || 0,
          },
        ];
      } else {
        return;
      }
    }

    const numStages = stages.length;
    const tourImgs = getImagesForSegment(segment);

    // Case 1: Single stage / day
    if (numStages === 1) {
      const stg = stages[0];
      const isLoop = isNear(stg.startCoord, stg.endCoord);

      if (isLoop) {
        // Combined Start/Goal pin
        const tooltip = `${t('startGoal')}: ${formatShortDate(segment.startTime)}`;
        const el = createPinElement('startgoal', '', tooltip);

        const btnId = `btn-stage-photos-${segId}-startgoal`;
        const distText = stg.distanceKm > 0 ? `${stg.distanceKm.toFixed(1)} km` : '';
        const durationText = formatDuration(segment.startTime, segment.endTime);

        const html = `
          <div class="tour-popup-container">
            <div class="tour-popup-badge tour-popup-badge-start">🟢🏁 ${t('startGoal')}</div>
            <div class="tour-popup-title">${t('startGoal')}</div>
            <div class="tour-popup-date">📅 ${formatShortDate(segment.startTime)}</div>
            <div class="tour-popup-stats">
              <div class="tour-popup-stat-row">
                <span class="tour-popup-stat-label">${t('startTime')}:</span>
                <span class="tour-popup-stat-val">${formatTime(segment.startTime)}</span>
              </div>
              <div class="tour-popup-stat-row">
                <span class="tour-popup-stat-label">${t('endTime')}:</span>
                <span class="tour-popup-stat-val">${formatTime(segment.endTime)}</span>
              </div>
              ${
                durationText
                  ? `
              <div class="tour-popup-stat-row">
                <span class="tour-popup-stat-label">${t('duration')}:</span>
                <span class="tour-popup-stat-val">${durationText}</span>
              </div>`
                  : ''
              }
              ${
                distText
                  ? `
              <div class="tour-popup-stat-row">
                <span class="tour-popup-stat-label">${t('distance')}:</span>
                <span class="tour-popup-stat-val">${distText}</span>
              </div>`
                  : ''
              }
            </div>
            ${
              tourImgs.length > 0
                ? `
              <button class="tour-popup-btn" id="${btnId}">${t('viewPhotos')} (${tourImgs.length})</button>
            `
                : ''
            }
          </div>
        `;

        const popup = createPopupElement(html, btnId, () =>
          openGallery(tourImgs, 0)
        );
        addPinMarker(el, stg.startCoord, popup);
      } else {
        // Separate Start and Goal pins
        // 1. Start pin
        const startTooltip = `${t('start')}: ${formatShortDate(segment.startTime)}`;
        const startEl = createPinElement('start', '', startTooltip);

        const startBtnId = `btn-stage-photos-${segId}-start`;
        const startHtml = `
          <div class="tour-popup-container">
            <div class="tour-popup-badge tour-popup-badge-start">🟢 ${t('start')}</div>
            <div class="tour-popup-title">${t('tourStart')}</div>
            <div class="tour-popup-date">📅 ${formatShortDate(segment.startTime)}</div>
            <div class="tour-popup-stats">
              <div class="tour-popup-stat-row">
                <span class="tour-popup-stat-label">${t('startTime')}:</span>
                <span class="tour-popup-stat-val">${formatTime(segment.startTime)}</span>
              </div>
              ${
                stg.distanceKm > 0
                  ? `
              <div class="tour-popup-stat-row">
                <span class="tour-popup-stat-label">${t('distance')}:</span>
                <span class="tour-popup-stat-val">${stg.distanceKm.toFixed(1)} km</span>
              </div>`
                  : ''
              }
            </div>
            ${
              tourImgs.length > 0
                ? `
              <button class="tour-popup-btn" id="${startBtnId}">${t('viewPhotos')} (${tourImgs.length})</button>
            `
                : ''
            }
          </div>
        `;
        const startPopup = createPopupElement(startHtml, startBtnId, () =>
          openGallery(tourImgs, 0)
        );
        addPinMarker(startEl, stg.startCoord, startPopup);

        // 2. Goal pin
        const goalTooltip = `${t('goal')}: ${formatShortDate(segment.endTime)}`;
        const goalEl = createPinElement('goal', '', goalTooltip);

        const goalBtnId = `btn-stage-photos-${segId}-goal`;
        const durationText = formatDuration(segment.startTime, segment.endTime);
        const goalHtml = `
          <div class="tour-popup-container">
            <div class="tour-popup-badge tour-popup-badge-goal">🏁 ${t('goal')}</div>
            <div class="tour-popup-title">${t('tourGoal')}</div>
            <div class="tour-popup-date">📅 ${formatShortDate(segment.endTime)}</div>
            <div class="tour-popup-stats">
              <div class="tour-popup-stat-row">
                <span class="tour-popup-stat-label">${t('endTime')}:</span>
                <span class="tour-popup-stat-val">${formatTime(segment.endTime)}</span>
              </div>
              ${
                durationText
                  ? `
              <div class="tour-popup-stat-row">
                <span class="tour-popup-stat-label">${t('duration')}:</span>
                <span class="tour-popup-stat-val">${durationText}</span>
              </div>`
                  : ''
              }
              ${
                stg.distanceKm > 0
                  ? `
              <div class="tour-popup-stat-row">
                <span class="tour-popup-stat-label">${t('distance')}:</span>
                <span class="tour-popup-stat-val">${stg.distanceKm.toFixed(1)} km</span>
              </div>`
                  : ''
              }
            </div>
            ${
              tourImgs.length > 0
                ? `
              <button class="tour-popup-btn" id="${goalBtnId}">${t('viewPhotos')} (${tourImgs.length})</button>
            `
                : ''
            }
          </div>
        `;
        const goalPopup = createPopupElement(goalHtml, goalBtnId, () =>
          openGallery(tourImgs, tourImgs.length - 1)
        );
        addPinMarker(goalEl, stg.endCoord, goalPopup);
      }
    } else {
      // Case 2: Multi-day tour (numStages > 1)
      stages.forEach((stg, stgIdx) => {
        const stageImgs = getImagesForStage(stg);
        const stageDuration = formatDuration(stg.startTime, stg.endTime);

        if (stgIdx === 0) {
          // Tour Start Pin (Stage 1)
          const tooltip = `${t('tourStart')} • ${t('stageBadge', 1)}: ${formatShortDate(stg.startTime)}`;
          const startEl = createPinElement('start', '', tooltip);

          const btnId = `btn-stage-photos-${segId}-stage-1`;
          const html = `
            <div class="tour-popup-container">
              <div class="tour-popup-badge tour-popup-badge-start">🟢 ${t('tourStart')} • ${t('stageBadge', 1)}</div>
              <div class="tour-popup-title">${t('dayStage', 1, numStages)}</div>
              <div class="tour-popup-date">📅 ${formatShortDate(stg.startTime)}</div>
              <div class="tour-popup-stats">
                <div class="tour-popup-stat-row">
                  <span class="tour-popup-stat-label">${t('startTime')}:</span>
                  <span class="tour-popup-stat-val">${formatTime(stg.startTime)}</span>
                </div>
                ${
                  stageDuration
                    ? `
                <div class="tour-popup-stat-row">
                  <span class="tour-popup-stat-label">${t('duration')}:</span>
                  <span class="tour-popup-stat-val">${stageDuration}</span>
                </div>`
                    : ''
                }
                <div class="tour-popup-stat-row">
                  <span class="tour-popup-stat-label">${t('stageDistance')}:</span>
                  <span class="tour-popup-stat-val">${stg.distanceKm.toFixed(1)} km</span>
                </div>
                <div class="tour-popup-stat-row">
                  <span class="tour-popup-stat-label">${t('totalDistance')}:</span>
                  <span class="tour-popup-stat-val">${(segment.distanceKm || stg.totalDistanceKm).toFixed(1)} km</span>
                </div>
              </div>
            </div>
            ${
              stageImgs.length > 0
                ? `
              <button class="tour-popup-btn" id="${btnId}">${t('viewPhotos')} (${stageImgs.length})</button>
            `
                : tourImgs.length > 0
                  ? `
              <button class="tour-popup-btn" id="${btnId}">${t('viewPhotos')}</button>
            `
                  : ''
            }
          </div>
        `;
        const popup = createPopupElement(html, btnId, () => {
          if (stageImgs.length > 0) openGallery(stageImgs, 0);
          else openGallery(tourImgs, 0);
        });
        addPinMarker(startEl, stg.startCoord, popup);
      } else {
        // Intermediate Day Stage Pins (Stage 2, Stage 3, ...)
        const tooltip = `${t('dayStage', stg.stageNumber, numStages)}: ${formatShortDate(stg.startTime)}`;
        const stageEl = createPinElement('stage', stg.stageNumber, tooltip);

        const btnId = `btn-stage-photos-${segId}-stage-${stg.stageNumber}`;
        const html = `
          <div class="tour-popup-container">
            <div class="tour-popup-badge tour-popup-badge-stage">🚩 ${t('dayStage', stg.stageNumber, numStages)}</div>
            <div class="tour-popup-title">${t('dayStage', stg.stageNumber, numStages)}</div>
            <div class="tour-popup-date">📅 ${formatShortDate(stg.startTime)}</div>
            <div class="tour-popup-stats">
              <div class="tour-popup-stat-row">
                <span class="tour-popup-stat-label">${t('startTime')}:</span>
                <span class="tour-popup-stat-val">${formatTime(stg.startTime)}</span>
              </div>
              ${
                stageDuration
                  ? `
              <div class="tour-popup-stat-row">
                <span class="tour-popup-stat-label">${t('duration')}:</span>
                <span class="tour-popup-stat-val">${stageDuration}</span>
              </div>`
                  : ''
              }
              <div class="tour-popup-stat-row">
                <span class="tour-popup-stat-label">${t('stageDistance')}:</span>
                <span class="tour-popup-stat-val">${stg.distanceKm.toFixed(1)} km</span>
              </div>
              <div class="tour-popup-stat-row">
                <span class="tour-popup-stat-label">${t('totalSoFar')}:</span>
                <span class="tour-popup-stat-val">${stg.totalDistanceKm.toFixed(1)} km</span>
              </div>
            </div>
            ${
              stageImgs.length > 0
                ? `
              <button class="tour-popup-btn" id="${btnId}">${t('viewPhotos')} (${stageImgs.length})</button>
            `
                : tourImgs.length > 0
                  ? `
              <button class="tour-popup-btn" id="${btnId}">${t('viewPhotos')}</button>
            `
                  : ''
            }
          </div>
        `;
        const popup = createPopupElement(html, btnId, () => {
          if (stageImgs.length > 0) openGallery(stageImgs, 0);
          else openGallery(tourImgs, 0);
        });
        addPinMarker(stageEl, stg.startCoord, popup);
      }

      // For the final stage, also add the Goal Pin
      if (stgIdx === numStages - 1) {
        const goalTooltip = `${t('tourGoal')}: ${formatShortDate(stg.endTime)}`;
        const goalEl = createPinElement('goal', '', goalTooltip);

        const goalBtnId = `btn-stage-photos-${segId}-goal`;
        const totalDuration = formatDuration(segment.startTime, segment.endTime);
        const goalHtml = `
          <div class="tour-popup-container">
            <div class="tour-popup-badge tour-popup-badge-goal">🏁 ${t('tourGoal')}</div>
            <div class="tour-popup-title">${t('tourGoal')} • ${t('stageBadge', numStages)}</div>
            <div class="tour-popup-date">📅 ${formatShortDate(stg.endTime)}</div>
            <div class="tour-popup-stats">
              <div class="tour-popup-stat-row">
                <span class="tour-popup-stat-label">${t('endTime')}:</span>
                <span class="tour-popup-stat-val">${formatTime(stg.endTime)}</span>
              </div>
              ${
                totalDuration
                  ? `
              <div class="tour-popup-stat-row">
                <span class="tour-popup-stat-label">${t('duration')}:</span>
                <span class="tour-popup-stat-val">${totalDuration}</span>
              </div>`
                  : ''
              }
              <div class="tour-popup-stat-row">
                <span class="tour-popup-stat-label">${t('stageDistance')}:</span>
                <span class="tour-popup-stat-val">${stg.distanceKm.toFixed(1)} km</span>
              </div>
              <div class="tour-popup-stat-row">
                <span class="tour-popup-stat-label">${t('totalDistance')}:</span>
                <span class="tour-popup-stat-val">${(segment.distanceKm || stg.totalDistanceKm).toFixed(1)} km</span>
              </div>
            </div>
            ${
              stageImgs.length > 0
                ? `
              <button class="tour-popup-btn" id="${goalBtnId}">${t('viewPhotos')} (${stageImgs.length})</button>
            `
                : tourImgs.length > 0
                  ? `
              <button class="tour-popup-btn" id="${goalBtnId}">${t('viewPhotos')}</button>
            `
                  : ''
            }
          </div>
        `;
        const goalPopup = createPopupElement(goalHtml, goalBtnId, () => {
          if (stageImgs.length > 0) openGallery(stageImgs, stageImgs.length - 1);
          else openGallery(tourImgs, tourImgs.length - 1);
        });
        addPinMarker(goalEl, stg.endCoord, goalPopup);
      }
    });
  }
});
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
      if (Array.isArray(segment.bounds) && segment.bounds.length === 4) {
        bounds.extend([segment.bounds[1], segment.bounds[0]]);
        bounds.extend([segment.bounds[3], segment.bounds[2]]);
        count += 2;
        continue;
      }
      if (Array.isArray(segment.lines)) {
        for (const line of segment.lines) {
          if (Array.isArray(line.coords)) {
            for (const pt of line.coords) {
              bounds.extend([pt[1], pt[0]]);
              count++;
            }
          }
        }
        continue;
      }
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
  drawStageMarkers(segments);
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
              drawStageMarkers(segments);
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

