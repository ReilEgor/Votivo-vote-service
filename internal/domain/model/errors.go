package model

import "errors"

// Domain errors of the vote service.
var (
	ErrPollNotFound           = errors.New("poll not found")
	ErrPollNotActive          = errors.New("poll is not active")
	ErrPollServiceUnavailable = errors.New("poll service is temporarily unavailable")

	ErrAlreadyVoted      = errors.New("user has already voted in this poll")
	ErrVoteNotFound      = errors.New("vote not found")
	ErrVoteNotChangeable = errors.New("vote cannot be changed or revoked in this poll")

	ErrUnknownQuestion     = errors.New("answer refers to a question that is not in the poll")
	ErrUnknownOption       = errors.New("answer refers to an option that is not in the question")
	ErrDuplicateAnswer     = errors.New("question or option is answered more than once")
	ErrQuestionNotAnswered = errors.New("every question needs at least one option")
	ErrTooManyOptions      = errors.New("question allows only one option")
)
