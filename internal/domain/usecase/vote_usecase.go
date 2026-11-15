// Package usecase declares the business operation contracts of the vote service.
package usecase

import (
	"context"

	"github.com/ReilEgor/Votivo-vote-service/internal/domain/model"
)

// VoteUseCase takes part in polls on behalf of a user.
type VoteUseCase interface {
	// Cast accepts the first vote of the user in an active poll, or returns model.ErrAlreadyVoted.
	Cast(ctx context.Context, userID string, pollID int64, answers []model.Answer) error
	// Change replaces the vote of the user when the poll allows it.
	Change(ctx context.Context, userID string, pollID int64, answers []model.Answer) error
	// Revoke withdraws the vote of the user when the poll allows it.
	Revoke(ctx context.Context, userID string, pollID int64) error
	// Status tells whether the user has voted and, for a non-anonymous poll, what they chose.
	Status(ctx context.Context, userID string, pollID int64) (model.VoteStatus, error)
}
