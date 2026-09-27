package telegram

import "testing"

// TestMarkdownToHTML pins the conversion shapes the reply path relies on:
// formatting wrappers, lifting (code and links survive the escape and the
// inline rewrites), line-anchored rewrites across all lines, and escaped
// literal text for anything the HTML subset does not model.
func TestMarkdownToHTML(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"plain stays plain", "just words", "just words"},
		{"bold stars", "**bold**", "<b>bold</b>"},
		{"bold underscore", "__bold__", "<b>bold</b>"},
		{"italic", "_soft_", "<i>soft</i>"},
		{"strikethrough", "~~gone~~", "<s>gone</s>"},
		{"inline code escapes its body", "`a<b & c`", "<code>a&lt;b &amp; c</code>"},
		{
			"fenced block drops the info string and escapes",
			"```go\nx < 1 && y > 2\n```",
			"<pre><code>x &lt; 1 &amp;&amp; y &gt; 2\n</code></pre>",
		},
		{"link", "[vivy](https://example.com/a?b=1)", `<a href="https://example.com/a?b=1">vivy</a>`},
		{"link label escapes", "[a<b](https://e.com)", `<a href="https://e.com">a&lt;b</a>`},
		{"raw url", "see https://example.com/x now", `see <a href="https://example.com/x">https://example.com/x</a> now`},
		{"heading strips the marker", "## Title", "Title"},
		{"heading not at line start stays", "no # heading here", "no # heading here"},
		{"blockquote marker stripped", "> quoted", "quoted"},
		{"list markers normalized", "- one\n* two", "• one\n• two"},
		{"plain text escapes html", "a < b & c > d", "a &lt; b &amp; c &gt; d"},
		{
			"code body is not reformatted",
			"`**not bold** _not soft_`",
			"<code>**not bold** _not soft_</code>",
		},
		{
			"nested formatting inside a sentence",
			"run **make** then _check_ `make test`",
			"run <b>make</b> then <i>check</i> <code>make test</code>",
		},
		{
			"unterminated fence travels as escaped text",
			"```go\nnever closed",
			"```go\nnever closed",
		},
		{
			"code block is lifted before list rewriting",
			"```\n- not a list\n```\n- real list",
			"<pre><code>- not a list\n</code></pre>\n• real list",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := markdownToHTML(tc.in); got != tc.want {
				t.Fatalf("markdownToHTML(%q) =\n%s\nwant\n%s", tc.in, got, tc.want)
			}
		})
	}
}
