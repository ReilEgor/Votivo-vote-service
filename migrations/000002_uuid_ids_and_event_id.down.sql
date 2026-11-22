TRUNCATE participations, votes, vote_answers, outbox_messages RESTART IDENTITY CASCADE;

ALTER TABLE outbox_messages DROP COLUMN event_id;
ALTER TABLE outbox_messages RENAME COLUMN routing_key TO queue;

ALTER TABLE vote_answers   ALTER COLUMN option_id TYPE BIGINT USING 0;
ALTER TABLE vote_answers   ALTER COLUMN question_id TYPE BIGINT USING 0;
ALTER TABLE votes          ALTER COLUMN poll_id TYPE BIGINT USING 0;
ALTER TABLE participations ALTER COLUMN poll_id TYPE BIGINT USING 0;
