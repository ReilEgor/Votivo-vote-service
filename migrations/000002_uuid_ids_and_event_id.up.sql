-- Poll Service identifies polls, questions and options with UUIDs, so Vote Service stores them as UUIDs too.
-- Nothing refers to the old numeric IDs yet: the tables are emptied before their columns change type.
TRUNCATE participations, votes, vote_answers, outbox_messages RESTART IDENTITY CASCADE;

ALTER TABLE participations ALTER COLUMN poll_id TYPE UUID USING gen_random_uuid();
ALTER TABLE votes          ALTER COLUMN poll_id TYPE UUID USING gen_random_uuid();
ALTER TABLE vote_answers   ALTER COLUMN question_id TYPE UUID USING gen_random_uuid();
ALTER TABLE vote_answers   ALTER COLUMN option_id TYPE UUID USING gen_random_uuid();

-- Events go to a topic exchange: the column holds the routing key, and event_id becomes the AMQP message_id.
ALTER TABLE outbox_messages RENAME COLUMN queue TO routing_key;
ALTER TABLE outbox_messages ADD COLUMN event_id UUID NOT NULL DEFAULT gen_random_uuid();
