# D01 historical failure mapping

The historical Windows full-product run remains failed and preserved in its
original log. The owner selected wiring/kernel plus affected Face/Web scoped
acceptance; this table maps every historical top-level failure to current
executed evidence or an intentionally removed hash-only assertion.

K = final `just kernel-ci` (all owning runtime/tool cases,1715 passed, one
separately named optional ripgrep skip). F = `just d02-ci` (protocol, shared
Faces/CODE and App shutdown). S = final full SDK execution (1,079 cases).
H = exact historical App/settings/embedded selector (13 cases).
D = D01 causal App/hooks/nudge/recovery selection (17 cases).
All letters refer to exact candidate cc74e7d8ea6474190c8c182d42f51496c4d7fccc.
See verification.md for commands, failures, skips and candidate boundaries.

| Historical test | Current disposition/evidence |
| --- | --- |
| `TestCodeFacePrintModeStreamsAssistantText` | H passed; App shutdown also passes F. |
| `TestCodeFaceJSONModeEmitsOrderedRecords` | H passed; App shutdown also passes F. |
| `TestCodeFacePrintModeCancelsBlockedRunLoudly` | H passed; App shutdown also passes F. |
| `TestPlanGoalIntegratedReviewAndTwoRounds` | H passed; App shutdown also passes F. |
| `TestPlanGoalIntegratedRoundLimitBlocksDurably` | H passed; App shutdown also passes F. |
| `TestAppShutdownStopsGoalAdmissionBeforeChannelDrain` | H passed; App shutdown also passes F. |
| `TestAppShutdownBounded` | H passed; App shutdown also passes F. |
| `TestSaveConcurrentWritersKeepDocumentValid` | H passed; App shutdown also passes F. |
| `TestPollCollectsRunEvents` | H passed; App shutdown also passes F. |
| `TestPollOverflowReportsGap` | H passed; App shutdown also passes F. |
| `TestShellPrefixBashAndCommandline` | K passed; retained owner behavior executed. |
| `TestCommandOutputSpillsToWorkspace` | K passed; retained owner behavior executed. |
| `TestMarketplaceSearchMapsDirectoryRows` | K passed; retained owner behavior executed. |
| `TestMarketplaceSearchClampsLimitAndRejectsShortQuery` | K passed; retained owner behavior executed. |
| `TestMarketplaceInstallWritesLoaderValidSkill` | K passed; retained owner behavior executed. |
| `TestMarketplaceInstallRejectsInvalidSnapshots` | K passed; retained owner behavior executed. |
| `TestMarketplaceUpgradeMirrorsSnapshot` | K passed; retained owner behavior executed. |
| `TestMarketplaceUpgradeUpToDateWritesNothing` | K passed; retained owner behavior executed. |
| `TestMarketplaceUpgradeGuards` | K passed; retained owner behavior executed. |
| `TestMarketplaceCheckUpdateStatuses` | K passed; retained owner behavior executed. |
| `TestMCPBackendStreamableHTTPAndEinoProjection` | K passed; retained owner behavior executed. |
| `TestMCPBackendModernDiscoverAvoidsLegacySession` | K passed; retained owner behavior executed. |
| `TestNetworkSearchServiceNormalizesConfiguredProviders` | K passed; retained owner behavior executed. |
| `TestNetworkSearchServiceDefaultsToSafePublicProvider` | K passed; retained owner behavior executed. |
| `TestNetworkSearchServicePrefersConfiguredProvider` | K passed; retained owner behavior executed. |
| `TestNetworkSearchServiceHonorsPreferredProvider` | K passed; retained owner behavior executed. |
| `TestNetworkSearchServiceRejectsInvalidAndOversizedResponses` | K passed; retained owner behavior executed. |
| `TestNudgeAcceptanceResumeNoStaleNotice` | K + D passed, with retained nudge semantics. |
| `TestDiscoverProjectInstructionsRootAgentsMD` | K passed; retained owner behavior executed. |
| `TestDiscoverProjectInstructionsMissingAgentsMDStillListsDefault` | K passed; retained owner behavior executed. |
| `TestClientCorrelatesResponsesAndStreamsEvents` | S + F passed (real CODE/public protocol paths). |
| `TestClientPropagatesProtocolErrors` | S + F passed (real CODE/public protocol paths). |
| `TestClientAgainstRealVivyCode` | S + F passed (real CODE/public protocol paths). |
| `TestExternalModuleConsumesPublicAPI` | S passed; native dependency checks kept enabled. |
| `TestGenerationFailureMatrixExecutesEveryCase` | S passed; retained structural/behavior assertions, ordinary opaque Generation identity. |
| `TestV1PackAndInspectProveRecipeRemoval` | S passed; retained structural/behavior assertions, ordinary opaque Generation identity. |
| `TestPackAndInspectSealUIAssemblyIdentity` | S passed; retained structural/behavior assertions, ordinary opaque Generation identity. |
| `TestPackBuildsSelectedUIIntoFinalArtifact` | S passed; retained structural/behavior assertions, ordinary opaque Generation identity. |
| `TestPackAndInspectCheckedInFullUIGateRecipe` | S passed; retained structural/behavior assertions, ordinary opaque Generation identity. |
| `TestPackAndInspectEveryShippedRecipe` | S passed; retained structural/behavior assertions, ordinary opaque Generation identity. |
| `TestPackAndInspectSCXCandidateIsDeterministicAndRemovable` | S passed; retained structural/behavior assertions, ordinary opaque Generation identity. |
| `TestInspectRejectsManifestFromAnotherBinary` | S passed; retained structural/behavior assertions, ordinary opaque Generation identity. |
| `TestVerifyEverySelectedPublicModule` | S passed; retained structural/behavior assertions, ordinary opaque Generation identity. |
| `TestPackSelectedToolWorlds` | S passed; retained structural/behavior assertions, ordinary opaque Generation identity. |
| `TestPackLinksExternalStandaloneModule` | S passed; retained structural/behavior assertions, ordinary opaque Generation identity. |
| `TestPackAndInspectSharedTarget` | S passed; retained structural/behavior assertions, ordinary opaque Generation identity. |
| `TestPackGoHostSealedExternalArtifact` | S passed; retained structural/behavior assertions, ordinary opaque Generation identity. |
| `TestInspectGoHostRejectsTamperedArtifact` | S passed; retained structural/behavior assertions, ordinary opaque Generation identity. |
| `TestPackGoHostReproducibleGenerationID` | Intentional contract change: TestPackGoHostAllocatesFreshGenerationID verifies distinct opaque IDs; S. Same explicit authoring ID/inputs regenerate byte-identically. |
| `TestPackGoHostRejectsOutputReuse` | S passed; retained structural/behavior assertions, ordinary opaque Generation identity. |
| `TestPackGoHostResolvesUnpinnedHostIdentity` | S passed; retained structural/behavior assertions, ordinary opaque Generation identity. |
| `TestMinimalArtifactPhysicallyOmitsOptionalModules` | S passed; retained structural/behavior assertions, ordinary opaque Generation identity. |
| `TestSCXCandidateRollbackRestoresPriorSealedGenerationWithoutJournalMutation` | S passed; retained structural/behavior assertions, ordinary opaque Generation identity. |
| `TestHashSourceTreeRejectsSymbolicLinks` | Retired with source-hash walks; no hash verifier or synthetic passing test retained. Explicit source/ref/root/Trust and native build integrity remain. |
| `TestCheckedInProviderConformanceMatchesExecutedSuites` | S strict executed23-Provider reproduction passed; no source-hash binding or evidence update flag. |
