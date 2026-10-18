# S4 recovery report

## Backend checkpoint (Task 1)

Implements strict `vivy.recap.{read,edit,clear,history,revision,restore,sources}` actions and deferred `actmem_{read,update,clear,history,restore}` providers. All use the actual bound Garden C1 through its existing bundle lifecycle. The private optional management callback reuses the attached control cell. Runtime authorizes the current trusted complete-domain singleton, and model tools additionally require the actual persisted primary resident run/source/tool-policy/grant pins, active process selected-tool/policy snapshot, and durable Journal policy hash. No process-global Recap owner, public binding/receipt/reconcile mutation, caller scope, or independent data store.

Owner full text/history, reconcile cause, cursor, epoch, replay and CAS conflict are preserved. Human mutations never retry a newer base automatically. Strict raw JSON validation rejects invalid UTF-8, unpaired UTF-16 surrogates, duplicate keys and nulls before Go can replace malformed text. Positive Chinese/CRLF/emoji/replacement-character/backslash/slash tests preserve exact text.

The existing assembler only supports one provider collection constructor per Module. Stateless T1 `vivy/recap-tools` provides std/tool@v1 with the existing ToolHost and cognitive-factory dependency. Composition supplies a per-App reference solely to its compiled provider type; IDs alone do not acquire the owner. Default/diva Recipes and default tool selection include it; an omitted module contributes no runtime implementations. Assembly regeneration used `go generate ./internal/generated/assembly`, not hand editing.

Eino check: pinned `github.com/cloudwego/eino/adk/middlewares/dynamictool/toolsearch` v0.9.20 supports only a fixed DynamicTools set. A bounded request adapter delegates its search algorithm and forward selection to the official middleware over currently authorized tools. Final activation disclosure and actual execution recheck authorization. A real HTTP restart test exposed that native `tool_search` previously lacked C2 tool-operation evidence. The adapter now uses the existing coordinator exactly once for retained runs, preserving actual args/full success/error evidence and explicit captured metadata. Verified full machine JSON is unframed for native activation; no C2 seal assertion was relaxed. Ordinary Coding search behavior retains the original native path.

## Evidence

All broad/focused suites used `/tmp/issue58-localonly-runner.py` (credentials removed by name, offline Go dependencies, non-forwarding deny proxy). Logs and metadata are in `logs/` alongside this report; original RED, compile-error, FAIL and diagnostic runs are retained.

- `go test -json ./internal/modules/diva-cognitive -count=1`: 49 tests/subtests and 1 package PASS, no skip/fail (before final native-search adapter increment; module source unchanged semantically afterward).
- Final focused command: `go test -json -p=1 ./internal/modules/diva-cognitive ./internal/runtime ./internal/app ./internal/actionhost ./internal/config -run 'Issue58|Cognitive|GeneratedTool|ToolSearch|DefaultGenerated|EveryContract|StrictInput|ServiceContextStatusAndCompactSession' -count=1 -timeout=8m`: 317 tests/subtests and 5 packages PASS, zero fail, 1 explicit opt-in fixture-capture SKIP. External requests rejected 0 / forwarded 0.
- Actual authenticated control Peer -> ActionHost -> actual Garden owner tests cover multiline write/read/conflict and ordinary/foreign session denial. Actual local HTTP model requests cover deferred disclosure, search activation, invocation, SQLite + owner reopen, second-run activation, and actual failed native-search evidence.
- Raw input normalization RED→GREEN covers action and tool paths. Selected status RED→GREEN proves legacy per-session estimated zero is no longer claimed as unified pressure.
- `git diff --check`: PASS.

## Explicit limits and next work

Task 2 UI remains in progress. Selected ContextStatus now explicitly marks shared measurement unavailable while returning the actual committed Recap revision; selected manual compaction marks token counts unavailable. UI consumers must honor these markers before full S4 acceptance. Sources action returns authorized `recap_sources_unavailable`; there is no parser clone or guessed historical evidence. Actual exact sources and loaded/pinned revision timing await S5. Rowless usage aggregates remain unqualified; no new estimator/store was added. #66 installed producer/sidebar is absent; trusted resolver fixtures prove only the consumer seam. Full runtime/app suites, just ci, SDK conformance/pack and actual split-browser flow remain root/integration gates. PostgreSQL no DSN, Windows and live model quality are not claimed.
