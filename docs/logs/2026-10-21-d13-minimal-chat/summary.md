# Usable minimal and selected browser chat

D13 completes the owner-authorized stable runtime sequence after D06/D08
integration `30af694d`, D04 `d098d4e2` and D05 `01be84a6`. The sole Story
status remains the issue-91 index.

Minimal now selects the existing protected-tools and provider-profiles owners
alongside its seven core owners. It can complete a real protected file read
through the packed production gateway, durable tool operation and next model
request. Optional Hosts and UI remain absent; the minimal kernel gate still
installs/builds no frontend. Default and lite Recipes are unchanged.

The new T1 `vivy/minimal-chat` source (`repo:faces/minimal-chat`) contributes
exclusive `std/ui-root@v1` Provider `vivy.minimal-chat.root` to PresentationHost.
Its eleven-module Recipe adds only that Host and UI root to usable minimal.
It consumes FullUIHost/FaceClientStore for messages, tool output, workspace
binding, admission, cancellation, errors and terminal state. It adds no Face
runner, transport, listener, backend result cache or execution authority.
The generation-scoped Go owner has no backend worker or requested Grant.
English and Chinese catalogs are sealed with the selected UI source.

The UI uses existing semantic tokens, an independently scrollable transcript,
accessible controls and stable test IDs. Empty assistant tool-call text is
omitted; existing timestamps place tool rows before the final answer. Finished
tools retain both result and failure, including empty successful completion.
Draft editing is locked only while admission is pending, so a delayed response
cannot erase another input. Failed admission preserves the submitted draft.
Workspace selection uses the existing chooseWorkspace action before the first
message, including the host's automatically created empty Session.

Runnable minimal, minimal-chat and minimal-plus-hello-fs examples use fresh
outputs and disposable homes. The small bind-workspace command calls existing
tui.Dial/session-create only, validates the returned workspace and identifies
the Session for the existing live terminal. It starts no Run. The hello-fs
example explicitly selects its ToolWorld/fs.read Grant and enables hello_stat
in the normal configuration. The actual helper and code-face submissions are
exercised against packed minimal and hello-fs gateways.

The SDK's single repository source table owns the UI Module entry. No generated
Assembly is edited. headless-compile now uses the shared PATH-selected Go
command in both shell families. The restored environment skill's baseline
wording follows the nine-module minimal and keeps browser acceptance separate.

Eino capability check: repository pin v0.9.21 already supplies the typed ADK
agent/runner, ToolsNode and WrapModel used by accepted predecessors. D13 adds
composition and presentation only; no Eino adapter, import or dependency change
is needed. The Service.Run/Journal/Policy/tool-operation path remains sole owner.

No schema, migration, default/lite feature selection, provider credential path
or core nudge/warming/steering behavior changed. #93 owns their eventual package
extraction. Full-product CI, native Windows package acceptance, real-token human
proof, final finite upgrade publication and #91 closure remain D14 work. No push.
