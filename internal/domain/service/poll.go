// Package service declares the contracts of the services this one depends on.
package service

import (
	"context"

	"github.com/ReilEgor/Votivo-vote-service/internal/domain/model"
)

// PollProvider returns the voting rules of a poll from Poll Service.
type PollProvider interface {
	// GetPoll returns the poll, model.ErrPollNotFound when it does not exist,
	// or model.ErrPollServiceUnavailable when Poll Service cannot answer.
	GetPoll(ctx context.Context, id int64) (model.Poll, error)
}
