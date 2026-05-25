package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	sessionCookieName = "glimpse_session"
	defaultPageSize   = 80
)

//go:embed templates/*.html assets/*
var embeddedFiles embed.FS

type Handler struct {
	cfg       *Config
	db        *Database
	scanner   *Scanner
	templates *template.Template
	assets    http.Handler
}

func NewHandler(cfg *Config, db *Database, scanner *Scanner) (*Handler, error) {
	tmpl, err := template.New("").Funcs(templateFuncs()).ParseFS(embeddedFiles, "templates/*.html")
	if err != nil {
		return nil, err
	}

	assetFS, err := fs.Sub(embeddedFiles, "assets")
	if err != nil {
		return nil, err
	}

	return &Handler{
		cfg:       cfg,
		db:        db,
		scanner:   scanner,
		templates: tmpl,
		assets:    http.FileServer(http.FS(assetFS)),
	}, nil
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", cacheAssets(h.assets)))

	mux.HandleFunc("GET /", h.Home)
	mux.HandleFunc("GET /login", h.LoginPage)
	mux.HandleFunc("POST /login", h.Login)
	mux.HandleFunc("POST /logout", h.Logout)

	mux.HandleFunc("GET /media", h.MediaIndex)
	mux.HandleFunc("GET /media/grid", h.MediaGrid)
	mux.HandleFunc("GET /media/{id}", h.MediaDetail)
	mux.HandleFunc("GET /media/{id}/thumbnail", h.GetThumbnail)
	mux.HandleFunc("GET /media/{id}/original", h.GetOriginal)
	mux.HandleFunc("GET /media/{id}/stream", h.StreamVideo)

	mux.HandleFunc("POST /scan", h.TriggerScan)
	mux.HandleFunc("GET /scan/status", h.ScanStatus)

	mux.HandleFunc("GET /api", http.NotFound)
	mux.HandleFunc("GET /api/", http.NotFound)
	mux.HandleFunc("POST /api/", http.NotFound)

	return securityHeaders(h.authMiddleware(mux))
}

func (h *Handler) Home(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/media", http.StatusFound)
}

func (h *Handler) LoginPage(w http.ResponseWriter, r *http.Request) {
	if h.cfg.APIKey == "" {
		http.Redirect(w, r, "/media", http.StatusFound)
		return
	}
	h.render(w, http.StatusOK, "login_page", LoginData{
		Next: sanitizeNext(r.URL.Query().Get("next")),
	})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if h.cfg.APIKey == "" {
		http.Redirect(w, r, "/media", http.StatusFound)
		return
	}
	if err := r.ParseForm(); err != nil {
		h.render(w, http.StatusBadRequest, "login_page", LoginData{Error: "Could not read the login form."})
		return
	}

	provided := []byte(r.Form.Get("api_key"))
	expected := []byte(h.cfg.APIKey)
	if subtle.ConstantTimeCompare(provided, expected) != 1 {
		h.render(w, http.StatusUnauthorized, "login_page", LoginData{
			Error: "That API key did not match.",
			Next:  sanitizeNext(r.Form.Get("next")),
		})
		return
	}

	http.SetCookie(w, h.sessionCookie(r.TLS != nil))
	http.Redirect(w, r, sanitizeNext(r.Form.Get("next")), http.StatusFound)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
	})
	http.Redirect(w, r, "/login", http.StatusFound)
}

func (h *Handler) MediaIndex(w http.ResponseWriter, r *http.Request) {
	data, err := h.libraryData(r, nil)
	if err != nil {
		h.serverError(w, err)
		return
	}
	h.render(w, http.StatusOK, "library_page", data)
}

func (h *Handler) MediaGrid(w http.ResponseWriter, r *http.Request) {
	filter := parseMediaFilter(r)
	grid, err := h.gridData(filter)
	if err != nil {
		h.serverError(w, err)
		return
	}

	if r.URL.Query().Get("append") == "1" {
		h.render(w, http.StatusOK, "media_cards_append", grid)
		return
	}

	grid.IncludeDetailReset = true
	w.Header().Set("HX-Push-Url", mediaURL(filter))
	h.render(w, http.StatusOK, "grid_region", grid)
}

func (h *Handler) MediaDetail(w http.ResponseWriter, r *http.Request) {
	item, err := h.mediaItemFromRequest(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if isHTMX(r) {
		h.render(w, http.StatusOK, "detail_panel", DetailData{
			Item:   item,
			Filter: parseMediaFilter(r),
		})
		return
	}

	data, err := h.libraryData(r, item)
	if err != nil {
		h.serverError(w, err)
		return
	}
	h.render(w, http.StatusOK, "library_page", data)
}

func (h *Handler) GetThumbnail(w http.ResponseWriter, r *http.Request) {
	item, err := h.mediaItemFromRequest(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	h.serveFile(w, r, item.ThumbnailPath, item.Filename+".jpg", "image/jpeg", false)
}

func (h *Handler) GetOriginal(w http.ResponseWriter, r *http.Request) {
	item, err := h.mediaItemFromRequest(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	h.serveFile(w, r, item.OriginalPath, item.Filename, "application/octet-stream", true)
}

func (h *Handler) StreamVideo(w http.ResponseWriter, r *http.Request) {
	item, err := h.mediaItemFromRequest(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if !item.IsVideo() {
		http.Error(w, "Not a video", http.StatusBadRequest)
		return
	}
	h.serveFile(w, r, item.OriginalPath, item.Filename, videoContentType(item.Extension), false)
}

func (h *Handler) TriggerScan(w http.ResponseWriter, r *http.Request) {
	h.scanner.TryScan()
	h.render(w, http.StatusOK, "scan_status", ScanStatusData{Scanning: h.scanner.IsScanning()})
}

func (h *Handler) ScanStatus(w http.ResponseWriter, r *http.Request) {
	h.render(w, http.StatusOK, "scan_status", ScanStatusData{Scanning: h.scanner.IsScanning()})
}

func (h *Handler) libraryData(r *http.Request, selected *MediaItem) (*LibraryData, error) {
	filter := parseMediaFilter(r)
	grid, err := h.gridData(filter)
	if err != nil {
		return nil, err
	}
	stats, err := h.db.GetStats()
	if err != nil {
		return nil, err
	}
	folders, err := h.db.ListFolders()
	if err != nil {
		return nil, err
	}

	return &LibraryData{
		AuthEnabled: h.cfg.APIKey != "",
		Stats:       stats,
		FolderTree:  buildFolderTree(folders, filter),
		Grid:        grid,
		Detail: DetailData{
			Item:   selected,
			Filter: filter,
		},
		AllURL:      mediaURL(clearFolder(filter)),
		AllGridURL:  gridURL(clearFolder(filter), 0, false),
		Filter:      filter,
		CurrentURL:  mediaURL(filter),
		ScanRunning: h.scanner.IsScanning(),
		Scan:        ScanStatusData{Scanning: h.scanner.IsScanning()},
	}, nil
}

func (h *Handler) gridData(filter MediaFilter) (*GridData, error) {
	items, err := h.db.ListMediaItems(filter)
	if err != nil {
		return nil, err
	}
	count, err := h.db.CountMediaItems(filter)
	if err != nil {
		return nil, err
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = defaultPageSize
	}
	nextOffset := filter.Offset + len(items)

	return &GridData{
		Items:       items,
		Filter:      filter,
		Count:       count,
		Limit:       limit,
		NextOffset:  nextOffset,
		HasMore:     nextOffset < count,
		FolderTitle: folderTitle(filter.Folder),
	}, nil
}

func (h *Handler) mediaItemFromRequest(r *http.Request) (*MediaItem, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return nil, err
	}
	return h.db.GetMediaItemByID(id)
}

func (h *Handler) serveFile(w http.ResponseWriter, r *http.Request, path, filename, contentType string, attachment bool) {
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
	w.Header().Set("Cache-Control", "public, max-age=86400")
	if attachment {
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	}
	http.ServeContent(w, r, filename, stat.ModTime(), file)
}

func (h *Handler) render(w http.ResponseWriter, status int, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := h.templates.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("template %s failed: %v", name, err)
	}
}

func (h *Handler) serverError(w http.ResponseWriter, err error) {
	log.Printf("server error: %v", err)
	http.Error(w, "Internal server error", http.StatusInternalServerError)
}

func (h *Handler) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.authExempt(r.URL.Path) || h.cfg.APIKey == "" || h.authenticated(r) {
			next.ServeHTTP(w, r)
			return
		}

		if strings.HasPrefix(r.URL.Path, "/api") {
			next.ServeHTTP(w, r)
			return
		}

		if isHTMX(r) {
			w.Header().Set("HX-Redirect", loginURL(r))
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if wantsHTML(r) {
			http.Redirect(w, r, loginURL(r), http.StatusFound)
			return
		}

		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	})
}

func (h *Handler) authExempt(path string) bool {
	return strings.HasPrefix(path, "/assets/") || path == "/login"
}

func (h *Handler) authenticated(r *http.Request) bool {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return false
	}
	expected := []byte(h.sessionToken())
	provided := []byte(cookie.Value)
	return subtle.ConstantTimeCompare(provided, expected) == 1
}

func (h *Handler) sessionCookie(secure bool) *http.Cookie {
	return &http.Cookie{
		Name:     sessionCookieName,
		Value:    h.sessionToken(),
		Path:     "/",
		MaxAge:   int((30 * 24 * time.Hour).Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
	}
}

func (h *Handler) sessionToken() string {
	sum := sha256.Sum256([]byte("glimpse-session:" + h.cfg.APIKey))
	return hex.EncodeToString(sum[:])
}

func parseMediaFilter(r *http.Request) MediaFilter {
	query := r.URL.Query()
	limit, _ := strconv.Atoi(query.Get("limit"))
	if limit <= 0 || limit > 200 {
		limit = defaultPageSize
	}
	offset, _ := strconv.Atoi(query.Get("offset"))
	if offset < 0 {
		offset = 0
	}

	mediaType := query.Get("media_type")
	if mediaType != MediaTypePhoto && mediaType != MediaTypeVideo {
		mediaType = ""
	}

	return MediaFilter{
		Folder:    strings.Trim(strings.TrimSpace(query.Get("folder")), "/"),
		MediaType: mediaType,
		Query:     strings.TrimSpace(query.Get("q")),
		Limit:     limit,
		Offset:    offset,
	}
}

func sanitizeNext(next string) string {
	if next == "" {
		return "/media"
	}
	u, err := url.Parse(next)
	if err != nil || u.IsAbs() || !strings.HasPrefix(u.Path, "/") {
		return "/media"
	}
	if strings.HasPrefix(u.Path, "/login") {
		return "/media"
	}
	return u.RequestURI()
}

func loginURL(r *http.Request) string {
	return "/login?next=" + url.QueryEscape(r.URL.RequestURI())
}

func wantsHTML(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	return accept == "" || strings.Contains(accept, "text/html") || strings.Contains(accept, "*/*")
}

func isHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

func cacheAssets(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		next.ServeHTTP(w, r)
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

type LoginData struct {
	Error string
	Next  string
}

type LibraryData struct {
	AuthEnabled bool
	Stats       *Stats
	FolderTree  []*FolderNode
	Grid        *GridData
	Detail      DetailData
	AllURL      string
	AllGridURL  string
	Filter      MediaFilter
	CurrentURL  string
	ScanRunning bool
	Scan        ScanStatusData
}

type GridData struct {
	Items              []*MediaItem
	Filter             MediaFilter
	Count              int
	Limit              int
	NextOffset         int
	HasMore            bool
	FolderTitle        string
	IncludeDetailReset bool
}

type DetailData struct {
	Item   *MediaItem
	Filter MediaFilter
}

type ScanStatusData struct {
	Scanning bool
}

type FolderNode struct {
	Path     string
	Name     string
	Count    int
	Active   bool
	Open     bool
	URL      string
	GridURL  string
	Children []*FolderNode
}

func buildFolderTree(folders []*Folder, filter MediaFilter) []*FolderNode {
	root := map[string]*FolderNode{}
	nodes := map[string]*FolderNode{}

	for _, folder := range folders {
		parts := strings.Split(folder.Path, "/")
		for i := range parts {
			path := strings.Join(parts[:i+1], "/")
			if _, ok := nodes[path]; !ok {
				nodeFilter := filter
				nodeFilter.Folder = path
				nodeFilter.Offset = 0
				nodes[path] = &FolderNode{
					Path:    path,
					Name:    filepath.Base(path),
					Active:  filter.Folder == path,
					Open:    filter.Folder == path || strings.HasPrefix(filter.Folder, path+"/"),
					URL:     mediaURL(nodeFilter),
					GridURL: gridURL(nodeFilter, 0, false),
				}
			}
		}
		nodes[folder.Path].Count = folder.MediaCount
	}

	for path, node := range nodes {
		if parentPath, ok := parentFolder(path); ok {
			parent := nodes[parentPath]
			parent.Children = append(parent.Children, node)
			if node.Open || node.Active {
				parent.Open = true
			}
		} else {
			root[path] = node
		}
	}

	roots := make([]*FolderNode, 0, len(root))
	for _, node := range root {
		sortFolderNode(node)
		roots = append(roots, node)
	}
	sort.Slice(roots, func(i, j int) bool {
		return roots[i].Path < roots[j].Path
	})
	return roots
}

func sortFolderNode(node *FolderNode) {
	sort.Slice(node.Children, func(i, j int) bool {
		return node.Children[i].Path < node.Children[j].Path
	})
	for _, child := range node.Children {
		sortFolderNode(child)
	}
}

func parentFolder(path string) (string, bool) {
	idx := strings.LastIndex(path, "/")
	if idx < 0 {
		return "", false
	}
	return path[:idx], true
}

func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"mediaURL":       mediaURL,
		"gridURL":        gridURL,
		"detailURL":      detailURL,
		"folderTitle":    folderTitle,
		"formatBytes":    formatBytes,
		"formatDate":     formatDate,
		"formatDuration": formatDuration,
		"formatMB":       formatMB,
		"mediaTypeName":  mediaTypeName,
		"add":            func(a, b int) int { return a + b },
	}
}

func mediaURL(filter MediaFilter) string {
	return "/media" + filterQuery(filter, 0, false)
}

func clearFolder(filter MediaFilter) MediaFilter {
	filter.Folder = ""
	filter.Offset = 0
	return filter
}

func gridURL(filter MediaFilter, offset int, appendMode bool) string {
	return "/media/grid" + filterQuery(filter, offset, appendMode)
}

func detailURL(id int64, filter MediaFilter) string {
	return fmt.Sprintf("/media/%d%s", id, filterQuery(filter, 0, false))
}

func filterQuery(filter MediaFilter, offset int, appendMode bool) string {
	values := url.Values{}
	if filter.Folder != "" {
		values.Set("folder", filter.Folder)
	}
	if filter.MediaType != "" {
		values.Set("media_type", filter.MediaType)
	}
	if filter.Query != "" {
		values.Set("q", filter.Query)
	}
	if filter.Limit > 0 && filter.Limit != defaultPageSize {
		values.Set("limit", strconv.Itoa(filter.Limit))
	}
	if offset > 0 {
		values.Set("offset", strconv.Itoa(offset))
	}
	if appendMode {
		values.Set("append", "1")
	}
	encoded := values.Encode()
	if encoded == "" {
		return ""
	}
	return "?" + encoded
}

func folderTitle(folder string) string {
	if folder == "" {
		return "All Media"
	}
	return filepath.Base(folder)
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func formatMB(mb int64) string {
	if mb >= 1024 {
		return fmt.Sprintf("%.1f GB", float64(mb)/1024.0)
	}
	return fmt.Sprintf("%d MB", mb)
}

func formatDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format("Jan 2, 2006 15:04")
}

func formatDuration(seconds float64) string {
	if seconds <= 0 {
		return ""
	}
	total := int(seconds + 0.5)
	hours := total / 3600
	minutes := (total % 3600) / 60
	secs := total % 60
	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, secs)
	}
	return fmt.Sprintf("%d:%02d", minutes, secs)
}

func mediaTypeName(mediaType string) string {
	switch mediaType {
	case MediaTypePhoto:
		return "Photo"
	case MediaTypeVideo:
		return "Video"
	default:
		return "Media"
	}
}

func videoContentType(ext string) string {
	switch strings.ToLower(ext) {
	case ".mp4", ".m4v":
		return "video/mp4"
	case ".mov":
		return "video/quicktime"
	case ".webm":
		return "video/webm"
	case ".mkv":
		return "video/x-matroska"
	case ".avi":
		return "video/x-msvideo"
	case ".wmv":
		return "video/x-ms-wmv"
	case ".flv":
		return "video/x-flv"
	default:
		return "application/octet-stream"
	}
}
