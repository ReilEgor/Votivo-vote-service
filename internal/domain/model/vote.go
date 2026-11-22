package model

import "time"

// Answer is the options a user chose for one question.
type Answer struct {
	QuestionID string
	OptionIDs  []string
}

// Vote is the ballot of one user in one poll.
type Vote struct {
	PollID  string
	UserID  string
	Answers []Answer
	// Anonymous tells the repository not to link the stored ballot to UserID.
	Anonymous bool
	CreatedAt time.Time
}

// VoteStatus answers "did this user vote". Answers are empty for an anonymous poll.
type VoteStatus struct {
	Voted   bool
	Answers []Answer
}

// OutboxMessage is an event waiting to be published to the broker.
// EventID travels as the AMQP message_id, so consumers can drop a message delivered twice.
type OutboxMessage struct {
	ID         int64
	EventID    string
	RoutingKey string
	Payload    []byte
}
