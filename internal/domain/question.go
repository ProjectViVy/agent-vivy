package domain

// QuestionStatus is the durable lifecycle of an ask_user interaction.
type QuestionStatus string

const (
	QuestionPending   QuestionStatus = "pending"
	QuestionAnswered  QuestionStatus = "answered"
	QuestionCancelled QuestionStatus = "cancelled"
)

// Question is a user-input suspension distinct from an effectful approval.
// Answer is persisted only after the caller passes server-side validation.
type Question struct {
	ID           string
	RunID        RunID
	ToolCallID   string
	Prompt       string
	Answer       string
	Status       QuestionStatus
	ExpiresAt    int64
	ResumeTarget string
}
