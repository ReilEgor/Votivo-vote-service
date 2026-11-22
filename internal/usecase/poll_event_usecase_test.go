package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ReilEgor/Votivo-vote-service/internal/domain/model"
	"github.com/ReilEgor/Votivo-vote-service/internal/usecase"
)

// pollStoreSpy keeps polls in memory and follows the contract of repository.PollRepository:
// Save leaves the status of an existing poll alone, SetStatus creates a missing one.
type pollStoreSpy struct {
	polls   map[string]model.Poll
	saveErr error
}

func newPollStoreSpy() *pollStoreSpy { return &pollStoreSpy{polls: map[string]model.Poll{}} }

func (s *pollStoreSpy) GetPoll(_ context.Context, id string) (model.Poll, error) {
	poll, ok := s.polls[id]
	if !ok {
		return model.Poll{}, model.ErrPollNotFound
	}
	return poll, nil
}

func (s *pollStoreSpy) Save(_ context.Context, poll model.Poll) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	if old, ok := s.polls[poll.ID]; ok {
		poll.Status = old.Status
	}
	s.polls[poll.ID] = poll
	return nil
}

func (s *pollStoreSpy) SetStatus(_ context.Context, id string, status model.PollStatus) error {
	poll := s.polls[id]
	poll.ID, poll.Status = id, status
	s.polls[id] = poll
	return nil
}

func startedPoll() model.Poll {
	return model.Poll{
		ID:          "poll-1",
		Anonymous:   true,
		AllowChange: true,
		Questions:   []model.Question{{ID: "q1", Multiple: true, OptionIDs: []string{"o1", "o2"}}},
	}
}

func TestPollEventUseCase_Start(t *testing.T) {
	t.Run("success - poll becomes active with its rules", func(t *testing.T) {
		store := newPollStoreSpy()

		err := usecase.NewPollEventUseCase(store).Start(t.Context(), startedPoll())

		require.NoError(t, err)
		got, err := store.GetPoll(t.Context(), "poll-1")
		require.NoError(t, err)
		assert.Equal(t, model.PollStatusActive, got.Status)
		assert.True(t, got.Anonymous)
		assert.True(t, got.AllowChange)
		assert.Equal(t, startedPoll().Questions, got.Questions)
	})

	t.Run("success - delivered twice, still one active poll", func(t *testing.T) {
		store := newPollStoreSpy()
		uc := usecase.NewPollEventUseCase(store)

		require.NoError(t, uc.Start(t.Context(), startedPoll()))
		require.NoError(t, uc.Start(t.Context(), startedPoll()))

		assert.Len(t, store.polls, 1)
	})

	t.Run("success - start delivered after the end does not reopen the poll", func(t *testing.T) {
		store := newPollStoreSpy()
		uc := usecase.NewPollEventUseCase(store)
		require.NoError(t, uc.Start(t.Context(), startedPoll()))
		require.NoError(t, uc.End(t.Context(), "poll-1", model.PollStatusClosed))

		require.NoError(t, uc.Start(t.Context(), startedPoll()))

		assert.Equal(t, model.PollStatusClosed, store.polls["poll-1"].Status)
	})

	t.Run("error - storage failure is returned so the message is redelivered", func(t *testing.T) {
		store := newPollStoreSpy()
		store.saveErr = errors.New("db down")

		err := usecase.NewPollEventUseCase(store).Start(t.Context(), startedPoll())

		require.ErrorIs(t, err, store.saveErr)
	})
}

func TestPollEventUseCase_End(t *testing.T) {
	tests := []struct {
		name   string
		status model.PollStatus
	}{
		{name: "success - closed", status: model.PollStatusClosed},
		{name: "success - blocked", status: model.PollStatusBlocked},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newPollStoreSpy()
			uc := usecase.NewPollEventUseCase(store)
			require.NoError(t, uc.Start(t.Context(), startedPoll()))

			err := uc.End(t.Context(), "poll-1", tt.status)

			require.NoError(t, err)
			assert.Equal(t, tt.status, store.polls["poll-1"].Status)
			assert.ErrorIs(t, store.polls["poll-1"].CheckActive(), model.ErrPollNotActive)
		})
	}

	t.Run("success - end of a poll never seen is recorded", func(t *testing.T) {
		store := newPollStoreSpy()

		err := usecase.NewPollEventUseCase(store).End(t.Context(), "poll-9", model.PollStatusBlocked)

		require.NoError(t, err)
		assert.Equal(t, model.PollStatusBlocked, store.polls["poll-9"].Status)
	})
}
