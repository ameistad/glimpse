package main

import (
	"encoding/json"
	"os"
	"time"
)

type Config struct {
	OriginalsPath  string        `json:"originals_path"`
	ThumbnailsPath string        `json:"thumbnails_path"`
	DatabasePath   string        `json:"database_path"`
	ListenAddr     string        `json:"listen_addr"`
	ScanInterval   time.Duration `json:"scan_interval"`
	ThumbnailSize  int           `json:"thumbnail_size"`
	RawExtensions   []string      `json:"raw_extensions"`
	APIKey          string        `json:"api_key"`
	VideoExtensions []string      `json:"video_extensions"`
}

type configJSON struct {
	OriginalsPath   string   `json:"originals_path"`
	ThumbnailsPath  string   `json:"thumbnails_path"`
	DatabasePath    string   `json:"database_path"`
	ListenAddr      string   `json:"listen_addr"`
	ScanIntervalSec int      `json:"scan_interval_seconds"`
	ThumbnailSize   int      `json:"thumbnail_size"`
	RawExtensions   []string `json:"raw_extensions"`
	APIKey          string   `json:"api_key"`
	VideoExtensions []string `json:"video_extensions"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return default config if file doesn't exist
			return DefaultConfig(), nil
		}
		return nil, err
	}

	var cj configJSON
	if err := json.Unmarshal(data, &cj); err != nil {
		return nil, err
	}

	cfg := &Config{
		OriginalsPath:   cj.OriginalsPath,
		ThumbnailsPath:  cj.ThumbnailsPath,
		DatabasePath:    cj.DatabasePath,
		ListenAddr:      cj.ListenAddr,
		ScanInterval:    time.Duration(cj.ScanIntervalSec) * time.Second,
		ThumbnailSize:   cj.ThumbnailSize,
		RawExtensions:   cj.RawExtensions,
		APIKey:          cj.APIKey,
		VideoExtensions: cj.VideoExtensions,
	}

	// Apply defaults for empty values
	if cfg.OriginalsPath == "" {
		cfg.OriginalsPath = "/pool/photos/originals"
	}
	if cfg.ThumbnailsPath == "" {
		cfg.ThumbnailsPath = "/pool/thumbnails"
	}
	if cfg.DatabasePath == "" {
		cfg.DatabasePath = "/pool/thumbnails/glimpse.db"
	}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = ":8080"
	}
	if cfg.ScanInterval == 0 {
		cfg.ScanInterval = 1 * time.Hour
	}
	if cfg.ThumbnailSize == 0 {
		cfg.ThumbnailSize = 800
	}
	if len(cfg.RawExtensions) == 0 {
		cfg.RawExtensions = DefaultRawExtensions()
	}
	if len(cfg.VideoExtensions) == 0 {
		cfg.VideoExtensions = DefaultVideoExtensions()
	}

	return cfg, nil
}

func DefaultConfig() *Config {
	return &Config{
		OriginalsPath:   "/pool/photos/originals",
		ThumbnailsPath:  "/pool/thumbnails",
		DatabasePath:    "/pool/thumbnails/glimpse.db",
		ListenAddr:      ":8080",
		ScanInterval:    1 * time.Hour,
		ThumbnailSize:   800,
		RawExtensions:   DefaultRawExtensions(),
		VideoExtensions: DefaultVideoExtensions(),
	}
}

func DefaultRawExtensions() []string {
	return []string{
		".cr2",
		".cr3",
		".nef",
		".nrw",
		".arw",
		".srf",
		".sr2",
		".orf",
		".pef",
		".raf",
		".rw2",
		".dng",
		".raw",
		".rwl",
		".3fr",
		".fff",
		".iiq",
		".jpg",
		".jpeg",
		".png",
	}
}

func DefaultVideoExtensions() []string {
	return []string{
		".mp4",
		".mov",
		".mkv",
		".avi",
		".webm",
		".m4v",
		".wmv",
		".flv",
	}
}

func (c *Config) SaveExample(path string) error {
	cj := configJSON{
		OriginalsPath:   c.OriginalsPath,
		ThumbnailsPath:  c.ThumbnailsPath,
		DatabasePath:    c.DatabasePath,
		ListenAddr:      c.ListenAddr,
		ScanIntervalSec: int(c.ScanInterval.Seconds()),
		ThumbnailSize:   c.ThumbnailSize,
		RawExtensions:   c.RawExtensions,
		APIKey:          c.APIKey,
		VideoExtensions: c.VideoExtensions,
	}

	data, err := json.MarshalIndent(cj, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
