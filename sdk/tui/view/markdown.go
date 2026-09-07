package view

import (
	"fmt"
	"strings"
	"sync"
	"unicode"

	"github.com/charmbracelet/glamour"
	glansi "github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

const (
	markdownMaxWidth   = 120
	markdownListIndent = 2
	markdownCodeMargin = 2
)

type mdRendererKey struct {
	width int
	quiet bool
}

var (
	mdCacheMu    sync.Mutex
	mdRenderMu   sync.Mutex
	mdRenderers  = map[mdRendererKey]*glamour.TermRenderer{}
	errMarkdownW = fmt.Errorf("markdown width")
)

func sanitizeMarkdownSource(text string) string {
	text = ansi.Strip(text)
	text = strings.ReplaceAll(text, "\t", "    ")
	return strings.Map(func(r rune) rune {
		if r == '\n' || (!unicode.IsControl(r) && !isBidiControl(r)) {
			return r
		}
		return -1
	}, text)
}

func markdownWrapWidth(contentWidth int) int {
	if contentWidth < 1 {
		return 1
	}
	if contentWidth > markdownMaxWidth {
		return markdownMaxWidth
	}
	return contentWidth
}

func renderMarkdown(source string, width int, quiet bool) (string, error) {
	if width < 1 {
		return "", errMarkdownW
	}
	if source == "" {
		return "", nil
	}
	mdRenderMu.Lock()
	defer mdRenderMu.Unlock()
	return renderMarkdownLocked(source, width, quiet)
}

// renderMarkdownLocked renders with a shared renderer; the caller must hold
// mdRenderMu so multi-fragment streaming renders cannot interleave with other
// renders on the same goldmark state.
func renderMarkdownLocked(source string, width int, quiet bool) (string, error) {
	renderer, err := markdownRenderer(width, quiet)
	if err != nil {
		return "", err
	}
	out, err := renderer.Render(source)
	if err != nil {
		return "", err
	}
	return trimMarkdownOutput(out), nil
}

func trimMarkdownOutput(out string) string {
	return strings.Trim(out, "\n\r")
}

func markdownRenderer(width int, quiet bool) (*glamour.TermRenderer, error) {
	key := mdRendererKey{width: width, quiet: quiet}
	mdCacheMu.Lock()
	defer mdCacheMu.Unlock()
	if renderer, ok := mdRenderers[key]; ok {
		return renderer, nil
	}
	styles := markdownStyle()
	if quiet {
		styles = quietMarkdownStyle()
	}
	renderer, err := glamour.NewTermRenderer(
		glamour.WithStyles(styles),
		glamour.WithWordWrap(width),
		glamour.WithColorProfile(termenv.TrueColor),
	)
	if err != nil {
		return nil, err
	}
	mdRenderers[key] = renderer
	return renderer, nil
}

func markdownStyle() glansi.StyleConfig {
	fg := hex(paletteFg)
	info := hex(paletteSecondary)
	primary := hex(palettePrimary)
	onPrimary := hex(paletteOnPrimary)
	muted := hex(paletteMuted)
	subtle := hex(paletteSubtle)
	success := hex(paletteSuccess)
	warn := hex(paletteWarn)
	danger := hex(paletteDanger)
	codeBg := hex(paletteCodeBg)
	str := hex(paletteString)
	link := hex(paletteLink)
	user := hex(paletteUser)
	yes := boolPtr(true)
	no := boolPtr(false)
	indent := uintPtr(1)
	margin := uintPtr(markdownCodeMargin)
	quoteToken := "│ "
	return glansi.StyleConfig{
		Document: glansi.StyleBlock{
			StylePrimitive: glansi.StylePrimitive{Color: fg},
		},
		BlockQuote: glansi.StyleBlock{
			Indent:      indent,
			IndentToken: &quoteToken,
		},
		List: glansi.StyleList{
			LevelIndent: markdownListIndent,
		},
		Heading: glansi.StyleBlock{
			StylePrimitive: glansi.StylePrimitive{
				BlockSuffix: "\n",
				Color:       info,
				Bold:        yes,
			},
		},
		H1: glansi.StyleBlock{
			StylePrimitive: glansi.StylePrimitive{
				Prefix:          " ",
				Suffix:          " ",
				Color:           onPrimary,
				BackgroundColor: primary,
				Bold:            yes,
			},
		},
		H2: glansi.StyleBlock{
			StylePrimitive: glansi.StylePrimitive{Prefix: "## "},
		},
		H3: glansi.StyleBlock{
			StylePrimitive: glansi.StylePrimitive{Prefix: "### "},
		},
		H4: glansi.StyleBlock{
			StylePrimitive: glansi.StylePrimitive{Prefix: "#### "},
		},
		H5: glansi.StyleBlock{
			StylePrimitive: glansi.StylePrimitive{Prefix: "##### "},
		},
		H6: glansi.StyleBlock{
			StylePrimitive: glansi.StylePrimitive{
				Prefix: "###### ",
				Color:  muted,
				Bold:   no,
			},
		},
		Strikethrough: glansi.StylePrimitive{CrossedOut: yes},
		Emph:          glansi.StylePrimitive{Italic: yes},
		Strong:        glansi.StylePrimitive{Bold: yes},
		HorizontalRule: glansi.StylePrimitive{
			Color:  subtle,
			Format: "\n--------\n",
		},
		Item:        glansi.StylePrimitive{BlockPrefix: "• "},
		Enumeration: glansi.StylePrimitive{BlockPrefix: ". "},
		Task: glansi.StyleTask{
			Ticked:   "[✓] ",
			Unticked: "[ ] ",
		},
		Link: glansi.StylePrimitive{
			Color:     link,
			Underline: yes,
		},
		LinkText: glansi.StylePrimitive{
			Color: success,
			Bold:  yes,
		},
		Image: glansi.StylePrimitive{
			Color:     user,
			Underline: yes,
		},
		ImageText: glansi.StylePrimitive{
			Color:  muted,
			Format: "Image: {{.text}} →",
		},
		Code: glansi.StyleBlock{
			StylePrimitive: glansi.StylePrimitive{
				Prefix:          " ",
				Suffix:          " ",
				Color:           danger,
				BackgroundColor: codeBg,
			},
		},
		CodeBlock: glansi.StyleCodeBlock{
			StyleBlock: glansi.StyleBlock{
				StylePrimitive: glansi.StylePrimitive{Color: muted},
				Margin:         margin,
			},
			Chroma: &glansi.Chroma{
				Text:                glansi.StylePrimitive{Color: fg},
				Error:               glansi.StylePrimitive{Color: onPrimary, BackgroundColor: danger},
				Comment:             glansi.StylePrimitive{Color: muted},
				CommentPreproc:      glansi.StylePrimitive{Color: warn},
				Keyword:             glansi.StylePrimitive{Color: info},
				KeywordReserved:     glansi.StylePrimitive{Color: primary},
				KeywordNamespace:    glansi.StylePrimitive{Color: primary},
				KeywordType:         glansi.StylePrimitive{Color: user},
				Operator:            glansi.StylePrimitive{Color: danger},
				Punctuation:         glansi.StylePrimitive{Color: warn},
				Name:                glansi.StylePrimitive{Color: fg},
				NameBuiltin:         glansi.StylePrimitive{Color: user},
				NameTag:             glansi.StylePrimitive{Color: primary},
				NameAttribute:       glansi.StylePrimitive{Color: info},
				NameClass:           glansi.StylePrimitive{Color: fg, Bold: yes, Underline: yes},
				NameConstant:        glansi.StylePrimitive{Color: user},
				NameDecorator:       glansi.StylePrimitive{Color: warn},
				NameException:       glansi.StylePrimitive{Color: danger},
				NameFunction:        glansi.StylePrimitive{Color: success},
				NameOther:           glansi.StylePrimitive{Color: fg},
				Literal:             glansi.StylePrimitive{Color: str},
				LiteralNumber:       glansi.StylePrimitive{Color: success},
				LiteralDate:         glansi.StylePrimitive{Color: info},
				LiteralString:       glansi.StylePrimitive{Color: str},
				LiteralStringEscape: glansi.StylePrimitive{Color: success},
				GenericDeleted:      glansi.StylePrimitive{Color: danger},
				GenericEmph:         glansi.StylePrimitive{Italic: yes},
				GenericInserted:     glansi.StylePrimitive{Color: success},
				GenericStrong:       glansi.StylePrimitive{Bold: yes},
				GenericSubheading:   glansi.StylePrimitive{Color: muted},
				Background:          glansi.StylePrimitive{BackgroundColor: codeBg},
			},
		},
	}
}

func quietMarkdownStyle() glansi.StyleConfig {
	fg := hex(paletteMuted)
	bg := hex(paletteCodeBg)
	yes := boolPtr(true)
	no := boolPtr(false)
	indent := uintPtr(1)
	margin := uintPtr(markdownCodeMargin)
	quoteToken := "│ "
	plain := glansi.StylePrimitive{Color: fg, BackgroundColor: bg}
	return glansi.StyleConfig{
		Document: glansi.StyleBlock{StylePrimitive: plain},
		BlockQuote: glansi.StyleBlock{
			StylePrimitive: plain,
			Indent:         indent,
			IndentToken:    &quoteToken,
		},
		List: glansi.StyleList{LevelIndent: markdownListIndent},
		Heading: glansi.StyleBlock{
			StylePrimitive: glansi.StylePrimitive{
				BlockSuffix:     "\n",
				Bold:            yes,
				Color:           fg,
				BackgroundColor: bg,
			},
		},
		H1: glansi.StyleBlock{
			StylePrimitive: glansi.StylePrimitive{
				Prefix:          " ",
				Suffix:          " ",
				Bold:            yes,
				Color:           fg,
				BackgroundColor: bg,
			},
		},
		H2:            glansi.StyleBlock{StylePrimitive: glansi.StylePrimitive{Prefix: "## ", Color: fg, BackgroundColor: bg}},
		H3:            glansi.StyleBlock{StylePrimitive: glansi.StylePrimitive{Prefix: "### ", Color: fg, BackgroundColor: bg}},
		H4:            glansi.StyleBlock{StylePrimitive: glansi.StylePrimitive{Prefix: "#### ", Color: fg, BackgroundColor: bg}},
		H5:            glansi.StyleBlock{StylePrimitive: glansi.StylePrimitive{Prefix: "##### ", Color: fg, BackgroundColor: bg}},
		H6:            glansi.StyleBlock{StylePrimitive: glansi.StylePrimitive{Prefix: "###### ", Color: fg, BackgroundColor: bg, Bold: no}},
		Strikethrough: glansi.StylePrimitive{CrossedOut: yes, Color: fg, BackgroundColor: bg},
		Emph:          glansi.StylePrimitive{Italic: yes, Color: fg, BackgroundColor: bg},
		Strong:        glansi.StylePrimitive{Bold: yes, Color: fg, BackgroundColor: bg},
		HorizontalRule: glansi.StylePrimitive{
			Color:           fg,
			BackgroundColor: bg,
			Format:          "\n--------\n",
		},
		Item:        glansi.StylePrimitive{BlockPrefix: "• ", Color: fg, BackgroundColor: bg},
		Enumeration: glansi.StylePrimitive{BlockPrefix: ". ", Color: fg, BackgroundColor: bg},
		Task: glansi.StyleTask{
			StylePrimitive: plain,
			Ticked:         "[✓] ",
			Unticked:       "[ ] ",
		},
		Link:     glansi.StylePrimitive{Color: fg, BackgroundColor: bg, Underline: yes},
		LinkText: glansi.StylePrimitive{Color: fg, BackgroundColor: bg, Bold: yes},
		Code: glansi.StyleBlock{
			StylePrimitive: glansi.StylePrimitive{
				Prefix:          " ",
				Suffix:          " ",
				Color:           fg,
				BackgroundColor: bg,
			},
		},
		CodeBlock: glansi.StyleCodeBlock{
			StyleBlock: glansi.StyleBlock{
				StylePrimitive: plain,
				Margin:         margin,
			},
		},
	}
}

func hex(color string) *string { return &color }

func boolPtr(v bool) *bool { return &v }

func uintPtr(v uint) *uint { return &v }
