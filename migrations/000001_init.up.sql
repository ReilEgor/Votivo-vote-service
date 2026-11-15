-- Who took part in a poll. The unique pair is what stops a second vote, also under concurrent requests.
CREATE TABLE participations (
    id         BIGSERIAL    PRIMARY KEY,
    poll_id    BIGINT       NOT NULL,
    user_id    VARCHAR(128) NOT NULL,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT now(),
    UNIQUE (poll_id, user_id)
);

-- The ballots. user_id is NULL for an anonymous poll, so nothing here links a user to the options they chose.
CREATE TABLE votes (
    id         BIGSERIAL    PRIMARY KEY,
    poll_id    BIGINT       NOT NULL,
    user_id    VARCHAR(128),
    created_at TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_votes_poll_id_user_id ON votes (poll_id, user_id) WHERE user_id IS NOT NULL;

CREATE TABLE vote_answers (
    id          BIGSERIAL PRIMARY KEY,
    vote_id     BIGINT    NOT NULL REFERENCES votes (id) ON DELETE CASCADE,
    question_id BIGINT    NOT NULL,
    option_id   BIGINT    NOT NULL,
    UNIQUE (vote_id, question_id, option_id)
);

CREATE TABLE outbox_messages (
    id         BIGSERIAL    PRIMARY KEY,
    queue      VARCHAR(255) NOT NULL,
    payload    JSONB        NOT NULL,
    status     VARCHAR(20)  NOT NULL DEFAULT 'PENDING',
    attempts   INT          NOT NULL DEFAULT 0,
    last_error TEXT,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_outbox_messages_status ON outbox_messages (status);
