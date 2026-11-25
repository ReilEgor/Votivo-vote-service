package model_test

import (
	"github.com/ReilEgor/Votivo-vote-service/internal/domain/model"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestPollDeadline(t *testing.T) {
	past, future := time.Now().Add(-time.Minute), time.Now().Add(time.Hour)
	poll := model.Poll{Status: model.PollStatusActive, EndsAt: &past}
	assert.ErrorIs(t, poll.CheckActive(), model.ErrPollNotActive)
	poll.EndsAt = &future
	assert.NoError(t, poll.CheckActive())
	poll.EndsAt = nil // Previously published polls have no deadline.
	assert.NoError(t, poll.CheckActive())
}
