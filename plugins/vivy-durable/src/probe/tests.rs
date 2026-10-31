use super::*;
use rig_core::test_utils::MockCompletionModel;
use rig_harness_test_support::{MemoryStore, app_on, first_agent, start_call};
use rig_harness_test_support::{calls, connect, reply};
use rig_memory::{Files, LogWriteError, Session, Store};
use serde_json::json;
use std::{
    io,
    sync::{
        Arc,
        atomic::{AtomicBool, Ordering},
    },
};

fn setup() -> (App, Entity) {
    setup_on(Arc::new(MemoryStore::default()))
}

fn setup_on(store: Arc<dyn Store>) -> (App, Entity) {
    let (mut app, _) = app_on(&MemoryStore::default());
    app.insert_resource(SessionStore(store));
    app.add_plugins((AgentPlugin, SessionLogPlugin, D0ProbePlugin));
    #[cfg(feature = "rig-fork-probe")]
    app.register_type::<Unserializable>();
    app.add_open_tool(
        "probe",
        "A synthetic open operation",
        ToolOptions::default(),
        |_: On<ToolCalled<serde_json::Value>>| {},
    );
    app.finish();
    app.update();
    let agent = first_agent(app.world_mut()).unwrap();
    (app, agent)
}

fn prepared(app: &mut App, agent: Entity) -> (Entity, Checkpoint, String) {
    let call = start_call(app.world_mut(), agent, ("probe", "t1"), json!({"value":1})).unwrap();
    app.world_mut().run_schedule(AgentPass);
    let cp = app.world().get::<ProbeState>(agent).unwrap().operations[0].clone();
    let id = app.world().get::<AgentId>(agent).unwrap().0.clone();
    (call, cp, id)
}

fn trace(app: &App) -> Vec<Value> {
    app.world()
        .resource::<Trace>()
        .0
        .lock()
        .unwrap()
        .events
        .clone()
}

#[test]
fn operation_cannot_start_during_passes_before_saved_state_is_appended() {
    let (mut app, agent) = setup();
    let call = start_call(app.world_mut(), agent, ("probe", "t1"), json!({})).unwrap();
    for _ in 0..20 {
        app.world_mut().run_schedule(AgentPass);
    }
    assert!(
        !app.world().entity(call).contains::<Started>(),
        "dispatch escaped before Last saved the matching operation state"
    );
    app.update();
    app.update();
    assert!(
        app.world().entity(call).contains::<Started>(),
        "an ordinary observed append should release this diagnostic gate: {:?}",
        app.world().resource::<Trace>().0.lock().unwrap().events
    );
}

#[test]
fn another_log_or_revision_cannot_release_this_operation() {
    let (mut app, agent) = setup();
    let (call, cp, id) = prepared(&mut app, agent);
    let mut stale = cp.clone();
    stale.revision += 1;
    app.world().resource::<Trace>().0.lock().unwrap().written =
        vec![("another-agent".into(), cp), (id, stale)];
    app.world_mut().run_schedule(First);
    app.world_mut().run_schedule(AgentPass);
    assert!(!app.world().entity(call).contains::<Started>());
    app.update();
    app.update();
    assert!(app.world().entity(call).contains::<Started>());
}

#[test]
fn changed_arguments_or_generation_require_a_new_append() {
    for change_generation in [false, true] {
        let (mut app, agent) = setup();
        let (call, cp, _) = prepared(&mut app, agent);
        app.update(); // Save the original checkpoint, without releasing it.
        if change_generation {
            app.world_mut()
                .get_mut::<ProbeState>(agent)
                .unwrap()
                .generation += 1;
        } else {
            app.world_mut()
                .get_mut::<ToolRequest>(call)
                .unwrap()
                .call
                .function
                .arguments = json!({"value":2}).as_object().unwrap().clone();
        }
        app.world_mut().run_schedule(First);
        app.world_mut().run_schedule(AgentPass);
        assert!(!app.world().entity(call).contains::<Started>());
        assert!(app.world().get::<ProbeState>(agent).unwrap().revision > cp.revision);
        app.update();
        app.update();
        assert!(app.world().entity(call).contains::<Started>());
    }
}

#[test]
fn model_rerouting_requires_a_new_checkpoint_even_when_payload_is_unchanged() {
    let (mut app, agent) = setup();
    let original = MockCompletionModel::from_stream_turns([reply("original")]);
    let replacement = MockCompletionModel::from_stream_turns([reply("replacement")]);
    app.world_mut()
        .entity_mut(agent)
        .insert(connect(&original).unwrap());
    app.world_mut()
        .get_mut::<Conversation>(agent)
        .unwrap()
        .push(rig_core::completion::Message::user("synthetic route check"));
    let call = app.world_mut().spawn(SideCall(agent)).id();
    app.world_mut().run_schedule(AgentPass);
    let cp = app.world().get::<ProbeState>(agent).unwrap().operations[0].clone();
    let payload = app
        .world()
        .get::<Adapted>(call)
        .unwrap()
        .sent(app.world().get::<ModelRequest>(call).unwrap());
    app.update(); // Original route's checkpoint appended; dispatch still held.
    let mut connection = connect(&replacement).unwrap();
    Arc::make_mut(&mut connection.spec).id = "rerouted-model".into();
    let adapted = Adapted(connection);
    assert_eq!(
        digest(&payload).unwrap(),
        digest(&adapted.sent(app.world().get::<ModelRequest>(call).unwrap())).unwrap()
    );
    app.world_mut().entity_mut(call).insert(adapted);
    app.world_mut().run_schedule(First);
    app.world_mut().run_schedule(AgentPass);
    assert!(
        !app.world().entity(call).contains::<Started>(),
        "the original model's acknowledgement released a different route"
    );
    assert!(app.world().get::<ProbeState>(agent).unwrap().revision > cp.revision);
    pump(&mut app, |world| world.get::<Settled>(call).is_some());
    assert_eq!(
        replacement.request_count(),
        1,
        "{:?}",
        app.world().get::<Outcome>(call)
    );
    assert_eq!(original.request_count(), 0);
}

#[derive(Clone, Default)]
struct FallibleStore {
    inner: MemoryStore,
    fail: Arc<AtomicBool>,
}
impl Files for FallibleStore {
    fn write_file(&self, n: &str, b: &[u8]) -> io::Result<()> {
        self.inner.write_file(n, b)
    }
    fn append_file(&self, n: &str, b: &[u8]) -> io::Result<()> {
        self.inner.append_file(n, b)
    }
    fn read_file(&self, n: &str) -> io::Result<Vec<u8>> {
        self.inner.read_file(n)
    }
    fn remove_file(&self, n: &str) -> io::Result<()> {
        self.inner.remove_file(n)
    }
    fn files(&self) -> io::Result<Vec<String>> {
        self.inner.files()
    }
}
impl Store for FallibleStore {
    fn logs(&self) -> io::Result<Vec<String>> {
        self.inner.logs()
    }
    fn read_log(&self, n: &str) -> io::Result<Vec<u8>> {
        self.inner.read_log(n)
    }
    fn append_log(&self, n: &str, l: u64, b: &[u8]) -> Result<(), LogWriteError> {
        if self.fail.load(Ordering::SeqCst) {
            Err(io::Error::other("injected append failure").into())
        } else {
            self.inner.append_log(n, l, b)
        }
    }
    fn replace_log(&self, n: &str, l: u64, b: &[u8]) -> Result<(), LogWriteError> {
        self.inner.replace_log(n, l, b)
    }
    fn destroy(&self) -> io::Result<()> {
        self.inner.destroy()
    }
}

#[test]
fn append_failure_cannot_release_the_diagnostic_gate() {
    let store = FallibleStore::default();
    let (mut app, agent) = setup_on(Arc::new(store.clone()));
    let (call, _, _) = prepared(&mut app, agent);
    store.fail.store(true, Ordering::SeqCst);
    for _ in 0..5 {
        app.update();
    }
    assert!(!app.world().entity(call).contains::<Started>());
    assert!(trace(&app).iter().any(|e| e["event"] == "append_failed"));
    assert!(
        !trace(&app)
            .iter()
            .any(|e| e["event"] == "write_gate_released")
    );
}

#[test]
fn ordinary_append_contains_both_saved_state_and_pending_conversation_change() {
    let store = MemoryStore::default();
    let (mut app, agent) = setup_on(Arc::new(store.clone()));
    prepared(&mut app, agent);
    app.world_mut()
        .get_mut::<Conversation>(agent)
        .unwrap()
        .push(rig_core::completion::Message::user(
            "synthetic checkpoint evidence",
        ));
    app.update();
    let events = trace(&app);
    let append = events
        .iter()
        .find(|e| {
            e["event"] == "append_observed" && !e["checkpoints"].as_array().unwrap().is_empty()
        })
        .unwrap();
    assert!(
        append["record_types"]
            .as_array()
            .unwrap()
            .contains(&json!("push"))
    );
    let session = Session::read(&store).unwrap();
    assert!(session.main_agent().unwrap().saved.contains_key(KEY));
    assert_eq!(
        append["durable"], false,
        "a Store write is not a disk receipt"
    );
}

#[test]
fn review_snapshot_precedes_final_kernel_redaction() {
    let (mut app, agent) = setup();
    let (call, _, _) = prepared(&mut app, agent);
    app.update();
    app.update();
    app.world_mut()
        .entity_mut(call)
        .insert(Outcome::from(ToolResult::success(
            "API_KEY=synthetic-secret-value-123456".into(),
        )));
    app.world_mut().run_schedule(AgentPass);
    let events = trace(&app);
    let reviewed = events.iter().find(|e| e["event"] == "review").unwrap();
    let settled = events
        .iter()
        .find(|e| e["event"] == "settled" && !e["digest"].is_null())
        .unwrap();
    assert_ne!(
        reviewed["digest"], settled["digest"],
        "characterization gap: Review is not the final redacted outcome"
    );
    let text = app
        .world()
        .get::<Outcome>(call)
        .unwrap()
        .tool_result()
        .output()
        .render();
    assert!(text.contains("[REDACTED]") && !text.contains("synthetic-secret"));
}

#[test]
fn unrecognized_and_malformed_required_state_are_not_fail_closed_by_rig() {
    let store = MemoryStore::default();
    let (mut app, agent) = setup_on(Arc::new(store.clone()));
    let (_, _, id) = prepared(&mut app, agent);
    app.update();
    drop(app);
    let original = store.read_log(&id).unwrap();
    let mut lines: Vec<Value> = original
        .split(|b| *b == b'\n')
        .filter(|b| !b.is_empty())
        .map(|line| serde_json::from_slice(line).unwrap())
        .collect();
    for line in &mut lines {
        if line["key"] == KEY {
            line["value"] = json!({"revision":"invalid"});
        }
    }
    let changed = lines
        .iter()
        .map(|line| format!("{}\n", line))
        .collect::<String>();
    store
        .replace_log(&id, original.len() as u64, changed.as_bytes())
        .unwrap();
    // The durable probe type is known, but the required value is invalid.
    let (app, agent) = setup_on(Arc::new(store.clone()));
    assert!(app.world().contains_resource::<SessionRestored>());
    assert!(app.world().get::<ProbeState>(agent).is_none());
    drop(app);
    // A recipe that has no such plugin also opens the session.
    let (mut legacy, _) = app_on(&store);
    legacy.add_plugins((AgentPlugin, SessionLogPlugin));
    legacy.finish();
    legacy.update();
    assert!(legacy.world().contains_resource::<SessionRestored>());
}

#[derive(Component, Reflect, serde::Deserialize, Clone)]
#[reflect(opaque)]
#[reflect(Component, Saved, Serialize, Deserialize)]
struct Unserializable;
impl serde::Serialize for Unserializable {
    fn serialize<S: serde::Serializer>(&self, _: S) -> Result<S::Ok, S::Error> {
        Err(serde::ser::Error::custom(
            "injected saved-state serialization failure",
        ))
    }
}

#[test]
#[cfg(not(feature = "rig-fork-probe"))]
fn saved_component_serialization_failure_does_not_fail_the_other_log_write() {
    let store = MemoryStore::default();
    let (mut app, _) = app_on(&store);
    app.add_plugins((AgentPlugin, SessionLogPlugin))
        .register_type::<Unserializable>();
    app.finish();
    app.update();
    let agent = first_agent(app.world_mut()).unwrap();
    app.world_mut().entity_mut(agent).insert(Unserializable);
    app.world_mut()
        .get_mut::<Conversation>(agent)
        .unwrap()
        .push(rig_core::completion::Message::user(
            "saved without required component",
        ));
    app.update();
    assert!(
        app.world()
            .resource::<rig_harness::rig_ecs::session_log::SessionLog>()
            .has_written()
    );
    assert!(
        app.world()
            .resource::<rig_harness::rig_ecs::session_log::SessionLog>()
            .failure()
            .is_none()
    );
    let saved = Session::read(&store).unwrap();
    let agent = saved.main_agent().unwrap();
    assert!(
        !agent
            .saved
            .contains_key(std::any::type_name::<Unserializable>())
    );
    assert!(!agent.conversation.messages().is_empty());
}

#[test]
#[cfg(feature = "rig-fork-probe")]
fn serialization_failure_keeps_vivy_execution_gate_closed() {
    let store = MemoryStore::default();
    let (mut app, agent) = setup_on(Arc::new(store.clone()));
    app.world_mut().entity_mut(agent).insert(Unserializable);
    let call = start_call(app.world_mut(), agent, ("probe", "failed-save"), json!({})).unwrap();
    for _ in 0..20 {
        app.update();
    }
    assert!(
        !app.world().entity(call).contains::<Started>(),
        "Vivy dispatched despite losing another saved component"
    );
    let failure = app
        .world()
        .resource::<rig_harness::rig_ecs::session_log::SessionLog>()
        .failure()
        .expect("serialization must be observable as a log failure");
    assert!(
        failure.contains("injected saved-state serialization failure"),
        "{failure}"
    );
    assert!(
        store.logs().unwrap().is_empty(),
        "an incomplete checkpoint was appended"
    );
}

fn pump(app: &mut App, done: impl Fn(&mut World) -> bool) {
    let until = std::time::Instant::now() + std::time::Duration::from_secs(10);
    while std::time::Instant::now() < until {
        app.update();
        if done(app.world_mut()) {
            return;
        }
        std::thread::sleep(std::time::Duration::from_millis(1));
    }
    panic!(
        "probe deadline exceeded; trace tail: {:?}",
        trace(app).into_iter().rev().take(20).collect::<Vec<_>>()
    );
}

fn model_app(
    store: &MemoryStore,
    model: &MockCompletionModel,
    carry: bool,
    executed: Arc<std::sync::atomic::AtomicUsize>,
) -> App {
    let (mut app, _) = app_on(store);
    app.add_plugins((AgentPlugin, SessionLogPlugin, D0ProbePlugin));
    let connection = connect(model).unwrap();
    app.add_observer(move |added: On<Add<Agent>>, mut commands: Commands| {
        commands.entity(added.entity).insert(connection.clone());
    });
    app.add_open_tool(
        "probe",
        "Synthetic operation",
        ToolOptions::default(),
        move |call: On<ToolCalled<Value>>,
              requests: Query<&ToolRequest>,
              mut decisions: Query<&mut Decision>| {
            executed.fetch_add(1, Ordering::SeqCst);
            if requests
                .get(call.call)
                .unwrap()
                .call
                .function
                .arguments
                .get("block")
                != Some(&json!(true))
            {
                decisions.get_mut(call.call).unwrap().decide(
                    Decider::plugin("fixture"),
                    Outcome::from(ToolResult::success("SAVED_TOOL_RESULT".into())),
                );
            }
        },
    );
    if carry {
        app.add_systems(
            AgentPass,
            (|turns: Query<Entity, (With<TurnOf>, With<Restored>)>, mut commands: Commands| {
                for turn in &turns {
                    commands.entity(turn).insert(CarryOn);
                }
            })
            .in_set(Stage::Gate),
        );
    }
    app.finish();
    app.update();
    app
}

#[test]
fn restored_turn_requires_opt_in_and_never_reexecutes_a_settled_or_uncertain_tool() {
    use std::sync::atomic::AtomicUsize;
    for carry in [false, true] {
        let store = MemoryStore::default();
        let executed = Arc::new(AtomicUsize::new(0));
        let model = MockCompletionModel::from_stream_turns([
            calls("done", "probe", json!({})),
            calls("uncertain", "probe", json!({"block":true})),
        ]);
        let mut app = model_app(&store, &model, false, executed.clone());
        let agent = first_agent(app.world_mut()).unwrap();
        app.world_mut()
            .trigger(Deliver::new(agent, "synthetic task", DeliveryMode::Queue));
        pump(&mut app, |_| executed.load(Ordering::SeqCst) == 2);
        app.update();
        drop(app); // Simulated host loss: no graceful StopTurn.
        let resumed = MockCompletionModel::from_stream_turns([reply("reconciled")]);
        let mut app = model_app(&store, &resumed, carry, executed.clone());
        pump(&mut app, |world| {
            let agent = first_agent(world).unwrap();
            world.get::<ActiveTurn>(agent).is_none()
        });
        assert_eq!(executed.load(Ordering::SeqCst), 2);
        assert_eq!(resumed.request_count(), usize::from(carry));
        let saved = Session::read(&store).unwrap();
        let text =
            serde_json::to_string(&saved.main_agent().unwrap().conversation.messages()).unwrap();
        assert!(text.contains("SAVED_TOOL_RESULT"), "{text}");
        assert!(
            text.contains("uncertain"),
            "original call identity was lost: {text}"
        );
    }
}

#[test]
fn stop_with_late_result_does_not_start_a_successor_model_call() {
    use std::sync::atomic::AtomicUsize;
    let store = MemoryStore::default();
    let executed = Arc::new(AtomicUsize::new(0));
    let model =
        MockCompletionModel::from_stream_turns([calls("wait", "probe", json!({"block":true}))]);
    let mut app = model_app(&store, &model, false, executed.clone());
    let agent = first_agent(app.world_mut()).unwrap();
    app.world_mut()
        .trigger(Deliver::new(agent, "synthetic task", DeliveryMode::Queue));
    pump(&mut app, |_| executed.load(Ordering::SeqCst) == 1);
    let call = app
        .world_mut()
        .query_filtered::<Entity, (With<ToolRequest>, With<Started>, Without<Settled>)>()
        .single(app.world())
        .unwrap();
    app.world_mut().trigger(StopTurn::new(agent));
    if let Ok(mut entity) = app.world_mut().get_entity_mut(call) {
        entity.insert(Outcome::from(ToolResult::success("late result".into())));
    }
    pump(&mut app, |world| world.get::<ActiveTurn>(agent).is_none());
    assert_eq!(model.request_count(), 1);
}

#[test]
fn two_directory_stores_have_write_conflict_checks_but_no_lifetime_writer_lease() {
    let path = std::env::temp_dir().join(format!(
        "vivy-d0-store-{}",
        rig_harness::harness_protocol::SessionId::generate()
    ));
    let a = rig_memory::DirStore::open(&path).unwrap();
    let b = rig_memory::DirStore::open(&path).unwrap();
    a.append_log("agent", 0, b"first\n").unwrap();
    assert!(matches!(
        b.append_log("agent", 0, b"stale\n"),
        Err(LogWriteError::Changed)
    ));
    b.append_log("agent", 6, b"second\n").unwrap();
    assert_eq!(a.read_log("agent").unwrap(), b"first\nsecond\n");
    // The persistent lock is the launcher's responsibility, not DirStore's.
    drop((a, b));
    assert!(path.starts_with(std::env::temp_dir()));
    std::fs::remove_dir_all(path).unwrap();
}
