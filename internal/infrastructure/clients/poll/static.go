package poll

import (
	"context"

	"github.com/ReilEgor/Votivo-vote-service/internal/domain/model"
)

// IDs understood by StaticClient. Each one is a poll with fixed rules, so every branch of the API
// can be exercised before Poll Service exists.
const (
	StaticChangeable  int64 = 1
	StaticAnonymous   int64 = 2
	StaticClosed      int64 = 3
	StaticFinal       int64 = 4
	StaticUnavailable int64 = 99
)

// StaticClient answers from a fixed list instead of calling Poll Service.
//
// Polls 1, 2 and 4 are active and have two questions: question 11 takes one of options 111-113, question 12
// takes any of options 121-123. Poll 1 allows changing the vote, poll 2 is anonymous, poll 4 is final.
type StaticClient struct{}

// NewStaticClient returns a StaticClient.
func NewStaticClient() *StaticClient {
	return &StaticClient{}
}

// GetPoll returns the fixed poll; unknown IDs do not exist.
func (StaticClient) GetPoll(_ context.Context, id int64) (model.Poll, error) {
	questions := []model.Question{
		{ID: 11, Multiple: false, OptionIDs: []int64{111, 112, 113}},
		{ID: 12, Multiple: true, OptionIDs: []int64{121, 122, 123}},
	}

	switch id {
	case StaticChangeable:
		return model.Poll{ID: id, Status: model.PollStatusActive, AllowChange: true, Questions: questions}, nil
	case StaticAnonymous:
		return model.Poll{ID: id, Status: model.PollStatusActive, Anonymous: true, Questions: questions}, nil
	case StaticClosed:
		return model.Poll{ID: id, Status: model.PollStatusClosed, Questions: questions}, nil
	case StaticFinal:
		return model.Poll{ID: id, Status: model.PollStatusActive, Questions: questions}, nil
	case StaticUnavailable:
		return model.Poll{}, model.ErrPollServiceUnavailable
	default:
		return model.Poll{}, model.ErrPollNotFound
	}
}
