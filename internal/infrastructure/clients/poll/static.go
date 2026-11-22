package poll

import (
	"context"

	"github.com/ReilEgor/Votivo-vote-service/internal/domain/model"
)

// IDs understood by StaticClient. Each one is a poll with fixed rules, so every branch of the API
// can be exercised before Poll Service events reach this service.
const (
	StaticChangeable  = "00000000-0000-4000-8000-000000000001"
	StaticAnonymous   = "00000000-0000-4000-8000-000000000002"
	StaticClosed      = "00000000-0000-4000-8000-000000000003"
	StaticFinal       = "00000000-0000-4000-8000-000000000004"
	StaticUnavailable = "00000000-0000-4000-8000-000000000099"
)

// IDs of the questions and options of the static polls.
const (
	StaticSingleQuestion   = "00000000-0000-4000-8000-000000000011"
	StaticMultipleQuestion = "00000000-0000-4000-8000-000000000012"
)

// StaticSingleOptions are the options of the single-choice question, StaticMultipleOptions of the multiple-choice one.
var (
	StaticSingleOptions = []string{
		"00000000-0000-4000-8000-000000000111",
		"00000000-0000-4000-8000-000000000112",
		"00000000-0000-4000-8000-000000000113",
	}
	StaticMultipleOptions = []string{
		"00000000-0000-4000-8000-000000000121",
		"00000000-0000-4000-8000-000000000122",
		"00000000-0000-4000-8000-000000000123",
	}
)

// StaticClient answers from a fixed list instead of asking Poll Service.
//
// Polls ...01, ...02 and ...04 are active and have two questions: the single-choice one takes one of three
// options, the multiple-choice one any of three. Poll ...01 allows changing the vote, ...02 is anonymous,
// ...04 is final.
type StaticClient struct{}

// NewStaticClient returns a StaticClient.
func NewStaticClient() *StaticClient {
	return &StaticClient{}
}

// GetPoll returns the fixed poll; unknown IDs do not exist.
func (StaticClient) GetPoll(_ context.Context, id string) (model.Poll, error) {
	questions := []model.Question{
		{ID: StaticSingleQuestion, Multiple: false, OptionIDs: StaticSingleOptions},
		{ID: StaticMultipleQuestion, Multiple: true, OptionIDs: StaticMultipleOptions},
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
