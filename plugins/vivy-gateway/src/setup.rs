use super::providers::ProvidersState;
use futures::StreamExt;
use rig_harness::pick::{Picked, Picker};
use rig_harness::prelude::*;
use rig_harness::rig_core::{
    completion::{CompletionRequest, Message, ToolDefinition},
    effect::{EffectId, EffectKind},
    serve::{Dispatch, Reply},
    streaming::{Item as StreamItem, Relayed, StreamEvent},
};
use rig_harness::rig_ecs::tasks::{Done, Running};
use std::time::{Duration, Instant};
use vivy_basics::setup::{self, OutcomeState, ProviderStatus, Report};

#[derive(Component)]
struct Verification {
    agent: Entity,
    scope: String,
    reference: String,
    deadline: Instant,
    previous: Report,
}
#[derive(Component, Default)]
struct Cached {
    scope: String,
    report: Report,
}

pub(super) fn install(app: &mut App) {
    setup::register(
        app,
        10,
        setup::Item {
            key: "provider",
            title: "Model provider",
            report,
            open,
        },
    );
    app.add_systems(Update, (publish, poll).chain())
        .add_observer(cancel);
}
fn identity(world: &World, agent: Entity) -> String {
    let state = world.resource::<ProvidersState>();
    format!("{}:{}", state.generation, persistent_scope(world, agent))
}
fn persistent_scope(world: &World, agent: Entity) -> String {
    let state = world.resource::<ProvidersState>();
    let mut bytes = serde_json::to_vec(&state.config).unwrap_or_default();
    bytes.extend_from_slice(
        world
            .get::<ModelChoice>(agent)
            .map(|m| m.0.as_bytes())
            .unwrap_or_default(),
    );
    for instance in state.config.instances.values() {
        if let super::config::Credential::Env { name } = &instance.credential {
            bytes.extend_from_slice(std::env::var(name).unwrap_or_default().as_bytes());
        }
    }
    setup::fingerprint(&bytes)
}
fn local(world: &World, agent: Entity) -> Report {
    let state = world.resource::<ProvidersState>();
    if let Some(issue) = &state.issue {
        return Report::error(issue);
    }
    let Some(model) = world.get::<ModelChoice>(agent) else {
        return Report {
            detail: if state.config.instances.is_empty() {
                "No provider configured. Add an endpoint, model and credential source."
            } else {
                "Select a model, then save it as the startup default."
            }
            .into(),
            ..Default::default()
        };
    };
    match world.resource::<Models>().connect(&model.0) {
        Ok(_) => Report::ready(
            "Configured; connection not tested. /setup provider offers an explicit test.",
        ),
        Err(error) => Report::error(error),
    }
}
fn publish(world: &mut World) {
    let agents: Vec<_> = world
        .query_filtered::<Entity, With<Agent>>()
        .iter(world)
        .collect();
    for agent in agents {
        let scope = identity(world, agent);
        if world.get::<Cached>(agent).is_none_or(|c| c.scope != scope) {
            let mut report = local(world, agent);
            report.scope = scope.clone();
            world.entity_mut(agent).insert((
                Cached {
                    scope,
                    report: report.clone(),
                },
                ProviderStatus(report),
            ));
        }
    }
}
pub(super) fn report(world: &World, agent: Entity) -> Report {
    let mut report = world
        .get::<Cached>(agent)
        .map(|c| c.report.clone())
        .unwrap_or_else(|| local(world, agent));
    report.scope = persistent_scope(world, agent);
    report
}
pub(super) fn open(world: &mut World, agent: Entity) {
    let report = report(world, agent);
    let picker = Picker::new("Provider initialization").for_agent(agent).body(format!("{}\nTesting sends a fixed small prompt and, for models declaring tools, one harmless tool-call request. It sends no session messages.", report.detail))
        .item("Configure provider / missing fields", "provider")
        .item("Choose current model", "model")
        .item("Save current model as default", "model-default")
        .item("Test current connection", "")
        .item("Continue without testing", "");
    world
        .spawn(picker)
        .observe(move |event: On<Picked>, mut commands: Commands| {
            if event.chosen.as_ref().and_then(|c| c.items.first()) == Some(&3) {
                commands.queue(move |world: &mut World| test(world, agent));
            }
        });
}
pub(super) fn test(world: &mut World, agent: Entity) {
    let Some(reference) = world.get::<ModelChoice>(agent).map(|m| m.0.clone()) else {
        world.write_message(Notice::error(agent, "Choose a model before testing"));
        return;
    };
    test_reference(world, agent, &reference);
}
pub(super) fn test_reference(world: &mut World, agent: Entity, reference: &str) {
    if world
        .query::<&Verification>()
        .iter(world)
        .any(|v| v.agent == agent)
    {
        return;
    }
    let connection = world.resource::<Models>().connect(reference);
    let connection = match connection {
        Ok(c) => c,
        Err(e) => {
            world.write_message(Notice::error(agent, e));
            return;
        }
    };
    let scope = identity(world, agent);
    let previous = report(world, agent);
    let work = Running::spawn(
        bevy_tasks::IoTaskPool::get_or_init(bevy_tasks::TaskPool::default),
        world.resource::<Wake>(),
        async move { probe(connection).await },
    );
    world
        .spawn((
            Verification {
                agent,
                scope: scope.clone(),
                reference: reference.into(),
                deadline: Instant::now() + Duration::from_secs(45),
                previous,
            },
            work,
            WakeAt::after(Duration::from_secs(45)),
            Picker::new("Testing provider")
                .for_agent(agent)
                .body("Testing streaming and declared tool calls. No session messages are sent.")
                .item("Cancel test", ""),
        ))
        .observe(
            |picked: On<Picked>, jobs: Query<&Verification>, mut commands: Commands| {
                if let Ok(job) = jobs.get(picked.entity) {
                    let agent = job.agent;
                    let scope = job.scope.clone();
                    let previous = job.previous.clone();
                    commands.queue(move |world: &mut World| {
                        restore_cancelled(world, agent, scope, previous)
                    });
                }
            },
        );
    let report = Report {
        phase: Some("testing".into()),
        detail: "Testing streaming and declared tool support; Esc cancels.".into(),
        scope,
        ..Default::default()
    };
    world
        .entity_mut(agent)
        .insert(ProviderStatus(report.clone()));
    if let Some(mut cache) = world.get_mut::<Cached>(agent) {
        cache.report = report;
    }
    world.write_message(Notice::info(
        agent,
        "Provider test started. Saved configuration is retained if the test fails.",
    ));
}
fn request(tools: bool) -> CompletionRequest {
    let mut request = CompletionRequest::from(vec![Message::user(if tools {
        "Call vivy_setup_ping exactly once with {}. This is a harmless connection test."
    } else {
        "Reply with OK. This is a connection test."
    })]);
    request.max_tokens = Some(256);
    if tools {
        request.tools.push(std::sync::Arc::new(ToolDefinition::new(
            rig_harness::rig_core::completion::message::ToolName::new("vivy_setup_ping").unwrap(),
            "Harmless connection test; returns OK and has no side effects",
            serde_json::json!({"type":"object","properties":{},"additionalProperties":false}),
        )));
    }
    request
}
async fn probe(connection: Connection) -> Result<String, String> {
    for tools in [false, true]
        .into_iter()
        .filter(|tools| !tools || connection.spec.tools)
    {
        let reply = connection
            .handler
            .0
            .handle(
                EffectKind::Completion {
                    request: request(tools).into(),
                    stream: true,
                },
                Dispatch::new(EffectId::from_raw(1), true),
            )
            .await;
        let mut stream = match reply {
            Reply::Stream(s) => s,
            Reply::Outcome(Err(e)) => return Err(e.to_string()),
            Reply::Outcome(Ok(_)) => return Err("Provider did not return a stream".into()),
        };
        let (mut streamed, mut tool, mut done) = (false, false, false);
        while let Some(event) = stream.next().await {
            match event.map_err(|e| e.to_string())? {
                Relayed::Item(StreamItem::Event(event)) => {
                    streamed = true;
                    if let StreamEvent::End {
                        content: rig_harness::rig_core::completion::AssistantContent::ToolCall(call),
                        ..
                    } = event
                    {
                        tool |= call.function.name.as_str() == "vivy_setup_ping"
                            && call.function.arguments.is_empty();
                    }
                }
                Relayed::Done(_) => {
                    done = true;
                    break;
                }
                _ => {}
            }
        }
        if !streamed || !done {
            return Err("Provider stream was empty or incomplete".into());
        }
        if tools && !tool {
            return Err("Streaming succeeded, but the declared tool-call capability did not return the harmless probe".into());
        }
    }
    Ok("Provider verified: streaming and declared tool support passed.".into())
}
fn poll(world: &mut World) {
    let jobs: Vec<_> = world
        .query::<(Entity, &Verification, Option<&Done<Result<String, String>>>)>()
        .iter(world)
        .filter_map(|(e, job, result)| {
            result
                .map(|r| {
                    (
                        e,
                        job.agent,
                        job.scope.clone(),
                        job.reference.clone(),
                        r.0.clone(),
                    )
                })
                .or_else(|| {
                    (Instant::now() >= job.deadline).then(|| {
                        (
                            e,
                            job.agent,
                            job.scope.clone(),
                            job.reference.clone(),
                            Err("Provider test timed out".into()),
                        )
                    })
                })
        })
        .collect();
    for (entity, agent, scope, reference, result) in jobs {
        world.despawn(entity);
        if world.get_entity(agent).is_err() || identity(world, agent) != scope {
            continue;
        }
        let report = match result {
            Ok(text) => Report::ready(text),
            Err(text) => Report::error(text.chars().take(4096).collect::<String>()),
        };
        world.write_message(if report.outcome == OutcomeState::Error {
            Notice::error(agent, &report.detail)
        } else {
            Notice::info(agent, &report.detail)
        });
        if world
            .get::<ModelChoice>(agent)
            .is_none_or(|m| m.0 != reference)
        {
            world.entity_mut(agent).remove::<Cached>();
            continue;
        }
        world.entity_mut(agent).insert((
            Cached {
                scope,
                report: report.clone(),
            },
            ProviderStatus(report),
        ));
    }
}
fn cancel(event: On<StopTurn>, jobs: Query<(Entity, &Verification)>, mut commands: Commands) {
    for (entity, job) in &jobs {
        if job.agent == event.entity {
            let agent = job.agent;
            let scope = job.scope.clone();
            let previous = job.previous.clone();
            commands.entity(entity).despawn();
            commands.queue(move |world: &mut World| {
                restore_cancelled(world, agent, scope, previous);
            });
        }
    }
}

fn restore_cancelled(world: &mut World, agent: Entity, scope: String, previous: Report) {
    if world.get_entity(agent).is_err() || identity(world, agent) != scope {
        return;
    }
    world.entity_mut(agent).insert((
        Cached {
            scope,
            report: previous.clone(),
        },
        ProviderStatus(previous),
    ));
    world.write_message(Notice::info(
        agent,
        "Provider test cancelled; saved configuration is unchanged.",
    ));
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn probes_never_include_history_or_real_tools() {
        let plain = request(false);
        assert_eq!(plain.chat_history.len(), 1);
        assert!(plain.tools.is_empty());
        let tool = request(true);
        assert_eq!(tool.tools.len(), 1);
        assert_eq!(tool.tools[0].name.as_str(), "vivy_setup_ping");
        assert_eq!(tool.max_tokens, Some(256));
    }
}
