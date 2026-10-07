package acp

import (
	"errors"
	"strings"
	"testing"

	acp "github.com/eino-contrib/acp"
)

func textBlock(s string) acp.ContentBlock {
	return acp.ContentBlock{Text: &acp.ContentBlockText{TextContent: acp.TextContent{Text: s}}}
}

func linkBlock(name, uri string) acp.ContentBlock {
	return acp.ContentBlock{ResourceLink: &acp.ContentBlockResourceLink{
		ResourceLink: acp.ResourceLink{Name: name, URI: uri},
	}}
}

func promptReq(root string, blocks ...acp.ContentBlock) acp.PromptRequest {
	return acp.PromptRequest{Prompt: blocks, SessionID: "s"}
}

func rpcErrCode(t *testing.T, err error) int {
	t.Helper()
	var re *acp.RPCError
	if !errors.As(err, &re) {
		t.Fatalf("not an RPCError: %v", err)
	}
	return re.Code
}

func TestPromptNormalization(t *testing.T) {
	root := "/ws"

	t.Run("text blocks concatenate in order with separators", func(t *testing.T) {
		text, paths, err := normalizePrompt(root, promptReq(root, textBlock("alpha"), textBlock("beta"), textBlock("gamma")))
		if err != nil {
			t.Fatalf("normalize: %v", err)
		}
		if text != "alpha\nbeta\ngamma" {
			t.Fatalf("text = %q", text)
		}
		if len(paths) != 0 {
			t.Fatalf("paths = %v", paths)
		}
	})

	t.Run("link-only prompt becomes a deterministic reference", func(t *testing.T) {
		text, paths, err := normalizePrompt(root, promptReq(root, linkBlock("spec.md", "https://example.com/spec.md")))
		if err != nil {
			t.Fatalf("normalize: %v", err)
		}
		if text != "[resource: spec.md] https://example.com/spec.md" {
			t.Fatalf("text = %q", text)
		}
		if len(paths) != 0 {
			t.Fatalf("non-file URI produced context paths: %v", paths)
		}
	})

	t.Run("deduplicated context paths in order", func(t *testing.T) {
		text, paths, err := normalizePrompt(root, promptReq(root,
			linkBlock("a", "file:///ws/a.txt"),
			textBlock("middle"),
			linkBlock("a2", "file:///ws/a.txt"),
			linkBlock("b", "file:///ws/dir/b.txt"),
		))
		if err != nil {
			t.Fatalf("normalize: %v", err)
		}
		if len(paths) != 2 || paths[0] != "a.txt" || paths[1] != "dir/b.txt" {
			t.Fatalf("paths = %v", paths)
		}
		if !strings.Contains(text, "middle") {
			t.Fatalf("text missing link/text ordering: %q", text)
		}
	})

	t.Run("256 KiB text bound", func(t *testing.T) {
		big := strings.Repeat("a", 256<<10)
		if _, _, err := normalizePrompt(root, promptReq(root, textBlock(big))); err != nil {
			t.Fatalf("boundary text rejected: %v", err)
		}
		if _, _, err := normalizePrompt(root, promptReq(root, textBlock(big), textBlock("x"))); err == nil {
			t.Fatal("overflow accepted")
		} else if rpcErrCode(t, err) != -32602 {
			t.Fatalf("code = %d", rpcErrCode(t, err))
		}
	})

	t.Run("unicode text is not corrupted", func(t *testing.T) {
		s := "汉字 \U0001F600 tail"
		text, _, err := normalizePrompt(root, promptReq(root, textBlock(s)))
		if err != nil || text != s {
			t.Fatalf("text = %q err=%v", text, err)
		}
	})

	t.Run("empty and whitespace-only prompts are -32602", func(t *testing.T) {
		for i, req := range []acp.PromptRequest{
			promptReq(root),
			promptReq(root, textBlock("")),
			promptReq(root, textBlock("  \n\t ")),
		} {
			if _, _, err := normalizePrompt(root, req); err == nil {
				t.Fatalf("case %d accepted", i)
			} else if rpcErrCode(t, err) != -32602 {
				t.Fatalf("case %d code = %d", i, rpcErrCode(t, err))
			}
		}
	})

	t.Run("unsupported blocks reject the whole prompt", func(t *testing.T) {
		img := acp.ContentBlock{Image: &acp.ContentBlockImage{}}
		res := acp.ContentBlock{Resource: &acp.ContentBlockResource{}}
		empty := acp.ContentBlock{}
		for i, b := range []acp.ContentBlock{img, res, empty} {
			if _, _, err := normalizePrompt(root, promptReq(root, textBlock("ok"), b)); err == nil {
				t.Fatalf("block %d accepted", i)
			} else if rpcErrCode(t, err) != -32602 {
				t.Fatalf("block %d code = %d", i, rpcErrCode(t, err))
			}
		}
	})

	t.Run("no adapter fetch for any resource", func(t *testing.T) {
		// normalizePrompt only manipulates strings: a file: link that is
		// safe becomes a path without the file existing on disk.
		_, paths, err := normalizePrompt(root, promptReq(root, linkBlock("", "file:///ws/definitely/missing.bin")))
		if err != nil {
			t.Fatalf("normalize: %v", err)
		}
		if len(paths) != 1 || paths[0] != "definitely/missing.bin" {
			t.Fatalf("paths = %v", paths)
		}
	})
}

func TestResourceURIPlatformForms(t *testing.T) {
	root := "/ws"

	unsafe := []struct{ name, uri string }{
		{"query", "file:///ws/a.txt?x=1"},
		{"fragment", "file:///ws/a.txt#frag"},
		{"force query", "file:///ws/a.txt?"},
		{"remote authority", "file://server/share/a.txt"},
		{"opaque", "file:ws/a.txt"},
		{"encoded slash", "file:///ws/a%2Fb.txt"},
		{"encoded backslash", "file:///ws/a%5Cb.txt"},
		{"encoded dot", "file:///ws/%2e%2e/x"},
		{"encoded nul", "file:///ws/a%00b.txt"},
		{"traversal", "file:///ws/../outside.txt"},
		{"control char", "file:///ws/a\x00b.txt"},
		{"credential", "file://user@/ws/a.txt"},
	}
	for _, tc := range unsafe {
		t.Run("reject "+tc.name, func(t *testing.T) {
			_, _, err := normalizePrompt(root, promptReq(root, linkBlock("n", tc.uri)))
			if err == nil {
				t.Fatalf("unsafe URI %q accepted", tc.uri)
			}
			if rpcErrCode(t, err) != -32602 {
				t.Fatalf("code = %d", rpcErrCode(t, err))
			}
		})
	}

	safe := []struct{ name, uri, wantPath string }{
		{"plain", "file:///ws/a.txt", "a.txt"},
		{"nested", "file:///ws/dir/sub/f.go", "dir/sub/f.go"},
		{"localhost authority", "file://localhost/ws/a.txt", "a.txt"},
		{"encoded space", "file:///ws/a%20b.txt", "a b.txt"},
	}
	for _, tc := range safe {
		t.Run("accept "+tc.name, func(t *testing.T) {
			_, paths, err := normalizePrompt(root, promptReq(root, linkBlock("", tc.uri)))
			if err != nil {
				t.Fatalf("safe URI %q rejected: %v", tc.uri, err)
			}
			if len(paths) != 1 || paths[0] != tc.wantPath {
				t.Fatalf("paths = %v, want [%s]", paths, tc.wantPath)
			}
		})
	}

	nonFile := []struct{ name, uri string }{
		{"https", "https://example.com/doc"},
		{"mailto", "mailto:a@b.c"},
	}
	for _, tc := range nonFile {
		t.Run("reference "+tc.name, func(t *testing.T) {
			text, paths, err := normalizePrompt(root, promptReq(root, linkBlock("doc", tc.uri)))
			if err != nil {
				t.Fatalf("non-file URI rejected: %v", err)
			}
			if len(paths) != 0 {
				t.Fatalf("non-file produced paths: %v", paths)
			}
			if !strings.Contains(text, tc.uri) {
				t.Fatalf("label missing uri: %q", text)
			}
		})
	}

	t.Run("malformed uri rejected", func(t *testing.T) {
		if _, _, err := normalizePrompt(root, promptReq(root, linkBlock("x", "://bad"))); err == nil {
			t.Fatal("malformed URI accepted")
		}
	})
}
