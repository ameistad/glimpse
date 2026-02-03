package main

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Scanner struct {
	cfg *Config
	db  *Database
}

func NewScanner(cfg *Config, db *Database) *Scanner {
	return &Scanner{cfg: cfg, db: db}
}

func (s *Scanner) Scan() error {
	// Ensure thumbnails directory exists
	if err := os.MkdirAll(s.cfg.ThumbnailsPath, 0755); err != nil {
		return fmt.Errorf("failed to create thumbnails directory: %w", err)
	}

	s.cleanup()

	// Walk the originals directory
	err := filepath.WalkDir(s.cfg.OriginalsPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Printf("Error accessing %s: %v", path, err)
			return nil // Continue despite errors
		}

		if d.IsDir() {
			return nil
		}

		// Check if it's a RAW file
		ext := strings.ToLower(filepath.Ext(path))
		if !s.isRawExtension(ext) {
			return nil
		}

		// Get file info
		info, err := d.Info()
		if err != nil {
			log.Printf("Error getting info for %s: %v", path, err)
			return nil
		}

		// Check if already processed with same mod time
		exists, err := s.db.PhotoExists(path, info.ModTime())
		if err != nil {
			log.Printf("Error checking existence for %s: %v", path, err)
			return nil
		}
		if exists {
			return nil
		}

		// Process the photo
		if err := s.processPhoto(path, info); err != nil {
			log.Printf("Error processing %s: %v", path, err)
		}

		return nil
	})

	return err
}

func (s *Scanner) cleanup() {
	paths, err := s.db.AllOriginalPaths()
	if err != nil {
		log.Printf("Error fetching paths for cleanup: %v", err)
		return
	}

	removed := 0
	for _, p := range paths {
		if _, err := os.Stat(p.OriginalPath); os.IsNotExist(err) {
			if err := s.db.DeletePhoto(p.OriginalPath); err != nil {
				log.Printf("Error removing db entry for %s: %v", p.OriginalPath, err)
				continue
			}
			os.Remove(p.ThumbnailPath)
			removed++
		}
	}

	if removed > 0 {
		log.Printf("Cleanup: removed %d orphaned entries", removed)
	}
}

func (s *Scanner) isRawExtension(ext string) bool {
	for _, rawExt := range s.cfg.RawExtensions {
		if ext == rawExt {
			return true
		}
	}
	return false
}

func (s *Scanner) processPhoto(path string, info fs.FileInfo) error {
	log.Printf("Processing: %s", path)

	// Calculate thumbnail path (mirror directory structure)
	relPath, err := filepath.Rel(s.cfg.OriginalsPath, path)
	if err != nil {
		return fmt.Errorf("failed to get relative path: %w", err)
	}

	// Change extension to .jpg for thumbnail
	thumbRelPath := strings.TrimSuffix(relPath, filepath.Ext(relPath)) + ".jpg"
	thumbPath := filepath.Join(s.cfg.ThumbnailsPath, thumbRelPath)

	// Create thumbnail directory if needed
	thumbDir := filepath.Dir(thumbPath)
	if err := os.MkdirAll(thumbDir, 0755); err != nil {
		return fmt.Errorf("failed to create thumbnail directory: %w", err)
	}

	// Generate thumbnail using dcraw + ImageMagick
	// dcraw extracts embedded JPEG preview or converts RAW
	// convert resizes to thumbnail size
	width, height, err := s.generateThumbnail(path, thumbPath)
	if err != nil {
		return fmt.Errorf("failed to generate thumbnail: %w", err)
	}

	// Calculate folder (relative to originals)
	folder := filepath.Dir(relPath)
	if folder == "." {
		folder = ""
	}

	// Store in database
	photo := &Photo{
		OriginalPath:  path,
		ThumbnailPath: thumbPath,
		Folder:        folder,
		Filename:      info.Name(),
		Extension:     strings.ToLower(filepath.Ext(path)),
		FileSize:      info.Size(),
		ModTime:       info.ModTime(),
		Width:         width,
		Height:        height,
	}

	return s.db.UpsertPhoto(photo)
}

func (s *Scanner) generateThumbnail(rawPath, thumbPath string) (width, height int, err error) {
	// First, try to extract embedded JPEG preview using dcraw -e
	// This is much faster than full RAW conversion
	previewPath := rawPath + ".thumb.jpg"

	// Try dcraw -e (extract embedded thumbnail/preview)
	cmd := exec.Command("dcraw", "-e", "-c", rawPath)
	previewData, err := cmd.Output()

	if err != nil || len(previewData) == 0 {
		// Fall back to full RAW conversion with dcraw
		log.Printf("No embedded preview, converting RAW for %s", rawPath)
		cmd = exec.Command("dcraw", "-c", "-w", "-h", rawPath) // -h for half-size (faster)
		previewData, err = cmd.Output()
		if err != nil {
			return 0, 0, fmt.Errorf("dcraw failed: %w", err)
		}
	}

	// Write preview to temp file
	tempFile, err := os.CreateTemp("", "glimpse-*.ppm")
	if err != nil {
		return 0, 0, fmt.Errorf("failed to create temp file: %w", err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)
	defer os.Remove(previewPath) // Clean up dcraw preview if it exists

	if _, err := tempFile.Write(previewData); err != nil {
		tempFile.Close()
		return 0, 0, fmt.Errorf("failed to write temp file: %w", err)
	}
	tempFile.Close()

	// Use ImageMagick to resize and convert to JPEG
	size := fmt.Sprintf("%dx%d>", s.cfg.ThumbnailSize, s.cfg.ThumbnailSize)
	cmd = exec.Command("convert", tempPath,
		"-resize", size,
		"-quality", "85",
		"-auto-orient",
		thumbPath,
	)
	if err := cmd.Run(); err != nil {
		return 0, 0, fmt.Errorf("convert failed: %w", err)
	}

	// Get dimensions of original RAW file
	cmd = exec.Command("dcraw", "-i", "-v", rawPath)
	output, err := cmd.Output()
	if err != nil {
		return 0, 0, nil
	}
	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "Image size:") {
			fmt.Sscanf(strings.TrimPrefix(line, "Image size:"), "%d x %d", &width, &height)
			break
		}
	}
	return width, height, nil
}
