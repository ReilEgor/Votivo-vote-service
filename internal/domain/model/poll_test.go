package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ReilEgor/Votivo-vote-service/internal/domain/model"
)

func testPoll() model.Poll {
	return model.Poll{
		ID:     1,
		Status: model.PollStatusActive,
		Questions: []model.Question{
			{ID: 1, Multiple: false, OptionIDs: []int64{10, 11}},
			{ID: 2, Multiple: true, OptionIDs: []int64{20, 21, 22}},
		},
	}
}

func TestPoll_ValidateAnswers(t *testing.T) {
	tests := []struct {
		name    string
		answers []model.Answer
		wantErr error
	}{
		{
			name: "success - one option and several options",
			answers: []model.Answer{
				{QuestionID: 1, OptionIDs: []int64{10}},
				{QuestionID: 2, OptionIDs: []int64{20, 22}},
			},
		},
		{
			name:    "error - question missing",
			answers: []model.Answer{{QuestionID: 1, OptionIDs: []int64{10}}},
			wantErr: model.ErrQuestionNotAnswered,
		},
		{
			name: "error - question without options",
			answers: []model.Answer{
				{QuestionID: 1, OptionIDs: []int64{10}},
				{QuestionID: 2},
			},
			wantErr: model.ErrQuestionNotAnswered,
		},
		{
			name: "error - unknown question",
			answers: []model.Answer{
				{QuestionID: 1, OptionIDs: []int64{10}},
				{QuestionID: 3, OptionIDs: []int64{30}},
			},
			wantErr: model.ErrUnknownQuestion,
		},
		{
			name: "error - unknown option",
			answers: []model.Answer{
				{QuestionID: 1, OptionIDs: []int64{20}},
				{QuestionID: 2, OptionIDs: []int64{20}},
			},
			wantErr: model.ErrUnknownOption,
		},
		{
			name: "error - several options for a single-choice question",
			answers: []model.Answer{
				{QuestionID: 1, OptionIDs: []int64{10, 11}},
				{QuestionID: 2, OptionIDs: []int64{20}},
			},
			wantErr: model.ErrTooManyOptions,
		},
		{
			name: "error - question answered twice",
			answers: []model.Answer{
				{QuestionID: 1, OptionIDs: []int64{10}},
				{QuestionID: 1, OptionIDs: []int64{11}},
			},
			wantErr: model.ErrDuplicateAnswer,
		},
		{
			name: "error - option chosen twice",
			answers: []model.Answer{
				{QuestionID: 1, OptionIDs: []int64{10}},
				{QuestionID: 2, OptionIDs: []int64{20, 20}},
			},
			wantErr: model.ErrDuplicateAnswer,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := testPoll().ValidateAnswers(tt.answers)

			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestPoll_CheckActive(t *testing.T) {
	tests := []struct {
		name    string
		status  model.PollStatus
		wantErr error
	}{
		{name: "success - active", status: model.PollStatusActive},
		{name: "error - draft", status: model.PollStatusDraft, wantErr: model.ErrPollNotActive},
		{name: "error - closed", status: model.PollStatusClosed, wantErr: model.ErrPollNotActive},
		{name: "error - archived", status: model.PollStatusArchived, wantErr: model.ErrPollNotActive},
		{name: "error - blocked", status: model.PollStatusBlocked, wantErr: model.ErrPollNotActive},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := model.Poll{Status: tt.status}.CheckActive()

			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestPoll_CheckChangeable(t *testing.T) {
	tests := []struct {
		name    string
		poll    model.Poll
		wantErr error
	}{
		{name: "success - change allowed", poll: model.Poll{AllowChange: true}},
		{name: "error - change not allowed", poll: model.Poll{}, wantErr: model.ErrVoteNotChangeable},
		{
			name:    "error - anonymous poll never allows it",
			poll:    model.Poll{Anonymous: true, AllowChange: true},
			wantErr: model.ErrVoteNotChangeable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.poll.CheckChangeable()

			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}
