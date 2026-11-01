//! Desktop transport on the shared Rig runner. No separate agent execution loop.
use rig_basics::print::{AppHeadlessExt, KeepRunning, Starting};
use rig_harness::prelude::*;
use serde::Deserialize;
use serde_json::{Value, json};
use std::io::{BufRead, Read, Write};
use std::sync::{Mutex, mpsc};

const VERSION: u32 = 1;
const MAX_FRAME: usize = 1024 * 1024;

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct Request {
    v: u32,
    id: String,
    method: Method,
}

#[derive(Deserialize)]
#[serde(rename_all = "snake_case")]
enum Method {
    Snapshot,
    Shutdown,
}

#[derive(Resource)]
struct Input(Mutex<mpsc::Receiver<Result<String, String>>>);

#[derive(Component)]
struct AwaitingModel;

#[derive(Resource)]
struct Output {
    instance: String,
    sequence: u64,
    ready: bool,
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
        app.insert_resource(Input(Mutex::new(rx)))
            .insert_resource(Output {
                instance: format!("{}-{}", std::process::id(), rig_harness::rig_core::now_ms()),
                sequence: 0,
                ready: false,
            });
        app.world_mut().spawn(KeepRunning);
        // Management must remain available before first-run provider setup.
        app.world_mut().spawn((Starting, AwaitingModel));
        app.add_systems(Update, poll.before(AgentLoop));
        app.run_headless(None);
    }
}

fn emit(world: &mut World, kind: &str, id: Value, data: Value) {
    let mut output = world.resource_mut::<Output>();
    output.sequence += 1;
    let frame = json!({"v":VERSION,"instance":output.instance,"seq":output.sequence,
        "type":kind,"id":id,"data":data});
    let mut stdout = std::io::stdout().lock();
    if writeln!(stdout, "{frame}")
        .and_then(|_| stdout.flush())
        .is_err()
    {
        drop(output);
        world.trigger(Exit(ExitReason::Quit));
    }
}

fn snapshot(world: &mut World) -> Value {
    let session = world
        .get_resource::<SessionPaths>()
        .and_then(|paths| paths.0.id())
        .map(|id| id.as_str().to_owned());
    let mut plugins: Vec<_> = world
        .query::<(&Name, &PluginSource)>()
        .iter(world)
        .map(|(name, source)| json!({"name":name.as_str(),"package":source.package}))
        .collect();
    plugins.sort_by_key(|p| p["name"].as_str().unwrap_or_default().to_owned());
    let agents: Vec<_> = world
        .query::<(&AgentId, Has<ActiveTurn>)>()
        .iter(world)
        .map(|(id, working)| json!({"id":id.0,"working":working}))
        .collect();
    json!({"session":session,"plugins":plugins,"agents":agents,
        "core_version":env!("CARGO_PKG_VERSION"),"protocol_version":VERSION})
}

fn poll(world: &mut World) {
    if !world.contains_resource::<SessionRestored>() {
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
        emit(world, "ready", Value::Null, state);
    }
    for _ in 0..32 {
        let line = world.resource::<Input>().0.lock().unwrap().try_recv();
        let line = match line {
            Ok(Ok(line)) => line,
            Ok(Err(error)) => {
                emit(
                    world,
                    "error",
                    Value::Null,
                    json!({"code":"invalid_frame","message":error}),
                );
                continue;
            }
            Err(mpsc::TryRecvError::Empty) => break,
            Err(mpsc::TryRecvError::Disconnected) => {
                world.trigger(Exit(ExitReason::Quit));
                break;
            }
        };
        let value = serde_json::from_str::<Value>(&line);
        let id = value
            .as_ref()
            .ok()
            .and_then(|v| v.get("id"))
            .filter(|v| v.as_str().is_some_and(|s| !s.is_empty() && s.len() <= 256))
            .cloned()
            .unwrap_or(Value::Null);
        let code = match value.as_ref().ok() {
            Some(v)
                if v.get("v")
                    .and_then(Value::as_u64)
                    .is_some_and(|v| v != u64::from(VERSION)) =>
            {
                "incompatible_version"
            }
            Some(v)
                if v.get("method")
                    .and_then(Value::as_str)
                    .is_some_and(|m| !matches!(m, "snapshot" | "shutdown")) =>
            {
                "unsupported_operation"
            }
            _ => "invalid_request",
        };
        let request = value
            .ok()
            .and_then(|v| serde_json::from_value::<Request>(v).ok());
        let Some(request) =
            request.filter(|r| r.v == VERSION && !r.id.is_empty() && r.id.len() <= 256)
        else {
            emit(
                world,
                "error",
                id,
                json!({"code":code,"message":"Expected protocol v1, a bounded nonempty id and a supported method"}),
            );
            continue;
        };
        match request.method {
            Method::Snapshot => {
                let state = snapshot(world);
                emit(world, "reply", id, state);
            }
            Method::Shutdown => {
                emit(world, "reply", id, json!({"shutting_down":true}));
                world.trigger(Exit(ExitReason::Quit));
                break;
            }
        }
    }
}
