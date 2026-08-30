package sdk

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
)

type sourceFile struct {
	rel  string
	file *ast.File
}

func parsePluginSources(dir string) ([]sourceFile, []string) {
	fset := token.NewFileSet()
	var files []sourceFile
	var issues []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != dir && (d.Name() == "testdata" || strings.HasPrefix(d.Name(), ".")) {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		src, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			issues = append(issues, fmt.Sprintf("parse %s: %v", rel(dir, path), err))
			return nil
		}
		files = append(files, sourceFile{rel: rel(dir, path), file: src})
		return nil
	})
	if err != nil {
		issues = append(issues, err.Error())
	}
	if len(files) == 0 && len(issues) == 0 {
		issues = append(issues, "no Go sources found")
	}
	return files, issues
}

// bannedImportPrefixes are import families no plugin of any seam may
// link. The reason text is printed verbatim in the verifier issue. At
// most one prefix can match a given import path, so issue order stays
// deterministic per file.
var bannedImportPrefixes = map[string]string{
	"agent-vivy/internal/":      "agent-vivy/internal is kernel-private (sdk/plugin is the only import window)",
	"github.com/cloudwego/eino": "eino is the kernel engine, not a plugin API",
	// CH-C7c: voice/WebRTC stays out of the species' channels. The ban is
	// prefix-wide so every pion module (webrtc, media, transport, rtp, ...)
	// is covered, not only webrtc itself.
	"github.com/pion/": "pion/webrtc is banned in plugins (no voice in Vivy channels)",
	// Review L1-F2 (contract §9.3): picoclaw is the reference clone this
	// epic learns from, never a dependency of a shipped plugin.
	"github.com/sipeed/picoclaw": "reference material must be rewritten, not imported (picoclaw/.workspace)",
}

// bannedImportSubstrings are path fragments no plugin import may contain
// anywhere (prefix matching cannot express that). `.workspace` is the
// in-tree reference/scratch checkout (AGENTS.md); importing from it means
// reference material leaked into a plugin. May double-report an import the
// prefix map already caught — both messages point at the same rewrite.
var bannedImportSubstrings = map[string]string{
	".workspace": "reference material must be rewritten, not imported (picoclaw/.workspace)",
}

func checkSources(files []sourceFile) []string {
	var issues []string
	hasCtor := false
	for _, src := range files {
		if src.file.Name.Name == "main" {
			issues = append(issues, src.rel+": package main is forbidden (would produce an exe)")
		}
		imports := importMap(src.file)
		for path := range imports {
			for prefix, reason := range bannedImportPrefixes {
				if strings.HasPrefix(path, prefix) {
					issues = append(issues, src.rel+": import of "+path+" is forbidden: "+reason)
				}
			}
			for fragment, reason := range bannedImportSubstrings {
				if strings.Contains(path, fragment) {
					issues = append(issues, src.rel+": import of "+path+" is forbidden: "+reason)
				}
			}
		}
		issues = append(issues, bannedCalls(src, imports)...)
		issues = append(issues, bannedEmbeds(src)...)
		if hasNewPlugin(src.file) {
			hasCtor = true
		}
	}
	if !hasCtor {
		issues = append(issues, "missing func New() plugin.Plugin")
	}
	return issues
}

func importMap(file *ast.File) map[string]string {
	out := map[string]string{}
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		name := path
		if i := strings.LastIndex(path, "/"); i >= 0 {
			name = path[i+1:]
		}
		if imp.Name != nil {
			if imp.Name.Name == "." || imp.Name.Name == "_" {
				continue
			}
			name = imp.Name.Name
		}
		out[path] = name
	}
	return out
}

func bannedCalls(src sourceFile, imports map[string]string) []string {
	osName := imports["os"]
	execName := imports["os/exec"]
	netName := imports["net"]
	httpName := imports["net/http"]
	tlsName := imports["crypto/tls"]
	var issues []string
	ast.Inspect(src.file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		ident, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		if osName != "" && ident.Name == osName {
			switch sel.Sel.Name {
			case "Open", "OpenFile", "Create", "StartProcess":
				issues = append(issues, src.rel+": "+osName+"."+sel.Sel.Name+" bypasses plugin.Env")
			}
		}
		if execName != "" && ident.Name == execName {
			switch sel.Sel.Name {
			case "Command", "CommandContext":
				issues = append(issues, src.rel+": "+execName+"."+sel.Sel.Name+" bypasses plugin.Env")
			}
		}
		// Listen is a kernel ChannelHost capability (VIVY-CHANNEL-PACK.md
		// §9.3); a plugin may only run outbound connections. Applied to all
		// seams — tool plugins already have no use for it, and for channel
		// plugins it is the rule that keeps this batch honest.
		if netName != "" && ident.Name == netName && sel.Sel.Name == "Listen" {
			issues = append(issues, src.rel+": "+netName+"."+sel.Sel.Name+" opens a listen socket (Listen belongs to the kernel ChannelHost)")
		}
		if httpName != "" && ident.Name == httpName {
			switch sel.Sel.Name {
			case "ListenAndServe", "ListenAndServeTLS":
				issues = append(issues, src.rel+": "+httpName+"."+sel.Sel.Name+" opens a listen socket (Listen belongs to the kernel ChannelHost)")
			}
		}
		if tlsName != "" && ident.Name == tlsName && sel.Sel.Name == "Listen" {
			issues = append(issues, src.rel+": "+tlsName+".Listen opens a listen socket (Listen belongs to the kernel ChannelHost)")
		}
		// Method-form widening (review L4): the package-qualified rules
		// above only see an identifier receiver, so `(&http.Server{}).
		// ListenAndServe()` or `net.ListenPacket` slipped through. Match the
		// selector NAME on ANY receiver — the same sockets open through a
		// value as through the package. Deliberately conservative: a plugin
		// type that happens to own a method of one of these names is
		// flagged too (accepted false positive — verify carries no
		// cross-package type information), and a package-qualified call can
		// now report twice (package rule + name rule).
		switch sel.Sel.Name {
		case "ListenAndServe", "ListenAndServeTLS", "ListenPacket":
			issues = append(issues, src.rel+": "+types.ExprString(sel)+" opens a listen socket (Listen belongs to the kernel ChannelHost)")
		}
		return true
	})
	return issues
}

func bannedEmbeds(src sourceFile) []string {
	var issues []string
	for _, group := range src.file.Comments {
		for _, c := range group.List {
			text := strings.TrimSpace(strings.TrimPrefix(c.Text, "//"))
			if !strings.HasPrefix(text, "go:embed") {
				continue
			}
			args := strings.TrimSpace(strings.TrimPrefix(text, "go:embed"))
			for _, field := range strings.Fields(args) {
				lower := strings.ToLower(field)
				for _, ext := range []string{".exe", ".dll", ".so", ".wasm", ".dylib"} {
					if strings.HasSuffix(lower, ext) {
						issues = append(issues, src.rel+": go:embed of "+field+" smuggles a binary")
					}
				}
			}
		}
	}
	return issues
}

func hasNewPlugin(file *ast.File) bool {
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil || fn.Name.Name != "New" || fn.Type.Results == nil || len(fn.Type.Results.List) != 1 {
			continue
		}
		sel, ok := fn.Type.Results.List[0].Type.(*ast.SelectorExpr)
		if !ok {
			continue
		}
		pkg, ok := sel.X.(*ast.Ident)
		if ok && pkg.Name == "plugin" && sel.Sel.Name == "Plugin" {
			return true
		}
	}
	return false
}

func rel(root, path string) string {
	out, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(out)
}
