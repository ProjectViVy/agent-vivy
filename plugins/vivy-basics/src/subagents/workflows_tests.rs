use super::tests::Fixture;
use super::*;
use rig_core::test_utils::{MockCompletionModel, MockError, MockStreamEvent};
use rig_harness_test_support::{
    MemoryStore, app_on, calls, connect, reply, run_until, scripted_models,
};
use serde_json::{Value, json};
use std::sync::mpsc::{Receiver, channel};

fn runtime(
    fixture: &Fixture,
    parent_model: &MockCompletionModel,
    children: &[(&str, &MockCompletionModel)],
) -> (App, Receiver<()>, Entity) {
    runtime_on(fixture, parent_model, children, &MemoryStore::default())
}

fn runtime_on(
    fixture: &Fixture,
    parent_model: &MockCompletionModel,
    children: &[(&str, &MockCompletionModel)],
    store: &MemoryStore,
) -> (App, Receiver<()>, Entity) {
    let (mut app, parent) = super::tests::setup_on(fixture, false, store);
    let (sender, wakes) = channel();
    app.insert_resource(Wake::new(move || {
        sender.send(()).ok();
    }))
    .insert_resource(scripted_models(children.iter().copied()));
    app.world_mut()
        .entity_mut(parent)
        .insert(connect(parent_model).unwrap());
    (app, wakes, parent)
}

fn resume(
    fixture: &Fixture,
    store: &MemoryStore,
    parent_model: &MockCompletionModel,
    child: &MockCompletionModel,
) -> (App, Receiver<()>, Entity) {
    let (mut app, wakes) = app_on(store);
    app.insert_resource(RigHome(Home::at(fixture.home.clone())))
        .insert_resource(ProjectRoot(fixture.project.clone()))
        .insert_resource(scripted_models([
            ("deepseek/deepseek-flash", parent_model),
            ("test/child", child),
        ]))
        .add_plugins((AgentPlugin, SessionLogPlugin));
    app.add_open_tool(
        "read",
        "Read.",
        ToolOptions::default(),
        |_: On<ToolCalled<Value>>| {},
    );
    app.add_open_tool(
        "block",
        "Wait.",
        ToolOptions::default(),
        |_: On<ToolCalled<Value>>| {},
    );
    rig_harness::load::<VivySubagentsPlugin>(&mut app, "vivy-basics", "test");
    app.finish();
    app.update();
    let world = app.world_mut();
    let parent = world
        .query_filtered::<Entity, (With<Agent>, Without<SpawnedBy>)>()
        .iter(world)
        .next()
        .unwrap();
    (app, wakes, parent)
}

fn send(app: &mut App, parent: Entity) {
    app.world_mut().trigger(Deliver::new(
        parent,
        "PARENT_PRIVATE_CONTEXT",
        DeliveryMode::Queue,
    ));
}

fn done(app: &mut App, wakes: &Receiver<()>, parent: Entity, model: &MockCompletionModel) {
    run_until(app, wakes, |world| {
        model.request_count() >= 2 && world.get::<ActiveTurn>(parent).is_none()
    });
}

fn messages(model: &MockCompletionModel) -> String {
    model
        .requests()
        .iter()
        .map(|request| format!("{:?}", request.chat_history))
        .collect::<Vec<_>>()
        .join("\n")
}

#[test]
fn native_single_executes_with_independent_context_and_reports_to_parent() {
    let fixture = Fixture::new();
    let child = MockCompletionModel::from_stream_turns([reply("CHILD_RESULT")]);
    let parent_model = MockCompletionModel::from_stream_turns([
        calls(
            "single",
            "task",
            json!({"description":"Investigate","prompt":"CHILD_BRIEF","model":"test/child","tools":["read"]}),
        ),
        reply("Parent continued."),
        reply("Report received."),
    ]);
    let (mut app, wakes, parent) = runtime(&fixture, &parent_model, &[("test/child", &child)]);
    send(&mut app, parent);
    run_until(&mut app, &wakes, |world| {
        messages(&parent_model).contains("CHILD_RESULT")
            && world.get::<ActiveTurn>(parent).is_none()
    });
    assert_eq!(child.request_count(), 1);
    let request = &child.requests()[0];
    assert!(format!("{:?}", request.chat_history).contains("CHILD_BRIEF"));
    assert!(!format!("{:?}", request.chat_history).contains("PARENT_PRIVATE_CONTEXT"));
    let names: Vec<_> = request
        .tools
        .iter()
        .map(|tool| tool.name.as_str())
        .collect();
    assert!(names.contains(&"read") && !names.contains(&"write"));
}

#[test]
fn parallel_returns_one_aggregate_without_steering_or_duplicate_reports() {
    let fixture = Fixture::new();
    let first = MockCompletionModel::from_stream_turns([reply("FIRST_RESULT")]);
    let second = MockCompletionModel::from_stream_turns([reply("SECOND_RESULT")]);
    let parent_model = MockCompletionModel::from_stream_turns([
        calls(
            "parallel",
            "task_parallel",
            json!({"tasks":[
                {"description":"First","prompt":"Inspect A","model":"test/first"},
                {"description":"Second","prompt":"Inspect B","model":"test/second"}
            ]}),
        ),
        reply("Combined."),
    ]);
    let (mut app, wakes, parent) = runtime(
        &fixture,
        &parent_model,
        &[("test/first", &first), ("test/second", &second)],
    );
    send(&mut app, parent);
    done(&mut app, &wakes, parent, &parent_model);
    assert_eq!((first.request_count(), second.request_count()), (1, 1));
    assert_eq!(
        parent_model.request_count(),
        2,
        "reports must not steer a new parent request"
    );
    let history = format!("{:?}", parent_model.requests()[1].chat_history);
    assert_eq!(history.matches("FIRST_RESULT").count(), 1, "{history}");
    assert_eq!(history.matches("SECOND_RESULT").count(), 1, "{history}");
    let total = app
        .world()
        .get_resource::<rig_basics::telemetry::usage::SessionUsage>()
        .map(|usage| usage.0.total.calls);
    assert_eq!(
        total,
        Some(4),
        "two parent calls and two child calls, without double counting"
    );
}

#[test]
fn workflow_waits_for_nested_child_report_before_completing() {
    let fixture = Fixture::new();
    let leaf = MockCompletionModel::from_stream_turns([
        calls("blocked-leaf", "block", json!({})),
        reply("LEAF_RESULT"),
    ]);
    let child = MockCompletionModel::from_stream_turns([
        calls(
            "nested",
            "task",
            json!({"description":"Leaf","prompt":"Find detail","model":"test/leaf"}),
        ),
        reply("WAITING_FOR_DESCENDANT"),
        reply("FINAL_CHILD_RESULT"),
    ]);
    let parent_model = MockCompletionModel::from_stream_turns([
        calls(
            "nested-workflow",
            "task_parallel",
            json!({"tasks":[{"description":"Worker","prompt":"Delegate then synthesize","model":"test/child"}]}),
        ),
        reply("Combined."),
    ]);
    let (mut app, wakes, parent) = runtime(
        &fixture,
        &parent_model,
        &[("test/child", &child), ("test/leaf", &leaf)],
    );
    app.insert_resource(rig_basics::subagents::SubagentPolicy { max_depth: 2 });
    app.add_open_tool(
        "block",
        "Wait",
        ToolOptions::default(),
        |_: On<ToolCalled<Value>>| {},
    );
    send(&mut app, parent);
    run_until(&mut app, &wakes, |world| {
        world.get::<Spawned>(parent).is_some_and(|children| {
            children.iter().any(|child| {
                matches!(
                    world.get::<rig_basics::subagents::Lifecycle>(child),
                    Some(rig_basics::subagents::Lifecycle::WaitingOn(_))
                )
            })
        })
    });
    for _ in 0..8 {
        app.update();
    }
    assert_eq!(
        parent_model.request_count(),
        1,
        "intermediate answer completed the workflow"
    );
    let world = app.world_mut();
    let blocked = world
        .query::<(Entity, &ToolRequest)>()
        .iter(world)
        .find(|(_, request)| request.call.function.name.as_str() == "block")
        .unwrap()
        .0;
    world
        .entity_mut(blocked)
        .insert(Outcome::custom(json!({"done":true})));
    done(&mut app, &wakes, parent, &parent_model);
    let history = format!("{:?}", parent_model.requests()[1].chat_history);
    assert!(
        history.contains("FINAL_CHILD_RESULT") && !history.contains("WAITING_FOR_DESCENDANT"),
        "{history}"
    );
    assert_eq!(
        parent_model.request_count(),
        2,
        "no late report should steer the parent"
    );
}

#[test]
fn chain_passes_previous_output_and_stops_at_a_failed_step() {
    let fixture = Fixture::new();
    let first = MockCompletionModel::from_stream_turns([reply("PRIOR_FINDING")]);
    let second = MockCompletionModel::from_stream_turns([vec![MockStreamEvent::Error(
        MockError::request("scripted child failure"),
    )]]);
    let third = MockCompletionModel::from_stream_turns([reply("MUST_NOT_RUN")]);
    let parent_model = MockCompletionModel::from_stream_turns([
        calls(
            "chain",
            "task_chain",
            json!({"steps":[
                {"description":"Scout","prompt":"Find facts","model":"test/first"},
                {"description":"Worker","prompt":"Use {previous}","model":"test/second"},
                {"description":"Review","prompt":"Review {previous}","model":"test/third"}
            ]}),
        ),
        reply("Failure explained."),
    ]);
    let (mut app, wakes, parent) = runtime(
        &fixture,
        &parent_model,
        &[
            ("test/first", &first),
            ("test/second", &second),
            ("test/third", &third),
        ],
    );
    send(&mut app, parent);
    done(&mut app, &wakes, parent, &parent_model);
    assert_eq!(
        (
            first.request_count(),
            second.request_count(),
            third.request_count()
        ),
        (1, 1, 0)
    );
    assert!(messages(&second).contains("PRIOR_FINDING"));
    assert!(messages(&parent_model).contains("scripted child failure"));
    assert_eq!(parent_model.request_count(), 2);
}

#[test]
fn parallel_keeps_successful_answers_when_another_child_fails() {
    let fixture = Fixture::new();
    let good = MockCompletionModel::from_stream_turns([reply("SUCCESSFUL_FINDING")]);
    let bad = MockCompletionModel::from_stream_turns([vec![MockStreamEvent::Error(
        MockError::request("child rejected"),
    )]]);
    let parent_model = MockCompletionModel::from_stream_turns([
        calls(
            "partial",
            "task_parallel",
            json!({"tasks":[
                {"description":"Good","prompt":"Inspect","model":"test/good"},
                {"description":"Bad","prompt":"Inspect","model":"test/bad"}
            ]}),
        ),
        reply("Partial result explained."),
    ]);
    let (mut app, wakes, parent) = runtime(
        &fixture,
        &parent_model,
        &[("test/good", &good), ("test/bad", &bad)],
    );
    send(&mut app, parent);
    done(&mut app, &wakes, parent, &parent_model);
    let seen = messages(&parent_model);
    assert!(
        seen.contains("SUCCESSFUL_FINDING") && seen.contains("child rejected"),
        "{seen}"
    );
    assert!(
        seen.contains("failed"),
        "failure must have a structured status: {seen}"
    );
}

#[test]
fn parallel_rejects_more_than_eight_tasks_without_starting_children() {
    let fixture = Fixture::new();
    let tasks: Vec<Value> = (0..9)
        .map(|n| json!({"description":format!("Task {n}"),"prompt":"Work"}))
        .collect();
    let parent_model = MockCompletionModel::from_stream_turns([
        calls("oversized", "task_parallel", json!({"tasks":tasks})),
        reply("Limit explained."),
    ]);
    let (mut app, wakes, parent) = runtime(&fixture, &parent_model, &[]);
    send(&mut app, parent);
    done(&mut app, &wakes, parent, &parent_model);
    assert!(
        messages(&parent_model).contains("at most 8"),
        "{}",
        messages(&parent_model)
    );
    assert!(
        app.world()
            .get::<Spawned>(parent)
            .is_none_or(|spawned| spawned.iter().next().is_none())
    );
}

#[test]
fn four_children_run_and_parent_cancel_does_not_admit_queued_work() {
    let fixture = Fixture::new();
    let child = MockCompletionModel::from_stream_turns(
        (0..6).map(|n| calls(&format!("block-{n}"), "block", json!({}))),
    );
    let tasks: Vec<_> = (0..6).map(|n| json!({"description":format!("Task {n}"),"prompt":"Wait in block","model":"test/child","tools":["block"]})).collect();
    let parent_model = MockCompletionModel::from_stream_turns([
        calls("queued", "task_parallel", json!({"tasks":tasks})),
        reply("Done."),
    ]);
    let (mut app, wakes, parent) = runtime(&fixture, &parent_model, &[("test/child", &child)]);
    app.add_open_tool(
        "block",
        "Wait for cancellation.",
        ToolOptions::default(),
        |_: On<ToolCalled<Value>>| {},
    );
    send(&mut app, parent);
    run_until(&mut app, &wakes, |_| {
        parent_model.request_count() >= 2 || child.request_count() >= 4
    });
    for _ in 0..12 {
        app.update();
    }
    let children: Vec<_> = app
        .world()
        .get::<Spawned>(parent)
        .map(|s| s.iter().collect())
        .unwrap_or_default();
    assert_eq!(children.len(), 4, "queued work must not be admitted early");
    app.world_mut().trigger(StopTurn::new(parent));
    for _ in 0..12 {
        app.update();
    }
    assert_eq!(child.request_count(), 4);
    assert!(
        children
            .iter()
            .all(|child| app.world().get::<ActiveTurn>(*child).is_none())
    );
    assert_eq!(
        app.world().get::<Spawned>(parent).unwrap().iter().count(),
        4
    );
}

#[test]
fn follow_up_turns_share_the_same_four_child_limit() {
    let fixture = Fixture::new();
    let child = MockCompletionModel::from_stream_turns(
        (0..6)
            .map(|_| reply("Initial answer"))
            .chain((0..6).map(|n| calls(&format!("follow-block-{n}"), "block", json!({})))),
    );
    let tasks: Vec<_> = (0..6)
        .map(|n| json!({"description":format!("Task {n}"),"prompt":"Answer","model":"test/child"}))
        .collect();
    let parent_model = MockCompletionModel::from_stream_turns([
        calls("initial", "task_parallel", json!({"tasks":tasks})),
        reply("Done."),
    ]);
    let (mut app, wakes, parent) = runtime(&fixture, &parent_model, &[("test/child", &child)]);
    app.add_open_tool(
        "block",
        "Wait.",
        ToolOptions::default(),
        |_: On<ToolCalled<Value>>| {},
    );
    send(&mut app, parent);
    done(&mut app, &wakes, parent, &parent_model);
    assert_eq!(child.request_count(), 6);
    let children: Vec<_> = app.world().get::<Spawned>(parent).unwrap().iter().collect();
    for (index, agent) in children.iter().enumerate() {
        let id = app.world().get::<AgentId>(*agent).unwrap().0.clone();
        rig_harness_test_support::start_call(
            app.world_mut(),
            parent,
            ("message", &format!("follow-{index}")),
            json!({"agent":id,"text":"Block now"}),
        );
    }
    run_until(&mut app, &wakes, |_| child.request_count() >= 10);
    for _ in 0..12 {
        app.update();
    }
    assert_eq!(
        child.request_count(),
        10,
        "follow-up messages must not bypass admission"
    );
    assert_eq!(
        children
            .iter()
            .filter(|agent| app.world().get::<ActiveTurn>(**agent).is_some())
            .count(),
        4
    );
    app.world_mut().trigger(StopTurn::new(parent));
    for _ in 0..12 {
        app.update();
    }
    assert_eq!(
        child.request_count(),
        10,
        "queued follow-ups must remain stopped after parent cancellation"
    );
}

#[test]
fn completed_child_role_and_directory_survive_resume_and_follow_up() {
    let fixture = Fixture::new();
    fixture.role(false, "---\ntools: [read]\n---\nFROZEN ROLE");
    std::fs::create_dir(fixture.project.join("nested")).unwrap();
    let store = MemoryStore::default();
    let child =
        MockCompletionModel::from_stream_turns([reply("CHILD_INITIAL"), reply("CHILD_FOLLOWUP")]);
    let parent_model = MockCompletionModel::from_stream_turns([
        calls(
            "persisted",
            "task",
            json!({"description":"Saved role","prompt":"Investigate","model":"test/child","profile":"analyst","cwd":"nested"}),
        ),
        reply("Continued."),
        reply("Result noted."),
    ]);
    let (mut app, wakes, parent) =
        runtime_on(&fixture, &parent_model, &[("test/child", &child)], &store);
    send(&mut app, parent);
    run_until(&mut app, &wakes, |world| {
        messages(&parent_model).contains("CHILD_INITIAL")
            && world.get::<ActiveTurn>(parent).is_none()
    });
    let child_id = app
        .world()
        .get::<Spawned>(parent)
        .unwrap()
        .iter()
        .next()
        .and_then(|agent| app.world().get::<AgentId>(agent))
        .unwrap()
        .0
        .clone();
    drop(app);
    fixture.role(false, "---\ntools: [read]\n---\nCHANGED ROLE");
    let parent_resume = MockCompletionModel::from_stream_turns([reply("Followup noted.")]);
    let (mut app, wakes, parent) = resume(&fixture, &store, &parent_resume, &child);
    let agent = app
        .world()
        .get::<Spawned>(parent)
        .unwrap()
        .iter()
        .find(|agent| {
            app.world()
                .get::<AgentId>(*agent)
                .is_some_and(|id| id.0 == child_id)
        })
        .unwrap();
    assert_eq!(
        app.world().get::<WorkDir>(agent).unwrap().0,
        fixture.project.join("nested").canonicalize().unwrap()
    );
    assert!(
        app.world()
            .get::<AppliedRole>(agent)
            .unwrap()
            .prompt
            .contains("FROZEN ROLE")
    );
    rig_harness_test_support::start_call(
        app.world_mut(),
        parent,
        ("message", "after-resume"),
        json!({"agent":child_id,"text":"One more detail"}),
    );
    run_until(&mut app, &wakes, |world| {
        child.request_count() == 2 && world.get::<ActiveTurn>(agent).is_none()
    });
    let history = format!("{:?}", child.requests()[1].chat_history);
    assert!(history.contains("CHILD_INITIAL") && history.contains("FROZEN ROLE"));
    assert!(!history.contains("CHANGED ROLE"));
}

#[test]
fn interrupted_workflow_resume_does_not_relaunch_or_admit_queued_children() {
    let fixture = Fixture::new();
    let store = MemoryStore::default();
    let child = MockCompletionModel::from_stream_turns(
        (0..8).map(|n| calls(&format!("block-{n}"), "block", json!({}))),
    );
    let tasks: Vec<_> = (0..6).map(|n| json!({"description":format!("Task {n}"),"prompt":"Wait","model":"test/child","tools":["block"]})).collect();
    let parent_model = MockCompletionModel::from_stream_turns([calls(
        "interrupted",
        "task_parallel",
        json!({"tasks":tasks}),
    )]);
    let (mut app, wakes, parent) =
        runtime_on(&fixture, &parent_model, &[("test/child", &child)], &store);
    app.add_open_tool(
        "block",
        "Wait.",
        ToolOptions::default(),
        |_: On<ToolCalled<Value>>| {},
    );
    send(&mut app, parent);
    run_until(&mut app, &wakes, |_| child.request_count() == 4);
    drop(app);
    let parent_resume = MockCompletionModel::from_stream_turns([reply("Restart explained.")]);
    let (mut app, wakes, parent) = resume(&fixture, &store, &parent_resume, &child);
    run_until(&mut app, &wakes, |world| {
        world.get::<ActiveTurn>(parent).is_none()
    });
    for _ in 0..8 {
        app.update();
    }
    assert_eq!(
        child.request_count(),
        4,
        "interrupted children must not execute again"
    );
    assert_eq!(
        app.world().get::<Spawned>(parent).unwrap().iter().count(),
        4
    );
    // Rig now stops restored turns unless a plugin explicitly opts in.
    // Ordinary coding delegation must not silently send another model request.
    assert_eq!(parent_resume.request_count(), 0);
    let conversation = app.world().get::<Conversation>(parent).unwrap();
    let restored = serde_json::to_string(&conversation.messages()).unwrap();
    assert!(
        restored.contains("interrupted") || restored.contains("stopped"),
        "{restored}"
    );
}
