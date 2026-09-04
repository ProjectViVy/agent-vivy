// Package surface is the built-in TUI transport alias for the shared core.
// Keeping this import path preserves the kernel face's internal seams while
// ensuring its data and command types have one source of truth.
package surface

import shared "agent-vivy/sdk/tui/surface"

type Session = shared.Session
type Context = shared.Context
type Sidebar = shared.Sidebar
type ToolCard = shared.ToolCard
type Attachment = shared.Attachment
type FileContext = shared.FileContext
type Message = shared.Message
type Gate = shared.Gate
type Meta = shared.Meta
type Driver = shared.Driver
type CommandExecutor = shared.CommandExecutor
type CommandResultMsg = shared.CommandResultMsg
type ErrMsg = shared.ErrMsg
type RefreshMsg = shared.RefreshMsg
type RestoreInputMsg = shared.RestoreInputMsg
type GateResolvedMsg = shared.GateResolvedMsg
type SidebarProvider = shared.SidebarProvider
type ThinkingController = shared.ThinkingController
type AttachmentProvider = shared.AttachmentProvider
type ContextSender = shared.ContextSender
type FileContextSender = shared.FileContextSender
type ProjectFileCompleter = shared.ProjectFileCompleter
type ProjectFilesMsg = shared.ProjectFilesMsg
type ShellExecutor = shared.ShellExecutor
type CapabilityReporter = shared.CapabilityReporter
type SessionController = shared.SessionController
type SessionsMsg = shared.SessionsMsg

const (
	RoleUser      = shared.RoleUser
	RoleAssistant = shared.RoleAssistant
	RoleTool      = shared.RoleTool
)
