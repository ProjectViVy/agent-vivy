//! A diagnostic write gate, not a durable commit gate. It waits for an exact
//! saved checkpoint in an ordinary Store append. Review and settlement are
//! observed without pretending that the kernel exposes a final-result receipt.
use rig_harness::host::session::StoreOpener;
use rig_harness::prelude::*;
use rig_harness::rig_ecs::session_log::SessionStore;
use rig_harness::rig_ecs::{SaveSystems, WriteLogs};
use serde::{Deserialize, Serialize};
use serde_json::{Value, json};
use sha2::{Digest, Sha256};
use std::sync::{Arc, Mutex};

mod store;
use store::ObservedStore;

pub(super) const KEY: &str = "vivy::d0_probe/1";
const MAX_OPERATIONS: usize = 256;

#[derive(Component, Reflect, Serialize, Deserialize, Clone, Default, Debug)]
#[reflect(opaque)]
#[reflect(Component, Saved, Serialize, Deserialize)]
pub(super) struct ProbeState {
    pub revision: u64,
    pub generation: u64,
    pub operations: Vec<Checkpoint>,
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, Eq)]
pub(super) struct Checkpoint {
    pub operation: String,
    pub kind: String,
    pub digest: String,
    pub revision: u64,
    pub generation: u64,
}

#[derive(Resource, Clone, Default)]
pub(super) struct Trace(Arc<Mutex<TraceData>>);

#[derive(Default)]
struct TraceData {
    events: Vec<Value>,
    // Diagnostic acknowledgements of ordinary appends. Never disk receipts.
    written: Vec<(String, Checkpoint)>,
    failure: Option<String>,
    path: Option<std::path::PathBuf>,
    frame: u64,
}

impl Trace {
    fn event(&self, mut event: Value) {
        use std::io::Write;
        let mut data = self.0.lock().unwrap_or_else(|e| e.into_inner());
        if data.failure.is_some() {
            return;
        }
        if data.events.len() >= 4096 {
            data.failure = Some("D0 trace limit reached".into());
            return;
        }
        event["sequence"] = json!(data.events.len());
        event["frame"] = json!(data.frame);
        event["durable"] = json!(false);
        if let Some(path) = &data.path {
            let result = (|| -> std::io::Result<()> {
                let mut out = std::fs::OpenOptions::new()
                    .create(true)
                    .append(true)
                    .open(path)?;
                serde_json::to_writer(&mut out, &event)?;
                out.write_all(b"\n")
            })();
            if let Err(error) = result {
                data.failure = Some(format!("D0 trace write failed: {error}"));
                return;
            }
        }
        data.events.push(event);
    }

    fn written(&self, log: &str, checkpoint: &Checkpoint) -> bool {
        let data = self.0.lock().unwrap_or_else(|e| e.into_inner());
        data.failure.is_none()
            && data
                .written
                .iter()
                .any(|(id, cp)| id == log && cp == checkpoint)
    }
}

#[derive(Component)]
struct ProbeHold(Entity);

pub(super) fn digest(value: &impl Serialize) -> Result<String, serde_json::Error> {
    Ok(format!("{:x}", Sha256::digest(serde_json::to_vec(value)?)))
}

/// Diagnostic plugin for disposable, synthetic sessions only.
#[derive(Default)]
pub struct D0ProbePlugin;

impl Plugin for D0ProbePlugin {
    fn build(&self, app: &mut App) {
        let trace = app
            .world()
            .get_resource::<Trace>()
            .cloned()
            .unwrap_or_default();
        if let Some(path) = std::env::var_os("VIVY_D0_TRACE") {
            trace.0.lock().unwrap_or_else(|e| e.into_inner()).path = Some(path.into());
        }
        if let Some(SessionStore(inner)) = app.world_mut().remove_resource::<SessionStore>() {
            app.insert_resource(SessionStore::new(ObservedStore::new(inner, trace.clone())));
        } else {
            let opener = app
                .world()
                .get_resource::<StoreOpener>()
                .cloned()
                .unwrap_or_default();
            let observed = trace.clone();
            app.insert_resource(StoreOpener::new(move |dir| {
                Ok(Arc::new(ObservedStore::new(
                    opener.open(dir)?,
                    observed.clone(),
                )))
            }));
        }
        app.insert_resource(trace)
            .register_type::<ProbeState>()
            .init_resource::<SavedKeys>();
        app.world_mut()
            .resource_mut::<SavedKeys>()
            .add(std::any::type_name::<ProbeState>(), KEY)
            .expect("the D0 saved key must be unique");
        app.add_systems(First, |trace: Res<Trace>| {
            trace.0.lock().unwrap_or_else(|e| e.into_inner()).frame += 1;
        })
        // A quiet AgentLoop does not run Gate merely because Store wrote.
        // Release in the next frame; removing HoldOf wakes the native loop.
        .add_systems(First, gate)
        .add_systems(AgentPass, gate.in_set(GateSystems::After))
        .add_systems(AgentPass, review.in_set(ReviewSystems::Act))
        .add_systems(Last, checkpoint_frame.after(SaveSystems).before(WriteLogs))
        .add_observer(started)
        .add_observer(settled);
    }
}

/// Includes our own held effects, which ReadyToStart deliberately excludes.
fn gate(world: &mut World) {
    let trace = world.resource::<Trace>().clone();
    let mut query = world.query_filtered::<(
        Entity,
        &ForAgent,
        &Effect,
        Option<&ToolRequest>,
        Option<&ModelRequest>,
        Option<&Adapted>,
        Option<&Incoming>,
    ), (
        With<Admitted>,
        Without<Started>,
        Without<Outcome>,
        Without<Settled>,
        Without<Waiting>,
        Without<SupersededBy>,
        Without<Interrupted>,
    )>();
    let pending: Vec<_> = query
        .iter(world)
        .filter_map(|(call, of, effect, tool, model, adapted, input)| {
            let (kind, digest) = if let Some(tool) = tool {
                ("tool", digest(&tool.call))
            } else if let (Some(model), Some(adapted)) = (model, adapted) {
                // The payload's model override is usually None; routing lives
                // in the connection. This binds the public catalog identity,
                // not the opaque handler's endpoint or credential revision.
                (
                    "model",
                    digest(&(
                        &adapted.0.spec.provider,
                        &adapted.0.spec.id,
                        adapted.sent(model),
                    )),
                )
            } else if let Some(input) = input {
                ("input", digest(input))
            } else {
                return None;
            };
            Some((call, of.0, effect.id().to_string(), kind, digest))
        })
        .collect();
    for (call, agent, operation, kind, digest) in pending {
        let held = world.get::<ProbeHold>(call).map(|held| held.0);
        let hold = held.unwrap_or_else(|| {
            let hold = world
                .spawn((
                    HoldOf(call),
                    Name::new("D0: waiting for observed append (not synced)"),
                ))
                .id();
            world.entity_mut(call).insert(ProbeHold(hold));
            hold
        });
        let Ok(digest) = digest else {
            trace.event(json!({"event":"request_serialization_failed","operation":operation}));
            continue;
        };
        let Some(id) = world.get::<AgentId>(agent).map(|id| id.0.clone()) else {
            continue;
        };
        let mut state = world.get::<ProbeState>(agent).cloned().unwrap_or_default();
        let previous = state.operations.iter().find(|cp| cp.operation == operation);
        let checkpoint = match previous {
            Some(cp) if cp.digest == digest && cp.generation == state.generation => cp.clone(),
            _ => {
                if state.operations.len() >= MAX_OPERATIONS {
                    trace.event(json!({"event":"operation_limit","agent":id}));
                    continue;
                }
                state.revision += 1;
                let checkpoint = Checkpoint {
                    operation: operation.clone(),
                    kind: kind.into(),
                    digest,
                    revision: state.revision,
                    generation: state.generation,
                };
                state.operations.retain(|cp| cp.operation != operation);
                state.operations.push(checkpoint.clone());
                world.entity_mut(agent).insert(state);
                trace.event(
                    json!({"event":"checkpoint_prepared","agent":id,"checkpoint":checkpoint}),
                );
                checkpoint
            }
        };
        if trace.written(&id, &checkpoint) {
            trace.event(json!({"event":"write_gate_released","agent":id,"checkpoint":checkpoint}));
            world.despawn(hold);
            world.entity_mut(call).remove::<ProbeHold>();
        }
    }
}

fn started(
    event: On<Add<Started>>,
    calls: Query<(&Effect, &ForAgent)>,
    agents: Query<&AgentId>,
    trace: Res<Trace>,
) {
    if let Ok((effect, of)) = calls.get(event.entity) {
        trace.event(json!({"event":"started","operation":effect.id().to_string(),"agent":agents.get(of.0).ok().map(|id| &id.0)}));
    }
}

fn review(
    calls: Query<(&Effect, &Outcome), (With<ForAgent>, Changed<Outcome>, Without<Settled>)>,
    trace: Res<Trace>,
) {
    for (effect, outcome) in &calls {
        trace.event(json!({"event":"review","operation":effect.id().to_string(),"digest":digest(&outcome.0).ok()}));
    }
}

fn settled(
    event: On<Add<Settled>>,
    calls: Query<(&Effect, Option<&Outcome>, Option<&TurnOutcome>)>,
    trace: Res<Trace>,
) {
    if let Ok((effect, outcome, turn)) = calls.get(event.entity) {
        trace.event(
            json!({"event":"settled","operation":effect.id().to_string(),
            "digest":outcome.and_then(|outcome| digest(&outcome.0).ok()),"turn":turn.is_some()}),
        );
    }
}

fn checkpoint_frame(trace: Res<Trace>) {
    trace.event(json!({"event":"before_write_logs"}));
}

#[cfg(test)]
mod tests;
