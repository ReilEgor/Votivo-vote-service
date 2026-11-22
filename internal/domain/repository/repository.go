// Package repository declares the data access contracts of the vote service.
package repository

import (
	"context"

	"github.com/ReilEgor/Votivo-vote-service/internal/domain/model"
)

// VoteReader reads votes.
type VoteReader interface {
	// GetByUser returns the ballot of the user, or model.ErrVoteNotFound when there is none.
	GetByUser(ctx context.Context, pollID, userID string) (model.Vote, error)
	// HasVoted reports whether the user has taken part in the poll.
	HasVoted(ctx context.Context, pollID, userID string) (bool, error)
}

// VoteWriter stores and removes votes. Both methods expect to run inside a transaction.
type VoteWriter interface {
	// Create stores the participation of the user and the ballot, or returns model.ErrAlreadyVoted
	// when the user has already taken part in the poll. An anonymous ballot is stored without the user.
	Create(ctx context.Context, vote model.Vote) error
	// Delete removes the participation and the ballot of the user, or returns model.ErrVoteNotFound.
	Delete(ctx context.Context, pollID, userID string) error
}

// VoteRepository is the full access to votes.
type VoteRepository interface {
	VoteReader
	VoteWriter
}

// PollRepository keeps the copy of the polls Vote Service learns from Poll Service events.
type PollRepository interface {
	// GetPoll returns the poll, or model.ErrPollNotFound when no event about it has arrived.
	GetPoll(ctx context.Context, id string) (model.Poll, error)
	// Save stores the rules and questions of a started poll. The status of a poll that has already ended stays,
	// so a redelivered start event does not reopen it.
	Save(ctx context.Context, poll model.Poll) error
	// SetStatus moves the poll to the status, also when the start event has not arrived yet.
	SetStatus(ctx context.Context, id string, status model.PollStatus) error
}

// OutboxRepository stores the messages that must reach the broker.
type OutboxRepository interface {
	// Insert adds a message in the transaction carried by ctx.
	Insert(ctx context.Context, routingKey string, payload []byte) error
	// FetchPending locks and returns up to limit messages that are waiting to be published.
	FetchPending(ctx context.Context, limit int) ([]model.OutboxMessage, error)
	// Delete removes a published message.
	Delete(ctx context.Context, id int64) error
	// Fail counts a failed attempt and marks the message as failed once maxAttempts is reached.
	Fail(ctx context.Context, id int64, maxAttempts int, lastErr string) error
}

// Transactor runs a function in a database transaction.
type Transactor interface {
	// WithinTransaction commits when fn returns nil and rolls back otherwise.
	WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}
