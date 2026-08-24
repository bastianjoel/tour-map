// Client-side i18n translation system supporting German and English

const urlParams = typeof window !== 'undefined' ? new URLSearchParams(window.location.search) : null;
const langParam = urlParams ? urlParams.get('lang') : null;
const browserLang = (typeof navigator !== 'undefined' ? (navigator.language || navigator.userLanguage || 'en') : 'en').toLowerCase();

export const currentLang = (langParam || browserLang).startsWith('de') ? 'de' : 'en';

export const translations = {
  en: {
    title: 'Tour Map',
    focusOnMap: '📍 Focus on Map',
    close: 'Close (Esc)',
    prev: 'Previous (Left Arrow)',
    next: 'Next (Right Arrow)',
    photoOf: (cur, total) => `Photo ${cur} of ${total}`,
    noPhotos: 'No photos recorded for this tour',
    noGPS: '📅 Date only • No GPS Coordinates',
    dateUnknown: 'Date unknown',
    imageAlt: 'Tour Image',
    mapLiberty: '🗺️ Map',
    mapSatellite: '🛰️ Satellite',
    start: 'Start',
    goal: 'Goal',
    startGoal: 'Start / Goal',
    tourStart: 'Tour Start',
    tourGoal: 'Tour Goal',
    stage: 'Stage',
    dayStage: (num, total) => (total > 1 ? `Stage ${num} of ${total}` : `Stage ${num}`),
    stageBadge: (num) => `Stage ${num}`,
    day: (num) => `Day ${num}`,
    distance: 'Distance',
    stageDistance: 'Stage Distance',
    totalDistance: 'Total Distance',
    totalSoFar: 'Total so far',
    duration: 'Duration',
    startTime: 'Start Time',
    endTime: 'End Time',
    viewPhotos: '📷 View Photos',
    noPhotosForStage: 'No photos recorded for this stage',
  },
  de: {
    title: 'Tour-Karte',
    focusOnMap: '📍 Auf Karte zeigen',
    close: 'Schließen (Esc)',
    prev: 'Vorheriges (Pfeiltaste links)',
    next: 'Nächstes (Pfeiltaste rechts)',
    photoOf: (cur, total) => `Foto ${cur} von ${total}`,
    noPhotos: 'Keine Fotos für diese Tour vorhanden',
    noGPS: '📅 Nur Datum • Keine GPS-Koordinaten',
    dateUnknown: 'Datum unbekannt',
    imageAlt: 'Tour-Foto',
    mapLiberty: '🗺️ Karte',
    mapSatellite: '🛰️ Satellit',
    start: 'Start',
    goal: 'Ziel',
    startGoal: 'Start / Ziel',
    tourStart: 'Tour-Start',
    tourGoal: 'Tour-Ziel',
    stage: 'Etappe',
    dayStage: (num, total) => (total > 1 ? `Etappe ${num} von ${total}` : `Etappe ${num}`),
    stageBadge: (num) => `Etappe ${num}`,
    day: (num) => `Tag ${num}`,
    distance: 'Distanz',
    stageDistance: 'Etappendistanz',
    totalDistance: 'Gesamtdistanz',
    totalSoFar: 'Bisher gesamt',
    duration: 'Dauer',
    startTime: 'Startzeit',
    endTime: 'Endzeit',
    viewPhotos: '📷 Fotos ansehen',
    noPhotosForStage: 'Keine Fotos für diese Etappe vorhanden',
  },
};

export function t(key, ...args) {
  const dict = translations[currentLang] || translations.en;
  const val = dict[key] || translations.en[key] || key;
  if (typeof val === 'function') {
    return val(...args);
  }
  return val;
}

export function formatDate(dateStr) {
  if (!dateStr) return t('dateUnknown');
  const d = new Date(dateStr);
  if (isNaN(d.getTime())) return dateStr;

  const locale = currentLang === 'de' ? 'de-DE' : 'en-US';
  return d.toLocaleString(locale, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  });
}

export function formatShortDate(dateStr) {
  if (!dateStr) return t('dateUnknown');
  const d = new Date(dateStr);
  if (isNaN(d.getTime())) return dateStr;

  const locale = currentLang === 'de' ? 'de-DE' : 'en-US';
  return d.toLocaleDateString(locale, {
    weekday: 'short',
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  });
}

export function formatTime(dateStr) {
  if (!dateStr) return '';
  const d = new Date(dateStr);
  if (isNaN(d.getTime())) return '';

  const locale = currentLang === 'de' ? 'de-DE' : 'en-US';
  return d.toLocaleTimeString(locale, {
    hour: '2-digit',
    minute: '2-digit',
  });
}

export function formatDuration(startStr, endStr) {
  if (!startStr || !endStr) return '';
  const s = new Date(startStr).getTime();
  const e = new Date(endStr).getTime();
  if (isNaN(s) || isNaN(e) || e <= s) return '';

  const totalMin = Math.round((e - s) / (60 * 1000));
  const hours = Math.floor(totalMin / 60);
  const minutes = totalMin % 60;

  if (hours === 0) return `${minutes}m`;
  return `${hours}h ${minutes}m`;
}

export function applyStaticTranslations() {
  if (typeof document === 'undefined') return;

  document.title = t('title');

  const focusBtn = document.getElementById('gallery-focus-btn');
  if (focusBtn) focusBtn.textContent = t('focusOnMap');

  const closeBtn = document.getElementById('gallery-close-btn');
  if (closeBtn) closeBtn.title = t('close');

  const prevBtn = document.getElementById('gallery-prev-btn');
  if (prevBtn) prevBtn.title = t('prev');

  const nextBtn = document.getElementById('gallery-next-btn');
  if (nextBtn) nextBtn.title = t('next');

  const libertyBtn = document.getElementById('style-liberty-btn');
  if (libertyBtn) libertyBtn.textContent = t('mapLiberty');

  const satelliteBtn = document.getElementById('style-satellite-btn');
  if (satelliteBtn) satelliteBtn.textContent = t('mapSatellite');
}
