package main

import (
	"database/sql"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Photo struct {
	ID            int64     `json:"id"`
	OriginalPath  string    `json:"original_path"`
	ThumbnailPath string    `json:"thumbnail_path"`
	Folder        string    `json:"folder"`
	Filename      string    `json:"filename"`
	Extension     string    `json:"extension"`
	FileSize      int64     `json:"file_size"`
	ModTime       time.Time `json:"mod_time"`
	Width         int       `json:"width,omitempty"`
	Height        int       `json:"height,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	MediaType     string    `json:"media_type"`
	Duration      float64   `json:"duration,omitempty"`
	VideoCodec    string    `json:"video_codec,omitempty"`
	AudioCodec    string    `json:"audio_codec,omitempty"`
	Framerate     float64   `json:"framerate,omitempty"`
}

type Folder struct {
	Path       string `json:"path"`
	PhotoCount int    `json:"photo_count"`
}

type Stats struct {
	TotalPhotos     int   `json:"total_photos"`
	TotalVideos     int   `json:"total_videos"`
	TotalFolders    int   `json:"total_folders"`
	TotalOriginalMB int64 `json:"total_original_mb"`
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
		return nil, err
	}

	d := &Database{db: db}
	if err := d.migrate(); err != nil {
		return nil, err
	}

	return d, nil
}

func (d *Database) Close() error {
	return d.db.Close()
}

func (d *Database) migrate() error {
	_, err := d.db.Exec(`
		CREATE TABLE IF NOT EXISTS photos (
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
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_photos_folder ON photos(folder);
		CREATE INDEX IF NOT EXISTS idx_photos_mod_time ON photos(mod_time);
		CREATE INDEX IF NOT EXISTS idx_photos_filename ON photos(filename);
	`)
	if err != nil {
		return err
	}

	for _, stmt := range []string{
		`ALTER TABLE photos ADD COLUMN media_type TEXT NOT NULL DEFAULT 'photo'`,
		`ALTER TABLE photos ADD COLUMN duration REAL DEFAULT 0`,
		`ALTER TABLE photos ADD COLUMN video_codec TEXT DEFAULT ''`,
		`ALTER TABLE photos ADD COLUMN audio_codec TEXT DEFAULT ''`,
		`ALTER TABLE photos ADD COLUMN framerate REAL DEFAULT 0`,
	} {
		d.db.Exec(stmt)
	}
	d.db.Exec(`CREATE INDEX IF NOT EXISTS idx_photos_media_type ON photos(media_type)`)

	return nil
}

func (d *Database) UpsertPhoto(p *Photo) error {
	_, err := d.db.Exec(`
		INSERT INTO photos (original_path, thumbnail_path, folder, filename, extension, file_size, mod_time, width, height, media_type, duration, video_codec, audio_codec, framerate)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(original_path) DO UPDATE SET
			thumbnail_path = excluded.thumbnail_path,
			file_size = excluded.file_size,
			mod_time = excluded.mod_time,
			width = excluded.width,
			height = excluded.height,
			media_type = excluded.media_type,
			duration = excluded.duration,
			video_codec = excluded.video_codec,
			audio_codec = excluded.audio_codec,
			framerate = excluded.framerate
	`, p.OriginalPath, p.ThumbnailPath, p.Folder, p.Filename, p.Extension, p.FileSize, p.ModTime, p.Width, p.Height, p.MediaType, p.Duration, p.VideoCodec, p.AudioCodec, p.Framerate)
	return err
}

const photoColumns = `id, original_path, thumbnail_path, folder, filename, extension, file_size, mod_time, width, height, created_at, media_type, duration, video_codec, audio_codec, framerate`

func scanPhoto(scanner interface{ Scan(...any) error }) (*Photo, error) {
	p := &Photo{}
	err := scanner.Scan(&p.ID, &p.OriginalPath, &p.ThumbnailPath, &p.Folder, &p.Filename, &p.Extension, &p.FileSize, &p.ModTime, &p.Width, &p.Height, &p.CreatedAt, &p.MediaType, &p.Duration, &p.VideoCodec, &p.AudioCodec, &p.Framerate)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (d *Database) GetPhotoByID(id int64) (*Photo, error) {
	return scanPhoto(d.db.QueryRow(`SELECT `+photoColumns+` FROM photos WHERE id = ?`, id))
}

func (d *Database) GetPhotoByPath(path string) (*Photo, error) {
	return scanPhoto(d.db.QueryRow(`SELECT `+photoColumns+` FROM photos WHERE original_path = ?`, path))
}

func (d *Database) ListPhotos(folder, mediaType string, limit, offset int) ([]*Photo, error) {
	query := `SELECT ` + photoColumns + ` FROM photos`
	var args []any
	var conditions []string

	if folder != "" {
		conditions = append(conditions, `(folder = ? OR folder LIKE ?)`)
		args = append(args, folder, folder+"/%")
	}
	if mediaType != "" {
		conditions = append(conditions, `media_type = ?`)
		args = append(args, mediaType)
	}

	if len(conditions) > 0 {
		query += " WHERE " + conditions[0]
		for _, c := range conditions[1:] {
			query += " AND " + c
		}
	}
	query += ` ORDER BY mod_time DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var photos []*Photo
	for rows.Next() {
		p, err := scanPhoto(rows)
		if err != nil {
			return nil, err
		}
		photos = append(photos, p)
	}

	return photos, rows.Err()
}

func (d *Database) ListFolders() ([]*Folder, error) {
	rows, err := d.db.Query(`
		SELECT folder, COUNT(*) as photo_count
		FROM photos
		GROUP BY folder
		ORDER BY folder
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var folders []*Folder
	for rows.Next() {
		f := &Folder{}
		if err := rows.Scan(&f.Path, &f.PhotoCount); err != nil {
			return nil, err
		}
		folders = append(folders, f)
	}

	return folders, rows.Err()
}

func (d *Database) GetStats() (*Stats, error) {
	s := &Stats{}

	err := d.db.QueryRow(`SELECT COUNT(*) FROM photos WHERE media_type = 'photo'`).Scan(&s.TotalPhotos)
	if err != nil {
		return nil, err
	}

	err = d.db.QueryRow(`SELECT COUNT(*) FROM photos WHERE media_type = 'video'`).Scan(&s.TotalVideos)
	if err != nil {
		return nil, err
	}

	err = d.db.QueryRow(`SELECT COUNT(DISTINCT folder) FROM photos`).Scan(&s.TotalFolders)
	if err != nil {
		return nil, err
	}

	err = d.db.QueryRow(`SELECT COALESCE(SUM(file_size), 0) / 1048576 FROM photos`).Scan(&s.TotalOriginalMB)
	if err != nil {
		return nil, err
	}

	return s, nil
}

func (d *Database) PhotoExists(path string, modTime time.Time) (bool, error) {
	var count int
	err := d.db.QueryRow(`
		SELECT COUNT(*) FROM photos WHERE original_path = ? AND mod_time = ?
	`, path, modTime).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (d *Database) DeletePhoto(path string) error {
	_, err := d.db.Exec(`DELETE FROM photos WHERE original_path = ?`, path)
	return err
}

func (d *Database) AllOriginalPaths() ([]struct{ OriginalPath, ThumbnailPath string }, error) {
	rows, err := d.db.Query(`SELECT original_path, thumbnail_path FROM photos`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var paths []struct{ OriginalPath, ThumbnailPath string }
	for rows.Next() {
		var p struct{ OriginalPath, ThumbnailPath string }
		if err := rows.Scan(&p.OriginalPath, &p.ThumbnailPath); err != nil {
			return nil, err
		}
		paths = append(paths, p)
	}
	return paths, rows.Err()
}
