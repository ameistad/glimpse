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
}

type Folder struct {
	Path       string `json:"path"`
	PhotoCount int    `json:"photo_count"`
}

type Stats struct {
	TotalPhotos     int   `json:"total_photos"`
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
	return err
}

func (d *Database) UpsertPhoto(p *Photo) error {
	_, err := d.db.Exec(`
		INSERT INTO photos (original_path, thumbnail_path, folder, filename, extension, file_size, mod_time, width, height)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(original_path) DO UPDATE SET
			thumbnail_path = excluded.thumbnail_path,
			file_size = excluded.file_size,
			mod_time = excluded.mod_time,
			width = excluded.width,
			height = excluded.height
	`, p.OriginalPath, p.ThumbnailPath, p.Folder, p.Filename, p.Extension, p.FileSize, p.ModTime, p.Width, p.Height)
	return err
}

func (d *Database) GetPhotoByID(id int64) (*Photo, error) {
	p := &Photo{}
	err := d.db.QueryRow(`
		SELECT id, original_path, thumbnail_path, folder, filename, extension, file_size, mod_time, width, height, created_at
		FROM photos WHERE id = ?
	`, id).Scan(&p.ID, &p.OriginalPath, &p.ThumbnailPath, &p.Folder, &p.Filename, &p.Extension, &p.FileSize, &p.ModTime, &p.Width, &p.Height, &p.CreatedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (d *Database) GetPhotoByPath(path string) (*Photo, error) {
	p := &Photo{}
	err := d.db.QueryRow(`
		SELECT id, original_path, thumbnail_path, folder, filename, extension, file_size, mod_time, width, height, created_at
		FROM photos WHERE original_path = ?
	`, path).Scan(&p.ID, &p.OriginalPath, &p.ThumbnailPath, &p.Folder, &p.Filename, &p.Extension, &p.FileSize, &p.ModTime, &p.Width, &p.Height, &p.CreatedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (d *Database) ListPhotos(folder string, limit, offset int) ([]*Photo, error) {
	var rows *sql.Rows
	var err error

	if folder != "" {
		rows, err = d.db.Query(`
			SELECT id, original_path, thumbnail_path, folder, filename, extension, file_size, mod_time, width, height, created_at
			FROM photos
			WHERE folder = ? OR folder LIKE ?
			ORDER BY mod_time DESC
			LIMIT ? OFFSET ?
		`, folder, folder+"/%", limit, offset)
	} else {
		rows, err = d.db.Query(`
			SELECT id, original_path, thumbnail_path, folder, filename, extension, file_size, mod_time, width, height, created_at
			FROM photos
			ORDER BY mod_time DESC
			LIMIT ? OFFSET ?
		`, limit, offset)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var photos []*Photo
	for rows.Next() {
		p := &Photo{}
		if err := rows.Scan(&p.ID, &p.OriginalPath, &p.ThumbnailPath, &p.Folder, &p.Filename, &p.Extension, &p.FileSize, &p.ModTime, &p.Width, &p.Height, &p.CreatedAt); err != nil {
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

	err := d.db.QueryRow(`SELECT COUNT(*) FROM photos`).Scan(&s.TotalPhotos)
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
