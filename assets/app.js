document.addEventListener("alpine:init", () => {
  Alpine.data("library", () => ({
    selectedId: null,

    init() {
      this.selectedId = this.idFromLocation();

      window.addEventListener("popstate", () => {
        this.selectedId = this.idFromLocation();
      });

      document.addEventListener("keydown", (event) => {
        this.handleKeydown(event);
      });

      document.body.addEventListener("htmx:afterSwap", () => {
        this.selectedId = this.idFromLocation();
      });
    },

    selectMedia(id) {
      this.selectedId = String(id);
    },

    clearDetail() {
      const detail = document.getElementById("detail-panel");
      const template = document.getElementById("empty-detail-template");
      if (detail && template) {
        detail.innerHTML = template.innerHTML;
      }

      this.selectedId = null;
      const state = document.getElementById("library-state");
      const listUrl = state?.dataset?.listUrl || "/media";
      history.pushState({}, "", listUrl);
    },

    handleKeydown(event) {
      const tag = document.activeElement?.tagName;
      if (["INPUT", "TEXTAREA", "SELECT", "BUTTON"].includes(tag)) {
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

    idFromLocation() {
      const match = window.location.pathname.match(/^\/media\/(\d+)/);
      return match ? match[1] : null;
    },
  }));
});
