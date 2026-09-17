package workflow

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// definitionSchema is kept beside the implementation so the same structural
// contract is available in packaged binaries. The normative copy lives under
// docs/plans/workflow-system/schema and is intentionally kept byte-for-byte
// aligned with this embedded resource.
//
//go:embed schema/workflow-definition.schema.json
var definitionSchema []byte

type ModelCapability struct {
	Provider   string
	Model      string
	Configured bool
	Active     bool
}

type ToolCapability struct {
	Readonly  bool
	Available bool
}

// Limits is the host-supplied workflow budget envelope. Validation never
// reads process-global configuration, which keeps the pure package reusable in
// tests and in future non-HTTP hosts.
type Limits struct {
	MaxNodes           int
	MaxParallelism     int
	MaxNodeOutputBytes int
	DefinitionMaxBytes int
}

func DefaultLimits() Limits {
	return Limits{
		MaxNodes:           32,
		MaxParallelism:     4,
		MaxNodeOutputBytes: 64 * 1024,
		DefinitionMaxBytes: 256 * 1024,
	}
}

// ValidationContext contains snapshots taken by the host at the validation
// boundary. Tools is the catalog visible to the generation; AvailableTools is
// optional and narrows that catalog for an invocation principal.
type ValidationContext struct {
	ModelProfiles  map[string]ModelCapability
	Tools          map[string]ToolCapability
	AvailableTools map[string]bool
	Limits         Limits
}

type ValidationResult struct {
	Definition  Definition
	Canonical   []byte
	Hash        string
	Order       []string
	Diagnostics []Diagnostic
}

func Validate(raw []byte, ctx ValidationContext) (ValidationResult, error) {
	return validate(raw, ctx, false)
}

// ValidateInvocation repeats validation with the invoking principal's tool
// set. Definition validation deliberately only checks catalog membership;
// invocation validation also applies the narrower grant snapshot.
func ValidateInvocation(raw []byte, ctx ValidationContext) (ValidationResult, error) {
	return validate(raw, ctx, true)
}

func ValidateDefinition(def Definition, canonical []byte, ctx ValidationContext) (ValidationResult, error) {
	if len(canonical) == 0 {
		var err error
		canonical, err = json.Marshal(def)
		if err != nil {
			return ValidationResult{}, validationError(
				[]Diagnostic{diagnostic("schema", "", "definition cannot be serialized")},
				ErrSchemaInvalid,
			)
		}
	}
	return validateDecoded(def, canonical, ctx, false)
}

func validate(raw []byte, ctx ValidationContext, invocation bool) (ValidationResult, error) {
	definition, canonical, hash, err := Decode(raw)
	if err != nil {
		return ValidationResult{}, validationError(
			[]Diagnostic{diagnostic("schema", "", err.Error())},
			ErrSchemaInvalid,
		)
	}
	result, err := validateDecoded(definition, canonical, ctx, invocation)
	if result.Hash == "" {
		result.Hash = hash
	}
	return result, err
}

func validateDecoded(definition Definition, canonical []byte, ctx ValidationContext, invocation bool) (ValidationResult, error) {
	result := ValidationResult{
		Definition: definition,
		Canonical:  append([]byte(nil), canonical...),
	}
	result.Hash = hashCanonical(canonical)

	diagnostics, err := validateSchema(canonical)
	if err != nil {
		result.Diagnostics = append(result.Diagnostics, diagnostics...)
		return result, validationError(diagnostics, ErrSchemaInvalid)
	}

	order, topologyDiagnostics, topologyCauses := validateTopology(definition)
	result.Order = append([]string(nil), order...)
	result.Diagnostics = append(result.Diagnostics, topologyDiagnostics...)

	capabilityDiagnostics, capabilityCauses := validateCapabilities(definition, ctx)
	result.Diagnostics = append(result.Diagnostics, capabilityDiagnostics...)

	budgetDiagnostics, budgetCauses := validateBudget(definition, canonical, order, ctx.Limits)
	result.Diagnostics = append(result.Diagnostics, budgetDiagnostics...)

	authorityDiagnostics, authorityCauses := validateAuthority(definition, ctx, invocation)
	result.Diagnostics = append(result.Diagnostics, authorityDiagnostics...)

	causes := make([]error, 0, len(topologyCauses)+len(capabilityCauses)+len(budgetCauses)+len(authorityCauses))
	causes = append(causes, topologyCauses...)
	causes = append(causes, capabilityCauses...)
	causes = append(causes, budgetCauses...)
	causes = append(causes, authorityCauses...)
	return result, validationError(result.Diagnostics, causes...)
}

func validateSchema(canonical []byte) ([]Diagnostic, error) {
	schemaDocument, err := jsonschema.UnmarshalJSON(bytes.NewReader(definitionSchema))
	if err != nil {
		return []Diagnostic{diagnostic("schema", "", "embedded workflow schema is invalid")}, err
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("vivy-workflow-definition.json", schemaDocument); err != nil {
		return []Diagnostic{diagnostic("schema", "", "workflow schema could not be compiled")}, err
	}
	compiled, err := compiler.Compile("vivy-workflow-definition.json")
	if err != nil {
		return []Diagnostic{diagnostic("schema", "", "workflow schema could not be compiled")}, err
	}
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(canonical))
	if err != nil {
		return []Diagnostic{diagnostic("schema", "", "canonical workflow is not valid JSON")}, err
	}
	if err := compiled.Validate(instance); err != nil {
		return []Diagnostic{diagnostic("schema", "", err.Error())}, err
	}
	return nil, nil
}

func validateTopology(definition Definition) ([]string, []Diagnostic, []error) {
	diagnostics := make([]Diagnostic, 0)
	causes := make([]error, 0)
	index := make(map[string]int, len(definition.Nodes))
	for i, node := range definition.Nodes {
		if previous, exists := index[node.ID]; exists {
			diagnostics = append(diagnostics, diagnostic("topology", fmt.Sprintf("nodes[%d].id", i), fmt.Sprintf("duplicates nodes[%d].id", previous)))
			continue
		}
		index[node.ID] = i
	}

	adjacency := make([][]int, len(definition.Nodes))
	indegree := make([]int, len(definition.Nodes))
	edges := make(map[[2]int]struct{}, len(definition.Edges))
	for i, edge := range definition.Edges {
		from, fromOK := index[edge.From]
		to, toOK := index[edge.To]
		if !fromOK {
			diagnostics = append(diagnostics, diagnostic("topology", fmt.Sprintf("edges[%d].from", i), fmt.Sprintf("node %q does not exist", edge.From)))
		}
		if !toOK {
			diagnostics = append(diagnostics, diagnostic("topology", fmt.Sprintf("edges[%d].to", i), fmt.Sprintf("node %q does not exist", edge.To)))
		}
		if !fromOK || !toOK {
			continue
		}
		key := [2]int{from, to}
		if _, duplicate := edges[key]; duplicate {
			diagnostics = append(diagnostics, diagnostic("topology", fmt.Sprintf("edges[%d]", i), "duplicate edge"))
			continue
		}
		edges[key] = struct{}{}
		adjacency[from] = append(adjacency[from], to)
		indegree[to]++
	}
	for i := range adjacency {
		sort.Ints(adjacency[i])
	}

	entries := make([]int, 0)
	for i, degree := range indegree {
		if degree == 0 {
			entries = append(entries, i)
		}
	}
	if len(entries) != 1 {
		diagnostics = append(diagnostics, diagnostic("topology", "nodes", fmt.Sprintf("workflow must have exactly one entry node, found %d", len(entries))))
		causes = append(causes, ErrTopologyInvalid)
	}

	order := kahnOrder(indegree, adjacency)
	if len(order) != len(definition.Nodes) {
		diagnostics = append(diagnostics, diagnostic("topology", "edges", "workflow graph contains a cycle"))
		causes = append(causes, ErrTopologyInvalid)
	}

	if len(entries) == 1 {
		reachable := make([]bool, len(definition.Nodes))
		stack := []int{entries[0]}
		for len(stack) > 0 {
			last := len(stack) - 1
			node := stack[last]
			stack = stack[:last]
			if reachable[node] {
				continue
			}
			reachable[node] = true
			stack = append(stack, adjacency[node]...)
		}
		for i, ok := range reachable {
			if !ok {
				diagnostics = append(diagnostics, diagnostic("topology", fmt.Sprintf("nodes[%d]", i), "node is not reachable from the entry node"))
				causes = append(causes, ErrTopologyInvalid)
			}
		}
	}

	outputNames := make(map[string]int, len(definition.Outputs))
	for i, output := range definition.Outputs {
		if previous, exists := outputNames[output.Name]; exists {
			diagnostics = append(diagnostics, diagnostic("topology", fmt.Sprintf("outputs[%d].name", i), fmt.Sprintf("duplicates outputs[%d].name", previous)))
			causes = append(causes, ErrTopologyInvalid)
			continue
		}
		outputNames[output.Name] = i
	}

	for i, node := range definition.Nodes {
		var templates []struct {
			path  string
			value string
			cover bool
		}
		switch node.Kind {
		case NodeKindModel:
			var config modelConfig
			if err := json.Unmarshal(node.Config, &config); err == nil {
				templates = append(templates,
					struct {
						path  string
						value string
						cover bool
					}{fmt.Sprintf("nodes[%d].config.user_template", i), config.UserTemplate, true})
				if config.System != "" {
					templates = append(templates, struct {
						path  string
						value string
						cover bool
					}{fmt.Sprintf("nodes[%d].config.system", i), config.System, true})
				}
			}
		case NodeKindAgent:
			var config agentConfig
			if err := json.Unmarshal(node.Config, &config); err == nil {
				templates = append(templates, struct {
					path  string
					value string
					cover bool
				}{fmt.Sprintf("nodes[%d].config.task_template", i), config.TaskTemplate, true})
			}
		case NodeKindIO:
			var config ioConfig
			if err := json.Unmarshal(node.Config, &config); err == nil {
				templates = append(templates, struct {
					path  string
					value string
					cover bool
				}{fmt.Sprintf("nodes[%d].config.template", i), config.Template, true})
			}
		}
		for _, template := range templates {
			checkTemplate(&diagnostics, &causes, template.path, template.value, definition.Inputs, index, edges, node.ID, template.cover)
		}
	}
	for i, output := range definition.Outputs {
		checkTemplate(&diagnostics, &causes, fmt.Sprintf("outputs[%d].template", i), output.Template, definition.Inputs, index, edges, "", false)
	}

	if len(diagnostics) > 0 && len(causes) == 0 {
		causes = append(causes, ErrTopologyInvalid)
	}
	return namesForOrder(order, definition.Nodes), diagnostics, uniqueErrors(causes)
}

func kahnOrder(indegree []int, adjacency [][]int) []int {
	degree := append([]int(nil), indegree...)
	queue := make([]int, 0, len(degree))
	for i, value := range degree {
		if value == 0 {
			queue = append(queue, i)
		}
	}
	order := make([]int, 0, len(degree))
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		order = append(order, node)
		for _, next := range adjacency[node] {
			degree[next]--
			if degree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}
	return order
}

func namesForOrder(order []int, nodes []Node) []string {
	names := make([]string, 0, len(order))
	for _, index := range order {
		if index >= 0 && index < len(nodes) {
			names = append(names, nodes[index].ID)
		}
	}
	return names
}

func checkTemplate(diagnostics *[]Diagnostic, causes *[]error, path, source string, inputs map[string]InputParameter, nodes map[string]int, edges map[[2]int]struct{}, current string, cover bool) {
	template, err := ParseTemplate(source)
	if err != nil {
		*diagnostics = append(*diagnostics, diagnostic("topology", path, err.Error()))
		*causes = append(*causes, ErrTopologyInvalid)
		return
	}
	currentIndex, _ := nodes[current]
	for _, reference := range template.References() {
		switch reference.Kind {
		case ReferenceInput:
			if _, ok := inputs[reference.Name]; !ok {
				*diagnostics = append(*diagnostics, diagnostic("topology", path, fmt.Sprintf("input %q is not declared", reference.Name)))
				*causes = append(*causes, ErrTopologyInvalid)
			}
		case ReferenceNode:
			sourceIndex, ok := nodes[reference.NodeID]
			if !ok {
				*diagnostics = append(*diagnostics, diagnostic("topology", path, fmt.Sprintf("node output reference %q does not exist", reference.NodeID)))
				*causes = append(*causes, ErrTopologyInvalid)
				continue
			}
			if cover {
				if _, ok := edges[[2]int{sourceIndex, currentIndex}]; !ok {
					*diagnostics = append(*diagnostics, diagnostic("topology", path, fmt.Sprintf("node output reference %q is not covered by a direct edge", reference.NodeID)))
					*causes = append(*causes, ErrTopologyInvalid)
				}
			}
		}
	}
}

type modelConfig struct {
	Profile      string          `json:"profile"`
	System       string          `json:"system"`
	UserTemplate string          `json:"user_template"`
	Sampling     json.RawMessage `json:"sampling"`
}

type agentConfig struct {
	TaskTemplate string   `json:"task_template"`
	Tools        []string `json:"tools"`
	MaxTurns     int      `json:"max_turns"`
}

type ioConfig struct {
	Template string `json:"template"`
}

func validateCapabilities(definition Definition, ctx ValidationContext) ([]Diagnostic, []error) {
	diagnostics := make([]Diagnostic, 0)
	causes := make([]error, 0)
	for i, node := range definition.Nodes {
		if node.Kind != NodeKindModel {
			continue
		}
		var config modelConfig
		if err := json.Unmarshal(node.Config, &config); err != nil {
			continue
		}
		capability, ok := ctx.ModelProfiles[config.Profile]
		if !ok {
			diagnostics = append(diagnostics, diagnostic("capability", fmt.Sprintf("nodes[%d].config.profile", i), fmt.Sprintf("model profile %q is not installed", config.Profile)))
			causes = append(causes, ErrCapabilityUnavailable)
			continue
		}
		if !capability.Configured || !capability.Active {
			diagnostics = append(diagnostics, diagnostic("capability", fmt.Sprintf("nodes[%d].config.profile", i), fmt.Sprintf("model profile %q is not configured and active", config.Profile)))
			causes = append(causes, ErrCapabilityUnavailable)
		}
	}
	if len(diagnostics) > 0 && len(causes) == 0 {
		causes = append(causes, ErrCapabilityUnavailable)
	}
	return diagnostics, uniqueErrors(causes)
}

func validateBudget(definition Definition, canonical []byte, order []string, limits Limits) ([]Diagnostic, []error) {
	diagnostics := make([]Diagnostic, 0)
	causes := make([]error, 0)
	if limits.MaxNodes <= 0 {
		diagnostics = append(diagnostics, diagnostic("budget", "workflow.max_nodes", "must be positive"))
	} else if len(definition.Nodes) > limits.MaxNodes {
		diagnostics = append(diagnostics, diagnostic("budget", "nodes", fmt.Sprintf("contains %d nodes, limit is %d", len(definition.Nodes), limits.MaxNodes)))
	}
	if limits.MaxParallelism <= 0 {
		diagnostics = append(diagnostics, diagnostic("budget", "workflow.max_parallelism", "must be positive"))
	} else if len(order) == len(definition.Nodes) {
		width := workflowWidth(definition)
		if width > limits.MaxParallelism {
			diagnostics = append(diagnostics, diagnostic("budget", "nodes", fmt.Sprintf("graph width is %d, limit is %d", width, limits.MaxParallelism)))
		}
	}
	if limits.MaxNodeOutputBytes <= 0 {
		diagnostics = append(diagnostics, diagnostic("budget", "workflow.max_node_output_bytes", "must be positive"))
	}
	if limits.DefinitionMaxBytes <= 0 {
		diagnostics = append(diagnostics, diagnostic("budget", "workflow.definition_max_bytes", "must be positive"))
	} else if len(canonical) > limits.DefinitionMaxBytes {
		diagnostics = append(diagnostics, diagnostic("budget", "definition", fmt.Sprintf("canonical size is %d bytes, limit is %d", len(canonical), limits.DefinitionMaxBytes)))
	}
	if len(diagnostics) > 0 {
		causes = append(causes, ErrBudgetInvalid)
	}
	return diagnostics, causes
}

func workflowWidth(definition Definition) int {
	n := len(definition.Nodes)
	index := make(map[string]int, n)
	for i, node := range definition.Nodes {
		index[node.ID] = i
	}
	reach := make([][]bool, n)
	for i := range reach {
		reach[i] = make([]bool, n)
	}
	for _, edge := range definition.Edges {
		from, fromOK := index[edge.From]
		to, toOK := index[edge.To]
		if fromOK && toOK {
			reach[from][to] = true
		}
	}
	for k := 0; k < n; k++ {
		for i := 0; i < n; i++ {
			if !reach[i][k] {
				continue
			}
			for j := 0; j < n; j++ {
				reach[i][j] = reach[i][j] || reach[k][j]
			}
		}
	}

	matchedRight := make([]int, n)
	for i := range matchedRight {
		matchedRight[i] = -1
	}
	matching := 0
	for left := 0; left < n; left++ {
		seen := make([]bool, n)
		if augmentWidth(left, reach, matchedRight, seen) {
			matching++
		}
	}
	return n - matching
}

func augmentWidth(left int, reach [][]bool, matchedRight []int, seen []bool) bool {
	for right, connected := range reach[left] {
		if !connected || seen[right] {
			continue
		}
		seen[right] = true
		if matchedRight[right] == -1 || augmentWidth(matchedRight[right], reach, matchedRight, seen) {
			matchedRight[right] = left
			return true
		}
	}
	return false
}

func validateAuthority(definition Definition, ctx ValidationContext, invocation bool) ([]Diagnostic, []error) {
	diagnostics := make([]Diagnostic, 0)
	causes := make([]error, 0)
	for i, node := range definition.Nodes {
		if node.Kind != NodeKindAgent {
			continue
		}
		var config agentConfig
		if err := json.Unmarshal(node.Config, &config); err != nil {
			continue
		}
		for j, name := range config.Tools {
			path := fmt.Sprintf("nodes[%d].config.tools[%d]", i, j)
			if name == "readonly" {
				continue
			}
			capability, ok := ctx.Tools[name]
			if !ok {
				diagnostics = append(diagnostics, diagnostic("authority", path, fmt.Sprintf("tool %q is not in the generation catalog", name)))
				causes = append(causes, ErrAuthorityDenied)
				continue
			}
			if invocation && !toolAvailable(name, capability, ctx) {
				diagnostics = append(diagnostics, diagnostic("authority", path, fmt.Sprintf("tool %q is not available to the invoking principal", name)))
				causes = append(causes, ErrAuthorityDenied)
			}
		}
	}
	if len(diagnostics) > 0 && len(causes) == 0 {
		causes = append(causes, ErrAuthorityDenied)
	}
	return diagnostics, uniqueErrors(causes)
}

func toolAvailable(name string, capability ToolCapability, ctx ValidationContext) bool {
	if ctx.AvailableTools != nil {
		return ctx.AvailableTools[name]
	}
	return capability.Available
}

func uniqueErrors(causes []error) []error {
	seen := make(map[string]struct{}, len(causes))
	unique := make([]error, 0, len(causes))
	for _, cause := range causes {
		if cause == nil {
			continue
		}
		key := cause.Error()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, cause)
	}
	return unique
}
