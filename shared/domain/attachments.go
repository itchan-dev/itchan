package domain

// File represents a file stored in the system
type File struct {
	Id               FileId  `json:"id,omitempty"`
	FilePath         string  `json:"file_path,omitempty"`
	OriginalFilename string  `json:"original_filename,omitempty"`
	FileSizeBytes    int64   `json:"file_size_bytes,omitempty"`
	MimeType         string  `json:"mime_type,omitempty"`
	OriginalMimeType *string `json:"original_mime_type,omitempty"`
	ImageWidth       *int    `json:"image_width,omitempty"`
	ImageHeight      *int    `json:"image_height,omitempty"`
	ThumbnailPath    *string `json:"thumbnail_path,omitempty"`
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
