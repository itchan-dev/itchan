package service

import (
	"image"
	"io"
)

// MediaStorage — интерфейс для хранения медиафайлов.
// Реализуется storage/fs.Storage (локальная ФС), в будущем — S3 и т.д.
type MediaStorage interface {
	// SaveFile stores a file's content.
	// It takes the board and thread IDs to construct the path and generates a unique filename.
	// It returns the relative path where the file was stored.
	SaveFile(fileData io.Reader, boardID, threadID, originalFilename string) (string, error)

	// SaveImage encodes and saves an image.Image (PNG if format="png", JPEG otherwise).
	// It returns the relative path where the image was stored and the file size in bytes.
	SaveImage(img image.Image, format, boardID, threadID, originalFilename string) (string, int64, error)

	// MoveFile moves a file from sourcePath to the storage location.
	// Used for sanitized videos to avoid loading into memory.
	// The source file will be deleted after successful move.
	// Returns the relative path where the file was stored.
	MoveFile(sourcePath, boardID, threadID, filename string) (string, error)

	// SaveThumbnail saves pre-encoded thumbnail bytes (e.g. JPEG from ffmpeg or Go encoder).
	// It returns the relative path where the thumbnail was stored.
	SaveThumbnail(data io.Reader, originalRelativePath string) (string, error)

	// Read opens a file for reading given its relative path.
	Read(filePath string) (io.ReadCloser, error)

	// DeleteFile removes a single file.
	DeleteFile(filePath string) error

	// DeleteThread removes all media for an entire thread.
	DeleteThread(boardID, threadID string) error

	// DeleteBoard removes all media for an entire board.
	DeleteBoard(boardID string) error
}
