use super::*;
use rig_core::test_utils::MockCompletionModel;
use rig_harness_test_support::{
    AnswerAsks, MemoryStore, answer_open, app_on, calls, connect, first_agent, reply, run_until,
};
use serde_json::json;

fn setup(
    store: &MemoryStore,
    model: &MockCompletionModel,
    interactive: bool,
) -> (App, std::sync::mpsc::Receiver<()>, Entity) {
    let (mut app, wakes) = app_on(store);
    app.add_plugins((AgentPlugin, SessionLogPlugin, PlanPlugin));
    if interactive {
        app.add_plugins(AnswerAsks);
    }
    app.finish();
    app.update();
    let agent = first_agent(app.world_mut()).unwrap();
    app.world_mut()
        .entity_mut(agent)
        .insert(connect(model).unwrap());
    (app, wakes, agent)
}

fn command(app: &mut App, agent: Entity, text: &str) {
    app.world_mut().trigger(RunCommand::new(agent, text));
    app.update();
}

fn send(app: &mut App, agent: Entity, text: &str) {
    app.world_mut()
        .trigger(Deliver::new(agent, text, DeliveryMode::Queue));
}

fn review(world: &World) -> Option<Entity> {
    world
        .iter_entities()
        .find(|entity| entity.contains::<PlanReview>() && !entity.contains::<Outcome>())
        .map(|entity| entity.id())
}

fn active(world: &World, agent: Entity) -> bool {
    world.get::<PlanState>(agent).unwrap().active
}

fn system(request: &rig_core::completion::CompletionRequest) -> String {
    format!("{:?}", request.chat_history)
}

#[test]
fn plan_context_and_subagents_have_explicit_native_stage_order() {
    for subagents in [false, true] {
        let (mut app, _) = app_on(&MemoryStore::default());
        app.add_plugins((
            AgentPlugin,
            SessionLogPlugin,
            PlanPlugin,
            crate::context::ContextFilesPlugin,
        ));
        if subagents {
            app.add_plugins(crate::subagents::VivySubagentsPlugin);
        }
        app.finish();
        app.update();
        let conflicts = rig_harness::stage_conflicts(app.world_mut());
        assert!(conflicts.is_empty(), "{}", conflicts.join("\n"));
    }
}

#[test]
fn mode_changes_guidance_without_changing_tools_or_reviewing_plain_answers() {
    let model =
        MockCompletionModel::from_stream_turns([reply("Clarify the scope."), reply("Ready.")]);
    let (mut app, wakes, agent) = setup(&MemoryStore::default(), &model, true);
    command(&mut app, agent, "plan investigate");
    run_until(&mut app, &wakes, |world| {
        !world.entity(agent).contains::<ActiveTurn>() && model.request_count() == 1
    });
    assert!(active(app.world(), agent));
    assert!(system(&model.requests()[0]).contains(PLAN_GUIDANCE));
    assert!(
        review(app.world_mut()).is_none(),
        "ordinary answers are not submitted plans"
    );
    command(&mut app, agent, "plan off");
    send(&mut app, agent, "continue");
    run_until(&mut app, &wakes, |world| {
        !world.entity(agent).contains::<ActiveTurn>() && model.request_count() == 2
    });
    let requests = model.requests();
    assert!(!system(&requests[1]).contains(PLAN_GUIDANCE));
    assert_eq!(requests[0].tools, requests[1].tools);
}

#[test]
fn approval_returns_to_the_same_turn_with_plan_guidance_removed() {
    let model = MockCompletionModel::from_stream_turns([
        calls(
            "p1",
            "exit_plan_mode",
            json!({"plan":"# Changes\nUse existing code."}),
        ),
        reply("Implemented."),
    ]);
    let (mut app, wakes, agent) = setup(&MemoryStore::default(), &model, true);
    command(&mut app, agent, "plan implement");
    run_until(&mut app, &wakes, |world| review(world).is_some());
    assert!(active(app.world(), agent));
    assert_eq!(model.request_count(), 1);
    assert!(answer_open(
        app.world_mut(),
        REVIEW_TITLE,
        &Answer::choice("approve")
    ));
    run_until(&mut app, &wakes, |world| {
        !world.entity(agent).contains::<ActiveTurn>()
    });
    assert!(!active(app.world(), agent));
    assert_eq!(
        model.request_count(),
        2,
        "approval continues once, not another delivered task"
    );
    assert!(!system(&model.requests()[1]).contains(PLAN_GUIDANCE));
    assert_eq!(
        app.world()
            .get::<Conversation>(agent)
            .unwrap()
            .turns()
            .count(),
        1
    );
}

#[test]
fn feedback_keeps_plan_and_reaches_the_model() {
    let model = MockCompletionModel::from_stream_turns([
        calls(
            "p1",
            "exit_plan_mode",
            json!({"plan":"# Changes\nFirst draft."}),
        ),
        reply("I will simplify it."),
    ]);
    let (mut app, wakes, agent) = setup(&MemoryStore::default(), &model, true);
    command(&mut app, agent, "plan plan it");
    run_until(&mut app, &wakes, |world| review(world).is_some());
    assert!(answer_open(
        app.world_mut(),
        REVIEW_TITLE,
        &Answer::choice("revise").with_text("Use fewer components")
    ));
    run_until(&mut app, &wakes, |world| {
        !world.entity(agent).contains::<ActiveTurn>()
    });
    assert!(active(app.world(), agent));
    let second = system(&model.requests()[1]);
    assert!(second.contains("Use fewer components"));
    assert!(second.contains(PLAN_GUIDANCE));
}

#[test]
fn dismissal_stops_the_turn_and_preserves_plan() {
    let model = MockCompletionModel::from_stream_turns([
        calls(
            "p1",
            "exit_plan_mode",
            json!({"plan":"# Changes\nFirst draft."}),
        ),
        reply("This must not be requested."),
    ]);
    let (mut app, wakes, agent) = setup(&MemoryStore::default(), &model, true);
    command(&mut app, agent, "plan draft");
    run_until(&mut app, &wakes, |world| review(world).is_some());
    assert!(answer_open(
        app.world_mut(),
        REVIEW_TITLE,
        &Answer::default()
    ));
    run_until(&mut app, &wakes, |world| {
        !world.entity(agent).contains::<ActiveTurn>()
    });
    assert!(active(app.world(), agent));
    assert_eq!(model.request_count(), 1);
}

#[test]
fn no_interactive_front_stops_without_an_approval_loop() {
    let model = MockCompletionModel::from_stream_turns([
        calls(
            "p1",
            "exit_plan_mode",
            json!({"plan":"# Changes\nFirst draft."}),
        ),
        reply("This must not be requested."),
    ]);
    let (mut app, wakes, agent) = setup(&MemoryStore::default(), &model, false);
    command(&mut app, agent, "plan draft");
    run_until(&mut app, &wakes, |world| {
        !world.entity(agent).contains::<ActiveTurn>() && model.request_count() == 1
    });
    assert!(active(app.world(), agent));
    assert!(review(app.world_mut()).is_none());
}

#[test]
fn an_exit_command_withdraws_review_and_does_not_accept_old_answers() {
    let model = MockCompletionModel::from_stream_turns([
        calls(
            "p1",
            "exit_plan_mode",
            json!({"plan":"# Changes\nFirst draft."}),
        ),
        reply("This must not be requested."),
    ]);
    let (mut app, wakes, agent) = setup(&MemoryStore::default(), &model, true);
    command(&mut app, agent, "plan draft");
    run_until(&mut app, &wakes, |world| review(world).is_some());
    command(&mut app, agent, "plan off");
    run_until(&mut app, &wakes, |world| {
        !world.entity(agent).contains::<ActiveTurn>()
    });
    assert!(!active(app.world(), agent));
    assert!(review(app.world_mut()).is_none());
    assert!(!answer_open(
        app.world_mut(),
        REVIEW_TITLE,
        &Answer::choice("approve")
    ));
    assert_eq!(model.request_count(), 1);
}

#[test]
fn state_survives_restore_and_new_sessions_start_without_plan() {
    let store = MemoryStore::default();
    let model = MockCompletionModel::from_stream_turns([reply("Investigating.")]);
    let (mut app, wakes, agent) = setup(&store, &model, true);
    command(&mut app, agent, "plan investigate");
    run_until(&mut app, &wakes, |world| {
        !world.entity(agent).contains::<ActiveTurn>() && model.request_count() == 1
    });
    app.update();
    drop(app);
    let (app, _, agent) = setup(&store, &model, false);
    assert!(active(app.world(), agent));
    let (app, _, agent) = setup(&MemoryStore::default(), &model, false);
    assert!(!active(app.world(), agent));
}

#[test]
fn mutation_tools_are_still_offered_and_can_run_while_planning() {
    let model = MockCompletionModel::from_stream_turns([
        calls("w1", "change", json!({})),
        reply("Observed."),
    ]);
    let (mut app, wakes, agent) = setup(&MemoryStore::default(), &model, true);
    app.add_open_tool(
        "change",
        "Change something",
        ToolOptions::default(),
        |call: On<ToolCalled<serde_json::Value>>, mut decisions: Query<&mut Decision>| {
            decisions.get_mut(call.call).unwrap().decide(
                Decider::plugin("test"),
                Outcome::from(ToolResult::success("changed".into())),
            );
        },
    );
    command(&mut app, agent, "plan investigate");
    run_until(&mut app, &wakes, |world| {
        !world.entity(agent).contains::<ActiveTurn>() && model.request_count() == 2
    });
    assert!(active(app.world(), agent));
    assert!(
        model.requests()[0]
            .tools
            .iter()
            .any(|tool| tool.name.as_str() == "change")
    );
}

#[test]
fn stale_approval_ends_its_call_without_leaving_plan_or_hanging() {
    let model = MockCompletionModel::from_stream_turns([
        calls("p1", "exit_plan_mode", json!({"plan":"# First draft"})),
        reply("This must not be requested."),
    ]);
    let (mut app, wakes, agent) = setup(&MemoryStore::default(), &model, true);
    command(&mut app, agent, "plan draft");
    run_until(&mut app, &wakes, |world| review(world).is_some());
    app.world_mut()
        .get_mut::<PlanState>(agent)
        .unwrap()
        .revision += 1;
    assert!(answer_open(
        app.world_mut(),
        REVIEW_TITLE,
        &Answer::choice("approve")
    ));
    for _ in 0..10 {
        app.update();
    }
    assert!(active(app.world(), agent));
    assert!(
        !app.world().entity(agent).contains::<ActiveTurn>(),
        "an expired review must not leave its call waiting forever"
    );
    assert_eq!(model.request_count(), 1);
}

#[test]
fn typing_plan_off_in_the_review_is_a_command_not_revision_feedback() {
    let model = MockCompletionModel::from_stream_turns([
        calls("p1", "exit_plan_mode", json!({"plan":"# First draft"})),
        reply("This must not be requested."),
    ]);
    let (mut app, wakes, agent) = setup(&MemoryStore::default(), &model, true);
    command(&mut app, agent, "plan draft");
    run_until(&mut app, &wakes, |world| review(world).is_some());
    assert!(answer_open(
        app.world_mut(),
        REVIEW_TITLE,
        &Answer::text("/plan off")
    ));
    run_until(&mut app, &wakes, |world| {
        !world.entity(agent).contains::<ActiveTurn>()
    });
    assert!(!active(app.world(), agent));
    assert_eq!(model.request_count(), 1);
}

#[test]
fn mode_changes_during_a_tool_apply_to_the_next_model_request() {
    for initial in [false, true] {
        let model = MockCompletionModel::from_stream_turns([
            calls("h1", "hold", json!({})),
            reply("Ready."),
        ]);
        let (mut app, wakes, agent) = setup(&MemoryStore::default(), &model, true);
        app.add_open_tool(
            "hold",
            "Wait",
            ToolOptions::default(),
            |_: On<ToolCalled<serde_json::Value>>| {},
        );
        if initial {
            command(&mut app, agent, "plan");
        }
        send(&mut app, agent, "work");
        run_until(&mut app, &wakes, |world| {
            world
                .iter_entities()
                .any(|entity| entity.contains::<ToolRequest>() && entity.contains::<Started>())
        });
        let call = app
            .world_mut()
            .query_filtered::<Entity, (With<ToolRequest>, With<Started>)>()
            .single(app.world())
            .unwrap();
        command(&mut app, agent, if initial { "plan off" } else { "plan" });
        assert_eq!(active(app.world(), agent), initial);
        assert!(app.world().get::<PendingMode>(agent).is_some());
        decide_call(
            app.world_mut(),
            call,
            Outcome::from(ToolResult::success("ready".into())),
        );
        run_until(&mut app, &wakes, |world| {
            !world.entity(agent).contains::<ActiveTurn>()
        });
        assert_eq!(active(app.world(), agent), !initial);
        assert_eq!(
            system(&model.requests()[0]).contains(PLAN_GUIDANCE),
            initial
        );
        assert_eq!(
            system(&model.requests()[1]).contains(PLAN_GUIDANCE),
            !initial
        );
    }
}

#[test]
fn forks_copy_mode_but_not_pending_review_or_pending_mode() {
    use rig_harness::rig_ecs::fork::ForkAgent;
    let model = MockCompletionModel::from_stream_turns([reply("Investigating.")]);
    let (mut app, wakes, agent) = setup(&MemoryStore::default(), &model, true);
    command(&mut app, agent, "plan investigate");
    run_until(&mut app, &wakes, |world| {
        !world.entity(agent).contains::<ActiveTurn>() && model.request_count() == 1
    });
    app.world_mut().entity_mut(agent).insert(PendingMode(false));
    app.world_mut().trigger(ForkAgent {
        agent,
        through: None,
    });
    app.update();
    let fork = app
        .world()
        .iter_entities()
        .find(|entity| entity.contains::<Agent>() && entity.id() != agent)
        .unwrap()
        .id();
    assert!(active(app.world(), fork));
    assert!(app.world().get::<PendingMode>(fork).is_none());
    assert!(reviews_of(app.world_mut(), fork).is_empty());
}

#[test]
fn malformed_submission_is_refused_and_the_model_can_revise() {
    let model = MockCompletionModel::from_stream_turns([
        calls("p1", "exit_plan_mode", json!({"plan":"not a titled plan"})),
        reply("I will add a title."),
    ]);
    let (mut app, wakes, agent) = setup(&MemoryStore::default(), &model, true);
    command(&mut app, agent, "plan draft");
    run_until(&mut app, &wakes, |world| {
        !world.entity(agent).contains::<ActiveTurn>() && model.request_count() == 2
    });
    assert!(active(app.world(), agent));
    assert!(review(app.world()).is_none());
    assert!(system(&model.requests()[1]).contains("starting with a # title"));
}

#[test]
fn restoring_an_unfinished_review_never_replays_approval() {
    let store = MemoryStore::default();
    let model = MockCompletionModel::from_stream_turns([
        calls("p1", "exit_plan_mode", json!({"plan":"# Pending draft"})),
        reply("This must not be requested."),
    ]);
    let (mut app, wakes, agent) = setup(&store, &model, true);
    command(&mut app, agent, "plan draft");
    run_until(&mut app, &wakes, |world| review(world).is_some());
    app.update();
    drop(app);
    let (mut app, _, agent) = setup(&store, &model, false);
    for _ in 0..10 {
        app.update();
    }
    assert!(active(app.world(), agent));
    assert!(review(app.world()).is_none());
    assert!(app.world().get::<PendingMode>(agent).is_none());
    assert_eq!(model.request_count(), 1);
}
