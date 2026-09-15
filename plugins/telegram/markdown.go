package telegram

import (
	"fmt"
	"html"
	"regexp"
	"strings"
)

// markdownToHTML converts one reply chunk from the model's markdown into
// the HTML subset Telegram's parse_mode=HTML accepts (tier-1 text loop; a
// read-only rewrite of the picoclaw approach, no import). The conversion
// never fails: markdown it does not understand travels as literal text,
// safely escaped, and a platform rejection of the formatted body falls
// back to plain text in Send.
//
// Mechanism: code blocks, inline code, links, and raw URLs are lifted out
// first and replaced with placeholders so their contents survive the
// escape and the inline rewrites untouched; the remainder is escaped and
// reformatted (headings and blockquote markers stripped, bold/italic/
// strikethrough wrapped, list markers normalized); the placeholders are
// then re-inserted as their HTML equivalents.
func markdownToHTML(text string) string {
	if text == "" {
		return ""
	}

	codeBlocks := extractCodeBlocks(text)
	text = codeBlocks.text

	inlineCodes := extractInlineCodes(text)
	text = inlineCodes.text

	links := extractLinks(text)
	text = links.text

	rawURLs := extractRawURLs(text)
	text = rawURLs.text

	text = reHeading.ReplaceAllString(text, "$1")
	text = reBlockquote.ReplaceAllString(text, "$1")
	text = escapeHTML(text)
	text = reBoldStar.ReplaceAllString(text, "<b>$1</b>")
	text = reBoldUnder.ReplaceAllString(text, "<b>$1</b>")
	text = reItalic.ReplaceAllString(text, "<i>$1</i>")
	text = reStrike.ReplaceAllString(text, "<s>$1</s>")
	text = reListItem.ReplaceAllString(text, "• ")

	for i := 0; 2*i+1 < len(links.items); i++ {
		label := escapeHTML(links.items[2*i])
		url := escapeHTMLAttr(links.items[2*i+1])
		text = strings.ReplaceAll(text, linkPlaceholder(i), fmt.Sprintf(`<a href="%s">%s</a>`, url, label))
	}
	for i, rawURL := range rawURLs.items {
		text = strings.ReplaceAll(text, rawURLPlaceholder(i),
			fmt.Sprintf(`<a href="%s">%s</a>`, escapeHTMLAttr(rawURL), escapeHTML(rawURL)))
	}
	for i, code := range inlineCodes.items {
		text = strings.ReplaceAll(text, inlineCodePlaceholder(i), "<code>"+escapeHTML(code)+"</code>")
	}
	for i, code := range codeBlocks.items {
		text = strings.ReplaceAll(text, codeBlockPlaceholder(i), "<pre><code>"+escapeHTML(code)+"</code></pre>")
	}

	return text
}

// The rewrite patterns are deliberately line-anchored (headings, quotes,
// list markers) — every line of the reply, not just the first.
var (
	reHeading    = regexp.MustCompile(`(?m)^#{1,6}\s+([^\n]+)`)
	reBlockquote = regexp.MustCompile(`(?m)^>\s*(.*)$`)
	reLink       = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	reBoldStar   = regexp.MustCompile(`\*\*(.+?)\*\*`)
	reBoldUnder  = regexp.MustCompile(`__(.+?)__`)
	reItalic     = regexp.MustCompile(`_([^_]+)_`)
	reStrike     = regexp.MustCompile(`~~(.+?)~~`)
	reListItem   = regexp.MustCompile(`(?m)^[-*]\s+`)
	reCodeBlock  = regexp.MustCompile("```[\\w]*\\n?([\\s\\S]*?)```")
	reInlineCode = regexp.MustCompile("`([^`]+)`")
	reRawURL     = regexp.MustCompile(`https?://[^\s<]+`)
)

func linkPlaceholder(i int) string       { return fmt.Sprintf("\x00LK%d\x00", i) }
func rawURLPlaceholder(i int) string     { return fmt.Sprintf("\x00RU%d\x00", i) }
func inlineCodePlaceholder(i int) string { return fmt.Sprintf("\x00IC%d\x00", i) }
func codeBlockPlaceholder(i int) string  { return fmt.Sprintf("\x00CB%d\x00", i) }

// lifted pairs the placeholder-substituted text with what was lifted out
// of it, in order.
type lifted struct {
	text  string
	items []string
}

// liftMatches removes every match of re, records the full match, and
// leaves placeholder(i) behind for each.
func liftMatches(text string, re *regexp.Regexp, ph func(int) string) lifted {
	matches := re.FindAllString(text, -1)
	i := 0
	text = re.ReplaceAllStringFunc(text, func(string) string {
		p := ph(i)
		i++
		return p
	})
	return lifted{text: text, items: matches}
}

// liftGroups removes every match of re, records the first capture group of
// each match, and leaves placeholder(i) behind for each.
func liftGroups(text string, re *regexp.Regexp, ph func(int) string) lifted {
	matches := re.FindAllStringSubmatch(text, -1)
	items := make([]string, 0, len(matches))
	for _, match := range matches {
		items = append(items, match[1])
	}
	i := 0
	text = re.ReplaceAllStringFunc(text, func(string) string {
		p := ph(i)
		i++
		return p
	})
	return lifted{text: text, items: items}
}

// liftLinkPairs removes every [label](url) match, records label/url pairs,
// and leaves placeholder(i) behind for each.
func liftLinkPairs(text string) lifted {
	matches := reLink.FindAllStringSubmatch(text, -1)
	pairs := make([]string, 0, len(matches))
	for _, match := range matches {
		pairs = append(pairs, match[1], match[2])
	}
	i := 0
	text = reLink.ReplaceAllStringFunc(text, func(string) string {
		p := linkPlaceholder(i)
		i++
		return p
	})
	return lifted{text: text, items: pairs}
}

// extractLinks lifts [label](url) links.
func extractLinks(text string) lifted { return liftLinkPairs(text) }

// extractRawURLs lifts bare http(s) URLs.
func extractRawURLs(text string) lifted {
	return liftMatches(text, reRawURL, rawURLPlaceholder)
}

// extractInlineCodes lifts `inline code` bodies.
func extractInlineCodes(text string) lifted {
	return liftGroups(text, reInlineCode, inlineCodePlaceholder)
}

// extractCodeBlocks lifts fenced code block bodies (info string dropped:
// Telegram's HTML subset has no language attribute on pre/code).
func extractCodeBlocks(text string) lifted {
	return liftGroups(text, reCodeBlock, codeBlockPlaceholder)
}

// escapeHTML escapes the three characters Telegram's HTML parse mode
// requires in text bodies. Attributes are escaped with the full stdlib
// rules instead.
func escapeHTML(text string) string {
	text = strings.ReplaceAll(text, "&", "&amp;")
	text = strings.ReplaceAll(text, "<", "&lt;")
	text = strings.ReplaceAll(text, ">", "&gt;")
	return text
}

// escapeHTMLAttr escapes a URL for use inside an href attribute.
func escapeHTMLAttr(text string) string { return html.EscapeString(text) }
