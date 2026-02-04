package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
)

type Handler struct {
	cfg     *Config
	db      *Database
	scanner *Scanner
}

func NewHandler(cfg *Config, db *Database, scanner *Scanner) *Handler {
	return &Handler{cfg: cfg, db: db, scanner: scanner}
}

func (h *Handler) ListPhotos(w http.ResponseWriter, r *http.Request) {
	folder := r.URL.Query().Get("folder")

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}

	photos, err := h.db.ListPhotos(folder, limit, offset)
	if err != nil {
		log.Printf("Error listing photos: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	h.jsonResponse(w, photos)
}

func (h *Handler) GetPhoto(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	photo, err := h.db.GetPhotoByID(id)
	if err != nil {
		http.Error(w, "Photo not found", http.StatusNotFound)
		return
	}

	h.jsonResponse(w, photo)
}

func (h *Handler) GetThumbnail(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	photo, err := h.db.GetPhotoByID(id)
	if err != nil {
		http.Error(w, "Photo not found", http.StatusNotFound)
		return
	}

	// Serve the thumbnail file
	h.serveFile(w, r, photo.ThumbnailPath, "image/jpeg")
}

func (h *Handler) GetOriginal(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	photo, err := h.db.GetPhotoByID(id)
	if err != nil {
		http.Error(w, "Photo not found", http.StatusNotFound)
		return
	}

	// Set content-disposition for download
	w.Header().Set("Content-Disposition", "attachment; filename=\""+photo.Filename+"\"")
	h.serveFile(w, r, photo.OriginalPath, "application/octet-stream")
}

func (h *Handler) ListFolders(w http.ResponseWriter, r *http.Request) {
	folders, err := h.db.ListFolders()
	if err != nil {
		log.Printf("Error listing folders: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	h.jsonResponse(w, folders)
}

func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.db.GetStats()
	if err != nil {
		log.Printf("Error getting stats: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	h.jsonResponse(w, stats)
}

func (h *Handler) jsonResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Error encoding JSON: %v", err)
	}
}

func (h *Handler) TriggerScan(w http.ResponseWriter, r *http.Request) {
	if !h.scanner.TryScan() {
		w.WriteHeader(http.StatusConflict)
		h.jsonResponse(w, map[string]string{"status": "already_running"})
		return
	}
	w.WriteHeader(http.StatusAccepted)
	h.jsonResponse(w, map[string]string{"status": "started"})
}

func (h *Handler) serveFile(w http.ResponseWriter, r *http.Request, path, contentType string) {
	file, err := os.Open(path)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.FormatInt(stat.Size(), 10))
	w.Header().Set("Cache-Control", "public, max-age=86400")

	io.Copy(w, file)
}
