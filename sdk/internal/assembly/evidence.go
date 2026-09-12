package assembly

import "agent-vivy/sdk/port"

// SupportedPortEvidence is the build-owned evidence ledger for Ports that
// completed their phase gates. Unlisted public Ports remain SPECIFIED and
// cannot be selected by the compiler.
func SupportedPortEvidence() map[string]port.SupportEvidence {
	return map[string]port.SupportEvidence{
		"std/tool@v1": completedEvidence(map[port.EvidenceKind]string{
			port.EvidencePortDefinition:    "sdk/port/catalog.go#std-tool-v1",
			port.EvidenceSDKContract:       "sdk/port/tool/tool.go#ToolProvider",
			port.EvidenceHostConsumer:      "internal/app/assembly_tools.go#bindGeneratedTools",
			port.EvidenceRealProvider:      "internal/modules/defaults/providers.go#ProtectedToolProviders",
			port.EvidenceFailureModel:      "internal/app/default_generation_test.go#TestGeneratedToolInventoryIsAuthoritative",
			port.EvidenceConformanceSuite:  "internal/app/assembly_governance_e2e_test.go#TestToolEnvelopeConformanceAcrossAllSourceClasses",
			port.EvidenceInspectProjection: "sdk/internal/frontend_v1_test.go#TestV1PackAndInspectProveRecipeRemoval",
		}),
		"std/tool-world@v1": completedEvidence(map[port.EvidenceKind]string{
			port.EvidencePortDefinition:    "sdk/port/catalog.go#std-tool-world-v1",
			port.EvidenceSDKContract:       "sdk/port/toolworld/toolworld.go#Provider",
			port.EvidenceHostConsumer:      "internal/app/assembly_worlds.go#bindToolWorlds",
			port.EvidenceRealProvider:      "plugins/hello-fs/plugin.go#NewProvider",
			port.EvidenceFailureModel:      "internal/app/assembly_worlds_test.go#TestGeneratedToolWorldInvokesThroughGrantedHost",
			port.EvidenceConformanceSuite:  "sdk/internal/assembly/p1_p2_conformance_test.go#TestP1P2PortConformanceSuite",
			port.EvidenceInspectProjection: "sdk/internal/frontend_v1_test.go#TestPackSelectedToolWorlds",
		}),
		"std/channel@v1": completedEvidence(map[port.EvidenceKind]string{
			port.EvidencePortDefinition:    "sdk/port/catalog.go#std-channel-v1",
			port.EvidenceSDKContract:       "sdk/port/channel/channel.go#ChannelProvider",
			port.EvidenceHostConsumer:      "internal/app/channels.go#bindChannels",
			port.EvidenceRealProvider:      "plugins/dingtalk/module_v1.go#NewProvider",
			port.EvidenceFailureModel:      "internal/app/channels_test.go#TestBindChannelsRejectsUncompiledConfig",
			port.EvidenceConformanceSuite:  "sdk/internal/assembly/p1_p2_conformance_test.go#TestP1P2PortConformanceSuite",
			port.EvidenceInspectProjection: "sdk/internal/frontend_v1_test.go#TestPackAndInspectEveryShippedRecipe",
		}),
		"std/face@v1": completedEvidence(map[port.EvidenceKind]string{
			port.EvidencePortDefinition:    "sdk/port/catalog.go#std-face-v1",
			port.EvidenceSDKContract:       "sdk/port/face/face.go#FaceProvider",
			port.EvidenceHostConsumer:      "internal/app/facehost.go#RunFaceProviderWithAppOptions",
			port.EvidenceRealProvider:      "faces/headless/module_v1.go#NewProvider",
			port.EvidenceFailureModel:      "internal/app/facehost_test.go#TestRunFaceWithoutOrganFails",
			port.EvidenceConformanceSuite:  "sdk/internal/assembly/p1_p2_conformance_test.go#TestP1P2PortConformanceSuite",
			port.EvidenceInspectProjection: "sdk/internal/frontend_v1_test.go#TestPackAndInspectEveryShippedRecipe",
		}),
		"std/provider-profile@v1": completedEvidence(map[port.EvidenceKind]string{
			port.EvidencePortDefinition:    "sdk/port/catalog.go#std-provider-profile-v1",
			port.EvidenceSDKContract:       "sdk/port/providerprofile/providerprofile.go#Provider",
			port.EvidenceHostConsumer:      "internal/modelhost/profile.go#Host",
			port.EvidenceRealProvider:      "internal/modules/defaults/providers.go#ProviderProfiles",
			port.EvidenceFailureModel:      "internal/modelhost/profile_test.go#TestResolveExecutableRejectsDeferredAdapter",
			port.EvidenceConformanceSuite:  "internal/modules/defaults/providers_test.go#TestDefaultProviderProfilesMatchExistingRuntimeFamilies",
			port.EvidenceInspectProjection: "sdk/generation/manifest.go#Manifest.ProviderProfiles",
		}),
		"std/middleware/pre-tool@v1": completedEvidence(map[port.EvidenceKind]string{
			port.EvidencePortDefinition:    "sdk/port/catalog.go#std-middleware-pre-tool-v1",
			port.EvidenceSDKContract:       "sdk/port/pretool/pretool.go#Provider",
			port.EvidenceHostConsumer:      "internal/toolhost/middleware.go#ApplyMiddleware",
			port.EvidenceRealProvider:      "plugins/governance/provider.go#Provider.Evaluate",
			port.EvidenceFailureModel:      "internal/toolhost/middleware_test.go#TestMiddlewarePanicAndInvalidDecisionFailClosed",
			port.EvidenceConformanceSuite:  "internal/app/assembly_governance_e2e_test.go#TestToolEnvelopeConformanceAcrossAllSourceClasses",
			port.EvidenceInspectProjection: "internal/toolhost/host.go#ListVisible",
		}),
		"std/observer/run@v1": completedEvidence(map[port.EvidenceKind]string{
			port.EvidencePortDefinition:    "sdk/port/catalog.go#std-observer-run-v1",
			port.EvidenceSDKContract:       "sdk/port/observer/observer.go#RunProvider",
			port.EvidenceHostConsumer:      "internal/observerhost/host.go#DeliverRun",
			port.EvidenceRealProvider:      "plugins/governance/provider.go#Provider.ObserveRun",
			port.EvidenceFailureModel:      "internal/observerhost/host_test.go#TestRunObserverCanReceiveDuplicateStableEventID",
			port.EvidenceConformanceSuite:  "internal/observerhost/host_test.go#TestRunObserverSeesOnlyCommittedEvents",
			port.EvidenceInspectProjection: "sdk/port/observer/observer.go#EventID",
		}),
		"std/observer/diagnostic@v1": completedEvidence(map[port.EvidenceKind]string{
			port.EvidencePortDefinition:    "sdk/port/catalog.go#std-observer-diagnostic-v1",
			port.EvidenceSDKContract:       "sdk/port/observer/observer.go#DiagnosticProvider",
			port.EvidenceHostConsumer:      "internal/observerhost/host.go#EmitDiagnostic",
			port.EvidenceRealProvider:      "plugins/governance/provider.go#Provider.ObserveDiagnostic",
			port.EvidenceFailureModel:      "internal/observerhost/host_test.go#TestDiagnosticOverloadIncrementsDropCounterWithoutBlocking",
			port.EvidenceConformanceSuite:  "sdk/port/observer/observer_test.go#TestDiagnosticBoundsMessageAndFields",
			port.EvidenceInspectProjection: "internal/observerhost/host.go#DiagnosticDrops",
		}),
		"std/status-source@v1": completedEvidence(map[port.EvidenceKind]string{
			port.EvidencePortDefinition:    "sdk/port/catalog.go#std-status-source-v1",
			port.EvidenceSDKContract:       "sdk/port/status/status.go#Provider",
			port.EvidenceHostConsumer:      "internal/statushost/host.go#Read",
			port.EvidenceRealProvider:      "plugins/governance/provider.go#Provider.Status",
			port.EvidenceFailureModel:      "internal/statushost/host_test.go#TestStatusTimeoutProjectsUnavailable",
			port.EvidenceConformanceSuite:  "internal/statushost/host_test.go#TestStatusReadDoesNotStartOrProbeProvider",
			port.EvidenceInspectProjection: "internal/statushost/host.go#Result",
		}),
		"std/context-source@v1": completedEvidence(map[port.EvidenceKind]string{
			port.EvidencePortDefinition:    "sdk/port/catalog.go#std-context-source-v1",
			port.EvidenceSDKContract:       "sdk/port/contextsource/contextsource.go#Provider",
			port.EvidenceHostConsumer:      "internal/contexthost/host.go#Host.QuerySources",
			port.EvidenceRealProvider:      "internal/modules/defaults/providers.go#ContextSourceProviders",
			port.EvidenceFailureModel:      "internal/contexthost/conformance_test.go#TestContextSourceConformance",
			port.EvidenceConformanceSuite:  "internal/contexthost/conformance_test.go#TestContextSourceConformance",
			port.EvidenceInspectProjection: "internal/app/assembly_validate.go#validateRuntimeAssembly",
		}),
		"std/skill-source@v1": completedEvidence(map[port.EvidenceKind]string{
			port.EvidencePortDefinition:    "sdk/port/catalog.go#std-skill-source-v1",
			port.EvidenceSDKContract:       "sdk/port/skillsource/skillsource.go#Provider",
			port.EvidenceHostConsumer:      "internal/skillhost/host.go#Host.Get",
			port.EvidenceRealProvider:      "internal/modules/defaults/providers.go#SkillSourceProviders",
			port.EvidenceFailureModel:      "internal/skillhost/conformance_test.go#TestSkillSourceConformanceTimeoutAndUnavailable",
			port.EvidenceConformanceSuite:  "internal/skillhost/conformance_test.go#TestSkillSourceConformance",
			port.EvidenceInspectProjection: "internal/app/assembly_validate.go#validateRuntimeAssembly",
		}),
		UIExtensionPort: completedEvidence(map[port.EvidenceKind]string{
			port.EvidencePortDefinition:    "sdk/port/catalog.go#std-ui-extension-v1",
			port.EvidenceSDKContract:       "sdk/ui/src/module.ts#UIExtension",
			port.EvidenceHostConsumer:      "ui/src/plugins/presentation-host.tsx#PresentationHost",
			port.EvidenceRealProvider:      "ui/src/generated/assembly.ts#generatedUIExtensions",
			port.EvidenceFailureModel:      "ui/src/plugins/conformance.test.tsx#rolls back a runtime install failure",
			port.EvidenceConformanceSuite:  "ui/src/plugins/conformance.test.tsx#full UI module conformance",
			port.EvidenceInspectProjection: "sdk/internal/frontend_v1_test.go#TestPackBuildsSelectedUIIntoFinalArtifact",
		}),
		UIRootPort: completedEvidence(map[port.EvidenceKind]string{
			port.EvidencePortDefinition:    "sdk/port/catalog.go#std-ui-root-v1",
			port.EvidenceSDKContract:       "sdk/ui/src/module.ts#UIRoot",
			port.EvidenceHostConsumer:      "ui/src/plugins/presentation-host.tsx#PresentationHost",
			port.EvidenceRealProvider:      "ui/src/generated/assembly.ts#generatedUIRoot",
			port.EvidenceFailureModel:      "ui/src/plugins/conformance.test.tsx#rolls back a runtime install failure",
			port.EvidenceConformanceSuite:  "ui/src/plugins/conformance.test.tsx#builds and presents default, extension, replacement-root, and minimal generations",
			port.EvidenceInspectProjection: "sdk/internal/frontend_v1_test.go#TestPackBuildsSelectedUIIntoFinalArtifact",
		}),
		"std/control-action@v1": completedEvidence(map[port.EvidenceKind]string{
			port.EvidencePortDefinition:    "sdk/port/catalog.go#std-control-action-v1",
			port.EvidenceSDKContract:       "sdk/port/controlaction/action.go#Provider",
			port.EvidenceHostConsumer:      "internal/actionhost/host.go#Host",
			port.EvidenceRealProvider:      "sdk/internal/testdata/full-ui-module/module.go#NewProvider",
			port.EvidenceFailureModel:      "internal/rpc/module_action_test.go#TestModuleActionRPCRejectsSpoofedModuleIDAndForgedAuthorityClaims",
			port.EvidenceConformanceSuite:  "internal/rpc/module_action_test.go#TestModuleActionRPCUsesAuthenticatedCallerAndReturnsBoundResult",
			port.EvidenceInspectProjection: "sdk/internal/frontend_v1_test.go#TestPackBuildsSelectedUIIntoFinalArtifact",
		}),
	}
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
