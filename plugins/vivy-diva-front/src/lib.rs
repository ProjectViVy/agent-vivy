//! Desktop transport on the shared Rig runner. No separate agent execution loop.
use rig_basics::print::{AppHeadlessExt, KeepRunning, Starting};
use rig_harness::prelude::*;
mod conversation;
mod files;
mod management;
mod projection;
pub mod protocol;
mod runtime_controls;
mod transport;
use protocol::*;
use serde_json::Value;
use std::io::{BufRead, Read};
use std::sync::{Mutex, mpsc};

#[derive(Resource)]
struct Input(Mutex<mpsc::Receiver<Result<String, String>>>);

#[derive(Component)]
struct AwaitingModel;

#[derive(Resource)]
struct Output {
    instance: String,
    sequence: u64,
    ready: bool,
    writer: transport::Writer,
    closing: Option<u64>,
}

#[derive(Default)]
pub struct DivaFrontPlugin;

impl Plugin for DivaFrontPlugin {
    fn build(&self, app: &mut App) {
        app.add_front(Front::new("diva").on_flag("diva", "Agent Diva desktop protocol"));
    }

    fn finish(&self, app: &mut App) {
        if !app.runs_front("diva") {
            return;
        }
        let (tx, rx) = mpsc::sync_channel(32);
        let wake = app.world().resource::<Wake>().clone();
        std::thread::spawn(move || {
            let mut reader = std::io::stdin().lock();
            loop {
                let mut bytes = Vec::new();
                let result = reader
                    .by_ref()
                    .take((MAX_FRAME + 1) as u64)
                    .read_until(b'\n', &mut bytes);
                let frame = match result {
                    Ok(0) => break,
                    Ok(_) if bytes.len() > MAX_FRAME => {
                        let _ = tx.send(Err("request frame exceeds 1 MiB".into()));
                        wake.wake();
                        break;
                    }
                    Ok(_) if !bytes.ends_with(b"\n") => {
                        let _ =
                            tx.send(Err("request frame is missing its newline terminator".into()));
                        wake.wake();
                        break;
                    }
                    Ok(_) => String::from_utf8(bytes).map_err(|_| "request is not UTF-8".into()),
                    Err(error) => {
                        let _ = tx.send(Err(format!("request pipe read failed: {error}")));
                        wake.wake();
                        break;
                    }
                };
                if tx.send(frame).is_err() {
                    break;
                }
                wake.wake();
            }
            drop(tx);
            wake.wake();
        });
        let writer =
            transport::Writer::start(std::io::stdout(), app.world().resource::<Wake>().clone());
        app.insert_resource(Input(Mutex::new(rx)))
            .insert_resource(Output {
                instance: format!("{}-{}", std::process::id(), rig_harness::rig_core::now_ms()),
                sequence: 0,
                ready: false,
                writer,
                closing: None,
            });
        app.world_mut().spawn(KeepRunning);
        app.world_mut().spawn(Answerer::default());
        app.init_resource::<conversation::Events>();
        app.init_resource::<files::Jobs>();
        app.init_resource::<vivy_python::managed::desktop::Desktop>();
        app.init_resource::<runtime_controls::Jobs>();
        // Management must remain available before first-run provider setup.
        app.world_mut().spawn((Starting, AwaitingModel));
        app.add_systems(Update, poll.before(AgentLoop));
        app.run_headless(None);
    }
}

fn emit(world: &mut World, id: Option<String>, payload: Payload) {
    let mut output = world.resource_mut::<Output>();
    output.sequence += 1;
    let frame = Frame {
        v: VERSION,
        instance: output.instance.clone(),
        seq: output.sequence,
        id,
        payload,
    };
    if let Err(error) = output.writer.send(frame) {
        eprintln!("{error}");
        drop(output);
        world.trigger(Exit(ExitReason::Quit));
    }
}

fn snapshot(world: &mut World) -> Snapshot {
    projection::snapshot(world, world.resource::<Output>().sequence + 1)
}

fn fault(world: &mut World, id: Option<String>, code: ErrorCode, message: impl Into<String>) {
    emit(
        world,
        id,
        Payload::Error(Fault {
            code,
            message: message.into(),
        }),
    );
}

fn poll(world: &mut World) {
    if !world.contains_resource::<SessionRestored>() {
        return;
    }
    let output = world.resource::<Output>();
    if let Some(error) = output.writer.failure() {
        eprintln!("{error}");
        world.trigger(Exit(ExitReason::Quit));
        return;
    }
    if let Some(sequence) = output.closing {
        if output.writer.written() >= sequence {
            world.trigger(Exit(ExitReason::Quit));
        }
        return;
    }
    if world
        .get_resource::<Models>()
        .is_some_and(|models| !models.0.reachable().is_empty())
    {
        let waiting: Vec<_> = world
            .query_filtered::<Entity, With<AwaitingModel>>()
            .iter(world)
            .collect();
        for entity in waiting {
            world.despawn(entity);
        }
    }
    if !world.resource::<Output>().ready {
        world.resource_mut::<Output>().ready = true;
        let state = snapshot(world);
        emit(world, None, Payload::Ready(state));
    }
    conversation::publish(world);
    files::poll(world);
    runtime_controls::poll(world);
    for _ in 0..32 {
        let line = world.resource::<Input>().0.lock().unwrap().try_recv();
        let line = match line {
            Ok(Ok(line)) => line,
            Ok(Err(error)) => {
                fault(world, None, ErrorCode::InvalidFrame, error);
                continue;
            }
            Err(mpsc::TryRecvError::Empty) => break,
            Err(mpsc::TryRecvError::Disconnected) => {
                let mut output = world.resource_mut::<Output>();
                output.closing = Some(output.sequence);
                world.resource::<Wake>().wake();
                break;
            }
        };
        let value = serde_json::from_str::<Value>(&line);
        let id = value
            .as_ref()
            .ok()
            .and_then(|v| v.get("id"))
            .filter(|v| v.as_str().is_some_and(|s| !s.is_empty() && s.len() <= 256))
            .and_then(Value::as_str)
            .map(str::to_owned);
        let code = match value.as_ref().ok() {
            Some(v)
                if v.get("v")
                    .and_then(Value::as_u64)
                    .is_some_and(|v| v != u64::from(VERSION)) =>
            {
                ErrorCode::IncompatibleVersion
            }
            Some(v)
                if v.get("method").and_then(Value::as_str).is_some_and(|m| {
                    !matches!(
                        m,
                        "snapshot"
                            | "shutdown"
                            | "submit"
                            | "admission"
                            | "cancel"
                            | "answer"
                            | "history"
                            | "providers_read"
                            | "providers_reload"
                            | "providers_save"
                            | "provider_key"
                            | "provider_test"
                            | "persona_read"
                            | "persona_save_file"
                            | "persona_rollback"
                            | "persona_initialize"
                            | "prompts_read"
                            | "prompt_save"
                            | "managed_read"
                            | "managed_open"
                            | "managed_import"
                            | "managed_edit"
                            | "managed_validate"
                            | "managed_cancel"
                            | "managed_dependencies"
                            | "managed_publish"
                            | "managed_rollback"
                            | "tasks_read"
                            | "wake_cancel"
                            | "wake_schedule"
                            | "resolve"
                            | "dream_request"
                            | "dream_cancel"
                            | "dream_detach"
                            | "dream_job_cancel"
                            | "mcp_read"
                            | "mcp_save"
                            | "mcp_control"
                            | "python_read"
                            | "python_prepare"
                            | "python_cancel"
                            | "skills_read"
                            | "skill_read"
                            | "skills_refresh"
                            | "skill_import"
                            | "skill_save"
                            | "skill_remove"
                            | "recap_read"
                            | "recap_refresh"
                    )
                }) =>
            {
                ErrorCode::UnsupportedOperation
            }
            _ => ErrorCode::InvalidRequest,
        };
        let request = value
            .ok()
            .and_then(|v| serde_json::from_value::<Request>(v).ok());
        let Some(request) =
            request.filter(|r| r.v == VERSION && !r.id.is_empty() && r.id.len() <= 256)
        else {
            fault(
                world,
                id,
                code,
                "Expected protocol v1, a bounded nonempty id and a supported method",
            );
            continue;
        };
        match request.method {
            Method::Snapshot => {
                let state = snapshot(world);
                emit(world, id, Payload::Reply(Reply::Snapshot(state)));
            }
            Method::Shutdown => {
                emit(
                    world,
                    id,
                    Payload::Reply(Reply::Shutdown {
                        shutting_down: true,
                    }),
                );
                let mut output = world.resource_mut::<Output>();
                output.closing = Some(output.sequence);
                break;
            }
            method => {
                let result = match method {
                    Method::Submit(input) => conversation::submit(world, input, false),
                    Method::Admission(input) => conversation::submit(world, input, true),
                    Method::Cancel(input) => conversation::cancel(world, input, request.id.clone()),
                    Method::Answer(input) => {
                        let instance = world.resource::<Output>().instance.clone();
                        conversation::answer(world, input, &instance)
                    }
                    Method::History(input) => conversation::history(world, input),
                    Method::ProvidersRead => Ok(management::providers(world)),
                    Method::ProvidersReload => management::reload(world),
                    Method::ProvidersSave(input) => management::save(world, input),
                    Method::ProviderKey(input) => management::key(world, input),
                    Method::ProviderTest(input) => {
                        runtime_controls::provider(world, request.id.clone(), input)
                    }
                    Method::PersonaInitialize(input) => {
                        runtime_controls::persona(world, request.id.clone(), input)
                    }
                    method @ (Method::TasksRead
                    | Method::WakeCancel(_)
                    | Method::WakeSchedule(_)
                    | Method::Resolve(_)
                    | Method::DreamRequest(_)
                    | Method::DreamCancel(_)
                    | Method::DreamDetach(_)
                    | Method::DreamJobCancel(_)) => {
                        runtime_controls::durable(world, request.id.clone(), method)
                    }
                    method @ (Method::PythonRead
                    | Method::PythonPrepare(_)
                    | Method::PythonCancel) => {
                        runtime_controls::python(world, request.id.clone(), method)
                    }
                    Method::RecapRead => files::recap(world, false),
                    Method::RecapRefresh => files::recap(world, true),
                    method @ (Method::ManagedRead
                    | Method::ManagedOpen(_)
                    | Method::ManagedImport(_)
                    | Method::ManagedEdit(_)
                    | Method::ManagedValidate(_)
                    | Method::ManagedCancel(_)
                    | Method::ManagedDependencies(_)
                    | Method::ManagedPublish(_)
                    | Method::ManagedRollback(_)
                    | Method::McpRead
                    | Method::McpSave(_)
                    | Method::McpControl(_)
                    | Method::SkillsRead
                    | Method::SkillRead(_)
                    | Method::SkillsRefresh
                    | Method::SkillImport(_)
                    | Method::SkillSave(_)
                    | Method::SkillRemove(_)
                    | Method::PersonaRead
                    | Method::PersonaSaveFile(_)
                    | Method::PersonaRollback(_)
                    | Method::PromptsRead
                    | Method::PromptSave(_)) => files::start(world, request.id.clone(), method),
                    _ => unreachable!(),
                };
                emit(
                    world,
                    id,
                    match result {
                        Ok(reply) => Payload::Reply(reply),
                        Err(error) => Payload::Error(error),
                    },
                );
            }
        }
    }
}
