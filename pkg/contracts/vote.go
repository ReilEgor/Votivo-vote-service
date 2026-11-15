// Package contracts defines the messages Vote Service publishes and the queues that carry them.
package contracts

// Queue names for vote events. Result Service consumes them to keep the counters of a poll up to date.
const (
	QueueVoteCast    = "vote.cast"
	QueueVoteRevoked = "vote.revoked"
)

// VoteAnswer is the options chosen for one question.
type VoteAnswer struct {
	QuestionID int64   `json:"question_id"`
	OptionIDs  []int64 `json:"option_ids"`
}

// VoteCastEvent is published when a vote is accepted: add one to the counter of every listed option.
// A changed vote is a VoteRevokedEvent of the old answers followed by a VoteCastEvent of the new ones.
// The events never carry the user, so an anonymous poll cannot be de-anonymized from the queue.
type VoteCastEvent struct {
	PollID    int64        `json:"poll_id"`
	Answers   []VoteAnswer `json:"answers"`
	RequestID string       `json:"request_id,omitempty"`
}

// VoteRevokedEvent is published when a vote is withdrawn: subtract one from the counter of every listed option.
type VoteRevokedEvent struct {
	PollID    int64        `json:"poll_id"`
	Answers   []VoteAnswer `json:"answers"`
	RequestID string       `json:"request_id,omitempty"`
}
