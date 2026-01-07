package domain

import "io"

// FileCommonMetadata contains common file metadata fields shared between
// PendingFile (uploaded) and File (stored). Following the MessageMetadata pattern.
type FileCommonMetadata struct {
	Filename    string `json:"filename,omitempty"`
	SizeBytes   int64  `json:"size_bytes,omitempty"`
	MimeType    string `json:"mime_type,omitempty"`
	ImageWidth  *int   `json:"image_width,omitempty"`
	ImageHeight *int   `json:"image_height,omitempty"`
}

// PendingFile represents a file upload being processed (moved from domain/message.go)
type PendingFile struct {
	FileCommonMetadata
	Data io.Reader `json:"-"`
}

// SanitizedImage represents a sanitized image file ready to be saved.
// Images are decoded in-memory (metadata stripped) and ready for encoding/thumbnailing.
type SanitizedImage struct {
	FileCommonMetadata
	Image  any    // Decoded image.Image (any to avoid image import here)
	Format string // Image format from image.Decode ("png", "jpeg", "gif")
}

// SanitizedVideo represents a sanitized video file ready to be moved.
// Videos are processed to disk to avoid memory overhead and moved via MoveFile.
type SanitizedVideo struct {
	FileCommonMetadata
	TempFilePath string // Path to sanitized video on disk (always present)
}

// File represents a file stored in the system
type File struct {
	FileCommonMetadata                             // Sanitized file metadata
	Id               FileId `json:"id,omitempty"` // Database ID
	FilePath         string `json:"file_path,omitempty"` // Full path on disk
	OriginalFilename string `json:"original_filename,omitempty"` // User's uploaded filename (before sanitization)
	OriginalMimeType string `json:"original_mime_type,omitempty"` // MIME type before sanitization (always present)
	ThumbnailPath    *string `json:"thumbnail_path,omitempty"` // Path to generated thumbnail (images only)
}

// Attachment represents an attachment linking a message to a file
type Attachment struct {
	Id        AttachmentId   `json:"id,omitempty"`
	Board     BoardShortName `json:"board,omitempty"`
	MessageId MsgId          `json:"message_id,omitempty"`
	FileId    FileId         `json:"file_id,omitempty"`
	File      *File          `json:"file,omitempty"` // Optional: populated when fetching with file details
}

// Attachments is a slice of attachments
type Attachments = []*Attachment
