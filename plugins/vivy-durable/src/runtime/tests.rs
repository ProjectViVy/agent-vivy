use super::*;
use rig_core::test_utils::MockCompletionModel;
use rig_harness_test_support::{MemoryStore, app_on, connect, first_agent, reply};

#[cfg(feature = "wake")]
#[test]
fn wait_closes_turn_but_keeps_the_task_and_makes_no_idle_model_calls() {
    let disk = MemoryStore::default();
    let model = MockCompletionModel::from_stream_turns([
        rig_harness_test_support::calls(
            "wait-1",
            "task_control",
            json!({
                "action":"wait", "task":"task:1", "generation":0,
                "seconds":3600, "reason":"check later"
            }),
        ),
        reply("must not be called while waiting"),
    ]);
    let (mut app, agent) = app(&disk, &model);
    submit(
        app.world_mut(),
        agent,
        "test",
        "one",
        "wait and continue later",
    )
    .unwrap();
    until(&mut app, |w| {
        !w.entity(agent).contains::<ActiveTurn>()
            && inspect(w, agent).unwrap().tasks[0].outcome.is_some()
    });
    let state = inspect(app.world(), agent).unwrap();
    assert_eq!(
        serde_json::to_value(&state.tasks[0].status).unwrap(),
        "waiting"
    );
    assert_eq!(state.tasks.len(), 1);
    for _ in 0..20 {
        app.update();
    }
    assert_eq!(model.request_count(), 1);
}

fn app(disk: &MemoryStore, model: &MockCompletionModel) -> (App, Entity) {
    let (mut app, _) = app_on(disk);
    app.add_plugins((AgentPlugin, SessionLogPlugin, DurablePlugin));
    let connection = connect(model).unwrap();
    app.add_observer(move |added: On<Add<Agent>>, mut commands: Commands| {
        commands.entity(added.entity).insert(connection.clone());
    });
    app.finish();
    app.update();
    let agent = first_agent(app.world_mut()).unwrap();
    (app, agent)
}

pub(super) fn until(app: &mut App, predicate: impl Fn(&World) -> bool) {
    let deadline = std::time::Instant::now() + std::time::Duration::from_secs(12);
    loop {
        app.update();
        if predicate(app.world()) {
            return;
        }
        if std::time::Instant::now() >= deadline {
            let state: Vec<_> = app
                .world_mut()
                .query::<&DurableAgentState>()
                .iter(app.world())
                .cloned()
                .collect();
            let calls: Vec<_> = app
                .world_mut()
                .query::<(
                    Entity,
                    Option<&Bound>,
                    Option<&Held>,
                    Option<&Outcome>,
                    Option<&FinalOutcome>,
                    Option<&Settled>,
                    Option<&Started>,
                )>()
                .iter(app.world())
                .filter(|(_, b, h, o, f, _, _)| {
                    b.is_some() || h.is_some() || o.is_some() || f.is_some()
                })
                .map(|(e, b, h, o, f, s, t)| {
                    format!(
                        "{e:?} bound {:?} held {} outcome {o:?} final {f:?} settled {} started {}",
                        b.map(|b| &b.operation),
                        h.is_some(),
                        s.is_some(),
                        t.is_some()
                    )
                })
                .collect();
            panic!("runtime deadline: {state:?}\n{calls:?}");
        }
        std::thread::sleep(std::time::Duration::from_millis(2));
    }
}

#[test]
fn admission_deduplicates_across_restart_and_rejects_changed_payload() {
    let disk = MemoryStore::default();
    let model = MockCompletionModel::from_stream_turns([reply("done")]);
    let (mut first, agent) = app(&disk, &model);
    let receipt = submit(first.world_mut(), agent, "test", "request-1", "hello").unwrap();
    assert!(!receipt.committed);
    until(&mut first, |world| {
        inspect(world, agent).unwrap().tasks[0].status == Status::Completed
    });
    assert!(submit(first.world_mut(), agent, "test", "request-1", "changed").is_err());
    drop(first);
    let (mut second, agent) = app(&disk, &model);
    let duplicate = submit(second.world_mut(), agent, "test", "request-1", "hello").unwrap();
    assert_eq!(receipt.task, duplicate.task);
    assert!(duplicate.committed);
    for _ in 0..8 {
        second.update();
    }
    assert_eq!(model.request_count(), 1);
}

#[test]
fn no_model_or_tool_dispatch_before_exact_intent_checkpoint() {
    let disk = MemoryStore::default();
    let model = MockCompletionModel::from_stream_turns([reply("done")]);
    let (mut app, agent) = app(&disk, &model);
    submit(app.world_mut(), agent, "test", "1", "hello").unwrap();
    for _ in 0..8 {
        app.world_mut().run_schedule(AgentPass);
    }
    assert_eq!(model.request_count(), 0);
    until(&mut app, |world| {
        inspect(world, agent).unwrap().tasks[0].status == Status::Completed
    });
    assert_eq!(model.request_count(), 1);
}

#[test]
fn every_committed_admission_boundary_resumes_once() {
    // Includes the gap between choosing the active task and delivering its input.
    for frames in 1..=6 {
        let disk = MemoryStore::default();
        let model = MockCompletionModel::from_stream_turns([reply("done")]);
        let (mut first, agent) = app(&disk, &model);
        submit(first.world_mut(), agent, "test", "1", "hello").unwrap();
        for _ in 0..frames {
            first.update();
        }
        drop(first);
        let replacement = MockCompletionModel::from_stream_turns([reply("done")]);
        let (mut second, agent) = app(&disk, &replacement);
        until(&mut second, |world| {
            inspect(world, agent).unwrap().tasks[0].status == Status::Completed
        });
        assert_eq!(inspect(second.world(), agent).unwrap().tasks.len(), 1);
    }
}

#[test]
fn saved_model_result_survives_restart_without_dispatch() {
    let disk = MemoryStore::default();
    let model = MockCompletionModel::from_stream_turns([reply("saved answer")]);
    let (mut first, agent) = app(&disk, &model);
    submit(first.world_mut(), agent, "test", "1", "hello").unwrap();
    until(&mut first, |world| {
        committed(world, agent).is_some_and(|s| {
            s.operations
                .iter()
                .any(|o| o.kind == "model" && o.outcome.is_some() && !o.published)
        })
    });
    drop(first);
    let replacement = MockCompletionModel::from_stream_turns([reply("must not call")]);
    let (mut second, agent) = app(&disk, &replacement);
    until(&mut second, |world| {
        inspect(world, agent).unwrap().tasks[0].status == Status::Completed
    });
    assert_eq!(replacement.request_count(), 0);
    assert_eq!(model.request_count(), 1);
}

#[derive(Resource, Default)]
struct ToolStarts(usize);
fn tool_app(disk: &MemoryStore, model: &MockCompletionModel) -> (App, Entity) {
    let (mut app, _) = app_on(disk);
    app.add_plugins((AgentPlugin, SessionLogPlugin, DurablePlugin));
    let connection = connect(model).unwrap();
    app.add_observer(move |added: On<Add<Agent>>, mut commands: Commands| {
        commands.entity(added.entity).insert(connection.clone());
    });
    app.add_open_tool(
        "mutate",
        "test side effect",
        ToolOptions::default(),
        |_: On<ToolCalled<Value>>| {},
    );
    app.init_resource::<ToolStarts>().add_observer(
        |event: On<Add<Started>>,
         tools: Query<(), With<ToolRequest>>,
         mut count: ResMut<ToolStarts>| {
            if tools.contains(event.entity) {
                count.0 += 1;
            }
        },
    );
    app.finish();
    app.update();
    let agent = first_agent(app.world_mut()).unwrap();
    (app, agent)
}

#[test]
fn interrupted_tool_is_unknown_and_never_redispatched() {
    use rig_harness_test_support::calls;
    let disk = MemoryStore::default();
    let model = MockCompletionModel::from_stream_turns([calls("side-effect", "mutate", json!({}))]);
    let (mut first, agent) = tool_app(&disk, &model);
    submit(first.world_mut(), agent, "test", "1", "act").unwrap();
    until(&mut first, |world| {
        world
            .iter_entities()
            .any(|e| e.contains::<ToolRequest>() && e.contains::<Started>())
    });
    drop(first);
    let model = MockCompletionModel::from_stream_turns([reply("inspect before proceeding")]);
    let (mut second, agent) = tool_app(&disk, &model);
    until(&mut second, |world| {
        committed(world, agent).is_some_and(|s| {
            s.operations
                .iter()
                .any(|o| o.phase == "unknown" && o.outcome.is_some())
        })
    });
    assert_eq!(second.world().resource::<ToolStarts>().0, 0);
    let state = inspect(second.world(), agent).unwrap();
    assert_eq!(state.tasks[0].status, Status::Suspended);
    let unknown = state
        .operations
        .iter()
        .find(|o| o.phase == "unknown")
        .unwrap()
        .clone();
    resolve(
        second.world_mut(),
        agent,
        &unknown.id,
        unknown.generation,
        "external receipt verified",
    )
    .unwrap();
    until(&mut second, |world| {
        inspect(world, agent).unwrap().tasks[0].status == Status::Completed
    });
}

#[test]
fn new_input_closes_unstarted_siblings_at_one_tool_boundary() {
    use rig_core::test_utils::MockStreamEvent;
    use rig_harness_test_support::calls;
    let mut two = calls("one", "mutate", json!({}));
    let second = calls("two", "mutate", json!({}));
    // Compose two tool-call stream events, keeping one final usage event.
    two.retain(|e| !matches!(e, MockStreamEvent::FinalResponse(_)));
    two.extend(second);
    let disk = MemoryStore::default();
    let model = MockCompletionModel::from_stream_turns([two, reply("considered new input")]);
    let (mut app, agent) = tool_app(&disk, &model);
    submit(app.world_mut(), agent, "test", "1", "act").unwrap();
    until(&mut app, |world| {
        world
            .iter_entities()
            .any(|e| e.contains::<ToolRequest>() && e.contains::<Started>())
    });
    submit(app.world_mut(), agent, "test", "2", "please reconsider").unwrap();
    let call = app
        .world()
        .iter_entities()
        .find(|e| e.contains::<ToolRequest>() && e.contains::<Started>())
        .unwrap()
        .id();
    app.world_mut()
        .entity_mut(call)
        .insert(Outcome::from(ToolResult::success(
            rig_core::tool::ToolOutput::text("first done"),
        )));
    until(&mut app, |_| model.request_count() == 2);
    let sent = format!("{:?}", model.requests()[1]);
    assert!(
        sent.contains("not_run") && sent.contains("please reconsider"),
        "{sent}"
    );
    assert_eq!(app.world().resource::<ToolStarts>().0, 1);
}

#[test]
fn cancellation_generation_survives_restart_and_fences_late_results() {
    use rig_harness_test_support::calls;
    let disk = MemoryStore::default();
    let model = MockCompletionModel::from_stream_turns([calls("one", "mutate", json!({}))]);
    let (mut first, agent) = tool_app(&disk, &model);
    let task = submit(first.world_mut(), agent, "test", "1", "act")
        .unwrap()
        .task;
    until(&mut first, |world| {
        world
            .iter_entities()
            .any(|e| e.contains::<ToolRequest>() && e.contains::<Started>())
    });
    let call = first
        .world()
        .iter_entities()
        .find(|e| e.contains::<ToolRequest>() && e.contains::<Started>())
        .unwrap()
        .id();
    cancel(first.world_mut(), agent, &task, 0).unwrap();
    first.world_mut().flush();
    assert!(
        !first
            .world()
            .iter_entities()
            .any(|e| e.contains::<Running>())
    );
    first
        .world_mut()
        .entity_mut(call)
        .insert(Outcome::from(ToolResult::success(
            rig_core::tool::ToolOutput::text("late success"),
        )));
    until(&mut first, |world| world.get::<ActiveTurn>(agent).is_none());
    assert!(
        committed(first.world(), agent)
            .unwrap()
            .task(&task)
            .is_some_and(|t| t.status == Status::Cancelled && t.generation == 1)
    );
    drop(first);
    let replacement = MockCompletionModel::from_stream_turns([reply("must not run")]);
    let (mut second, agent) = tool_app(&disk, &replacement);
    for _ in 0..8 {
        second.update();
    }
    assert_eq!(replacement.request_count(), 0);
    assert_eq!(
        inspect(second.world(), agent)
            .unwrap()
            .task(&task)
            .unwrap()
            .status,
        Status::Cancelled
    );
}

#[test]
fn finish_silently_commits_control_before_ending_without_another_model_call() {
    use rig_harness_test_support::calls;
    let disk = MemoryStore::default();
    let model = MockCompletionModel::from_stream_turns([calls(
        "control",
        "task_control",
        json!({"action":"finish_silently","task":"task:1","generation":0}),
    )]);
    let (mut app, agent) = app(&disk, &model);
    submit(
        app.world_mut(),
        agent,
        "test",
        "1",
        "finish without a message",
    )
    .unwrap();
    until(&mut app, |world| {
        committed(world, agent).is_some_and(|s| {
            s.task("task:1")
                .is_some_and(|t| t.status == Status::Completed)
        }) && world.get::<ActiveTurn>(agent).is_none()
    });
    assert_eq!(model.request_count(), 1);
    assert!(
        app.world()
            .get::<Conversation>(agent)
            .unwrap()
            .pending_calls()
            .is_empty()
    );
}

#[test]
fn human_stop_wins_over_pending_controls_before_and_after_checkpoint() {
    use rig_harness_test_support::calls;
    for action in ["continue", "switch", "defer", "finish_silently"] {
        for (save_control, reopen) in [(false, false), (false, true), (true, false), (true, true)] {
            let disk = MemoryStore::default();
            let target = if action == "switch" {
                "task:2"
            } else {
                "task:1"
            };
            let model = MockCompletionModel::from_stream_turns([calls(
                "control",
                "task_control",
                json!({"action":action,"task":target,"generation":0}),
            )]);
            let (mut app, agent) = app(&disk, &model);
            submit(app.world_mut(), agent, "test", "1", "first").unwrap();
            if action == "switch" {
                submit(app.world_mut(), agent, "test", "2", "second").unwrap();
            }
            until(&mut app, |w| {
                committed(w, agent).is_some_and(|s| {
                    s.operations
                        .iter()
                        .any(|o| o.kind == "tool" && o.outcome.is_none())
                })
            });
            for _ in 0..20 {
                app.world_mut().run_schedule(First);
                app.world_mut().run_schedule(AgentPass);
                if inspect(app.world(), agent).unwrap().control.is_some() {
                    break;
                }
            }
            assert!(inspect(app.world(), agent).unwrap().control.is_some());
            assert!(committed(app.world(), agent).unwrap().control.is_none());
            if save_control {
                app.world_mut().run_schedule(Last);
            }
            cancel(app.world_mut(), agent, "task:1", 0).unwrap();
            // Commit cancellation before the pending control could apply.
            app.world_mut().run_schedule(Last);
            let replacement = MockCompletionModel::from_stream_turns([reply("must not run")]);
            let (mut reopened, agent) = if reopen {
                drop(app);
                self::app(&disk, &replacement)
            } else {
                (app, agent)
            };
            for _ in 0..16 {
                reopened.update();
            }
            let state = committed(reopened.world(), agent).unwrap();
            assert_eq!(
                state.task("task:1").unwrap().status,
                Status::Cancelled,
                "{action}, committed={save_control}"
            );
            assert!(state.control.is_none());
            assert_eq!(
                replacement.request_count(),
                0,
                "human stop must not start another task"
            );
            assert_eq!(
                model.request_count(),
                1,
                "human stop must not continue live work"
            );
        }
    }
}

#[test]
fn ending_one_task_does_not_strand_an_already_delivered_task() {
    use rig_harness_test_support::calls;
    for action in ["defer", "finish_silently", "answer"] {
        for reopen in [false, true] {
            let disk = MemoryStore::default();
            let model = MockCompletionModel::from_stream_turns([
                if action == "answer" {
                    reply("first done")
                } else {
                    calls(
                        "control",
                        "task_control",
                        json!({"action":action,"task":"task:1","generation":0}),
                    )
                },
                reply("second done"),
            ]);
            let (mut first, agent) = app(&disk, &model);
            submit(first.world_mut(), agent, "test", "1", "first request").unwrap();
            submit(first.world_mut(), agent, "test", "2", "SECOND_REQUEST").unwrap();
            until(&mut first, |w| {
                w.get::<ActiveTurn>(agent).is_none()
                    && committed(w, agent).is_some_and(|s| {
                        s.task("task:2").is_some_and(|t| t.delivered)
                            && s.task("task:1").is_some_and(|t| {
                                matches!(t.status, Status::Completed | Status::Suspended)
                            })
                    })
            });
            let (mut current, agent) = if reopen {
                drop(first);
                app(
                    &disk,
                    &MockCompletionModel::from_stream_turns([reply("second done")]),
                )
            } else {
                (first, agent)
            };
            until(&mut current, |w| {
                committed(w, agent).is_some_and(|s| {
                    s.task("task:2")
                        .is_some_and(|t| t.status == Status::Completed)
                })
            });
            let conversation = serde_json::to_string(
                current
                    .world()
                    .get::<Conversation>(agent)
                    .unwrap()
                    .messages(),
            )
            .unwrap();
            assert_eq!(
                conversation.matches("SECOND_REQUEST").count(),
                1,
                "must not append an already-delivered input again"
            );
        }
    }
}

#[test]
fn cancelling_an_accepted_inbox_task_prevents_model_delivery() {
    use rig_harness_test_support::calls;
    let disk = MemoryStore::default();
    let model = MockCompletionModel::from_stream_turns([
        calls("work", "mutate", json!({})),
        reply("first done"),
    ]);
    let (mut app, agent) = tool_app(&disk, &model);
    submit(app.world_mut(), agent, "test", "1", "first").unwrap();
    until(&mut app, |w| {
        w.iter_entities()
            .any(|e| e.contains::<ToolRequest>() && e.contains::<Started>())
    });
    let tool = app
        .world_mut()
        .query_filtered::<Entity, (With<ToolRequest>, With<Started>)>()
        .single(app.world())
        .unwrap();
    let second = submit(
        app.world_mut(),
        agent,
        "test",
        "2",
        "CANCELLED_PRIVATE_INPUT",
    )
    .unwrap()
    .task;
    until(&mut app, |w| {
        w.iter_entities().any(|e| {
            e.get::<InputTask>().is_some_and(|t| t.0 == second)
                && e.contains::<Started>()
                && !e.contains::<Outcome>()
        })
    });
    cancel(app.world_mut(), agent, &second, 0).unwrap();
    app.world_mut()
        .entity_mut(tool)
        .insert(Outcome::from(ToolResult::success(
            rig_core::tool::ToolOutput::text("done"),
        )));
    until(&mut app, |w| w.get::<ActiveTurn>(agent).is_none());
    assert_eq!(model.request_count(), 2);
    assert!(
        !serde_json::to_string(&model.requests())
            .unwrap()
            .contains("CANCELLED_PRIVATE_INPUT")
    );
    assert_eq!(
        committed(app.world(), agent)
            .unwrap()
            .task(&second)
            .unwrap()
            .status,
        Status::Cancelled
    );
}

#[test]
fn invalid_required_state_cannot_decode_as_an_empty_or_different_task() {
    let mut value = serde_json::to_value(DurableAgentState::default()).unwrap();
    value["active"] = json!("missing task");
    assert!(serde_json::from_value::<DurableAgentState>(value).is_err());
}

#[test]
fn incompatible_saved_requests_controls_and_outcomes_are_rejected() {
    let disk = MemoryStore::default();
    let model = MockCompletionModel::from_stream_turns([reply("done")]);
    let (mut app, agent) = app(&disk, &model);
    submit(app.world_mut(), agent, "test", "1", "hello").unwrap();
    until(&mut app, |w| {
        inspect(w, agent).unwrap().tasks[0].status == Status::Completed
    });
    let good = serde_json::to_value(inspect(app.world(), agent).unwrap()).unwrap();
    assert!(serde_json::from_value::<DurableAgentState>(good.clone()).is_ok());
    let mut bad = good.clone();
    bad["operations"][0]["request"] = json!({"invalid":"request"});
    let request = bad["operations"][0]["request"].clone();
    let route = bad["operations"][0]["route"].clone();
    bad["operations"][0]["digest"] = json!(digest(&(&request, &route)));
    assert!(
        serde_json::from_value::<DurableAgentState>(bad).is_err(),
        "invalid replay request accepted"
    );
    let mut bad = good.clone();
    bad["operations"][0]["digest"] = json!("wrong");
    assert!(serde_json::from_value::<DurableAgentState>(bad).is_err());
    let mut bad = good.clone();
    bad["control"] = json!({"action":"switch","task":"missing","generation":0});
    assert!(serde_json::from_value::<DurableAgentState>(bad).is_err());
    let mut bad = good;
    bad["tasks"][0]["outcome"] = json!({"turn":{"answered":false}});
    assert!(serde_json::from_value::<DurableAgentState>(bad).is_err());
}

#[test]
fn cancelling_a_queued_input_prevents_delivery_and_model_dispatch() {
    let disk = MemoryStore::default();
    let model = MockCompletionModel::from_stream_turns([reply("must not call")]);
    let (mut app, agent) = app(&disk, &model);
    let task = submit(app.world_mut(), agent, "test", "1", "cancel me")
        .unwrap()
        .task;
    cancel(app.world_mut(), agent, &task, 0).unwrap();
    for _ in 0..8 {
        app.update();
    }
    assert_eq!(model.request_count(), 0);
    assert_eq!(
        committed(app.world(), agent)
            .unwrap()
            .task(&task)
            .unwrap()
            .status,
        Status::Cancelled
    );
}

#[test]
fn deferred_task_does_not_block_a_later_new_task() {
    use rig_harness_test_support::calls;
    let disk = MemoryStore::default();
    let model = MockCompletionModel::from_stream_turns([
        calls(
            "defer",
            "task_control",
            json!({"action":"defer","task":"task:1","generation":0}),
        ),
        reply("new task done"),
    ]);
    let (mut app, agent) = app(&disk, &model);
    submit(app.world_mut(), agent, "test", "1", "defer me").unwrap();
    until(&mut app, |world| {
        committed(world, agent).is_some_and(|s| {
            s.task("task:1")
                .is_some_and(|t| t.status == Status::Suspended)
        }) && world.get::<ActiveTurn>(agent).is_none()
    });
    submit(app.world_mut(), agent, "test", "2", "new work").unwrap();
    until(&mut app, |world| {
        committed(world, agent).is_some_and(|s| {
            s.task("task:2")
                .is_some_and(|t| t.status == Status::Completed)
        })
    });
    assert_eq!(model.request_count(), 2);
}

#[test]
fn changing_route_on_restart_suspends_before_redispatch() {
    let disk = MemoryStore::default();
    let model = MockCompletionModel::from_stream_turns([reply("not sent")]);
    let (mut first, agent) = app(&disk, &model);
    submit(first.world_mut(), agent, "test", "1", "hello").unwrap();
    until(&mut first, |world| {
        committed(world, agent).is_some_and(|s| {
            s.operations
                .iter()
                .any(|o| o.kind == "model" && o.phase == "prepared")
        })
    });
    drop(first);
    let replacement = MockCompletionModel::from_stream_turns([reply("must not call")]);
    let (mut second, _) = app_on(&disk);
    second.add_plugins((AgentPlugin, SessionLogPlugin, DurablePlugin));
    // The provider configuration identity differs even with the same model id.
    second.insert_resource(vivy_gateway::ProvidersState {
        config: Default::default(),
        issue: None,
        generation: 0,
    });
    let connection = connect(&replacement).unwrap();
    second.add_observer(move |added: On<Add<Agent>>, mut commands: Commands| {
        commands.entity(added.entity).insert(connection.clone());
    });
    second.finish();
    second.update();
    let agent = first_agent(second.world_mut()).unwrap();
    until(&mut second, |world| {
        world.get::<ActiveTurn>(agent).is_none()
    });
    assert_eq!(replacement.request_count(), 0);
    assert_eq!(
        inspect(second.world(), agent).unwrap().tasks[0].status,
        Status::Suspended
    );
}

#[test]
fn committed_retryable_failure_keeps_rigs_attempt_and_deadline_after_restart() {
    use rig_core::{
        ProviderResponseError,
        test_utils::{MockError, MockStreamEvent},
    };
    let disk = MemoryStore::default();
    let failure = MockStreamEvent::Error(MockError::ProviderResponse(
        ProviderResponseError::without_status("overloaded").with_transient(Some(true)),
    ));
    let model = MockCompletionModel::from_stream_turns([vec![failure]]);
    let (mut first, agent) = app(&disk, &model);
    submit(first.world_mut(), agent, "test", "1", "retry").unwrap();
    until(&mut first, |world| {
        committed(world, agent).is_some_and(|s| {
            s.operations
                .iter()
                .any(|o| o.outcome.is_some() && o.retry_at_ms > 0 && !o.published)
        })
    });
    let deadline = committed(first.world(), agent).unwrap().operations[0].retry_at_ms;
    drop(first);
    let replacement = MockCompletionModel::from_stream_turns([reply("recovered")]);
    let (mut second, agent) = app(&disk, &replacement);
    until(&mut second, |world| {
        committed(world, agent).is_some_and(|s| s.tasks[0].status == Status::Completed)
    });
    let state = committed(second.world(), agent).unwrap();
    assert_eq!(
        state
            .operations
            .iter()
            .filter(|o| o.kind == "model")
            .map(|o| o.attempt)
            .collect::<Vec<_>>(),
        [0, 1]
    );
    assert_eq!(state.operations[0].retry_at_ms, deadline);
    assert_eq!(replacement.request_count(), 1);
    assert_eq!(model.request_count(), 1);
}
