package usecase_test

import (
	"github.com/ReilEgor/Votivo-vote-service/internal/domain/model"
	"github.com/ReilEgor/Votivo-vote-service/internal/usecase"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestExpiredPollRejectsAllVoteMutationsBeforeEndEvent(t *testing.T) {
	deadline := time.Now().Add(-time.Second)
	polls := &pollSpy{polls: map[string]model.Poll{
		pollOpen: {ID: pollOpen, Status: model.PollStatusActive, AllowChange: true, EndsAt: &deadline},
	}}
	votes, outbox := newVoteSpy(), &outboxSpy{}
	uc := usecase.NewVoteUseCase(polls, votes, outbox, txSpy{})
	require.ErrorIs(t, uc.Cast(t.Context(), userID, pollOpen, answer("o10")), model.ErrPollNotActive)
	require.ErrorIs(t, uc.Change(t.Context(), userID, pollOpen, answer("o10")), model.ErrPollNotActive)
	require.ErrorIs(t, uc.Revoke(t.Context(), userID, pollOpen), model.ErrPollNotActive)
	require.Empty(t, votes.votes)
	require.Empty(t, outbox.queues)
}
