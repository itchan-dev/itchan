package utils

import (
	"bytes"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"testing"
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
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("failed to create test PNG: %v", err)
	}

	// Sanitize
	sanitized, mimeType, ext, width, height, err := SanitizeImage(bytes.NewReader(buf.Bytes()), "image/png")

	if err != nil {
		t.Fatalf("sanitization failed: %v", err)
	}
	if mimeType != "image/png" {
		t.Errorf("expected image/png, got %s", mimeType)
	}
	if ext != ".png" {
		t.Errorf("expected .png, got %s", ext)
	}
	if *width != 100 || *height != 100 {
		t.Errorf("expected 100x100, got %dx%d", *width, *height)
	}
	if len(sanitized) == 0 {
		t.Error("sanitized data is empty")
	}

	// Verify it's a valid PNG by decoding it
	_, err = png.Decode(bytes.NewReader(sanitized))
	if err != nil {
		t.Errorf("sanitized output is not valid PNG: %v", err)
	}
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
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("failed to create test JPEG: %v", err)
	}

	// Sanitize
	sanitized, mimeType, ext, width, height, err := SanitizeImage(bytes.NewReader(buf.Bytes()), "image/jpeg")

	if err != nil {
		t.Fatalf("sanitization failed: %v", err)
	}
	if mimeType != "image/jpeg" {
		t.Errorf("expected image/jpeg, got %s", mimeType)
	}
	if ext != ".jpg" {
		t.Errorf("expected .jpg, got %s", ext)
	}
	if *width != 50 || *height != 50 {
		t.Errorf("expected 50x50, got %dx%d", *width, *height)
	}

	// Verify it's a valid JPEG
	_, err = jpeg.Decode(bytes.NewReader(sanitized))
	if err != nil {
		t.Errorf("sanitized output is not valid JPEG: %v", err)
	}
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
	if err := gif.Encode(&buf, img, nil); err != nil {
		t.Fatalf("failed to create test GIF: %v", err)
	}

	// Sanitize
	sanitized, mimeType, ext, width, height, err := SanitizeImage(bytes.NewReader(buf.Bytes()), "image/gif")

	if err != nil {
		t.Fatalf("sanitization failed: %v", err)
	}
	if mimeType != "image/jpeg" {
		t.Errorf("expected image/jpeg (GIF→JPEG conversion), got %s", mimeType)
	}
	if ext != ".jpg" {
		t.Errorf("expected .jpg, got %s", ext)
	}
	if *width != 80 || *height != 60 {
		t.Errorf("expected 80x60, got %dx%d", *width, *height)
	}

	// Verify it's a valid JPEG (not GIF)
	_, err = jpeg.Decode(bytes.NewReader(sanitized))
	if err != nil {
		t.Errorf("sanitized output is not valid JPEG: %v", err)
	}

	// Verify it's NOT a GIF anymore
	_, err = gif.Decode(bytes.NewReader(sanitized))
	if err == nil {
		t.Error("output should not be decodable as GIF after conversion")
	}
}

func TestSanitizeImage_InvalidData(t *testing.T) {
	// Test with invalid image data
	invalidData := []byte("not an image")

	_, _, _, _, _, err := SanitizeImage(bytes.NewReader(invalidData), "image/jpeg")

	if err == nil {
		t.Error("expected error for invalid image data, got nil")
	}
}

func TestSanitizeImage_EmptyData(t *testing.T) {
	// Test with empty data
	emptyData := []byte{}

	_, _, _, _, _, err := SanitizeImage(bytes.NewReader(emptyData), "image/png")

	if err == nil {
		t.Error("expected error for empty data, got nil")
	}
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
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 100}); err != nil {
		t.Fatalf("failed to create test JPEG: %v", err)
	}

	originalSize := buf.Len()

	// Sanitize
	sanitized, _, _, _, _, err := SanitizeImage(bytes.NewReader(buf.Bytes()), "image/jpeg")
	if err != nil {
		t.Fatalf("sanitization failed: %v", err)
	}

	sanitizedSize := len(sanitized)

	// Quality 85 should produce reasonably sized output
	// It might be slightly smaller or larger depending on content
	t.Logf("Original size: %d bytes, Sanitized size: %d bytes", originalSize, sanitizedSize)

	// Verify sanitized image is still valid
	decodedImg, err := jpeg.Decode(bytes.NewReader(sanitized))
	if err != nil {
		t.Errorf("failed to decode sanitized image: %v", err)
	}

	bounds := decodedImg.Bounds()
	if bounds.Dx() != 200 || bounds.Dy() != 200 {
		t.Errorf("dimensions changed after sanitization: got %dx%d", bounds.Dx(), bounds.Dy())
	}
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
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("failed to create test PNG: %v", err)
	}

	// Sanitize
	sanitized, mimeType, _, _, _, err := SanitizeImage(bytes.NewReader(buf.Bytes()), "image/png")
	if err != nil {
		t.Fatalf("sanitization failed: %v", err)
	}

	// Should remain PNG (not converted to JPEG)
	if mimeType != "image/png" {
		t.Errorf("PNG should not be converted, got %s", mimeType)
	}

	// Verify it's still a valid PNG
	_, err = png.Decode(bytes.NewReader(sanitized))
	if err != nil {
		t.Errorf("sanitized output is not valid PNG: %v", err)
	}
}

func BenchmarkSanitizeImage_PNG(b *testing.B) {
	img := image.NewRGBA(image.Rect(0, 0, 1000, 3000))
	var buf bytes.Buffer
	png.Encode(&buf, img)
	data := buf.Bytes()

	for b.Loop() {
		SanitizeImage(bytes.NewReader(data), "image/png")
	}
}

func BenchmarkSanitizeImage_JPEG(b *testing.B) {
	img := image.NewRGBA(image.Rect(0, 0, 1000, 3000))
	var buf bytes.Buffer
	jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90})
	data := buf.Bytes()

	for b.Loop() {
		SanitizeImage(bytes.NewReader(data), "image/jpeg")
	}
}
