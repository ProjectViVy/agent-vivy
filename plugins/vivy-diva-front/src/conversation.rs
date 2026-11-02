//! Conversation adapter. Durable state and receipts remain owned by the core.
use crate::{emit, protocol::*};
use rig_harness::prelude::*;
use std::collections::BTreeMap;
use vivy_durable::runtime::{self, DurableAgentState};

#[derive(Resource, Default)]
pub struct Events {
    admissions: BTreeMap<String, String>,
    cancellations: BTreeMap<String, (String, u64)>,
    revision: u64,
    questions: Vec<Question>,
    live: Option<Live>,
    readiness: Option<(
        bool,
        u64,
        Option<vivy_basics::workbench_status::WorkbenchStatus>,
    )>,
}

fn error(code: ErrorCode, message: impl Into<String>) -> Fault {
    Fault {
        code,
        message: rig_harness::rig_core::redact::text(&message.into()).into_owned(),
    }
}
fn agent(world: &World) -> Result<Entity, Fault> {
    runtime::foreground(world).ok_or_else(|| {
        error(
            ErrorCode::NotReady,
            "foreground assistant is not initialized",
        )
    })
}
fn valid_input(input: &SubmitParams) -> Result<(), Fault> {
    if input.request_id.is_empty() || input.request_id.len() > 256 || input.text.trim().is_empty() {
        return Err(error(
            ErrorCode::InvalidRequest,
            "nonempty bounded request ID and text required",
        ));
    }
    Ok(())
}
fn view(key: String, admission: Option<runtime::Admission>) -> AdmissionView {
    AdmissionView {
        request_id: key,
        task: admission.as_ref().map(|a| a.task.clone()),
        phase: match admission {
            None => AdmissionPhase::Missing,
            Some(a) if a.committed => AdmissionPhase::Committed,
            _ => AdmissionPhase::Pending,
        },
    }
}

pub fn submit(world: &mut World, input: SubmitParams, query: bool) -> Result<Reply, Fault> {
    valid_input(&input)?;
    let agent = agent(world)?;
    let previous = runtime::admission(world, agent, "diva", &input.request_id, &input.text)
        .map_err(|e| error(ErrorCode::Conflict, e.to_string()))?;
    if query || previous.as_ref().is_some_and(|a| a.committed) {
        return Ok(Reply::Admission(view(input.request_id, previous)));
    }
    if world.get::<Connection>(agent).is_none() {
        return Err(error(
            ErrorCode::NotReady,
            "configure a reachable model before submitting",
        ));
    }
    let admission = runtime::submit(world, agent, "diva", &input.request_id, &input.text)
        .map_err(|e| error(ErrorCode::StorageFailure, e.to_string()))?;
    if !admission.committed {
        world
            .resource_mut::<Events>()
            .admissions
            .insert(input.request_id.clone(), admission.task.clone());
    }
    Ok(Reply::Admission(view(input.request_id, Some(admission))))
}

pub fn cancel(world: &mut World, input: CancelParams, id: String) -> Result<Reply, Fault> {
    let agent = agent(world)?;
    runtime::cancel(world, agent, &input.task, input.generation)
        .map_err(|e| error(ErrorCode::StaleIdentity, e.to_string()))?;
    world
        .resource_mut::<Events>()
        .cancellations
        .insert(id.clone(), (input.task, input.generation + 1));
    Ok(Reply::Operation(PendingOperation {
        operation: id,
        committed: false,
    }))
}

pub fn questions(world: &mut World) -> Vec<Question> {
    world
        .query_filtered::<(Entity, &Ask, &Decision), (Without<Settled>, Without<SupersededBy>)>()
        .iter(world)
        .filter(|(_, _, d)| !d.is_decided())
        .map(|(e, q, _)| Question {
            id: e.to_bits().to_string(),
            title: q.title.clone(),
            body: q.body.clone(),
            choices: q
                .choices
                .iter()
                .map(|c| crate::protocol::Choice {
                    id: c.id.clone(),
                    label: c.label.clone(),
                })
                .collect(),
            free_text: q.free_text,
            multiple: q.multiple,
        })
        .collect()
}

pub fn answer(world: &mut World, input: AnswerParams, instance: &str) -> Result<Reply, Fault> {
    if input.instance != instance {
        return Err(error(
            ErrorCode::StaleIdentity,
            "question belongs to another core instance",
        ));
    }
    let entity = input
        .question
        .parse::<u64>()
        .ok()
        .and_then(Entity::try_from_bits)
        .ok_or_else(|| error(ErrorCode::StaleIdentity, "invalid question identity"))?;
    let q = world
        .get::<Ask>(entity)
        .ok_or_else(|| error(ErrorCode::StaleIdentity, "question no longer exists"))?;
    if world.get::<Settled>(entity).is_some()
        || world.get::<SupersededBy>(entity).is_some()
        || world.get::<Decision>(entity).is_none_or(|d| d.is_decided())
    {
        return Err(error(
            ErrorCode::StaleIdentity,
            "question is no longer open",
        ));
    }
    if (!q.multiple && input.choices.len() > 1)
        || input
            .choices
            .iter()
            .any(|id| !q.choices.iter().any(|c| c.id == *id))
        || input
            .choices
            .iter()
            .collect::<std::collections::BTreeSet<_>>()
            .len()
            != input.choices.len()
        || (input.text.is_some() && !q.free_text)
        || (input.choices.is_empty() && input.text.as_ref().is_none_or(|t| t.trim().is_empty()))
    {
        return Err(error(
            ErrorCode::InvalidRequest,
            "answer does not match offered choices or text policy",
        ));
    }
    world.get_mut::<Decision>(entity).unwrap().decide(
        Decider::Person,
        Answer {
            choices: input.choices,
            text: input.text,
        }
        .outcome(),
    );
    world.resource::<Wake>().wake();
    Ok(Reply::Answer { answered: true })
}

pub fn task_info(state: &DurableAgentState) -> Vec<TaskInfo> {
    state
        .tasks
        .iter()
        .filter(|t| t.source == "diva")
        .map(|t| TaskInfo {
            id: t.id.clone(),
            request_id: t.request_key.clone(),
            generation: t.generation,
            error: t
                .outcome
                .as_ref()
                .and_then(|v| v.pointer("/turn/failed"))
                .and_then(serde_json::Value::as_str)
                .map(|s| rig_harness::rig_core::redact::text(s).into_owned()),
            status: serde_json::to_value(&t.status)
                .unwrap()
                .as_str()
                .unwrap()
                .into(),
        })
        .collect()
}

pub fn tool_info(state: &DurableAgentState) -> Vec<ToolInfo> {
    state
        .operations
        .iter()
        .filter(|o| o.kind == "tool")
        .filter(|o| {
            state
                .tasks
                .iter()
                .any(|t| t.id == o.task && t.source == "diva")
        })
        .rev()
        .take(100)
        .map(|o| {
            let output = o
                .outcome
                .as_ref()
                .map(|v| rig_harness::rig_core::redact::text(&v.to_string()).into_owned());
            let truncated = output.as_ref().is_some_and(|s| s.len() > 4096);
            ToolInfo {
                id: o.id.clone(),
                task: o.task.clone(),
                name: o
                    .request
                    .pointer("/function/name")
                    .and_then(serde_json::Value::as_str)
                    .unwrap_or("tool")
                    .into(),
                phase: o.phase.clone(),
                output: output.map(|s| preview(&s, 4096)),
                truncated,
            }
        })
        .collect()
}

pub fn history(world: &World, params: HistoryParams) -> Result<Reply, Fault> {
    if !(1..=100).contains(&params.limit) {
        return Err(error(
            ErrorCode::InvalidRequest,
            "history limit must be 1..100",
        ));
    }
    let agent = agent(world)?;
    let state = runtime::inspect_committed(world, agent);
    let revision = state.map_or(0, |s| s.revision);
    let session = world
        .get_resource::<SessionPaths>()
        .and_then(|s| s.0.id())
        .map(|s| s.as_str().to_owned())
        .unwrap_or_default();
    let identity = format!(
        "{session}:{}:{revision}",
        world.get::<AgentId>(agent).unwrap().0
    );
    let offset = match params.cursor {
        None => 0,
        Some(cursor) => decode_cursor(&cursor, &identity)?,
    };
    let mut messages = Vec::new();
    if let Some(state) = state {
        for task in state.tasks.iter().filter(|t| t.source == "diva") {
            let status = serde_json::to_value(&task.status)
                .unwrap()
                .as_str()
                .unwrap()
                .to_owned();
            messages.push(HistoryMessage {
                id: format!(
                    "{session}:{}:{}:user",
                    world.get::<AgentId>(agent).unwrap().0,
                    task.id
                ),
                task: task.id.clone(),
                role: "user".into(),
                text: preview(task.input.text(), 256 * 1024),
                truncated: task.input.text().len() > 256 * 1024,
                status: status.clone(),
            });
            for operation in state
                .operations
                .iter()
                .filter(|o| o.task == task.id && o.kind == "model")
            {
                let Some(value) = &operation.outcome else {
                    continue;
                };
                let outcome = rig_harness::rig_ecs::stage::FinalOutcome::from_value(value.clone())
                    .map_err(|e| error(ErrorCode::StorageFailure, e.to_string()))?;
                if let Some(Outcome(Ok(rig_harness::rig_core::effect::Answered::Completion(
                    response,
                )))) = outcome
                    .effect_result()
                    .map_err(|e| error(ErrorCode::StorageFailure, e.to_string()))?
                {
                    let text = rig_harness::rig_core::message::AssistantMessage::new(
                        response.choice.clone(),
                    )
                    .text();
                    if !text.is_empty() {
                        messages.push(HistoryMessage {
                            id: format!(
                                "{session}:{}:{}",
                                world.get::<AgentId>(agent).unwrap().0,
                                operation.id
                            ),
                            task: task.id.clone(),
                            role: "assistant".into(),
                            truncated: text.len() > 256 * 1024,
                            text: preview(&text, 256 * 1024),
                            status: if response.aborted.is_some() {
                                "interrupted".into()
                            } else {
                                status.clone()
                            },
                        });
                    }
                }
            }
        }
    }
    if offset > messages.len() {
        return Err(error(
            ErrorCode::StaleIdentity,
            "history cursor exceeds committed projection",
        ));
    }
    let end = history_end(&messages, offset, params.limit as usize);
    let next_cursor = (end < messages.len()).then(|| encode_cursor(&format!("{identity}|{end}")));
    Ok(Reply::History(HistoryPage {
        messages: messages.drain(offset..end).collect(),
        next_cursor,
    }))
}
fn preview(text: &str, limit: usize) -> String {
    let mut end = text.len().min(limit);
    while !text.is_char_boundary(end) {
        end -= 1;
    }
    text[..end].to_owned()
}
fn history_end(messages: &[HistoryMessage], offset: usize, limit: usize) -> usize {
    let mut size = 0;
    let mut end = offset;
    for message in messages.iter().skip(offset).take(limit) {
        let length = serde_json::to_vec(message)
            .expect("history DTO serialization")
            .len();
        if end > offset && size + length > 2 * 1024 * 1024 {
            break;
        }
        size += length;
        end += 1;
    }
    end
}
#[cfg(test)]
mod projection_tests {
    use super::*;
    #[test]
    fn page_budget_accounts_for_json_escaping_and_keeps_utf8() {
        let text = "\u{0001}".repeat(256 * 1024);
        let messages: Vec<_> = (0..9)
            .map(|i| HistoryMessage {
                id: i.to_string(),
                task: "t".into(),
                role: "user".into(),
                text: text.clone(),
                status: "completed".into(),
                truncated: false,
            })
            .collect();
        let end = history_end(&messages, 0, 100);
        assert_eq!(end, 1);
        assert!(serde_json::to_vec(&messages[..end]).unwrap().len() < 2 * 1024 * 1024);
        assert_eq!(preview("中文", 4), "中");
    }
}
fn encode_cursor(raw: &str) -> String {
    raw.bytes().map(|b| format!("{b:02x}")).collect()
}
fn decode_cursor(cursor: &str, identity: &str) -> Result<usize, Fault> {
    let bad = || {
        error(
            ErrorCode::StaleIdentity,
            "expired or invalid history cursor; resynchronize",
        )
    };
    if cursor.len() > 2048 || !cursor.len().is_multiple_of(2) || !cursor.is_ascii() {
        return Err(bad());
    }
    let bytes: Result<Vec<_>, _> = (0..cursor.len())
        .step_by(2)
        .map(|i| u8::from_str_radix(&cursor[i..i + 2], 16))
        .collect();
    let raw = String::from_utf8(bytes.map_err(|_| bad())?).map_err(|_| bad())?;
    let (prefix, offset) = raw.rsplit_once('|').ok_or_else(bad)?;
    if prefix != identity {
        return Err(bad());
    }
    offset.parse().map_err(|_| bad())
}

pub fn publish(world: &mut World) {
    let Some(agent) = runtime::foreground(world) else {
        return;
    };
    let state = runtime::inspect_committed(world, agent).cloned();
    let mut events = world.remove_resource::<Events>().unwrap();
    let readiness = (
        world.get::<Connection>(agent).is_some(),
        world
            .get_resource::<vivy_gateway::ProvidersState>()
            .map_or(0, |s| s.generation),
        world
            .get::<vivy_basics::workbench_status::WorkbenchStatus>(agent)
            .copied(),
    );
    if events.readiness.as_ref() != Some(&readiness) {
        events.readiness = Some(readiness);
        emit(
            world,
            None,
            Payload::Changed {
                revision: state.as_ref().map_or(0, |s| s.revision),
            },
        );
    }
    if let Some(state) = state {
        events.admissions.retain(|key, task| {
            if state
                .tasks
                .iter()
                .any(|t| t.id == *task && t.request_key == *key && t.source == "diva")
            {
                emit(
                    world,
                    None,
                    Payload::AdmissionCommitted(AdmissionView {
                        request_id: key.clone(),
                        task: Some(task.clone()),
                        phase: AdmissionPhase::Committed,
                    }),
                );
                false
            } else {
                true
            }
        });
        events.cancellations.retain(|id, (task, generation)| {
            if state.tasks.iter().any(|t| {
                t.id == *task
                    && t.generation == *generation
                    && t.status == runtime::Status::Cancelled
            }) {
                emit(
                    world,
                    None,
                    Payload::OperationCommitted(PendingOperation {
                        operation: id.clone(),
                        committed: true,
                    }),
                );
                false
            } else {
                true
            }
        });
        if state.revision != events.revision {
            events.revision = state.revision;
            emit(
                world,
                None,
                Payload::Changed {
                    revision: state.revision,
                },
            );
        }
    }
    let questions = questions(world);
    if questions != events.questions {
        events.questions = questions.clone();
        emit(world, None, Payload::Questions(questions));
    }
    let text = world
        .query::<(&ForAgent, &Partial)>()
        .iter(world)
        .filter(|(a, _)| a.0 == agent)
        .map(|(_, p)| p.text.as_str())
        .collect::<String>();
    let task = runtime::inspect(world, agent).and_then(|s| s.active.clone());
    let live = Live { task, text };
    if events.live.as_ref() != Some(&live) {
        if events.live.is_some() || !live.text.is_empty() {
            emit(world, None, Payload::Live(live.clone()));
        }
        events.live = Some(live);
    }
    world.insert_resource(events);
}
