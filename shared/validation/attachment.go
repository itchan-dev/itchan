package validation

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"mime"
	"mime/multipart"
	"path/filepath"
	"strings"

	"github.com/itchan-dev/itchan/shared/domain"
)

func ValidateAttachments(fileHeaders []*multipart.FileHeader, allowedImageMimes, allowedVideoMimes []string) ([]*domain.PendingFile, error) {
	if len(fileHeaders) == 0 {
		return nil, nil
	}

	// Build allowed MIME types map
	allowedMimes := BuildAllowedMimeMap(allowedImageMimes, allowedVideoMimes)

	var pendingFiles []*domain.PendingFile

	for _, fileHeader := range fileHeaders {
		// Open the uploaded file
		file, err := fileHeader.Open()
		if err != nil {
			return nil, fmt.Errorf("failed to open uploaded file: %w", err)
		}

		// Detect MIME type
		mimeType, err := DetectMimeType(fileHeader)
		if err != nil {
			file.Close()
			return nil, err
		}

		// Validate that it's an allowed type
		if !allowedMimes[mimeType] {
			file.Close()
			return nil, fmt.Errorf("%w: %s (file: %s)", ErrInvalidMimeType, mimeType, fileHeader.Filename)
		}

		// Try to get image dimensions if it's an image
		width, height := ExtractImageDimensions(file, mimeType)

		pendingFiles = append(pendingFiles, &domain.PendingFile{
			FileCommonMetadata: domain.FileCommonMetadata{
				Filename:    fileHeader.Filename,
				SizeBytes:   fileHeader.Size,
				MimeType:    mimeType,
				ImageWidth:  width,
				ImageHeight: height,
			},
			Data: file,
		})
	}

	return pendingFiles, nil
}

func BuildAllowedMimeMap(imageMimes, videoMimes []string) map[string]bool {
	allowedMimes := make(map[string]bool)
	for _, m := range imageMimes {
		allowedMimes[m] = true
	}
	for _, m := range videoMimes {
		allowedMimes[m] = true
	}
	return allowedMimes
}

func DetectMimeType(fileHeader *multipart.FileHeader) (string, error) {
	mimeType := fileHeader.Header.Get("Content-Type")

	// If no Content-Type or it's generic, detect from extension
	if mimeType == "" || mimeType == "application/octet-stream" {
		ext := filepath.Ext(fileHeader.Filename)
		detectedType := mime.TypeByExtension(ext)
		if detectedType != "" {
			mimeType = detectedType
		}
	}

	if mimeType == "" {
		return "", fmt.Errorf("could not detect MIME type for file: %s", fileHeader.Filename)
	}

	return mimeType, nil
}

func ExtractImageDimensions(file multipart.File, mimeType string) (*int, *int) {
	// Only process images
	if !strings.HasPrefix(mimeType, "image/") {
		return nil, nil
	}

	// Try to decode image config
	img, _, err := image.DecodeConfig(file)
	if err != nil {
		// If we can't decode, just return nil (not a fatal error)
		file.Seek(0, 0) // Reset file pointer
		return nil, nil
	}

	// Reset file pointer after reading
	file.Seek(0, 0)

	width, height := img.Width, img.Height
	return &width, &height
}
