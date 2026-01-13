package pg

import (
	"context"
	"fmt"
	"time"

	"github.com/itchan-dev/itchan/shared/domain"
)

// GetUserMessages fetches user's last N messages across all boards.
// Returns FULLY enriched domain.Message objects with Author, Attachments, Replies.
func (s *Storage) GetUserMessages(userId domain.UserId, limit int) ([]domain.Message, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Step 1: Fetch messages with author data
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			m.id, m.board, m.thread_id, m.text, m.created_at, m.ordinal, m.is_op,
			u.id, u.email, u.is_admin
		FROM messages m
		JOIN users u ON m.author_id = u.id
		WHERE m.author_id = $1
		ORDER BY m.created_at DESC
		LIMIT $2
	`, userId, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user messages: %w", err)
	}
	defer rows.Close()

	// Build data structures for enrichment
	// First collect all messages (slice might reallocate during append)
	var messages []domain.Message

	for rows.Next() {
		var msg domain.Message
		var author domain.User
		err := rows.Scan(
			&msg.Id, &msg.Board, &msg.ThreadId, &msg.Text, &msg.CreatedAt, &msg.Ordinal, &msg.Op,
			&author.Id, &author.Email, &author.Admin,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user message: %w", err)
		}
		msg.Author = author
		msg.Replies = domain.Replies{}
		msg.Attachments = domain.Attachments{}

		messages = append(messages, msg)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating user messages: %w", err)
	}

	// Now build boardToMessages map with stable pointers (slice won't reallocate anymore)
	boardToMessages := make(map[domain.BoardShortName][]*domain.Message)
	for i := range messages {
		msgPtr := &messages[i]
		boardToMessages[msgPtr.Board] = append(boardToMessages[msgPtr.Board], msgPtr)
	}

	// Step 2: Enrich board-by-board with board-specific maps
	for board, boardMessages := range boardToMessages {
		// Build board-specific map to avoid ID collisions across boards
		idToMessage := make(map[domain.MsgId]*domain.Message)
		messageIds := make([]domain.MsgId, 0, len(boardMessages))

		for _, msg := range boardMessages {
			idToMessage[msg.Id] = msg
			messageIds = append(messageIds, msg.Id)
		}

		// Enrich with replies and attachments for this board only
		if err := enrichMessagesWithReplies(s.db, board, messageIds, idToMessage); err != nil {
			return nil, fmt.Errorf("failed to enrich replies for board %s: %w", board, err)
		}
		if err := enrichMessagesWithAttachments(s.db, board, messageIds, idToMessage); err != nil {
			return nil, fmt.Errorf("failed to enrich attachments for board %s: %w", board, err)
		}
	}

	// Return empty slice instead of nil
	if messages == nil {
		messages = []domain.Message{}
	}

	return messages, nil
}
