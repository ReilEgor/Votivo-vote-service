// Package postgres implements the repositories of the vote service on PostgreSQL.
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ReilEgor/Votivo-vote-service/internal/domain/model"
	sharedPostgres "github.com/ReilEgor/Votivo-vote-service/internal/infrastructure/storage/postgres"
)

// uniqueViolation is the PostgreSQL error code of a violated unique constraint.
const uniqueViolation = "23505"

const createParticipationQuery = `INSERT INTO participations (poll_id, user_id) VALUES ($1, $2)`

// The ballot keeps the user only when the poll is not anonymous: a NULL user_id breaks the link to the answers.
const createVoteQuery = `INSERT INTO votes (poll_id, user_id) VALUES ($1, $2) RETURNING id`

const createAnswersQuery = `INSERT INTO vote_answers (vote_id, question_id, option_id)
SELECT $1, question_id, option_id FROM unnest($2::bigint[], $3::bigint[]) AS a (question_id, option_id)`

const hasVotedQuery = `SELECT EXISTS (SELECT 1 FROM participations WHERE poll_id = $1 AND user_id = $2)`

const getAnswersByUserQuery = `SELECT a.question_id, a.option_id
FROM vote_answers a
JOIN votes v ON v.id = a.vote_id
WHERE v.poll_id = $1 AND v.user_id = $2
ORDER BY a.question_id, a.option_id`

const deleteVoteQuery = `DELETE FROM votes WHERE poll_id = $1 AND user_id = $2`

const deleteParticipationQuery = `DELETE FROM participations WHERE poll_id = $1 AND user_id = $2`

// VoteRepository is the PostgreSQL repository.VoteRepository.
type VoteRepository struct{ pool *pgxpool.Pool }

// NewVoteRepository returns a VoteRepository on pool.
func NewVoteRepository(pool *pgxpool.Pool) *VoteRepository {
	return &VoteRepository{pool: pool}
}

// Create stores the participation and the ballot, or returns model.ErrAlreadyVoted.
func (r *VoteRepository) Create(ctx context.Context, vote model.Vote) error {
	const op = "VoteRepository.Create"

	db := sharedPostgres.Executor(ctx, r.pool)

	if _, err := db.Exec(ctx, createParticipationQuery, vote.PollID, vote.UserID); err != nil {
		if isUniqueViolation(err) {
			return model.ErrAlreadyVoted
		}
		return fmt.Errorf("%s: create participation: %w", op, err)
	}

	var ballotUser *string
	if !vote.Anonymous {
		ballotUser = &vote.UserID
	}
	var voteID int64
	if err := db.QueryRow(ctx, createVoteQuery, vote.PollID, ballotUser).Scan(&voteID); err != nil {
		return fmt.Errorf("%s: create vote: %w", op, err)
	}

	questionIDs, optionIDs := flatten(vote.Answers)
	if _, err := db.Exec(ctx, createAnswersQuery, voteID, questionIDs, optionIDs); err != nil {
		return fmt.Errorf("%s: create answers: %w", op, err)
	}
	return nil
}

// HasVoted reports whether the user has taken part in the poll.
func (r *VoteRepository) HasVoted(ctx context.Context, pollID int64, userID string) (bool, error) {
	const op = "VoteRepository.HasVoted"

	var voted bool
	if err := sharedPostgres.Executor(ctx, r.pool).
		QueryRow(ctx, hasVotedQuery, pollID, userID).
		Scan(&voted); err != nil {
		return false, fmt.Errorf("%s: %w", op, err)
	}
	return voted, nil
}

// GetByUser returns the ballot of the user, or model.ErrVoteNotFound.
func (r *VoteRepository) GetByUser(ctx context.Context, pollID int64, userID string) (model.Vote, error) {
	const op = "VoteRepository.GetByUser"

	rows, err := sharedPostgres.Executor(ctx, r.pool).Query(ctx, getAnswersByUserQuery, pollID, userID)
	if err != nil {
		return model.Vote{}, fmt.Errorf("%s: query: %w", op, err)
	}
	defer rows.Close()

	var answers []model.Answer
	for rows.Next() {
		var questionID, optionID int64
		if err = rows.Scan(&questionID, &optionID); err != nil {
			return model.Vote{}, fmt.Errorf("%s: scan: %w", op, err)
		}
		answers = appendOption(answers, questionID, optionID)
	}
	if err = rows.Err(); err != nil {
		return model.Vote{}, fmt.Errorf("%s: rows: %w", op, err)
	}
	if len(answers) == 0 {
		return model.Vote{}, model.ErrVoteNotFound
	}
	return model.Vote{PollID: pollID, UserID: userID, Answers: answers}, nil
}

// Delete removes the participation and the ballot, or returns model.ErrVoteNotFound.
func (r *VoteRepository) Delete(ctx context.Context, pollID int64, userID string) error {
	const op = "VoteRepository.Delete"

	db := sharedPostgres.Executor(ctx, r.pool)

	if _, err := db.Exec(ctx, deleteVoteQuery, pollID, userID); err != nil {
		return fmt.Errorf("%s: delete vote: %w", op, err)
	}
	tag, err := db.Exec(ctx, deleteParticipationQuery, pollID, userID)
	if err != nil {
		return fmt.Errorf("%s: delete participation: %w", op, err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrVoteNotFound
	}
	return nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == uniqueViolation
}

// flatten turns the answers into two parallel slices, one entry per chosen option, for unnest.
func flatten(answers []model.Answer) (questionIDs, optionIDs []int64) {
	for _, a := range answers {
		for _, optionID := range a.OptionIDs {
			questionIDs = append(questionIDs, a.QuestionID)
			optionIDs = append(optionIDs, optionID)
		}
	}
	return questionIDs, optionIDs
}

// appendOption adds the option to the answer of its question; rows arrive ordered by question.
func appendOption(answers []model.Answer, questionID, optionID int64) []model.Answer {
	if n := len(answers); n > 0 && answers[n-1].QuestionID == questionID {
		answers[n-1].OptionIDs = append(answers[n-1].OptionIDs, optionID)
		return answers
	}
	return append(answers, model.Answer{QuestionID: questionID, OptionIDs: []int64{optionID}})
}
