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
	assemblyv1 "agent-vivy/sdk/internal/assembly"
	"agent-vivy/sdk/module"
	"agent-vivy/sdk/port"
	"example.com/vivy/plugins/dingtalk"
	"example.com/vivy/plugins/discord"
	"example.com/vivy/plugins/feishu"
	"example.com/vivy/plugins/qq"
	"example.com/vivy/plugins/telegram"
	"gopkg.in/yaml.v3"
)

type extern struct {
	descriptor           module.Descriptor
	dir, importPath, pkg string
}

func main() {
	repo := flag.String("repo", ".", "repository root")
	output := flag.String("output", "zz_default.go", "generated output")
	flag.Parse()
	root, err := filepath.Abs(*repo)
	must(err)
	raw, err := os.ReadFile(filepath.Join(root, "recipes/default.vivy.yml"))
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
		records = append(records, assemblyv1.SourceRecord{Descriptor: r.Descriptor, Trust: assemblyv1.TrustT1, Root: filepath.Join(root, "internal"), Ref: "file:internal", Binding: assemblyv1.GoBinding{ImportPath: r.Binding.ImportPath, Package: r.Binding.Package, Constructor: r.Binding.Constructor, ProviderConstructor: r.Binding.ProviderConstructor, ProviderCollection: r.Binding.ProviderCollection}})
	}
	externals := []extern{{dingtalk.New().Descriptor(), "plugins/dingtalk", "example.com/vivy/plugins/dingtalk", "dingtalk"}, {discord.New().Descriptor(), "plugins/discord", "example.com/vivy/plugins/discord", "discord"}, {feishu.New().Descriptor(), "plugins/feishu", "example.com/vivy/plugins/feishu", "feishu"}, {qq.New().Descriptor(), "plugins/qq", "example.com/vivy/plugins/qq", "qq"}, {telegram.New().Descriptor(), "plugins/telegram", "example.com/vivy/plugins/telegram", "telegram"}}
	for _, e := range externals {
		records = append(records, assemblyv1.SourceRecord{Descriptor: e.descriptor, Trust: assemblyv1.TrustT1, Root: filepath.Join(root, e.dir), Ref: "repo:" + e.dir, Binding: assemblyv1.GoBinding{ImportPath: e.importPath, Package: e.pkg, Constructor: "New", ProviderConstructor: "NewProvider"}})
	}
	catalog, err := assemblyv1.NewSourceCatalog(records)
	must(err)
	evidence := assemblyv1.P1P2PortEvidence()
	plan, err := (assemblyv1.Compiler{Ports: port.PublicCatalog(), Sources: catalog, PortEvidence: evidence}).Compile(context.Background(), recipe)
	must(err)
	generated, err := assemblyv1.GenerateRuntimeAssembly(plan, "assembly")
	must(err)
	must(os.WriteFile(*output, generated, 0o644))
}
func must(err error) {
	if err != nil {
		panic(err)
	}
}
