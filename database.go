package main

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const (
	MediaTypePhoto = "photo"
	MediaTypeVideo = "video"
)

var ErrOldPhotoSchema = errors.New("old photos schema detected")

type MediaItem struct {
	ID            int64
	OriginalPath  string
	ThumbnailPath string
	Folder        string
	Filename      string
	Extension     string
	FileSize      int64
	ModTime       time.Time
	Width         int
	Height        int
	CreatedAt     time.Time
	MediaType     string
	Duration      float64
	VideoCodec    string
	AudioCodec    string
	Framerate     float64
}

func (m *MediaItem) IsVideo() bool {
	return m.MediaType == MediaTypeVideo
}

func (m *MediaItem) IsPlayableVideo() bool {
	if !m.IsVideo() {
		return false
	}
	switch strings.ToLower(m.Extension) {
	case ".mp4", ".m4v", ".mov", ".webm":
		return true
	default:
		return false
	}
}

type Folder struct {
	Path       string
	MediaCount int
}

func (f Folder) Name() string {
	if f.Path == "" {
		return "Root"
	}
	return filepath.Base(f.Path)
}

type Stats struct {
	TotalPhotos     int
	TotalVideos     int
	TotalFolders    int
	TotalOriginalMB int64
}

type MediaFilter struct {
	Folder    string
	MediaType string
	Query     string
	Limit     int
	Offset    int
}

type StoredPath struct {
	OriginalPath  string
	ThumbnailPath string
}

type Database struct {
	db *sql.DB
}

func NewDatabase(path string) (*Database, error) {
	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}

	d := &Database{db: db}
	if err := d.migrate(); err != nil {
		db.Close()
		return nil, err
	}

	return d, nil
}

func (d *Database) Close() error {
	return d.db.Close()
}

func (d *Database) migrate() error {
	hasOldPhotos, err := d.tableExists("photos")
	if err != nil {
		return err
	}
	if hasOldPhotos {
		return fmt.Errorf("%w: delete the existing database and restart Glimpse to rebuild the media_items schema", ErrOldPhotoSchema)
	}

	_, err = d.db.Exec(`
		CREATE TABLE IF NOT EXISTS media_items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			original_path TEXT UNIQUE NOT NULL,
			thumbnail_path TEXT NOT NULL,
			folder TEXT NOT NULL,
			filename TEXT NOT NULL,
			extension TEXT NOT NULL,
			file_size INTEGER NOT NULL,
			mod_time DATETIME NOT NULL,
			width INTEGER DEFAULT 0,
			height INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			media_type TEXT NOT NULL DEFAULT 'photo',
			duration REAL DEFAULT 0,
			video_codec TEXT DEFAULT '',
			audio_codec TEXT DEFAULT '',
			framerate REAL DEFAULT 0
		);

		CREATE INDEX IF NOT EXISTS idx_media_items_folder ON media_items(folder);
		CREATE INDEX IF NOT EXISTS idx_media_items_mod_time ON media_items(mod_time);
		CREATE INDEX IF NOT EXISTS idx_media_items_filename ON media_items(filename);
		CREATE INDEX IF NOT EXISTS idx_media_items_media_type ON media_items(media_type);
	`)
	return err
}

func (d *Database) tableExists(name string) (bool, error) {
	var count int
	err := d.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&count)
	return count > 0, err
}

func (d *Database) UpsertMediaItem(item *MediaItem) error {
	_, err := d.db.Exec(`
		INSERT INTO media_items (original_path, thumbnail_path, folder, filename, extension, file_size, mod_time, width, height, media_type, duration, video_codec, audio_codec, framerate)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(original_path) DO UPDATE SET
			thumbnail_path = excluded.thumbnail_path,
			folder = excluded.folder,
			filename = excluded.filename,
			extension = excluded.extension,
			file_size = excluded.file_size,
			mod_time = excluded.mod_time,
			width = excluded.width,
			height = excluded.height,
			media_type = excluded.media_type,
			duration = excluded.duration,
			video_codec = excluded.video_codec,
			audio_codec = excluded.audio_codec,
			framerate = excluded.framerate
	`, item.OriginalPath, item.ThumbnailPath, item.Folder, item.Filename, item.Extension, item.FileSize, item.ModTime, item.Width, item.Height, item.MediaType, item.Duration, item.VideoCodec, item.AudioCodec, item.Framerate)
	return err
}

const mediaItemColumns = `id, original_path, thumbnail_path, folder, filename, extension, file_size, mod_time, width, height, created_at, media_type, duration, video_codec, audio_codec, framerate`

func scanMediaItem(scanner interface{ Scan(...any) error }) (*MediaItem, error) {
	item := &MediaItem{}
	err := scanner.Scan(&item.ID, &item.OriginalPath, &item.ThumbnailPath, &item.Folder, &item.Filename, &item.Extension, &item.FileSize, &item.ModTime, &item.Width, &item.Height, &item.CreatedAt, &item.MediaType, &item.Duration, &item.VideoCodec, &item.AudioCodec, &item.Framerate)
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (d *Database) GetMediaItemByID(id int64) (*MediaItem, error) {
	return scanMediaItem(d.db.QueryRow(`SELECT `+mediaItemColumns+` FROM media_items WHERE id = ?`, id))
}

func (d *Database) ListMediaItems(filter MediaFilter) ([]*MediaItem, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 80
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	query := `SELECT ` + mediaItemColumns + ` FROM media_items`
	where, args := mediaWhere(filter)
	if where != "" {
		query += " WHERE " + where
	}
	query += ` ORDER BY mod_time DESC, id DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]*MediaItem, 0, limit)
	for rows.Next() {
		item, err := scanMediaItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (d *Database) CountMediaItems(filter MediaFilter) (int, error) {
	query := `SELECT COUNT(*) FROM media_items`
	where, args := mediaWhere(filter)
	if where != "" {
		query += " WHERE " + where
	}
	var count int
	err := d.db.QueryRow(query, args...).Scan(&count)
	return count, err
}

func mediaWhere(filter MediaFilter) (string, []any) {
	var conditions []string
	var args []any

	if filter.Folder != "" {
		conditions = append(conditions, `(folder = ? OR folder LIKE ?)`)
		args = append(args, filter.Folder, filter.Folder+"/%")
	}
	if filter.MediaType == MediaTypePhoto || filter.MediaType == MediaTypeVideo {
		conditions = append(conditions, `media_type = ?`)
		args = append(args, filter.MediaType)
	}
	if q := strings.TrimSpace(filter.Query); q != "" {
		conditions = append(conditions, `(filename LIKE ? OR folder LIKE ?)`)
		pattern := "%" + q + "%"
		args = append(args, pattern, pattern)
	}

	return strings.Join(conditions, " AND "), args
}

func (d *Database) ListFolders() ([]*Folder, error) {
	rows, err := d.db.Query(`
		SELECT folder, COUNT(*)
		FROM media_items
		WHERE folder <> ''
		GROUP BY folder
		ORDER BY folder
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := map[string]int{}
	for rows.Next() {
		var path string
		var count int
		if err := rows.Scan(&path, &count); err != nil {
			return nil, err
		}
		parts := strings.Split(path, "/")
		for i := range parts {
			ancestor := strings.Join(parts[:i+1], "/")
			counts[ancestor] += count
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	folders := make([]*Folder, 0, len(counts))
	for path, count := range counts {
		folders = append(folders, &Folder{Path: path, MediaCount: count})
	}
	sort.Slice(folders, func(i, j int) bool {
		return folders[i].Path < folders[j].Path
	})
	return folders, nil
}

func (d *Database) GetStats() (*Stats, error) {
	stats := &Stats{}

	if err := d.db.QueryRow(`SELECT COUNT(*) FROM media_items WHERE media_type = 'photo'`).Scan(&stats.TotalPhotos); err != nil {
		return nil, err
	}
	if err := d.db.QueryRow(`SELECT COUNT(*) FROM media_items WHERE media_type = 'video'`).Scan(&stats.TotalVideos); err != nil {
		return nil, err
	}
	if err := d.db.QueryRow(`SELECT COALESCE(SUM(file_size), 0) / 1048576 FROM media_items`).Scan(&stats.TotalOriginalMB); err != nil {
		return nil, err
	}

	folders, err := d.ListFolders()
	if err != nil {
		return nil, err
	}
	stats.TotalFolders = len(folders)

	return stats, nil
}

func (d *Database) MediaItemExists(path string, modTime time.Time) (bool, error) {
	var count int
	err := d.db.QueryRow(`
		SELECT COUNT(*) FROM media_items WHERE original_path = ? AND mod_time = ?
	`, path, modTime).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (d *Database) DeleteMediaItem(path string) error {
	_, err := d.db.Exec(`DELETE FROM media_items WHERE original_path = ?`, path)
	return err
}

func (d *Database) AllOriginalPaths() ([]StoredPath, error) {
	rows, err := d.db.Query(`SELECT original_path, thumbnail_path FROM media_items`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var paths []StoredPath
	for rows.Next() {
		var path StoredPath
		if err := rows.Scan(&path.OriginalPath, &path.ThumbnailPath); err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return paths, rows.Err()
}
