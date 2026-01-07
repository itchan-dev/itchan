package utils

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/itchan-dev/itchan/shared/domain"
)

// sanitizeVideo strips metadata using ffmpeg (internal helper)
// Returns path to sanitized temp file (caller must clean up)
func sanitizeVideo(inputPath string) (string, error) {
	// Extract extension from input file so ffmpeg can determine output format
	ext := filepath.Ext(inputPath)
	tmpFile, err := os.CreateTemp("", "sanitized_video_*"+ext)
	if err != nil {
		return "", err
	}
	outputPath := tmpFile.Name()
	tmpFile.Close()

	// Build ffmpeg command to strip all metadata while preserving quality
	cmd := exec.Command("ffmpeg",
		"-i", inputPath,
		"-map_metadata", "-1", // Remove all global metadata (title, artist, etc.)
		"-map_metadata:s:v", "-1", // Remove video stream metadata (encoder, creation time, etc.)
		"-map_metadata:s:a", "-1", // Remove audio stream metadata
		"-c", "copy", // Copy streams without re-encoding (fast, no quality loss)
		"-fflags", "+bitexact", // Remove encoder version info for reproducible output
		"-y", // Overwrite output file without prompting
		outputPath,
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		os.Remove(outputPath)
		return "", fmt.Errorf("ffmpeg failed: %w (stderr: %s)", err, stderr.String())
	}

	return outputPath, nil
}

// CheckFFmpegAvailable verifies ffmpeg is installed
func CheckFFmpegAvailable() error {
	cmd := exec.Command("ffmpeg", "-version")
	return cmd.Run()
}

// SanitizeImage sanitizes an image by decoding it (which strips EXIF metadata).
// Returns a SanitizedImage with decoded image ready for encoding/thumbnailing.
func SanitizeImage(pendingFile *domain.PendingFile) (*domain.SanitizedImage, error) {
	// Decode image once - this strips EXIF metadata automatically
	img, format, err := image.Decode(pendingFile.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	// Extract dimensions from decoded image
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	// Determine final MIME type and extension based on format
	var finalMimeType, finalExt string
	if format == "png" {
		finalMimeType = "image/png"
		finalExt = ".png"
	} else {
		// JPEG, GIF, and others become JPEG
		finalMimeType = "image/jpeg"
		finalExt = ".jpg"
	}

	// Update filename extension if format changed
	origExt := filepath.Ext(pendingFile.Filename)
	baseName := strings.TrimSuffix(pendingFile.Filename, origExt)
	newFilename := baseName + finalExt

	return &domain.SanitizedImage{
		FileCommonMetadata: domain.FileCommonMetadata{
			Filename:    newFilename,
			SizeBytes:   0, // Size unknown until encoding (will be set by SaveImage)
			MimeType:    finalMimeType,
			ImageWidth:  &width,
			ImageHeight: &height,
		},
		Image:  img,
		Format: format,
	}, nil
}

// SanitizeVideo sanitizes a video file by stripping metadata using ffmpeg.
// Returns a SanitizedVideo with path to temp file on disk (caller must move/delete it).
func SanitizeVideo(pendingFile *domain.PendingFile) (*domain.SanitizedVideo, error) {
	origExt := filepath.Ext(pendingFile.Filename)

	// Create temp input file for ffmpeg
	tmpInput, err := os.CreateTemp("", "upload_video_*"+origExt)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp input file: %w", err)
	}
	tmpInputPath := tmpInput.Name()

	// Write uploaded data to temp file
	_, copyErr := io.Copy(tmpInput, pendingFile.Data)
	tmpInput.Close()
	if copyErr != nil {
		os.Remove(tmpInputPath)
		return nil, fmt.Errorf("failed to write temp video: %w", copyErr)
	}

	// Sanitize with ffmpeg
	tmpOutputPath, err := sanitizeVideo(tmpInputPath)
	os.Remove(tmpInputPath) // Clean up input immediately
	if err != nil {
		return nil, fmt.Errorf("failed to sanitize video: %w", err)
	}

	// Get file size for metadata (file size may change after sanitization)
	fileInfo, err := os.Stat(tmpOutputPath)
	if err != nil {
		os.Remove(tmpOutputPath)
		return nil, fmt.Errorf("failed to stat sanitized video: %w", err)
	}

	return &domain.SanitizedVideo{
		FileCommonMetadata: domain.FileCommonMetadata{
			Filename:    pendingFile.Filename,
			SizeBytes:   fileInfo.Size(), // Updated size after sanitization
			MimeType:    pendingFile.MimeType,
			ImageWidth:  pendingFile.ImageWidth,
			ImageHeight: pendingFile.ImageHeight,
		},
		TempFilePath: tmpOutputPath,
	}, nil
}
