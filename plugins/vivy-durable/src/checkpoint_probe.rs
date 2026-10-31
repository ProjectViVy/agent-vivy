//! Acceptance consumer of the opt-in Rig checkpoint and final-outcome seams.

use rig_harness::rig_ecs::{
    SaveSystems, WriteLogs,
    session_log::{SessionLog, SessionStore},
    stage::{CommitBarrier, FinalOutcome, PersistOutcomes},
};
use rig_harness::{
    host::session::{SessionPaths, StoreOpener},
    prelude::*,
};
use rig_memory::CheckpointStore;
use serde::{Deserialize, Serialize};
use serde_json::Value;
use sha2::{Digest, Sha256};
use std::{
    collections::BTreeMap,
    path::PathBuf,
    sync::{Arc, Mutex},
};

const KEY: &str = "vivy.checkpoint_probe.v1";

#[derive(Component, Reflect, Clone, Default, Debug, Serialize, Deserialize, PartialEq)]
#[reflect(opaque, Component, Saved, Serialize, Deserialize)]
struct State {
    revision: u64,
    generation: u64,
    conversation_revision: u64,
    intents: BTreeMap<String, String>,
    outcomes: BTreeMap<String, Value>,
}

#[derive(Resource, Default)]
struct Committed(BTreeMap<String, State>);

#[derive(Resource, Default, Clone)]
struct Stores(Arc<Mutex<BTreeMap<PathBuf, CheckpointStore>>>);

#[derive(Resource, Default)]
struct Evidence {
    sequence: u64,
    path: Option<PathBuf>,
    failed: bool,
    seen: BTreeMap<String, (rig_harness::rig_core::conversation::Lineage, u64)>,
}

#[derive(Component)]
struct ProbeHold(Entity);

/// Diagnostic checkpoint consumer for disposable synthetic sessions. It has no
/// task lifecycle or automatic crash continuation; the D1 runtime is separate.
#[derive(Default)]
pub struct CheckpointProbePlugin;

impl Plugin for CheckpointProbePlugin {
    fn build(&self, app: &mut App) {
        let stores = Stores::default();
        if let Some(SessionStore(inner)) = app.world_mut().remove_resource::<SessionStore>() {
            let store = CheckpointStore::new(inner);
            stores
                .0
                .lock()
                .unwrap()
                .insert(PathBuf::new(), store.clone());
            app.insert_resource(SessionStore::new(store));
        } else {
            let opener = app
                .world()
                .get_resource::<StoreOpener>()
                .cloned()
                .unwrap_or_default();
            let captured = stores.clone();
            app.insert_resource(StoreOpener::new(move |dir| {
                let store = CheckpointStore::new(opener.open(dir)?);
                captured
                    .0
                    .lock()
                    .unwrap()
                    .insert(dir.path().to_path_buf(), store.clone());
                Ok(Arc::new(store))
            }));
        }
        app.insert_resource(stores)
            .init_resource::<Committed>()
            .insert_resource(Evidence {
                path: std::env::var_os("VIVY_CHECKPOINT_TRACE").map(Into::into),
                ..Default::default()
            })
            .register_type::<State>()
            .init_resource::<SavedKeys>();
        app.world_mut()
            .resource_mut::<SavedKeys>()
            .require(std::any::type_name::<State>(), KEY, 1)
            .expect("unique checkpoint probe schema");
        app.add_observer(|added: On<Add<Agent>>, mut commands: Commands| {
            commands
                .entity(added.entity)
                .insert(PersistOutcomes)
                .insert_if_new(State::default());
        })
        .add_systems(First, (gate, acknowledge).chain())
        .add_systems(AgentPass, gate.in_set(GateSystems::After))
        .add_systems(AgentPass, capture.in_set(Stage::Finalize))
        .add_systems(Last, boundary.in_set(SaveSystems))
        .add_systems(Last, receipts.after(WriteLogs))
        .add_observer(
            |added: On<Add<Started>>,
             effects: Query<(&Effect, Has<Incoming>, Has<ModelRequest>, Has<ToolRequest>)>,
             mut evidence: ResMut<Evidence>| {
                if let Ok((effect, input, model, tool)) = effects.get(added.entity) {
                    let kind = if input {
                        "started_input"
                    } else if model {
                        "started_model"
                    } else if tool {
                        "started_tool"
                    } else {
                        "started_other"
                    };
                    evidence.record(kind, &effect.id().to_string(), 0);
                }
            },
        )
        .add_observer(
            |added: On<Add<Settled>>, effects: Query<&Effect>, mut evidence: ResMut<Evidence>| {
                if let Ok(effect) = effects.get(added.entity) {
                    evidence.record("settled", &effect.id().to_string(), 0);
                }
            },
        );
    }
}

impl Evidence {
    fn record(&mut self, event: &str, operation: &str, revision: u64) {
        use std::io::Write;
        self.sequence += 1;
        if let Some(path) = &self.path {
            let result = (|| -> std::io::Result<()> {
                let mut file = std::fs::OpenOptions::new()
                    .create(true)
                    .append(true)
                    .open(path)?;
                serde_json::to_writer(
                    &mut file,
                    &serde_json::json!({"sequence":self.sequence,"event":event,"operation":operation,"revision":revision,"process_crash_receipt":event == "checkpoint_committed"}),
                )?;
                file.write_all(b"\n")
            })();
            self.failed |= result.is_err();
        }
    }
}

fn active_store(world: &World) -> Option<CheckpointStore> {
    let stores = world.resource::<Stores>().0.lock().unwrap();
    let key = world
        .get_resource::<SessionPaths>()
        .map(|p| p.path().to_path_buf())
        .unwrap_or_default();
    stores
        .get(&key)
        .or_else(|| stores.get(&PathBuf::new()))
        .cloned()
}

fn healthy(world: &World) -> bool {
    !world.resource::<Evidence>().failed
        && world.resource::<SessionLog>().failure().is_none()
        && active_store(world).is_some_and(|s| s.failure().is_none())
}

fn hash(value: &impl Serialize) -> Option<String> {
    serde_json::to_vec(value)
        .ok()
        .map(|bytes| format!("{:x}", Sha256::digest(bytes)))
}

fn gate(world: &mut World) {
    let pending: Vec<_> = world
        .query_filtered::<(
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
        )>()
        .iter(world)
        .filter_map(|(call, of, effect, tool, model, adapted, input)| {
            let digest = if let Some(tool) = tool {
                hash(&tool.call)
            } else if let (Some(model), Some(adapted)) = (model, adapted) {
                hash(&(
                    &adapted.0.spec.provider,
                    &adapted.0.spec.id,
                    adapted.sent(model),
                ))
            } else {
                input.and_then(hash)
            }?;
            Some((call, of.0, effect.id().to_string(), digest))
        })
        .collect();
    for (call, agent, operation, digest) in pending {
        let Some(mut state) = world.get::<State>(agent).cloned() else {
            continue;
        };
        let Some(id) = world.get::<AgentId>(agent).map(|id| id.0.clone()) else {
            continue;
        };
        let hold = world
            .get::<ProbeHold>(call)
            .map(|h| h.0)
            .unwrap_or_else(|| {
                let hold = world.spawn(HoldOf(call)).id();
                world.entity_mut(call).insert(ProbeHold(hold));
                hold
            });
        if state.intents.get(&operation) != Some(&digest) {
            state.intents.insert(operation.clone(), digest.clone());
            state.revision += 1;
            world.entity_mut(agent).insert(state.clone());
        }
        let committed = world.resource::<Committed>().0.get(&id);
        if healthy(world)
            && committed.is_some_and(|c| {
                c.generation == state.generation && c.intents.get(&operation) == Some(&digest)
            })
        {
            world
                .resource_mut::<Evidence>()
                .record("intent_released", &operation, state.revision);
            world.despawn(hold);
            world.entity_mut(call).remove::<ProbeHold>();
        }
    }
}

fn key(world: &World, entity: Entity) -> String {
    world
        .get::<Effect>(entity)
        .map(|e| e.id().to_string())
        .unwrap_or_else(|| format!("diagnostic-turn:{entity:?}"))
}

fn capture(world: &mut World) {
    let results: Vec<_> = world
        .query_filtered::<(Entity, &ForAgent, &FinalOutcome), Without<Settled>>()
        .iter(world)
        .map(|(e, of, value)| (e, of.0, value.value().clone()))
        .collect();
    for (call, agent, result) in results {
        let operation = key(world, call);
        if let Some(mut state) = world.get_mut::<State>(agent) {
            if state.outcomes.get(&operation) != Some(&result) {
                state.outcomes.insert(operation, result);
                state.revision += 1;
            }
        }
    }
}

fn acknowledge(world: &mut World) {
    if !healthy(world) {
        return;
    }
    let results: Vec<_> = world
        .query_filtered::<(Entity, &ForAgent, &FinalOutcome), Without<Settled>>()
        .iter(world)
        .map(|(e, of, value)| (e, of.0, value.clone()))
        .collect();
    for (call, agent, result) in results {
        let operation = key(world, call);
        let Some(id) = world.get::<AgentId>(agent).map(|id| id.0.clone()) else {
            continue;
        };
        if world
            .resource::<Committed>()
            .0
            .get(&id)
            .is_some_and(|state| state.outcomes.get(&operation) == Some(result.value()))
        {
            world
                .entity_mut(call)
                .insert(CommitBarrier::acknowledged(&result));
            if let Some(hold) = world.get::<ProbeHold>(call).map(|hold| hold.0) {
                world.despawn(hold);
                world.entity_mut(call).remove::<ProbeHold>();
            }
        }
    }
}

fn boundary(mut agents: Query<(&Conversation, &mut State)>) {
    for (conversation, mut state) in &mut agents {
        if state.conversation_revision != conversation.revision() {
            state.conversation_revision = conversation.revision();
            state.revision += 1;
        }
    }
}

fn receipts(world: &mut World) {
    if !healthy(world) {
        return;
    }
    let Some(store) = active_store(world) else {
        return;
    };
    let ids: Vec<_> = world
        .query::<&AgentId>()
        .iter(world)
        .map(|id| id.0.clone())
        .collect();
    for id in ids {
        let Some(receipt) = store.receipt(&id) else {
            continue;
        };
        let identity = (receipt.lineage, receipt.sequence);
        if world.resource::<Evidence>().seen.get(&id) == Some(&identity) {
            continue;
        }
        for line in receipt
            .bytes
            .split(|b| *b == b'\n')
            .filter(|l| !l.is_empty())
        {
            let Ok(value) = serde_json::from_slice::<Value>(line) else {
                continue;
            };
            if value["type"] == "state" && value["key"] == KEY {
                if let Ok(state) = serde_json::from_value::<State>(value["value"].clone()) {
                    world.resource_mut::<Evidence>().record(
                        "checkpoint_committed",
                        &id,
                        state.revision,
                    );
                    world
                        .resource_mut::<Committed>()
                        .0
                        .insert(id.clone(), state);
                }
            }
        }
        world.resource_mut::<Evidence>().seen.insert(id, identity);
        world.resource::<Wake>().wake();
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use rig_harness_test_support::{MemoryStore, app_on, first_agent, start_call};
    use rig_memory::Store;
    use serde_json::json;

    #[test]
    fn saved_model_response_is_reinjected_after_reopen_without_provider_dispatch() {
        use rig_core::test_utils::MockCompletionModel;
        use rig_harness_test_support::{connect, reply};
        let (mut app, agent, disk) = setup();
        let model = MockCompletionModel::from_stream_turns([reply("SAVED_MODEL_RESPONSE")]);
        app.world_mut()
            .entity_mut(agent)
            .insert(connect(&model).unwrap());
        app.world_mut()
            .trigger(Deliver::new(agent, "go", DeliveryMode::Steer));
        let deadline = std::time::Instant::now() + std::time::Duration::from_secs(10);
        let saved = loop {
            app.update();
            let id = app.world().get::<AgentId>(agent).unwrap().0.clone();
            let saved = app
                .world()
                .resource::<Committed>()
                .0
                .get(&id)
                .and_then(|s| s.outcomes.values().find(|v| v.get("effect").is_some()))
                .cloned();
            if let Some(saved) = saved {
                break saved;
            }
            assert!(std::time::Instant::now() < deadline);
            std::thread::sleep(std::time::Duration::from_millis(2));
        };
        // Stop before the next frame can acknowledge and publish the response.
        assert!(
            !app.world()
                .get::<Conversation>(agent)
                .unwrap()
                .messages()
                .iter()
                .any(|m| m.final_answer().is_some())
        );
        drop(app);
        let replay = FinalOutcome::from_value(saved)
            .unwrap()
            .effect_result()
            .unwrap()
            .unwrap();
        let empty = MockCompletionModel::from_stream_turns([reply("MUST_NOT_CALL")]);
        let (mut reopened, _) = app_on(&disk);
        reopened.add_plugins((AgentPlugin, SessionLogPlugin, CheckpointProbePlugin));
        let connection = connect(&empty).unwrap();
        reopened.add_observer(move |added: On<Add<Agent>>, mut commands: Commands| {
            commands.entity(added.entity).insert(connection.clone());
        });
        reopened.add_systems(
            AgentPass,
            (move |mut commands: Commands,
                   turns: Query<Entity, With<Restored>>,
                   mut calls: Query<&mut Decision, With<TurnCall>>| {
                for turn in &turns {
                    commands.entity(turn).insert(CarryOn);
                }
                for mut decision in &mut calls {
                    decision.decide(Decider::plugin("saved-result"), replay.clone());
                }
            })
            .in_set(Stage::Gate),
        );
        reopened.finish();
        for _ in 0..12 {
            reopened.update();
        }
        let agent = first_agent(reopened.world_mut()).unwrap();
        assert!(
            reopened
                .world()
                .get::<Conversation>(agent)
                .unwrap()
                .messages()
                .iter()
                .any(|m| m.final_answer().is_some())
        );
        assert_eq!(empty.request_count(), 0);
        assert_eq!(model.request_count(), 1);
    }

    fn setup() -> (App, Entity, MemoryStore) {
        let disk = MemoryStore::default();
        let (mut app, _) = app_on(&MemoryStore::default());
        app.insert_resource(SessionStore::new(disk.clone()));
        app.add_plugins((AgentPlugin, SessionLogPlugin, CheckpointProbePlugin));
        app.add_open_tool(
            "probe",
            "Test operation",
            ToolOptions::default(),
            |_: On<ToolCalled<Value>>| {},
        );
        app.finish();
        app.update();
        let agent = first_agent(app.world_mut()).unwrap();
        (app, agent, disk)
    }

    #[test]
    fn intent_and_result_are_committed_before_dispatch_and_settlement() {
        let (mut app, agent, _) = setup();
        let call = start_call(app.world_mut(), agent, ("probe", "p1"), json!({"arg":1})).unwrap();
        for _ in 0..5 {
            app.world_mut().run_schedule(AgentPass);
        }
        assert!(app.world().get::<Started>(call).is_none());
        app.update();
        app.update();
        assert!(app.world().get::<Started>(call).is_some());
        app.world_mut()
            .entity_mut(call)
            .insert(Outcome::custom(json!("done")));
        for _ in 0..5 {
            app.world_mut().run_schedule(AgentPass);
        }
        assert!(app.world().get::<Settled>(call).is_none());
        app.update();
        let id = app.world().get::<AgentId>(agent).unwrap().0.clone();
        let committed = app.world().resource::<Committed>().0.get(&id).unwrap();
        assert!(
            committed
                .outcomes
                .values()
                .any(|v| v.to_string().contains("done"))
        );
        app.update();
        assert!(app.world().get::<Settled>(call).is_some());
    }

    #[test]
    fn a_changed_request_cannot_use_the_previous_intent_receipt() {
        let (mut app, agent, _) = setup();
        let call = start_call(app.world_mut(), agent, ("probe", "p1"), json!({"arg":1})).unwrap();
        app.update();
        app.world_mut()
            .get_mut::<ToolRequest>(call)
            .unwrap()
            .call
            .function
            .arguments
            .insert("arg".into(), json!(2));
        app.world_mut().run_schedule(First);
        assert!(app.world().get::<Started>(call).is_none());
        assert!(app.world().get::<ProbeHold>(call).is_some());
    }

    #[test]
    fn the_checkpoint_contains_the_final_redacted_tool_result() {
        let (mut app, agent, disk) = setup();
        let call = start_call(app.world_mut(), agent, ("probe", "redact"), json!({})).unwrap();
        app.update();
        app.update();
        let secret = "API_KEY=synthetic-secret-that-must-not-be-saved";
        app.world_mut().entity_mut(call).insert(Outcome::from(
            rig_core::tool::ToolResult::success(rig_core::tool::ToolOutput::text(secret)),
        ));
        app.update();
        let id = app.world().get::<AgentId>(agent).unwrap().0.clone();
        let saved = serde_json::to_string(&app.world().resource::<Committed>().0[&id]).unwrap();
        assert!(saved.contains("[REDACTED]"));
        assert!(!saved.contains("synthetic-secret-that-must-not-be-saved"));
        let logical = CheckpointStore::new(std::sync::Arc::new(disk))
            .read_log(&id)
            .unwrap();
        assert!(
            !String::from_utf8(logical)
                .unwrap()
                .contains("synthetic-secret-that-must-not-be-saved")
        );
    }

    #[test]
    fn a_committed_effect_snapshot_decodes_without_a_handler() {
        let value = json!({"effect":{"Ok":{"kind":"custom","payload":"saved"}},"superseded":false,"cancelled":false});
        // Use the kernel serializer rather than guessing a provider response shape.
        let (mut app, agent, disk) = setup();
        let call = start_call(app.world_mut(), agent, ("probe", "restore"), json!({})).unwrap();
        app.update();
        app.update();
        app.world_mut()
            .entity_mut(call)
            .insert(Outcome::custom(value));
        app.update();
        let id = app.world().get::<AgentId>(agent).unwrap().0.clone();
        let saved = app.world().resource::<Committed>().0[&id]
            .outcomes
            .values()
            .next()
            .unwrap()
            .clone();
        let restored = FinalOutcome::from_value(saved)
            .unwrap()
            .effect_result()
            .unwrap()
            .unwrap();
        assert!(restored.0.is_ok());
        assert!(!disk.read_log(&id).unwrap().is_empty());
    }
}
