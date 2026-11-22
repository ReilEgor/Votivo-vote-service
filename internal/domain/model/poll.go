package model

// Limits of a poll from the requirements specification.
const (
	MaxQuestions          = 50
	MaxOptionsPerQuestion = 20
)

// PollStatus is the lifecycle state of a poll owned by Poll Service.
type PollStatus string

// Statuses of a poll. Only an active poll accepts votes.
const (
	PollStatusDraft    PollStatus = "draft"
	PollStatusActive   PollStatus = "active"
	PollStatusClosed   PollStatus = "closed"
	PollStatusArchived PollStatus = "archived"
	PollStatusBlocked  PollStatus = "blocked"
)

// Question is a question of a poll with the IDs of its options.
type Question struct {
	ID        string
	Multiple  bool
	OptionIDs []string
}

// Poll holds the voting rules Vote Service needs; the poll itself is owned by Poll Service.
type Poll struct {
	ID          string
	Status      PollStatus
	Anonymous   bool
	AllowChange bool
	Questions   []Question
}

// CheckActive returns ErrPollNotActive unless the poll accepts votes.
func (p Poll) CheckActive() error {
	if p.Status != PollStatusActive {
		return ErrPollNotActive
	}
	return nil
}

// CheckChangeable returns ErrVoteNotChangeable unless a user may change or revoke their vote.
// An anonymous poll never allows it: the ballot is not linked to the user, so it cannot be found again.
func (p Poll) CheckChangeable() error {
	if p.Anonymous || !p.AllowChange {
		return ErrVoteNotChangeable
	}
	return nil
}

// ValidateAnswers checks that the answers cover every question of the poll exactly once
// and choose only existing options, one per single-choice question.
func (p Poll) ValidateAnswers(answers []Answer) error {
	questions := make(map[string]Question, len(p.Questions))
	for _, q := range p.Questions {
		questions[q.ID] = q
	}

	answered := make(map[string]struct{}, len(answers))
	for _, a := range answers {
		q, ok := questions[a.QuestionID]
		if !ok {
			return ErrUnknownQuestion
		}
		if _, dup := answered[a.QuestionID]; dup {
			return ErrDuplicateAnswer
		}
		answered[a.QuestionID] = struct{}{}

		if err := q.validate(a); err != nil {
			return err
		}
	}
	if len(answered) != len(p.Questions) {
		return ErrQuestionNotAnswered
	}
	return nil
}

func (q Question) validate(a Answer) error {
	if len(a.OptionIDs) == 0 {
		return ErrQuestionNotAnswered
	}
	if !q.Multiple && len(a.OptionIDs) > 1 {
		return ErrTooManyOptions
	}

	known := make(map[string]struct{}, len(q.OptionIDs))
	for _, id := range q.OptionIDs {
		known[id] = struct{}{}
	}
	chosen := make(map[string]struct{}, len(a.OptionIDs))
	for _, id := range a.OptionIDs {
		if _, ok := known[id]; !ok {
			return ErrUnknownOption
		}
		if _, dup := chosen[id]; dup {
			return ErrDuplicateAnswer
		}
		chosen[id] = struct{}{}
	}
	return nil
}
