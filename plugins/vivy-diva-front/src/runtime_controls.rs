use crate::{emit, protocol::*};
use rig_harness::prelude::*;
#[derive(Resource, Default)]
pub struct Jobs {
    probes: std::collections::BTreeMap<String, (String, Vec<ProbeCheck>)>,
    python: Option<(String, Entity)>,
    durable: std::collections::BTreeMap<String, Vec<(Entity, u64)>>,
}
fn fault(text: impl ToString) -> Fault {
    Fault {
        code: ErrorCode::NotReady,
        message: rig_harness::rig_core::redact::text(&text.to_string()).into_owned(),
    }
}
#[cfg(feature = "persistent")]
fn report(world: &World, agent: Entity) -> PythonReport {
    let value = vivy_python::management::desktop_report(world, agent);
    PythonReport {
        outcome: serde_json::to_value(value.outcome)
            .unwrap()
            .as_str()
            .unwrap()
            .into(),
        phase: value.phase,
        detail: rig_harness::rig_core::redact::text(&value.detail).into_owned(),
        scope: value.scope,
    }
}
pub fn python(world: &mut World, id: String, method: Method) -> Result<Reply, Fault> {
    #[cfg(feature = "persistent")]
    {
        let agent = vivy_durable::runtime::foreground(world)
            .ok_or_else(|| fault("assistant is not ready"))?;
        match method {
            Method::PythonRead => Ok(Reply::Python(report(world, agent))),
            Method::PythonPrepare(input) => {
                if world.resource::<Jobs>().python.is_some() {
                    return Err(fault("Python preparation is already pending"));
                }
                vivy_python::management::desktop_prepare(world, agent, input.managed)
                    .map_err(fault)?;
                world.resource_mut::<Jobs>().python = Some((id.clone(), agent));
                Ok(Reply::Operation(PendingOperation {
                    operation: id,
                    committed: false,
                }))
            }
            Method::PythonCancel => {
                vivy_python::management::desktop_cancel(world, agent).map_err(fault)?;
                Ok(Reply::Python(report(world, agent)))
            }
            _ => unreachable!(),
        }
    }
    #[cfg(not(feature = "persistent"))]
    {
        let _ = (world, id, method);
        Err(fault("Python adapter is not compiled"))
    }
}
pub fn poll(world: &mut World) {
    poll_probes(world);
    #[cfg(feature = "persistent")]
    poll_persona(world);
    let complete = world
        .resource::<Jobs>()
        .durable
        .iter()
        .filter(|(_, revisions)| {
            revisions.iter().all(|(agent, revision)| {
                vivy_durable::runtime::inspect_committed(world, *agent)
                    .is_some_and(|s| s.revision >= *revision)
            })
        })
        .map(|(id, _)| id.clone())
        .collect::<Vec<_>>();
    for id in complete {
        world.resource_mut::<Jobs>().durable.remove(&id);
        emit(
            world,
            None,
            Payload::OperationCommitted(PendingOperation {
                operation: id,
                committed: true,
            }),
        );
    }

    #[cfg(feature = "persistent")]
    if let Some((operation, agent)) = world.resource::<Jobs>().python.clone() {
        let value = report(world, agent);
        if value.phase.is_none() {
            world.resource_mut::<Jobs>().python = None;
            emit(
                world,
                None,
                Payload::ManagementCompleted(ManagementCompleted {
                    operation,
                    result: Some(ManagementResult::Python(value)),
                    error: None,
                }),
            );
        }
    }
}

pub fn durable(world: &mut World, id: String, method: Method) -> Result<Reply, Fault> {
    #[cfg(feature = "persistent")]
    {
        use vivy_durable::runtime::{self, autodream, wake};
        let agent = runtime::foreground(world).ok_or_else(|| fault("assistant is not ready"))?;
        if matches!(method, Method::TasksRead) {
            let dream = autodream::dream_agent(world, agent);
            let saved = runtime::inspect_committed(world, agent);
            let dream_saved = dream.and_then(|e| runtime::inspect_committed(world, e));
            return Ok(Reply::Tasks(TasksView {
                revision: saved.map_or(0, |s| s.revision),
                dream_revision: dream_saved.map(|s| s.revision),
                wakes: saved
                    .into_iter()
                    .flat_map(|s| &s.wakes)
                    .map(|w| WakeView {
                        id: w.id.clone(),
                        task: w.task.clone(),
                        generation: w.generation,
                        task_generation: w.task_generation,
                        deadline_ms: w.deadline_ms,
                        reason: w.reason.clone(),
                        phase: serde_json::to_value(&w.phase)
                            .unwrap()
                            .as_str()
                            .unwrap()
                            .into(),
                        delivered: w.delivered.is_some(),
                    })
                    .collect(),
                dreams: saved
                    .into_iter()
                    .flat_map(|s| &s.evolution.requests)
                    .map(|r| DreamRequestView {
                        id: r.id.clone(),
                        task: r.task.clone(),
                        reasons: r.reasons.clone(),
                        finished: r.finished,
                    })
                    .collect(),
                jobs: dream_saved
                    .into_iter()
                    .flat_map(|s| &s.evolution.jobs)
                    .map(|j| DreamJobView {
                        id: j.id.clone(),
                        task: j.task.clone(),
                        target: j.target.clone(),
                        phase: j.phase.clone(),
                        result: j.result.as_ref().map(|r| {
                            rig_harness::rig_core::redact::text(&r.to_string()).into_owned()
                        }),
                        error: j
                            .error
                            .as_ref()
                            .map(|e| rig_harness::rig_core::redact::text(e).into_owned()),
                        skipped: j.skipped,
                    })
                    .collect(),
            }));
        }
        if world.resource::<Jobs>().durable.len() >= 32 {
            return Err(fault("too many pending durable controls"));
        }
        let revisions = match method {
            Method::WakeCancel(p) => vec![(
                agent,
                wake::cancel_wake(world, agent, &p.id, p.generation).map_err(fault)?,
            )],
            Method::WakeSchedule(p) => {
                wake::schedule_wake(
                    world,
                    agent,
                    &p.task,
                    p.generation,
                    p.previous,
                    p.deadline_ms,
                    &p.reason,
                )
                .map_err(fault)?;
                vec![(agent, runtime::inspect(world, agent).unwrap().revision)]
            }
            Method::Resolve(p) => {
                if p.evidence.len() > 64 * 1024 {
                    return Err(fault("resolution evidence exceeds 64 KiB"));
                }
                vec![(
                    agent,
                    runtime::resolve(world, agent, &p.operation, p.generation, &p.evidence)
                        .map_err(fault)?,
                )]
            }
            Method::DreamRequest(p) => {
                autodream::request(world, agent, &p.request_id, &p.reason).map_err(fault)?;
                vec![(agent, runtime::inspect(world, agent).unwrap().revision)]
            }
            Method::DreamCancel(p) => {
                autodream::cancel_desktop(world, agent, p.revision, p.dream_revision)
                    .map_err(fault)?
            }
            Method::DreamDetach(p) => {
                vec![autodream::detach_desktop(world, agent, &p.job, p.revision).map_err(fault)?]
            }
            Method::DreamJobCancel(p) => {
                vec![
                    autodream::cancel_job_desktop(world, agent, &p.job, p.revision)
                        .map_err(fault)?,
                ]
            }
            _ => unreachable!(),
        };
        world
            .resource_mut::<Jobs>()
            .durable
            .insert(id.clone(), revisions);
        Ok(Reply::Operation(PendingOperation {
            operation: id,
            committed: false,
        }))
    }
    #[cfg(not(feature = "persistent"))]
    {
        let _ = (world, id, method);
        Err(fault("durable management adapter is not compiled"))
    }
}

#[derive(Component)]
struct Probe {
    operation: String,
    kind: String,
    deadline: std::time::Instant,
}
pub fn provider(world: &mut World, id: String, input: ProviderTest) -> Result<Reply, Fault> {
    use rig_harness::rig_ecs::tasks::Running;
    if world.resource::<Jobs>().probes.len() >= 4 {
        return Err(fault("provider test queue is full"));
    }
    if world.resource::<vivy_gateway::ProvidersState>().generation != input.generation {
        return Err(Fault {
            code: ErrorCode::Conflict,
            message: "provider configuration changed; reload before testing".into(),
        });
    }
    let connection = world
        .resource::<Models>()
        .connect(&input.reference)
        .map_err(fault)?;
    world
        .resource_mut::<Jobs>()
        .probes
        .insert(id.clone(), (input.reference, vec![]));
    for (kind, streaming, tools) in [
        ("text", false, false),
        ("stream", true, false),
        ("tools", true, true),
    ] {
        let connection = connection.clone();
        let work = Running::spawn(
            bevy_tasks::IoTaskPool::get_or_init(bevy_tasks::TaskPool::default),
            world.resource::<Wake>(),
            async move { vivy_gateway::setup::probe_step(connection, streaming, tools).await },
        );
        world.spawn((
            Probe {
                operation: id.clone(),
                kind: kind.into(),
                deadline: std::time::Instant::now() + std::time::Duration::from_secs(45),
            },
            work,
            rig_harness::clock::WakeAt::after(std::time::Duration::from_secs(45)),
        ));
    }
    Ok(Reply::Operation(PendingOperation {
        operation: id,
        committed: false,
    }))
}
fn poll_probes(world: &mut World) {
    use rig_harness::rig_ecs::tasks::Done;
    let finished = world
        .query::<(Entity, &Probe, Option<&Done<Result<(), String>>>)>()
        .iter(world)
        .filter_map(|(entity, probe, result)| {
            let result = result.map(|d| d.0.clone()).or_else(|| {
                (std::time::Instant::now() >= probe.deadline)
                    .then(|| Err("provider test timed out after 45 seconds".into()))
            })?;
            Some((
                entity,
                probe.operation.clone(),
                ProbeCheck {
                    kind: probe.kind.clone(),
                    passed: result.is_ok(),
                    detail: result
                        .err()
                        .map(|e| rig_harness::rig_core::redact::text(&e).into_owned())
                        .unwrap_or_else(|| "passed".into()),
                },
            ))
        })
        .collect::<Vec<_>>();
    for (entity, id, check) in finished {
        world.despawn(entity);
        if let Some((_, checks)) = world.resource_mut::<Jobs>().probes.get_mut(&id) {
            checks.push(check);
        }
        if world
            .resource::<Jobs>()
            .probes
            .get(&id)
            .is_some_and(|(_, c)| c.len() == 3)
        {
            let (reference, checks) = world.resource_mut::<Jobs>().probes.remove(&id).unwrap();
            emit(
                world,
                None,
                Payload::ManagementCompleted(ManagementCompleted {
                    operation: id,
                    result: Some(ManagementResult::ProviderTest(ProbeReport {
                        reference,
                        checks,
                    })),
                    error: None,
                }),
            );
        }
    }
}

#[cfg(feature = "persistent")]
#[derive(Component)]
struct PersonaJob {
    operation: String,
    update_id: String,
    deadline: std::time::Instant,
}

#[cfg(feature = "persistent")]
fn persona_store(world: &World) -> vivy_laputa::PersonaStore {
    match world.get_resource::<vivy_laputa::PersonaSource>() {
        Some(source) => vivy_laputa::PersonaStore::at(source.0.clone()),
        None => {
            let root = world
                .get_resource::<ProjectRoot>()
                .map(|root| root.0.clone())
                .unwrap_or_default();
            vivy_laputa::PersonaStore::global(&root)
        }
    }
}

#[cfg(feature = "persistent")]
fn emit_persona_error(world: &mut World, operation: String, error: impl ToString) {
    emit(
        world,
        None,
        Payload::ManagementCompleted(ManagementCompleted {
            operation,
            result: None,
            error: Some(fault(error)),
        }),
    );
}

#[cfg(feature = "persistent")]
fn emit_onboarded(
    world: &mut World,
    operation: String,
    store: &vivy_laputa::PersonaStore,
    persona: &vivy_laputa::Persona,
    fallback: bool,
) -> Result<(), Fault> {
    let current = crate::files::persona_view(store)?;
    emit(
        world,
        None,
        Payload::ManagementCompleted(ManagementCompleted {
            operation,
            result: Some(ManagementResult::PersonaOnboarded(PersonaOnboarded {
                current,
                name: persona.name().into(),
                fallback,
            })),
            error: None,
        }),
    );
    Ok(())
}

/// Onboarding writes the persona directly. `fallback` skips the model; otherwise
/// one tool-free completion extracts a draft and the poll publishes it.
pub fn persona(world: &mut World, id: String, input: PersonaInitialize) -> Result<Reply, Fault> {
    #[cfg(feature = "persistent")]
    {
        use rig_harness::rig_ecs::tasks::Running;
        if input.description.len() > 8 * 1024 {
            return Err(Fault {
                code: ErrorCode::InvalidRequest,
                message: "the introduction exceeds 8 KiB".into(),
            });
        }
        if input.update_id.trim().is_empty() || input.update_id.len() > 256 {
            return Err(Fault {
                code: ErrorCode::InvalidRequest,
                message: "an initialization update ID is required".into(),
            });
        }
        let store = persona_store(world);
        if let Some(prior) = store.update(&input.update_id).map_err(fault)? {
            emit_onboarded(world, id.clone(), &store, &prior, input.fallback)?;
            return Ok(Reply::Operation(PendingOperation {
                operation: id,
                committed: false,
            }));
        }
        if store.initialized().map_err(fault)? {
            return Err(Fault {
                code: ErrorCode::Conflict,
                message: "a persona is already initialized".into(),
            });
        }
        if input.fallback {
            let draft = vivy_laputa::fallback();
            let persona =
                vivy_laputa::write(&store, &draft, &input.update_id, false).map_err(fault)?;
            emit_onboarded(world, id.clone(), &store, &persona, true)?;
            return Ok(Reply::Operation(PendingOperation {
                operation: id,
                committed: false,
            }));
        }
        let agent = vivy_durable::runtime::foreground(world)
            .ok_or_else(|| fault("assistant is not ready"))?;
        let reference = world
            .get::<ModelChoice>(agent)
            .map(|choice| choice.0.clone())
            .ok_or_else(|| fault("choose a model before initializing a persona"))?;
        let connection = world
            .resource::<Models>()
            .connect(&reference)
            .map_err(fault)?;
        let description = input.description;
        let work = Running::spawn(
            bevy_tasks::IoTaskPool::get_or_init(bevy_tasks::TaskPool::default),
            world.resource::<Wake>(),
            async move { vivy_laputa::extract(connection, &description).await },
        );
        world.spawn((
            PersonaJob {
                operation: id.clone(),
                update_id: input.update_id,
                deadline: std::time::Instant::now() + std::time::Duration::from_secs(120),
            },
            work,
            rig_harness::clock::WakeAt::after(std::time::Duration::from_secs(120)),
        ));
        Ok(Reply::Operation(PendingOperation {
            operation: id,
            committed: false,
        }))
    }
    #[cfg(not(feature = "persistent"))]
    {
        let _ = (world, id, input);
        Err(fault("persona initialization is not compiled"))
    }
}

#[cfg(feature = "persistent")]
fn poll_persona(world: &mut World) {
    use rig_harness::rig_ecs::tasks::Done;
    let finished = world
        .query::<(
            Entity,
            &PersonaJob,
            Option<&Done<Result<vivy_laputa::Draft, String>>>,
        )>()
        .iter(world)
        .filter_map(|(entity, job, result)| {
            let result = result.map(|done| done.0.clone()).or_else(|| {
                (std::time::Instant::now() >= job.deadline)
                    .then(|| Err("persona initialization timed out after 120 seconds".into()))
            })?;
            Some((entity, job.operation.clone(), job.update_id.clone(), result))
        })
        .collect::<Vec<_>>();
    for (entity, operation, update_id, result) in finished {
        world.despawn(entity);
        let store = persona_store(world);
        let draft = match result {
            Ok(draft) => draft,
            Err(error) => {
                emit_persona_error(world, operation, error);
                continue;
            }
        };
        match vivy_laputa::write(&store, &draft, &update_id, false) {
            Ok(persona) => {
                if let Err(error) =
                    emit_onboarded(world, operation.clone(), &store, &persona, draft.fallback)
                {
                    emit_persona_error(world, operation, error.message);
                }
            }
            Err(error) => emit_persona_error(world, operation, error),
        }
    }
}
