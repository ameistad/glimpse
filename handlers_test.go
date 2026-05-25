package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestOldAPIRoutesReturnNotFound(t *testing.T) {
	handler := newTestHandler(t, "")

	req := httptest.NewRequest(http.MethodGet, "/api/photos", nil)
	res := httptest.NewRecorder()
	handler.Routes().ServeHTTP(res, req)

	if res.Code != http.StatusNotFound {
		t.Fatalf("old API status = %d, want 404", res.Code)
	}
}

func TestMediaPageRendersWithoutAPIKey(t *testing.T) {
	handler := newTestHandler(t, "")

	req := httptest.NewRequest(http.MethodGet, "/media", nil)
	res := httptest.NewRecorder()
	handler.Routes().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("/media status = %d, want 200", res.Code)
	}
	if !strings.Contains(res.Body.String(), "All Media") {
		t.Fatalf("expected rendered media page, got %q", res.Body.String())
	}
	if got := res.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("media page Cache-Control = %q, want no-store", got)
	}
	if !strings.Contains(res.Body.String(), `/assets/styles.css?v=`) || !strings.Contains(res.Body.String(), `/assets/app.js?v=`) {
		t.Fatalf("expected cache-busted asset URLs, got %q", res.Body.String())
	}
}

func TestAuthRedirectAndLoginCookie(t *testing.T) {
	handler := newTestHandler(t, "secret")
	routes := handler.Routes()

	req := httptest.NewRequest(http.MethodGet, "/media", nil)
	req.Header.Set("Accept", "text/html")
	res := httptest.NewRecorder()
	routes.ServeHTTP(res, req)

	if res.Code != http.StatusFound {
		t.Fatalf("unauthenticated /media status = %d, want 302", res.Code)
	}
	if got := res.Header().Get("Location"); !strings.HasPrefix(got, "/login") {
		t.Fatalf("redirect location = %q, want /login", got)
	}

	form := url.Values{"api_key": {"secret"}, "next": {"/media"}}
	req = httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res = httptest.NewRecorder()
	routes.ServeHTTP(res, req)

	if res.Code != http.StatusFound {
		t.Fatalf("login status = %d, want 302", res.Code)
	}
	cookies := res.Result().Cookies()
	if len(cookies) == 0 || cookies[0].Name != sessionCookieName || cookies[0].Value == "" {
		t.Fatalf("expected session cookie, got %#v", cookies)
	}

	req = httptest.NewRequest(http.MethodGet, "/media", nil)
	req.AddCookie(cookies[0])
	res = httptest.NewRecorder()
	routes.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("authenticated /media status = %d, want 200", res.Code)
	}
}

func TestHTMXGridPartialPushesCanonicalMediaURL(t *testing.T) {
	handler := newTestHandler(t, "")
	insertTestMediaItem(t, handler.db, "2024/trip", "mountain.cr3", MediaTypePhoto)

	req := httptest.NewRequest(http.MethodGet, "/media/grid?folder=2024&q=mountain", nil)
	req.Header.Set("HX-Request", "true")
	res := httptest.NewRecorder()
	handler.Routes().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("grid status = %d, want 200", res.Code)
	}
	if got := res.Header().Get("HX-Push-Url"); got != "/media?folder=2024&q=mountain" {
		t.Fatalf("HX-Push-Url = %q", got)
	}
	if !strings.Contains(res.Body.String(), "mountain.cr3") {
		t.Fatalf("expected media card in grid partial, got %q", res.Body.String())
	}
}

func TestMediaGridInfiniteScrollUsesGridScrollContainer(t *testing.T) {
	handler := newTestHandler(t, "")
	for i := 0; i < defaultPageSize+1; i++ {
		insertTestMediaItem(t, handler.db, "batch", "item-"+strconv.Itoa(i)+".jpg", MediaTypePhoto)
	}

	req := httptest.NewRequest(http.MethodGet, "/media/grid?folder=batch", nil)
	req.Header.Set("HX-Request", "true")
	res := httptest.NewRecorder()
	handler.Routes().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("grid status = %d, want 200", res.Code)
	}
	body := res.Body.String()
	if !strings.Contains(body, `class="load-sentinel"`) {
		t.Fatalf("expected infinite scroll sentinel, got %q", body)
	}
	if !strings.Contains(body, `hx-get="/media/grid?append=1&amp;folder=batch&amp;offset=80"`) {
		t.Fatalf("expected next page URL with append offset, got %q", body)
	}
	if !strings.Contains(body, `hx-trigger="intersect once root:#grid-region threshold:0.1"`) {
		t.Fatalf("expected sentinel to observe the grid scroll container, got %q", body)
	}
}

func TestMediaPageRendersReactiveMediaTypeToolbar(t *testing.T) {
	handler := newTestHandler(t, "")

	req := httptest.NewRequest(http.MethodGet, "/media?media_type=video", nil)
	res := httptest.NewRecorder()
	handler.Routes().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("/media status = %d, want 200", res.Code)
	}
	body := res.Body.String()
	if !strings.Contains(body, `name="media_type" value="video" x-ref="mediaTypeInput"`) {
		t.Fatalf("expected media type hidden input to carry current filter, got %q", body)
	}
	if strings.Count(body, `name="media_type"`) != 1 {
		t.Fatalf("expected only the hidden media_type field to submit the filter, got %q", body)
	}
	if !strings.Contains(body, `setMediaType('video')`) {
		t.Fatalf("expected toolbar button to update media type client state, got %q", body)
	}
	if !strings.Contains(body, `>Videos</button>`) || !strings.Contains(body, `aria-pressed="true" :aria-pressed="(activeMediaType === 'video').toString()">Videos`) {
		t.Fatalf("expected video toolbar button to render active aria state, got %q", body)
	}
}

func TestMediaDetailShowsOriginalPathAndPlayableVideoSource(t *testing.T) {
	handler := newTestHandler(t, "")
	insertTestMediaItem(t, handler.db, "2024/trip", "clip.mp4", MediaTypeVideo)
	item := firstMediaItem(t, handler.db)

	req := httptest.NewRequest(http.MethodGet, "/media/"+strconv.FormatInt(item.ID, 10), nil)
	res := httptest.NewRecorder()
	handler.Routes().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("detail status = %d, want 200", res.Code)
	}
	body := res.Body.String()
	if !strings.Contains(body, item.OriginalPath) {
		t.Fatalf("expected detail page to include original path %q, got %q", item.OriginalPath, body)
	}
	if !strings.Contains(body, `<source src="/media/`+strconv.FormatInt(item.ID, 10)+`/stream" type="video/mp4">`) {
		t.Fatalf("expected playable video source with content type, got %q", body)
	}
	if !strings.Contains(body, "playsinline") {
		t.Fatalf("expected inline video playback attribute, got %q", body)
	}
}

func TestMediaDetailShowsFullWidthPreviewForPhotos(t *testing.T) {
	handler := newTestHandler(t, "")
	insertTestMediaItem(t, handler.db, "2024/trip", "portrait.jpg", MediaTypePhoto)
	item := firstMediaItem(t, handler.db)

	req := httptest.NewRequest(http.MethodGet, "/media/"+strconv.FormatInt(item.ID, 10), nil)
	res := httptest.NewRecorder()
	handler.Routes().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("detail status = %d, want 200", res.Code)
	}
	body := res.Body.String()
	if !strings.Contains(body, `class="detail-full-button"`) {
		t.Fatalf("expected full-width preview button, got %q", body)
	}
	if !strings.Contains(body, `data-full-src="/media/`+strconv.FormatInt(item.ID, 10)+`/thumbnail"`) {
		t.Fatalf("expected full-width preview thumbnail source, got %q", body)
	}
	if !strings.Contains(body, `class="full-preview"`) {
		t.Fatalf("expected full-width preview overlay, got %q", body)
	}
	if !strings.Contains(body, `openFullPreview($event.currentTarget.dataset.fullSrc, $event.currentTarget.dataset.fullAlt)`) {
		t.Fatalf("expected full-width preview click handler, got %q", body)
	}
}

func TestMediaDetailShowsUnsupportedVideoForBrowserHostileCodec(t *testing.T) {
	handler := newTestHandler(t, "")
	if err := handler.db.UpsertMediaItem(&MediaItem{
		OriginalPath:  filepath.Join(handler.cfg.OriginalsPath, "clip.mov"),
		ThumbnailPath: filepath.Join(handler.cfg.ThumbnailsPath, "clip.jpg"),
		Filename:      "clip.mov",
		Extension:     ".mov",
		FileSize:      42,
		ModTime:       time.Now().UTC(),
		MediaType:     MediaTypeVideo,
		VideoCodec:    "hevc",
		AudioCodec:    "pcm_s16le",
	}); err != nil {
		t.Fatal(err)
	}
	item := firstMediaItem(t, handler.db)

	req := httptest.NewRequest(http.MethodGet, "/media/"+strconv.FormatInt(item.ID, 10), nil)
	res := httptest.NewRecorder()
	handler.Routes().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("detail status = %d, want 200", res.Code)
	}
	body := res.Body.String()
	if strings.Contains(body, "<video controls") {
		t.Fatalf("expected unsupported video to avoid inline player, got %q", body)
	}
	if !strings.Contains(body, "Download to play this video") {
		t.Fatalf("expected unsupported video message, got %q", body)
	}
	if !strings.Contains(body, `<span class="video-badge">Video</span>`) {
		t.Fatalf("expected unsupported video grid badge, got %q", body)
	}
}

func TestStreamVideoSupportsInlineRangeRequests(t *testing.T) {
	handler := newTestHandler(t, "")
	videoPath := filepath.Join(handler.cfg.OriginalsPath, "clip.mp4")
	if err := os.WriteFile(videoPath, []byte("0123456789"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := handler.db.UpsertMediaItem(&MediaItem{
		OriginalPath:  videoPath,
		ThumbnailPath: filepath.Join(handler.cfg.ThumbnailsPath, "clip.jpg"),
		Filename:      "clip.mp4",
		Extension:     ".mp4",
		FileSize:      10,
		ModTime:       time.Now().UTC(),
		MediaType:     MediaTypeVideo,
	}); err != nil {
		t.Fatal(err)
	}
	item := firstMediaItem(t, handler.db)

	req := httptest.NewRequest(http.MethodGet, "/media/"+strconv.FormatInt(item.ID, 10)+"/stream", nil)
	req.Header.Set("Range", "bytes=0-3")
	res := httptest.NewRecorder()
	handler.Routes().ServeHTTP(res, req)

	if res.Code != http.StatusPartialContent {
		t.Fatalf("stream status = %d, want 206", res.Code)
	}
	if got := res.Header().Get("Content-Type"); got != "video/mp4" {
		t.Fatalf("Content-Type = %q, want video/mp4", got)
	}
	if got := res.Header().Get("Accept-Ranges"); got != "bytes" {
		t.Fatalf("Accept-Ranges = %q, want bytes", got)
	}
	if got := res.Header().Get("Content-Disposition"); !strings.HasPrefix(got, "inline;") {
		t.Fatalf("Content-Disposition = %q, want inline", got)
	}
	if got := res.Header().Get("Content-Range"); got != "bytes 0-3/10" {
		t.Fatalf("Content-Range = %q, want bytes 0-3/10", got)
	}
	if got := res.Body.String(); got != "0123" {
		t.Fatalf("range body = %q, want 0123", got)
	}
}

func TestDevelopmentReloadEndpointIsDevOnlyAndAuthExempt(t *testing.T) {
	handler := newTestHandler(t, "")

	req := httptest.NewRequest(http.MethodGet, "/__dev/reload-version", nil)
	res := httptest.NewRecorder()
	handler.Routes().ServeHTTP(res, req)

	if res.Code != http.StatusNotFound {
		t.Fatalf("production reload endpoint status = %d, want 404", res.Code)
	}

	handler = newTestHandlerWithDevelopment(t, "secret", true)
	req = httptest.NewRequest(http.MethodGet, "/__dev/reload-version", nil)
	res = httptest.NewRecorder()
	handler.Routes().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("development reload endpoint status = %d, want 200", res.Code)
	}
	if got := res.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
	if !strings.Contains(res.Body.String(), "version") {
		t.Fatalf("expected version JSON, got %q", res.Body.String())
	}
}

func TestDevelopmentPageInjectsReloadScriptAndDisablesAssetCache(t *testing.T) {
	handler := newTestHandlerWithDevelopment(t, "", true)

	req := httptest.NewRequest(http.MethodGet, "/media", nil)
	res := httptest.NewRecorder()
	handler.Routes().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("/media status = %d, want 200", res.Code)
	}
	if !strings.Contains(res.Body.String(), "/__dev/reload-version") {
		t.Fatalf("expected development reload script in page, got %q", res.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/assets/styles.css", nil)
	res = httptest.NewRecorder()
	handler.Routes().ServeHTTP(res, req)

	if got := res.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("development asset Cache-Control = %q, want no-store", got)
	}
}

func newTestHandler(t *testing.T, apiKey string) *Handler {
	return newTestHandlerWithDevelopment(t, apiKey, false)
}

func newTestHandlerWithDevelopment(t *testing.T, apiKey string, development bool) *Handler {
	t.Helper()
	db := newTestDatabase(t)
	t.Cleanup(func() {
		db.Close()
	})

	cfg := DefaultConfig()
	cfg.APIKey = apiKey
	cfg.Development = development
	cfg.OriginalsPath = t.TempDir()
	cfg.ThumbnailsPath = t.TempDir()
	cfg.DatabasePath = ""
	cfg.ListenAddr = "127.0.0.1:0"

	handler, err := NewHandler(cfg, db, NewScanner(cfg, db))
	if err != nil {
		t.Fatal(err)
	}
	return handler
}

func firstMediaItem(t *testing.T, db *Database) *MediaItem {
	t.Helper()
	items, err := db.ListMediaItems(MediaFilter{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) == 0 {
		t.Fatal("expected at least one media item")
	}
	return items[0]
}
