package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ReilEgor/Votivo-vote-service/internal/domain/model"
	sharedPostgres "github.com/ReilEgor/Votivo-vote-service/internal/infrastructure/storage/postgres"
)

const insertOutboxQuery = `INSERT INTO outbox_messages (queue, payload) VALUES ($1, $2)`

// FOR UPDATE SKIP LOCKED lets several relays run side by side without publishing a message twice.
const fetchPendingOutboxQuery = `SELECT id, queue, payload
FROM outbox_messages
WHERE status = 'PENDING'
ORDER BY id
LIMIT $1
FOR UPDATE SKIP LOCKED`

const deleteOutboxQuery = `DELETE FROM outbox_messages WHERE id = $1`

const failOutboxQuery = `UPDATE outbox_messages
SET attempts   = attempts + 1,
    last_error = $3,
    status     = CASE WHEN attempts + 1 >= $2 THEN 'FAILED' ELSE 'PENDING' END
WHERE id = $1`

// OutboxRepository is the PostgreSQL repository.OutboxRepository.
type OutboxRepository struct{ pool *pgxpool.Pool }

// NewOutboxRepository returns an OutboxRepository on pool.
func NewOutboxRepository(pool *pgxpool.Pool) *OutboxRepository {
	return &OutboxRepository{pool: pool}
}

// Insert adds a message in the transaction carried by ctx.
func (r *OutboxRepository) Insert(ctx context.Context, queue string, payload []byte) error {
	const op = "OutboxRepository.Insert"

	if _, err := sharedPostgres.Executor(ctx, r.pool).Exec(ctx, insertOutboxQuery, queue, payload); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

// FetchPending locks and returns up to limit messages that are waiting to be published.
func (r *OutboxRepository) FetchPending(ctx context.Context, limit int) ([]model.OutboxMessage, error) {
	const op = "OutboxRepository.FetchPending"

	rows, err := sharedPostgres.Executor(ctx, r.pool).Query(ctx, fetchPendingOutboxQuery, limit)
	if err != nil {
		return nil, fmt.Errorf("%s: query: %w", op, err)
	}
	defer rows.Close()

	var msgs []model.OutboxMessage
	for rows.Next() {
		var msg model.OutboxMessage
		if err = rows.Scan(&msg.ID, &msg.Queue, &msg.Payload); err != nil {
			return nil, fmt.Errorf("%s: scan: %w", op, err)
		}
		msgs = append(msgs, msg)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: rows: %w", op, err)
	}
	return msgs, nil
}

// Delete removes a published message.
func (r *OutboxRepository) Delete(ctx context.Context, id int64) error {
	const op = "OutboxRepository.Delete"

	if _, err := sharedPostgres.Executor(ctx, r.pool).Exec(ctx, deleteOutboxQuery, id); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

// Fail counts a failed attempt and marks the message as failed once maxAttempts is reached.
func (r *OutboxRepository) Fail(ctx context.Context, id int64, maxAttempts int, lastErr string) error {
	const op = "OutboxRepository.Fail"

	if _, err := sharedPostgres.Executor(ctx, r.pool).Exec(ctx, failOutboxQuery, id, maxAttempts, lastErr); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}
