//! Typed management; no Python source is generated from desktop commands.
use super::*;
use serde::Serialize;
use std::{collections::BTreeMap, path::Path};

#[derive(Clone, Debug, Serialize, Deserialize)]
#[cfg_attr(feature = "protocol-types", derive(ts_rs::TS))]
#[serde(deny_unknown_fields)]
pub struct EnvReference {
    pub env: String,
}
#[derive(Clone, Debug, Serialize, Deserialize)]
#[cfg_attr(feature = "protocol-types", derive(ts_rs::TS))]
#[serde(tag = "type", rename_all = "snake_case")]
pub enum McpTransport {
    Stdio {
        command: String,
        #[serde(default)]
        args: Vec<String>,
        #[serde(default, skip_serializing_if = "Option::is_none")]
        cwd: Option<String>,
    },
    Http {
        url: String,
    },
}
#[derive(Clone, Debug, Serialize)]
#[cfg_attr(feature = "protocol-types", derive(ts_rs::TS))]
#[serde(rename_all = "camelCase")]
pub struct McpConnection {
    #[serde(flatten)]
    pub transport: McpTransport,
    #[serde(default = "yes")]
    pub enabled: bool,
    #[serde(default)]
    pub env: BTreeMap<String, EnvReference>,
    #[serde(default)]
    pub headers: BTreeMap<String, EnvReference>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub enabled_tools: Option<Vec<String>>,
    #[serde(default)]
    pub disabled_tools: Vec<String>,
    #[serde(default = "startup")]
    pub startup_timeout_ms: u32,
    #[serde(default = "timeout")]
    pub call_timeout_ms: u32,
}
impl<'de> Deserialize<'de> for McpConnection {
    fn deserialize<D: serde::Deserializer<'de>>(deserializer: D) -> Result<Self, D::Error> {
        use serde::de::Error;
        let value = serde_json::Value::deserialize(deserializer)?;
        let object = value
            .as_object()
            .ok_or_else(|| D::Error::custom("MCP connection must be an object"))?;
        let mut allowed = vec![
            "type",
            "enabled",
            "env",
            "headers",
            "enabledTools",
            "disabledTools",
            "startupTimeoutMs",
            "callTimeoutMs",
        ];
        match object.get("type").and_then(|v| v.as_str()) {
            Some("stdio") => allowed.extend(["command", "args", "cwd"]),
            Some("http") => allowed.push("url"),
            _ => return Err(D::Error::custom("MCP type must be stdio or http")),
        }
        if object.keys().any(|k| !allowed.contains(&k.as_str())) {
            return Err(D::Error::custom("unsupported MCP connection field"));
        }
        #[derive(Deserialize)]
        #[serde(rename_all = "camelCase")]
        struct Options {
            #[serde(default = "yes")]
            enabled: bool,
            #[serde(default)]
            env: BTreeMap<String, EnvReference>,
            #[serde(default)]
            headers: BTreeMap<String, EnvReference>,
            #[serde(default)]
            enabled_tools: Option<Vec<String>>,
            #[serde(default)]
            disabled_tools: Vec<String>,
            #[serde(default = "startup")]
            startup_timeout_ms: u32,
            #[serde(default = "timeout")]
            call_timeout_ms: u32,
        }
        let options: Options = serde_json::from_value(value.clone()).map_err(D::Error::custom)?;
        let transport = serde_json::from_value(value).map_err(D::Error::custom)?;
        Ok(Self {
            transport,
            enabled: options.enabled,
            env: options.env,
            headers: options.headers,
            enabled_tools: options.enabled_tools,
            disabled_tools: options.disabled_tools,
            startup_timeout_ms: options.startup_timeout_ms,
            call_timeout_ms: options.call_timeout_ms,
        })
    }
}
fn yes() -> bool {
    true
}

#[cfg(test)]
mod contract_tests {
    use super::*;
    #[test]
    fn unknown_fields_and_literal_credentials_are_rejected() {
        for value in [
            serde_json::json!({"type":"stdio","command":"python","mystery":true}),
            serde_json::json!({"type":"http","url":"http://localhost","command":"ignored"}),
            serde_json::json!({"type":"stdio","command":"python","env":{"TOKEN":"literal"}}),
        ] {
            assert!(serde_json::from_value::<McpConnection>(value).is_err());
        }
    }
    #[test]
    fn omitted_and_empty_filters_remain_distinct() {
        let mut value = serde_json::json!({"type":"stdio","command":"python","env":{"TOKEN":{"env":"MY_TOKEN"}}});
        let all: McpConnection = serde_json::from_value(value.clone()).unwrap();
        assert!(all.enabled_tools.is_none());
        value["enabledTools"] = serde_json::json!([]);
        let none: McpConnection = serde_json::from_value(value).unwrap();
        assert_eq!(none.enabled_tools, Some(vec![]));
        let text = toml::to_string(&none).unwrap();
        let restored: McpConnection = toml::from_str(&text).unwrap();
        assert_eq!(restored.env["TOKEN"].env, "MY_TOKEN");
        assert_eq!(restored.enabled_tools, Some(vec![]));
    }
}
fn startup() -> u32 {
    20000
}
fn timeout() -> u32 {
    60000
}
#[derive(Debug, Serialize, Deserialize)]
#[cfg_attr(feature = "protocol-types", derive(ts_rs::TS))]
pub struct McpView {
    pub revision: String,
    pub connections: BTreeMap<String, McpConnection>,
}
fn read_text(home: &Path) -> Result<String, String> {
    match std::fs::read_to_string(home.join("settings.toml")) {
        Ok(v) => Ok(v),
        Err(e) if e.kind() == std::io::ErrorKind::NotFound => Ok(String::new()),
        Err(e) => Err(format!("read settings.toml: {e}")),
    }
}
pub fn read(home: &Path) -> Result<McpView, String> {
    let text = read_text(home)?;
    let doc: toml::Value =
        toml::from_str(&text).map_err(|e| format!("invalid settings.toml: {e}"))?;
    let connections = match doc.get("python").and_then(|p| p.get("connections")) {
        Some(v) => v
            .clone()
            .try_into()
            .map_err(|e| format!("invalid MCP configuration: {e}"))?,
        None => BTreeMap::new(),
    };
    validate(&connections)?;
    Ok(McpView {
        revision: vivy_basics::setup::fingerprint(text.as_bytes()),
        connections,
    })
}
fn validate(connections: &BTreeMap<String, McpConnection>) -> Result<(), String> {
    if connections.len() > 64 {
        return Err("at most 64 MCP connections are supported".into());
    }
    for (id, c) in connections {
        if id.is_empty()
            || id.len() > 64
            || !id
                .bytes()
                .all(|b| b.is_ascii_alphanumeric() || b == b'-' || b == b'_')
        {
            return Err("invalid MCP connection ID".into());
        }
        if c.startup_timeout_ms == 0
            || c.startup_timeout_ms > 120000
            || c.call_timeout_ms == 0
            || c.call_timeout_ms > 600000
        {
            return Err("MCP timeout is outside the supported range".into());
        }
        match &c.transport {
            McpTransport::Stdio { command, .. } if command.trim().is_empty() => {
                return Err("MCP command is empty".into());
            }
            McpTransport::Http { url }
                if !(url.starts_with("http://") || url.starts_with("https://"))
                    || url.contains('@') =>
            {
                return Err("MCP URL must use HTTP(S) without embedded credentials".into());
            }
            _ => {}
        }
        for value in c.env.values().chain(c.headers.values()) {
            if value.env.is_empty()
                || !value
                    .env
                    .bytes()
                    .all(|b| b.is_ascii_alphanumeric() || b == b'_')
            {
                return Err("MCP values require environment variable references".into());
            }
        }
    }
    Ok(())
}
pub fn save(
    home: &Path,
    revision: &str,
    connections: &BTreeMap<String, McpConnection>,
) -> Result<McpView, String> {
    validate(connections)?;
    std::fs::create_dir_all(home).map_err(|e| e.to_string())?;
    let lock = std::fs::OpenOptions::new()
        .create(true)
        .truncate(false)
        .write(true)
        .open(home.join("settings.toml.lock"))
        .map_err(|e| e.to_string())?;
    lock.try_lock()
        .map_err(|e| format!("settings are being edited: {e}"))?;
    let text = read_text(home)?;
    if vivy_basics::setup::fingerprint(text.as_bytes()) != revision {
        return Err("settings.toml changed; reload before saving".into());
    }
    let mut doc: toml_edit::DocumentMut = text
        .parse()
        .map_err(|e| format!("invalid settings.toml: {e}"))?;
    let encoded = toml::to_string(connections).map_err(|e| e.to_string())?;
    let table = encoded
        .parse::<toml_edit::DocumentMut>()
        .map_err(|e| e.to_string())?
        .into_table();
    if doc.get("python").is_none() {
        doc["python"] = toml_edit::Item::Table(toml_edit::Table::new());
    }
    doc["python"]["connections"] = toml_edit::Item::Table(table);
    rig_harness::rig_tools::fs::write_atomic(
        &home.join("settings.toml"),
        doc.to_string().as_bytes(),
    )
    .map_err(|e| e.to_string())?;
    read(home)
}

pub fn settings_connections(home: &Path) -> Result<HashMap<String, Value>, String> {
    read(home)?
        .connections
        .into_iter()
        .map(|(id, c)| Ok((id, serde_json::to_value(c).map_err(|e| e.to_string())?)))
        .collect()
}

pub struct Target {
    state: Arc<Workbench>,
    settings: PythonSettings,
    dir: PathBuf,
}
pub fn target(world: &mut World, agent: Entity) -> Target {
    let mut settings = world.resource::<PythonSettings>().clone();
    settings.project_root = world.resource::<ProjectRoot>().0.clone();
    settings.control_home = RigHome::of(world).root().to_path_buf();
    settings.agent = world
        .get::<AgentId>(agent)
        .map(|id| id.0.clone())
        .unwrap_or_default();
    settings.session = world
        .get_resource::<SessionPaths>()
        .and_then(|p| p.0.id())
        .map(|id| id.to_string())
        .unwrap_or_default();
    settings.skills = management::skill_metadata(
        world.get_resource::<vivy_basics::skills::TrustedSkillInventory>(),
    );
    #[cfg(feature = "managed-skills")]
    {
        settings.user_home = settings.control_home.clone();
    }
    let state = world
        .resource_mut::<Workbenches>()
        .0
        .entry(agent)
        .or_default()
        .clone();
    Target {
        dir: settings.project_root.clone(),
        state,
        settings,
    }
}
impl Target {
    pub fn run(&self, action: &str, server: &str) -> Result<Value, String> {
        let port = self.state.management.lock().unwrap().clone();
        let port = match port {
            Some(port) => port,
            None => {
                // Only an explicit inspect/reconnect starts a prepared worker.
                if action == "close" || action == "apply" {
                    return Ok(json!({"server":server,"state":"not_running","tools":[]}));
                }
                let mut slot = self.state.worker.lock().unwrap();
                if slot.is_none() {
                    let cancel = Cancellation::default();
                    let worker = Worker::start(&self.settings, &self.dir, &cancel)?;
                    *self.state.control.lock().unwrap() = Some(worker.process.control());
                    *self.state.management.lock().unwrap() = Some(worker.management.clone());
                    *slot = Some(worker);
                }
                slot.as_ref().unwrap().management.clone()
            }
        };
        port.request(action, server)
    }
}
