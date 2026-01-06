package utils

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/itchan-dev/itchan/shared/domain"
)

// SanitizeImage re-encodes images to strip metadata
// PNG → PNG (preserves transparency)
// JPEG/GIF → JPEG 85
// Returns: sanitized bytes, final MIME type, final extension, dimensions, error
func SanitizeImage(data io.Reader, mimeType string) ([]byte, string, string, *int, *int, error) {
	img, format, err := image.Decode(data)
	if err != nil {
		return nil, "", "", nil, nil, fmt.Errorf("failed to decode image: %w", err)
	}

	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	var buf bytes.Buffer
	var finalMimeType, finalExt string

	if format == "png" {
		err = png.Encode(&buf, img)
		finalMimeType = "image/png"
		finalExt = ".png"
	} else {
		err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85})
		finalMimeType = "image/jpeg"
		finalExt = ".jpg"
	}

	if err != nil {
		return nil, "", "", nil, nil, err
	}

	return buf.Bytes(), finalMimeType, finalExt, &width, &height, nil
}

// SanitizeVideo strips metadata using ffmpeg
// Returns path to sanitized temp file (caller must clean up)
func SanitizeVideo(inputPath string) (string, error) {
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

// SanitizeFile sanitizes a pending file (image or video), stripping metadata
// Returns a new PendingFile with sanitized data
// Handles all temp file creation and cleanup internally
func SanitizeFile(pendingFile *domain.PendingFile) (*domain.PendingFile, error) {
	origExt := filepath.Ext(pendingFile.Filename)
	if strings.HasPrefix(pendingFile.MimeType, "image/") {
		// Sanitize image
		sanitized, finalMime, finalExt, width, height, err := SanitizeImage(pendingFile.Data, pendingFile.MimeType)
		if err != nil {
			return nil, fmt.Errorf("failed to sanitize image: %w", err)
		}

		// Update filename extension if format changed

		baseName := strings.TrimSuffix(pendingFile.Filename, origExt)
		newFilename := baseName + finalExt

		// Return new PendingFile with sanitized data
		return &domain.PendingFile{
			FileCommonMetadata: domain.FileCommonMetadata{
				Filename:    newFilename,
				SizeBytes:   int64(len(sanitized)),
				MimeType:    finalMime,
				ImageWidth:  width,
				ImageHeight: height,
			},
			Data: bytes.NewReader(sanitized),
		}, nil

	} else if strings.HasPrefix(pendingFile.MimeType, "video/") {
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
		tmpOutputPath, err := SanitizeVideo(tmpInputPath)
		os.Remove(tmpInputPath) // Clean up input immediately
		if err != nil {
			return nil, fmt.Errorf("failed to sanitize video: %w", err)
		}
		defer os.Remove(tmpOutputPath) // Clean up output when function returns

		// Read sanitized data
		sanitizedData, err := os.ReadFile(tmpOutputPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read sanitized video: %w", err)
		}

		// Video keeps original filename and MIME type (metadata stripped, format unchanged)
		return &domain.PendingFile{
			FileCommonMetadata: domain.FileCommonMetadata{
				Filename:    pendingFile.Filename,
				SizeBytes:   int64(len(sanitizedData)),
				MimeType:    pendingFile.MimeType,
				ImageWidth:  pendingFile.ImageWidth,
				ImageHeight: pendingFile.ImageHeight,
			},
			Data: bytes.NewReader(sanitizedData),
		}, nil
	}

	// Should never reach here if validation is correct
	return nil, fmt.Errorf("unsupported file type: %s", pendingFile.MimeType)
}
