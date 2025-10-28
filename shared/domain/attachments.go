package domain

// File represents a file stored in the system
type File struct {
	Id               FileId
	FilePath         string
	OriginalFilename string
	FileSizeBytes    int64
	MimeType         string
	ImageWidth       *int
	ImageHeight      *int
}

// Attachment represents an attachment linking a message to a file
type Attachment struct {
	Id        AttachmentId
	Board     BoardShortName
	MessageId MsgId
	FileId    FileId
	File      *File // Optional: populated when fetching with file details
}

// Attachments is a slice of attachments
type Attachments = []*Attachment
