package amqp_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ReilEgor/Votivo-vote-service/internal/domain/model"
	amqphandler "github.com/ReilEgor/Votivo-vote-service/internal/transport/amqp"
	"github.com/ReilEgor/Votivo-vote-service/pkg/contracts"
)

type endCall struct {
	pollID string
	status model.PollStatus
}

// pollEventSpy records what the handler passed to the use case.
type pollEventSpy struct {
	started []model.Poll
	ended   []endCall
}

func (s *pollEventSpy) Start(_ context.Context, poll model.Poll) error {
	s.started = append(s.started, poll)
	return nil
}

func (s *pollEventSpy) End(_ context.Context, pollID string, status model.PollStatus) error {
	s.ended = append(s.ended, endCall{pollID, status})
	return nil
}

const startedBody = `{
  "poll_id": "11111111-1111-4111-8111-111111111111",
  "title": "Lunch",
  "anonymous": true,
  "allow_vote_change": true,
  "questions": [
    {"id": "q-1", "text": "Where?", "multiple": false,
     "options": [{"id": "o-1", "text": "Pizza"}, {"id": "o-2", "text": "Sushi"}]},
    {"id": "q-2", "text": "Sides?", "multiple": true, "options": [{"id": "o-3", "text": "Fries"}]}
  ]
}`

func TestHandler_Handle(t *testing.T) {
	tests := []struct {
		name       string
		routingKey string
		body       string
		wantErr    error
		check      func(t *testing.T, spy *pollEventSpy)
	}{
		{
			name:       "success - poll.started becomes a poll with its rules and options",
			routingKey: contracts.RoutingKeyPollStarted,
			body:       startedBody,
			check: func(t *testing.T, spy *pollEventSpy) {
				t.Helper()
				require.Len(t, spy.started, 1)
				assert.Equal(t, model.Poll{
					ID:          "11111111-1111-4111-8111-111111111111",
					Anonymous:   true,
					AllowChange: true,
					Questions: []model.Question{
						{ID: "q-1", Multiple: false, OptionIDs: []string{"o-1", "o-2"}},
						{ID: "q-2", Multiple: true, OptionIDs: []string{"o-3"}},
					},
				}, spy.started[0])
			},
		},
		{
			name:       "success - poll.ended closed",
			routingKey: contracts.RoutingKeyPollEnded,
			body:       `{"poll_id": "p-1", "reason": "closed"}`,
			check: func(t *testing.T, spy *pollEventSpy) {
				t.Helper()
				assert.Equal(t, []endCall{{"p-1", model.PollStatusClosed}}, spy.ended)
			},
		},
		{
			name:       "success - poll.ended blocked",
			routingKey: contracts.RoutingKeyPollEnded,
			body:       `{"poll_id": "p-1", "reason": "blocked"}`,
			check: func(t *testing.T, spy *pollEventSpy) {
				t.Helper()
				assert.Equal(t, []endCall{{"p-1", model.PollStatusBlocked}}, spy.ended)
			},
		},
		{
			name:       "success - unknown fields are ignored so Poll Service can extend the event",
			routingKey: contracts.RoutingKeyPollEnded,
			body:       `{"poll_id": "p-1", "reason": "closed", "closed_at": "2026-11-22T10:00:00Z"}`,
			check: func(t *testing.T, spy *pollEventSpy) {
				t.Helper()
				assert.Len(t, spy.ended, 1)
			},
		},
		{
			name:       "error - started body is not JSON",
			routingKey: contracts.RoutingKeyPollStarted,
			body:       `nope`,
			wantErr:    model.ErrInvalidEvent,
		},
		{
			name:       "error - started without poll_id",
			routingKey: contracts.RoutingKeyPollStarted,
			body:       `{}`,
			wantErr:    model.ErrInvalidEvent,
		},
		{
			name:       "error - ended body is not JSON",
			routingKey: contracts.RoutingKeyPollEnded,
			body:       `[`,
			wantErr:    model.ErrInvalidEvent,
		},
		{
			name:       "error - ended without poll_id",
			routingKey: contracts.RoutingKeyPollEnded,
			body:       `{"reason": "closed"}`,
			wantErr:    model.ErrInvalidEvent,
		},
		{
			name:       "error - ended with unknown reason",
			routingKey: contracts.RoutingKeyPollEnded,
			body:       `{"poll_id": "p", "reason": "x"}`,
			wantErr:    model.ErrInvalidEvent,
		},
		{
			name:       "error - unexpected routing key",
			routingKey: "poll.deleted",
			body:       `{}`,
			wantErr:    model.ErrInvalidEvent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spy := &pollEventSpy{}

			err := amqphandler.NewHandler(spy).Handle(t.Context(), tt.routingKey, []byte(tt.body))

			require.ErrorIs(t, err, tt.wantErr)
			if tt.check != nil {
				tt.check(t, spy)
				return
			}
			assert.Empty(t, spy.started)
			assert.Empty(t, spy.ended)
		})
	}
}
