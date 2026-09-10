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

func completedEvidence(ids map[port.EvidenceKind]string) port.SupportEvidence {
	refs := make([]port.EvidenceReference, 0, len(port.RequiredEvidenceKinds()))
	for _, kind := range port.RequiredEvidenceKinds() {
		refs = append(refs, port.EvidenceReference{Kind: kind, ID: ids[kind]})
	}
	return port.SupportEvidence{References: refs, GatesPassed: true}
}
