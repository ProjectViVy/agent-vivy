// Package surface is the built-in TUI transport alias for the shared core.
// Keeping this import path preserves the kernel face's internal seams while
// ensuring its data and command types have one source of truth.
package surface

import shared "agent-vivy/sdk/tui/surface"

type Session = shared.Session
type ToolCard = shared.ToolCard
type Message = shared.Message
type Gate = shared.Gate
type Meta = shared.Meta
type Driver = shared.Driver
type ErrMsg = shared.ErrMsg
type RefreshMsg = shared.RefreshMsg
type GateResolvedMsg = shared.GateResolvedMsg

const (
	RoleUser      = shared.RoleUser
	RoleAssistant = shared.RoleAssistant
	RoleTool      = shared.RoleTool
)
