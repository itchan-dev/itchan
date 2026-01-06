package utils

import (
	"crypto/rand"
	"fmt"
	"image"
	"math/big"
	"unicode"
	"unicode/utf8"

	"github.com/itchan-dev/itchan/shared/config"
	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/itchan-dev/itchan/shared/errors"
	"golang.org/x/image/draw"
)

func IsLetter(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

type BoardNameValidator struct{ Сfg *config.Public }

func (e *BoardNameValidator) Name(name string) error {
	if utf8.RuneCountInString(name) > e.Сfg.BoardNameMaxLen {
		return &errors.ErrorWithStatusCode{Message: "Name is too long", StatusCode: 400}
	}
	if !IsLetter(name) {
		return &errors.ErrorWithStatusCode{Message: "Name should contain only letters", StatusCode: 400}
	}
	return nil
}

func (e *BoardNameValidator) ShortName(name string) error {
	if utf8.RuneCountInString(name) > e.Сfg.BoardShortNameMaxLen {
		return &errors.ErrorWithStatusCode{Message: "Name is too long", StatusCode: 400}
	}
	if !IsLetter(name) {
		return &errors.ErrorWithStatusCode{Message: "Name should contain only letters", StatusCode: 400}
	}
	return nil
}

func New(cfg *config.Public) *BoardNameValidator {
	return &BoardNameValidator{Сfg: cfg}
}

type ThreadTitleValidator struct{ Сfg *config.Public }

func (e *ThreadTitleValidator) Title(name string) error {
	if utf8.RuneCountInString(name) > e.Сfg.ThreadTitleMaxLen {
		return &errors.ErrorWithStatusCode{Message: "Name is too long", StatusCode: 400}
	}
	return nil
}

type MessageValidator struct{ Сfg *config.Public }

func (e *MessageValidator) Text(text string) error {
	runeCount := utf8.RuneCountInString(text)

	if runeCount > e.Сfg.MessageTextMaxLen {
		return &errors.ErrorWithStatusCode{Message: "Text is too long", StatusCode: 400}
	}

	if runeCount < e.Сfg.MessageTextMinLen {
		return &errors.ErrorWithStatusCode{Message: "Text is too short", StatusCode: 400}
	}

	return nil
}

// PendingFiles checks if pending files meet the configured constraints
func (e *MessageValidator) PendingFiles(files []*domain.PendingFile) error {
	if files == nil {
		return nil
	}

	// Check max count
	if len(files) > e.Сfg.MaxAttachmentsPerMessage {
		return &errors.ErrorWithStatusCode{
			Message:    fmt.Sprintf("too many attachments: max %d allowed", e.Сfg.MaxAttachmentsPerMessage),
			StatusCode: 400,
		}
	}

	var totalSize int64
	allowedMimeTypes := e.buildAllowedMimeTypes()

	// Validate each file
	for _, file := range files {
		if err := e.validateFileMeta(file.MimeType, file.SizeBytes, allowedMimeTypes); err != nil {
			return err
		}

		totalSize += file.SizeBytes
	}

	// Check total size
	if totalSize > e.Сfg.MaxTotalAttachmentSize {
		return &errors.ErrorWithStatusCode{
			Message:    fmt.Sprintf("total attachments size too large: max %d bytes allowed", e.Сfg.MaxTotalAttachmentSize),
			StatusCode: 400,
		}
	}

	return nil
}

// buildAllowedMimeTypes returns a map of allowed MIME types
func (e *MessageValidator) buildAllowedMimeTypes() map[string]bool {
	allowedMimeTypes := make(map[string]bool)

	for _, mimeType := range e.Сfg.AllowedImageMimeTypes {
		allowedMimeTypes[mimeType] = true
	}
	for _, mimeType := range e.Сfg.AllowedVideoMimeTypes {
		allowedMimeTypes[mimeType] = true
	}

	return allowedMimeTypes
}

// validateFileMeta validates a single file's MIME type and size
func (e *MessageValidator) validateFileMeta(mimeType string, size int64, allowedMimeTypes map[string]bool) error {
	// Check MIME type
	if !allowedMimeTypes[mimeType] {
		return &errors.ErrorWithStatusCode{
			Message:    fmt.Sprintf("unsupported file type: %s", mimeType),
			StatusCode: 400,
		}
	}

	// Check individual file size
	if size > e.Сfg.MaxAttachmentSizeBytes {
		return &errors.ErrorWithStatusCode{
			Message:    fmt.Sprintf("file too large: max %d bytes allowed", e.Сfg.MaxAttachmentSizeBytes),
			StatusCode: 400,
		}
	}

	return nil
}

// GenerateConfirmationCode generates a cryptographically secure random confirmation code
// using alphanumeric characters (excluding ambiguous ones like 0, O, I, 1)
func GenerateConfirmationCode(length int) string {
	// Charset excludes ambiguous characters: 0, O, I, 1
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

	b := make([]byte, length)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			// Fallback to a simpler random if crypto/rand fails (should never happen)
			panic(fmt.Sprintf("failed to generate random number: %v", err))
		}
		b[i] = charset[n.Int64()]
	}
	return string(b)
}

// GenerateThumbnail creates a thumbnail of the given image with a maximum size.
// It maintains the aspect ratio and returns a new image that fits within maxSize x maxSize.
func GenerateThumbnail(src image.Image, maxSize int) image.Image {
	bounds := src.Bounds()
	srcWidth := bounds.Dx()
	srcHeight := bounds.Dy()

	// Calculate new dimensions while maintaining aspect ratio
	var dstWidth, dstHeight int
	if srcWidth > srcHeight {
		dstWidth = maxSize
		dstHeight = (srcHeight * maxSize) / srcWidth
	} else {
		dstHeight = maxSize
		dstWidth = (srcWidth * maxSize) / srcHeight
	}

	// Create destination image
	dst := image.NewRGBA(image.Rect(0, 0, dstWidth, dstHeight))

	// Use BiLinear interpolation for good quality and performance balance
	draw.BiLinear.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)

	return dst
}
