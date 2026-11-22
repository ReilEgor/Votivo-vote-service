package usecase_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ReilEgor/Votivo-vote-service/internal/domain/model"
	"github.com/ReilEgor/Votivo-vote-service/internal/usecase"
	"github.com/ReilEgor/Votivo-vote-service/pkg/contracts"
)

const (
	pollOpen      = "poll-open"
	pollFinal     = "poll-final"
	pollAnonymous = "poll-anonymous"
	pollClosed    = "poll-closed"
	userID        = "user-1"
)

type fixture struct {
	uc     *usecase.VoteUseCase
	votes  *voteSpy
	outbox *outboxSpy
}

func newFixture() fixture {
	questions := []model.Question{{ID: "q1", Multiple: false, OptionIDs: []string{"o10", "o11"}}}
	polls := &pollSpy{polls: map[string]model.Poll{
		pollOpen:      {ID: pollOpen, Status: model.PollStatusActive, AllowChange: true, Questions: questions},
		pollFinal:     {ID: pollFinal, Status: model.PollStatusActive, Questions: questions},
		pollAnonymous: {ID: pollAnonymous, Status: model.PollStatusActive, Anonymous: true, Questions: questions},
		pollClosed:    {ID: pollClosed, Status: model.PollStatusClosed, AllowChange: true, Questions: questions},
	}}
	votes, outbox := newVoteSpy(), &outboxSpy{}
	return fixture{uc: usecase.NewVoteUseCase(polls, votes, outbox, txSpy{}), votes: votes, outbox: outbox}
}

func answer(option string) []model.Answer {
	return []model.Answer{{QuestionID: "q1", OptionIDs: []string{option}}}
}

func TestVoteUseCase_Cast(t *testing.T) {
	tests := []struct {
		name     string
		pollID   string
		answers  []model.Answer
		prepare  func(t *testing.T, f fixture)
		wantErr  error
		wantKeys []string
	}{
		{
			name:     "success - vote stored and event queued",
			pollID:   pollOpen,
			answers:  answer("o10"),
			wantKeys: []string{contracts.RoutingKeyVoteCast},
		},
		{
			name:    "error - already voted",
			pollID:  pollOpen,
			answers: answer("o11"),
			prepare: func(t *testing.T, f fixture) {
				t.Helper()
				require.NoError(t, f.uc.Cast(t.Context(), userID, pollOpen, answer("o10")))
			},
			wantErr:  model.ErrAlreadyVoted,
			wantKeys: []string{contracts.RoutingKeyVoteCast},
		},
		{
			name:    "error - poll not found",
			pollID:  "poll-missing",
			answers: answer("o10"),
			wantErr: model.ErrPollNotFound,
		},
		{name: "error - poll closed", pollID: pollClosed, answers: answer("o10"), wantErr: model.ErrPollNotActive},
		{name: "error - unknown option", pollID: pollOpen, answers: answer("o99"), wantErr: model.ErrUnknownOption},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			if tt.prepare != nil {
				tt.prepare(t, f)
			}

			err := f.uc.Cast(t.Context(), userID, tt.pollID, tt.answers)

			require.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, tt.wantKeys, f.outbox.sent())
		})
	}
}

func TestVoteUseCase_Change(t *testing.T) {
	tests := []struct {
		name      string
		pollID    string
		voteFirst bool
		wantErr   error
		wantKeys  []string
	}{
		{
			name:      "success - old answers revoked and new ones cast",
			pollID:    pollOpen,
			voteFirst: true,
			wantKeys: []string{
				contracts.RoutingKeyVoteCast, contracts.RoutingKeyVoteRevoked, contracts.RoutingKeyVoteCast,
			},
		},
		{
			name:      "error - poll does not allow change",
			pollID:    pollFinal,
			voteFirst: true,
			wantErr:   model.ErrVoteNotChangeable,
		},
		{name: "error - anonymous poll", pollID: pollAnonymous, voteFirst: true, wantErr: model.ErrVoteNotChangeable},
		{name: "error - poll closed", pollID: pollClosed, wantErr: model.ErrPollNotActive},
		{name: "error - nothing to change", pollID: pollOpen, wantErr: model.ErrVoteNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			if tt.voteFirst {
				require.NoError(t, f.uc.Cast(t.Context(), userID, tt.pollID, answer("o10")))
			}
			before := len(f.outbox.sent())

			err := f.uc.Change(t.Context(), userID, tt.pollID, answer("o11"))

			require.ErrorIs(t, err, tt.wantErr)
			if tt.wantErr == nil {
				assert.Equal(t, tt.wantKeys, f.outbox.sent())
				return
			}
			assert.Len(t, f.outbox.sent(), before, "a failed change must not queue events")
		})
	}
}

func TestVoteUseCase_Revoke(t *testing.T) {
	t.Run("success - vote removed and event queued", func(t *testing.T) {
		f := newFixture()
		require.NoError(t, f.uc.Cast(t.Context(), userID, pollOpen, answer("o10")))

		err := f.uc.Revoke(t.Context(), userID, pollOpen)

		require.NoError(t, err)
		assert.Equal(t, []string{contracts.RoutingKeyVoteCast, contracts.RoutingKeyVoteRevoked}, f.outbox.sent())
		assert.Empty(t, f.votes.votes)
	})

	t.Run("error - poll does not allow revoke", func(t *testing.T) {
		f := newFixture()
		require.NoError(t, f.uc.Cast(t.Context(), userID, pollFinal, answer("o10")))

		err := f.uc.Revoke(t.Context(), userID, pollFinal)

		require.ErrorIs(t, err, model.ErrVoteNotChangeable)
		assert.Len(t, f.votes.votes, 1)
	})

	t.Run("error - nothing to revoke", func(t *testing.T) {
		f := newFixture()

		err := f.uc.Revoke(t.Context(), userID, pollOpen)

		require.ErrorIs(t, err, model.ErrVoteNotFound)
		assert.Empty(t, f.outbox.sent())
	})
}

func TestVoteUseCase_Status(t *testing.T) {
	tests := []struct {
		name        string
		pollID      string
		voteFirst   bool
		wantVoted   bool
		wantAnswers []model.Answer
		wantErr     error
	}{
		{name: "success - not voted", pollID: pollOpen},
		{
			name:        "success - voted, answers shown",
			pollID:      pollOpen,
			voteFirst:   true,
			wantVoted:   true,
			wantAnswers: answer("o10"),
		},
		{
			name:      "success - voted in anonymous poll, answers hidden",
			pollID:    pollAnonymous,
			voteFirst: true,
			wantVoted: true,
		},
		{
			name:        "success - voted in poll without vote change",
			pollID:      pollFinal,
			voteFirst:   true,
			wantVoted:   true,
			wantAnswers: answer("o10"),
		},
		{name: "error - poll not found", pollID: "poll-missing", wantErr: model.ErrPollNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			if tt.voteFirst {
				require.NoError(t, f.uc.Cast(t.Context(), userID, tt.pollID, answer("o10")))
			}

			status, err := f.uc.Status(t.Context(), userID, tt.pollID)

			require.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, tt.wantVoted, status.Voted)
			assert.Equal(t, tt.wantAnswers, status.Answers)
		})
	}
}
