document.addEventListener("alpine:init", () => {
  Alpine.data("library", () => ({
    selectedId: null,
    activeFolder: "",
    activeMediaType: "",
    fullPreview: {
      open: false,
      src: "",
      alt: "",
    },

    init() {
      this.syncFromLocation();

      window.addEventListener("popstate", () => {
        this.syncFromLocation();
      });

      document.addEventListener("keydown", (event) => {
        this.handleKeydown(event);
      });

      document.body.addEventListener("htmx:afterSettle", () => {
        this.syncFromLocation();
      });
    },

    selectMedia(id) {
      this.selectedId = String(id);
    },

    openFullPreview(src, alt) {
      this.fullPreview = {
        open: true,
        src: src || "",
        alt: alt || "Preview",
      };
    },

    closeFullPreview() {
      this.fullPreview.open = false;
    },

    openFullPreviewFromCard(card) {
      if (!card?.dataset?.fullSrc) {
        return;
      }

      this.selectedId = card.dataset.mediaId || this.selectedId;
      this.openFullPreview(card.dataset.fullSrc, card.dataset.fullAlt || "Preview");
    },

    setMediaType(mediaType) {
      this.activeMediaType = mediaType === "photo" || mediaType === "video" ? mediaType : "";
      if (this.$refs.mediaTypeInput) {
        this.$refs.mediaTypeInput.value = this.activeMediaType;
      }
    },

    setActiveFolder(folder) {
      this.activeFolder = this.normalizeFolder(folder);
      if (this.$refs.folderInput) {
        this.$refs.folderInput.value = this.activeFolder;
      }
      this.syncActiveFolderLinks();
    },

    syncToolbarMediaType() {
      if (this.$refs.mediaTypeInput) {
        this.$refs.mediaTypeInput.value = this.activeMediaType;
      }
    },

    clearDetail() {
      const detail = document.getElementById("detail-panel");
      const template = document.getElementById("empty-detail-template");
      if (detail && template) {
        detail.innerHTML = template.innerHTML;
      }

      this.closeFullPreview();
      this.selectedId = null;
      const state = document.getElementById("library-state");
      const listUrl = state?.dataset?.listUrl || "/media";
      history.pushState({}, "", listUrl);
      this.syncFromLocation();
    },

    handleKeydown(event) {
      if (this.fullPreview.open) {
        if (event.key === "Escape") {
          event.preventDefault();
          this.closeFullPreview();
          return;
        }

        if (["ArrowRight", "ArrowDown", "ArrowLeft", "ArrowUp"].includes(event.key)) {
          event.preventDefault();
          const direction = event.key === "ArrowRight" || event.key === "ArrowDown" ? 1 : -1;
          this.showAdjacentFullPreview(direction);
        }

        return;
      }

      const tag = document.activeElement?.tagName;
      if (["INPUT", "TEXTAREA", "SELECT", "BUTTON", "VIDEO", "AUDIO"].includes(tag)) {
        return;
      }

      if (event.key === "Escape" && this.selectedId) {
        event.preventDefault();
        this.clearDetail();
        return;
      }

      if (!["ArrowRight", "ArrowDown", "ArrowLeft", "ArrowUp", "Enter"].includes(event.key)) {
        return;
      }

      const cards = Array.from(document.querySelectorAll("[data-media-card]"));
      if (cards.length === 0) {
        return;
      }

      let index = cards.findIndex((card) => card.dataset.mediaId === this.selectedId);
      if (index < 0) {
        index = Math.max(0, cards.indexOf(document.activeElement));
      }

      if (event.key === "Enter") {
        if (index >= 0) {
          event.preventDefault();
          cards[index].click();
        }
        return;
      }

      event.preventDefault();
      const direction = event.key === "ArrowRight" || event.key === "ArrowDown" ? 1 : -1;
      const nextIndex = Math.min(cards.length - 1, Math.max(0, index + direction));
      const next = cards[nextIndex];
      next.focus({ preventScroll: false });
      next.click();
    },

    showAdjacentFullPreview(direction) {
      const cards = Array.from(document.querySelectorAll("[data-media-card][data-full-src]"));
      if (cards.length === 0) {
        return;
      }

      let index = cards.findIndex((card) => card.dataset.mediaId === this.selectedId);
      if (index < 0) {
        index = cards.findIndex((card) => card.dataset.fullSrc === this.fullPreview.src);
      }
      if (index < 0) {
        return;
      }

      const nextIndex = Math.min(cards.length - 1, Math.max(0, index + direction));
      if (nextIndex === index) {
        return;
      }

      const next = cards[nextIndex];
      this.openFullPreviewFromCard(next);
      next.click();
    },

    idFromLocation() {
      const match = window.location.pathname.match(/^\/media\/(\d+)/);
      return match ? match[1] : null;
    },

    mediaTypeFromLocation() {
      const mediaType = new URLSearchParams(window.location.search).get("media_type");
      return mediaType === "photo" || mediaType === "video" ? mediaType : "";
    },

    folderFromLocation() {
      return this.normalizeFolder(new URLSearchParams(window.location.search).get("folder") || "");
    },

    normalizeFolder(folder) {
      return String(folder || "").trim().replace(/^\/+|\/+$/g, "");
    },

    syncActiveFolderLinks() {
      document.querySelectorAll("[data-folder-link]").forEach((link) => {
        const isActive = (link.dataset.folder || "") === this.activeFolder;
        link.classList.toggle("is-active", isActive);
        if (isActive) {
          link.setAttribute("aria-current", "page");
        } else {
          link.removeAttribute("aria-current");
        }
      });
    },

    syncFromLocation() {
      this.selectedId = this.idFromLocation();
      this.setMediaType(this.mediaTypeFromLocation());
      this.setActiveFolder(this.folderFromLocation());
    },
  }));
});
