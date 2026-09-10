package assembly

import "agent-vivy/sdk/port"

// P1P2PortEvidence is retained as the compatibility entry point for the
// build-owned support ledger. PLG-P3 extends the completed set with governed
// middleware, observer, and status Ports. Unlisted public Ports remain
// SPECIFIED and cannot be selected by the compiler.
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
		"std/middleware/pre-tool@v1": completedEvidence(map[port.EvidenceKind]string{
			port.EvidencePortDefinition:    "sdk/port/catalog.go#std-middleware-pre-tool-v1",
			port.EvidenceSDKContract:       "sdk/port/pretool/pretool.go#Provider",
			port.EvidenceHostConsumer:      "internal/toolhost/middleware.go#ApplyMiddleware",
			port.EvidenceRealProvider:      "plugins/governance/provider.go#Provider.Evaluate",
			port.EvidenceFailureModel:      "internal/toolhost/middleware_test.go#TestMiddlewarePanicAndInvalidDecisionFailClosed",
			port.EvidenceConformanceSuite:  "internal/runtime/pretool_bridge_test.go#TestToolAdapterRechecksPolicyAfterPublicMiddlewareRewrite",
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
	}
}

func completedEvidence(ids map[port.EvidenceKind]string) port.SupportEvidence {
	refs := make([]port.EvidenceReference, 0, len(port.RequiredEvidenceKinds()))
	for _, kind := range port.RequiredEvidenceKinds() {
		refs = append(refs, port.EvidenceReference{Kind: kind, ID: ids[kind]})
	}
	return port.SupportEvidence{References: refs, GatesPassed: true}
}
