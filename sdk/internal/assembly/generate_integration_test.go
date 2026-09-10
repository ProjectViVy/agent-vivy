package assembly

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestGeneratedBinderCompilesAndRollsBackLifecycle(t *testing.T) {
	plan := AssemblyPlan{
		Modules: []ResolvedModule{
			{Descriptor: testDescriptor("fixture/clock"), Trust: TrustT1, Binding: GoBinding{ImportPath: "generatedtest/clock", Package: "clock", Constructor: "New"}},
			{Descriptor: testDescriptor("fixture/search"), Trust: TrustT2, Binding: GoBinding{ImportPath: "generatedtest/search", Package: "search", Constructor: "New"}},
		},
		LifecycleOrder: []string{"fixture/clock", "fixture/search"},
	}
	binder, err := GenerateBinder(plan, "binder")
	if err != nil {
		t.Fatal(err)
	}
	runtimeAssembly, err := GenerateRuntimeAssembly(plan, "runtimeassembly")
	if err != nil {
		t.Fatal(err)
	}

	repositoryRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	goMod := fmt.Sprintf("module generatedtest\n\ngo 1.26.4\n\nrequire agent-vivy v0.0.0\nreplace agent-vivy => %s\n", filepath.ToSlash(repositoryRoot))
	writeGeneratedTestFile(t, root, "go.mod", goMod)
	writeGeneratedTestFile(t, root, "events/events.go", `package events

var Items []string
var FailStart string

func Add(item string) { Items = append(Items, item) }
func Reset() { Items = nil; FailStart = "" }
`)
	writeGeneratedTestFile(t, root, "clock/module.go", generatedFixtureModule("clock", "clock"))
	writeGeneratedTestFile(t, root, "search/module.go", generatedFixtureModule("search", "search"))
	writeGeneratedTestFile(t, root, "binder/zz_assembly.go", string(binder))
	writeGeneratedTestFile(t, root, "binder/zz_assembly_test.go", generatedBinderBehaviorTest)
	writeGeneratedTestFile(t, root, "runtimeassembly/zz_default.go", string(runtimeAssembly))
	writeGeneratedTestFile(t, root, "runtimeassembly/zz_default_test.go", generatedRuntimeAssemblyBehaviorTest)

	command := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "test", "./...")
	command.Dir = root
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("generated binder did not pass real module test: %v\n%s", err, output)
	}
}

func generatedFixtureModule(packageName, instanceName string) string {
	return fmt.Sprintf(`package %s

import (
    "context"
    "fmt"

    "agent-vivy/sdk/module"
    "generatedtest/events"
)

type definition struct{}
func New() module.Module { return definition{} }
func (definition) Descriptor() module.Descriptor { return module.Descriptor{} }
func (definition) Construct(context.Context, module.Host) (module.Instance, error) {
    events.Add("construct:%s")
    return instance{}, nil
}

type instance struct{}
func (instance) Start(context.Context) error {
    events.Add("start:%s")
    if events.FailStart == %q { return fmt.Errorf("start failed") }
    return nil
}
func (instance) Ready(context.Context) error { events.Add("ready:%s"); return nil }
func (instance) Stop(context.Context) error { events.Add("stop:%s"); return nil }
func (instance) Close(context.Context) error { events.Add("close:%s"); return nil }
`, packageName, instanceName, instanceName, instanceName, instanceName, instanceName, instanceName)
}

func writeGeneratedTestFile(t *testing.T, root, relative, source string) {
	t.Helper()
	filename := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(filename), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
}

const generatedBinderBehaviorTest = `package binder

import (
    "context"
    "reflect"
    "testing"

    "agent-vivy/sdk/module"
    "generatedtest/events"
)

type host string
func (value host) ModuleID() string { return string(value) }
type hosts struct{}
func (hosts) ForModule(id string) module.Host { return host(id) }

func TestLifecycle(t *testing.T) {
    events.Reset()
    owners, err := Construct(context.Background(), hosts{})
    if err != nil { t.Fatal(err) }
    if err := owners.Start(context.Background()); err != nil { t.Fatal(err) }
    if err := owners.Close(context.Background()); err != nil { t.Fatal(err) }
    want := []string{"construct:clock", "construct:search", "start:clock", "ready:clock", "start:search", "ready:search", "stop:search", "close:search", "stop:clock", "close:clock"}
    if !reflect.DeepEqual(events.Items, want) { t.Fatalf("events = %v, want %v", events.Items, want) }
}

func TestStartFailureRollsBackEveryConstructedOwner(t *testing.T) {
    events.Reset()
    owners, err := Construct(context.Background(), hosts{})
    if err != nil { t.Fatal(err) }
    events.FailStart = "search"
    if err := owners.Start(context.Background()); err == nil { t.Fatal("Start() succeeded") }
    want := []string{"construct:clock", "construct:search", "start:clock", "ready:clock", "start:search", "stop:search", "stop:clock", "close:search", "close:clock"}
    if !reflect.DeepEqual(events.Items, want) { t.Fatalf("events = %v, want %v", events.Items, want) }
}
`

const generatedRuntimeAssemblyBehaviorTest = `package runtimeassembly

import (
    "context"
    "reflect"
    "testing"

    "agent-vivy/sdk/module"
    "generatedtest/events"
)

type host string
func (value host) ModuleID() string { return string(value) }
type hosts struct{}
func (hosts) ForModule(id string) module.Host { return host(id) }

func TestExecutableAssemblyLifecycle(t *testing.T) {
    events.Reset()
    assembly := BuildDefault()
    if err := assembly.Start(context.Background(), hosts{}); err != nil { t.Fatal(err) }
    if err := assembly.Close(context.Background()); err != nil { t.Fatal(err) }
	if err := assembly.Close(context.Background()); err != nil { t.Fatal(err) }
    want := []string{"construct:clock", "construct:search", "start:clock", "ready:clock", "start:search", "ready:search", "stop:search", "close:search", "stop:clock", "close:clock"}
    if !reflect.DeepEqual(events.Items, want) { t.Fatalf("events = %v, want %v", events.Items, want) }
}
`
