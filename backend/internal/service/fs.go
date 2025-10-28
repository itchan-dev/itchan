package service

import "io"

type MediaStorage interface {
	// Save stores a file's content for a given message.
	// It takes the board and thread IDs to construct the path.
	// It returns the relative path where the file was stored.
	Save(fileData io.Reader, boardID, threadID, messageID, originalExtension string) (string, error)

	// Read opens a file for reading given its relative path.
	Read(filePath string) (io.ReadCloser, error)

	// DeleteFile removes a single file.
	DeleteFile(filePath string) error

	// DeleteThread removes all media for an entire thread.
	DeleteThread(boardID, threadID string) error

	// DeleteBoard removes all media for an entire board.
	DeleteBoard(boardID string) error
}
