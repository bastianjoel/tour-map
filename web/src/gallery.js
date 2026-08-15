// Gallery controller
import { t, formatDate, applyStaticTranslations } from "./i18n.js";

const galleryModal = document.getElementById("gallery-modal");
const galleryMainImg = document.getElementById("gallery-main-img");
const galleryCounter = document.getElementById("gallery-counter");
const galleryDate = document.getElementById("gallery-date");
const galleryFocusBtn = document.getElementById("gallery-focus-btn");
const galleryThumbs = document.getElementById("gallery-thumbs");

let activeTourImages = [];
let currentImageIndex = 0;
let mapInstance = null;

export function setGalleryMap(map) {
  mapInstance = map;
}

export function showToast(message) {
  const toast = document.getElementById("toast");
  if (!toast) return;
  toast.textContent = message;
  toast.classList.add("show");
  setTimeout(() => {
    toast.classList.remove("show");
  }, 3000);
}

function renderThumbnails() {
  if (!galleryThumbs) return;
  galleryThumbs.innerHTML = "";

  activeTourImages.forEach((img, idx) => {
    const item = document.createElement("div");
    item.className = `gallery-thumb-item ${idx === currentImageIndex ? "active" : ""}`;
    item.id = `thumb-${idx}`;

    const thumbImg = document.createElement("img");
    thumbImg.className = "gallery-thumb-img";
    thumbImg.src = `/images/${encodeURI(img.filename)}`;
    thumbImg.alt = img.filename || t("imageAlt");
    thumbImg.loading = "lazy";

    item.appendChild(thumbImg);

    if (img.location && img.location.lat) {
      const geoIcon = document.createElement("span");
      geoIcon.className = "gallery-thumb-geo-icon";
      geoIcon.textContent = "📍";
      item.appendChild(geoIcon);
    }

    item.addEventListener("click", () => {
      showImage(idx);
    });

    galleryThumbs.appendChild(item);
  });
}

export function showImage(index) {
  if (!activeTourImages || activeTourImages.length === 0) return;
  if (index < 0) index = activeTourImages.length - 1;
  if (index >= activeTourImages.length) index = 0;
  currentImageIndex = index;

  const img = activeTourImages[currentImageIndex];
  if (galleryMainImg) {
    galleryMainImg.src = `/images/${encodeURI(img.filename)}`;
    galleryMainImg.alt = img.filename || t("imageAlt");
  }
  if (galleryCounter) {
    galleryCounter.textContent = t(
      "photoOf",
      currentImageIndex + 1,
      activeTourImages.length,
    );
  }
  if (galleryDate) {
    galleryDate.textContent = formatDate(img.timestamp);
  }

  // Configure "Focus on Map" button
  if (galleryFocusBtn) {
    if (img.location && img.location.lat && img.location.lng) {
      galleryFocusBtn.style.display = "inline-flex";
      galleryFocusBtn.textContent = t("focusOnMap");
      galleryFocusBtn.onclick = () => {
        if (mapInstance) {
          mapInstance.flyTo({
            center: [img.location.lng, img.location.lat],
            zoom: Math.max(mapInstance.getZoom(), 15),
            speed: 1.2,
          });
        }
        closeGallery();
      };
    } else {
      galleryFocusBtn.style.display = "none";
      galleryFocusBtn.onclick = null;
    }
  }

  // Highlight active thumbnail and scroll into view
  document.querySelectorAll(".gallery-thumb-item").forEach((el, i) => {
    el.classList.toggle("active", i === currentImageIndex);
  });
  const activeThumb = document.getElementById(`thumb-${currentImageIndex}`);
  if (activeThumb) {
    activeThumb.scrollIntoView({
      behavior: "smooth",
      block: "nearest",
      inline: "center",
    });
  }
}

export function openGallery(imagesToDisplay, startIndex = 0) {
  if (!imagesToDisplay || imagesToDisplay.length === 0) {
    showToast(t("noPhotos"));
    return;
  }
  activeTourImages = imagesToDisplay;
  renderThumbnails();
  if (galleryModal) {
    galleryModal.classList.add("open");
  }
  showImage(startIndex);
}

export function closeGallery() {
  if (galleryModal) {
    galleryModal.classList.remove("open");
  }
}

// Attach event listeners
export function initGalleryEvents() {
  applyStaticTranslations();

  const closeBtn = document.getElementById("gallery-close-btn");
  const prevBtn = document.getElementById("gallery-prev-btn");
  const nextBtn = document.getElementById("gallery-next-btn");

  if (closeBtn) closeBtn.addEventListener("click", closeGallery);
  if (prevBtn)
    prevBtn.addEventListener("click", () => showImage(currentImageIndex - 1));
  if (nextBtn)
    nextBtn.addEventListener("click", () => showImage(currentImageIndex + 1));

  window.addEventListener("keydown", (e) => {
    if (galleryModal && !galleryModal.classList.contains("open")) return;
    if (e.key === "Escape") closeGallery();
    if (e.key === "ArrowLeft") showImage(currentImageIndex - 1);
    if (e.key === "ArrowRight") showImage(currentImageIndex + 1);
  });
}
