package assembly

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"agent-vivy/sdk/module"
)

func TestGeneratedSelectedMaskAssemblyCompiles(t *testing.T) {
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	if _, err := os.Stat(goBin); err != nil {
		t.Skipf("Go toolchain unavailable at %s: %v", goBin, err)
	}
	repositoryRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	testdataRoot := filepath.Join(repositoryRoot, "sdk", "internal", "assembly", "testdata")
	temporaryRoot, err := os.MkdirTemp(testdataRoot, "generated-mask-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(temporaryRoot)
	relativeRoot, err := filepath.Rel(repositoryRoot, temporaryRoot)
	if err != nil {
		t.Fatal(err)
	}
	importPrefix := "agent-vivy/" + filepath.ToSlash(relativeRoot)
	fixtureSource, err := os.ReadFile(filepath.Join(testdataRoot, "maskfixture", "module.go"))
	if err != nil {
		t.Fatal(err)
	}
	writeGeneratedTestFile(t, temporaryRoot, "maskfixture/module.go", string(fixtureSource))

	descriptor := testDescriptor("vivy/masks")
	descriptor.Source.Ref = "testdata:maskfixture"
	descriptor.Provides = []module.PortRef{{Port: "core/mask-service@v1", ID: "vivy.mask-service"}}
	plan := AssemblyPlan{
		Modules: []ResolvedModule{{
			Descriptor: descriptor,
			Binding: GoBinding{
				ImportPath:  importPrefix + "/maskfixture",
				Package:     "maskfixture",
				Constructor: "NewModule",
				MaskFactory: "Factory",
			},
		}},
		LifecycleOrder: []string{"vivy/masks"},
	}
	generated, err := GenerateRuntimeAssembly(plan, "runtimeassembly")
	if err != nil {
		t.Fatal(err)
	}
	writeGeneratedTestFile(t, temporaryRoot, "runtimeassembly/zz_generated.go", string(generated))
	writeGeneratedTestFile(t, temporaryRoot, "runtimeassembly/zz_generated_test.go", generatedSelectedMaskAssemblyTest)

	command := exec.Command(goBin, "test", "./"+filepath.ToSlash(relativeRoot)+"/maskfixture", "./"+filepath.ToSlash(relativeRoot)+"/runtimeassembly")
	command.Dir = repositoryRoot
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("generated selected mask Assembly did not compile: %v\n%s", err, output)
	}
}

const generatedSelectedMaskAssemblyTest = `package runtimeassembly

import (
    "context"
    "testing"

    "agent-vivy/sdk/module"
)

type generatedMaskHost string

func (host generatedMaskHost) ModuleID() string { return string(host) }

type generatedMaskHosts struct{}

func (generatedMaskHosts) ForModule(id string) module.Host { return generatedMaskHost(id) }

func TestGeneratedMaskFactorySelector(t *testing.T) {
    assembly := BuildDefault()
    if !assembly.HasMaskFactory() || assembly.MaskFactory == nil {
        t.Fatal("generated Assembly did not bind maskfixture.Factory")
    }
    if err := assembly.Start(context.Background(), generatedMaskHosts{}); err != nil {
        t.Fatal(err)
    }
    if err := assembly.Close(context.Background()); err != nil {
        t.Fatal(err)
    }
}
`
