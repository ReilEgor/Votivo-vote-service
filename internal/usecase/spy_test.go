package usecase_test

import (
	"context"
	"slices"

	"github.com/ReilEgor/Votivo-vote-service/internal/domain/model"
)

// pollSpy returns the polls it was given.
type pollSpy struct {
	polls map[string]model.Poll
}

func (s *pollSpy) GetPoll(_ context.Context, id string) (model.Poll, error) {
	poll, ok := s.polls[id]
	if !ok {
		return model.Poll{}, model.ErrPollNotFound
	}
	return poll, nil
}

type voteKey struct {
	pollID string
	userID string
}

// voteSpy is an in-memory repository.VoteRepository. Anonymity is the business of the SQL, not of this spy.
type voteSpy struct {
	votes map[voteKey]model.Vote
}

func newVoteSpy() *voteSpy {
	return &voteSpy{votes: map[voteKey]model.Vote{}}
}

func (s *voteSpy) Create(_ context.Context, vote model.Vote) error {
	key := voteKey{vote.PollID, vote.UserID}
	if _, ok := s.votes[key]; ok {
		return model.ErrAlreadyVoted
	}
	s.votes[key] = vote
	return nil
}

func (s *voteSpy) Delete(_ context.Context, pollID, userID string) error {
	key := voteKey{pollID, userID}
	if _, ok := s.votes[key]; !ok {
		return model.ErrVoteNotFound
	}
	delete(s.votes, key)
	return nil
}

func (s *voteSpy) GetByUser(_ context.Context, pollID, userID string) (model.Vote, error) {
	vote, ok := s.votes[voteKey{pollID, userID}]
	if !ok {
		return model.Vote{}, model.ErrVoteNotFound
	}
	return vote, nil
}

func (s *voteSpy) HasVoted(_ context.Context, pollID, userID string) (bool, error) {
	_, ok := s.votes[voteKey{pollID, userID}]
	return ok, nil
}

// outboxSpy records the routing keys of the messages it received, in order.
type outboxSpy struct {
	queues []string
}

func (s *outboxSpy) Insert(_ context.Context, routingKey string, _ []byte) error {
	s.queues = append(s.queues, routingKey)
	return nil
}

func (*outboxSpy) FetchPending(context.Context, int) ([]model.OutboxMessage, error) { return nil, nil }
func (*outboxSpy) Delete(context.Context, int64) error                              { return nil }
func (*outboxSpy) Fail(context.Context, int64, int, string) error                   { return nil }

func (s *outboxSpy) sent() []string { return slices.Clone(s.queues) }

// txSpy runs the function without a transaction: the spies have nothing to roll back, so the tests check
// only that a failed operation leaves no message behind.
type txSpy struct{}

func (txSpy) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}
