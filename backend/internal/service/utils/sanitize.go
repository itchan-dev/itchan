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

// sanitizeVideoWithThumbnail strips metadata from a video and extracts a scaled
// first-frame thumbnail in a single ffmpeg invocation.
// Returns (sanitizedVideoPath, thumbnailJPEGBytes, error).
// Thumbnail bytes may be nil if extraction failed (non-fatal).
func sanitizeVideoWithThumbnail(inputPath string, thumbMaxSize int) (string, []byte, error) {
	// Extract extension from input file so ffmpeg can determine output format
	ext := filepath.Ext(inputPath)
	tmpFile, err := os.CreateTemp("", "sanitized_video_*"+ext)
	if err != nil {
		return "", nil, err
	}
	outputPath := tmpFile.Name()
	tmpFile.Close()

	vfExpr := fmt.Sprintf(
		"scale=w='min(iw,%d)':h='min(ih,%d)':force_original_aspect_ratio=decrease",
		thumbMaxSize, thumbMaxSize,
	)

	// Single ffmpeg command with two outputs:
	//   1. Sanitized video file (copy streams, strip metadata)
	//   2. Scaled first-frame JPEG thumbnail (to stdout pipe)
	cmd := exec.Command("ffmpeg",
		"-protocol_whitelist", "file,pipe", // Prevent SSRF: only allow local file access
		"-i", inputPath,
		// Output 1: sanitized video
		"-map", "0",
		"-c", "copy",
		"-map_metadata", "-1",
		"-map_metadata:s:v", "-1",
		"-map_metadata:s:a", "-1",
		"-fflags", "+bitexact",
		"-y",
		outputPath,
		// Output 2: thumbnail to stdout
		"-ss", "00:00:00",
		"-vframes", "1",
		"-vf", vfExpr,
		"-f", "image2pipe",
		"-vcodec", "mjpeg",
		"pipe:1",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		os.Remove(outputPath)
		return "", nil, fmt.Errorf("ffmpeg failed: %w (stderr: %s)", err, stderr.String())
	}

	var thumbnail []byte
	if stdout.Len() > 0 {
		thumbnail = stdout.Bytes()
	}

	return outputPath, thumbnail, nil
}

func CheckFFmpegAvailable() error {
	cmd := exec.Command("ffmpeg", "-version")
	return cmd.Run()
}

func SanitizeImage(pendingFile *domain.PendingFile, maxDecodedSize int64) (*domain.SanitizedImage, error) {
	// Check decoded image size before decoding to prevent OOM from crafted image headers.
	// A malicious file can claim 65535x65535 in its header, causing image.Decode to allocate ~16GB.
	var reader io.Reader = pendingFile.Data

	if pendingFile.ImageWidth != nil && pendingFile.ImageHeight != nil {
		// Dimensions already extracted by validation layer (DecodeConfig)
		w, h := int64(*pendingFile.ImageWidth), int64(*pendingFile.ImageHeight)
		if w*h*4 > maxDecodedSize {
			return nil, fmt.Errorf("image too large: %dx%d pixels, decoded size would exceed %d bytes limit", w, h, maxDecodedSize)
		}
	} else {
		// Dimensions not available — buffer data and check via DecodeConfig
		data, err := io.ReadAll(pendingFile.Data)
		if err != nil {
			return nil, fmt.Errorf("failed to read image data: %w", err)
		}
		cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("failed to read image dimensions: %w", err)
		}
		if int64(cfg.Width)*int64(cfg.Height)*4 > maxDecodedSize {
			return nil, fmt.Errorf("image too large: %dx%d pixels, decoded size would exceed %d bytes limit", cfg.Width, cfg.Height, maxDecodedSize)
		}
		reader = bytes.NewReader(data)
	}

	// Decode image — this strips EXIF metadata automatically
	img, format, err := image.Decode(reader)
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
			SizeBytes:   0, // Will be set by SaveImage after encoding
			MimeType:    finalMimeType,
			ImageWidth:  &width,
			ImageHeight: &height,
		},
		Image:  img,
		Format: format,
	}, nil
}

// SanitizeVideo sanitizes a video file by stripping metadata and extracting a
// scaled thumbnail, both in a single ffmpeg invocation.
// thumbMaxSize is the maximum dimension (px) for the thumbnail.
// Returns a SanitizedVideo with path to temp file on disk (caller must move/delete it).
func SanitizeVideo(pendingFile *domain.PendingFile, thumbMaxSize int) (*domain.SanitizedVideo, error) {
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

	// Sanitize + extract thumbnail in one ffmpeg pass
	tmpOutputPath, thumbnail, err := sanitizeVideoWithThumbnail(tmpInputPath, thumbMaxSize)
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
		Thumbnail:    thumbnail,
	}, nil
}
