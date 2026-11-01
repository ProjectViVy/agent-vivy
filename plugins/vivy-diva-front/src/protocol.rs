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
    pub method: Method,
}

#[derive(Debug, Serialize, Deserialize, TS)]
#[serde(rename_all = "snake_case")]
pub enum Method {
    Snapshot,
    Shutdown,
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
    Ready(Snapshot),
    Reply(Reply),
    Error(Fault),
}

#[derive(Debug, Serialize, Deserialize, TS)]
#[serde(untagged)]
pub enum Reply {
    Snapshot(Snapshot),
    Shutdown { shutting_down: bool },
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
    pub memory: Readiness,
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
