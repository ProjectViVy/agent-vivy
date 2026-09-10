package sdk

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"

	assemblyv1 "agent-vivy/sdk/internal/assembly"
	"agent-vivy/sdk/module"
)

func verifySource(dir string, descriptor module.Descriptor) error {
	digest, err := assemblyv1.HashSourceTree(dir, descriptor.Source.SHA256)
	if err != nil {
		return err
	}
	if digest != descriptor.Source.SHA256 {
		return fmt.Errorf("source hash mismatch for %s: got %s, want %s", descriptor.Module.ID, digest, descriptor.Source.SHA256)
	}
	fset := token.NewFileSet()
	constructors := map[string]bool{}
	err = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return parseErr
		}
		if file.Name.Name == "main" {
			return fmt.Errorf("source package main is not linkable as a Module")
		}
		imports := make(map[string]string)
		for _, spec := range file.Imports {
			importPath, unquoteErr := strconv.Unquote(spec.Path.Value)
			if unquoteErr != nil {
				return unquoteErr
			}
			for _, forbidden := range []string{"agent-vivy/internal/", "github.com/cloudwego/eino", "github.com/pion/", ".workspace"} {
				if strings.Contains(importPath, forbidden) {
					return fmt.Errorf("forbidden source import %s", importPath)
				}
			}
			name := filepath.Base(importPath)
			if spec.Name != nil {
				name = spec.Name.Name
			}
			if name == "." {
				return fmt.Errorf("dot import %s bypasses the capability firewall", importPath)
			}
			imports[name] = importPath
		}
		for _, decl := range file.Decls {
			if function, ok := decl.(*ast.FuncDecl); ok && function.Recv == nil {
				constructors[function.Name.Name] = true
			}
		}
		var violation error
		ast.Inspect(file, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			identifier, ok := selector.X.(*ast.Ident)
			if !ok {
				return true
			}
			importPath := imports[identifier.Name]
			if forbiddenCall(importPath, selector.Sel.Name) {
				violation = fmt.Errorf("forbidden direct capability call %s.%s; use the focused Host", importPath, selector.Sel.Name)
				return false
			}
			return true
		})
		return violation
	})
	if err != nil {
		return err
	}
	if !constructors["New"] || !constructors["NewProvider"] {
		return fmt.Errorf("source must export typed New and NewProvider constructors")
	}
	return nil
}

func forbiddenCall(importPath, name string) bool {
	switch importPath {
	case "os":
		return map[string]bool{"Open": true, "OpenFile": true, "Create": true, "ReadFile": true, "WriteFile": true, "Mkdir": true, "MkdirAll": true, "Remove": true, "RemoveAll": true, "Rename": true, "Getenv": true, "LookupEnv": true, "Environ": true}[name]
	case "os/exec":
		return name == "Command" || name == "CommandContext"
	case "net":
		return strings.HasPrefix(name, "Listen") || strings.HasPrefix(name, "Dial")
	case "net/http":
		return map[string]bool{"DefaultClient": true, "Get": true, "Head": true, "Post": true, "PostForm": true, "ListenAndServe": true, "ListenAndServeTLS": true}[name]
	default:
		return false
	}
}
