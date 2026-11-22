-- What Vote Service knows about a poll: the rules and the questions from the poll.started event, and the status
-- that poll.started and poll.ended move. Poll Service owns the poll; this table is only a copy of what voting needs.
CREATE TABLE polls (
    id                UUID        PRIMARY KEY,
    status            VARCHAR(16) NOT NULL,
    anonymous         BOOLEAN     NOT NULL DEFAULT FALSE,
    allow_vote_change BOOLEAN     NOT NULL DEFAULT FALSE,
    questions         JSONB       NOT NULL DEFAULT '[]',
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
