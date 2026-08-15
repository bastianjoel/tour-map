// Client-side i18n translation system supporting German and English

const urlParams =
  typeof window !== "undefined"
    ? new URLSearchParams(window.location.search)
    : null;
const langParam = urlParams ? urlParams.get("lang") : null;
const browserLang = (
  typeof navigator !== "undefined"
    ? navigator.language || navigator.userLanguage || "en"
    : "en"
).toLowerCase();

export const currentLang = (langParam || browserLang).startsWith("de")
  ? "de"
  : "en";

export const translations = {
  en: {
    title: "Tour Map",
    focusOnMap: "📍 Focus on Map",
    close: "Close (Esc)",
    prev: "Previous (Left Arrow)",
    next: "Next (Right Arrow)",
    photoOf: (cur, total) => `Photo ${cur} of ${total}`,
    noPhotos: "No photos recorded for this tour",
    noGPS: "📅 Date only • No GPS Coordinates",
    dateUnknown: "Date unknown",
    imageAlt: "Tour Image",
  },
  de: {
    title: "Tour-Karte",
    focusOnMap: "📍 Auf Karte zeigen",
    close: "Schließen (Esc)",
    prev: "Vorheriges (Pfeiltaste links)",
    next: "Nächstes (Pfeiltaste rechts)",
    photoOf: (cur, total) => `Foto ${cur} von ${total}`,
    noPhotos: "Keine Fotos für diese Tour vorhanden",
    noGPS: "📅 Nur Datum • Keine GPS-Koordinaten",
    dateUnknown: "Datum unbekannt",
    imageAlt: "Tour-Foto",
  },
};

export function t(key, ...args) {
  const dict = translations[currentLang] || translations.en;
  const val = dict[key] || translations.en[key] || key;
  if (typeof val === "function") {
    return val(...args);
  }
  return val;
}

export function formatDate(dateStr) {
  if (!dateStr) return t("dateUnknown");
  const d = new Date(dateStr);
  if (isNaN(d.getTime())) return dateStr;

  const locale = currentLang === "de" ? "de-DE" : "en-US";
  return d.toLocaleString(locale, {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}

export function applyStaticTranslations() {
  if (typeof document === "undefined") return;

  document.title = t("title");

  const focusBtn = document.getElementById("gallery-focus-btn");
  if (focusBtn) focusBtn.textContent = t("focusOnMap");

  const closeBtn = document.getElementById("gallery-close-btn");
  if (closeBtn) closeBtn.title = t("close");

  const prevBtn = document.getElementById("gallery-prev-btn");
  if (prevBtn) prevBtn.title = t("prev");

  const nextBtn = document.getElementById("gallery-next-btn");
  if (nextBtn) nextBtn.title = t("next");
}
