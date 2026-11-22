package contracts

// ExchangePolls is the durable topic exchange Poll Service publishes its events to. Vote Service only consumes it.
// Every message carries its event ID as the AMQP message_id.
const ExchangePolls = "poll.events.exchange"

// Routing keys of the poll events.
const (
	RoutingKeyPollStarted = "poll.started"
	RoutingKeyPollEnded   = "poll.ended"
)

// Queue of Vote Service for the poll events. A message that cannot be processed is dead-lettered to the
// dead-letter exchange under the name of the queue, and the queue named Queue + DeadLetterQueueSuffix keeps it.
const (
	QueuePollEvents       = "vote-service.poll-events"
	ExchangeDeadLetter    = "votivo.dlx"
	DeadLetterQueueSuffix = ".dlq"
)

// Reasons a poll ended.
const (
	PollEndedReasonClosed  = "closed"
	PollEndedReasonBlocked = "blocked"
)

// PollOption is an option of a question.
type PollOption struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// PollQuestion is a question of a started poll.
type PollQuestion struct {
	ID       string       `json:"id"`
	Text     string       `json:"text"`
	Multiple bool         `json:"multiple"`
	Options  []PollOption `json:"options"`
}

// PollStartedEvent is published when a poll is published and starts accepting votes.
// Its content cannot change afterwards, so the event is the whole snapshot voting needs.
type PollStartedEvent struct {
	PollID          string         `json:"poll_id"`
	Title           string         `json:"title"`
	Anonymous       bool           `json:"anonymous"`
	AllowVoteChange bool           `json:"allow_vote_change"`
	Questions       []PollQuestion `json:"questions"`
}

// PollEndedEvent is published when a poll stops accepting votes: its owner closed it or a moderator blocked it.
type PollEndedEvent struct {
	PollID string `json:"poll_id"`
	// Reason is PollEndedReasonClosed or PollEndedReasonBlocked.
	Reason string `json:"reason"`
}
