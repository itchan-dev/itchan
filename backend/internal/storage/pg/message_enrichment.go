package pg

import (
	"fmt"

	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/lib/pq"
)

// enrichMessagesWithReplies fetches and attaches reply data to messages.
// It queries message_replies table for the given board and message IDs,
// and populates the Replies field of each message in the idToMessage map.
//
// This function is board-specific due to table partitioning.
// Call it once per board when enriching cross-board message lists.
func enrichMessagesWithReplies(
	q Querier,
	board domain.BoardShortName,
	messageIds []domain.MsgId,
	idToMessage map[domain.MsgId]*domain.Message,
) error {
	if len(messageIds) == 0 {
		return nil // No messages to enrich
	}

	rows, err := q.Query(`
		SELECT
			sender_message_id,
			sender_thread_id,
			receiver_message_id,
			receiver_thread_id,
			created_at
		FROM message_replies
		WHERE board = $1 AND receiver_message_id = ANY($2)
		ORDER BY created_at
	`, board, pq.Array(messageIds))
	if err != nil {
		return fmt.Errorf("failed to fetch replies for board %s: %w", board, err)
	}
	defer rows.Close()

	for rows.Next() {
		var reply domain.Reply
		if err := rows.Scan(
			&reply.From,
			&reply.FromThreadId,
			&reply.To,
			&reply.ToThreadId,
			&reply.CreatedAt,
		); err != nil {
			return fmt.Errorf("failed to scan reply row for board %s: %w", board, err)
		}

		reply.Board = board // Set the board field

		if msg, ok := idToMessage[reply.To]; ok {
			msg.Replies = append(msg.Replies, &reply)
		}
	}

	return rows.Err()
}

// enrichMessagesWithAttachments fetches and attaches file attachments to messages.
// It queries attachments and files tables with JOIN for the given board and message IDs,
// and populates the Attachments field of each message in the idToMessage map.
//
// This function is board-specific due to table partitioning.
// Call it once per board when enriching cross-board message lists.
func enrichMessagesWithAttachments(
	q Querier,
	board domain.BoardShortName,
	messageIds []domain.MsgId,
	idToMessage map[domain.MsgId]*domain.Message,
) error {
	if len(messageIds) == 0 {
		return nil // No messages to enrich
	}

	rows, err := q.Query(`
		SELECT a.id, a.board, a.message_id, a.file_id,
		       f.file_path, f.filename, f.original_filename, f.file_size_bytes,
		       f.mime_type, f.original_mime_type, f.image_width, f.image_height, f.thumbnail_path
		FROM attachments a
		JOIN files f ON a.file_id = f.id
		WHERE a.board = $1 AND a.message_id = ANY($2)
		ORDER BY a.id
	`, board, pq.Array(messageIds))
	if err != nil {
		return fmt.Errorf("failed to fetch attachments for board %s: %w", board, err)
	}
	defer rows.Close()

	for rows.Next() {
		var attachment domain.Attachment
		var file domain.File
		if err := rows.Scan(
			&attachment.Id, &attachment.Board, &attachment.MessageId, &attachment.FileId,
			&file.FilePath, &file.Filename, &file.OriginalFilename, &file.SizeBytes,
			&file.MimeType, &file.OriginalMimeType, &file.ImageWidth, &file.ImageHeight, &file.ThumbnailPath,
		); err != nil {
			return fmt.Errorf("failed to scan attachment row for board %s: %w", board, err)
		}
		attachment.File = &file
		if msg, ok := idToMessage[attachment.MessageId]; ok {
			msg.Attachments = append(msg.Attachments, &attachment)
		}
	}

	return rows.Err()
}
