package model

import "time"

// Answer is the options a user chose for one question.
type Answer struct {
	QuestionID int64
	OptionIDs  []int64
}

// Vote is the ballot of one user in one poll.
type Vote struct {
	PollID  int64
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

// OutboxMessage is a message waiting to be published to the broker.
type OutboxMessage struct {
	ID      int64
	Queue   string
	Payload []byte
}
