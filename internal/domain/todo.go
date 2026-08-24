package domain

import "encoding/json"

type TodoStatus string

const (
	TodoPending    TodoStatus = "pending"
	TodoInProgress TodoStatus = "in_progress"
	TodoCompleted  TodoStatus = "completed"
	TodoCancelled  TodoStatus = "cancelled"
)

// Todo is the durable, session-scoped task projection used by the plantask
// compatible tools. Dependencies are IDs within the same session.
type Todo struct {
	ID          string
	SessionID   SessionID
	Subject     string
	Description string
	Status      TodoStatus
	Blocks      []string
	BlockedBy   []string
	ActiveForm  string
	Owner       string
	Metadata    json.RawMessage
	Position    int
	CreatedAt   int64
	UpdatedAt   int64
}
