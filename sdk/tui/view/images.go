package view

import (
	"encoding/base64"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"strings"

	"agent-vivy/sdk/tui/surface"

	tea "github.com/charmbracelet/bubbletea"
	_ "golang.org/x/image/webp"
)

// Inline terminal images (VCP-G4): image attachments render inside the
// transcript when the terminal speaks a graphics protocol — kitty (also
// WezTerm) or iTerm2. `images: auto|on|off`; off and undetected keep the
// attachment chips only.
//
// kitty transmit is a one-shot: image bytes go out once per attachment via
// `a=t` and each frame emits only the cheap `a=p` placement. iTerm2 has no
// re-placement protocol, so its whole File= sequence rides inside the frame
// and payloads are capped smaller.

const (
	imageProtoKitty  = "kitty"
	imageProtoITerm2 = "iterm2"
	imageProtoNone   = ""
)

// itermInlineMaxBytes caps the inline-transmitted iTerm2 payload (it repeats
// every repaint); larger images stay on the chip path.
const itermInlineMaxBytes = 768 * 1024

// imageMaxRows bounds the transcript rows an inline image may occupy.
const imageMaxRows = 18

type imageCache struct {
	protocol string
	nextID   uint32
	byPath   map[string]uint32 // path -> kitty image id (transmit once)
	iterm    map[string]string // path -> cached iTerm2 File= sequence
}

// detectImageProtocol probes the environment for a graphics-capable
// terminal: kitty marks KITTY_WINDOW_ID/TERM, iTerm2 sets TERM_PROGRAM, and
// WezTerm implements the kitty protocol.
func detectImageProtocol(env func(string) string) string {
	if env("KITTY_WINDOW_ID") != "" || strings.Contains(env("TERM"), "kitty") {
		return imageProtoKitty
	}
	switch env("TERM_PROGRAM") {
	case "iTerm.app":
		return imageProtoITerm2
	case "WezTerm":
		return imageProtoKitty
	}
	return imageProtoNone
}

// resolveImageProtocol maps the `images` option to the active protocol.
// "on" forces kitty when detection fails (operator asked for it); "auto"
// only enables detected terminals; anything else means off.
func resolveImageProtocol(option string, env func(string) string) string {
	switch strings.ToLower(strings.TrimSpace(option)) {
	case "off":
		return imageProtoNone
	case "on":
		if proto := detectImageProtocol(env); proto != imageProtoNone {
			return proto
		}
		return imageProtoKitty
	default: // "auto", "", unknown
		return detectImageProtocol(env)
	}
}

func isImageAttachment(att surface.Attachment) bool {
	return att.Path != "" && strings.HasPrefix(strings.ToLower(att.MimeType), "image/")
}

// imageTransmitCmds emits the one-shot kitty transmit sequences for image
// attachments not yet sent. Called from Update; the map check makes idle
// frames free.
func (m Model) imageTransmitCmds() []tea.Cmd {
	state := m.imageState
	if state == nil || state.protocol != imageProtoKitty {
		return nil
	}
	var cmds []tea.Cmd
	for _, message := range m.driver.ActiveMessages() {
		for _, att := range message.Attachments {
			if !isImageAttachment(att) {
				continue
			}
			if _, sent := state.byPath[att.Path]; sent {
				continue
			}
			data, err := os.ReadFile(att.Path)
			if err != nil || len(data) == 0 {
				state.byPath[att.Path] = 0 // 0 = failed; never retry
				continue
			}
			state.nextID++
			state.byPath[att.Path] = state.nextID
			cmds = append(cmds, tea.Println(kittyTransmitSequence(state.nextID, data)))
		}
	}
	return cmds
}

// kittyTransmitSequence is the a=t payload upload: base64 in 4096-char
// chunks with m=1 continuation. f=100 keeps the container format (png/jpeg/
// gif/webp) so kitty decodes it; q=2 suppresses the terminal's response.
func kittyTransmitSequence(id uint32, data []byte) string {
	encoded := base64.StdEncoding.EncodeToString(data)
	var b strings.Builder
	for len(encoded) > 0 {
		n := min(len(encoded), 4096)
		chunk := encoded[:n]
		encoded = encoded[n:]
		more := 1
		if len(encoded) == 0 {
			more = 0
		}
		fmt.Fprintf(&b, "\x1b_Ga=t,f=100,t=d,q=2,i=%d,m=%d;%s\x1b\\", id, more, chunk)
	}
	return b.String()
}

// itermFileSequence is the OSC 1337 inline File= form, transmitting and
// displaying in one shot. Returns "" for unreadable/oversized payloads.
func itermFileSequence(path string) string {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 || len(data) > itermInlineMaxBytes {
		return ""
	}
	return fmt.Sprintf("\x1b]1337;File=inline=1;preserveAspectRatio=1:%s\x07",
		base64.StdEncoding.EncodeToString(data))
}

// imageCellRows converts pixel dimensions to a transcript row reservation:
// terminal cells are roughly twice as tall as wide, capped at imageMaxRows.
func imageCellRows(pixelW, pixelH, cols int) int {
	if pixelW <= 0 || pixelH <= 0 || cols <= 0 {
		return 0
	}
	rows := (pixelH*cols*2/pixelW + 1) / 2
	if rows < 1 {
		rows = 1
	}
	return min(rows, imageMaxRows)
}

// imageDimensions reads the image header only.
func imageDimensions(path string) (w, h int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	return cfg.Width, cfg.Height, err
}

// renderImageAttachment yields the transcript rows for an inline image:
// a placement sequence followed by blank rows that hold the space. Returns
// nil when the image cannot render (missing file, unreadable dims, or an
// oversized iTerm2 payload) so the caller falls back to the chip.
func (m Model) renderImageAttachment(att surface.Attachment, width int) []string {
	state := m.imageState
	if state == nil || state.protocol == imageProtoNone || !isImageAttachment(att) {
		return nil
	}
	pixelW, pixelH, err := imageDimensions(att.Path)
	if err != nil {
		return nil
	}
	cols := min(60, max(1, width-4))
	rows := imageCellRows(pixelW, pixelH, cols)
	if rows == 0 {
		return nil
	}
	var placement string
	switch state.protocol {
	case imageProtoKitty:
		id, sent := state.byPath[att.Path]
		if !sent || id == 0 {
			return nil // upload failed or not issued yet; chip stands in
		}
		placement = fmt.Sprintf("\x1b_Ga=p,i=%d,c=%d,r=%d\x1b\\", id, cols, rows)
	case imageProtoITerm2:
		seq, cached := state.iterm[att.Path]
		if !cached {
			seq = itermFileSequence(att.Path)
			state.iterm[att.Path] = seq
		}
		if seq == "" {
			return nil
		}
		placement = seq
	default:
		return nil
	}
	return append([]string{placement}, make([]string, rows-1)...)
}
