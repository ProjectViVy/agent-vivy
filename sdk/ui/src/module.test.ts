import { describe, expect, it } from "vitest";
import {
  UI_SDK_PACKAGE_NAME,
  UI_SDK_PROVENANCE,
  UI_SDK_VERSION,
} from "./index";
import {
  cleanupHandleFromInstall,
  composeUI,
  defineUIExtension,
  type FullUIHost,
  type FaceNavigationOptions,
  type FaceStoreState,
  type UICompositionInput,
  type UIExtension,
  type UIProvider,
  type UIExtensionProvider,
  type UIRootProvider,
  type UIRoot,
  UI_EXTENSION_PORT,
  UI_ROOT_PORT,
} from "./module";

function root(id: string): UIRoot {
  return { id, render: () => null };
}

function extension(id: string, relations: Partial<UIExtension> = {}): UIExtension {
  return { id, install: () => undefined, ...relations };
}

const unavailableFaceOperation = async () => {
  throw new Error("fixture operation not invoked");
};

const emptyFaceStoreState: FaceStoreState = {
  initialized: true,
  initializationError: null,
  capabilities: [],
  connection: "connected",
  sessions: [],
  sessionsPhase: "empty",
  sessionsError: null,
  sessionBusyId: null,
  activeSessionId: null,
  messages: [],
  messagesPhase: "empty",
  messagesError: null,
  sessionContext: null,
  todos: [],
  todosPhase: "empty",
  todosError: null,
  todoPanelOpen: false,
  currentRun: null,
  runEvents: [],
  streamingText: "",
  streamingReasoning: "",
  runError: null,
  runBusy: false,
  queuedMessages: [],
  backgroundRuns: [],
  backgroundPhase: "empty",
  backgroundError: null,
  backgroundBusyId: null,
  children: [],
  childrenPhase: "empty",
  childrenError: null,
  childBusyId: null,
  selectedChild: null,
  reviews: [],
  reviewsPhase: "empty",
  reviewsError: null,
  reviewBusyIds: [],
  reviewCenterOpen: false,
  filesPanelOpen: false,
  sessionDrawerOpen: false,
  settings: null,
  settingsPhase: "idle",
  settingsError: null,
  providers: [],
  providersPhase: "empty",
  providersError: null,
  species: null,
  generations: [],
  evals: [],
  promotions: [],
  lifecyclePhase: "idle",
  lifecycleError: null,
  lifecycleBusy: false,
  initialize: unavailableFaceOperation,
  retryInitialize: unavailableFaceOperation,
  loadSessions: unavailableFaceOperation,
  createSession: unavailableFaceOperation,
  renameSession: unavailableFaceOperation,
  setSessionPermission: unavailableFaceOperation,
  deleteSession: unavailableFaceOperation,
  selectSession: unavailableFaceOperation,
  startRun: unavailableFaceOperation,
  editSession: unavailableFaceOperation,
  enqueueMessage: () => undefined,
  removeQueuedMessage: () => undefined,
  clearQueue: () => undefined,
  cancelCurrentRun: unavailableFaceOperation,
  openRun: unavailableFaceOperation,
  loadBackgroundRuns: unavailableFaceOperation,
  attachBackgroundRun: unavailableFaceOperation,
  loadChildren: unavailableFaceOperation,
  startChild: unavailableFaceOperation,
  openChild: unavailableFaceOperation,
  waitChild: unavailableFaceOperation,
  cancelChild: unavailableFaceOperation,
  loadReviews: unavailableFaceOperation,
  respondReview: unavailableFaceOperation,
  setReviewCenterOpen: () => undefined,
  setFilesPanelOpen: () => undefined,
  setSessionDrawerOpen: () => undefined,
  loadTodos: unavailableFaceOperation,
  updateTodoStatus: unavailableFaceOperation,
  setTodoPanelOpen: () => undefined,
  loadSettings: unavailableFaceOperation,
  saveSettings: unavailableFaceOperation,
  saveLocale: unavailableFaceOperation,
  loadSessionContext: unavailableFaceOperation,
  compactSession: unavailableFaceOperation,
  rewindSession: unavailableFaceOperation,
  forkSession: unavailableFaceOperation,
  loadProviders: unavailableFaceOperation,
  saveProvider: unavailableFaceOperation,
  removeProvider: unavailableFaceOperation,
  refreshProvider: unavailableFaceOperation,
  loadLifecycle: unavailableFaceOperation,
  createGeneration: unavailableFaceOperation,
  rejectGeneration: unavailableFaceOperation,
  startEval: unavailableFaceOperation,
  recordEval: unavailableFaceOperation,
  promoteGeneration: unavailableFaceOperation,
};

describe("full-code UI composition", () => {
  it("rejects duplicate root providers", () => {
    const input: UICompositionInput = {
      roots: [root("vivy/default-ui"), root("example/search-ui")],
      extensions: [],
    };

    expect(() => composeUI(input)).toThrow("duplicate root provider");
  });

  it("rejects missing provider IDs", () => {
    expect(() => composeUI({ extensions: [extension(" ")] })).toThrow("id is required");
  });

  it("rejects duplicate extension IDs", () => {
    expect(() => composeUI({ extensions: [extension("example/search"), extension("example/search")] }))
      .toThrow("duplicate UI provider id example/search");
  });

  it("normalizes explicit root and extension provider envelopes", () => {
    const selectedRoot: UIRootProvider = {
      root: root("vivy/default-ui"),
      port: UI_ROOT_PORT,
    };
    const selectedExtension: UIExtensionProvider = {
      extension: extension("example/search"),
      port: UI_EXTENSION_PORT,
    };

    const result = composeUI({ rootProviders: [selectedRoot], extensionProviders: [selectedExtension] });

    expect(result.root?.id).toBe("vivy/default-ui");
    expect(result.extensions.map(({ id }) => id)).toEqual(["example/search"]);
    expect(result.rootProviders[0]).toMatchObject(selectedRoot);
    expect(result.extensionProviders[0]).toMatchObject(selectedExtension);
  });

  it("requires explicit Ports on provider envelopes and canonicalizes direct inputs", () => {
    const direct = composeUI({
      root: root("vivy/default-ui"),
      extensions: [extension("example/direct")],
    });

    expect(direct.rootProviders[0]?.port).toBe(UI_ROOT_PORT);
    expect(direct.extensionProviders[0]?.port).toBe(UI_EXTENSION_PORT);

    const missingExtensionPort = {
      provider: extension("example/missing-port"),
    } as never;
    expect(() => composeUI({ extensions: [missingExtensionPort] }))
      .toThrow("must declare std/ui-extension@v1");

    // @ts-expect-error generated extension envelopes must declare their Port.
    const missingExtensionPortType: UIExtensionProvider = { extension: extension("example/missing-port-type") };
    // @ts-expect-error generated root envelopes must declare their Port.
    const missingRootPortType: UIRootProvider = { root: root("example/missing-root-port-type") };
    expect(missingExtensionPortType).toBeDefined();
    expect(missingRootPortType).toBeDefined();
  });

  it.each([
    ["before", { before: "missing" }],
    ["after", { after: "missing" }],
    ["replaces", { replaces: "missing" }],
  ] as const)("rejects unresolved %s references", (relation, metadata) => {
    expect(() => composeUI({ extensions: [extension("example/search", metadata)] }))
      .toThrow(`unresolved ${relation} reference missing`);
  });

  it("orders extensions by explicit before and after relationships", () => {
    const result = composeUI({ extensions: [
      extension("example/last", { after: "example/first" }),
      extension("example/first"),
      extension("example/middle", { before: "example/last", after: "example/first" }),
    ] });

    expect(result.extensions.map(({ id }) => id)).toEqual([
      "example/first",
      "example/middle",
      "example/last",
    ]);
  });

  it("rejects an install result that is neither void nor a cleanup callback", () => {
    const invalidCleanup = { dispose: () => undefined } as never;
    expect(() => cleanupHandleFromInstall(invalidCleanup)).toThrow("cleanup must be a function");

    // @ts-expect-error cleanup handles are callbacks, not arbitrary objects.
    if (false) cleanupHandleFromInstall({ dispose: () => undefined });
  });

  it("provides a type-safe extension authoring helper", () => {
    const valid = defineUIExtension({ id: "example/valid", install: () => undefined });
    expect(valid.id).toBe("example/valid");

    // @ts-expect-error install must return void or a cleanup callback.
    defineUIExtension({ id: "example/invalid", install: () => ({ dispose: () => undefined }) });
    // @ts-expect-error every UI provider requires a stable id.
    defineUIExtension({ install: () => undefined });
  });

  it("keeps malformed provider shapes out of the public type", () => {
    // @ts-expect-error provider IDs are mandatory.
    const missingID: UIExtension = { install: () => undefined };
    // @ts-expect-error cleanup handles are callbacks, not arbitrary objects.
    const invalidCleanup: UIExtension = { id: "example/invalid", install: () => ({ dispose: () => undefined }) };
    expect(missingID).toBeDefined();
    expect(invalidCleanup).toBeDefined();
  });

  it("makes cleanup handles explicit and idempotent", () => {
    let calls = 0;
    const handle = cleanupHandleFromInstall(() => { calls += 1; });
    expect(handle.active).toBe(true);
    handle.dispose();
    handle.dispose();
    expect(calls).toBe(1);
    expect(handle.active).toBe(false);
  });

  it("exposes client and composition capabilities without a permission request", () => {
    type HostKeys = keyof FullUIHost;
    type HasPermissionRequest = "requestPermission" extends HostKeys ? true : false;
    const noPermissionRequest: false = false as HasPermissionRequest;
    expect(noPermissionRequest).toBe(false);

    type APIKeys = keyof import("./module").FaceClientAPI;
    type HasDynamicAPIKeys = string extends APIKeys ? true : false;
    const noDynamicAPIKeys: false = false as HasDynamicAPIKeys;
    expect(noDynamicAPIKeys).toBe(false);
  });

  it("exposes the current RPC seam as a typed host reference", () => {
    const host: FullUIHost = {
      api: {
        request: async <T>() => undefined as T,
        settingsUpdateFrom: () => ({ provider: "fixture", default_model: "fixture", base_url: "" }),
        initialize: async () => ({ protocol_version: "vivy/rpc-v1", capabilities: [] }),
        listWorkspaceFiles: unavailableFaceOperation,
        readWorkspaceFile: unavailableFaceOperation,
        listSessions: async () => ({ sessions: [] }),
        getSession: async () => ({
          session: { id: "fixture/session", title: "Fixture", created_at: 0 },
          messages: [],
        }),
        createSession: async () => ({ id: "fixture/session", title: "Fixture", created_at: 0 }),
        renameSession: async () => ({ id: "fixture/session", title: "Renamed", created_at: 0 }),
        setSessionPermission: unavailableFaceOperation,
        deleteSession: unavailableFaceOperation,
        listMessages: async () => ({ messages: [] }),
        getSessionContext: unavailableFaceOperation,
        compactSession: unavailableFaceOperation,
        rewindSession: unavailableFaceOperation,
        forkSession: unavailableFaceOperation,
        editSession: unavailableFaceOperation,
        listSessionCompactions: unavailableFaceOperation,
        listTodos: unavailableFaceOperation,
        updateTodo: unavailableFaceOperation,
        startTurn: unavailableFaceOperation,
        interruptRun: unavailableFaceOperation,
        cancelRun: unavailableFaceOperation,
        getRun: async () => ({ id: "fixture/run", session_id: "fixture/session", status: "completed" as const, created_at: 0 }),
        getRunLog: async () => ({ events: [] }),
        recoverBackgroundRuns: unavailableFaceOperation,
        listBackgroundRuns: unavailableFaceOperation,
        attachBackgroundRun: unavailableFaceOperation,
        startChild: unavailableFaceOperation,
        getChild: unavailableFaceOperation,
        listChildren: unavailableFaceOperation,
        waitChild: unavailableFaceOperation,
        cancelChild: unavailableFaceOperation,
        listApprovals: unavailableFaceOperation,
        respondApproval: unavailableFaceOperation,
        listQuestions: unavailableFaceOperation,
        respondQuestion: unavailableFaceOperation,
        listReviews: unavailableFaceOperation,
        getReview: unavailableFaceOperation,
        respondReview: unavailableFaceOperation,
        inspectSpecies: unavailableFaceOperation,
        listGenerations: unavailableFaceOperation,
        getGeneration: unavailableFaceOperation,
        createGeneration: unavailableFaceOperation,
        rejectGeneration: unavailableFaceOperation,
        listEvals: unavailableFaceOperation,
        recordEval: unavailableFaceOperation,
        startEval: unavailableFaceOperation,
        listPromotions: unavailableFaceOperation,
        promoteGeneration: unavailableFaceOperation,
        getSettings: async () => ({
          locale: "en" as const,
          generation_locale: "en" as const,
          workspace_locale: "" as const,
          locale_read_only: false,
          provider: "fixture",
          default_model: "fixture",
          base_url: "",
          read_only: false,
          config_provider: "fixture",
          config_model: "fixture",
        }),
        updateLocale: async () => ({ locale: "en" as const, generation_locale: "en" as const, workspace_locale: "" as const, locale_read_only: false }),
        listTools: unavailableFaceOperation,
        setActiveTools: unavailableFaceOperation,
        updateSettings: unavailableFaceOperation,
        listProviders: unavailableFaceOperation,
        upsertProvider: unavailableFaceOperation,
        deleteProvider: unavailableFaceOperation,
        refreshProviderModels: unavailableFaceOperation,
        listMcpServers: unavailableFaceOperation,
        upsertMcpServer: unavailableFaceOperation,
        deleteMcpServer: unavailableFaceOperation,
        probeMcpServer: unavailableFaceOperation,
        inspectChannels: unavailableFaceOperation,
        getChannel: unavailableFaceOperation,
        updateChannel: unavailableFaceOperation,
        getTokenUsage: unavailableFaceOperation,
        listSkills: unavailableFaceOperation,
        getSkill: unavailableFaceOperation,
        setSkillEnabled: unavailableFaceOperation,
        searchMarketplaceSkills: unavailableFaceOperation,
        featuredMarketplaceSkills: unavailableFaceOperation,
        installMarketplaceSkill: unavailableFaceOperation,
        checkMarketplaceUpdate: unavailableFaceOperation,
        listCronJobs: unavailableFaceOperation,
        createCronJob: unavailableFaceOperation,
        updateCronJob: unavailableFaceOperation,
        deleteCronJob: unavailableFaceOperation,
        triggerCronJob: unavailableFaceOperation,
        stopCronJob: unavailableFaceOperation,
        fetchSessionTrajectory: unavailableFaceOperation,
      },
      rpc: {
        capabilities: { protocol_version: "vivy/rpc-v1", capabilities: [] },
        call: async <T>() => undefined as T,
        onNotification: () => () => undefined,
        onClose: () => () => undefined,
        close: () => undefined,
      },
      store: {
        getState: () => emptyFaceStoreState,
        getInitialState: () => emptyFaceStoreState,
        setState: () => undefined,
        subscribe: (listener) => {
          listener(emptyFaceStoreState, emptyFaceStoreState);
          return () => undefined;
        },
      },
      router: {
        navigate: async () => undefined,
        invalidate: async () => undefined,
      },
      composition: {
        routes: { register: () => ({ id: "route", active: true, dispose: () => undefined }), unregister: () => undefined },
        navigation: { register: () => ({ id: "nav", active: true, dispose: () => undefined }), unregister: () => undefined },
        pages: { register: () => ({ id: "page", active: true, dispose: () => undefined }), unregister: () => undefined },
        components: { register: () => ({ id: "component", active: true, dispose: () => undefined }), unregister: () => undefined },
        styles: { register: () => ({ id: "style", active: true, dispose: () => undefined }), unregister: () => undefined },
        themes: { register: () => ({ id: "theme", active: true, dispose: () => undefined }), unregister: () => undefined },
        shortcuts: { register: () => ({ id: "shortcut", active: true, dispose: () => undefined }), unregister: () => undefined },
        commands: { register: () => ({ id: "command", active: true, dispose: () => undefined }), unregister: () => undefined },
        registerCleanup: () => cleanupHandleFromInstall(undefined),
      },
      t: (key) => key,
      registerCleanup: () => cleanupHandleFromInstall(undefined),
    };

    expect(host.rpc.capabilities.protocol_version).toBe("vivy/rpc-v1");
    expect(host.composition.routes.register("fixture/route", { render: () => null }).id).toBe("route");
    expect(host.store.getState().sessions).toEqual([]);
    expect(host.store.getInitialState().sessions).toEqual([]);
  });

  it("accepts current route search and path parameters", () => {
    const options: FaceNavigationOptions = {
      to: "/settings",
      from: "/",
      params: { sessionId: "fixture" },
      search: { tab: "model" },
      hash: "models",
      replace: true,
    };
    const router: FullUIHost["router"] = {
      navigate: async (received) => {
        expect(received).toEqual(options);
      },
      invalidate: async () => undefined,
    };

    return router.navigate(options);
  });

  it("keeps the localization surface key-and-arguments based", () => {
    const currentFaceTranslator = (key: string, args?: Record<string, string | number>) =>
      `${key}:${String(args?.name ?? "")}`;
    const compatibleTranslator: FullUIHost["t"] = currentFaceTranslator;
    const translator: FullUIHost["t"] = (key, args, form = "") =>
      `${form}:${key}:${String(args?.name ?? "")}`;

    expect(compatibleTranslator("vivy.loading", { name: "Vivy" }))
      .toBe("vivy.loading:Vivy");
    expect(translator("plugin.example/greeting.title", { name: "Vivy" }, "short"))
      .toBe("short:plugin.example/greeting.title:Vivy");
    expect(translator("vivy.loading", undefined, "long"))
      .toBe("long:vivy.loading:");
  });

  it("publishes one deterministic package identity for build provenance", () => {
    expect(UI_SDK_PROVENANCE).toEqual({
      packageName: UI_SDK_PACKAGE_NAME,
      version: UI_SDK_VERSION,
    });
    expect(UI_SDK_PACKAGE_NAME).toBe("@vivy/ui-sdk");
    expect(UI_SDK_VERSION).toBe("1.0.0");
  });

  it("ties provider envelopes to their declared UI Port", () => {
    const validExtension: UIProvider<UIExtension> = {
      provider: extension("example/port-bound"),
      port: UI_EXTENSION_PORT,
    };
    expect(validExtension.port).toBe(UI_EXTENSION_PORT);

    const invalidExtension: UIProvider<UIExtension> = {
      provider: extension("example/wrong-port"),
      // @ts-expect-error an extension provider cannot claim the exclusive root Port.
      port: UI_ROOT_PORT,
    };
    expect(invalidExtension).toBeDefined();
  });

  it("rejects a provider envelope that claims the wrong UI Port at runtime", () => {
    const malformed = {
      provider: extension("example/wrong-port-runtime"),
      port: UI_ROOT_PORT,
    } as never;
    expect(() => composeUI({ extensions: [malformed] }))
      .toThrow("std/ui-extension@v1");
  });

  it("rejects provider envelopes whose behavior does not match their Port", () => {
    const rootAsExtension = {
      provider: root("example/root-as-extension"),
      port: UI_EXTENSION_PORT,
    } as never;
    expect(() => composeUI({ extensions: [rootAsExtension] }))
      .toThrow("install");

    const extensionAsRoot = {
      provider: extension("example/extension-as-root"),
      port: UI_ROOT_PORT,
    } as never;
    expect(() => composeUI({ roots: [extensionAsRoot] }))
      .toThrow("render");
  });

  it("exposes a usable typed store subscription", () => {
    type State = { sessions: readonly string[]; activeSessionId: string | null };
    const store: import("./module").FaceClientStore<State> = {
      getState: () => ({ sessions: [], activeSessionId: null }),
      getInitialState: () => ({ sessions: [], activeSessionId: null }),
      setState: (partial) => {
        if (typeof partial === "function") partial({ sessions: [], activeSessionId: null });
      },
      subscribe: (listener) => {
        listener({ sessions: [], activeSessionId: null }, { sessions: [], activeSessionId: null });
        return () => undefined;
      },
    };

    let observed: string | null = null;
    store.subscribe((state, previousState) => {
      observed = `${state.sessions.length}:${previousState.activeSessionId ?? "none"}`;
    });
    expect(observed).toBe("0:none");
  });
});
