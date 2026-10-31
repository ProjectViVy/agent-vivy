//! Explicit native-memory operations over the existing workbench protocol.
use super::*;
use serde::Serialize;

#[derive(Default)]
pub struct MemoryToolsPlugin;
impl Plugin for MemoryToolsPlugin {
    fn build(&self, app: &mut App) {
        if !app.is_plugin_added::<PythonPlugin>() {
            app.add_plugins(PythonPlugin);
        }
        let tool = PythonTool::for_directory(PathBuf::new());
        app.add_tool_per_call(Remember(tool.clone()), ToolOptions::default());
        app.add_tool_per_call(Get(tool.clone()), ToolOptions::default());
        app.add_tool_per_call(Search(tool), ToolOptions::default());
    }
}

pub(super) fn is_tool(name: &str) -> bool {
    matches!(name, "memory_remember" | "memory_get" | "memory_search")
}
pub(super) fn bind(name: &str, tool: &PythonTool) -> Option<Handler> {
    Some(match name {
        "memory_remember" => Handler(ErasedHandler::new(ToolAdapter::new(Remember(tool.clone())))),
        "memory_get" => Handler(ErasedHandler::new(ToolAdapter::new(Get(tool.clone())))),
        "memory_search" => Handler(ErasedHandler::new(ToolAdapter::new(Search(tool.clone())))),
        _ => return None,
    })
}

#[derive(Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
struct RememberArgs {
    content: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    source_id: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    event_id: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    wing: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    room: Option<String>,
}
#[derive(Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
struct GetArgs {
    #[serde(skip_serializing_if = "Option::is_none")]
    identifier: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    source_id: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    event_id: Option<String>,
}
#[derive(Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
struct SearchArgs {
    query: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    source_id: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    wing: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    room: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    limit: Option<u32>,
}

async fn invoke(
    tool: &PythonTool,
    action: &str,
    arguments: Value,
) -> Result<String, ToolExecutionError> {
    if tool.settings.agent.is_empty()
        || tool.settings.agent == "standalone"
        || tool.settings.session.is_empty()
        || tool.settings.session == "unsaved"
    {
        return Err(ToolExecutionError::other(
            "Memory tools require a saved session and stable agent identity",
        ));
    }
    let cancellation = Arc::<Cancellation>::default();
    let mut guard = CancelOnDrop(cancellation.clone(), true);
    let tool = tool.clone();
    let frame = json!({"type":"memory","action":action,"arguments":arguments});
    let result = blocking(move || tool.execute_frame(frame, None, &cancellation)).await;
    guard.1 = false;
    let (text, error) = result?;
    if error {
        Err(ToolExecutionError::other(text))
    } else {
        Ok(text)
    }
}

macro_rules! memory_tool {
    ($ty:ident,$args:ty,$name:literal,$action:literal,$description:literal,$schema:expr) => {
        #[derive(Clone)]
        struct $ty(PythonTool);
        impl PerCallTool for $ty {
            fn at(&self, place: CallPlace) -> Self {
                Self(self.0.at(place))
            }
        }
        impl Tool for $ty {
            const NAME: &'static str = $name;
            type Args = $args;
            type Output = String;
            type Error = ToolExecutionError;
            fn description(&self) -> String {
                $description.into()
            }
            fn parameters(&self) -> Value {
                $schema
            }
            async fn call(&self, _: &mut ToolContext, args: $args) -> Result<String, Self::Error> {
                invoke(
                    &self.0,
                    $action,
                    serde_json::to_value(args)
                        .map_err(|e| ToolExecutionError::other(e.to_string()))?,
                )
                .await
            }
        }
    };
}
memory_tool!(
    Remember,
    RememberArgs,
    "memory_remember",
    "remember",
    "Explicitly remember an event in native workspace memory. Durable hosts supply omitted stable source_id/event_id before dispatch. Reusing an identity with different content is a conflict. After an uncertain write use memory_get with the saved identity, never blindly rewrite. Memory must be enabled and prepared.",
    json!({"type":"object","properties":{"content":{"type":"string"},"source_id":{"type":"string"},"event_id":{"type":"string"},"wing":{"type":"string"},"room":{"type":"string"}},"required":["content"],"additionalProperties":false})
);
memory_tool!(
    Get,
    GetArgs,
    "memory_get",
    "get",
    "Read one native memory by identifier, or by the exact source_id and event_id from a prior write. A missing or incomplete result does not prove an uncertain write succeeded.",
    json!({"type":"object","properties":{"identifier":{"type":"string"},"source_id":{"type":"string"},"event_id":{"type":"string"}},"additionalProperties":false})
);
memory_tool!(
    Search,
    SearchArgs,
    "memory_search",
    "search",
    "Explicit semantic search of native workspace memory. Optional source and partition filters; no automatic recall. Returns bounded hits with identities for memory_get.",
    json!({"type":"object","properties":{"query":{"type":"string"},"source_id":{"type":"string"},"wing":{"type":"string"},"room":{"type":"string"},"limit":{"type":"integer","minimum":1,"maximum":50}},"required":["query"],"additionalProperties":false})
);

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn missing_identity_does_not_start_a_worker() {
        let tool = PythonTool::for_directory(std::env::temp_dir());
        let result = futures::executor::block_on(invoke(&tool, "get", json!({"identifier":"x"})));
        assert!(
            result
                .unwrap_err()
                .to_string()
                .contains("stable agent identity")
        );
        assert!(tool.state.worker.lock().unwrap().is_none());
    }
    #[test]
    fn typed_operation_reuses_worker_without_executing_argument_text() {
        let mut tool = PythonTool::for_directory(std::env::temp_dir());
        tool.settings.agent = "agent-fixture".into();
        tool.settings.session = "session-fixture".into();
        tool.execute(
            PythonArgs {
                code: "kept=42".into(),
                timeout_secs: None,
            },
            &Arc::default(),
        )
        .unwrap();
        let generation = tool
            .state
            .worker
            .lock()
            .unwrap()
            .as_ref()
            .unwrap()
            .process
            .child
            .id();
        assert!(
            futures::executor::block_on(invoke(
                &tool,
                "get",
                json!({"identifier":"exec('kept=0')"})
            ))
            .is_err()
        );
        assert_eq!(
            generation,
            tool.state
                .worker
                .lock()
                .unwrap()
                .as_ref()
                .unwrap()
                .process
                .child
                .id()
        );
        let (text, error) = tool
            .execute(
                PythonArgs {
                    code: "assert kept==42;print('KEPT')".into(),
                    timeout_secs: None,
                },
                &Arc::default(),
            )
            .unwrap();
        assert!(!error && text.contains("KEPT"));
        tool.state.retire();
        assert!(serde_json::from_value::<GetArgs>(json!({"code":"anything"})).is_err());
    }
}
