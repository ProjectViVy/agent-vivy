package view

import (
	"strings"
	"sync"
)

// streamMarkdownRender renders a growing streaming bubble at the given wrap
// width, reusing a cached "stable prefix" glamour render so each flush only
// re-renders the trailing portion (ported from crush's
// internal/ui/chat/streaming_markdown.go; board row TUI-MD-STREAM-CACHE).
//
// The stable/trailing cut sits immediately after a blank line at which no
// markdown construct can be open (fenced code, list, table, quote, setext
// heading, HTML block, link reference). Two concatenated glamour renders are
// not generally equal to one render of the whole document, so the boundary
// detection is deliberately conservative: any doubt falls back to a full
// render and leaves the cache untouched.
//
// Entries are keyed by (message id, quiet) and bounded; a width change or a
// non-prefix extension resets the entry. Rendering errors are returned so the
// caller can fall back to its plain-text path.
func streamMarkdownRender(id, source string, width int, quiet bool) (string, error) {
	if width < 1 {
		return "", errMarkdownW
	}
	if source == "" {
		return "", nil
	}
	streamMu.Lock()
	defer streamMu.Unlock()
	key := streamEntryKey{id: id, quiet: quiet}
	entry := streamEntries[key]
	if entry == nil {
		if len(streamEntries) >= streamEntryLimit {
			streamEntries = map[streamEntryKey]*streamEntry{}
		}
		entry = &streamEntry{}
		streamEntries[key] = entry
	}
	// The shared glamour renderers are stateful; hold mdRenderMu across the
	// whole prefix+trailing sequence so no other render interleaves.
	mdRenderMu.Lock()
	defer mdRenderMu.Unlock()
	if entry.width != width || !strings.HasPrefix(source, entry.stablePrefix) {
		entry.reset(width)
		out, err := renderMarkdownLocked(source, width, quiet)
		if err != nil {
			return "", err
		}
		entry.trySeed(source, width, quiet)
		return out, nil
	}
	boundary, haveBoundary := entry.findBoundaryAfter(source)
	if !haveBoundary {
		// No safe boundary anywhere yet; a later flush may find one.
		return renderMarkdownLocked(source, width, quiet)
	}
	if boundary <= len(entry.stablePrefix) {
		// The cached prefix already covers an at-least-as-late boundary.
		trail, err := entry.renderTrailing(source[len(entry.stablePrefix):], width, quiet)
		if err != nil {
			return "", err
		}
		return glueRenders(entry.stablePrefixRender, trail), nil
	}
	newChunk := source[len(entry.stablePrefix):boundary]
	chunkRender, err := entry.renderTrailing(newChunk, width, quiet)
	if err != nil {
		return "", err
	}
	entry.stablePrefixRender = glueRenders(entry.stablePrefixRender, chunkRender)
	entry.stablePrefix = source[:boundary]
	entry.baseFenceCount += countFenceLines(newChunk)
	entry.baseHasListMarker = entry.baseHasListMarker || chunkHasListMarker(newChunk)
	trail := source[boundary:]
	if trail == "" {
		return entry.stablePrefixRender, nil
	}
	trailRender, err := entry.renderTrailing(trail, width, quiet)
	if err != nil {
		return "", err
	}
	return glueRenders(entry.stablePrefixRender, trailRender), nil
}

var (
	streamMu      sync.Mutex
	streamEntries = map[streamEntryKey]*streamEntry{}
)

const streamEntryLimit = 8

type streamEntryKey struct {
	id    string
	quiet bool
}

// streamEntry caches the render of a stable content prefix plus the
// cumulative scan state needed to validate new boundary candidates in O(delta)
// instead of re-scanning the whole prefix.
type streamEntry struct {
	width              int
	stablePrefix       string
	stablePrefixRender string
	// baseFenceCount is always even (safe boundaries require even fence
	// parity), so the delta scan always starts outside a fence.
	baseFenceCount    int
	baseHasListMarker bool
}

func (e *streamEntry) reset(width int) {
	e.width = width
	e.stablePrefix = ""
	e.stablePrefixRender = ""
	e.baseFenceCount = 0
	e.baseHasListMarker = false
}

// trySeed pays one extra prefix render after an unavoidable full render so
// the next flush can start from a cached boundary.
func (e *streamEntry) trySeed(source string, width int, quiet bool) {
	boundary := findSafeMarkdownBoundary(source)
	if boundary <= 0 {
		return
	}
	out, err := renderMarkdownLocked(source[:boundary], width, quiet)
	if err != nil {
		return
	}
	e.stablePrefix = source[:boundary]
	e.stablePrefixRender = trimGlamourMargins(out)
	e.baseFenceCount = countFenceLines(e.stablePrefix)
	e.baseHasListMarker = chunkHasListMarker(e.stablePrefix)
}

// findBoundaryAfter reports the latest safe boundary in content strictly
// after the stable prefix. haveBoundary=false means no safe boundary exists
// anywhere yet (full render, cache untouched). A boundary equal to the
// current prefix length means "render the trail fresh, keep the cache".
func (e *streamEntry) findBoundaryAfter(content string) (int, bool) {
	if e.stablePrefix == "" {
		p := findSafeMarkdownBoundary(content)
		return p, p >= 0
	}
	for p := blankLineBefore(content, len(content)); p > len(e.stablePrefix); p = blankLineBefore(content, p-1) {
		if e.isSafeBoundaryIncremental(content, p) {
			return p, true
		}
	}
	return len(e.stablePrefix), true
}

// isSafeBoundaryIncremental validates a boundary candidate at position p
// using the cached cumulative state plus a delta scan of
// content[len(stablePrefix):p].
func (e *streamEntry) isSafeBoundaryIncremental(content string, p int) bool {
	delta := content[len(e.stablePrefix):p]

	// Fence parity: base count + delta count must be even.
	if (e.baseFenceCount+countFenceLines(delta))%2 != 0 {
		return false
	}

	// HTML and link-ref hazards anywhere in the delta.
	if deltaHasHTMLorRef(delta) {
		return false
	}

	// List hazard: if a list marker exists anywhere before the boundary
	// (base or delta), the last non-blank line must not be an indented
	// continuation paragraph without its own marker (loose-list check).
	hasListMarker := e.baseHasListMarker || chunkHasListMarker(delta)
	if hasListMarker {
		lastLine := lastNonBlankLine(content[:p])
		if lastLine != "" && !isListItemMarker(strings.TrimLeft(lastLine, " \t")) {
			if lastLine[0] == ' ' || lastLine[0] == '\t' {
				return false
			}
		}
	}

	// The last non-blank line must not open a construct.
	lastLine := lastNonBlankLine(content[:p])
	if lastLine != "" && lineOpensConstruct(lastLine) {
		return false
	}

	// A setext underline after the boundary would retroactively turn the
	// last paragraph of the prefix into a heading.
	if rest := content[p:]; rest != "" {
		if isSetextUnderlineCandidate(firstNonBlankLine(rest)) {
			return false
		}
	}

	return true
}

func (e *streamEntry) renderTrailing(text string, width int, quiet bool) (string, error) {
	if text == "" {
		return "", nil
	}
	out, err := renderMarkdownLocked(text, width, quiet)
	if err != nil {
		return "", err
	}
	return trimGlamourMargins(out), nil
}

// glueRenders concatenates two glamour-rendered fragments with a single blank
// line separator; trimming both sides prevents a visible double-margin seam.
func glueRenders(prefix, trail string) string {
	prefix = trimGlamourMargins(prefix)
	trail = trimGlamourMargins(trail)
	switch {
	case prefix == "" && trail == "":
		return ""
	case prefix == "":
		return trail
	case trail == "":
		return prefix
	default:
		return prefix + "\n\n" + trail
	}
}

func trimGlamourMargins(s string) string {
	return strings.Trim(s, " \t\n")
}

// findSafeMarkdownBoundary returns the byte offset of the end of the latest
// safe boundary in content (always immediately after a blank-line separator),
// or -1 when none exists. SAFETY FIRST: any doubt returns -1.
func findSafeMarkdownBoundary(content string) int {
	if len(content) == 0 {
		return -1
	}
	for p := blankLineBefore(content, len(content)); p > 0; p = blankLineBefore(content, p-1) {
		if isSafeBoundaryAt(content, p) {
			return p
		}
	}
	return -1
}

// blankLineBefore returns the byte offset of the first character after the
// latest blank-line separator ending strictly before `until`, or -1.
func blankLineBefore(content string, until int) int {
	if until <= 0 {
		return -1
	}
	end := until
	for end > 0 {
		nl := strings.LastIndexByte(content[:end], '\n')
		if nl < 0 {
			return -1
		}
		prev := strings.LastIndexByte(content[:nl], '\n')
		for prev >= 0 {
			if isBlankOrSpaces(content[prev+1 : nl]) {
				return nl + 1
			}
			break
		}
		end = nl
	}
	return -1
}

func isBlankOrSpaces(s string) bool {
	for i := range len(s) {
		if s[i] != ' ' && s[i] != '\t' {
			return false
		}
	}
	return true
}

// isSafeBoundaryAt reports whether content[:p] is a safe stable prefix. Three
// "anywhere in the prefix" hazards force a reject because they cannot be
// reliably reasoned about from the trailing line alone.
func isSafeBoundaryAt(content string, p int) bool {
	prefix := content[:p]

	// Even number of fence lines: no open fenced block.
	if countFenceLines(prefix)%2 != 0 {
		return false
	}

	if prefixHasOpenHazard(prefix) {
		return false
	}

	if lastLine := lastNonBlankLine(prefix); lastLine != "" && lineOpensConstruct(lastLine) {
		return false
	}

	if rest := content[p:]; rest != "" && isSetextUnderlineCandidate(firstNonBlankLine(rest)) {
		return false
	}

	return true
}

// prefixHasOpenHazard reports whether prefix contains a construct that cannot
// be safely cut at a blank-line boundary:
//
//	B1 (loose lists): a list is potentially open when a marker appeared
//	   earlier and the last non-blank line is indented without being a
//	   marker itself (continuation paragraph). A non-indented last line
//	   means the list was closed, so boundaries after closed lists survive.
//	B2 (HTML blocks): any HTML-block opener anywhere forces a reject.
//	B3 (reference link definitions): any ref-def opener anywhere forces a
//	   reject — the suffix may reference it and each half renders as an
//	   independent document.
func prefixHasOpenHazard(prefix string) bool {
	inFence := false
	hasListMarker := false
	var lastNonBlankTrimmed string
	var lastNonBlankRaw string
	for line := range splitLines(prefix) {
		if isFenceLine(line) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		trimmed := strings.TrimLeft(line, " \t")
		if trimmed == "" {
			continue
		}
		lastNonBlankTrimmed = trimmed
		lastNonBlankRaw = line
		if isListItemMarker(trimmed) {
			hasListMarker = true
		}
		if isHTMLBlockOpener(line) || isLinkRefDefinition(line) {
			return true
		}
	}
	if hasListMarker && lastNonBlankTrimmed != "" && !isListItemMarker(lastNonBlankTrimmed) {
		if lastNonBlankRaw[0] == ' ' || lastNonBlankRaw[0] == '\t' {
			return true
		}
	}
	return false
}

// deltaHasHTMLorRef reports whether the delta between the stable prefix and a
// boundary candidate contains an HTML block opener or link reference
// definition (outside fenced code blocks).
func deltaHasHTMLorRef(delta string) bool {
	inFence := false
	for line := range splitLines(delta) {
		if isFenceLine(line) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if isHTMLBlockOpener(line) || isLinkRefDefinition(line) {
			return true
		}
	}
	return false
}

// chunkHasListMarker reports whether any line in chunk is a list-item marker
// (outside fenced code blocks).
func chunkHasListMarker(chunk string) bool {
	inFence := false
	for line := range splitLines(chunk) {
		if isFenceLine(line) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if isListItemMarker(strings.TrimLeft(line, " \t")) {
			return true
		}
	}
	return false
}

// countFenceLines counts lines that open or close a fenced code block: first
// non-whitespace run is at least three backticks or tildes. An even count
// means every opened fence has been closed.
func countFenceLines(s string) int {
	n := 0
	for line := range splitLines(s) {
		if isFenceLine(line) {
			n++
		}
	}
	return n
}

func isFenceLine(line string) bool {
	i := 0
	for i < len(line) && i < 3 && line[i] == ' ' {
		i++
	}
	if i >= len(line) {
		return false
	}
	c := line[i]
	if c != '`' && c != '~' {
		return false
	}
	run := 0
	for i < len(line) && line[i] == c {
		i++
		run++
	}
	return run >= 3
}

func lastNonBlankLine(s string) string {
	last := ""
	for line := range splitLines(s) {
		if strings.TrimSpace(line) != "" {
			last = line
		}
	}
	return last
}

func firstNonBlankLine(s string) string {
	for line := range splitLines(s) {
		if strings.TrimSpace(line) != "" {
			return line
		}
	}
	return ""
}

// splitLines yields the lines of s without their terminators; the final
// segment is yielded even when not newline-terminated.
func splitLines(s string) func(yield func(string) bool) {
	return func(yield func(string) bool) {
		start := 0
		for i := 0; i < len(s); i++ {
			if s[i] == '\n' {
				if !yield(s[start:i]) {
					return
				}
				start = i + 1
			}
		}
		if start <= len(s)-1 {
			yield(s[start:])
		}
	}
}

// lineOpensConstruct reports whether line keeps a markdown construct open
// across the boundary. Err on the conservative side.
func lineOpensConstruct(line string) bool {
	if len(line) > 0 && line[0] == '\t' {
		return true
	}
	if strings.HasPrefix(line, "    ") {
		return true
	}
	trimmed := strings.TrimLeft(line, " \t")
	if trimmed == "" {
		return false
	}
	if trimmed[0] == '>' {
		return true
	}
	if isListItemMarker(trimmed) {
		return true
	}
	// Table: any pipe anywhere in the line. Pipe-in-prose is rare and the
	// cost of bailing is one slow frame.
	if strings.ContainsRune(line, '|') {
		return true
	}
	if isSetextUnderlineCandidate(trimmed) {
		return true
	}
	return false
}

// isListItemMarker reports whether line (already left-trimmed) starts with a
// CommonMark list-item marker followed by a space or tab.
func isListItemMarker(line string) bool {
	if line == "" {
		return false
	}
	c := line[0]
	if c == '-' || c == '*' || c == '+' {
		return len(line) >= 2 && (line[1] == ' ' || line[1] == '\t')
	}
	i := 0
	for i < len(line) && line[i] >= '0' && line[i] <= '9' {
		i++
	}
	if i == 0 || i > 9 || i >= len(line) {
		return false
	}
	if line[i] != '.' && line[i] != ')' {
		return false
	}
	if i+1 >= len(line) {
		return false
	}
	return line[i+1] == ' ' || line[i+1] == '\t'
}

// isSetextUnderlineCandidate reports whether line (with optional leading
// whitespace) consists entirely of '=' or entirely of '-' characters with
// optional trailing whitespace.
func isSetextUnderlineCandidate(line string) bool {
	i := 0
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	if i == len(line) {
		return false
	}
	c := line[i]
	if c != '=' && c != '-' {
		return false
	}
	j := i
	for j < len(line) && line[j] == c {
		j++
	}
	for j < len(line) {
		if line[j] != ' ' && line[j] != '\t' {
			return false
		}
		j++
	}
	return j-i >= 1
}

// isHTMLBlockOpener reports whether line begins one of the seven CommonMark
// HTML block patterns (up to three leading spaces accepted). Matching is
// intentionally loose.
func isHTMLBlockOpener(line string) bool {
	i := 0
	for i < len(line) && i < 3 && line[i] == ' ' {
		i++
	}
	rest := line[i:]
	if len(rest) < 2 || rest[0] != '<' {
		return false
	}
	if strings.HasPrefix(rest, "<!--") || strings.HasPrefix(rest, "<?") || strings.HasPrefix(rest, "<![CDATA[") {
		return true
	}
	if len(rest) >= 3 && rest[1] == '!' && isASCIILetter(rest[2]) {
		return true
	}
	low := strings.ToLower(rest)
	for _, t := range []string{"<script", "<pre", "<style", "<textarea"} {
		if strings.HasPrefix(low, t) {
			next := byte(0)
			if len(low) > len(t) {
				next = low[len(t)]
			}
			if next == 0 || next == ' ' || next == '\t' || next == '>' {
				return true
			}
		}
	}
	// Types 6 & 7 collapsed into one check: the line starts (after up to
	// three spaces) with '<' or '</' followed by an ASCII letter.
	j := 1
	if j < len(rest) && rest[j] == '/' {
		j++
	}
	if j >= len(rest) || !isASCIILetter(rest[j]) {
		return false
	}
	return true
}

func isASCIILetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// isLinkRefDefinition reports whether line matches the conservative
// CommonMark link reference definition opener:
//
//	^[ ]{0,3}\[[^\]]+\]:\s*\S+
func isLinkRefDefinition(line string) bool {
	i := 0
	for i < len(line) && i < 3 && line[i] == ' ' {
		i++
	}
	if i >= len(line) || line[i] != '[' {
		return false
	}
	i++
	labelStart := i
	for i < len(line) && line[i] != ']' {
		i++
	}
	if i >= len(line) || i == labelStart {
		return false
	}
	i++
	if i >= len(line) || line[i] != ':' {
		return false
	}
	i++
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	return i < len(line)
}
