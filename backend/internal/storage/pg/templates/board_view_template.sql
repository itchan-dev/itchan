-- Create materialized view to store op message and several last replies
-- This is neccessary for fast access to board page, otherwise it would require complex queries every GetBoard request
-- This view store op message and several (value in config) last messages
CREATE MATERIALIZED VIEW %[1]s AS
	WITH data AS (
		SELECT
			t.title as thread_title,
			t.message_count as message_count,
			t.last_bumped_at as last_bumped_at,
			t.id as thread_id,
			m.id as msg_id,
			m.author_id as author_id,
			m.text as text,
			m.created_at as created_at,
			m.is_op as is_op,
			m.ordinal as ordinal
		FROM threads as t
		JOIN messages as m
			ON t.id = m.thread_id
			AND t.board = m.board
		WHERE 
		(m.is_op OR ((t.message_count - m.ordinal) < %[2]d)) -- op msg and last messages should be presented
		AND t.board = %[3]s
	)
	SELECT
		*
		,dense_rank() over(order by last_bumped_at desc, thread_id) as thread_order -- for pagination
	FROM data;
CREATE UNIQUE INDEX ON %[1]s (msg_id);
