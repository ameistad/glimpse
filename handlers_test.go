package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
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
