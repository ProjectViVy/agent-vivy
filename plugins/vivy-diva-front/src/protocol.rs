//! The wire contract. Exported TypeScript and examples are derived here.
use serde::{Deserialize, Serialize};
use ts_rs::TS;

pub const VERSION: u32 = 1;
pub const MAX_FRAME: usize = 1024 * 1024;

#[derive(Debug, Serialize, Deserialize, TS)]
#[serde(deny_unknown_fields)]
pub struct Request {
    pub v: u32,
    pub id: String,
    #[serde(flatten)]
    pub method: Method,
}

#[derive(Debug, Serialize, Deserialize, TS)]
#[serde(tag = "method", content = "params", rename_all = "snake_case")]
pub enum Method {
    Snapshot,
    Shutdown,
    Submit(SubmitParams),
    Admission(SubmitParams),
    Cancel(CancelParams),
    Answer(AnswerParams),
    History(HistoryParams),
    ProvidersRead,
    ProvidersReload,
    ProvidersSave(ProvidersSave),
    ProviderKey(ProviderKey),
    ProviderTest(ProviderTest),
    PersonaRead,
    PersonaSaveFile(PersonaSaveFile),
    PersonaRollback(PersonaRollback),
    PromptsRead,
    PromptSave(PromptSave),
    ManagedRead,
    ManagedOpen(ManagedId),
    ManagedImport(ManagedImport),
    ManagedEdit(ManagedEdit),
    ManagedValidate(ManagedValidate),
    ManagedCancel(ManagedId),
    ManagedDependencies(ManagedId),
    ManagedPublish(ManagedPublish),
    ManagedRollback(ManagedRollback),
    TasksRead,
    WakeCancel(WakeCancel),
    WakeSchedule(WakeSchedule),
    Resolve(Resolve),
    DreamRequest(DreamRequest),
    DreamCancel(DreamCancel),
    DreamDetach(DreamDetach),
    DreamJobCancel(DreamDetach),
    McpRead,
    McpSave(McpSave),
    McpControl(McpControl),
    PythonRead,
    PythonPrepare(PythonPrepare),
    PythonCancel,
    SkillsRead,
    SkillRead(SkillName),
    SkillsRefresh,
    SkillImport(SkillImport),
    SkillSave(SkillSave),
    SkillRemove(SkillRemove),
    RecapRead,
    RecapRefresh,
}

#[derive(Debug, Serialize, Deserialize, TS)]
#[serde(deny_unknown_fields)]
pub struct ProviderTest {
    #[ts(type = "number")]
    pub generation: u64,
    pub reference: String,
}
#[derive(Debug, Clone, Serialize, Deserialize, TS)]
pub struct ProbeCheck {
    pub kind: String,
    pub passed: bool,
    pub detail: String,
}
#[derive(Debug, Serialize, Deserialize, TS)]
pub struct ProbeReport {
    pub reference: String,
    pub checks: Vec<ProbeCheck>,
}
#[derive(Debug, Serialize, Deserialize, TS)]
#[serde(deny_unknown_fields)]
pub struct ManagedImport {
    pub base: String,
    pub name: String,
    pub source: String,
}
#[derive(Debug, Serialize, Deserialize, TS)]
#[serde(deny_unknown_fields)]
pub struct ManagedEdit {
    pub id: String,
    pub files: std::collections::BTreeMap<String, String>,
}
#[derive(Debug, Serialize, Deserialize, TS)]
#[serde(deny_unknown_fields)]
pub struct ManagedValidate {
    pub id: String,
    pub checks: String,
}
#[derive(Debug, Serialize, Deserialize, TS)]
#[serde(deny_unknown_fields)]
pub struct ManagedId {
    pub id: String,
}
#[derive(Debug, Serialize, Deserialize, TS)]
#[serde(deny_unknown_fields)]
pub struct ManagedPublish {
    pub id: String,
    pub update_id: String,
}
#[derive(Debug, Serialize, Deserialize, TS)]
#[serde(deny_unknown_fields)]
pub struct ManagedRollback {
    pub base: String,
    pub target: String,
    pub update_id: String,
}
#[derive(Debug, Serialize, Deserialize, TS)]
#[serde(deny_unknown_fields)]
pub struct WakeCancel {
    pub id: String,
    #[ts(type = "number")]
    pub generation: u64,
}
#[derive(Debug, Serialize, Deserialize, TS)]
#[serde(deny_unknown_fields)]
pub struct WakeSchedule {
    pub task: String,
    #[ts(type = "number")]
    pub generation: u64,
    #[ts(type = "number | null")]
    pub previous: Option<u64>,
    #[ts(type = "number")]
    pub deadline_ms: u64,
    pub reason: String,
}
#[derive(Debug, Serialize, Deserialize, TS)]
#[serde(deny_unknown_fields)]
pub struct Resolve {
    pub operation: String,
    #[ts(type = "number")]
    pub generation: u64,
    pub evidence: String,
}
#[derive(Debug, Serialize, Deserialize, TS)]
#[serde(deny_unknown_fields)]
pub struct DreamRequest {
    pub request_id: String,
    pub reason: String,
}
#[derive(Debug, Serialize, Deserialize, TS)]
#[serde(deny_unknown_fields)]
pub struct DreamCancel {
    #[ts(type = "number")]
    pub revision: u64,
    #[ts(type = "number | null")]
    pub dream_revision: Option<u64>,
}
#[derive(Debug, Serialize, Deserialize, TS)]
#[serde(deny_unknown_fields)]
pub struct DreamDetach {
    pub job: String,
    #[ts(type = "number")]
    pub revision: u64,
}
#[derive(Debug, Serialize, Deserialize, TS)]
pub struct WakeView {
    pub id: String,
    pub task: String,
    #[ts(type = "number")]
    pub generation: u64,
    #[ts(type = "number")]
    pub task_generation: u64,
    #[ts(type = "number")]
    pub deadline_ms: u64,
    pub reason: String,
    pub phase: String,
    pub delivered: bool,
}
#[derive(Debug, Serialize, Deserialize, TS)]
pub struct DreamRequestView {
    pub id: String,
    pub task: Option<String>,
    pub reasons: Vec<String>,
    pub finished: bool,
}
#[derive(Debug, Serialize, Deserialize, TS)]
pub struct DreamJobView {
    pub id: String,
    pub task: String,
    pub target: String,
    pub phase: String,
    pub result: Option<String>,
    pub error: Option<String>,
    pub skipped: bool,
}
#[derive(Debug, Serialize, Deserialize, TS)]
pub struct TasksView {
    #[ts(type = "number")]
    pub revision: u64,
    #[ts(type = "number | null")]
    pub dream_revision: Option<u64>,
    pub wakes: Vec<WakeView>,
    pub dreams: Vec<DreamRequestView>,
    pub jobs: Vec<DreamJobView>,
}
#[derive(Debug, Serialize, Deserialize, TS)]
#[serde(deny_unknown_fields)]
pub struct McpSave {
    pub revision: String,
    pub connections: std::collections::BTreeMap<String, vivy_python::control::McpConnection>,
}
#[derive(Debug, Serialize, Deserialize, TS)]
#[serde(rename_all = "snake_case")]
pub enum McpAction {
    Inspect,
    Reconnect,
    Close,
}
#[derive(Debug, Serialize, Deserialize, TS)]
#[serde(deny_unknown_fields)]
pub struct McpControl {
    pub server: String,
    pub action: McpAction,
}
#[derive(Debug, Serialize, Deserialize, TS)]
pub struct McpResult {
    pub server: String,
    pub state: String,
    pub tools: Vec<McpTool>,
}
#[derive(Debug, Serialize, Deserialize, TS)]
pub struct McpTool {
    pub name: String,
    pub description: String,
    pub schema: String,
}
#[derive(Debug, Serialize, Deserialize, TS)]
#[serde(deny_unknown_fields)]
pub struct PythonPrepare {
    pub managed: bool,
}
#[derive(Debug, Serialize, Deserialize, TS)]
pub struct PythonReport {
    pub outcome: String,
    pub phase: Option<String>,
    pub detail: String,
    pub scope: String,
}
#[derive(Debug, Serialize, Deserialize, TS)]
#[serde(deny_unknown_fields)]
pub struct SkillImport {
    pub source: String,
    pub name: String,
}
#[derive(Debug, Serialize, Deserialize, TS)]
#[serde(deny_unknown_fields)]
pub struct SkillSave {
    pub name: String,
    pub revision: String,
    pub text: String,
}
#[derive(Debug, Serialize, Deserialize, TS)]
#[serde(deny_unknown_fields)]
pub struct SkillRemove {
    pub name: String,
    pub revision: String,
}
#[derive(Debug, Serialize, Deserialize, TS)]
pub struct SkillDocument {
    pub name: String,
    pub description: String,
    pub text: String,
    pub revision: String,
    pub editable: bool,
}
#[derive(Debug, Serialize, Deserialize, TS)]
#[serde(deny_unknown_fields)]
pub struct SkillName {
    pub name: String,
}
#[derive(Debug, Serialize, Deserialize, TS)]
#[serde(deny_unknown_fields)]
pub struct PersonaSaveFile {
    #[ts(type = "number")]
    pub revision: u64,
    pub expected_update_id: String,
    pub update_id: String,
    pub name: String,
    pub text: String,
}
#[derive(Debug, Serialize, Deserialize, TS)]
#[serde(deny_unknown_fields)]
pub struct PersonaRollback {
    #[ts(type = "number")]
    pub revision: u64,
    #[ts(type = "number")]
    pub target: u64,
    pub update_id: String,
}
#[derive(Debug, Serialize, Deserialize, TS)]
#[serde(deny_unknown_fields)]
pub struct PromptSave {
    pub name: String,
    pub revision: String,
    pub text: String,
}
#[derive(Debug, Serialize, Deserialize, TS)]
pub struct PersonaFileView {
    pub name: String,
    pub text: String,
    pub required: bool,
    pub auto: bool,
}
#[derive(Debug, Serialize, Deserialize, TS)]
pub struct PersonaView {
    #[ts(type = "number")]
    pub revision: u64,
    pub update_id: String,
    pub directory: String,
    pub text: String,
    pub files: Vec<PersonaFileView>,
    pub unpublished: bool,
}
#[derive(Debug, Serialize, Deserialize, TS)]
pub struct PersonaVersionView {
    #[ts(type = "number")]
    pub revision: u64,
    pub update_id: String,
    pub text: String,
    pub sources: Vec<String>,
    pub truncated: bool,
}
#[derive(Debug, Serialize, Deserialize, TS)]
pub struct PromptView {
    pub name: String,
    pub source: String,
    pub revision: String,
    pub text: String,
}
#[derive(Debug, Serialize, Deserialize, TS)]
#[serde(tag = "kind", content = "data", rename_all = "snake_case")]
pub enum ManagementResult {
    ManagedCandidate(vivy_python::managed::desktop::CandidateView),
    Skill(SkillDocument),
    ProviderTest(ProbeReport),
    Managed(vivy_python::managed::desktop::ManagedView),
    Mcp(vivy_python::control::McpView),
    McpResult(McpResult),
    Python(PythonReport),
    Skills {
        skills: Vec<SkillDocument>,
    },
    Persona {
        current: PersonaView,
        history: Vec<PersonaVersionView>,
    },
    Prompts {
        prompts: Vec<PromptView>,
    },
}
#[derive(Debug, Serialize, Deserialize, TS)]
pub struct ManagementCompleted {
    pub operation: String,
    pub result: Option<ManagementResult>,
    pub error: Option<Fault>,
}
#[derive(Debug, Serialize, Deserialize, TS)]
pub struct RecapView {
    #[ts(type = "number")]
    pub revision: u64,
    pub summary: String,
    pub covered: std::collections::BTreeMap<String, usize>,
    pub jobs: Vec<RecapJob>,
    pub refresh_pending: bool,
    pub error: Option<String>,
}
#[derive(Debug, Serialize, Deserialize, TS)]
pub struct RecapJob {
    pub id: String,
    pub phase: String,
    pub spans: usize,
}

#[derive(Debug, Serialize, Deserialize, TS)]
#[serde(deny_unknown_fields)]
pub struct SubmitParams {
    pub request_id: String,
    pub text: String,
}
#[derive(Debug, Serialize, Deserialize, TS)]
#[serde(deny_unknown_fields)]
pub struct CancelParams {
    pub task: String,
    #[ts(type = "number")]
    pub generation: u64,
}
#[derive(Debug, Serialize, Deserialize, TS)]
#[serde(deny_unknown_fields)]
pub struct AnswerParams {
    pub instance: String,
    pub question: String,
    pub choices: Vec<String>,
    pub text: Option<String>,
}
#[derive(Debug, Serialize, Deserialize, TS)]
#[serde(deny_unknown_fields)]
pub struct HistoryParams {
    pub cursor: Option<String>,
    pub limit: u32,
}

#[derive(Debug, Serialize, Deserialize, TS)]
pub struct AdmissionView {
    pub request_id: String,
    pub task: Option<String>,
    pub phase: AdmissionPhase,
}
#[derive(Debug, Serialize, Deserialize, TS)]
#[serde(rename_all = "snake_case")]
pub enum AdmissionPhase {
    Missing,
    Pending,
    Committed,
}
#[derive(Debug, Serialize, Deserialize, TS)]
pub struct PendingOperation {
    pub operation: String,
    pub committed: bool,
}
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize, TS)]
pub struct TaskInfo {
    pub id: String,
    pub request_id: String,
    pub status: String,
    pub error: Option<String>,
    #[ts(type = "number")]
    pub generation: u64,
}
#[derive(Debug, Serialize, Deserialize, TS)]
pub struct HistoryPage {
    pub messages: Vec<HistoryMessage>,
    pub next_cursor: Option<String>,
}
#[derive(Debug, Serialize, Deserialize, TS)]
pub struct HistoryMessage {
    pub id: String,
    pub task: String,
    pub role: String,
    pub text: String,
    pub reasoning: String,
    pub status: String,
    #[ts(type = "number")]
    pub step: u64,
    pub truncated: bool,
}
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize, TS)]
pub struct Question {
    pub id: String,
    pub title: String,
    pub body: String,
    pub choices: Vec<Choice>,
    pub free_text: bool,
    pub multiple: bool,
}
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize, TS)]
pub struct Choice {
    pub id: String,
    pub label: String,
}
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize, TS)]
pub struct Live {
    pub task: Option<String>,
    pub text: String,
    pub reasoning: String,
}
#[derive(Debug, Serialize, Deserialize, TS)]
pub struct ToolInfo {
    pub id: String,
    pub task: String,
    pub name: String,
    pub phase: String,
    #[ts(type = "number")]
    pub step: u64,
    pub output: Option<String>,
    pub truncated: bool,
}
#[derive(Debug, Serialize, Deserialize, TS)]
#[serde(deny_unknown_fields)]
pub struct ProvidersSave {
    #[ts(type = "number")]
    pub generation: u64,
    pub config: vivy_gateway::config::ProvidersConfig,
}
#[derive(Debug, Serialize, Deserialize, TS)]
#[serde(deny_unknown_fields)]
pub struct ProviderKey {
    #[ts(type = "number")]
    pub generation: u64,
    pub provider: String,
    pub key: Option<String>,
}
#[derive(Debug, Serialize, Deserialize, TS)]
pub struct ProvidersView {
    pub presets: Vec<vivy_gateway::desktop_presets::DesktopPreset>,
    #[ts(type = "number")]
    pub generation: u64,
    pub config: vivy_gateway::config::ProvidersConfig,
    pub issue: Option<String>,
    pub protocols: Vec<String>,
}

#[derive(Debug, Serialize, Deserialize, TS)]
pub struct Frame {
    pub v: u32,
    pub instance: String,
    #[ts(type = "number")]
    pub seq: u64,
    pub id: Option<String>,
    #[serde(flatten)]
    pub payload: Payload,
}

#[derive(Debug, Serialize, Deserialize, TS)]
#[serde(tag = "type", content = "data", rename_all = "snake_case")]
pub enum Payload {
    ManagementCompleted(ManagementCompleted),
    Ready(Snapshot),
    Reply(Reply),
    Error(Fault),
    AdmissionCommitted(AdmissionView),
    OperationCommitted(PendingOperation),
    Changed {
        #[ts(type = "number")]
        revision: u64,
    },
    Questions(Vec<Question>),
    Live(Live),
}

#[derive(Debug, Serialize, Deserialize, TS)]
#[serde(untagged)]
pub enum Reply {
    Tasks(TasksView),
    Python(PythonReport),
    Recap(RecapView),
    Snapshot(Snapshot),
    Shutdown { shutting_down: bool },
    Admission(AdmissionView),
    Operation(PendingOperation),
    Answer { answered: bool },
    History(HistoryPage),
    Providers(ProvidersView),
}

#[derive(Debug, Serialize, Deserialize, TS)]
pub struct Snapshot {
    pub session: Option<String>,
    pub assistant: Option<String>,
    pub core_version: String,
    pub protocol_version: u32,
    #[ts(type = "number")]
    pub watermark: u64,
    pub plugins: Vec<PluginInfo>,
    pub agents: Vec<AgentInfo>,
    pub model: Readiness,
    pub python: Readiness,
    pub shell: Readiness,
    pub memory: Readiness,
    #[ts(type = "number")]
    pub revision: u64,
    pub active_task: Option<String>,
    pub tasks: Vec<TaskInfo>,
    pub questions: Vec<Question>,
    pub tools: Vec<ToolInfo>,
}

#[derive(Debug, Serialize, Deserialize, TS)]
pub struct PluginInfo {
    pub name: String,
    pub package: String,
    pub compiled: bool,
}

#[derive(Debug, Serialize, Deserialize, TS)]
pub struct AgentInfo {
    pub id: String,
    pub working: bool,
}

#[derive(Debug, Serialize, Deserialize, TS)]
pub struct Readiness {
    pub compiled: bool,
    pub configured: bool,
    pub ready: bool,
    pub state: String,
    pub issue: Option<String>,
}

#[derive(Debug, Serialize, Deserialize, TS)]
#[serde(rename_all = "snake_case")]
pub enum ErrorCode {
    InvalidFrame,
    InvalidRequest,
    IncompatibleVersion,
    UnsupportedOperation,
    Conflict,
    NotReady,
    StaleIdentity,
    StorageFailure,
    Indeterminate,
}

#[derive(Debug, Serialize, Deserialize, TS)]
pub struct Fault {
    pub code: ErrorCode,
    pub message: String,
}

pub fn typescript() -> String {
    let cfg = ts_rs::Config::default();
    let declarations = [
        Request::decl(&cfg),
        Method::decl(&cfg),
        Frame::decl(&cfg),
        Payload::decl(&cfg),
        Reply::decl(&cfg),
        Snapshot::decl(&cfg),
        PluginInfo::decl(&cfg),
        AgentInfo::decl(&cfg),
        Readiness::decl(&cfg),
        ErrorCode::decl(&cfg),
        Fault::decl(&cfg),
        ProviderTest::decl(&cfg),
        ProbeCheck::decl(&cfg),
        ProbeReport::decl(&cfg),
        ManagedImport::decl(&cfg),
        ManagedEdit::decl(&cfg),
        ManagedValidate::decl(&cfg),
        ManagedId::decl(&cfg),
        ManagedPublish::decl(&cfg),
        ManagedRollback::decl(&cfg),
        vivy_python::managed::desktop::ManagedView::decl(&cfg),
        vivy_python::managed::desktop::PublicationView::decl(&cfg),
        vivy_python::managed::desktop::CandidateView::decl(&cfg),
        WakeCancel::decl(&cfg),
        WakeSchedule::decl(&cfg),
        Resolve::decl(&cfg),
        DreamRequest::decl(&cfg),
        DreamCancel::decl(&cfg),
        DreamDetach::decl(&cfg),
        WakeView::decl(&cfg),
        DreamRequestView::decl(&cfg),
        DreamJobView::decl(&cfg),
        TasksView::decl(&cfg),
        McpSave::decl(&cfg),
        McpControl::decl(&cfg),
        McpAction::decl(&cfg),
        McpResult::decl(&cfg),
        McpTool::decl(&cfg),
        vivy_python::control::McpView::decl(&cfg),
        vivy_python::control::McpConnection::decl(&cfg),
        vivy_python::control::McpTransport::decl(&cfg),
        vivy_python::control::EnvReference::decl(&cfg),
        PythonPrepare::decl(&cfg),
        PythonReport::decl(&cfg),
        SkillImport::decl(&cfg),
        SkillSave::decl(&cfg),
        SkillRemove::decl(&cfg),
        SkillDocument::decl(&cfg),
        SkillName::decl(&cfg),
        PersonaSaveFile::decl(&cfg),
        PersonaRollback::decl(&cfg),
        PromptSave::decl(&cfg),
        PersonaFileView::decl(&cfg),
        PersonaView::decl(&cfg),
        PersonaVersionView::decl(&cfg),
        PromptView::decl(&cfg),
        ManagementResult::decl(&cfg),
        ManagementCompleted::decl(&cfg),
        RecapView::decl(&cfg),
        RecapJob::decl(&cfg),
        SubmitParams::decl(&cfg),
        CancelParams::decl(&cfg),
        AnswerParams::decl(&cfg),
        HistoryParams::decl(&cfg),
        AdmissionView::decl(&cfg),
        AdmissionPhase::decl(&cfg),
        PendingOperation::decl(&cfg),
        TaskInfo::decl(&cfg),
        HistoryPage::decl(&cfg),
        HistoryMessage::decl(&cfg),
        Question::decl(&cfg),
        Choice::decl(&cfg),
        Live::decl(&cfg),
        ToolInfo::decl(&cfg),
        ProvidersSave::decl(&cfg),
        ProviderKey::decl(&cfg),
        ProvidersView::decl(&cfg),
        vivy_gateway::desktop_presets::DesktopPreset::decl(&cfg),
        vivy_gateway::desktop_presets::DesktopVariant::decl(&cfg),
        vivy_gateway::config::ProvidersConfig::decl(&cfg),
        vivy_gateway::config::Instance::decl(&cfg),
        vivy_gateway::config::Protocol::decl(&cfg),
        vivy_gateway::config::Credential::decl(&cfg),
        vivy_gateway::config::Model::decl(&cfg),
    ];
    format!(
        "// Generated from vivy-diva-front Rust DTOs. Do not edit.\nexport const PROTOCOL_VERSION = {VERSION} as const;\n{}\n",
        declarations.map(|s| format!("export {s}")).join("\n")
    )
}

pub fn examples() -> String {
    serde_json::to_string_pretty(&serde_json::json!({
        "request": Request { v: VERSION, id: "snapshot-1".into(), method: Method::Snapshot },
        "response": Frame { v: VERSION, instance: "fixture".into(), seq: 1,
            id: Some("exit-1".into()), payload: Payload::Reply(Reply::Shutdown { shutting_down: true }) },
        "error": Frame { v: VERSION, instance: "fixture".into(), seq: 2,
            id: None, payload: Payload::Error(Fault { code: ErrorCode::Conflict, message: "request ID conflict".into() }) }
    })).unwrap() + "\n"
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn checked_in_contract_matches_rust() {
        let root = std::path::Path::new(env!("CARGO_MANIFEST_DIR")).join("contract");
        assert_eq!(
            std::fs::read_to_string(root.join("protocol.ts")).unwrap(),
            typescript()
        );
        assert_eq!(
            std::fs::read_to_string(root.join("examples.json")).unwrap(),
            examples()
        );
    }
    #[test]
    fn envelope_round_trips_and_rejects_unknown_fields() {
        let example: serde_json::Value = serde_json::from_str(&examples()).unwrap();
        let frame: Frame = serde_json::from_value(example["response"].clone()).unwrap();
        assert_eq!(serde_json::to_value(frame).unwrap(), example["response"]);
        assert!(
            serde_json::from_str::<Request>(
                r#"{"v":1,"id":"a","method":"snapshot","credential":"secret"}"#
            )
            .is_err()
        );
    }
}
