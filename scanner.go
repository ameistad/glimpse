package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
)

var ErrScanAlreadyRunning = errors.New("scan already running")

type Scanner struct {
	cfg      *Config
	db       *Database
	scanning atomic.Bool
}

func NewScanner(cfg *Config, db *Database) *Scanner {
	return &Scanner{cfg: cfg, db: db}
}

func (s *Scanner) IsScanning() bool {
	return s.scanning.Load()
}

func (s *Scanner) TryScan() bool {
	if !s.scanning.CompareAndSwap(false, true) {
		return false
	}
	go func() {
		defer s.scanning.Store(false)
		log.Println("Starting on-demand scan...")
		if err := s.scan(); err != nil {
			log.Printf("On-demand scan error: %v", err)
		}
		log.Println("On-demand scan complete")
	}()
	return true
}

func (s *Scanner) Scan() error {
	if !s.scanning.CompareAndSwap(false, true) {
		return ErrScanAlreadyRunning
	}
	defer s.scanning.Store(false)
	return s.scan()
}

func (s *Scanner) scan() error {
	if err := os.MkdirAll(s.cfg.ThumbnailsPath, 0755); err != nil {
		return fmt.Errorf("failed to create thumbnails directory: %w", err)
	}

	s.cleanup()

	return filepath.WalkDir(s.cfg.OriginalsPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Printf("Error accessing %s: %v", path, err)
			return nil
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

		info, err := d.Info()
		if err != nil {
			log.Printf("Error getting info for %s: %v", path, err)
			return nil
		}

		exists, err := s.db.MediaItemExists(path, info.ModTime())
		if err != nil {
			log.Printf("Error checking existence for %s: %v", path, err)
			return nil
		}
		if exists {
			return nil
		}

		if s.isVideoExtension(ext) {
			if err := s.processVideo(path, info); err != nil {
				log.Printf("Error processing video %s: %v", path, err)
			}
			return nil
		}

		if err := s.processImage(path, info); err != nil {
			log.Printf("Error processing image %s: %v", path, err)
		}
		return nil
	})
}

func (s *Scanner) cleanup() {
	paths, err := s.db.AllOriginalPaths()
	if err != nil {
		log.Printf("Error fetching paths for cleanup: %v", err)
		return
	}

	removed := 0
	for _, path := range paths {
		if _, err := os.Stat(path.OriginalPath); os.IsNotExist(err) {
			if err := s.db.DeleteMediaItem(path.OriginalPath); err != nil {
				log.Printf("Error removing database entry for %s: %v", path.OriginalPath, err)
				continue
			}
			if pathWithin(s.cfg.ThumbnailsPath, path.ThumbnailPath) {
				os.Remove(path.ThumbnailPath)
			} else {
				log.Printf("Skipping thumbnail cleanup outside thumbnails path: %s", path.ThumbnailPath)
			}
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
	return s.isVideoExtension(ext)
}

func (s *Scanner) isVideoExtension(ext string) bool {
	for _, supported := range s.cfg.VideoExtensions {
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
		for _, candidate := range candidates {
			if _, err := os.Stat(candidate); err == nil {
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
	default:
		return false
	}
}

func (s *Scanner) processImage(path string, info fs.FileInfo) error {
	log.Printf("Processing image: %s", path)

	relPath, err := filepath.Rel(s.cfg.OriginalsPath, path)
	if err != nil {
		return fmt.Errorf("failed to get relative path: %w", err)
	}

	thumbPath := s.thumbnailPath(relPath)
	if err := os.MkdirAll(filepath.Dir(thumbPath), 0755); err != nil {
		return fmt.Errorf("failed to create thumbnail directory: %w", err)
	}

	width, height, err := s.generateThumbnail(path, thumbPath)
	if err != nil {
		return fmt.Errorf("failed to generate thumbnail: %w", err)
	}

	item := &MediaItem{
		OriginalPath:  path,
		ThumbnailPath: thumbPath,
		Folder:        folderFromRelPath(relPath),
		Filename:      info.Name(),
		Extension:     strings.ToLower(filepath.Ext(path)),
		FileSize:      info.Size(),
		ModTime:       info.ModTime(),
		Width:         width,
		Height:        height,
		MediaType:     MediaTypePhoto,
	}

	return s.db.UpsertMediaItem(item)
}

func (s *Scanner) thumbnailPath(relPath string) string {
	thumbRelPath := strings.TrimSuffix(relPath, filepath.Ext(relPath)) + ".jpg"
	return filepath.Join(s.cfg.ThumbnailsPath, thumbRelPath)
}

func folderFromRelPath(relPath string) string {
	folder := filepath.Dir(relPath)
	if folder == "." {
		return ""
	}
	return filepath.ToSlash(folder)
}

func (s *Scanner) generateThumbnail(mediaPath, thumbPath string) (width, height int, err error) {
	ext := strings.ToLower(filepath.Ext(mediaPath))
	if isStandardImage(ext) {
		return s.generateStandardThumbnail(mediaPath, thumbPath)
	}
	return s.generateRawThumbnail(mediaPath, thumbPath)
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

	tempFile, err := os.CreateTemp(filepath.Dir(thumbPath), "glimpse-*.ppm")
	if err != nil {
		return 0, 0, fmt.Errorf("failed to create temp file: %w", err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

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

type videoMetadata struct {
	Width      int
	Height     int
	Duration   float64
	VideoCodec string
	AudioCodec string
	Framerate  float64
}

func (s *Scanner) processVideo(path string, info fs.FileInfo) error {
	log.Printf("Processing video: %s", path)

	relPath, err := filepath.Rel(s.cfg.OriginalsPath, path)
	if err != nil {
		return fmt.Errorf("failed to get relative path: %w", err)
	}

	thumbPath := s.thumbnailPath(relPath)
	if err := os.MkdirAll(filepath.Dir(thumbPath), 0755); err != nil {
		return fmt.Errorf("failed to create thumbnail directory: %w", err)
	}

	meta, err := s.generateVideoThumbnail(path, thumbPath)
	if err != nil {
		return fmt.Errorf("failed to generate video thumbnail: %w", err)
	}

	item := &MediaItem{
		OriginalPath:  path,
		ThumbnailPath: thumbPath,
		Folder:        folderFromRelPath(relPath),
		Filename:      info.Name(),
		Extension:     strings.ToLower(filepath.Ext(path)),
		FileSize:      info.Size(),
		ModTime:       info.ModTime(),
		Width:         meta.Width,
		Height:        meta.Height,
		MediaType:     MediaTypeVideo,
		Duration:      meta.Duration,
		VideoCodec:    meta.VideoCodec,
		AudioCodec:    meta.AudioCodec,
		Framerate:     meta.Framerate,
	}

	return s.db.UpsertMediaItem(item)
}

func (s *Scanner) generateVideoThumbnail(videoPath, thumbPath string) (*videoMetadata, error) {
	meta := s.probeVideo(videoPath)

	seekTime := "1"
	if meta.Duration > 0 && meta.Duration < 4 {
		seekTime = fmt.Sprintf("%.2f", meta.Duration*0.25)
	}

	size := fmt.Sprintf("%d", s.cfg.ThumbnailSize)
	cmd := exec.Command("ffmpeg",
		"-ss", seekTime,
		"-i", videoPath,
		"-vframes", "1",
		"-vf", fmt.Sprintf("scale=%s:%s:force_original_aspect_ratio=decrease", size, size),
		"-y",
		thumbPath,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("ffmpeg thumbnail failed: %w: %s", err, output)
	}

	return meta, nil
}

func pathWithin(basePath, targetPath string) bool {
	base, err := filepath.Abs(basePath)
	if err != nil {
		return false
	}
	target, err := filepath.Abs(targetPath)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != "..")
}

func (s *Scanner) probeVideo(videoPath string) *videoMetadata {
	meta := &videoMetadata{}

	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		videoPath,
	)
	output, err := cmd.Output()
	if err != nil {
		log.Printf("ffprobe failed for %s: %v", videoPath, err)
		return meta
	}

	var probe struct {
		Streams []struct {
			CodecType  string `json:"codec_type"`
			CodecName  string `json:"codec_name"`
			Width      int    `json:"width"`
			Height     int    `json:"height"`
			RFrameRate string `json:"r_frame_rate"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}

	if err := json.Unmarshal(output, &probe); err != nil {
		log.Printf("ffprobe parse failed for %s: %v", videoPath, err)
		return meta
	}

	if dur, err := strconv.ParseFloat(probe.Format.Duration, 64); err == nil {
		meta.Duration = dur
	}

	for _, stream := range probe.Streams {
		switch stream.CodecType {
		case "video":
			if meta.VideoCodec == "" {
				meta.VideoCodec = stream.CodecName
				meta.Width = stream.Width
				meta.Height = stream.Height
				if parts := strings.Split(stream.RFrameRate, "/"); len(parts) == 2 {
					num, _ := strconv.ParseFloat(parts[0], 64)
					den, _ := strconv.ParseFloat(parts[1], 64)
					if den > 0 {
						meta.Framerate = num / den
					}
				}
			}
		case "audio":
			if meta.AudioCodec == "" {
				meta.AudioCodec = stream.CodecName
			}
		}
	}

	return meta
}
