// Package dto holds the request and response bodies of the HTTP API.
package dto

import "github.com/ReilEgor/Votivo-vote-service/internal/domain/model"

// AnswerDTO is the options chosen for one question.
type AnswerDTO struct {
	QuestionID string   `json:"question_id"`
	OptionIDs  []string `json:"option_ids"`
}

// VoteRequest is the body of POST and PUT /api/v1/polls/:poll_id/vote.
type VoteRequest struct {
	Answers []AnswerDTO `json:"answers"`
}

// ToAnswers converts the request to domain answers.
func (r VoteRequest) ToAnswers() []model.Answer {
	answers := make([]model.Answer, 0, len(r.Answers))
	for _, a := range r.Answers {
		answers = append(answers, model.Answer{QuestionID: a.QuestionID, OptionIDs: a.OptionIDs})
	}
	return answers
}

// VoteStatusResponse is the body of GET /api/v1/polls/:poll_id/vote.
// Answers are omitted when the user has not voted or the poll is anonymous.
type VoteStatusResponse struct {
	Voted   bool        `json:"voted"`
	Answers []AnswerDTO `json:"answers,omitempty"`
}

// NewVoteStatusResponse converts the status to the response body.
func NewVoteStatusResponse(status model.VoteStatus) VoteStatusResponse {
	answers := make([]AnswerDTO, 0, len(status.Answers))
	for _, a := range status.Answers {
		answers = append(answers, AnswerDTO{QuestionID: a.QuestionID, OptionIDs: a.OptionIDs})
	}
	return VoteStatusResponse{Voted: status.Voted, Answers: answers}
}

// MessageResponse is the body of a successful action.
type MessageResponse struct {
	Message string `json:"message"`
}

// ErrorResponse is the body of a failed request with one error.
type ErrorResponse struct {
	Error string `json:"error"`
}

// ErrorsResponse is the body of a request that failed validation with several errors.
type ErrorsResponse struct {
	Errors []string `json:"errors"`
}
