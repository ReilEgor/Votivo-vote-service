package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ReilEgor/Votivo-vote-service/internal/domain/model"
	"github.com/ReilEgor/Votivo-vote-service/internal/domain/repository"
)

// PollEventUseCase implements usecase.PollEventUseCase.
type PollEventUseCase struct {
	polls repository.PollRepository
}

// NewPollEventUseCase returns a PollEventUseCase.
func NewPollEventUseCase(polls repository.PollRepository) *PollEventUseCase {
	return &PollEventUseCase{polls: polls}
}

// Start records the rules and questions of a poll that began accepting votes.
func (u *PollEventUseCase) Start(ctx context.Context, poll model.Poll) error {
	const op = "PollEventUseCase.Start"

	poll.Status = model.PollStatusActive
	if err := u.polls.Save(ctx, poll); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	slog.InfoContext(ctx, "poll started", slog.String("op", op), slog.String("poll_id", poll.ID))
	return nil
}

// End records that the poll stopped accepting votes.
func (u *PollEventUseCase) End(ctx context.Context, pollID string, status model.PollStatus) error {
	const op = "PollEventUseCase.End"

	if err := u.polls.SetStatus(ctx, pollID, status); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	slog.InfoContext(ctx, "poll ended",
		slog.String("op", op), slog.String("poll_id", pollID), slog.String("status", string(status)))
	return nil
}
