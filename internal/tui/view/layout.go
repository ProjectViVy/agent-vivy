package view

// layout is Crush chat geometry reduced to string cells:
//
//	wide:   [ main+editor | sidebar ]
//	          -------------
//	          help
//
//	compact: header
//	         main
//	         editor
//	         help
//
// No top header bar in wide mode — logo lives in the sidebar (Crush).
type layout struct {
	width, height int
	sidebarW      int
	showSidebar   bool
	headerH       int // 1 in compact, 0 in wide
	editorH       int
	statusH       int
	marginX       int
	marginY       int
}

const (
	sidebarBreakpoint = 100
	defaultSidebarW   = 32 // Crush sidebarWidth
	compactHeaderH    = 1
	editorHeight      = 3
	statusHeight      = 1
	appMarginX        = 1
	appMarginY        = 1
)

func computeLayout(width, height int) layout {
	l := layout{
		width:       max(1, width),
		height:      max(1, height),
		editorH:     editorHeight,
		statusH:     statusHeight,
		showSidebar: width >= sidebarBreakpoint,
		sidebarW:    defaultSidebarW,
		marginX:     appMarginX,
		marginY:     appMarginY,
	}
	if !l.showSidebar {
		l.sidebarW = 0
		l.headerH = compactHeaderH
	}
	// Keep sidebar from eating the chat on mid widths.
	inner := l.innerW()
	if l.showSidebar && l.sidebarW >= inner-40 {
		l.sidebarW = max(18, inner/4)
	}
	return l
}

func (l layout) innerW() int {
	return max(1, l.width-2*l.marginX)
}

func (l layout) innerH() int {
	// top+bottom margin around app; status sits in bottom margin row conceptually
	return max(1, l.height-2*l.marginY-l.statusH)
}

func (l layout) mainW() int {
	w := l.innerW() - l.sidebarW
	if l.showSidebar {
		// 1 col gap between main and sidebar (Crush sideRect.Min.X += 1)
		w = max(1, w-1)
	}
	return max(1, w)
}

func (l layout) mainH() int {
	return max(1, l.innerH()-l.headerH-l.editorH-1) // -1 bottom margin under chat
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
