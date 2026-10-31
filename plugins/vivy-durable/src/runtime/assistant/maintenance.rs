use super::*;
use rig_harness::rig_ecs::tasks::Running;
use rig_harness::{host::cli::Cli, rig_core::completion::CompletionRequest};

#[derive(Clone, Debug, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct Span {
    pub source: String,
    pub start: usize,
    pub end: usize,
    pub digest: String,
}
#[derive(Clone, Debug, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct RecapJob {
    pub id: u64,
    pub base_revision: u64,
    pub spans: Vec<Span>,
    pub request: CompletionRequest,
    pub route: String,
    pub request_digest: String,
    pub attempt: u32,
    pub retry_at_ms: u64,
    pub phase: String,
    pub result: Option<Value>,
}
#[derive(Component, Clone)]
struct RecapCall(u64, u32);
#[derive(Resource, Default)]
struct Initialized(std::collections::BTreeSet<String>);
struct Source {
    id: String,
    text: String,
}

pub(super) fn install(app: &mut App) {
    app.init_resource::<Initialized>()
        .add_systems(
            First,
            (tick, gate, capture)
                .chain()
                .after(super::super::admit_inputs)
                .before(super::super::gate),
        )
        .add_systems(
            AgentPass,
            gate.in_set(GateSystems::After).before(super::super::gate),
        )
        .add_systems(
            AgentPass,
            capture.in_set(Stage::Finalize).after(super::super::capture),
        )
        .add_systems(
            Last,
            observe.after(super::super::boundary).in_set(SaveSystems),
        )
        .add_command("recap", "Recap status | refresh", command)
        .add_observer(|e: On<StopTurn>, mut commands: Commands| {
            let agent = e.entity;
            commands.queue(move |world: &mut World| stop(world, agent));
        });
}

fn stop(world: &mut World, agent: Entity) {
    let calls: Vec<_> = world
        .query_filtered::<(Entity, &ForAgent, &RecapCall), Without<Settled>>()
        .iter(world)
        .filter(|(_, a, _)| a.0 == agent)
        .map(|(e, _, c)| (e, c.0, c.1))
        .collect();
    if calls.is_empty() {
        return;
    }
    if let Some(mut s) = world.get_mut::<AssistantContextState>(agent) {
        for job in &mut s.jobs {
            if calls.iter().any(|(_, id, _)| *id == job.id) {
                job.phase = "cancelled".into();
            }
        }
        s.refresh = false;
        s.error = Some("Recap stopped; use /recap refresh to resume".into());
        s.revision += 1;
    }
    for (call, _, _) in calls {
        world.entity_mut(call).remove::<Running>();
        if world.get::<Outcome>(call).is_none() {
            world
                .entity_mut(call)
                .insert(Outcome::refused("Recap stopped"));
        }
        release(world, call);
    }
}

fn command(In(args): In<CommandArgs>, world: &mut World) {
    if args.args.trim() == "refresh" {
        match request_refresh(world, args.agent) {
            Ok(()) => {
                world.write_message(Notice::info(
                    args.agent,
                    "Recap refresh requested; foreground work has priority.",
                ));
            }
            Err(e) => {
                world.write_message(Notice::error(args.agent, e.to_string()));
            }
        }
    } else if matches!(args.args.trim(), "" | "status") {
        let text=storage::committed_context(world,args.agent).map(|s|format!("Recap revision {}; covered sources {}; pending sources {}; job {}; failure: {}",s.recap_revision,s.covered.len(),committed(world,args.agent).map(|d|sources(d).iter().filter(|v|s.covered.get(&v.id).copied().unwrap_or(0)<v.text.len()).count()).unwrap_or(0),s.jobs.last().map(|j|j.phase.as_str()).unwrap_or("none"),s.error.as_deref().unwrap_or("none"))).unwrap_or_else(||"Recap has no committed state yet.".into());
        world.write_message(Notice::info(args.agent, text));
    } else {
        world.write_message(Notice::error(
            args.agent,
            "Use recap status or recap refresh.",
        ));
    }
}

fn sources(state: &DurableAgentState) -> Vec<Source> {
    let mut sources = vec![];
    for task in &state.tasks {
        if (!matches!(
            task.status,
            Status::Completed | Status::Failed | Status::Cancelled | Status::Suspended
        ) && !super::super::is_waiting(&task.status))
            || task.outcome.is_none()
        {
            continue;
        }
        let Ok((messages, refs)) = task_messages(state, &task.id) else {
            continue;
        };
        for (message, id) in messages.into_iter().zip(refs) {
            sources.push(Source {
                id,
                text: serde_json::to_string(&message).unwrap(),
            });
        }
        sources.push(Source {
            id: format!("{}/status/{}/{:?}", task.id, task.generation, task.status),
            text: format!(
                "Task {} is {:?}. Unknown effects: {}",
                task.id,
                task.status,
                serde_json::to_string(
                    &state
                        .operations
                        .iter()
                        .filter(|o| o.task == task.id && o.phase == "unknown")
                        .map(|o| (&o.id, &o.resolution))
                        .collect::<Vec<_>>()
                )
                .unwrap()
            ),
        });
    }
    #[cfg(feature = "wake")]
    for (id, text) in super::super::wake::sources(state) {
        sources.push(Source { id, text });
    }
    // A wake can be observed in the foreground task and later in its target.
    // Those projections share one immutable source; cover its bytes only once.
    #[cfg(feature = "wake")]
    {
        let mut seen = std::collections::BTreeSet::new();
        sources.retain(|s| seen.insert(s.id.clone()));
    }
    sources
}

fn observe(mut agents: Query<(&DurableAgentState, &mut AssistantContextState)>) {
    for (durable, mut context) in &mut agents {
        for task in &durable.tasks {
            let key = format!("{}/input", task.id);
            if let std::collections::btree_map::Entry::Vacant(e) = context.source_times.entry(key) {
                e.insert(now_ms());
                context.revision += 1;
            }
        }
        for op in durable.operations.iter().filter(|o| o.outcome.is_some()) {
            if let std::collections::btree_map::Entry::Vacant(e) =
                context.source_times.entry(op.id.clone())
            {
                e.insert(now_ms());
                context.revision += 1;
            }
        }
        for source in sources(durable) {
            if let std::collections::btree_map::Entry::Vacant(e) =
                context.source_times.entry(source.id)
            {
                e.insert(now_ms());
                context.revision += 1;
            }
        }
    }
}

const INSTRUCTIONS: &str = include_str!("../../../../../prompts/RECAP.md");

fn request(summary: &str, body: &str, output: u64, instructions: &str) -> CompletionRequest {
    CompletionRequest::new(Message::user(format!(
        "<previous-recap>\n{summary}\n</previous-recap>\n<sources>\n{body}\n</sources>"
    )))
    .preamble(instructions)
    .max_tokens(output)
}
fn plan(
    context: &AssistantContextState,
    durable: &DurableAgentState,
    spec: &rig_harness::rig_core::catalog::ModelSpec,
    instructions: &str,
) -> io::Result<Option<(CompletionRequest, Vec<Span>)>> {
    let limit = spec
        .input_limit()
        .map(u64::from)
        .ok_or_else(|| invalid("Recap requires configured model window facts"))?;
    let output = 2048
        .min(spec.max_output_tokens.map(u64::from).unwrap_or(2048))
        .min(limit / 4)
        .max(1);
    let budget = limit.saturating_sub(output + rig_memory::OUTPUT_MARGIN.min(limit / 4));
    let mut body = String::new();
    let mut spans = vec![];
    for source in sources(durable) {
        let start = context.covered.get(&source.id).copied().unwrap_or(0);
        if start >= source.text.len() {
            continue;
        }
        if !source.text.is_char_boundary(start) {
            return Err(invalid("invalid Recap source offset"));
        }
        let stamp = context.source_times.get(&source.id).copied();
        let mut end = source.text.len();
        let mut selected = None;
        while end > start {
            let addition=serde_json::to_string(&json!({"source":source.id,"occurred_at_unix_ms":stamp,"start_byte":start,"end_byte":end,"total_bytes":source.text.len(),"content":&source.text[start..end]})).unwrap();
            let candidate = format!("{body}{addition}\n");
            let req = request(&context.summary, &candidate, output, instructions);
            if rig_memory::estimate_tokens(&req.chat_history) <= budget {
                selected = Some(candidate);
                break;
            }
            end = start + (end - start) / 2;
            while end > start && !source.text.is_char_boundary(end) {
                end -= 1;
            }
        }
        let Some(candidate) = selected else { break };
        body = candidate;
        spans.push(Span {
            source: source.id,
            start,
            end,
            digest: digest(&&source.text[start..end]),
        });
        if end < source.text.len() {
            break;
        }
    }
    if spans.is_empty() {
        if sources(durable)
            .iter()
            .any(|s| context.covered.get(&s.id).copied().unwrap_or(0) < s.text.len())
        {
            return Err(invalid(
                "Recap and required source metadata exceed the model window",
            ));
        }
        return Ok(None);
    }
    Ok(Some((
        request(&context.summary, &body, output, instructions),
        spans,
    )))
}

fn busy(world: &World, agent: Entity) -> bool {
    world.get::<ActiveTurn>(agent).is_some()
        || world.get::<DurableAgentState>(agent).is_some_and(|s| {
            s.active.is_some()
                || s.decision_pending
                || s.tasks.iter().any(|t| t.status == Status::Queued)
        })
}

fn tick(world: &mut World) {
    if !healthy(world) {
        return;
    }
    let agents: Vec<_> = world
        .query::<(Entity, &AgentId, &AssistantContextState)>()
        .iter(world)
        .map(|(e, id, _)| (e, id.0.clone()))
        .collect();
    for (agent, id) in agents {
        let Some(saved) = storage::committed_context(world, agent).cloned() else {
            continue;
        };
        let Some(durable) = committed(world, agent).cloned() else {
            continue;
        };
        let Some(connection) = world.get::<Connection>(agent).cloned() else {
            continue;
        };
        let foreground = busy(world, agent);
        let first = world.resource_mut::<Initialized>().0.insert(id);
        let live: Vec<_> = world
            .query::<(Entity, &ForAgent, &RecapCall)>()
            .iter(world)
            .filter(|(_, a, _)| a.0 == agent)
            .map(|(e, _, c)| (e, c.0, c.1))
            .collect();
        let mut context = world.get::<AssistantContextState>(agent).unwrap().clone();
        let before = serde_json::to_value(&context).unwrap();
        if first {
            for job in context
                .jobs
                .iter_mut()
                .filter(|j| matches!(j.phase.as_str(), "prepared" | "dispatched"))
            {
                if job.attempt >= rig_harness::rig_ecs::turn::RETRY_POLICY.max_retries {
                    job.phase = "failed".into();
                    context.error = Some("interrupted Recap exhausted Rig retry policy".into());
                } else {
                    job.attempt += 1;
                    job.retry_at_ms = now_ms()
                        + rig_harness::rig_ecs::turn::RETRY_POLICY
                            .backoff(job.attempt - 1)
                            .as_millis() as u64;
                    job.phase = "prepared".into();
                }
            }
        }
        if foreground {
            for job in context
                .jobs
                .iter_mut()
                .filter(|j| matches!(j.phase.as_str(), "prepared" | "dispatched"))
            {
                job.phase = "cancelled".into();
                context.refresh = false;
                context.error = None;
            }
            for (call, job_id, _) in &live {
                if context
                    .jobs
                    .iter()
                    .any(|j| j.id == *job_id && j.phase == "cancelled")
                    && world.get::<Settled>(*call).is_none()
                {
                    world.entity_mut(*call).remove::<Running>();
                    if world.get::<Outcome>(*call).is_none() {
                        world
                            .entity_mut(*call)
                            .insert(Outcome::refused("Recap yielded to foreground input"));
                    }
                    release(world, *call);
                }
            }
        }
        for job in &mut context.jobs {
            let Some(record) = saved.jobs.iter().find(|j| j.id == job.id) else {
                continue;
            };
            if job.phase != "received"
                || record.result != job.result
                || live.iter().any(|(e, id, attempt)| {
                    *id == job.id && *attempt == job.attempt && world.get::<Settled>(*e).is_none()
                })
            {
                continue;
            }
            let Some(value) = &job.result else { continue };
            let outcome = FinalOutcome::from_value(value.clone())
                .and_then(|s| s.effect_result().map_err(io::Error::other));
            let result = match outcome {
                Ok(Some(outcome)) => match outcome.0 {
                    Ok(rig_harness::rig_core::effect::Answered::Completion(reply)) => {
                        rig_memory::Summarizer::summary_text(&reply).map_err(|e| e.to_string())
                    }
                    Err(error) => {
                        if let rig_harness::rig_ecs::retry::Verdict::Retry(delay) =
                            rig_harness::rig_ecs::turn::RETRY_POLICY.verdict(
                                &error,
                                job.attempt,
                                std::time::SystemTime::now(),
                            )
                        {
                            job.attempt += 1;
                            job.retry_at_ms = now_ms() + delay.as_millis() as u64;
                            job.result = None;
                            job.phase = "prepared".into();
                            continue;
                        }
                        Err(error.to_string())
                    }
                    _ => Err("Recap did not produce a model reply".into()),
                },
                _ => Err("invalid Recap result".into()),
            };
            match result {
                Ok(summary) => {
                    let material = sources(&durable);
                    let valid = job.base_revision == context.recap_revision
                        && job.spans.iter().all(|span| {
                            context.covered.get(&span.source).copied().unwrap_or(0) == span.start
                                && material
                                    .iter()
                                    .find(|s| s.id == span.source)
                                    .and_then(|s| s.text.get(span.start..span.end))
                                    .is_some_and(|text| digest(&text) == span.digest)
                        });
                    if valid {
                        context.summary = summary;
                        context.recap_revision += 1;
                        for span in &job.spans {
                            context.covered.insert(span.source.clone(), span.end);
                        }
                        job.phase = "published".into();
                        context.error = None;
                    } else {
                        job.phase = "stale".into();
                    }
                }
                Err(error) => {
                    job.phase = "failed".into();
                    context.error = Some(error);
                }
            }
        }
        let auto = world
            .get_resource::<Cli>()
            .is_none_or(|c| !matches!(c.front(), Some("print" | "json")));
        #[cfg(feature = "autodream")]
        let auto = auto && !durable.evolution.dream;
        if !foreground
            && !context
                .jobs
                .iter()
                .any(|j| matches!(j.phase.as_str(), "prepared" | "dispatched" | "received"))
            && context.error.is_none()
            && (auto || context.refresh)
            && context.recap_revision == saved.recap_revision
        {
            let planned = vivy_basics::prompt::load(world, "RECAP.md", INSTRUCTIONS)
                .and_then(|instructions| plan(&context, &durable, &connection.spec, &instructions));
            match planned {
                Ok(Some((request, spans))) => {
                    let route = super::super::route(world, &Adapted(connection.clone()));
                    let request_digest = digest(&(&request, &route));
                    context.jobs.push(RecapJob {
                        id: context.jobs.len() as u64 + 1,
                        base_revision: context.recap_revision,
                        spans,
                        request,
                        route,
                        request_digest,
                        attempt: 0,
                        retry_at_ms: 0,
                        phase: "prepared".into(),
                        result: None,
                    });
                    context.refresh = false;
                }
                Ok(None) => context.refresh = false,
                Err(error) => {
                    context.error = Some(error.to_string());
                    context.refresh = false;
                }
            }
        }
        if serde_json::to_value(&context).unwrap() != before {
            context.revision += 1;
            world.entity_mut(agent).insert(context.clone());
            world.resource::<Wake>().wake();
        }
        if !foreground {
            for job in context.jobs.iter().filter(|j| j.phase == "prepared") {
                let active = live.iter().any(|(e, n, attempt)| {
                    *n == job.id && *attempt == job.attempt && world.get::<Settled>(*e).is_none()
                });
                if active {
                    continue;
                }
                if job.route != super::super::route(world, &Adapted(connection.clone())) {
                    let mut s = world.get_mut::<AssistantContextState>(agent).unwrap();
                    s.jobs.iter_mut().find(|j| j.id == job.id).unwrap().phase = "failed".into();
                    s.error =
                        Some("Recap provider route changed; request an explicit refresh".into());
                    s.revision += 1;
                    continue;
                }
                world.spawn((
                    Name::new("Recap maintenance"),
                    ForAgent(agent),
                    RecapCall(job.id, job.attempt),
                    rig_harness::clock::WakeAt(
                        std::time::UNIX_EPOCH + std::time::Duration::from_millis(job.retry_at_ms),
                    ),
                    ModelRequest::new(job.request.clone()).on(connection.clone()),
                    Effect::default().with_purpose("vivy.recap"),
                ));
            }
        }
    }
}

fn gate(world: &mut World) {
    let calls: Vec<_> = world
        .query_filtered::<(Entity, &ForAgent, &RecapCall, &ModelRequest, &Adapted), (
            With<Admitted>,
            Without<Started>,
            Without<Outcome>,
            Without<Settled>,
        )>()
        .iter(world)
        .map(|(e, a, c, m, r)| (e, a.0, c.0, c.1, m.clone(), r.clone()))
        .collect();
    for (call, agent, id, attempt, model, adapted) in calls {
        hold(world, call);
        if !healthy(world) {
            continue;
        }
        let sent = adapted.sent(&model);
        let route = super::super::route(world, &adapted);
        let hash = digest(&(&sent, &route));
        let mut context = world.get_mut::<AssistantContextState>(agent).unwrap();
        let Some(job) = context.jobs.iter_mut().find(|j| j.id == id) else {
            continue;
        };
        if job.attempt != attempt || !matches!(job.phase.as_str(), "prepared" | "dispatched") {
            drop(context);
            decide(world, call, Outcome::refused("Recap no longer eligible"));
            continue;
        }
        if job.retry_at_ms > now_ms() {
            continue;
        }
        if job.request_digest != hash {
            job.request = sent;
            job.request_digest = hash.clone();
            context.revision += 1;
        }
        drop(context);
        if storage::committed_context(world, agent).is_some_and(|s| {
            s.jobs.iter().any(|j| {
                j.id == id
                    && j.attempt == attempt
                    && j.request_digest == hash
                    && matches!(j.phase.as_str(), "prepared" | "dispatched")
            })
        }) && !busy(world, agent)
        {
            let mut context = world.get_mut::<AssistantContextState>(agent).unwrap();
            context.jobs.iter_mut().find(|j| j.id == id).unwrap().phase = "dispatched".into();
            context.revision += 1;
            drop(context);
            probe(
                "after_intent",
                world.get::<AssistantContextState>(agent).unwrap(),
            );
            release(world, call);
        }
    }
}

fn capture(world: &mut World) {
    let results: Vec<_> = world
        .query_filtered::<(Entity, &ForAgent, &RecapCall, &FinalOutcome), Without<Settled>>()
        .iter(world)
        .map(|(e, a, c, r)| (e, a.0, c.0, c.1, r.clone()))
        .collect();
    for (call, agent, id, attempt, result) in results {
        let Some(mut context) = world.get_mut::<AssistantContextState>(agent) else {
            continue;
        };
        let Some(job) = context.jobs.iter_mut().find(|j| j.id == id) else {
            continue;
        };
        if job.attempt != attempt {
            continue;
        }
        if job.result.as_ref() != Some(result.value()) {
            job.result = Some(result.value().clone());
            if matches!(job.phase.as_str(), "prepared" | "dispatched") {
                job.phase = "received".into();
            }
            context.revision += 1;
        }
        drop(context);
        probe(
            "after_effect",
            world.get::<AssistantContextState>(agent).unwrap(),
        );
        if healthy(world)
            && storage::committed_context(world, agent).is_some_and(|s| {
                s.jobs.iter().any(|j| {
                    j.id == id && j.attempt == attempt && j.result.as_ref() == Some(result.value())
                })
            })
        {
            world
                .entity_mut(call)
                .insert(CommitBarrier::acknowledged(&result));
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::runtime::tests::until;
    use rig_core::test_utils::MockCompletionModel;
    use rig_harness_test_support::{MemoryStore, reply};
    #[cfg(feature = "wake")]
    #[test]
    fn one_wake_seen_by_two_tasks_has_one_recap_source() {
        use crate::runtime::wake::{Delivery, Phase, WakeRecord};
        let disk = MemoryStore::default();
        let model = MockCompletionModel::from_stream_turns([reply("one"), reply("two")]);
        let (mut app, agent) = super::super::tests::app(&disk, &model);
        submit(app.world_mut(), agent, "test", "one", "one").unwrap();
        until(&mut app, |w| {
            committed(w, agent)
                .is_some_and(|s| s.tasks[0].status == Status::Completed && s.active.is_none())
        });
        submit(app.world_mut(), agent, "test", "two", "two").unwrap();
        until(&mut app, |w| {
            committed(w, agent).is_some_and(|s| s.tasks[1].status == Status::Completed)
        });
        let mut state = inspect(app.world(), agent).unwrap().clone();
        state.wakes.push(WakeRecord {
            id: "wake:task:1".into(),
            generation: 0,
            task: "task:1".into(),
            task_generation: 0,
            created_ms: 1,
            deadline_ms: 2,
            reason: "two recipients".into(),
            phase: Phase::Fired,
            fired_ms: Some(3),
            cancelled_ms: None,
            step: Some(0),
            ready: true,
            delivered: Some(Delivery {
                task: "task:2".into(),
                step: 0,
            }),
        });
        let entries = sources(&state);
        let ids: std::collections::BTreeSet<_> = entries.iter().map(|s| &s.id).collect();
        assert_eq!(ids.len(), entries.len());
    }
    #[test]
    fn long_unicode_sources_are_covered_only_through_the_actual_requested_bytes() {
        let disk = MemoryStore::default();
        let model = MockCompletionModel::from_stream_turns([reply("done")]);
        let (mut app, agent) = super::super::tests::app(&disk, &model);
        submit(
            app.world_mut(),
            agent,
            "test",
            "long",
            &"中文long-source".repeat(1500),
        )
        .unwrap();
        until(&mut app, |w| {
            inspect(w, agent).unwrap().tasks[0].status == Status::Completed
                && !w.entity(agent).contains::<ActiveTurn>()
        });
        let durable = committed(app.world(), agent).unwrap().clone();
        let mut context = AssistantContextState::default();
        let mut spec = (*app.world().get::<Connection>(agent).unwrap().spec).clone();
        spec.context_window = Some(2048);
        let material = sources(&durable);
        let mut partial = false;
        for _ in 0..100 {
            let Some((request, spans)) = plan(&context, &durable, &spec, INSTRUCTIONS).unwrap()
            else {
                break;
            };
            assert!(rig_memory::estimate_tokens(&request.chat_history) < 2048);
            let payload = serde_json::to_string(&request).unwrap();
            for span in spans {
                let source = material.iter().find(|s| s.id == span.source).unwrap();
                assert_eq!(
                    span.start,
                    context.covered.get(&span.source).copied().unwrap_or(0)
                );
                assert!(
                    source.text.is_char_boundary(span.start)
                        && source.text.is_char_boundary(span.end)
                );
                assert_eq!(
                    span.digest,
                    digest(&source.text.get(span.start..span.end).unwrap())
                );
                assert!(payload.contains(&span.end.to_string()));
                partial |= span.end < source.text.len();
                context.covered.insert(span.source, span.end);
            }
        }
        assert!(partial);
        for source in material {
            assert_eq!(context.covered[&source.id], source.text.len());
        }
    }
    #[test]
    fn cancelled_call_with_a_late_success_cannot_publish() {
        let disk = MemoryStore::default();
        let model = MockCompletionModel::from_stream_turns([reply("LATE_REPLY")]);
        let (mut app, agent) = super::super::tests::app(&disk, &model);
        submit(app.world_mut(), agent, "test", "one", "input").unwrap();
        until(&mut app, |w| {
            w.get::<AssistantContextState>(agent)
                .is_some_and(|s| !s.jobs.is_empty())
        });
        let call = app
            .world_mut()
            .query::<(Entity, &RecapCall)>()
            .iter(app.world())
            .next()
            .unwrap()
            .0;
        stop(app.world_mut(), agent);
        let saved = committed(app.world(), agent).unwrap().operations[0]
            .outcome
            .clone()
            .unwrap();
        let late = FinalOutcome::from_value(saved)
            .unwrap()
            .effect_result()
            .unwrap()
            .unwrap();
        app.world_mut().entity_mut(call).insert(late);
        for _ in 0..12 {
            app.update();
        }
        let context = storage::committed_context(app.world(), agent).unwrap();
        assert_eq!(context.recap_revision, 0);
        assert!(context.covered.is_empty());
        assert_eq!(context.jobs[0].phase, "cancelled");
    }
}
