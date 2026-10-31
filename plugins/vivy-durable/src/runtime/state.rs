use super::*;

#[derive(Clone, Debug, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum Status {
    Queued,
    Running,
    Suspended,
    #[cfg(feature = "wake")]
    Waiting,
    Completed,
    Failed,
    Cancelled,
}

#[derive(Clone, Debug, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct Task {
    pub id: String,
    pub source: String,
    pub request_key: String,
    pub input: Incoming,
    pub digest: String,
    pub status: Status,
    pub generation: u64,
    pub next_step: u64,
    pub delivered: bool,
    pub delivery_ready: bool,
    pub admission_revision: u64,
    pub outcome: Option<Value>,
}

#[derive(Clone, Debug, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct Operation {
    pub id: String,
    pub task: String,
    pub generation: u64,
    pub kind: String,
    pub call_id: Option<String>,
    pub request: Value,
    pub digest: String,
    pub route: String,
    pub step: u64,
    pub attempt: u32,
    pub retry_at_ms: u64,
    pub phase: String,
    pub outcome: Option<Value>,
    pub published: bool,
    pub resolution: Option<String>,
    #[cfg(feature = "autodream")]
    pub publication: Option<String>,
    #[cfg(feature = "autodream")]
    pub usage: Vec<Value>,
}

#[derive(Component, Reflect, Clone, Debug, Serialize)]
#[reflect(opaque, Component, Saved, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct DurableAgentState {
    pub revision: u64,
    pub conversation_revision: u64,
    pub workspace: String,
    pub active: Option<String>,
    pub tasks: Vec<Task>,
    pub operations: Vec<Operation>,
    pub control: Option<PendingControl>,
    /// An ended foreground turn still owes a decision to delivered queued input.
    /// Human stop clears this; it never creates idle or scheduled work.
    pub decision_pending: bool,
    #[cfg(feature = "wake")]
    pub wakes: Vec<wake::WakeRecord>,
    #[cfg(feature = "autodream")]
    pub evolution: autodream::EvolutionState,
}

impl<'de> Deserialize<'de> for DurableAgentState {
    fn deserialize<D: serde::Deserializer<'de>>(deserializer: D) -> Result<Self, D::Error> {
        #[derive(Deserialize)]
        #[serde(deny_unknown_fields)]
        struct Wire {
            revision: u64,
            conversation_revision: u64,
            workspace: String,
            active: Option<String>,
            tasks: Vec<Task>,
            operations: Vec<Operation>,
            control: Option<PendingControl>,
            decision_pending: bool,
            #[cfg(feature = "wake")]
            wakes: Vec<wake::WakeRecord>,
            #[cfg(feature = "autodream")]
            evolution: autodream::EvolutionState,
        }
        let wire = Wire::deserialize(deserializer)?;
        let state = Self {
            revision: wire.revision,
            conversation_revision: wire.conversation_revision,
            workspace: wire.workspace,
            active: wire.active,
            tasks: wire.tasks,
            operations: wire.operations,
            control: wire.control,
            decision_pending: wire.decision_pending,
            #[cfg(feature = "wake")]
            wakes: wire.wakes,
            #[cfg(feature = "autodream")]
            evolution: wire.evolution,
        };
        state.validate().map_err(serde::de::Error::custom)?;
        Ok(state)
    }
}

impl DurableAgentState {
    pub(super) fn validate(&self) -> io::Result<()> {
        let mut ids = std::collections::BTreeSet::new();
        let mut keys = std::collections::BTreeSet::new();
        for task in &self.tasks {
            if !ids.insert(task.id.clone())
                || !keys.insert((&task.source, &task.request_key))
                || task.digest != digest(&task.input)
                || task.admission_revision > self.revision
            {
                return Err(invalid("invalid durable input identity or digest"));
            }
            if let Some(value) = &task.outcome {
                FinalOutcome::from_value(value.clone())?;
                if value.get("turn").is_none() {
                    return Err(invalid("a task contains a non-turn outcome"));
                }
            }
        }
        if self.active.as_ref().is_some_and(|id| !ids.contains(id)) {
            return Err(invalid("active task does not exist"));
        }
        if let Some(control) = &self.control {
            if !control.request.valid_action()
                || !ids.contains(&control.request.task)
                || !ids.contains(&control.origin)
            {
                return Err(invalid("invalid durable task control"));
            }
        }
        #[cfg(feature = "wake")]
        wake::validate(self)?;
        #[cfg(feature = "autodream")]
        autodream::validate(self)?;
        let mut operations = std::collections::BTreeSet::new();
        for op in &self.operations {
            if !operations.insert(&op.id)
                || !ids.contains(&op.task)
                || !["prepared", "dispatched", "settled", "unknown", "not_run"]
                    .contains(&op.phase.as_str())
            {
                return Err(invalid("invalid durable operation identity or phase"));
            }
            if !["model", "tool"].contains(&op.kind.as_str())
                || op.id != format!("{}/{}/{}", op.task, op.step, op.attempt)
            {
                return Err(invalid("invalid durable operation kind or identity"));
            }
            if op.route == "not_run" {
                if !op.request.is_null()
                    || op.outcome.is_none()
                    || op.digest != digest(&Value::Null)
                {
                    return Err(invalid("invalid non-dispatched operation"));
                }
            } else {
                if op.digest != digest(&(&op.request, &op.route)) {
                    return Err(invalid("durable request digest mismatch"));
                }
                if op.kind == "model" {
                    serde_json::from_value::<rig_harness::rig_core::completion::CompletionRequest>(
                        op.request.clone(),
                    )
                    .map_err(|e| invalid(format!("invalid saved model request: {e}")))?;
                } else {
                    let call = serde_json::from_value::<rig_harness::rig_core::message::ToolCall>(
                        op.request.clone(),
                    )
                    .map_err(|e| invalid(format!("invalid saved tool request: {e}")))?;
                    if op.call_id.as_deref()
                        != Some(
                            serde_json::to_string(&call.id)
                                .map_err(io::Error::other)?
                                .as_str(),
                        )
                    {
                        return Err(invalid("saved tool identity mismatch"));
                    }
                }
            }
            if let Some(value) = &op.outcome {
                if FinalOutcome::from_value(value.clone())?
                    .effect_result()
                    .map_err(io::Error::other)?
                    .is_none()
                    && value.get("superseded") != Some(&Value::Bool(true))
                {
                    return Err(invalid("an operation contains a non-effect outcome"));
                }
            }
        }
        Ok(())
    }
}

impl Default for DurableAgentState {
    fn default() -> Self {
        Self {
            revision: 0,
            conversation_revision: 0,
            workspace: workspace(),
            active: None,
            tasks: Vec::new(),
            operations: Vec::new(),
            control: None,
            decision_pending: false,
            #[cfg(feature = "wake")]
            wakes: Vec::new(),
            #[cfg(feature = "autodream")]
            evolution: Default::default(),
        }
    }
}

impl DurableAgentState {
    pub(super) fn task(&self, id: &str) -> Option<&Task> {
        self.tasks.iter().find(|t| t.id == id)
    }
    pub(super) fn task_mut(&mut self, id: &str) -> Option<&mut Task> {
        self.tasks.iter_mut().find(|t| t.id == id)
    }
    pub(super) fn bump(&mut self) {
        self.revision += 1;
    }
}

#[derive(Clone, Debug, Serialize, Deserialize, schemars::JsonSchema, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct Control {
    /// continue, switch, defer, finish_silently; wake builds also support wait and cancel_wake.
    pub action: String,
    pub task: String,
    pub generation: u64,
    /// Relative wait in seconds, frozen as UTC when the control is accepted.
    #[cfg(feature = "wake")]
    #[serde(default)]
    pub seconds: Option<u64>,
    #[cfg(feature = "wake")]
    #[serde(default)]
    pub reason: Option<String>,
    /// Required for cancel_wake; get the exact identity from wake_status.
    #[cfg(feature = "wake")]
    #[serde(default)]
    pub wake_id: Option<String>,
    #[cfg(feature = "wake")]
    #[serde(default)]
    pub wake_generation: Option<u64>,
}

impl Control {
    pub(super) fn valid_action(&self) -> bool {
        let ordinary =
            ["continue", "switch", "defer", "finish_silently"].contains(&self.action.as_str());
        #[cfg(feature = "wake")]
        {
            ordinary || matches!(self.action.as_str(), "wait" | "cancel_wake")
        }
        #[cfg(not(feature = "wake"))]
        {
            ordinary
        }
    }
}

/// Host-bound control proposal. The model supplies only `request`; origin and
/// generation are captured from the active task and rechecked at application.
#[derive(Clone, Debug, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct PendingControl {
    pub request: Control,
    pub origin: String,
    pub origin_generation: u64,
    #[cfg(feature = "wake")]
    pub wake: Option<wake::Proposal>,
}

#[derive(Debug, Clone)]
pub struct Admission {
    pub task: String,
    pub revision: u64,
    pub committed: bool,
}

pub(super) fn workspace() -> String {
    std::env::current_dir()
        .and_then(|p| p.canonicalize())
        .map(|p| p.to_string_lossy().into_owned())
        .unwrap_or_default()
}
