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

		name := d.Name()
		if strings.HasPrefix(name, "._") || strings.HasPrefix(name, ".") {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if !s.isSupportedExtension(ext) {
			return nil
		}

		if isStandardImage(ext) && s.hasRawCompanion(path) {
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

func (s *Scanner) isSupportedExtension(ext string) bool {
	if isStandardImage(ext) {
		return true
	}
	for _, supported := range s.cfg.RawExtensions {
		if ext == supported {
			return true
		}
	}
	return false
}

func (s *Scanner) hasRawCompanion(imgPath string) bool {
	base := strings.TrimSuffix(imgPath, filepath.Ext(imgPath))
	for _, rawExt := range s.cfg.RawExtensions {
		if isStandardImage(rawExt) {
			continue
		}
		candidates := []string{base + rawExt, base + strings.ToUpper(rawExt)}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				return true
			}
		}
	}
	return false
}

func isStandardImage(ext string) bool {
	switch ext {
	case ".jpg", ".jpeg", ".png", ".tif", ".tiff":
		return true
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
	ext := strings.ToLower(filepath.Ext(rawPath))

	if isStandardImage(ext) {
		return s.generateStandardThumbnail(rawPath, thumbPath)
	}
	return s.generateRawThumbnail(rawPath, thumbPath)
}

func (s *Scanner) generateStandardThumbnail(imgPath, thumbPath string) (width, height int, err error) {
	size := fmt.Sprintf("%dx%d>", s.cfg.ThumbnailSize, s.cfg.ThumbnailSize)
	cmd := exec.Command("convert", imgPath+"[0]",
		"-resize", size,
		"-quality", "85",
		"-auto-orient",
		thumbPath,
	)
	if err := cmd.Run(); err != nil {
		return 0, 0, fmt.Errorf("convert failed: %w", err)
	}

	cmd = exec.Command("identify", "-format", "%w %h", imgPath+"[0]")
	output, err := cmd.Output()
	if err != nil {
		return 0, 0, nil
	}
	fmt.Sscanf(string(output), "%d %d", &width, &height)
	return width, height, nil
}

func (s *Scanner) generateRawThumbnail(rawPath, thumbPath string) (width, height int, err error) {
	previewPath := rawPath + ".thumb.jpg"

	cmd := exec.Command("dcraw", "-e", "-c", rawPath)
	previewData, err := cmd.Output()

	if err != nil || len(previewData) == 0 {
		log.Printf("No embedded preview, converting RAW for %s", rawPath)
		cmd = exec.Command("dcraw", "-c", "-w", "-h", rawPath)
		previewData, err = cmd.Output()
		if err != nil {
			return 0, 0, fmt.Errorf("dcraw failed: %w", err)
		}
	}

	tempFile, err := os.CreateTemp("", "glimpse-*.ppm")
	if err != nil {
		return 0, 0, fmt.Errorf("failed to create temp file: %w", err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)
	defer os.Remove(previewPath)

	if _, err := tempFile.Write(previewData); err != nil {
		tempFile.Close()
		return 0, 0, fmt.Errorf("failed to write temp file: %w", err)
	}
	tempFile.Close()

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
