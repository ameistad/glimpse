package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGenerateRawThumbnailUsesThumbnailTreeForTempFiles(t *testing.T) {
	originalsDir := t.TempDir()
	thumbnailsDir := t.TempDir()
	rawPath := filepath.Join(originalsDir, "image.cr3")
	sentinelPath := rawPath + ".thumb.jpg"

	if err := os.WriteFile(rawPath, []byte("raw"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sentinelPath, []byte("sentinel"), 0644); err != nil {
		t.Fatal(err)
	}

	binDir := t.TempDir()
	convertInputLog := filepath.Join(binDir, "convert-input")
	writeExecutable(t, filepath.Join(binDir, "dcraw"), `#!/bin/bash
set -euo pipefail

if [ "${1:-}" = "-e" ]; then
  printf 'P3\n1 1\n255\n0 0 0\n'
elif [ "${1:-}" = "-i" ]; then
  printf 'Image size: 6000 x 4000\n'
else
  exit 1
fi
`)
	writeExecutable(t, filepath.Join(binDir, "convert"), `#!/bin/bash
set -euo pipefail

printf '%s\n' "$1" > "$FAKE_CONVERT_INPUT_LOG"
out="${!#}"
printf 'thumb' > "$out"
`)

	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_CONVERT_INPUT_LOG", convertInputLog)

	cfg := DefaultConfig()
	cfg.ThumbnailSize = 800
	scanner := NewScanner(cfg, nil)
	thumbPath := filepath.Join(thumbnailsDir, "image.jpg")

	width, height, err := scanner.generateRawThumbnail(rawPath, thumbPath)
	if err != nil {
		t.Fatal(err)
	}
	if width != 6000 || height != 4000 {
		t.Fatalf("raw dimensions = %dx%d, want 6000x4000", width, height)
	}

	if got, err := os.ReadFile(sentinelPath); err != nil || string(got) != "sentinel" {
		t.Fatalf("original-side sentinel changed: content=%q err=%v", got, err)
	}

	convertInput, err := os.ReadFile(convertInputLog)
	if err != nil {
		t.Fatal(err)
	}
	tempPath := strings.TrimSpace(string(convertInput))
	if !pathWithin(thumbnailsDir, tempPath) {
		t.Fatalf("RAW temp file = %q, want inside %q", tempPath, thumbnailsDir)
	}
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Fatalf("RAW temp file was not cleaned up: %v", err)
	}
}

func TestCleanupDoesNotDeleteThumbnailsOutsideConfiguredPath(t *testing.T) {
	db := newTestDatabase(t)
	defer db.Close()

	cfg := DefaultConfig()
	cfg.OriginalsPath = t.TempDir()
	cfg.ThumbnailsPath = t.TempDir()
	outsideDir := t.TempDir()
	outsideThumbnail := filepath.Join(outsideDir, "outside.jpg")
	if err := os.WriteFile(outsideThumbnail, []byte("outside"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := db.UpsertMediaItem(&MediaItem{
		OriginalPath:  filepath.Join(cfg.OriginalsPath, "missing.cr3"),
		ThumbnailPath: outsideThumbnail,
		Filename:      "missing.cr3",
		Extension:     ".cr3",
		FileSize:      42,
		ModTime:       time.Now().UTC(),
		Width:         100,
		Height:        80,
		MediaType:     MediaTypePhoto,
	}); err != nil {
		t.Fatal(err)
	}

	NewScanner(cfg, db).cleanup()

	if got, err := os.ReadFile(outsideThumbnail); err != nil || string(got) != "outside" {
		t.Fatalf("outside thumbnail changed: content=%q err=%v", got, err)
	}
	count, err := db.CountMediaItems(MediaFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("media item count = %d, want 0", count)
	}
}

func writeExecutable(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0755); err != nil {
		t.Fatal(err)
	}
}
