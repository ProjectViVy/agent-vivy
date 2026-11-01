//! Desktop transport on the shared Rig runner. No separate agent execution loop.
use rig_basics::print::{AppHeadlessExt, KeepRunning, Starting};
use rig_harness::prelude::*;
mod projection;
pub mod protocol;
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
                if v.get("method")
                    .and_then(Value::as_str)
                    .is_some_and(|m| !matches!(m, "snapshot" | "shutdown")) =>
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
        }
    }
}
