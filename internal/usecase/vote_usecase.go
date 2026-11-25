// Package usecase implements the voting scenarios of the vote service.
package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/ReilEgor/Votivo-vote-service/internal/domain/model"
	"github.com/ReilEgor/Votivo-vote-service/internal/domain/repository"
	"github.com/ReilEgor/Votivo-vote-service/internal/domain/service"
	"github.com/ReilEgor/Votivo-vote-service/pkg/contracts"
	"github.com/ReilEgor/Votivo-vote-service/pkg/ctxlog"
)

// VoteUseCase implements usecase.VoteUseCase.
//
// Poll Service is called before the transaction opens: nothing slow runs while a database connection is held.
// Every change of votes and the event that reports it are written in one transaction (outbox).
type VoteUseCase struct {
	polls  service.PollProvider
	votes  repository.VoteRepository
	outbox repository.OutboxRepository
	tx     repository.Transactor
}

// NewVoteUseCase returns a VoteUseCase.
func NewVoteUseCase(
	polls service.PollProvider,
	votes repository.VoteRepository,
	outbox repository.OutboxRepository,
	tx repository.Transactor,
) *VoteUseCase {
	return &VoteUseCase{polls: polls, votes: votes, outbox: outbox, tx: tx}
}

// Cast accepts the first vote of the user in an active poll.
func (u *VoteUseCase) Cast(ctx context.Context, userID, pollID string, answers []model.Answer) error {
	const op = "VoteUseCase.Cast"

	slog.DebugContext(ctx, "called", slog.String("op", op), slog.String("poll_id", pollID))

	poll, err := u.activePoll(ctx, pollID)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	if err = poll.ValidateAnswers(answers); err != nil {
		return fmt.Errorf("%s: validate answers: %w", op, err)
	}

	vote := model.Vote{PollID: pollID, UserID: userID, Answers: answers, Anonymous: poll.Anonymous}
	err = u.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := poll.CheckActive(); err != nil {
			return err
		}
		if createErr := u.votes.Create(txCtx, vote); createErr != nil {
			return fmt.Errorf("create vote: %w", createErr)
		}
		return u.enqueueCast(txCtx, pollID, answers)
	})
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	slog.InfoContext(ctx, "vote cast", slog.String("op", op), slog.String("poll_id", pollID))
	return nil
}

// Change replaces the vote of the user: the old answers are revoked and the new ones cast in one transaction.
func (u *VoteUseCase) Change(ctx context.Context, userID, pollID string, answers []model.Answer) error {
	const op = "VoteUseCase.Change"

	slog.DebugContext(ctx, "called", slog.String("op", op), slog.String("poll_id", pollID))

	poll, err := u.changeablePoll(ctx, pollID)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	if err = poll.ValidateAnswers(answers); err != nil {
		return fmt.Errorf("%s: validate answers: %w", op, err)
	}

	vote := model.Vote{PollID: pollID, UserID: userID, Answers: answers}
	err = u.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := poll.CheckActive(); err != nil {
			return err
		}
		old, removeErr := u.remove(txCtx, pollID, userID)
		if removeErr != nil {
			return removeErr
		}
		if createErr := u.votes.Create(txCtx, vote); createErr != nil {
			return fmt.Errorf("create vote: %w", createErr)
		}
		if enqueueErr := u.enqueueRevoked(txCtx, pollID, old.Answers); enqueueErr != nil {
			return enqueueErr
		}
		return u.enqueueCast(txCtx, pollID, answers)
	})
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	slog.InfoContext(ctx, "vote changed", slog.String("op", op), slog.String("poll_id", pollID))
	return nil
}

// Revoke withdraws the vote of the user.
func (u *VoteUseCase) Revoke(ctx context.Context, userID, pollID string) error {
	const op = "VoteUseCase.Revoke"

	slog.DebugContext(ctx, "called", slog.String("op", op), slog.String("poll_id", pollID))

	poll, err := u.changeablePoll(ctx, pollID)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	err = u.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := poll.CheckActive(); err != nil {
			return err
		}
		old, removeErr := u.remove(txCtx, pollID, userID)
		if removeErr != nil {
			return removeErr
		}
		return u.enqueueRevoked(txCtx, pollID, old.Answers)
	})
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	slog.InfoContext(ctx, "vote revoked", slog.String("op", op), slog.String("poll_id", pollID))
	return nil
}

// Status tells whether the user has voted. It works for a poll in any state, so a user can still see
// "you have voted" after the poll is closed; the answers are returned only for a non-anonymous poll.
func (u *VoteUseCase) Status(ctx context.Context, userID, pollID string) (model.VoteStatus, error) {
	const op = "VoteUseCase.Status"

	slog.DebugContext(ctx, "called", slog.String("op", op), slog.String("poll_id", pollID))

	poll, err := u.polls.GetPoll(ctx, pollID)
	if err != nil {
		return model.VoteStatus{}, fmt.Errorf("%s: get poll: %w", op, err)
	}
	voted, err := u.votes.HasVoted(ctx, pollID, userID)
	if err != nil {
		return model.VoteStatus{}, fmt.Errorf("%s: check participation: %w", op, err)
	}
	if !voted || poll.Anonymous {
		return model.VoteStatus{Voted: voted}, nil
	}
	vote, err := u.votes.GetByUser(ctx, pollID, userID)
	if err != nil {
		return model.VoteStatus{}, fmt.Errorf("%s: get vote: %w", op, err)
	}
	return model.VoteStatus{Voted: true, Answers: vote.Answers}, nil
}

func (u *VoteUseCase) activePoll(ctx context.Context, pollID string) (model.Poll, error) {
	poll, err := u.polls.GetPoll(ctx, pollID)
	if err != nil {
		return model.Poll{}, fmt.Errorf("get poll: %w", err)
	}
	if err = poll.CheckActive(); err != nil {
		return model.Poll{}, fmt.Errorf("check poll: %w", err)
	}
	return poll, nil
}

func (u *VoteUseCase) changeablePoll(ctx context.Context, pollID string) (model.Poll, error) {
	poll, err := u.activePoll(ctx, pollID)
	if err != nil {
		return model.Poll{}, err
	}
	if err = poll.CheckChangeable(); err != nil {
		return model.Poll{}, fmt.Errorf("check poll: %w", err)
	}
	return poll, nil
}

// remove reads the ballot of the user and deletes it. A concurrent revoke finds nothing to delete and fails
// with model.ErrVoteNotFound, so one vote is never revoked twice.
func (u *VoteUseCase) remove(ctx context.Context, pollID, userID string) (model.Vote, error) {
	old, err := u.votes.GetByUser(ctx, pollID, userID)
	if err != nil {
		return model.Vote{}, fmt.Errorf("get vote: %w", err)
	}
	if err = u.votes.Delete(ctx, pollID, userID); err != nil {
		return model.Vote{}, fmt.Errorf("delete vote: %w", err)
	}
	return old, nil
}

func (u *VoteUseCase) enqueueCast(ctx context.Context, pollID string, answers []model.Answer) error {
	return u.enqueue(ctx, contracts.RoutingKeyVoteCast, contracts.VoteCastEvent{
		PollID:    pollID,
		Answers:   toContract(answers),
		RequestID: ctxlog.RequestID(ctx),
	})
}

func (u *VoteUseCase) enqueueRevoked(ctx context.Context, pollID string, answers []model.Answer) error {
	return u.enqueue(ctx, contracts.RoutingKeyVoteRevoked, contracts.VoteRevokedEvent{
		PollID:    pollID,
		Answers:   toContract(answers),
		RequestID: ctxlog.RequestID(ctx),
	})
}

func (u *VoteUseCase) enqueue(ctx context.Context, routingKey string, event any) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal %s event: %w", routingKey, err)
	}
	if err = u.outbox.Insert(ctx, routingKey, payload); err != nil {
		return fmt.Errorf("enqueue %s event: %w", routingKey, err)
	}
	return nil
}

func toContract(answers []model.Answer) []contracts.VoteAnswer {
	out := make([]contracts.VoteAnswer, 0, len(answers))
	for _, a := range answers {
		out = append(out, contracts.VoteAnswer{QuestionID: a.QuestionID, OptionIDs: a.OptionIDs})
	}
	return out
}
