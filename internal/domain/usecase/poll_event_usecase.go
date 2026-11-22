package usecase

import (
	"context"

	"github.com/ReilEgor/Votivo-vote-service/internal/domain/model"
)

// PollEventUseCase applies the events Poll Service publishes. Both operations are idempotent:
// the broker delivers an event at least once.
type PollEventUseCase interface {
	// Start records the rules and questions of a poll that began accepting votes.
	Start(ctx context.Context, poll model.Poll) error
	// End records that the poll stopped accepting votes; status is closed or blocked.
	End(ctx context.Context, pollID string, status model.PollStatus) error
}
