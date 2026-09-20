package rpc

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"agent-vivy/internal/rpccontract"
)

type testContribution []rpccontract.MethodBinding

func (c testContribution) RPCBindings() []rpccontract.MethodBinding {
	return append([]rpccontract.MethodBinding(nil), c...)
}

func TestControlHandlerDispatchesValidatedContribution(t *testing.T) {
	seenPeer := false
	env := newControlTestEnv(t, func(deps *ControlDeps) {
		deps.Contributions = []rpccontract.Contribution{testContribution{{
			Method: "test/ping", Capability: "test.ping",
			Handler: func(_ context.Context, peer rpccontract.Peer, request rpccontract.Request) (any, *rpccontract.Error) {
				seenPeer = peer != nil
				return map[string]string{"method": request.Method}, nil
			},
		}}}
	})
	peer := NewPeer(nil, nil, Options{})
	got, rpcErr := env.handler.Handle(context.Background(), peer, Request{Method: "test/ping"})
	if rpcErr != nil || got.(map[string]string)["method"] != "test/ping" || !seenPeer {
		t.Fatalf("got=%#v err=%v peer=%v", got, rpcErr, seenPeer)
	}
	initialized, rpcErr := callControl(t, env.handler, "initialize", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	capabilities := initialized.(map[string]any)["capabilities"].([]string)
	if !hasContributedCapability(capabilities, "test.ping") {
		t.Fatalf("capabilities=%v", capabilities)
	}
}

func TestControlHandlerContributionValidation(t *testing.T) {
	handler := func(context.Context, rpccontract.Peer, rpccontract.Request) (any, *rpccontract.Error) {
		return nil, nil
	}
	tests := []struct {
		name          string
		contributions []rpccontract.Contribution
		want          string
	}{
		{name: "empty"},
		{name: "nil contribution", contributions: []rpccontract.Contribution{nil}},
		{name: "duplicate method", contributions: []rpccontract.Contribution{
			testContribution{{Method: "test/ping", Capability: "test.ping", Handler: handler}},
			testContribution{{Method: "test/ping", Capability: "test.pong", Handler: handler}},
		}, want: `duplicate method "test/ping"`},
		{name: "core collision", contributions: []rpccontract.Contribution{
			testContribution{{Method: "initialize", Capability: "test.init", Handler: handler}},
		}, want: `binding collides with core method "initialize"`},
		{name: "alias collision", contributions: []rpccontract.Contribution{
			testContribution{{Method: "attachment/resolve", Capability: "test.alias", Handler: handler}},
		}, want: `binding collides with core method "attachment/resolve"`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := newContributionDispatch(test.contributions)
			if test.want == "" && err != nil {
				t.Fatal(err)
			}
			if test.want != "" && (err == nil || !strings.Contains(err.Error(), test.want)) {
				t.Fatalf("error=%v, want containing %q", err, test.want)
			}
		})
	}
}

func TestControlHandlerContributionCapabilitiesAreSortedUniqueAndDetached(t *testing.T) {
	handler := func(context.Context, rpccontract.Peer, rpccontract.Request) (any, *rpccontract.Error) {
		return nil, nil
	}
	bindings := testContribution{
		{Method: "test/z", Capability: "test.shared", Handler: handler},
		{Method: "test/a", Capability: "test.alpha", Handler: handler},
		{Method: "test/y", Capability: "test.shared", Handler: handler},
	}
	dispatch, err := newContributionDispatch([]rpccontract.Contribution{bindings})
	if err != nil {
		t.Fatal(err)
	}
	bindings[0].Method = "mutated"
	if _, ok := dispatch.handlers["test/z"]; !ok {
		t.Fatal("dispatch retained caller alias")
	}
	if want := []string{"test.alpha", "test.shared"}; !reflect.DeepEqual(dispatch.capabilities, want) {
		t.Fatalf("capabilities=%v, want %v", dispatch.capabilities, want)
	}
}

func TestControlHandlerCapabilitiesAreGloballyUnique(t *testing.T) {
	env := newControlTestEnv(t, func(deps *ControlDeps) {
		deps.Contributions = []rpccontract.Contribution{testContribution{{
			Method: "test/session-capability", Capability: "session",
			Handler: func(context.Context, rpccontract.Peer, rpccontract.Request) (any, *rpccontract.Error) {
				return nil, nil
			},
		}}}
	})
	initialized, rpcErr := callControl(t, env.handler, "initialize", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	capabilities := initialized.(map[string]any)["capabilities"].([]string)
	count := 0
	for _, capability := range capabilities {
		if capability == "session" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("session capability count = %d in %v, want 1", count, capabilities)
	}
}

func TestControlHandlerContributionDiagnosticsAreDeterministic(t *testing.T) {
	handler := func(context.Context, rpccontract.Peer, rpccontract.Request) (any, *rpccontract.Error) {
		return nil, nil
	}
	left := testContribution{
		{Method: "initialize", Capability: "test.init", Handler: handler},
		{Method: "test/dup", Capability: "test.one", Handler: handler},
		{Method: "test/dup", Capability: "test.two", Handler: handler},
	}
	right := testContribution{left[2], left[1], left[0]}
	_, leftErr := newContributionDispatch([]rpccontract.Contribution{left})
	_, rightErr := newContributionDispatch([]rpccontract.Contribution{right})
	if leftErr == nil || rightErr == nil || leftErr.Error() != rightErr.Error() {
		t.Fatalf("diagnostics differ:\nleft:  %v\nright: %v", leftErr, rightErr)
	}
}

func hasContributedCapability(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestCoreMethodVocabularyMatchesHandleSwitch(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "control.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	actual := make(map[string]struct{})
	found := false
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != "Handle" || function.Recv == nil {
			continue
		}
		ast.Inspect(function.Body, func(node ast.Node) bool {
			statement, ok := node.(*ast.SwitchStmt)
			if !ok || found {
				return true
			}
			selector, ok := statement.Tag.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "Method" {
				return true
			}
			found = true
			for _, item := range statement.Body.List {
				clause := item.(*ast.CaseClause)
				for _, expression := range clause.List {
					switch value := expression.(type) {
					case *ast.BasicLit:
						method, err := strconv.Unquote(value.Value)
						if err != nil {
							t.Fatal(err)
						}
						actual[method] = struct{}{}
					case *ast.Ident:
						if value.Name != "ModuleActionMethod" {
							t.Fatalf("unresolved core method identifier %q", value.Name)
						}
						actual[ModuleActionMethod] = struct{}{}
					default:
						t.Fatalf("unsupported core method case expression %T", expression)
					}
				}
			}
			return false
		})
	}
	if !found {
		t.Fatal("request.Method switch not found")
	}
	missing, extra := vocabularyDifference(actual, coreMethodNames), vocabularyDifference(coreMethodNames, actual)
	if len(missing) > 0 || len(extra) > 0 {
		t.Fatalf("core vocabulary mismatch: missing=%v extra=%v", missing, extra)
	}
}

func vocabularyDifference(left, right map[string]struct{}) []string {
	out := make([]string, 0)
	for method := range left {
		if _, ok := right[method]; !ok {
			out = append(out, method)
		}
	}
	sort.Strings(out)
	return out
}
