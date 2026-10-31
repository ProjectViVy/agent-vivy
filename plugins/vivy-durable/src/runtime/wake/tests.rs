use super::*;
use crate::runtime::tests::until;
use rig_core::test_utils::MockCompletionModel;
use rig_harness_test_support::{MemoryStore, app_on, calls, connect, first_agent, reply};
use rig_memory::{Files, LogWriteError, Store};
use std::sync::{
    Arc,
    atomic::{AtomicBool, Ordering},
};

#[derive(Clone)]
struct FailingStore {
    disk: MemoryStore,
    fail: Arc<AtomicBool>,
}
impl Files for FailingStore {
    fn write_file(&self, n: &str, b: &[u8]) -> io::Result<()> {
        self.disk.write_file(n, b)
    }
    fn append_file(&self, n: &str, b: &[u8]) -> io::Result<()> {
        self.disk.append_file(n, b)
    }
    fn read_file(&self, n: &str) -> io::Result<Vec<u8>> {
        self.disk.read_file(n)
    }
    fn remove_file(&self, n: &str) -> io::Result<()> {
        self.disk.remove_file(n)
    }
    fn files(&self) -> io::Result<Vec<String>> {
        self.disk.files()
    }
}
impl Store for FailingStore {
    fn logs(&self) -> io::Result<Vec<String>> {
        self.disk.logs()
    }
    fn read_log(&self, n: &str) -> io::Result<Vec<u8>> {
        self.disk.read_log(n)
    }
    fn append_log(&self, n: &str, len: u64, b: &[u8]) -> Result<(), LogWriteError> {
        if self.fail.load(Ordering::SeqCst) {
            return Err(io::Error::other("synthetic append failure").into());
        }
        self.disk.append_log(n, len, b)
    }
    fn replace_log(&self, n: &str, len: u64, b: &[u8]) -> Result<(), LogWriteError> {
        self.disk.replace_log(n, len, b)
    }
    fn destroy(&self) -> io::Result<()> {
        self.disk.destroy()
    }
}

fn app(disk: &MemoryStore, model: &MockCompletionModel) -> (App, Entity) {
    app_with_failure(disk, model, Arc::new(AtomicBool::new(false)))
}
fn app_with_failure(
    disk: &MemoryStore,
    model: &MockCompletionModel,
    fail: Arc<AtomicBool>,
) -> (App, Entity) {
    let (mut app, _) = app_on(disk);
    app.insert_resource(SessionStore::new(FailingStore {
        disk: disk.clone(),
        fail,
    }));
    app.add_plugins((
        AgentPlugin,
        SessionLogPlugin,
        DurablePlugin,
        assistant::AssistantContextPlugin,
    ));
    let connection = connect(model).unwrap();
    app.add_observer(move |e: On<Add<Agent>>, mut commands: Commands| {
        commands.entity(e.entity).insert(connection.clone());
    });
    app.finish();
    app.update();
    let agent = first_agent(app.world_mut()).unwrap();
    (app, agent)
}
fn wait(id: &str, seconds: u64) -> Vec<rig_core::test_utils::MockStreamEvent> {
    calls(
        id,
        "task_control",
        json!({"action":"wait","task":"task:1","generation":0,"seconds":seconds,"reason":"RECHECK_CANARY"}),
    )
}
fn idle_wait(app: &mut App, agent: Entity) {
    until(app, |w| {
        committed(w, agent)
            .is_some_and(|s| s.tasks[0].status == Status::Waiting && s.active.is_none())
            && !w.entity(agent).contains::<ActiveTurn>()
    });
}
fn foreground_calls(model: &MockCompletionModel) -> usize {
    model
        .requests()
        .iter()
        .filter(|r| {
            !serde_json::to_string(r)
                .unwrap()
                .contains("Maintain concise event memory")
        })
        .count()
}

#[test]
fn repeated_wakes_resume_the_same_task_and_keep_context_and_steps() {
    let disk = MemoryStore::default();
    let model = MockCompletionModel::from_stream_turns([
        wait("w1", 0),
        wait("w2", 0),
        calls(
            "end",
            "task_control",
            json!({"action":"finish_silently","task":"task:1","generation":0}),
        ),
    ]);
    let (mut app, agent) = app(&disk, &model);
    submit(
        app.world_mut(),
        agent,
        "test",
        "one",
        "ORIGINAL_TASK_CANARY",
    )
    .unwrap();
    until(&mut app, |w| {
        committed(w, agent)
            .is_some_and(|s| s.tasks[0].status == Status::Completed && s.active.is_none())
    });
    let state = inspect(app.world(), agent).unwrap();
    assert_eq!(state.tasks.len(), 1);
    assert_eq!(state.wakes.len(), 2);
    assert!(state.wakes.iter().all(|w| w.delivered.is_some()));
    assert_eq!(state.wakes[1].generation, 1);
    assert_eq!(model.request_count(), 3);
    let text = serde_json::to_string(&model.requests()[2]).unwrap();
    assert!(
        text.contains("ORIGINAL_TASK_CANARY")
            && text.contains("RECHECK_CANARY")
            && text.contains("not the user's words"),
        "{text}"
    );
    let mut last = None;
    for op in &state.operations {
        if let Some(n) = last {
            assert!(op.step > n);
        }
        last = Some(op.step);
    }
    for _ in 0..12 {
        app.update();
    }
    assert_eq!(foreground_calls(&model), 3);
}

#[test]
fn replace_cancel_and_clock_changes_fence_old_generations() {
    let disk = MemoryStore::default();
    let model = MockCompletionModel::from_stream_turns([wait("w", 3600)]);
    let (mut app, agent) = app(&disk, &model);
    submit(app.world_mut(), agent, "test", "one", "wait").unwrap();
    idle_wait(&mut app, agent);
    let old = inspect(app.world(), agent).unwrap().wakes[0].clone();
    let at = now_ms() + 100_000;
    let new = schedule_wake(
        app.world_mut(),
        agent,
        "task:1",
        0,
        Some(0),
        at,
        "replacement",
    )
    .unwrap();
    assert_eq!(new.generation, 1);
    assert!(cancel_wake(app.world_mut(), agent, &old.id, 0).is_err());
    assert!(
        schedule_wake(
            app.world_mut(),
            agent,
            "task:1",
            0,
            Some(0),
            at,
            "duplicate"
        )
        .is_err()
    );
    for _ in 0..3 {
        app.update();
    }
    tick_at(app.world_mut(), at - 1);
    assert_eq!(
        inspect(app.world(), agent).unwrap().wakes[1].phase,
        Phase::Scheduled
    );
    // The clock mark may be stale after a backwards jump; UTC is authoritative.
    let alarm = app
        .world_mut()
        .query_filtered::<Entity, With<Alarm>>()
        .single(app.world())
        .unwrap();
    app.world_mut().entity_mut(alarm).insert(Due);
    tick_at(app.world_mut(), at - 1);
    assert!(app.world().get::<Due>(alarm).is_none());
    tick_at(app.world_mut(), at);
    assert_eq!(
        inspect(app.world(), agent).unwrap().wakes[1].phase,
        Phase::Fired
    );
    cancel_wake(app.world_mut(), agent, &new.id, 1).unwrap();
    for _ in 0..10 {
        app.update();
    }
    assert_eq!(foreground_calls(&model), 1);
    let s = inspect(app.world(), agent).unwrap();
    assert_eq!(s.tasks[0].status, Status::Suspended);
    assert!(!pending(s));
    assert!(serde_json::from_value::<DurableAgentState>(serde_json::to_value(s).unwrap()).is_ok());
}

#[test]
fn overdue_wake_reopens_without_a_new_user_task() {
    let disk = MemoryStore::default();
    let first = MockCompletionModel::from_stream_turns([wait("w", 3600)]);
    let (mut a, agent) = app(&disk, &first);
    submit(a.world_mut(), agent, "test", "one", "ORIGINAL_CANARY").unwrap();
    idle_wait(&mut a, agent);
    schedule_wake(
        a.world_mut(),
        agent,
        "task:1",
        0,
        Some(0),
        now_ms() - 1,
        "overdue",
    )
    .unwrap();
    // Save just the schedule; kill before the next First timer delivery.
    a.world_mut().run_schedule(Last);
    drop(a);
    let model = MockCompletionModel::from_stream_turns([reply("resumed")]);
    let (mut a, agent) = app(&disk, &model);
    until(&mut a, |w| {
        committed(w, agent).is_some_and(|s| s.tasks[0].status == Status::Completed)
    });
    let s = inspect(a.world(), agent).unwrap();
    assert_eq!(s.tasks.len(), 1);
    assert_eq!(model.request_count(), 1);
    assert_eq!(s.wakes.iter().filter(|w| w.delivered.is_some()).count(), 1);
    assert!(
        serde_json::to_string(&model.requests()[0])
            .unwrap()
            .contains("ORIGINAL_CANARY")
    );
}

#[test]
fn wake_during_a_tool_yields_siblings_but_does_not_switch_the_active_task() {
    use rig_core::test_utils::MockStreamEvent;
    let mut batch = calls("one", "mutate", json!({}));
    batch.retain(|e| !matches!(e, MockStreamEvent::FinalResponse(_)));
    batch.extend(calls("two", "mutate", json!({})));
    let disk = MemoryStore::default();
    let model = MockCompletionModel::from_stream_turns([
        wait("wait", 3600),
        batch,
        calls(
            "end-a",
            "task_control",
            json!({"action":"finish_silently","task":"task:2","generation":0}),
        ),
        reply("resumed target"),
    ]);
    let (mut app, agent) = app(&disk, &model);
    app.add_open_tool(
        "mutate",
        "held test effect",
        ToolOptions::default(),
        |_: On<ToolCalled<Value>>| {},
    );
    submit(app.world_mut(), agent, "test", "one", "PRIVATE_WAIT_TASK").unwrap();
    idle_wait(&mut app, agent);
    submit(app.world_mut(), agent, "test", "two", "FOREGROUND_PRIVATE").unwrap();
    until(&mut app, |w| {
        w.iter_entities().any(|e| {
            e.contains::<ToolRequest>() && e.contains::<Started>() && !e.contains::<Settled>()
        })
    });
    let call = app
        .world()
        .iter_entities()
        .find(|e| {
            e.get::<ToolRequest>()
                .is_some_and(|t| t.call.function.name.as_str() == "mutate")
                && e.contains::<Started>()
        })
        .unwrap()
        .id();
    let at = inspect(app.world(), agent).unwrap().wakes[0].deadline_ms;
    tick_at(app.world_mut(), at);
    for _ in 0..5 {
        app.update();
    }
    assert_eq!(model.request_count(), 2);
    assert_eq!(
        inspect(app.world(), agent).unwrap().active.as_deref(),
        Some("task:2")
    );
    app.world_mut()
        .entity_mut(call)
        .insert(Outcome::from(ToolResult::success(
            rig_core::tool::ToolOutput::text("first tool finished"),
        )));
    until(&mut app, |w| {
        committed(w, agent).is_some_and(|s| s.tasks.iter().all(|t| t.status == Status::Completed))
    });
    let sent = serde_json::to_string(&model.requests()[2]).unwrap();
    assert!(
        sent.contains("not_run")
            && sent.contains("RECHECK_CANARY")
            && sent.contains("FOREGROUND_PRIVATE"),
        "{sent}"
    );
    assert!(!sent.contains("PRIVATE_WAIT_TASK"), "{sent}");
    let target = serde_json::to_string(&model.requests()[3]).unwrap();
    assert!(
        target.contains("PRIVATE_WAIT_TASK")
            && target.contains("RECHECK_CANARY")
            && !target.contains("FOREGROUND_PRIVATE"),
        "{target}"
    );
}

#[test]
fn cancelling_fired_input_before_delivery_does_not_leave_an_active_turn_stuck() {
    let disk = MemoryStore::default();
    let model = MockCompletionModel::from_stream_turns([wait("wait", 3600)]);
    let (mut app, agent) = app(&disk, &model);
    submit(app.world_mut(), agent, "test", "one", "wait").unwrap();
    idle_wait(&mut app, agent);
    let at = inspect(app.world(), agent).unwrap().wakes[0].deadline_ms;
    tick_at(app.world_mut(), at);
    // Persist fire, then admit without running the next agent pass.
    app.world_mut().run_schedule(Last);
    tick_at(app.world_mut(), at);
    super::super::admit_inputs(app.world_mut());
    app.world_mut().run_schedule(Last);
    assert_eq!(
        inspect(app.world(), agent).unwrap().active.as_deref(),
        Some("task:1")
    );
    cancel_wake(app.world_mut(), agent, "wake:task:1", 0).unwrap();
    for _ in 0..15 {
        app.update();
    }
    let s = inspect(app.world(), agent).unwrap();
    assert_eq!(s.tasks[0].status, Status::Suspended);
    assert!(s.active.is_none());
    assert!(!app.world().entity(agent).contains::<ActiveTurn>());
    assert_eq!(foreground_calls(&model), 1);
}

#[test]
fn each_fire_and_delivery_checkpoint_reopens_exactly_once() {
    for frames in 0..=8 {
        let disk = MemoryStore::default();
        let model = MockCompletionModel::from_stream_turns([wait("wait", 3600), reply("awake")]);
        let (mut first, agent) = app(&disk, &model);
        submit(first.world_mut(), agent, "test", "one", "wait").unwrap();
        idle_wait(&mut first, agent);
        schedule_wake(
            first.world_mut(),
            agent,
            "task:1",
            0,
            Some(0),
            now_ms() - 1,
            "resume",
        )
        .unwrap();
        first.world_mut().run_schedule(Last);
        for _ in 0..frames {
            first.update();
        }
        drop(first);
        let replacement = MockCompletionModel::from_stream_turns([reply("awake")]);
        let (mut second, agent) = app(&disk, &replacement);
        until(&mut second, |w| {
            committed(w, agent).is_some_and(|s| s.tasks[0].status == Status::Completed)
        });
        let s = inspect(second.world(), agent).unwrap();
        assert_eq!(s.tasks.len(), 1, "frames={frames}");
        assert_eq!(
            s.wakes.iter().filter(|w| w.delivered.is_some()).count(),
            1,
            "frames={frames}"
        );
        assert!(foreground_calls(&replacement) <= 1);
    }
}

#[test]
fn saved_wait_control_finishes_a_restored_turn_before_arming_delivery() {
    let disk = MemoryStore::default();
    let model = MockCompletionModel::from_stream_turns([wait("wait", 0)]);
    let (mut first, agent) = app(&disk, &model);
    submit(first.world_mut(), agent, "test", "one", "wait").unwrap();
    until(&mut first, |w| {
        committed(w, agent).is_some_and(|s| {
            s.control.is_some()
                && s.operations
                    .iter()
                    .any(|o| o.kind == "tool" && o.outcome.is_some() && o.published)
        })
    });
    drop(first);
    let model = MockCompletionModel::from_stream_turns([reply("awake")]);
    let (mut second, agent) = app(&disk, &model);
    until(&mut second, |w| {
        committed(w, agent)
            .is_some_and(|s| s.tasks[0].status == Status::Completed && s.active.is_none())
    });
    assert_eq!(foreground_calls(&model), 1);
}

#[test]
fn failed_fire_append_cannot_release_input_and_reopen_keeps_the_schedule() {
    let disk = MemoryStore::default();
    let flag = Arc::new(AtomicBool::new(false));
    let model = MockCompletionModel::from_stream_turns([wait("wait", 3600)]);
    let (mut first, agent) = app_with_failure(&disk, &model, flag.clone());
    submit(first.world_mut(), agent, "test", "one", "wait").unwrap();
    idle_wait(&mut first, agent);
    let deadline = now_ms() - 1;
    schedule_wake(
        first.world_mut(),
        agent,
        "task:1",
        0,
        Some(0),
        deadline,
        "durable despite append failure",
    )
    .unwrap();
    first.world_mut().run_schedule(Last);
    flag.store(true, Ordering::SeqCst);
    for _ in 0..8 {
        first.update();
    }
    assert!(!healthy(first.world()));
    assert_eq!(foreground_calls(&model), 1);
    assert_eq!(
        committed(first.world(), agent).unwrap().wakes[1].phase,
        Phase::Scheduled
    );
    assert_eq!(
        inspect(first.world(), agent).unwrap().wakes[1].phase,
        Phase::Fired
    );
    assert!(cancel_wake(first.world_mut(), agent, "wake:task:1", 1).is_err());
    drop(first);
    let model = MockCompletionModel::from_stream_turns([reply("awake")]);
    let (mut second, agent) = app(&disk, &model);
    until(&mut second, |w| {
        committed(w, agent).is_some_and(|s| s.tasks[0].status == Status::Completed)
    });
    assert_eq!(foreground_calls(&model), 1);
    assert_eq!(
        inspect(second.world(), agent).unwrap().wakes[1].deadline_ms,
        deadline
    );
}

#[test]
fn cancelling_another_tasks_wait_does_not_finish_the_caller() {
    let disk = MemoryStore::default();
    let model = MockCompletionModel::from_stream_turns([
        wait("wait", 3600),
        calls(
            "cancel",
            "task_control",
            json!({"action":"cancel_wake","task":"task:1","generation":0,"wake_id":"wake:task:1","wake_generation":0}),
        ),
        reply("caller continues"),
    ]);
    let (mut app, agent) = app(&disk, &model);
    submit(app.world_mut(), agent, "test", "one", "waiting task").unwrap();
    idle_wait(&mut app, agent);
    submit(
        app.world_mut(),
        agent,
        "test",
        "two",
        "cancel earlier wait then answer me",
    )
    .unwrap();
    until(&mut app, |w| {
        committed(w, agent).is_some_and(|s| s.tasks[1].status == Status::Completed)
    });
    let s = inspect(app.world(), agent).unwrap();
    assert_eq!(s.tasks[0].status, Status::Suspended);
    assert_eq!(s.wakes[0].phase, Phase::Cancelled);
    assert_eq!(foreground_calls(&model), 3);
    assert!(
        serde_json::to_string(&model.requests()[2])
            .unwrap()
            .contains("cancel earlier wait")
    );
}

#[test]
fn lifecycle_sources_are_immutable_through_fire_cancel_and_reschedule() {
    let disk = MemoryStore::default();
    let model = MockCompletionModel::from_stream_turns([wait("wait", 3600)]);
    let (mut app, agent) = app(&disk, &model);
    submit(app.world_mut(), agent, "test", "one", "wait").unwrap();
    idle_wait(&mut app, agent);
    let old = sources(inspect(app.world(), agent).unwrap());
    tick_at(app.world_mut(), now_ms() + 3_600_001);
    let fired = sources(inspect(app.world(), agent).unwrap());
    cancel_wake(app.world_mut(), agent, "wake:task:1", 0).unwrap();
    schedule_wake(
        app.world_mut(),
        agent,
        "task:1",
        0,
        Some(0),
        now_ms() + 100_000,
        "replacement",
    )
    .unwrap();
    let current = sources(inspect(app.world(), agent).unwrap());
    assert!(
        old.iter()
            .chain(&fired)
            .all(|source| current.contains(source))
    );
}
