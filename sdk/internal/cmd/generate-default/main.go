package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"agent-vivy/internal/modules/defaults"
	sessiontree "agent-vivy/plugins/coding/session-tree"
	"agent-vivy/plugins/dingtalk"
	"agent-vivy/plugins/discord"
	"agent-vivy/plugins/feishu"
	localllm "agent-vivy/plugins/infra/llm"
	"agent-vivy/plugins/qq"
	"agent-vivy/plugins/telegram"
	"agent-vivy/plugins/vivy-evolution"
	"agent-vivy/plugins/vivy-masks-ui"
	"agent-vivy/plugins/vivy-memory"
	"agent-vivy/plugins/vivy-notebook"
	"agent-vivy/plugins/vivy-persona"
	"agent-vivy/plugins/vivy-workflow"
	assemblyv1 "agent-vivy/sdk/internal/assembly"
	"agent-vivy/sdk/module"
	"agent-vivy/sdk/port"
	"gopkg.in/yaml.v3"
)

type extern struct {
	descriptor           module.Descriptor
	dir, importPath, pkg string
}

func main() {
	repo := flag.String("repo", ".", "repository root")
	output := flag.String("output", "zz_default.go", "generated output")
	recipePath := flag.String("recipe", "recipes/default.vivy.yml", "recipe relative to repository root")
	flag.Parse()
	root, err := filepath.Abs(*repo)
	must(err)
	raw, err := os.ReadFile(filepath.Join(root, *recipePath))
	must(err)
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	var recipe assemblyv1.Recipe
	must(decoder.Decode(&recipe))
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		must(fmt.Errorf("recipe must contain one document"))
	}
	internal, err := defaults.Catalog(root)
	must(err)
	records := make([]assemblyv1.SourceRecord, 0, len(internal)+5)
	for _, r := range internal {
		records = append(records, assemblyv1.SourceRecord{Descriptor: r.Descriptor, Trust: assemblyv1.TrustT1, Root: filepath.Join(root, "internal"), Ref: "file:internal", Binding: assemblyv1.GoBinding{ImportPath: r.Binding.ImportPath, Package: r.Binding.Package, Constructor: r.Binding.Constructor, ProviderConstructor: r.Binding.ProviderConstructor, ProviderCollection: r.Binding.ProviderCollection, MaskFactory: r.Binding.MaskFactory, CognitiveFactory: r.Binding.CognitiveFactory, NotebookFactory: r.Binding.NotebookFactory, ReportFactory: r.Binding.ReportFactory, ContextSourceProvider: r.Binding.ContextSourceProvider, SkillSourceProvider: r.Binding.SkillSourceProvider, MCPHostProvider: r.Binding.MCPHostProvider, RunObserverProvider: r.Binding.RunObserverProvider}})
	}
	externals := []extern{{dingtalk.New().Descriptor(), "plugins/dingtalk", "agent-vivy/plugins/dingtalk", "dingtalk"}, {discord.New().Descriptor(), "plugins/discord", "agent-vivy/plugins/discord", "discord"}, {feishu.New().Descriptor(), "plugins/feishu", "agent-vivy/plugins/feishu", "feishu"}, {qq.New().Descriptor(), "plugins/qq", "agent-vivy/plugins/qq", "qq"}, {telegram.New().Descriptor(), "plugins/telegram", "agent-vivy/plugins/telegram", "telegram"}, {vivypersona.New().Descriptor(), "plugins/vivy-persona", "agent-vivy/plugins/vivy-persona", "vivypersona"}, {vivyevolution.New().Descriptor(), "plugins/vivy-evolution", "agent-vivy/plugins/vivy-evolution", "vivyevolution"}, {vivymemory.New().Descriptor(), "plugins/vivy-memory", "agent-vivy/plugins/vivy-memory", "vivymemory"}, {vivynotebook.New().Descriptor(), "plugins/vivy-notebook", "agent-vivy/plugins/vivy-notebook", "vivynotebook"}, {vivymasksui.New().Descriptor(), "plugins/vivy-masks-ui", "agent-vivy/plugins/vivy-masks-ui", "vivymasksui"}, {vivyworkflow.New().Descriptor(), "plugins/vivy-workflow", "agent-vivy/plugins/vivy-workflow", "vivyworkflow"}, {sessiontree.New().Descriptor(), "plugins/coding/session-tree", "agent-vivy/plugins/coding/session-tree", "sessiontree"}, {localllm.New().Descriptor(), "plugins/infra/llm", "agent-vivy/plugins/infra/llm", "localllm"}}
	for _, e := range externals {
		records = append(records, assemblyv1.SourceRecord{Descriptor: e.descriptor, Trust: assemblyv1.TrustT1, Root: filepath.Join(root, e.dir), Ref: "repo:" + e.dir, Binding: assemblyv1.GoBinding{ImportPath: e.importPath, Package: e.pkg, Constructor: "New", ProviderConstructor: "NewProvider"}})
	}
	catalog, err := assemblyv1.NewSourceCatalog(records)
	must(err)
	evidence := assemblyv1.SupportedPortEvidence()
	plan, err := (assemblyv1.Compiler{Ports: port.PublicCatalog(), Sources: catalog, PortEvidence: evidence, ConformanceResults: assemblyv1.SupportedPortConformance()}).Compile(context.Background(), recipe)
	must(err)
	// The committed default composition is the species' headless form: it
	// declares its own form identity in the generated artifact. Packed builds
	// replace this file through a build overlay and never pass the identity,
	// so their sealed identity keeps coming from the embedded manifest.
	generated, err := assemblyv1.GenerateRuntimeAssembly(plan, "assembly", assemblyv1.WithFormIdentity("vivy-headless/1"))
	must(err)
	must(os.WriteFile(*output, generated, 0o644))
}
func must(err error) {
	if err != nil {
		panic(err)
	}
}
