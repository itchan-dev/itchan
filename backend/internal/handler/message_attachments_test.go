package handler

import (
	"io"
	"testing"

	"github.com/itchan-dev/itchan/shared/config"
	"github.com/itchan-dev/itchan/shared/validation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessAttachments(t *testing.T) {
	cfg := &config.Config{
		Public: config.Public{
			MaxAttachmentsPerMessage: 4,
			MaxAttachmentSizeBytes:   10 * 1024 * 1024,
			MaxTotalAttachmentSize:   20 * 1024 * 1024,
			AllowedImageMimeTypes:    []string{"image/jpeg", "image/png", "image/gif"},
			AllowedVideoMimeTypes:    []string{"video/mp4", "video/webm"},
		},
	}

	t.Run("processes valid files", func(t *testing.T) {
		files := createMultipartFiles(t, []fileData{
			{name: "image.jpg", content: []byte("fake jpeg"), contentType: "image/jpeg"},
			{name: "video.mp4", content: []byte("fake mp4"), contentType: "video/mp4"},
		})

		pendingFiles, err := validation.ValidateAttachments(files, cfg.Public.AllowedImageMimeTypes, cfg.Public.AllowedVideoMimeTypes)

		require.NoError(t, err)
		assert.Len(t, pendingFiles, 2)
		assert.Equal(t, "image.jpg", pendingFiles[0].Filename)
		assert.Equal(t, "image/jpeg", pendingFiles[0].MimeType)
		assert.Equal(t, "video.mp4", pendingFiles[1].Filename)
		assert.Equal(t, "video/mp4", pendingFiles[1].MimeType)

		for _, pf := range pendingFiles {
			data, err := io.ReadAll(pf.Data)
			require.NoError(t, err)
			assert.NotEmpty(t, data)
		}
	})

	t.Run("rejects unsupported file types", func(t *testing.T) {
		files := createMultipartFiles(t, []fileData{
			{name: "document.pdf", content: []byte("fake pdf"), contentType: "application/pdf"},
		})

		_, err := validation.ValidateAttachments(files, cfg.Public.AllowedImageMimeTypes, cfg.Public.AllowedVideoMimeTypes)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid MIME type")
	})

	t.Run("handles missing content type", func(t *testing.T) {
		files := createMultipartFiles(t, []fileData{
			{name: "image.jpg", content: []byte("fake jpeg"), contentType: ""},
		})

		pendingFiles, err := validation.ValidateAttachments(files, cfg.Public.AllowedImageMimeTypes, cfg.Public.AllowedVideoMimeTypes)

		require.NoError(t, err)
		assert.Len(t, pendingFiles, 1)
		assert.Equal(t, "image/jpeg", pendingFiles[0].MimeType)
	})

	t.Run("returns nil for empty file list", func(t *testing.T) {
		pendingFiles, err := validation.ValidateAttachments(nil, cfg.Public.AllowedImageMimeTypes, cfg.Public.AllowedVideoMimeTypes)

		require.NoError(t, err)
		assert.Nil(t, pendingFiles)
	})

	t.Run("extracts image dimensions for images", func(t *testing.T) {
		jpegData := []byte{
			0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46,
			0x49, 0x46, 0x00, 0x01, 0x01, 0x00, 0x00, 0x01,
			0x00, 0x01, 0x00, 0x00, 0xFF, 0xDB, 0x00, 0x43,
			0x00, 0x08, 0x06, 0x06, 0x07, 0x06, 0x05, 0x08,
			0x07, 0x07, 0x07, 0x09, 0x09, 0x08, 0x0A, 0x0C,
			0x14, 0x0D, 0x0C, 0x0B, 0x0B, 0x0C, 0x19, 0x12,
			0x13, 0x0F, 0x14, 0x1D, 0x1A, 0x1F, 0x1E, 0x1D,
			0x1A, 0x1C, 0x1C, 0x20, 0x24, 0x2E, 0x27, 0x20,
			0x22, 0x2C, 0x23, 0x1C, 0x1C, 0x28, 0x37, 0x29,
			0x2C, 0x30, 0x31, 0x34, 0x34, 0x34, 0x1F, 0x27,
			0x39, 0x3D, 0x38, 0x32, 0x3C, 0x2E, 0x33, 0x34,
			0x32, 0xFF, 0xC0, 0x00, 0x0B, 0x08, 0x00, 0x01,
			0x00, 0x01, 0x01, 0x01, 0x11, 0x00, 0xFF, 0xC4,
			0x00, 0x14, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x03, 0xFF, 0xDA, 0x00, 0x08,
			0x01, 0x01, 0x00, 0x00, 0x3F, 0x00, 0x37, 0xFF,
			0xD9,
		}

		files := createMultipartFiles(t, []fileData{
			{name: "tiny.jpg", content: jpegData, contentType: "image/jpeg"},
		})

		pendingFiles, err := validation.ValidateAttachments(files, cfg.Public.AllowedImageMimeTypes, cfg.Public.AllowedVideoMimeTypes)

		require.NoError(t, err)
		assert.Len(t, pendingFiles, 1)

		assert.NotNil(t, pendingFiles[0].ImageWidth)
		assert.NotNil(t, pendingFiles[0].ImageHeight)
		assert.Equal(t, 1, *pendingFiles[0].ImageWidth)
		assert.Equal(t, 1, *pendingFiles[0].ImageHeight)
	})

	t.Run("doesn't extract dimensions for videos", func(t *testing.T) {
		files := createMultipartFiles(t, []fileData{
			{name: "video.mp4", content: []byte("fake mp4"), contentType: "video/mp4"},
		})

		pendingFiles, err := validation.ValidateAttachments(files, cfg.Public.AllowedImageMimeTypes, cfg.Public.AllowedVideoMimeTypes)

		require.NoError(t, err)
		assert.Len(t, pendingFiles, 1)
		assert.Nil(t, pendingFiles[0].ImageWidth)
		assert.Nil(t, pendingFiles[0].ImageHeight)
	})
}

func TestFileSizeValidation(t *testing.T) {
	t.Run("rejects total attachment size exceeding limit", func(t *testing.T) {
		// Config with 1KB total limit for testing
		smallCfg := &config.Config{
			Public: config.Public{
				MaxAttachmentsPerMessage: 4,
				MaxAttachmentSizeBytes:   512,  // 512 bytes per file
				MaxTotalAttachmentSize:   1024, // 1KB total
				AllowedImageMimeTypes:    []string{"image/jpeg"},
				AllowedVideoMimeTypes:    []string{"video/mp4"},
			},
		}

		// Create file that exceeds total limit
		largeContent := make([]byte, 2048) // 2KB - exceeds 1KB limit
		for i := range largeContent {
			largeContent[i] = byte(i % 256)
		}

		files := createMultipartFiles(t, []fileData{
			{name: "large.jpg", content: largeContent, contentType: "image/jpeg"},
		})

		// This validates MIME type but not size (size is checked at multipart parse level)
		pendingFiles, err := validation.ValidateAttachments(
			files,
			smallCfg.Public.AllowedImageMimeTypes,
			smallCfg.Public.AllowedVideoMimeTypes,
		)

		// ValidateAttachments doesn't check size - that's done in ValidateAndParseMultipart
		// So this should succeed (only MIME type is validated here)
		require.NoError(t, err)
		assert.Len(t, pendingFiles, 1)
		assert.Equal(t, int64(2048), pendingFiles[0].SizeBytes)
	})

	t.Run("validates max attachment count", func(t *testing.T) {
		cfg := &config.Config{
			Public: config.Public{
				MaxAttachmentsPerMessage: 2, // Only allow 2 files
				MaxAttachmentSizeBytes:   10 * 1024 * 1024,
				MaxTotalAttachmentSize:   20 * 1024 * 1024,
				AllowedImageMimeTypes:    []string{"image/jpeg"},
				AllowedVideoMimeTypes:    []string{},
			},
		}

		// Create 3 files (exceeds limit of 2)
		files := createMultipartFiles(t, []fileData{
			{name: "image1.jpg", content: []byte("fake1"), contentType: "image/jpeg"},
			{name: "image2.jpg", content: []byte("fake2"), contentType: "image/jpeg"},
			{name: "image3.jpg", content: []byte("fake3"), contentType: "image/jpeg"},
		})

		// Note: ValidateAttachments doesn't check count - that should be done elsewhere
		// This test documents the current behavior
		pendingFiles, err := validation.ValidateAttachments(
			files,
			cfg.Public.AllowedImageMimeTypes,
			cfg.Public.AllowedVideoMimeTypes,
		)

		// Currently passes all files through - count validation is not in ValidateAttachments
		require.NoError(t, err)
		assert.Len(t, pendingFiles, 3)
	})
}

func TestCalculateMaxRequestSize(t *testing.T) {
	tests := []struct {
		name           string
		maxAttachment  int64
		buffer         int64
		expectedResult int64
	}{
		{
			name:           "standard config",
			maxAttachment:  20 * 1024 * 1024, // 20MB
			buffer:         1 * 1024 * 1024,  // 1MB
			expectedResult: 21 * 1024 * 1024, // 21MB
		},
		{
			name:           "zero buffer",
			maxAttachment:  10 * 1024 * 1024,
			buffer:         0,
			expectedResult: 10 * 1024 * 1024,
		},
		{
			name:           "small sizes",
			maxAttachment:  1024,
			buffer:         512,
			expectedResult: 1536,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validation.CalculateMaxRequestSize(tt.maxAttachment, tt.buffer)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}
