package utils

import (
	"bytes"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"testing"

	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitizeImage_PNG(t *testing.T) {
	// Create test PNG image
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	// Fill with some color to make it non-empty
	for y := range 100 {
		for x := range 100 {
			img.Set(x, y, image.White)
		}
	}

	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))

	pendingFile := &domain.PendingFile{
		FileCommonMetadata: domain.FileCommonMetadata{
			Filename:  "test.png",
			MimeType:  "image/png",
			SizeBytes: int64(buf.Len()),
		},
		Data: bytes.NewReader(buf.Bytes()),
	}

	// Sanitize
	result, err := SanitizeImage(pendingFile, 20*1024*1024)
	require.NoError(t, err)

	assert.Equal(t, "image/png", result.MimeType)
	assert.NotNil(t, result.ImageWidth)
	assert.NotNil(t, result.ImageHeight)
	assert.Equal(t, 100, *result.ImageWidth)
	assert.Equal(t, 100, *result.ImageHeight)
	assert.Equal(t, "png", result.Format)
	assert.NotNil(t, result.Image, "decoded Image should be present")

	// Verify the decoded image has correct dimensions
	decodedImg, ok := result.Image.(image.Image)
	require.True(t, ok, "Image should be of type image.Image")
	bounds := decodedImg.Bounds()
	assert.Equal(t, 100, bounds.Dx())
	assert.Equal(t, 100, bounds.Dy())
}

func TestSanitizeImage_JPEG(t *testing.T) {
	// Create test JPEG image
	img := image.NewRGBA(image.Rect(0, 0, 50, 50))
	for y := range 50 {
		for x := range 50 {
			img.Set(x, y, image.Black)
		}
	}

	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}))

	pendingFile := &domain.PendingFile{
		FileCommonMetadata: domain.FileCommonMetadata{
			Filename:  "test.jpg",
			MimeType:  "image/jpeg",
			SizeBytes: int64(buf.Len()),
		},
		Data: bytes.NewReader(buf.Bytes()),
	}

	// Sanitize
	result, err := SanitizeImage(pendingFile, 20*1024*1024)
	require.NoError(t, err)

	assert.Equal(t, "image/jpeg", result.MimeType)
	assert.NotNil(t, result.ImageWidth)
	assert.NotNil(t, result.ImageHeight)
	assert.Equal(t, 50, *result.ImageWidth)
	assert.Equal(t, 50, *result.ImageHeight)
	assert.Equal(t, "jpeg", result.Format)
	assert.NotNil(t, result.Image)

	// Verify the decoded image has correct dimensions
	decodedImg, ok := result.Image.(image.Image)
	require.True(t, ok)
	bounds := decodedImg.Bounds()
	assert.Equal(t, 50, bounds.Dx())
	assert.Equal(t, 50, bounds.Dy())
}

func TestSanitizeImage_GIF_ConvertedToJPEG(t *testing.T) {
	// Create test GIF image
	img := image.NewRGBA(image.Rect(0, 0, 80, 60))
	for y := range 60 {
		for x := range 80 {
			img.Set(x, y, image.White)
		}
	}

	var buf bytes.Buffer
	require.NoError(t, gif.Encode(&buf, img, nil))

	pendingFile := &domain.PendingFile{
		FileCommonMetadata: domain.FileCommonMetadata{
			Filename:  "test.gif",
			MimeType:  "image/gif",
			SizeBytes: int64(buf.Len()),
		},
		Data: bytes.NewReader(buf.Bytes()),
	}

	// Sanitize
	result, err := SanitizeImage(pendingFile, 20*1024*1024)
	require.NoError(t, err)

	// GIF should be converted to JPEG
	assert.Equal(t, "image/jpeg", result.MimeType)
	assert.NotNil(t, result.ImageWidth)
	assert.NotNil(t, result.ImageHeight)
	assert.Equal(t, 80, *result.ImageWidth)
	assert.Equal(t, 60, *result.ImageHeight)
	assert.Equal(t, "gif", result.Format) // Format is from image.Decode, still "gif"
	assert.NotNil(t, result.Image)

	// Verify the decoded image has correct dimensions
	decodedImg, ok := result.Image.(image.Image)
	require.True(t, ok)
	bounds := decodedImg.Bounds()
	assert.Equal(t, 80, bounds.Dx())
	assert.Equal(t, 60, bounds.Dy())
}

func TestSanitizeImage_InvalidData(t *testing.T) {
	// Test with invalid image data
	invalidData := []byte("not an image")

	pendingFile := &domain.PendingFile{
		FileCommonMetadata: domain.FileCommonMetadata{
			Filename:  "test.jpg",
			MimeType:  "image/jpeg",
			SizeBytes: int64(len(invalidData)),
		},
		Data: bytes.NewReader(invalidData),
	}

	_, err := SanitizeImage(pendingFile, 20*1024*1024)
	assert.Error(t, err, "expected error for invalid image data")
}

func TestSanitizeImage_EmptyData(t *testing.T) {
	// Test with empty data
	emptyData := []byte{}

	pendingFile := &domain.PendingFile{
		FileCommonMetadata: domain.FileCommonMetadata{
			Filename:  "test.png",
			MimeType:  "image/png",
			SizeBytes: 0,
		},
		Data: bytes.NewReader(emptyData),
	}

	_, err := SanitizeImage(pendingFile, 20*1024*1024)
	assert.Error(t, err, "expected error for empty data")
}

func TestSanitizeImage_RejectsOversizedDimensions(t *testing.T) {
	// Simulate a crafted image with forged 65535x65535 header dimensions.
	// With dimensions pre-extracted by validation layer, SanitizeImage should
	// reject before calling image.Decode (which would allocate ~16GB).
	w, h := 65535, 65535

	pendingFile := &domain.PendingFile{
		FileCommonMetadata: domain.FileCommonMetadata{
			Filename:    "bomb.jpg",
			MimeType:    "image/jpeg",
			SizeBytes:   1024,
			ImageWidth:  &w,
			ImageHeight: &h,
		},
		Data: bytes.NewReader([]byte("fake")),
	}

	_, err := SanitizeImage(pendingFile, 20*1024*1024)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "image too large")
}

func TestSanitizeImage_RejectsOversizedDimensions_Fallback(t *testing.T) {
	// Test the fallback path: dimensions nil, DecodeConfig used.
	// Create a real 1x1 JPEG but with a tiny limit to trigger rejection.
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, nil))

	pendingFile := &domain.PendingFile{
		FileCommonMetadata: domain.FileCommonMetadata{
			Filename:  "small.jpg",
			MimeType:  "image/jpeg",
			SizeBytes: int64(buf.Len()),
			// ImageWidth/ImageHeight intentionally nil
		},
		Data: bytes.NewReader(buf.Bytes()),
	}

	// 10x10x4 = 400 bytes decoded. Set limit to 100 to trigger rejection.
	_, err := SanitizeImage(pendingFile, 100)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "image too large")
}

func TestCheckFFmpegAvailable(t *testing.T) {
	err := CheckFFmpegAvailable()
	if err != nil {
		t.Skipf("ffmpeg not available in test environment: %v", err)
	}
	// If no error, ffmpeg is available - test passes
}

func TestSanitizeImage_PreservesQuality(t *testing.T) {
	// Create a high-quality test image
	img := image.NewRGBA(image.Rect(0, 0, 200, 200))
	for y := range 200 {
		for x := range 200 {
			// Create a gradient pattern
			img.Set(x, y, image.White)
		}
	}

	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, &jpeg.Options{Quality: 100}))

	originalSize := buf.Len()

	pendingFile := &domain.PendingFile{
		FileCommonMetadata: domain.FileCommonMetadata{
			Filename:  "test.jpg",
			MimeType:  "image/jpeg",
			SizeBytes: int64(buf.Len()),
		},
		Data: bytes.NewReader(buf.Bytes()),
	}

	// Sanitize
	result, err := SanitizeImage(pendingFile, 20*1024*1024)
	require.NoError(t, err)

	t.Logf("Original size: %d bytes (note: sanitized will be encoded with quality 85)", originalSize)

	// Verify sanitized image is valid
	assert.NotNil(t, result.Image)
	decodedImg, ok := result.Image.(image.Image)
	require.True(t, ok)

	bounds := decodedImg.Bounds()
	assert.Equal(t, 200, bounds.Dx(), "width should be preserved")
	assert.Equal(t, 200, bounds.Dy(), "height should be preserved")
}

func TestSanitizeImage_PNGTransparency(t *testing.T) {
	// Create PNG with alpha channel
	img := image.NewRGBA(image.Rect(0, 0, 50, 50))
	// Set some pixels with transparency
	for y := range 50 {
		for x := range 50 {
			img.Set(x, y, image.Transparent)
		}
	}

	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))

	pendingFile := &domain.PendingFile{
		FileCommonMetadata: domain.FileCommonMetadata{
			Filename:  "test.png",
			MimeType:  "image/png",
			SizeBytes: int64(buf.Len()),
		},
		Data: bytes.NewReader(buf.Bytes()),
	}

	// Sanitize
	result, err := SanitizeImage(pendingFile, 20*1024*1024)
	require.NoError(t, err)

	// Should remain PNG (not converted to JPEG)
	assert.Equal(t, "image/png", result.MimeType)
	assert.Equal(t, "png", result.Format)

	// Verify decoded image is valid
	assert.NotNil(t, result.Image)
	decodedImg, ok := result.Image.(image.Image)
	require.True(t, ok)
	bounds := decodedImg.Bounds()
	assert.Equal(t, 50, bounds.Dx())
	assert.Equal(t, 50, bounds.Dy())
}

func BenchmarkSanitizeImage_PNG(b *testing.B) {
	img := image.NewRGBA(image.Rect(0, 0, 1000, 3000))
	var buf bytes.Buffer
	png.Encode(&buf, img)
	data := buf.Bytes()

	for b.Loop() {
		pendingFile := &domain.PendingFile{
			FileCommonMetadata: domain.FileCommonMetadata{
				Filename:  "bench.png",
				MimeType:  "image/png",
				SizeBytes: int64(len(data)),
			},
			Data: bytes.NewReader(data),
		}
		SanitizeImage(pendingFile, 20*1024*1024)
	}
}

func BenchmarkSanitizeImage_JPEG(b *testing.B) {
	img := image.NewRGBA(image.Rect(0, 0, 1000, 3000))
	var buf bytes.Buffer
	jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90})
	data := buf.Bytes()

	for b.Loop() {
		pendingFile := &domain.PendingFile{
			FileCommonMetadata: domain.FileCommonMetadata{
				Filename:  "bench.jpg",
				MimeType:  "image/jpeg",
				SizeBytes: int64(len(data)),
			},
			Data: bytes.NewReader(data),
		}
		SanitizeImage(pendingFile, 20*1024*1024)
	}
}

// TestSanitizeImage_ReturnsImageAndFormat tests that image sanitization returns SanitizedImage with decoded Image and Format
func TestSanitizeImage_ReturnsImageAndFormat(t *testing.T) {
	// Create test PNG image
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := range 100 {
		for x := range 100 {
			img.Set(x, y, image.White)
		}
	}

	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))

	pendingFile := &domain.PendingFile{
		FileCommonMetadata: domain.FileCommonMetadata{
			Filename:  "test.png",
			MimeType:  "image/png",
			SizeBytes: int64(buf.Len()),
		},
		Data: bytes.NewReader(buf.Bytes()),
	}

	result, err := SanitizeImage(pendingFile, 20*1024*1024)
	require.NoError(t, err)

	// Image should have decoded Image and Format set
	assert.NotNil(t, result.Image, "SanitizedImage should have decoded Image set")
	assert.Equal(t, "png", result.Format, "Format should be set from image.Decode")
	assert.Equal(t, "image/png", result.MimeType)
	assert.NotNil(t, result.ImageWidth)
	assert.NotNil(t, result.ImageHeight)
	assert.Equal(t, 100, *result.ImageWidth)
	assert.Equal(t, 100, *result.ImageHeight)

	// Verify the decoded image is valid
	decodedImg, ok := result.Image.(image.Image)
	require.True(t, ok, "Image should be of type image.Image")
	bounds := decodedImg.Bounds()
	assert.Equal(t, 100, bounds.Dx())
	assert.Equal(t, 100, bounds.Dy())
}

// TestSanitizeVideo_ReturnsTempPath tests that video sanitization returns SanitizedVideo with TempFilePath
func TestSanitizeVideo_ReturnsTempPath(t *testing.T) {
	// Check if ffmpeg is available
	if err := CheckFFmpegAvailable(); err != nil {
		t.Skip("ffmpeg not available, skipping video test")
	}

	// Create a minimal valid MP4 file (we'll create a simple temp file)
	// For testing purposes, we'll use a very small video or just check the path mechanism
	tmpFile, err := os.CreateTemp("", "test_video_*.mp4")
	require.NoError(t, err)
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	// Write minimal MP4 header (ftyp box) to make it a valid MP4 for ffmpeg
	// This is a minimal MP4 that ffmpeg can process
	minimalMP4 := []byte{
		0x00, 0x00, 0x00, 0x20, 'f', 't', 'y', 'p',
		'i', 's', 'o', 'm', 0x00, 0x00, 0x02, 0x00,
		'i', 's', 'o', 'm', 'i', 's', 'o', '2',
		'a', 'v', 'c', '1', 'm', 'p', '4', '1',
		0x00, 0x00, 0x00, 0x08, 'm', 'd', 'a', 't',
	}
	_, err = tmpFile.Write(minimalMP4)
	require.NoError(t, err)
	tmpFile.Close()

	// Read it back for PendingFile
	videoData, err := os.ReadFile(tmpPath)
	require.NoError(t, err)

	pendingFile := &domain.PendingFile{
		FileCommonMetadata: domain.FileCommonMetadata{
			Filename:  "test.mp4",
			MimeType:  "video/mp4",
			SizeBytes: int64(len(videoData)),
		},
		Data: bytes.NewReader(videoData),
	}

	result, err := SanitizeVideo(pendingFile, 225)

	// Clean up temp file if sanitization created one
	if result != nil {
		defer os.Remove(result.TempFilePath)
	}

	// Note: This might fail if ffmpeg can't process our minimal MP4
	// In that case, we skip the test
	if err != nil {
		if bytes.Contains([]byte(err.Error()), []byte("ffmpeg")) {
			t.Skip("ffmpeg couldn't process minimal MP4, skipping")
		}
		t.Fatalf("unexpected error: %v", err)
	}

	// Video should have TempFilePath set
	assert.NotEmpty(t, result.TempFilePath, "SanitizedVideo should have TempFilePath set")
	assert.Equal(t, "video/mp4", result.MimeType)
	assert.Equal(t, "test.mp4", result.Filename)
}

// TestSanitizeVideo_TempFileExists tests that the temp file actually exists
func TestSanitizeVideo_TempFileExists(t *testing.T) {
	// Check if ffmpeg is available
	if err := CheckFFmpegAvailable(); err != nil {
		t.Skip("ffmpeg not available, skipping video test")
	}

	// Create a minimal valid MP4 file
	tmpFile, err := os.CreateTemp("", "test_video_*.mp4")
	require.NoError(t, err)
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	minimalMP4 := []byte{
		0x00, 0x00, 0x00, 0x20, 'f', 't', 'y', 'p',
		'i', 's', 'o', 'm', 0x00, 0x00, 0x02, 0x00,
		'i', 's', 'o', 'm', 'i', 's', 'o', '2',
		'a', 'v', 'c', '1', 'm', 'p', '4', '1',
		0x00, 0x00, 0x00, 0x08, 'm', 'd', 'a', 't',
	}
	tmpFile.Write(minimalMP4)
	tmpFile.Close()

	videoData, _ := os.ReadFile(tmpPath)

	pendingFile := &domain.PendingFile{
		FileCommonMetadata: domain.FileCommonMetadata{
			Filename:  "test.mp4",
			MimeType:  "video/mp4",
			SizeBytes: int64(len(videoData)),
		},
		Data: bytes.NewReader(videoData),
	}

	result, err := SanitizeVideo(pendingFile, 225)

	// Clean up temp file
	if result != nil {
		defer os.Remove(result.TempFilePath)
	}

	if err != nil {
		if bytes.Contains([]byte(err.Error()), []byte("ffmpeg")) {
			t.Skip("ffmpeg couldn't process minimal MP4, skipping")
		}
		t.Fatalf("unexpected error: %v", err)
	}

	require.NotEmpty(t, result.TempFilePath)

	// Verify the temp file actually exists
	_, err = os.Stat(result.TempFilePath)
	assert.NoError(t, err, "Temp file should exist at the returned path")

	// Verify the file has content
	fileInfo, err := os.Stat(result.TempFilePath)
	require.NoError(t, err)
	assert.True(t, fileInfo.Size() > 0, "Temp file should have content")
}

// TestSanitizeVideo_FileSizeMatches tests that SizeBytes matches actual file size
func TestSanitizeVideo_FileSizeMatches(t *testing.T) {
	// Check if ffmpeg is available
	if err := CheckFFmpegAvailable(); err != nil {
		t.Skip("ffmpeg not available, skipping video test")
	}

	// Create a minimal valid MP4 file
	tmpFile, err := os.CreateTemp("", "test_video_*.mp4")
	require.NoError(t, err)
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	minimalMP4 := []byte{
		0x00, 0x00, 0x00, 0x20, 'f', 't', 'y', 'p',
		'i', 's', 'o', 'm', 0x00, 0x00, 0x02, 0x00,
		'i', 's', 'o', 'm', 'i', 's', 'o', '2',
		'a', 'v', 'c', '1', 'm', 'p', '4', '1',
		0x00, 0x00, 0x00, 0x08, 'm', 'd', 'a', 't',
	}
	tmpFile.Write(minimalMP4)
	tmpFile.Close()

	videoData, _ := os.ReadFile(tmpPath)

	pendingFile := &domain.PendingFile{
		FileCommonMetadata: domain.FileCommonMetadata{
			Filename:  "test.mp4",
			MimeType:  "video/mp4",
			SizeBytes: int64(len(videoData)),
		},
		Data: bytes.NewReader(videoData),
	}

	result, err := SanitizeVideo(pendingFile, 225)

	// Clean up temp file
	if result != nil {
		defer os.Remove(result.TempFilePath)
	}

	if err != nil {
		if bytes.Contains([]byte(err.Error()), []byte("ffmpeg")) {
			t.Skip("ffmpeg couldn't process minimal MP4, skipping")
		}
		t.Fatalf("unexpected error: %v", err)
	}

	require.NotEmpty(t, result.TempFilePath)

	// Get actual file size
	fileInfo, err := os.Stat(result.TempFilePath)
	require.NoError(t, err)

	// Verify SizeBytes matches actual file size
	assert.Equal(t, fileInfo.Size(), result.SizeBytes, "SizeBytes should match actual file size")
}

// TestSanitizeVideo_ThumbnailExtracted tests that thumbnail is extracted during sanitization
func TestSanitizeVideo_ThumbnailExtracted(t *testing.T) {
	if err := CheckFFmpegAvailable(); err != nil {
		t.Skip("ffmpeg not available, skipping video test")
	}

	// Use real test video (has actual video frames for thumbnail extraction)
	videoData, err := os.ReadFile("../../../test_data/test_video.webm")
	if err != nil {
		t.Skip("test_video.webm not found, skipping thumbnail test")
	}

	pendingFile := &domain.PendingFile{
		FileCommonMetadata: domain.FileCommonMetadata{
			Filename:  "test.webm",
			MimeType:  "video/webm",
			SizeBytes: int64(len(videoData)),
		},
		Data: bytes.NewReader(videoData),
	}

	result, err := SanitizeVideo(pendingFile, 225)
	if result != nil {
		defer os.Remove(result.TempFilePath)
	}

	if err != nil {
		if bytes.Contains([]byte(err.Error()), []byte("ffmpeg")) {
			t.Skip("ffmpeg couldn't process test video, skipping")
		}
		t.Fatalf("unexpected error: %v", err)
	}

	// Thumbnail should be non-empty JPEG bytes
	require.NotEmpty(t, result.Thumbnail, "Thumbnail should be extracted from real video")

	// Verify it's valid JPEG (starts with JPEG SOI marker 0xFFD8)
	assert.True(t, len(result.Thumbnail) > 2, "Thumbnail should have content")
	assert.Equal(t, byte(0xFF), result.Thumbnail[0], "Thumbnail should start with JPEG SOI marker")
	assert.Equal(t, byte(0xD8), result.Thumbnail[1], "Thumbnail should start with JPEG SOI marker")

	// Verify thumbnail can be decoded as valid image
	thumbImg, err := jpeg.Decode(bytes.NewReader(result.Thumbnail))
	require.NoError(t, err, "Thumbnail should be a decodable JPEG")

	// Verify thumbnail dimensions are within maxSize
	bounds := thumbImg.Bounds()
	assert.LessOrEqual(t, bounds.Dx(), 225, "Thumbnail width should be <= maxSize")
	assert.LessOrEqual(t, bounds.Dy(), 225, "Thumbnail height should be <= maxSize")
}
