package view

// layout is the Crush chat geometry reduced to terminal cells. A sidebar is
// only useful when both axes leave enough room for the chat and editor; this
// mirrors Crush's compact decision instead of letting a short terminal grow
// a clipped right rail.
type layout struct {
	width, height int
	sidebarW      int
	showSidebar   bool
	headerH       int // 1 in compact, 0 in wide
	editorH       int
	chromeH       int
	statusH       int
	marginX       int
	marginY       int
}

const (
	sidebarBreakpoint   = 100
	minimumWideHeight   = 30
	defaultSidebarW     = 32
	compactHeaderH      = 1
	editorBorderRows    = 2 // rounded box top + bottom border
	editorChipsRow      = 1 // model · mode · provider chips
	editorAttachmentRow = 1
	editorPasteGuardRow = 1
	maxEditorLines      = 6 // visible composer input lines; taller drafts scroll inside the box
	pasteThresholdChars = 2000
	pasteThresholdLines = 40
	chromeHeight        = 1
	statusHeight        = 0
	appMarginX          = 1
	appMarginY          = 1
)

// editorReserve sizes the composer: rounded border rows, the chips row, the
// attachment and paste-guard rows when present, and the visible input lines.
// Drafts past maxEditorLines keep a fixed box and scroll inside it, so a huge
// paste can never swallow the chat.
func editorReserve(hasAttachments, pasteGuard bool, inputLines int) int {
	lines := min(max(1, inputLines), maxEditorLines)
	reserve := editorBorderRows + editorChipsRow + lines
	if hasAttachments {
		reserve += editorAttachmentRow
	}
	if pasteGuard {
		reserve += editorPasteGuardRow
	}
	return reserve
}

func computeLayout(width, height int) layout {
	l := layout{
		width:       max(1, width),
		height:      max(1, height),
		editorH:     editorReserve(false, false, 1),
		chromeH:     chromeHeight,
		statusH:     statusHeight,
		showSidebar: width >= sidebarBreakpoint && height >= minimumWideHeight,
		sidebarW:    defaultSidebarW,
		marginX:     appMarginX,
		marginY:     appMarginY,
	}
	if !l.showSidebar {
		l.sidebarW = 0
		l.headerH = compactHeaderH
	}
	return l
}

func (l layout) innerW() int {
	return max(1, l.width-2*l.marginX)
}

func (l layout) innerH() int {
	return max(1, l.height-2*l.marginY-l.statusH)
}

func (l layout) mainW() int {
	w := l.innerW() - l.sidebarW
	if l.showSidebar {
		w-- // one cell between chat and sidebar, as in Crush.
	}
	return max(1, w)
}

func (l layout) mainH() int {
	return max(1, l.innerH()-l.headerH-l.editorH-l.chromeH-1)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
