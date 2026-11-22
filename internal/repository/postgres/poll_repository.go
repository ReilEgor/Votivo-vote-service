package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ReilEgor/Votivo-vote-service/internal/domain/model"
	sharedPostgres "github.com/ReilEgor/Votivo-vote-service/internal/infrastructure/storage/postgres"
)

const getPollQuery = `SELECT status, anonymous, allow_vote_change, questions FROM polls WHERE id = $1`

// On conflict the status is kept: a start event delivered again, or after the poll has ended, must not reopen it.
const savePollQuery = `INSERT INTO polls (id, status, anonymous, allow_vote_change, questions)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (id) DO UPDATE
SET anonymous         = EXCLUDED.anonymous,
    allow_vote_change = EXCLUDED.allow_vote_change,
    questions         = EXCLUDED.questions,
    updated_at        = now()`

// A poll.ended can overtake nothing on one queue, but the row is created when it does so the status is never lost.
const setPollStatusQuery = `INSERT INTO polls (id, status) VALUES ($1, $2)
ON CONFLICT (id) DO UPDATE SET status = EXCLUDED.status, updated_at = now()`

// storedQuestion is the JSON form of a question in polls.questions.
type storedQuestion struct {
	ID        string   `json:"id"`
	Multiple  bool     `json:"multiple"`
	OptionIDs []string `json:"option_ids"`
}

// PollRepository is the PostgreSQL repository.PollRepository.
type PollRepository struct{ pool *pgxpool.Pool }

// NewPollRepository returns a PollRepository on pool.
func NewPollRepository(pool *pgxpool.Pool) *PollRepository {
	return &PollRepository{pool: pool}
}

// GetPoll returns the poll, or model.ErrPollNotFound.
func (r *PollRepository) GetPoll(ctx context.Context, id string) (model.Poll, error) {
	const op = "PollRepository.GetPoll"

	poll := model.Poll{ID: id}
	var status string
	var raw []byte
	err := sharedPostgres.Executor(ctx, r.pool).
		QueryRow(ctx, getPollQuery, id).
		Scan(&status, &poll.Anonymous, &poll.AllowChange, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Poll{}, model.ErrPollNotFound
	}
	if err != nil {
		return model.Poll{}, fmt.Errorf("%s: %w", op, err)
	}
	poll.Status = model.PollStatus(status)

	var stored []storedQuestion
	if err = json.Unmarshal(raw, &stored); err != nil {
		return model.Poll{}, fmt.Errorf("%s: decode questions: %w", op, err)
	}
	for _, q := range stored {
		poll.Questions = append(poll.Questions, model.Question{ID: q.ID, Multiple: q.Multiple, OptionIDs: q.OptionIDs})
	}
	return poll, nil
}

// Save stores the rules and questions of a started poll.
func (r *PollRepository) Save(ctx context.Context, poll model.Poll) error {
	const op = "PollRepository.Save"

	stored := make([]storedQuestion, 0, len(poll.Questions))
	for _, q := range poll.Questions {
		stored = append(stored, storedQuestion{ID: q.ID, Multiple: q.Multiple, OptionIDs: q.OptionIDs})
	}
	raw, err := json.Marshal(stored)
	if err != nil {
		return fmt.Errorf("%s: encode questions: %w", op, err)
	}
	_, err = sharedPostgres.Executor(ctx, r.pool).
		Exec(ctx, savePollQuery, poll.ID, string(poll.Status), poll.Anonymous, poll.AllowChange, raw)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

// SetStatus moves the poll to the status.
func (r *PollRepository) SetStatus(ctx context.Context, id string, status model.PollStatus) error {
	const op = "PollRepository.SetStatus"

	if _, err := sharedPostgres.Executor(ctx, r.pool).Exec(ctx, setPollStatusQuery, id, string(status)); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}
