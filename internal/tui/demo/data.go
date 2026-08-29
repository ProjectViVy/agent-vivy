// Package demo holds deterministic mock data for the Crush-style TUI
// skeleton. No time.Now, no random, no RPC. Field names mirror the
// control-plane DTOs so a later slice can swap this store for Client.
package demo

import (
	"fmt"
	"strings"

	"agent-vivy/internal/domain"
)

// Session is one sidebar row.
type Session struct {
	ID               string
	Title            string
	PermissionPreset string
}

// ToolCard is an inline tool result / pending approval inside the chat.
type ToolCard struct {
	ToolName   string
	Status     string // pending | done | denied | failed
	Preview    string
	Result     string
	ApprovalID string
}

// Message is one chat bubble or tool card.
type Message struct {
	ID      string
	Role    string // user | assistant | tool
	Content string
	Tool    *ToolCard
}

// Gate is the modal approval / question overlay.
type Gate struct {
	Kind  string // approval | question
	ID    string
	Title string
	Body  string
}

// Store is the in-memory demo world.
type Store struct {
	Sessions []Session
	Messages map[string][]Message
	ActiveID string
	seq      int
}

// NewStore returns the fixed three-session script from the plan.
func NewStore() *Store {
	s := &Store{
		Sessions: []Session{
			{ID: "sess_overnight", Title: "过夜", PermissionPreset: "smart"},
			{ID: "sess_approval", Title: "审批中", PermissionPreset: "cautious"},
			{ID: "sess_empty", Title: "空", PermissionPreset: "smart"},
		},
		Messages: map[string][]Message{
			"sess_overnight": {
				{ID: "msg_o1", Role: string(domain.RoleUser), Content: "把欢迎文案改得更短一点。"},
				{ID: "msg_o2", Role: string(domain.RoleAssistant), Content: "我会改 `ui/src/i18n/zh.ts` 里的空状态句子。"},
				{
					ID:   "msg_o3",
					Role: "tool",
					Tool: &ToolCard{
						ToolName: "write_file",
						Status:   "done",
						Preview:  "ui/src/i18n/zh.ts",
						Result:   "patched 1 line",
					},
				},
				{ID: "msg_o4", Role: string(domain.RoleAssistant), Content: "已缩短为空状态「寻找真心之旅」。"},
			},
			"sess_approval": {
				{ID: "msg_a1", Role: string(domain.RoleUser), Content: "把 README 标题改成 Vivy。"},
				{ID: "msg_a2", Role: string(domain.RoleAssistant), Content: "需要写文件，先请你批一下。"},
				{
					ID:   "msg_a3",
					Role: "tool",
					Tool: &ToolCard{
						ToolName:   "write_file",
						Status:     "pending",
						Preview:    "README.md\n- Agent Diva\n+ Vivy",
						ApprovalID: "appr_demo_1",
					},
				},
			},
			"sess_empty": {},
		},
		ActiveID: "sess_approval",
		seq:      100,
	}
	return s
}

// Active returns the selected session, or a zero value.
func (s *Store) Active() Session {
	for _, session := range s.Sessions {
		if session.ID == s.ActiveID {
			return session
		}
	}
	return Session{}
}

// ActiveMessages returns chat rows for the selected session.
func (s *Store) ActiveMessages() []Message {
	return append([]Message(nil), s.Messages[s.ActiveID]...)
}

// PendingGate is the modal for the active session, if any.
func (s *Store) PendingGate() *Gate {
	for _, message := range s.Messages[s.ActiveID] {
		if message.Tool == nil || message.Tool.Status != "pending" || message.Tool.ApprovalID == "" {
			continue
		}
		body := message.Tool.ToolName
		if message.Tool.Preview != "" {
			body = message.Tool.ToolName + "\n" + message.Tool.Preview
		}
		return &Gate{
			Kind:  "approval",
			ID:    message.Tool.ApprovalID,
			Title: message.Tool.ToolName,
			Body:  body,
		}
	}
	return nil
}

// SelectSession switches the active session by id. Unknown ids are ignored.
func (s *Store) SelectSession(id string) {
	for _, session := range s.Sessions {
		if session.ID == id {
			s.ActiveID = id
			return
		}
	}
}

// MoveSession steps the sidebar selection by delta (-1 / +1), wrapping.
func (s *Store) MoveSession(delta int) {
	if len(s.Sessions) == 0 {
		return
	}
	idx := 0
	for i, session := range s.Sessions {
		if session.ID == s.ActiveID {
			idx = i
			break
		}
	}
	n := len(s.Sessions)
	idx = (idx + delta) % n
	if idx < 0 {
		idx += n
	}
	s.ActiveID = s.Sessions[idx].ID
}

// NewSession appends a blank session and selects it.
func (s *Store) NewSession(title string) {
	title = strings.TrimSpace(title)
	if title == "" {
		title = "新会话"
	}
	s.seq++
	id := fmt.Sprintf("sess_demo_%d", s.seq)
	s.Sessions = append(s.Sessions, Session{ID: id, Title: title, PermissionPreset: "smart"})
	if s.Messages == nil {
		s.Messages = map[string][]Message{}
	}
	s.Messages[id] = nil
	s.ActiveID = id
}

// AppendUser adds a user bubble and a fixed assistant demo reply.
// When a gate is open it refuses (caller should decide first).
func (s *Store) AppendUser(text string) {
	text = strings.TrimSpace(text)
	if text == "" || s.PendingGate() != nil {
		return
	}
	s.seq++
	userID := fmt.Sprintf("msg_demo_%d", s.seq)
	s.seq++
	asstID := fmt.Sprintf("msg_demo_%d", s.seq)
	s.Messages[s.ActiveID] = append(s.Messages[s.ActiveID],
		Message{ID: userID, Role: string(domain.RoleUser), Content: text},
		Message{ID: asstID, Role: string(domain.RoleAssistant), Content: "（demo：未接控制面）"},
	)
}

// DecideApproval applies y/n to the pending tool card on the active session.
// decision must be domain.ApprovalApproved or domain.ApprovalDenied.
func (s *Store) DecideApproval(decision string) bool {
	if decision != domain.ApprovalApproved && decision != domain.ApprovalDenied {
		return false
	}
	msgs := s.Messages[s.ActiveID]
	for i := range msgs {
		tool := msgs[i].Tool
		if tool == nil || tool.Status != "pending" || tool.ApprovalID == "" {
			continue
		}
		if decision == domain.ApprovalApproved {
			tool.Status = "done"
			tool.Result = "demo approved"
		} else {
			tool.Status = "denied"
			tool.Result = "demo denied"
		}
		s.seq++
		reply := "已按你的决定继续（demo）。"
		if decision == domain.ApprovalDenied {
			reply = "已拒绝该工具（demo）。"
		}
		s.Messages[s.ActiveID] = append(msgs, Message{
			ID:      fmt.Sprintf("msg_demo_%d", s.seq),
			Role:    string(domain.RoleAssistant),
			Content: reply,
		})
		return true
	}
	return false
}
