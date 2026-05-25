package main

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func TestNewDatabaseRejectsOldPhotosSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "glimpse.db")
	raw, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`CREATE TABLE photos (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}

	db, err := NewDatabase(path)
	if err == nil {
		db.Close()
		t.Fatal("expected old photos schema to be rejected")
	}
	if !errors.Is(err, ErrOldPhotoSchema) {
		t.Fatalf("expected ErrOldPhotoSchema, got %v", err)
	}
}

func TestListFoldersUsesRecursiveCounts(t *testing.T) {
	db := newTestDatabase(t)
	defer db.Close()

	insertTestMediaItem(t, db, "2024/trip", "one.cr3", MediaTypePhoto)
	insertTestMediaItem(t, db, "2024/trip/day1", "two.cr3", MediaTypePhoto)
	insertTestMediaItem(t, db, "2023", "clip.mp4", MediaTypeVideo)

	folders, err := db.ListFolders()
	if err != nil {
		t.Fatal(err)
	}

	counts := map[string]int{}
	for _, folder := range folders {
		counts[folder.Path] = folder.MediaCount
	}

	assertCount(t, counts, "2024", 2)
	assertCount(t, counts, "2024/trip", 2)
	assertCount(t, counts, "2024/trip/day1", 1)
	assertCount(t, counts, "2023", 1)
}

func TestListMediaItemsSearchAndTypeFilter(t *testing.T) {
	db := newTestDatabase(t)
	defer db.Close()

	insertTestMediaItem(t, db, "2024/trip", "mountain.cr3", MediaTypePhoto)
	insertTestMediaItem(t, db, "2024/trip", "mountain.mov", MediaTypeVideo)
	insertTestMediaItem(t, db, "2023", "portrait.cr3", MediaTypePhoto)

	items, err := db.ListMediaItems(MediaFilter{Query: "mountain", MediaType: MediaTypeVideo, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Filename != "mountain.mov" {
		t.Fatalf("unexpected filtered items: %#v", items)
	}

	count, err := db.CountMediaItems(MediaFilter{Folder: "2024", Query: "mountain"})
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("expected recursive folder/search count 2, got %d", count)
	}
}

func TestMediaItemPlayableVideoUsesBrowserSafeContainerAndCodecs(t *testing.T) {
	tests := []struct {
		name string
		item MediaItem
		want bool
	}{
		{
			name: "mp4 without probed codecs falls back to playable",
			item: MediaItem{MediaType: MediaTypeVideo, Extension: ".mp4"},
			want: true,
		},
		{
			name: "h264 aac mp4 is playable",
			item: MediaItem{MediaType: MediaTypeVideo, Extension: ".mp4", VideoCodec: "h264", AudioCodec: "aac"},
			want: true,
		},
		{
			name: "hevc mp4 is not treated as broadly playable",
			item: MediaItem{MediaType: MediaTypeVideo, Extension: ".mp4", VideoCodec: "hevc", AudioCodec: "aac"},
			want: false,
		},
		{
			name: "pcm audio blocks mp4 playback",
			item: MediaItem{MediaType: MediaTypeVideo, Extension: ".mp4", VideoCodec: "h264", AudioCodec: "pcm_s16le"},
			want: false,
		},
		{
			name: "mov is not treated as browser playable",
			item: MediaItem{MediaType: MediaTypeVideo, Extension: ".mov", VideoCodec: "h264", AudioCodec: "aac"},
			want: false,
		},
		{
			name: "vp9 opus webm is playable",
			item: MediaItem{MediaType: MediaTypeVideo, Extension: ".webm", VideoCodec: "vp9", AudioCodec: "opus"},
			want: true,
		},
		{
			name: "photo is not playable video",
			item: MediaItem{MediaType: MediaTypePhoto, Extension: ".mp4", VideoCodec: "h264", AudioCodec: "aac"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.item.IsPlayableVideo(); got != tt.want {
				t.Fatalf("IsPlayableVideo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func newTestDatabase(t *testing.T) *Database {
	t.Helper()
	db, err := NewDatabase(filepath.Join(t.TempDir(), "glimpse.db"))
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func insertTestMediaItem(t *testing.T, db *Database, folder, filename, mediaType string) {
	t.Helper()
	ext := filepath.Ext(filename)
	err := db.UpsertMediaItem(&MediaItem{
		OriginalPath:  filepath.Join("/originals", folder, filename),
		ThumbnailPath: filepath.Join("/thumbs", folder, filename+".jpg"),
		Folder:        folder,
		Filename:      filename,
		Extension:     ext,
		FileSize:      42,
		ModTime:       time.Now().UTC(),
		Width:         100,
		Height:        80,
		MediaType:     mediaType,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func assertCount(t *testing.T, counts map[string]int, path string, want int) {
	t.Helper()
	if got := counts[path]; got != want {
		t.Fatalf("folder %q count = %d, want %d; all counts: %#v", path, got, want, counts)
	}
}
