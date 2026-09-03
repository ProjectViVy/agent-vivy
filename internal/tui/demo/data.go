// Package demo holds deterministic offline data for the Crush-style TUI
// skeleton. No time.Now, no random, no RPC. Field names mirror the
// control-plane DTOs so the live driver can share surface.Driver.
package demo

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tui/surface"
)

// Store is the in-memory demo world and a surface.Driver.
type Store struct {
	sessions []surface.Session
	messages map[string][]surface.Message
	activeID string
	seq      int
}

// NewStore returns the fixed three-session script from the plan.
func NewStore() *Store {
	s := &Store{
		sessions: []surface.Session{
			{ID: "sess_overnight", Title: "过夜", PermissionPreset: "smart"},
			{ID: "sess_approval", Title: "审批中", PermissionPreset: "cautious"},
			{ID: "sess_empty", Title: "空", PermissionPreset: "smart"},
		},
		messages: map[string][]surface.Message{
			"sess_overnight": {
				{ID: "msg_o1", Role: string(domain.RoleUser), Content: "把欢迎文案改得更短一点。"},
				{ID: "msg_o2", Role: string(domain.RoleAssistant), Content: "我会改 `ui/src/i18n/zh.ts` 里的空状态句子。"},
				{
					ID:   "msg_o3",
					Role: "tool",
					Tool: &surface.ToolCard{
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
					Tool: &surface.ToolCard{
						ToolName:   "write_file",
						Status:     "pending",
						Preview:    "README.md\n- Agent Diva\n+ Vivy",
						ApprovalID: "appr_demo_1",
					},
				},
			},
			"sess_empty": {},
		},
		activeID: "sess_approval",
		seq:      100,
	}
	return s
}

// Sessions implements surface.Driver.
func (s *Store) Sessions() []surface.Session {
	return append([]surface.Session(nil), s.sessions...)
}

// Active implements surface.Driver.
func (s *Store) Active() surface.Session {
	for _, session := range s.sessions {
		if session.ID == s.activeID {
			return session
		}
	}
	return surface.Session{}
}

// ActiveMessages implements surface.Driver.
func (s *Store) ActiveMessages() []surface.Message {
	return append([]surface.Message(nil), s.messages[s.activeID]...)
}

// PendingGate implements surface.Driver.
func (s *Store) PendingGate() *surface.Gate {
	for _, message := range s.messages[s.activeID] {
		if message.Tool == nil || message.Tool.Status != "pending" || message.Tool.ApprovalID == "" {
			continue
		}
		body := message.Tool.ToolName
		if message.Tool.Preview != "" {
			body = message.Tool.ToolName + "\n" + message.Tool.Preview
		}
		return &surface.Gate{
			Kind:  "approval",
			ID:    message.Tool.ApprovalID,
			Title: message.Tool.ToolName,
			Body:  body,
		}
	}
	return nil
}

// Meta implements surface.Driver.
func (s *Store) Meta() surface.Meta {
	return surface.Meta{
		Mode:   "demo",
		Footer: "demo · not connected",
	}
}

// Init implements surface.Driver.
func (s *Store) Init() tea.Cmd { return nil }

// Handle implements surface.Driver.
func (s *Store) Handle(tea.Msg) tea.Cmd { return nil }

// MoveSession implements surface.Driver.
func (s *Store) MoveSession(delta int) tea.Cmd {
	s.moveSession(delta)
	return nil
}

// NewSession implements surface.Driver.
func (s *Store) NewSession(title string) tea.Cmd {
	s.newSession(title)
	return nil
}

// Send implements surface.Driver.
func (s *Store) Send(text string) tea.Cmd {
	s.appendUser(text)
	return nil
}

// DecideApproval implements surface.Driver.
func (s *Store) DecideApproval(decision string) tea.Cmd {
	s.decideApproval(decision)
	return nil
}

// AnswerQuestion implements surface.Driver (demo has no question gate).
func (s *Store) AnswerQuestion(string) tea.Cmd { return nil }

// SetPermission implements surface.Driver for the explicit demo fixture.
func (s *Store) SetPermission(preset string) tea.Cmd {
	for i := range s.sessions {
		if s.sessions[i].ID == s.activeID {
			s.sessions[i].PermissionPreset = preset
			break
		}
	}
	return nil
}

func (s *Store) ClearQueue() bool { return false }

// Cancel implements surface.Driver (demo has nothing in flight).
func (s *Store) Cancel() tea.Cmd { return nil }

// SelectSession switches the active session by id. Unknown ids are ignored.
// Kept for tests.
func (s *Store) SelectSession(id string) {
	for _, session := range s.sessions {
		if session.ID == id {
			s.activeID = id
			return
		}
	}
}

func (s *Store) moveSession(delta int) {
	if len(s.sessions) == 0 {
		return
	}
	idx := 0
	for i, session := range s.sessions {
		if session.ID == s.activeID {
			idx = i
			break
		}
	}
	n := len(s.sessions)
	idx = (idx + delta) % n
	if idx < 0 {
		idx += n
	}
	s.activeID = s.sessions[idx].ID
}

func (s *Store) newSession(title string) {
	title = strings.TrimSpace(title)
	if title == "" {
		title = "新会话"
	}
	s.seq++
	id := fmt.Sprintf("sess_demo_%d", s.seq)
	s.sessions = append(s.sessions, surface.Session{ID: id, Title: title, PermissionPreset: "smart"})
	if s.messages == nil {
		s.messages = map[string][]surface.Message{}
	}
	s.messages[id] = nil
	s.activeID = id
}

func (s *Store) appendUser(text string) {
	text = strings.TrimSpace(text)
	if text == "" || s.PendingGate() != nil {
		return
	}
	s.seq++
	userID := fmt.Sprintf("msg_demo_%d", s.seq)
	s.seq++
	asstID := fmt.Sprintf("msg_demo_%d", s.seq)
	s.messages[s.activeID] = append(s.messages[s.activeID],
		surface.Message{ID: userID, Role: string(domain.RoleUser), Content: text},
		surface.Message{ID: asstID, Role: string(domain.RoleAssistant), Content: "（demo：未接控制面）"},
	)
}

func (s *Store) decideApproval(decision string) bool {
	if decision != domain.ApprovalApproved && decision != domain.ApprovalDenied {
		return false
	}
	msgs := s.messages[s.activeID]
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
		s.messages[s.activeID] = append(msgs, surface.Message{
			ID:      fmt.Sprintf("msg_demo_%d", s.seq),
			Role:    string(domain.RoleAssistant),
			Content: reply,
		})
		return true
	}
	return false
}

// Ensure Store satisfies surface.Driver at compile time.
var _ surface.Driver = (*Store)(nil)
