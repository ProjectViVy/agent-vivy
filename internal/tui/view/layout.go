package view

// layout rects are character cells, Crush-reduced: header, sidebar, main, editor, status.
type layout struct {
	width, height int
	sidebarW      int
	showSidebar   bool
	headerH       int
	editorH       int
	statusH       int
}

const (
	sidebarBreakpoint = 100
	defaultSidebarW   = 22
	headerHeight      = 1
	editorHeight      = 3
	statusHeight      = 1
)

func computeLayout(width, height int) layout {
	l := layout{
		width:       max(1, width),
		height:      max(1, height),
		headerH:     headerHeight,
		editorH:     editorHeight,
		statusH:     statusHeight,
		showSidebar: width >= sidebarBreakpoint,
		sidebarW:    defaultSidebarW,
	}
	if !l.showSidebar {
		l.sidebarW = 0
	}
	if l.sidebarW >= l.width {
		l.sidebarW = max(0, l.width/4)
	}
	return l
}

func (l layout) mainW() int {
	return max(1, l.width-l.sidebarW)
}

func (l layout) mainH() int {
	return max(1, l.height-l.headerH-l.editorH-l.statusH)
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
