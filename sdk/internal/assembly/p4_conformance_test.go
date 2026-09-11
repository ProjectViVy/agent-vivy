package assembly

import (
	"context"
	"strings"
	"testing"

	"agent-vivy/sdk/module"
)

func TestP4SourcePortsCompileWithSoleHostAndRejectInvalidGraphs(t *testing.T) {
	for _, tc := range []struct {
		portName, hostID, sourceID string
	}{
		{portName: "std/context-source@v1", hostID: "vivy/context-host", sourceID: "vivy/context-source"},
		{portName: "std/skill-source@v1", hostID: "vivy/skill-host", sourceID: "vivy/skill-source"},
	} {
		t.Run(tc.portName, func(t *testing.T) {
			hostPort := "core/" + strings.TrimSuffix(strings.TrimPrefix(tc.hostID, "vivy/"), "-host") + "-host@v1"
			host := testDescriptor(tc.hostID)
			host.Source.Ref = "internal:" + tc.hostID
			host.Provides = []module.PortRef{{Port: hostPort, ID: tc.hostID}}
			source := testDescriptor(tc.sourceID)
			source.Source.Ref = "internal:" + tc.sourceID
			source.Provides = []module.PortRef{{Port: tc.portName, ID: strings.ReplaceAll(tc.sourceID, "/", ".")}}
			source.Requires = []module.Requirement{{PortRef: module.PortRef{Port: hostPort}, Provider: tc.hostID}}

			compiler := fixtureCompiler(t, []module.Descriptor{host, source})
			recipe := Recipe{APIVersion: RecipeAPIVersionV1, Modules: []string{host.Module.ID, source.Module.ID}}
			plan, err := compiler.Compile(context.Background(), recipe)
			if err != nil {
				t.Fatalf("valid source/Host graph rejected: %v", err)
			}
			found := false
			for _, edge := range plan.PortEdges {
				if edge.Port.Port == tc.portName && edge.Provider == tc.sourceID && edge.Consumer == tc.hostID {
					found = true
				}
			}
			if !found {
				t.Fatalf("missing %s -> %s Port edge: %#v", tc.sourceID, tc.hostID, plan.PortEdges)
			}

			missingHost := source
			missingHost.Requires[0].Provider = "vivy/missing-host"
			assertP4CompileError(t, []module.Descriptor{missingHost}, Recipe{APIVersion: RecipeAPIVersionV1, Modules: []string{missingHost.Module.ID}}, "missing provider")

			duplicate := source
			duplicate.Module.ID = "vivy/other-source"
			duplicate.Source.Ref = "internal:vivy/other-source"
			assertP4CompileError(t, []module.Descriptor{host, source, duplicate}, Recipe{APIVersion: RecipeAPIVersionV1, Modules: []string{host.Module.ID, source.Module.ID, duplicate.Module.ID}}, "duplicate provider id")

			cycleHost := host
			cycleSource := source
			cycleHost.Lifecycle.After = []string{cycleSource.Module.ID}
			cycleSource.Lifecycle.After = []string{cycleHost.Module.ID}
			assertP4CompileError(t, []module.Descriptor{cycleHost, cycleSource}, Recipe{APIVersion: RecipeAPIVersionV1, Modules: []string{cycleHost.Module.ID, cycleSource.Module.ID}}, "dependency cycle")
		})
	}
}

func assertP4CompileError(t *testing.T, descriptors []module.Descriptor, recipe Recipe, want string) {
	t.Helper()
	_, err := fixtureCompiler(t, descriptors).Compile(context.Background(), recipe)
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("Compile() error = %v, want %q", err, want)
	}
}
