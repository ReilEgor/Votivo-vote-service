// Package contracts defines the messages Vote Service publishes and the exchange that carries them.
package contracts

// ExchangeVotes is the durable topic exchange Vote Service publishes to. Every message carries its event ID
// as the AMQP message_id; delivery is at least once, so a consumer drops an ID it has already handled.
const ExchangeVotes = "vote.events.exchange"

// Routing keys of the vote events. Result Service binds a queue to them to keep the counters of a poll up to date.
const (
	RoutingKeyVoteCast    = "vote.cast"
	RoutingKeyVoteRevoked = "vote.revoked"
)

// VoteAnswer is the options chosen for one question. All IDs are UUIDs assigned by Poll Service.
type VoteAnswer struct {
	QuestionID string   `json:"question_id"`
	OptionIDs  []string `json:"option_ids"`
}

// VoteCastEvent is published when a vote is accepted: add one to the counter of every listed option.
// A changed vote is a VoteRevokedEvent of the old answers followed by a VoteCastEvent of the new ones.
// The events never carry the user, so an anonymous poll cannot be de-anonymized from the broker.
type VoteCastEvent struct {
	PollID    string       `json:"poll_id"`
	Answers   []VoteAnswer `json:"answers"`
	RequestID string       `json:"request_id,omitempty"`
}

// VoteRevokedEvent is published when a vote is withdrawn: subtract one from the counter of every listed option.
type VoteRevokedEvent struct {
	PollID    string       `json:"poll_id"`
	Answers   []VoteAnswer `json:"answers"`
	RequestID string       `json:"request_id,omitempty"`
}
