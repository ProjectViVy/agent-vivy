use super::*;
use crate::runtime::tests::until;
use rig_core::test_utils::MockCompletionModel;
use rig_harness_test_support::{MemoryStore, app_on, connect, first_agent, reply};

pub(super) fn app(disk: &MemoryStore, model: &MockCompletionModel) -> (App, Entity) {
    let (mut app, _) = app_on(disk);
    app.add_plugins((
        AgentPlugin,
        SessionLogPlugin,
        DurablePlugin,
        AssistantContextPlugin,
    ));
    let connection = connect(model).unwrap();
    app.add_observer(move |added: On<Add<Agent>>, mut commands: Commands| {
        commands.entity(added.entity).insert(connection.clone());
    });
    app.finish();
    app.update();
    let agent = first_agent(app.world_mut()).unwrap();
    (app, agent)
}

#[test]
fn a_new_task_cannot_see_old_raw_input_or_reply_even_before_recap_exists() {
    let disk = MemoryStore::default();
    let model = MockCompletionModel::from_stream_turns([reply("OLD_REPLY_CANARY"), reply("done")]);
    let (mut app, agent) = app(&disk, &model);
    submit(app.world_mut(), agent, "test", "one", "OLD_INPUT_CANARY").unwrap();
    until(&mut app, |w| {
        inspect(w, agent).unwrap().tasks[0].status == Status::Completed
            && !w.entity(agent).contains::<ActiveTurn>()
    });
    submit(app.world_mut(), agent, "test", "two", "CURRENT_INPUT").unwrap();
    until(&mut app, |w| {
        inspect(w, agent).unwrap().tasks[1].status == Status::Completed
    });
    let request = format!("{:?}", model.requests()[1].chat_history);
    assert!(request.contains("CURRENT_INPUT"), "{request}");
    assert!(!request.contains("OLD_INPUT_CANARY"), "{request}");
    assert!(!request.contains("OLD_REPLY_CANARY"), "{request}");
}

#[test]
fn tool_results_survive_continuation_but_not_the_next_task() {
    use rig_harness_test_support::calls;
    let disk = MemoryStore::default();
    let model = MockCompletionModel::from_stream_turns([
        calls("tool-one", "inspect", json!({})),
        reply("done"),
        reply("new"),
    ]);
    let (mut application, _) = app_on(&disk);
    application.add_plugins((
        AgentPlugin,
        SessionLogPlugin,
        DurablePlugin,
        AssistantContextPlugin,
    ));
    application.add_open_tool(
        "inspect",
        "inspection",
        ToolOptions::default(),
        |e: On<ToolCalled<Value>>, mut commands: Commands| {
            commands
                .entity(e.call)
                .insert(Outcome::from(ToolResult::success(
                    rig_core::tool::ToolOutput::text("TOOL_CONTEXT_CANARY"),
                )));
        },
    );
    let connection = connect(&model).unwrap();
    application.add_observer(move |e: On<Add<Agent>>, mut commands: Commands| {
        commands.entity(e.entity).insert(connection.clone());
    });
    application.finish();
    application.update();
    let agent = first_agent(application.world_mut()).unwrap();
    submit(application.world_mut(), agent, "test", "one", "use tool").unwrap();
    until(&mut application, |w| {
        inspect(w, agent).unwrap().tasks[0].status == Status::Completed
            && !w.entity(agent).contains::<ActiveTurn>()
    });
    submit(application.world_mut(), agent, "test", "two", "new task").unwrap();
    until(&mut application, |w| {
        inspect(w, agent).unwrap().tasks[1].status == Status::Completed
    });
    let requests = model.requests();
    assert!(format!("{:?}", requests[1].chat_history).contains("TOOL_CONTEXT_CANARY"));
    assert!(!format!("{:?}", requests[2].chat_history).contains("TOOL_CONTEXT_CANARY"));
    for request in requests {
        pairing::validate(&request.chat_history).unwrap();
    }
}

#[test]
fn required_context_overflow_suspends_without_discarding_input_or_calling_model() {
    let disk = MemoryStore::default();
    let model = MockCompletionModel::from_stream_turns([reply("must not run")]);
    let (mut app, agent) = app(&disk, &model);
    let mut connection = connect(&model).unwrap();
    Arc::make_mut(&mut connection.spec).context_window = Some(1024);
    app.world_mut().entity_mut(agent).insert(connection);
    let text = "MUST_KEEP".repeat(5000);
    submit(app.world_mut(), agent, "test", "large", &text).unwrap();
    until(&mut app, |w| {
        matches!(
            inspect(w, agent).unwrap().tasks[0].status,
            Status::Completed | Status::Suspended | Status::Failed
        )
    });
    assert_eq!(model.request_count(), 0);
    assert_eq!(
        inspect(app.world(), agent).unwrap().tasks[0].status,
        Status::Suspended
    );
    assert_eq!(
        inspect(app.world(), agent).unwrap().tasks[0].input.text(),
        text
    );
}

#[test]
fn recap_commits_independently_and_is_the_only_cross_task_history() {
    let disk = MemoryStore::default();
    let model = MockCompletionModel::from_stream_turns([
        reply("OLD_RAW_ANSWER"),
        reply("COMMITTED_RECAP"),
        reply("new answer"),
    ]);
    let (mut app, agent) = app(&disk, &model);
    submit(app.world_mut(), agent, "test", "one", "OLD_RAW_INPUT").unwrap();
    until(&mut app, |w| {
        inspect(w, agent).unwrap().tasks[0].status == Status::Completed
            && !w.entity(agent).contains::<ActiveTurn>()
    });
    request_refresh(app.world_mut(), agent).unwrap();
    until(&mut app, |w| {
        storage::committed_context(w, agent).is_some_and(|s| s.recap_revision == 1)
    });
    assert_eq!(inspect(app.world(), agent).unwrap().tasks.len(), 1);
    submit(app.world_mut(), agent, "test", "two", "new input").unwrap();
    until(&mut app, |w| {
        inspect(w, agent).unwrap().tasks[1].status == Status::Completed
    });
    let requests = model.requests();
    assert_eq!(requests.len(), 3);
    assert!(requests[1].tools.is_empty());
    let foreground = format!("{:?}", requests[2].chat_history);
    assert!(foreground.contains("COMMITTED_RECAP"));
    assert!(!foreground.contains("OLD_RAW_ANSWER"));
    assert!(!foreground.contains("OLD_RAW_INPUT"));
}

#[test]
fn saved_recap_result_publishes_after_reopen_without_another_model_request() {
    let disk = MemoryStore::default();
    let model = MockCompletionModel::from_stream_turns([reply("answer"), reply("SAVED_RECAP")]);
    let (mut first, agent) = app(&disk, &model);
    submit(first.world_mut(), agent, "test", "one", "remember this").unwrap();
    until(&mut first, |w| {
        storage::committed_context(w, agent)
            .is_some_and(|s| s.jobs.iter().any(|j| j.result.is_some()) && s.recap_revision == 0)
    });
    drop(first);
    let replacement = MockCompletionModel::from_stream_turns([reply("MUST_NOT_CALL")]);
    let (mut second, agent) = app(&disk, &replacement);
    until(&mut second, |w| {
        storage::committed_context(w, agent).is_some_and(|s| s.summary == "SAVED_RECAP")
    });
    assert_eq!(replacement.request_count(), 0);
}

#[test]
fn foreground_cancels_prepared_recap_without_delaying_new_input() {
    let disk = MemoryStore::default();
    let model =
        MockCompletionModel::from_stream_turns([reply("first"), reply("foreground answer")]);
    let (mut app, agent) = app(&disk, &model);
    submit(app.world_mut(), agent, "test", "one", "old input").unwrap();
    until(&mut app, |w| {
        w.get::<AssistantContextState>(agent)
            .is_some_and(|s| s.jobs.iter().any(|j| j.phase == "prepared"))
    });
    submit(app.world_mut(), agent, "test", "two", "urgent new input").unwrap();
    until(&mut app, |w| {
        inspect(w, agent).unwrap().tasks[1].status == Status::Completed
    });
    let state = app.world().get::<AssistantContextState>(agent).unwrap();
    assert_eq!(state.jobs[0].phase, "cancelled");
    assert_eq!(state.recap_revision, 0);
    assert_eq!(model.request_count(), 2);
}

#[test]
fn maintenance_commands_do_not_create_or_strand_user_tasks() {
    let disk = MemoryStore::default();
    let model = MockCompletionModel::from_stream_turns([reply("normal")]);
    let (mut app, agent) = app(&disk, &model);
    app.world_mut()
        .trigger(Deliver::new(agent, "/recap status", DeliveryMode::Queue));
    for _ in 0..15 {
        app.update();
    }
    assert!(inspect(app.world(), agent).unwrap().tasks.is_empty());
    assert!(inspect(app.world(), agent).unwrap().active.is_none());
    assert_eq!(model.request_count(), 0);
}

#[test]
fn malformed_saved_context_is_rejected_instead_of_becoming_empty_or_advancing_coverage() {
    let mut value = serde_json::to_value(AssistantContextState::default()).unwrap();
    value["recap_revision"] = json!(3);
    value["covered"] = json!({"task:missing/input":123});
    assert!(serde_json::from_value::<AssistantContextState>(value).is_err());
}

#[test]
fn transient_recap_failure_retries_once_without_recapturing_the_old_result() {
    use rig_core::{
        ProviderResponseError,
        test_utils::{MockError, MockStreamEvent},
    };
    let disk = MemoryStore::default();
    let failure = MockStreamEvent::Error(MockError::ProviderResponse(
        ProviderResponseError::without_status("busy").with_transient(Some(true)),
    ));
    let model = MockCompletionModel::from_stream_turns([
        reply("answer"),
        vec![failure],
        reply("RECOVERED_RECAP"),
    ]);
    let (mut app, agent) = app(&disk, &model);
    submit(app.world_mut(), agent, "test", "retry-recap", "input").unwrap();
    until(&mut app, |w| {
        storage::committed_context(w, agent).is_some_and(|s| s.summary == "RECOVERED_RECAP")
    });
    assert_eq!(model.request_count(), 3);
    assert_eq!(
        storage::committed_context(app.world(), agent).unwrap().jobs[0].attempt,
        1
    );
}

#[test]
fn foreground_retry_keeps_the_persona_and_request_frozen_before_the_failure() {
    use rig_core::{
        ProviderResponseError,
        test_utils::{MockError, MockStreamEvent},
    };
    let disk = MemoryStore::default();
    let failure = MockStreamEvent::Error(MockError::ProviderResponse(
        ProviderResponseError::without_status("busy").with_transient(Some(true)),
    ));
    let model = MockCompletionModel::from_stream_turns([vec![failure], reply("done")]);
    let root = std::env::temp_dir().join(format!("d2-retry-persona-{}", std::process::id()));
    std::fs::create_dir_all(&root).unwrap();
    let store = vivy_laputa::PersonaStore::new(&root);
    store.update(0, "one", "PERSONA_BEFORE", vec![]).unwrap();
    let (mut app, _) = app_on(&disk);
    app.insert_resource(vivy_laputa::PersonaSource(root.clone()))
        .add_plugins((
            AgentPlugin,
            SessionLogPlugin,
            DurablePlugin,
            AssistantContextPlugin,
            vivy_laputa::LaputaPlugin,
        ));
    let connection = connect(&model).unwrap();
    app.add_observer(move |e: On<Add<Agent>>, mut commands: Commands| {
        commands.entity(e.entity).insert(connection.clone());
    });
    app.finish();
    app.update();
    let agent = first_agent(app.world_mut()).unwrap();
    submit(app.world_mut(), agent, "test", "retry", "input").unwrap();
    until(&mut app, |w| {
        committed(w, agent).is_some_and(|s| s.operations.iter().any(|o| o.retry_at_ms > 0))
    });
    store.update(1, "two", "PERSONA_AFTER", vec![]).unwrap();
    until(&mut app, |w| {
        inspect(w, agent).unwrap().tasks[0].status == Status::Completed
    });
    let requests = model.requests();
    assert_eq!(
        serde_json::to_value(&requests[0]).unwrap(),
        serde_json::to_value(&requests[1]).unwrap()
    );
    assert!(format!("{:?}", requests[1]).contains("PERSONA_BEFORE"));
    std::fs::remove_dir_all(root).unwrap();
}

#[test]
fn delivered_task_switch_and_reopen_do_not_import_the_previous_tasks_raw_context() {
    use rig_harness_test_support::calls;
    for reopen in [false, true] {
        let disk = MemoryStore::default();
        let model = MockCompletionModel::from_stream_turns([
            calls(
                "defer-one",
                "task_control",
                json!({"action":"defer","task":"task:1","generation":0}),
            ),
            reply("second done"),
        ]);
        let (mut first, agent) = app(&disk, &model);
        submit(
            first.world_mut(),
            agent,
            "test",
            "one",
            "PREVIOUS_TASK_ONLY",
        )
        .unwrap();
        submit(first.world_mut(), agent, "test", "two", "SECOND_TASK_ONLY").unwrap();
        until(&mut first, |w| {
            w.get::<ActiveTurn>(agent).is_none()
                && committed(w, agent).is_some_and(|s| {
                    s.task("task:1").unwrap().status == Status::Suspended
                        && s.task("task:2").unwrap().delivered
                })
        });
        let replacement = MockCompletionModel::from_stream_turns([reply("second done")]);
        let (mut current, agent) = if reopen {
            drop(first);
            app(&disk, &replacement)
        } else {
            (first, agent)
        };
        until(&mut current, |w| {
            committed(w, agent)
                .is_some_and(|s| s.task("task:2").unwrap().status == Status::Completed)
        });
        let requests = if reopen {
            replacement.requests()
        } else {
            model.requests()
        };
        let last = requests.last().unwrap();
        let text = serde_json::to_string(last).unwrap();
        assert!(
            text.contains("SECOND_TASK_ONLY") && !text.contains("PREVIOUS_TASK_ONLY"),
            "{text}"
        );
        pairing::validate(&last.chat_history).unwrap();
    }
}
