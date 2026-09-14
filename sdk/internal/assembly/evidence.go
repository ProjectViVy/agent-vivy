package assembly

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	providerconformance "agent-vivy/sdk/conformance"
	"agent-vivy/sdk/module"
	"agent-vivy/sdk/port"
)

//go:embed conformance_results.json
var conformanceResultBundleJSON []byte

type conformanceResultBundle struct {
	APIVersion     string                  `json:"apiVersion"`
	RequiredChecks []string                `json:"requiredChecks"`
	Suites         []attestedProviderSuite `json:"suites"`
}

type attestedProviderSuite struct {
	Port         string   `json:"port"`
	ProviderID   string   `json:"providerId"`
	SourceSHA256 string   `json:"sourceSha256"`
	EvidenceID   string   `json:"evidenceId"`
	PassedChecks []string `json:"passedChecks"`
}

// SupportedPortEvidence is the build-owned evidence ledger for Ports that
// completed their phase gates. Unlisted public Ports remain SPECIFIED and
// cannot be selected by the compiler.
func SupportedPortEvidence() map[string]port.SupportEvidence {
	return map[string]port.SupportEvidence{
		"std/tool@v1": completedEvidence(map[port.EvidenceKind]string{
			port.EvidencePortDefinition:    "sdk/port/catalog.go#PublicCatalog",
			port.EvidenceSDKContract:       "sdk/port/tool/tool.go#ToolProvider",
			port.EvidenceHostConsumer:      "internal/app/assembly_tools.go#bindGeneratedTools",
			port.EvidenceRealProvider:      "internal/modules/defaults/providers.go#ProtectedToolProviders",
			port.EvidenceFailureModel:      "internal/app/default_generation_test.go#TestGeneratedToolInventoryIsAuthoritative",
			port.EvidenceConformanceSuite:  "internal/app/assembly_governance_e2e_test.go#TestToolEnvelopeConformanceAcrossAllSourceClasses",
			port.EvidenceInspectProjection: "sdk/internal/frontend_v1_test.go#TestV1PackAndInspectProveRecipeRemoval",
		}),
		"std/tool-world@v1": completedEvidence(map[port.EvidenceKind]string{
			port.EvidencePortDefinition:    "sdk/port/catalog.go#PublicCatalog",
			port.EvidenceSDKContract:       "sdk/port/toolworld/toolworld.go#Provider",
			port.EvidenceHostConsumer:      "internal/app/assembly_worlds.go#bindToolWorlds",
			port.EvidenceRealProvider:      "plugins/hello-fs/plugin.go#NewProvider",
			port.EvidenceFailureModel:      "internal/app/assembly_worlds_test.go#TestGeneratedToolWorldInvokesThroughGrantedHost",
			port.EvidenceConformanceSuite:  "sdk/internal/assembly/p1_p2_conformance_test.go#TestP1P2PortConformanceSuite",
			port.EvidenceInspectProjection: "sdk/internal/frontend_v1_test.go#TestPackSelectedToolWorlds",
		}),
		"std/channel@v1": completedEvidence(map[port.EvidenceKind]string{
			port.EvidencePortDefinition:    "sdk/port/catalog.go#PublicCatalog",
			port.EvidenceSDKContract:       "sdk/port/channel/channel.go#ChannelProvider",
			port.EvidenceHostConsumer:      "internal/app/channels.go#bindChannels",
			port.EvidenceRealProvider:      "plugins/dingtalk/module_v1.go#NewProvider",
			port.EvidenceFailureModel:      "internal/app/channels_test.go#TestBindChannelsRejectsUncompiledConfig",
			port.EvidenceConformanceSuite:  "sdk/internal/assembly/p1_p2_conformance_test.go#TestP1P2PortConformanceSuite",
			port.EvidenceInspectProjection: "sdk/internal/frontend_v1_test.go#TestPackAndInspectEveryShippedRecipe",
		}),
		"std/face@v1": completedEvidence(map[port.EvidenceKind]string{
			port.EvidencePortDefinition:    "sdk/port/catalog.go#PublicCatalog",
			port.EvidenceSDKContract:       "sdk/port/face/face.go#FaceProvider",
			port.EvidenceHostConsumer:      "internal/app/facehost.go#RunFaceProviderWithAppOptions",
			port.EvidenceRealProvider:      "faces/headless/module_v1.go#NewProvider",
			port.EvidenceFailureModel:      "internal/app/facehost_test.go#TestRunFaceWithoutOrganFails",
			port.EvidenceConformanceSuite:  "sdk/internal/assembly/p1_p2_conformance_test.go#TestP1P2PortConformanceSuite",
			port.EvidenceInspectProjection: "sdk/internal/frontend_v1_test.go#TestPackAndInspectEveryShippedRecipe",
		}),
		"std/provider-profile@v1": completedEvidence(map[port.EvidenceKind]string{
			port.EvidencePortDefinition:    "sdk/port/catalog.go#PublicCatalog",
			port.EvidenceSDKContract:       "sdk/port/providerprofile/providerprofile.go#Provider",
			port.EvidenceHostConsumer:      "internal/modelhost/profile.go#Host",
			port.EvidenceRealProvider:      "internal/modules/defaults/providers.go#ProviderProfiles",
			port.EvidenceFailureModel:      "internal/modelhost/profile_test.go#TestResolveExecutableRejectsDeferredAdapter",
			port.EvidenceConformanceSuite:  "internal/modules/defaults/providers_test.go#TestDefaultProviderProfilesMatchExistingRuntimeFamilies",
			port.EvidenceInspectProjection: "sdk/generation/manifest.go#ProviderProfiles",
		}),
		"std/middleware/pre-tool@v1": completedEvidence(map[port.EvidenceKind]string{
			port.EvidencePortDefinition:    "sdk/port/catalog.go#PublicCatalog",
			port.EvidenceSDKContract:       "sdk/port/pretool/pretool.go#Provider",
			port.EvidenceHostConsumer:      "internal/toolhost/middleware.go#ApplyMiddleware",
			port.EvidenceRealProvider:      "plugins/governance/provider.go#Evaluate",
			port.EvidenceFailureModel:      "internal/toolhost/middleware_test.go#TestMiddlewarePanicAndInvalidDecisionFailClosed",
			port.EvidenceConformanceSuite:  "internal/app/assembly_governance_e2e_test.go#TestToolEnvelopeConformanceAcrossAllSourceClasses",
			port.EvidenceInspectProjection: "internal/toolhost/host.go#ListVisible",
		}),
		"std/observer/run@v1": completedEvidence(map[port.EvidenceKind]string{
			port.EvidencePortDefinition:    "sdk/port/catalog.go#PublicCatalog",
			port.EvidenceSDKContract:       "sdk/port/observer/observer.go#RunProvider",
			port.EvidenceHostConsumer:      "internal/observerhost/host.go#DeliverRun",
			port.EvidenceRealProvider:      "plugins/scx-reference/module.go#ObserveRunWithReceipt",
			port.EvidenceFailureModel:      "internal/observerhost/scx_conformance_test.go#TestSCXMemoryReturnUsesCommittedFilteredReceiptAwareDelivery",
			port.EvidenceConformanceSuite:  "internal/observerhost/scx_conformance_test.go#TestSCXObserverWorkerResumesPendingDeliveryAfterReconnect",
			port.EvidenceInspectProjection: "sdk/internal/frontend_v1_test.go#TestPackAndInspectSCXCandidateIsDeterministicAndRemovable",
		}),
		"std/observer/diagnostic@v1": completedEvidence(map[port.EvidenceKind]string{
			port.EvidencePortDefinition:    "sdk/port/catalog.go#PublicCatalog",
			port.EvidenceSDKContract:       "sdk/port/observer/observer.go#DiagnosticProvider",
			port.EvidenceHostConsumer:      "internal/observerhost/host.go#EmitDiagnostic",
			port.EvidenceRealProvider:      "plugins/governance/provider.go#ObserveDiagnostic",
			port.EvidenceFailureModel:      "internal/observerhost/host_test.go#TestDiagnosticOverloadIncrementsDropCounterWithoutBlocking",
			port.EvidenceConformanceSuite:  "sdk/port/observer/observer_test.go#TestDiagnosticBoundsMessageAndFields",
			port.EvidenceInspectProjection: "internal/observerhost/host.go#DiagnosticDrops",
		}),
		"std/status-source@v1": completedEvidence(map[port.EvidenceKind]string{
			port.EvidencePortDefinition:    "sdk/port/catalog.go#PublicCatalog",
			port.EvidenceSDKContract:       "sdk/port/status/status.go#Provider",
			port.EvidenceHostConsumer:      "internal/statushost/host.go#Read",
			port.EvidenceRealProvider:      "plugins/governance/provider.go#Status",
			port.EvidenceFailureModel:      "internal/statushost/host_test.go#TestStatusTimeoutProjectsUnavailable",
			port.EvidenceConformanceSuite:  "internal/statushost/host_test.go#TestStatusReadDoesNotStartOrProbeProvider",
			port.EvidenceInspectProjection: "internal/statushost/host.go#Result",
		}),
		"std/context-source@v1": completedEvidence(map[port.EvidenceKind]string{
			port.EvidencePortDefinition:    "sdk/port/catalog.go#PublicCatalog",
			port.EvidenceSDKContract:       "sdk/port/contextsource/contextsource.go#Provider",
			port.EvidenceHostConsumer:      "internal/contexthost/host.go#QuerySources",
			port.EvidenceRealProvider:      "plugins/scx-reference/module.go#Provider",
			port.EvidenceFailureModel:      "internal/contexthost/scx_conformance_test.go#TestSCXRequiredContextCannotBeSilentlyDroppedForBudget",
			port.EvidenceConformanceSuite:  "internal/contexthost/scx_conformance_test.go#TestSCXExactVersionResourceResolutionIsScopedBoundedAndReplayable",
			port.EvidenceInspectProjection: "sdk/internal/frontend_v1_test.go#TestPackAndInspectSCXCandidateIsDeterministicAndRemovable",
		}),
		"std/skill-source@v1": completedEvidence(map[port.EvidenceKind]string{
			port.EvidencePortDefinition:    "sdk/port/catalog.go#PublicCatalog",
			port.EvidenceSDKContract:       "sdk/port/skillsource/skillsource.go#Provider",
			port.EvidenceHostConsumer:      "internal/skillhost/host.go#Get",
			port.EvidenceRealProvider:      "internal/modules/defaults/providers.go#SkillSourceProviders",
			port.EvidenceFailureModel:      "internal/skillhost/conformance_test.go#TestSkillSourceConformanceTimeoutAndUnavailable",
			port.EvidenceConformanceSuite:  "internal/skillhost/conformance_test.go#TestSkillSourceConformance",
			port.EvidenceInspectProjection: "internal/app/assembly_validate.go#validateRuntimeAssembly",
		}),
		UIExtensionPort: completedEvidence(map[port.EvidenceKind]string{
			port.EvidencePortDefinition:    "sdk/port/catalog.go#PublicCatalog",
			port.EvidenceSDKContract:       "sdk/ui/src/module.ts#UIExtension",
			port.EvidenceHostConsumer:      "ui/src/plugins/presentation-host.tsx#PresentationHost",
			port.EvidenceRealProvider:      "ui/src/generated/assembly.ts#generatedUIExtensions",
			port.EvidenceFailureModel:      "ui/src/plugins/conformance.test.tsx#rolls back a runtime install failure",
			port.EvidenceConformanceSuite:  "ui/src/plugins/conformance.test.tsx#full UI module conformance",
			port.EvidenceInspectProjection: "sdk/internal/frontend_v1_test.go#TestPackBuildsSelectedUIIntoFinalArtifact",
		}),
		UIRootPort: completedEvidence(map[port.EvidenceKind]string{
			port.EvidencePortDefinition:    "sdk/port/catalog.go#PublicCatalog",
			port.EvidenceSDKContract:       "sdk/ui/src/module.ts#UIRoot",
			port.EvidenceHostConsumer:      "ui/src/plugins/presentation-host.tsx#PresentationHost",
			port.EvidenceRealProvider:      "ui/src/generated/assembly.ts#generatedUIRoot",
			port.EvidenceFailureModel:      "ui/src/plugins/conformance.test.tsx#rolls back a runtime install failure",
			port.EvidenceConformanceSuite:  "ui/src/plugins/conformance.test.tsx#builds and presents default, extension, replacement-root, and minimal generations",
			port.EvidenceInspectProjection: "sdk/internal/frontend_v1_test.go#TestPackBuildsSelectedUIIntoFinalArtifact",
		}),
		"std/control-action@v1": completedEvidence(map[port.EvidenceKind]string{
			port.EvidencePortDefinition:    "sdk/port/catalog.go#PublicCatalog",
			port.EvidenceSDKContract:       "sdk/port/controlaction/action.go#Provider",
			port.EvidenceHostConsumer:      "internal/actionhost/host.go#Host",
			port.EvidenceRealProvider:      "sdk/internal/testdata/full-ui-module/module.go#NewProvider",
			port.EvidenceFailureModel:      "internal/rpc/module_action_test.go#TestModuleActionRPCRejectsSpoofedModuleIDAndForgedAuthorityClaims",
			port.EvidenceConformanceSuite:  "internal/rpc/module_action_test.go#TestModuleActionRPCUsesAuthenticatedCallerAndReturnsBoundResult",
			port.EvidenceInspectProjection: "sdk/internal/frontend_v1_test.go#TestPackBuildsSelectedUIIntoFinalArtifact",
		}),
	}
}

// ConformanceResultsForPlan projects immutable release results only for the
// exact Provider source selected by the compiled Generation. Result identity
// is never rewritten. Third-party Providers without build-owned release
// evidence remain packable but acquire no conformance records in Inspect.
func ConformanceResultsForPlan(plan AssemblyPlan) ([]providerconformance.ConformanceResult, error) {
	all := SupportedPortConformance()
	results := make([]providerconformance.ConformanceResult, 0)
	seen := make(map[string]struct{})
	add := func(ref module.PortRef, providerID, sourceSHA256 string) error {
		if !strings.HasPrefix(ref.Port, "std/") {
			return nil
		}
		key := ref.Port + "\x00" + providerID + "\x00" + sourceSHA256
		if _, exists := seen[key]; exists {
			return nil
		}
		selected := make([]providerconformance.ConformanceResult, 0, len(providerconformance.RequiredProviderChecks()))
		for _, result := range all {
			if result.Port.Port == ref.Port && result.ProviderID == providerID && result.SourceSHA256 == sourceSHA256 {
				selected = append(selected, result)
			}
		}
		if len(selected) == 0 {
			seen[key] = struct{}{}
			return nil
		}
		if !completePassingResults(selected) {
			return fmt.Errorf("conformance: selected Provider %s source %s has no complete passing release result for %s", providerID, sourceSHA256, ref.Port)
		}
		results = append(results, selected...)
		seen[key] = struct{}{}
		return nil
	}
	for _, resolved := range plan.Modules {
		for _, provided := range resolved.Descriptor.Provides {
			if err := add(provided, resolved.Descriptor.Module.ID, resolved.Descriptor.Source.SHA256); err != nil {
				return nil, err
			}
		}
	}
	return providerconformance.CanonicalResults(results)
}

// SupportedPortConformance expands the checked-in release result bundle. The
// bundle records an explicit outcome for every mandatory check and binds it to
// the exact Provider source and focused-suite evidence path. It is independent
// of SupportEvidence.GatesPassed and cannot be promoted by documentation claims.
func SupportedPortConformance() []providerconformance.ConformanceResult {
	var bundle conformanceResultBundle
	decoder := json.NewDecoder(strings.NewReader(string(conformanceResultBundleJSON)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&bundle); err != nil {
		panic(fmt.Errorf("conformance: decode release result bundle: %w", err))
	}
	if bundle.APIVersion != "vivy.provider-conformance-results/v1" {
		panic(fmt.Errorf("conformance: unsupported result bundle apiVersion %q", bundle.APIVersion))
	}
	if !reflect.DeepEqual(bundle.RequiredChecks, providerconformance.RequiredProviderChecks()) {
		panic(fmt.Errorf("conformance: release result bundle mandatory-check schema drift"))
	}
	results := make([]providerconformance.ConformanceResult, 0, len(bundle.Suites)*len(bundle.RequiredChecks))
	for _, suite := range bundle.Suites {
		if !reflect.DeepEqual(suite.PassedChecks, bundle.RequiredChecks) {
			panic(fmt.Errorf("conformance: Provider %s result bundle for %s is partial or out of canonical order", suite.ProviderID, suite.Port))
		}
		for _, check := range suite.PassedChecks {
			results = append(results, providerconformance.ConformanceResult{
				Port: module.PortRef{Port: suite.Port}, ProviderID: suite.ProviderID,
				SourceSHA256: suite.SourceSHA256, Suite: check, Passed: true, EvidenceID: suite.EvidenceID,
			})
		}
	}
	canonical, err := providerconformance.CanonicalResults(results)
	if err != nil {
		panic(err)
	}
	return canonical
}

func completePassingResults(results []providerconformance.ConformanceResult) bool {
	passed := make(map[string]bool, len(results))
	for _, result := range results {
		passed[result.Suite] = result.Passed
	}
	for _, required := range providerconformance.RequiredProviderChecks() {
		if !passed[required] {
			return false
		}
	}
	return true
}

// P6UIPortEvidence retains the P6-focused view for its conformance tests while
// deriving every record from the single supported-Port evidence ledger.
func P6UIPortEvidence() map[string]port.SupportEvidence {
	evidence := SupportedPortEvidence()
	return map[string]port.SupportEvidence{
		UIExtensionPort:         evidence[UIExtensionPort],
		UIRootPort:              evidence[UIRootPort],
		"std/control-action@v1": evidence["std/control-action@v1"],
	}
}

// P1P2PortEvidence retains the original entry point for repository callers
// while all completed phases share one support ledger.
func P1P2PortEvidence() map[string]port.SupportEvidence {
	return SupportedPortEvidence()
}

func completedEvidence(ids map[port.EvidenceKind]string) port.SupportEvidence {
	refs := make([]port.EvidenceReference, 0, len(port.RequiredEvidenceKinds()))
	for _, kind := range port.RequiredEvidenceKinds() {
		refs = append(refs, port.EvidenceReference{Kind: kind, ID: ids[kind]})
	}
	return port.SupportEvidence{References: refs, GatesPassed: true}
}
