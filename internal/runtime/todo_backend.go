package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/eino/adk/middlewares/filesystem"
	"github.com/cloudwego/eino/adk/middlewares/plantask"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/tools"
)

const (
	maxTodoCount       = 256
	maxTodoSubject     = 512
	maxTodoDescription = 16 << 10
)

var _ tools.TodoOperations = (*EinoTodoBackend)(nil)
var _ plantask.Backend = (*EinoTodoBackend)(nil)

// EinoTodoBackend is the task compatible adapter backed by durable
// session storage. The virtual JSON-file methods exist for Eino conformance;
// production Vivy tools call the typed operations below.
type EinoTodoBackend struct {
	store   storage.TodoStore
	baseDir string
}

func NewEinoTodoBackend(store storage.TodoStore, baseDir string) *EinoTodoBackend {
	return &EinoTodoBackend{store: store, baseDir: strings.TrimSuffix(strings.TrimSpace(baseDir), "/")}
}

func (b *EinoTodoBackend) CreateTodo(ctx context.Context, sessionID domain.SessionID, todo domain.Todo) (domain.Todo, error) {
	if err := b.require(sessionID); err != nil {
		return domain.Todo{}, err
	}
	if err := validateTodoText(todo.Subject, todo.Description); err != nil {
		return domain.Todo{}, err
	}
	items, err := b.store.ListTodos(ctx, sessionID)
	if err != nil {
		return domain.Todo{}, err
	}
	if len(items) >= maxTodoCount {
		return domain.Todo{}, fmt.Errorf("todo: maximum of %d tasks reached", maxTodoCount)
	}
	maxID := 0
	for _, item := range items {
		if id, parseErr := strconv.Atoi(item.ID); parseErr == nil && id > maxID {
			maxID = id
		}
	}
	now := time.Now().UnixMilli()
	todo.ID = strconv.Itoa(maxID + 1)
	todo.SessionID = sessionID
	todo.Status = domain.TodoPending
	todo.Position = len(items)
	todo.CreatedAt, todo.UpdatedAt = now, now
	if todo.Blocks == nil {
		todo.Blocks = []string{}
	}
	if todo.BlockedBy == nil {
		todo.BlockedBy = []string{}
	}
	if err := b.store.CreateTodo(ctx, todo); err != nil {
		return domain.Todo{}, err
	}
	return todo, nil
}

func (b *EinoTodoBackend) GetTodo(ctx context.Context, sessionID domain.SessionID, id string) (domain.Todo, error) {
	if err := b.require(sessionID); err != nil {
		return domain.Todo{}, err
	}
	if !validTodoID(id) {
		return domain.Todo{}, errors.New("todo: invalid task id")
	}
	return b.store.GetTodo(ctx, sessionID, id)
}

func (b *EinoTodoBackend) ListTodos(ctx context.Context, sessionID domain.SessionID) ([]domain.Todo, error) {
	if err := b.require(sessionID); err != nil {
		return nil, err
	}
	return b.store.ListTodos(ctx, sessionID)
}

func (b *EinoTodoBackend) UpdateTodo(ctx context.Context, todo domain.Todo) (domain.Todo, error) {
	if err := b.require(todo.SessionID); err != nil {
		return domain.Todo{}, err
	}
	if !validTodoID(todo.ID) {
		return domain.Todo{}, errors.New("todo: invalid task id")
	}
	if err := validateTodoText(todo.Subject, todo.Description); err != nil {
		return domain.Todo{}, err
	}
	if !validTodoStatus(todo.Status) {
		return domain.Todo{}, fmt.Errorf("todo: unsupported status %q", todo.Status)
	}
	items, err := b.store.ListTodos(ctx, todo.SessionID)
	if err != nil {
		return domain.Todo{}, err
	}
	byID := make(map[string]domain.Todo, len(items))
	for _, item := range items {
		byID[item.ID] = item
	}
	if _, ok := byID[todo.ID]; !ok {
		return domain.Todo{}, storage.ErrNotFound
	}
	for _, dependency := range append(append([]string{}, todo.Blocks...), todo.BlockedBy...) {
		if dependency == todo.ID {
			return domain.Todo{}, errors.New("todo: task cannot depend on itself")
		}
		if _, ok := byID[dependency]; !ok {
			return domain.Todo{}, fmt.Errorf("todo: dependency %q does not exist", dependency)
		}
	}
	if todo.Status == domain.TodoInProgress {
		for _, item := range items {
			if item.ID != todo.ID && item.Status == domain.TodoInProgress {
				return domain.Todo{}, errors.New("todo: only one task may be in_progress")
			}
		}
	}
	if wouldCycle(byID, todo) {
		return domain.Todo{}, errors.New("todo: dependency cycle rejected")
	}
	for _, item := range items {
		if item.ID == todo.ID {
			continue
		}
		if contains(todo.Blocks, item.ID) {
			item.BlockedBy = appendUnique(item.BlockedBy, todo.ID)
		}
		if contains(todo.BlockedBy, item.ID) {
			item.Blocks = appendUnique(item.Blocks, todo.ID)
		}
		item.UpdatedAt = time.Now().UnixMilli()
		// These writes are idempotent; only persist rows whose dependency
		// projection changed to avoid needless sqlite churn.
		if !sameTodoDependencies(item, byID[item.ID]) {
			if err := b.store.UpdateTodo(ctx, item); err != nil {
				return domain.Todo{}, err
			}
		}
	}
	todo.UpdatedAt = time.Now().UnixMilli()
	if err := b.store.UpdateTodo(ctx, todo); err != nil {
		return domain.Todo{}, err
	}
	return todo, nil
}

func (b *EinoTodoBackend) require(sessionID domain.SessionID) error {
	if b == nil || b.store == nil {
		return errors.New("todo: backend not wired")
	}
	if sessionID == "" {
		return errors.New("todo: session identity is required")
	}
	return nil
}

func (b *EinoTodoBackend) LsInfo(ctx context.Context, req *plantask.LsInfoRequest) ([]plantask.FileInfo, error) {
	sessionID := tools.SessionIDFromContext(ctx)
	items, err := b.ListTodos(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	base := b.baseDir
	if req != nil && req.Path != "" {
		base = req.Path
	}
	out := make([]plantask.FileInfo, 0, len(items))
	for _, item := range items {
		out = append(out, plantask.FileInfo{Path: filepath.Join(base, item.ID+".json"), Size: int64(len(item.Description) + len(item.Subject))})
	}
	return out, nil
}

func (b *EinoTodoBackend) Read(ctx context.Context, req *plantask.ReadRequest) (*filesystem.FileContent, error) {
	if req == nil {
		return nil, errors.New("todo: read request is nil")
	}
	id := strings.TrimSuffix(filepath.Base(req.FilePath), ".json")
	todo, err := b.GetTodo(ctx, tools.SessionIDFromContext(ctx), id)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(struct {
		ID          string            `json:"id"`
		Subject     string            `json:"subject"`
		Description string            `json:"description"`
		Status      domain.TodoStatus `json:"status"`
		Blocks      []string          `json:"blocks"`
		BlockedBy   []string          `json:"blockedBy"`
		ActiveForm  string            `json:"activeForm,omitempty"`
		Owner       string            `json:"owner,omitempty"`
		Metadata    json.RawMessage   `json:"metadata,omitempty"`
	}{ID: todo.ID, Subject: todo.Subject, Description: todo.Description, Status: todo.Status, Blocks: todo.Blocks,
		BlockedBy: todo.BlockedBy, ActiveForm: todo.ActiveForm, Owner: todo.Owner, Metadata: todo.Metadata})
	if err != nil {
		return nil, err
	}
	return &filesystem.FileContent{Content: string(data)}, nil
}

func (b *EinoTodoBackend) Write(ctx context.Context, req *plantask.WriteRequest) error {
	if req == nil {
		return errors.New("todo: write request is nil")
	}
	var todo domain.Todo
	if err := json.Unmarshal([]byte(req.Content), &todo); err != nil {
		return fmt.Errorf("todo: decode virtual task: %w", err)
	}
	todo.ID = strings.TrimSuffix(filepath.Base(req.FilePath), ".json")
	todo.SessionID = tools.SessionIDFromContext(ctx)
	if todo.ID == "" {
		return errors.New("todo: task id is required")
	}
	if _, err := b.GetTodo(ctx, todo.SessionID, todo.ID); errors.Is(err, storage.ErrNotFound) {
		_, err = b.CreateTodo(ctx, todo.SessionID, todo)
		return err
	} else if err != nil {
		return err
	}
	_, err := b.UpdateTodo(ctx, todo)
	return err
}

func (b *EinoTodoBackend) Delete(ctx context.Context, req *plantask.DeleteRequest) error {
	if req == nil {
		return errors.New("todo: delete request is nil")
	}
	id := strings.TrimSuffix(filepath.Base(req.FilePath), ".json")
	todo, err := b.GetTodo(ctx, tools.SessionIDFromContext(ctx), id)
	if err != nil {
		return err
	}
	todo.Status = domain.TodoCancelled
	_, err = b.UpdateTodo(ctx, todo)
	return err
}

func validateTodoText(subject, description string) error {
	if len(subject) == 0 || len(subject) > maxTodoSubject || len(description) == 0 || len(description) > maxTodoDescription {
		return errors.New("todo: subject/description is empty or too large")
	}
	return nil
}

func validTodoID(id string) bool {
	if id == "" {
		return false
	}
	for _, r := range id {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func validTodoStatus(status domain.TodoStatus) bool {
	switch status {
	case domain.TodoPending, domain.TodoInProgress, domain.TodoCompleted, domain.TodoCancelled:
		return true
	default:
		return false
	}
}

func appendUnique(base []string, id string) []string {
	if contains(base, id) {
		return base
	}
	return append(base, id)
}

func contains(items []string, wanted string) bool {
	for _, item := range items {
		if item == wanted {
			return true
		}
	}
	return false
}

func sameTodoDependencies(a, b domain.Todo) bool {
	return strings.Join(a.Blocks, "\x00") == strings.Join(b.Blocks, "\x00") && strings.Join(a.BlockedBy, "\x00") == strings.Join(b.BlockedBy, "\x00")
}

func wouldCycle(byID map[string]domain.Todo, candidate domain.Todo) bool {
	graph := make(map[string][]string, len(byID))
	for id, item := range byID {
		graph[id] = append([]string(nil), item.Blocks...)
	}
	graph[candidate.ID] = append([]string(nil), candidate.Blocks...)
	for blocker := range byID {
		if contains(candidate.BlockedBy, blocker) {
			graph[blocker] = appendUnique(graph[blocker], candidate.ID)
		}
	}
	var visit func(string, map[string]bool) bool
	visit = func(id string, stack map[string]bool) bool {
		if stack[id] {
			return true
		}
		stack[id] = true
		for _, next := range graph[id] {
			if visit(next, stack) {
				return true
			}
		}
		delete(stack, id)
		return false
	}
	for id := range graph {
		if visit(id, map[string]bool{}) {
			return true
		}
	}
	return false
}
