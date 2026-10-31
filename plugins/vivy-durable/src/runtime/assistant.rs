//! Per-agent Recap projection and foreground-preemptible maintenance.
use super::*;
use rig_harness::rig_core::{
    conversation::pairing,
    message::{Message, UserContent},
};
use std::sync::Arc;
mod maintenance;
#[cfg(feature = "checkpoint-probe")]
pub(super) fn probe(stage: &str, state: &AssistantContextState) {
    use std::io::Write;
    if !std::env::var("VIVY_RECAP_PAUSE").is_ok_and(|s| s == stage) {
        return;
    }
    let Some(path) = std::env::var_os("VIVY_RECAP_TRACE") else {
        return;
    };
    let mut file = std::fs::OpenOptions::new()
        .append(true)
        .create(true)
        .open(path)
        .expect("recap probe trace");
    writeln!(
        file,
        "{}",
        json!({"stage":stage,"state":state,"paused":true})
    )
    .expect("recap probe write");
    file.flush().expect("recap probe flush");
    loop {
        std::thread::park_timeout(std::time::Duration::from_secs(30));
    }
}
#[cfg(not(feature = "checkpoint-probe"))]
pub(super) fn probe(_: &str, _: &AssistantContextState) {}

pub(super) const CONTEXT_KEY: &str = "vivy.assistant-context.v1";
#[derive(Component, Reflect, Clone, Debug, Default, Serialize)]
#[reflect(opaque, Component, Saved, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct AssistantContextState {
    pub revision: u64,
    pub recap_revision: u64,
    pub summary: String,
    pub covered: BTreeMap<String, usize>,
    pub requests: BTreeMap<String, ContextRef>,
    pub source_times: BTreeMap<String, u64>,
    pub jobs: Vec<maintenance::RecapJob>,
    pub refresh: bool,
    pub error: Option<String>,
}

impl<'de> Deserialize<'de> for AssistantContextState {
    fn deserialize<D: serde::Deserializer<'de>>(deserializer: D) -> Result<Self, D::Error> {
        #[derive(Deserialize)]
        #[serde(deny_unknown_fields)]
        struct Wire {
            revision: u64,
            recap_revision: u64,
            summary: String,
            covered: BTreeMap<String, usize>,
            requests: BTreeMap<String, ContextRef>,
            source_times: BTreeMap<String, u64>,
            jobs: Vec<maintenance::RecapJob>,
            refresh: bool,
            error: Option<String>,
        }
        let w = Wire::deserialize(deserializer)?;
        let state = Self {
            revision: w.revision,
            recap_revision: w.recap_revision,
            summary: w.summary,
            covered: w.covered,
            requests: w.requests,
            source_times: w.source_times,
            jobs: w.jobs,
            refresh: w.refresh,
            error: w.error,
        };
        state.validate().map_err(serde::de::Error::custom)?;
        Ok(state)
    }
}
impl AssistantContextState {
    fn validate(&self) -> io::Result<()> {
        let mut covered = BTreeMap::new();
        let mut publications = 0;
        if self.recap_revision > self.revision
            || (self.recap_revision == 0) != self.summary.is_empty()
        {
            return Err(invalid("invalid Recap revision or text"));
        }
        for (index, job) in self.jobs.iter().enumerate() {
            if job.id != index as u64 + 1
                || job.base_revision > self.recap_revision
                || job.spans.is_empty()
                || !job.request.tools.is_empty()
                || job.request_digest != digest(&(&job.request, &job.route))
                || !matches!(
                    job.phase.as_str(),
                    "prepared"
                        | "dispatched"
                        | "received"
                        | "published"
                        | "failed"
                        | "cancelled"
                        | "stale"
                )
            {
                return Err(invalid("invalid saved Recap operation"));
            }
            if let Some(value) = &job.result {
                FinalOutcome::from_value(value.clone())?;
            }
            if job.spans.iter().any(|s| {
                s.source.is_empty()
                    || s.start >= s.end
                    || s.digest.len() != 64
                    || !s.digest.bytes().all(|b| b.is_ascii_hexdigit())
            }) {
                return Err(invalid("invalid Recap source range"));
            }
            if job.phase == "published" {
                if job.result.is_none() || job.base_revision != publications {
                    return Err(invalid("invalid Recap publication lineage"));
                }
                for span in &job.spans {
                    if covered.get(&span.source).copied().unwrap_or(0) != span.start {
                        return Err(invalid("non-contiguous Recap coverage"));
                    }
                    covered.insert(span.source.clone(), span.end);
                }
                publications += 1;
            }
        }
        if publications != self.recap_revision || covered != self.covered {
            return Err(invalid("Recap coverage lacks a committed publication"));
        }
        if self.requests.values().any(|r| {
            r.task.is_empty()
                || r.recap_revision > self.recap_revision
                || r.request_digest.len() != 64
        }) {
            return Err(invalid("invalid request context reference"));
        }
        Ok(())
    }
}
#[derive(Clone, Debug, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct ContextRef {
    pub persona: Option<vivy_laputa::PersonaRevision>,
    pub recap_revision: u64,
    pub task: String,
    pub sources: Vec<String>,
    pub request_digest: String,
}
#[derive(Component, Clone)]
struct Selected {
    recap_revision: u64,
    task: String,
    sources: Vec<String>,
}
#[derive(Component)]
struct FrozenProjection;

#[derive(Default)]
pub struct AssistantContextPlugin;
impl Plugin for AssistantContextPlugin {
    fn build(&self, app: &mut App) {
        app.register_type::<AssistantContextState>()
            .init_resource::<SavedKeys>();
        app.world_mut()
            .resource_mut::<SavedKeys>()
            .require(
                std::any::type_name::<AssistantContextState>(),
                CONTEXT_KEY,
                1,
            )
            .expect("unique context schema");
        app.add_observer(|added: On<Add<Agent>>, mut commands: Commands| {
            commands
                .entity(added.entity)
                .insert_if_new(AssistantContextState::default());
        })
        .add_systems(
            First,
            project.after(super::admit_inputs).before(super::gate),
        )
        .add_systems(AgentPass, project.in_set(PrepareSystems::Fit));
        app.configure_sets(AgentPass, vivy_laputa::PersonaCompose.after(super::context));
        maintenance::install(app);
    }
}

fn input_message(input: &Incoming) -> Message {
    let mut content = Vec::new();
    if let Some(header) = input.author.header() {
        content.push(UserContent::text(header));
    }
    for part in &input.parts {
        match part {
            MessagePart::Text(text) => content.push(UserContent::text(text.clone())),
            MessagePart::Attachment(a) => content.push(a.content.clone()),
        }
    }
    Message::User { content }
}

/// Materialize only this task's saved execution records, not another turn's raw
/// transcript. References are admitted-input and operation IDs, never text matches.
fn task_messages(
    state: &DurableAgentState,
    task: &str,
) -> io::Result<(Vec<Arc<Message>>, Vec<String>)> {
    let record = state
        .task(task)
        .ok_or_else(|| invalid("context task is missing"))?;
    let mut messages = vec![Arc::new(input_message(&record.input))];
    let mut sources = vec![format!("{task}/input")];
    let mut calls = BTreeMap::new();
    #[cfg(feature = "wake")]
    let mut notices = {
        let mut notices = super::wake::notices(state, task);
        #[cfg(feature = "autodream")]
        notices.extend(super::autodream::jobs::notices(state, task));
        notices.sort_by_key(|n| n.0);
        notices.into_iter().peekable()
    };
    for op in state.operations.iter().filter(|o| o.task == task) {
        #[cfg(feature = "wake")]
        if op.kind == "model" {
            while notices.peek().is_some_and(|n| n.0 <= op.step) {
                let (_, id, input) = notices.next().unwrap();
                messages.push(Arc::new(input_message(&input)));
                sources.push(id);
            }
        }
        let Some(value) = &op.outcome else { continue };
        let Some(outcome) = FinalOutcome::from_value(value.clone())?
            .effect_result()
            .map_err(io::Error::other)?
        else {
            continue;
        };
        if op.kind == "model" {
            if let Some(reply) = outcome.completion() {
                if let Some(message) = reply.message() {
                    for call in reply.tool_calls() {
                        calls.insert(serde_json::to_string(&call.id).unwrap(), call.clone());
                    }
                    messages.push(Arc::new(message));
                    sources.push(op.id.clone());
                }
            }
        } else if let Some(call) = op.call_id.as_ref().and_then(|id| calls.remove(id)) {
            messages.push(Arc::new(Message::tool_results(vec![
                call.answer(&outcome.tool_result()),
            ])));
            sources.push(op.id.clone());
        }
    }
    #[cfg(feature = "wake")]
    for (_, id, input) in notices {
        messages.push(Arc::new(input_message(&input)));
        sources.push(id);
    }
    pairing::validate(&messages).map_err(io::Error::other)?;
    Ok((messages, sources))
}

fn project(world: &mut World) {
    let calls: Vec<_> = world
        .query_filtered::<(Entity, &ForAgent, &ModelRequest), (
            With<Admitted>,
            With<TurnCall>,
            Without<Selected>,
            Without<Started>,
            Without<Outcome>,
        )>()
        .iter(world)
        .filter(|(_, _, m)| m.history.is_some())
        .map(|(e, a, m)| (e, a.0, m.clone()))
        .collect();
    for (call, agent, mut request) in calls {
        let Some(state) = committed(world, agent) else {
            continue;
        };
        #[cfg(feature = "wake")]
        if world
            .get::<DurableAgentState>(agent)
            .is_some_and(|local| local.wakes != state.wakes)
        {
            continue;
        }
        let Some(task) = state.active.clone() else {
            continue;
        };
        if world
            .get::<DurableAgentState>(agent)
            .is_none_or(|s| s.active.as_ref() != Some(&task))
        {
            continue;
        }
        let recap = storage::committed_context(world, agent)
            .cloned()
            .unwrap_or_default();
        // A persisted native call may precede its durable context/intent receipt.
        // Such a call was never allowed to dispatch and must be projected on reopen.
        // Calls with a saved reference instead replay their exact frozen request.
        if world.get::<Restored>(call).is_some()
            && world.get::<Bound>(call).is_some_and(|b| {
                state
                    .operations
                    .iter()
                    .find(|o| o.id == b.operation)
                    .is_some_and(|o| {
                        recap
                            .requests
                            .values()
                            .any(|r| r.task == o.task && r.request_digest == o.digest)
                    })
            })
        {
            continue;
        }
        let retry = world
            .get::<CallOf>(call)
            .and_then(|of| world.get::<rig_harness::rig_ecs::turn::Recovery>(of.0))
            .is_some_and(|r| r.retries > 0);
        if retry {
            if let Some(operation) = state
                .operations
                .iter()
                .rev()
                .find(|o| o.task == task && o.kind == "model")
                .filter(|o| {
                    recap
                        .requests
                        .values()
                        .any(|r| r.task == o.task && r.request_digest == o.digest)
                })
            {
                if let Ok(frozen) = serde_json::from_value(operation.request.clone()) {
                    request.request = Arc::new(frozen);
                    request.prompt = None;
                    request.history = None;
                    world.entity_mut(call).insert((request, FrozenProjection));
                    continue;
                }
            }
        }
        match task_messages(state, &task) {
            Ok((mut messages, mut sources)) => {
                for queued in state
                    .tasks
                    .iter()
                    .filter(|t| t.id != task && t.delivered && t.status == Status::Queued)
                {
                    #[cfg(feature = "wake")]
                    if state
                        .wakes
                        .iter()
                        .any(|w| w.task == queued.id && w.phase == super::wake::Phase::Fired)
                    {
                        continue;
                    }
                    messages.push(Arc::new(input_message(&queued.input)));
                    sources.push(format!("{}/input", queued.id));
                }
                if !recap.summary.is_empty() {
                    messages.insert(
                        0,
                        Arc::new(Message::user(format!(
                            "<committed-recap revision=\"{}\">\n{}\n</committed-recap>",
                            recap.recap_revision, recap.summary
                        ))),
                    );
                }
                request.history = None;
                request.request_mut().chat_history = messages;
                if let Some(adapted) = world.get::<Adapted>(call).cloned() {
                    let sent = adapted.sent(&request);
                    let used = rig_memory::estimate_tokens(&sent.chat_history)
                        + serde_json::to_vec(&sent.tools).map_or(0, |v| v.len().div_ceil(4) as u64);
                    let limit = adapted.0.spec.input_limit().map(u64::from);
                    let margin = limit
                        .map(|n| rig_memory::OUTPUT_MARGIN.min(n / 4))
                        .unwrap_or(0);
                    if limit.is_none_or(|limit| used.saturating_add(margin) >= limit) {
                        let why = if limit.is_none() {
                            "assistant context needs the model's context_window or max_input_tokens"
                        } else {
                            "required assistant context exceeds the configured model window; input is preserved"
                        };
                        let mut state = world.get_mut::<DurableAgentState>(agent).unwrap();
                        state.task_mut(&task).unwrap().status = Status::Suspended;
                        state.bump();
                        drop(state);
                        if let Some(turn) = world.get::<CallOf>(call).map(|of| of.0) {
                            world
                                .entity_mut(turn)
                                .insert(TurnOutcome::Stopped(Some(why.into())));
                        }
                        world.entity_mut(call).insert(Outcome::refused(why));
                        world.write_message(Notice::error(agent, why));
                        continue;
                    }
                    if let Some(window) = adapted.0.spec.context_window {
                        let room = u64::from(window).saturating_sub(used + margin);
                        let output = sent.max_tokens.unwrap_or(rig_memory::TURN_OUTPUT).min(room);
                        request.request_mut().max_tokens = Some(output);
                    }
                }
                world.entity_mut(call).insert((
                    request,
                    Selected {
                        recap_revision: recap.recap_revision,
                        task,
                        sources,
                    },
                ));
            }
            Err(error) => {
                world
                    .entity_mut(call)
                    .insert(Outcome::refused(format!("Assistant context: {error}")));
            }
        }
    }
}

pub fn request_refresh(world: &mut World, agent: Entity) -> io::Result<()> {
    let mut state = world
        .get_mut::<AssistantContextState>(agent)
        .ok_or_else(|| invalid("assistant context is not compiled for this agent"))?;
    state.refresh = true;
    state.error = None;
    state.revision += 1;
    drop(state);
    world.resource::<Wake>().wake();
    Ok(())
}

pub(super) fn is_maintenance(world: &World, call: Entity) -> bool {
    world
        .get::<Effect>(call)
        .is_some_and(|e| e.purpose.as_deref() == Some("vivy.recap"))
}

pub(super) fn projection_ready(world: &World, agent: Entity, call: Entity) -> bool {
    world.get::<AssistantContextState>(agent).is_none()
        || world.get::<Selected>(call).is_some()
        || world.get::<FrozenProjection>(call).is_some()
        || world.get::<Bound>(call).is_some_and(|b| {
            storage::committed_context(world, agent).is_some_and(|s| {
                s.requests.keys().any(|id| {
                    id == &b.operation
                        || id.rsplit_once('/').map(|p| p.0)
                            == b.operation.rsplit_once('/').map(|p| p.0)
                })
            })
        })
}

pub(super) fn intent_ready(
    world: &mut World,
    agent: Entity,
    call: Entity,
    operation: &Operation,
) -> bool {
    if world.get::<AssistantContextState>(agent).is_none() {
        return true;
    }
    let Some(selected) = world.get::<Selected>(call).cloned() else {
        let previous = storage::committed_context(world, agent)
            .and_then(|s| {
                s.requests.get(&operation.id).or_else(|| {
                    s.requests
                        .iter()
                        .find(|(id, r)| {
                            id.starts_with(&format!("{}/{}/", operation.task, operation.step))
                                && r.request_digest == operation.digest
                        })
                        .map(|(_, r)| r)
                })
            })
            .cloned();
        if let Some(reference) = previous {
            let mut state = world.get_mut::<AssistantContextState>(agent).unwrap();
            if !state.requests.contains_key(&operation.id) {
                state.requests.insert(operation.id.clone(), reference);
                state.revision += 1;
            }
        }
        return storage::committed_context(world, agent).is_some_and(|s| {
            s.requests
                .get(&operation.id)
                .is_some_and(|r| r.request_digest == operation.digest)
        });
    };
    if selected.task != operation.task {
        return false;
    }
    let reference = ContextRef {
        persona: world.get::<vivy_laputa::PersonaRevision>(call).cloned(),
        recap_revision: selected.recap_revision,
        task: selected.task,
        sources: selected.sources,
        request_digest: operation.digest.clone(),
    };
    let mut state = world.get_mut::<AssistantContextState>(agent).unwrap();
    if state.requests.get(&operation.id) != Some(&reference) {
        state
            .requests
            .insert(operation.id.clone(), reference.clone());
        state.revision += 1;
    }
    drop(state);
    storage::committed_context(world, agent)
        .is_some_and(|s| s.requests.get(&operation.id) == Some(&reference))
}

#[cfg(test)]
mod tests;
