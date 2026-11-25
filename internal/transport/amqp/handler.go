// Package amqp turns the poll events from the broker into calls of the poll event use case.
package amqp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ReilEgor/Votivo-vote-service/internal/domain/model"
	"github.com/ReilEgor/Votivo-vote-service/internal/domain/usecase"
	"github.com/ReilEgor/Votivo-vote-service/pkg/contracts"
)

// Handler routes poll events to the use case.
type Handler struct {
	polls usecase.PollEventUseCase
}

// NewHandler returns a Handler.
func NewHandler(polls usecase.PollEventUseCase) *Handler {
	return &Handler{polls: polls}
}

// Handle processes one message by its routing key. An unknown key or a body that does not decode
// is model.ErrInvalidEvent: redelivering it cannot help.
func (h *Handler) Handle(ctx context.Context, routingKey string, body []byte) error {
	switch routingKey {
	case contracts.RoutingKeyPollStarted:
		return h.started(ctx, body)
	case contracts.RoutingKeyPollEnded:
		return h.ended(ctx, body)
	default:
		return fmt.Errorf("%w: unexpected routing key %q", model.ErrInvalidEvent, routingKey)
	}
}

func (h *Handler) started(ctx context.Context, body []byte) error {
	var event contracts.PollStartedEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return fmt.Errorf("%w: decode poll.started: %w", model.ErrInvalidEvent, err)
	}
	if strings.TrimSpace(event.PollID) == "" {
		return fmt.Errorf("%w: poll.started without poll_id", model.ErrInvalidEvent)
	}

	poll := model.Poll{
		ID:          event.PollID,
		Anonymous:   event.Anonymous,
		AllowChange: event.AllowVoteChange,
		EndsAt:      event.EndsAt,
		Questions:   make([]model.Question, 0, len(event.Questions)),
	}
	for _, q := range event.Questions {
		options := make([]string, 0, len(q.Options))
		for _, o := range q.Options {
			options = append(options, o.ID)
		}
		poll.Questions = append(poll.Questions, model.Question{ID: q.ID, Multiple: q.Multiple, OptionIDs: options})
	}
	if err := h.polls.Start(ctx, poll); err != nil {
		return fmt.Errorf("start poll %s: %w", poll.ID, err)
	}
	return nil
}

func (h *Handler) ended(ctx context.Context, body []byte) error {
	var event contracts.PollEndedEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return fmt.Errorf("%w: decode poll.ended: %w", model.ErrInvalidEvent, err)
	}
	if strings.TrimSpace(event.PollID) == "" {
		return fmt.Errorf("%w: poll.ended without poll_id", model.ErrInvalidEvent)
	}

	var status model.PollStatus
	switch event.Reason {
	case contracts.PollEndedReasonClosed:
		status = model.PollStatusClosed
	case contracts.PollEndedReasonBlocked:
		status = model.PollStatusBlocked
	default:
		return fmt.Errorf("%w: poll.ended with unknown reason %q", model.ErrInvalidEvent, event.Reason)
	}
	if err := h.polls.End(ctx, event.PollID, status); err != nil {
		return fmt.Errorf("end poll %s: %w", event.PollID, err)
	}
	return nil
}
