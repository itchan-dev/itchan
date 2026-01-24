package pg

import (
	"fmt"

	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/itchan-dev/itchan/shared/utils"
	"github.com/lib/pq"
)

// MsgKey is a composite key for identifying a message within a board.
// Since message IDs are per-thread sequential, we need both thread_id and msg_id.
type MsgKey struct {
	ThreadId domain.ThreadId
	MsgId    domain.MsgId
}

// enrichMessagesWithReplies fetches and attaches reply data to messages.
// It queries message_replies table for the given board and message keys,
// and populates the Replies field of each message in the idToMessage map.
//
// This function is board-specific due to table partitioning.
// Call it once per board when enriching cross-board message lists.
func enrichMessagesWithReplies(
	q Querier,
	board domain.BoardShortName,
	messageKeys []MsgKey,
	idToMessage map[MsgKey]*domain.Message,
	messagesPerPage int,
) error {
	if len(messageKeys) == 0 {
		return nil // No messages to enrich
	}

	// Build arrays of thread_ids and msg_ids for the query
	threadIds := make([]int64, len(messageKeys))
	msgIds := make([]int64, len(messageKeys))
	for i, key := range messageKeys {
		threadIds[i] = int64(key.ThreadId)
		msgIds[i] = int64(key.MsgId)
	}

	// Query replies matching our message keys using JOIN with unnest for optimal performance
	rows, err := q.Query(`
		SELECT
			mr.sender_message_id,
			mr.sender_thread_id,
			mr.receiver_message_id,
			mr.receiver_thread_id,
			mr.created_at
		FROM message_replies mr
		JOIN unnest($2::bigint[], $3::bigint[]) AS keys(thread_id, msg_id)
		  ON mr.receiver_thread_id = keys.thread_id
		  AND mr.receiver_message_id = keys.msg_id
		WHERE mr.board = $1
		ORDER BY mr.created_at
	`, board, pq.Array(threadIds), pq.Array(msgIds))
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

		reply.Board = board
		// From (sender_message_id) is the per-thread sequential ID, which is also the ordinal
		reply.FromPage = utils.CalculatePage(int(reply.From), messagesPerPage)

		key := MsgKey{ThreadId: reply.ToThreadId, MsgId: reply.To}
		if msg, ok := idToMessage[key]; ok {
			msg.Replies = append(msg.Replies, &reply)
		}
	}

	return rows.Err()
}

// enrichMessagesWithAttachments fetches and attaches file attachments to messages.
// It queries attachments and files tables with JOIN for the given board and message keys,
// and populates the Attachments field of each message in the idToMessage map.
//
// This function is board-specific due to table partitioning.
// Call it once per board when enriching cross-board message lists.
func enrichMessagesWithAttachments(
	q Querier,
	board domain.BoardShortName,
	messageKeys []MsgKey,
	idToMessage map[MsgKey]*domain.Message,
) error {
	if len(messageKeys) == 0 {
		return nil // No messages to enrich
	}

	// Build arrays of thread_ids and msg_ids for the query
	threadIds := make([]int64, len(messageKeys))
	msgIds := make([]int64, len(messageKeys))
	for i, key := range messageKeys {
		threadIds[i] = int64(key.ThreadId)
		msgIds[i] = int64(key.MsgId)
	}

	// Query attachments matching our message keys using JOIN with unnest for optimal performance
	rows, err := q.Query(`
		SELECT
			a.id,
			a.board,
			a.thread_id,
			a.message_id,
			a.file_id,
			f.file_path,
			f.filename,
			f.original_filename,
			f.file_size_bytes,
			f.mime_type,
			f.original_mime_type,
			f.image_width,
			f.image_height,
			f.thumbnail_path
		FROM attachments a
		JOIN files f ON a.file_id = f.id
		JOIN unnest($2::bigint[], $3::bigint[]) AS keys(thread_id, msg_id)
		  ON a.thread_id = keys.thread_id
		  AND a.message_id = keys.msg_id
		WHERE a.board = $1
		ORDER BY a.id
	`, board, pq.Array(threadIds), pq.Array(msgIds))
	if err != nil {
		return fmt.Errorf("failed to fetch attachments for board %s: %w", board, err)
	}
	defer rows.Close()

	for rows.Next() {
		var attachment domain.Attachment
		var file domain.File
		if err := rows.Scan(
			&attachment.Id, &attachment.Board, &attachment.ThreadId, &attachment.MessageId, &attachment.FileId,
			&file.FilePath, &file.Filename, &file.OriginalFilename, &file.SizeBytes,
			&file.MimeType, &file.OriginalMimeType, &file.ImageWidth, &file.ImageHeight, &file.ThumbnailPath,
		); err != nil {
			return fmt.Errorf("failed to scan attachment row for board %s: %w", board, err)
		}
		attachment.File = &file
		key := MsgKey{ThreadId: attachment.ThreadId, MsgId: attachment.MessageId}
		if msg, ok := idToMessage[key]; ok {
			msg.Attachments = append(msg.Attachments, &attachment)
		}
	}

	return rows.Err()
}
