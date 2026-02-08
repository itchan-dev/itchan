-- Create materialized view to store op message and several last replies
-- This is neccessary for fast access to board page, otherwise it would require complex queries every GetBoard request
-- This view stores op message (id=1) and several (value in config) last messages
CREATE MATERIALIZED VIEW %[1]s AS
	WITH data AS (
		SELECT
			t.title as thread_title,
			t.message_count as message_count,
			t.last_bumped_at as last_bumped_at,
			t.id as thread_id,
			t.is_pinned as is_pinned,
			m.id as msg_id,
			m.author_id as author_id,
			u.email_domain as email_domain,
			u.is_admin as author_is_admin,
			m.show_email_domain as show_email_domain,
			m.text as text,
			m.created_at as created_at
		FROM threads as t
		JOIN messages as m
			ON t.id = m.thread_id
			AND t.board = m.board
		JOIN users u ON m.author_id = u.id
		WHERE
		(m.id = 1 OR ((t.message_count - m.id) < %[2]d)) -- op msg (id=1) and last messages should be presented
		AND t.board = %[3]s
	)
	SELECT
		*
		,dense_rank() over(order by is_pinned desc, last_bumped_at desc, thread_id) as thread_order -- pinned first, then by bump time
	FROM data;
CREATE UNIQUE INDEX ON %[1]s (thread_id, msg_id);
