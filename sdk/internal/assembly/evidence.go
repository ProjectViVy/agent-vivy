package assembly

import "agent-vivy/sdk/port"

// P1P2PortEvidence is the build-owned evidence ledger for the four Ports
// completed by PLG-P2. Unlisted public Ports remain SPECIFIED and cannot be
// selected by the compiler.
func P1P2PortEvidence() map[string]port.SupportEvidence {
	return map[string]port.SupportEvidence{
		"std/tool@v1": completedEvidence(map[port.EvidenceKind]string{
			port.EvidencePortDefinition:    "sdk/port/catalog.go#std-tool-v1",
			port.EvidenceSDKContract:       "sdk/port/tool/tool.go#ToolProvider",
			port.EvidenceHostConsumer:      "internal/app/assembly_tools.go#bindGeneratedTools",
			port.EvidenceRealProvider:      "internal/modules/defaults/providers.go#ProtectedToolProviders",
			port.EvidenceFailureModel:      "internal/app/default_generation_test.go#TestGeneratedToolInventoryIsAuthoritative",
			port.EvidenceConformanceSuite:  "sdk/internal/assembly/p1_p2_conformance_test.go#TestP1P2PortConformanceSuite",
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
	}
}

// P6UIPortEvidence is the build-owned evidence ledger for the full-code Web
// UI Ports completed by P6 Task 7. It is kept separate from P1P2PortEvidence so
// the earlier four-Port conformance gate remains an exact record of its own
// promotion boundary.
func P6UIPortEvidence() map[string]port.SupportEvidence {
	return map[string]port.SupportEvidence{
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

func completedEvidence(ids map[port.EvidenceKind]string) port.SupportEvidence {
	refs := make([]port.EvidenceReference, 0, len(port.RequiredEvidenceKinds()))
	for _, kind := range port.RequiredEvidenceKinds() {
		refs = append(refs, port.EvidenceReference{Kind: kind, ID: ids[kind]})
	}
	return port.SupportEvidence{References: refs, GatesPassed: true}
}
