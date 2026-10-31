use super::*;
use crate::tui::looks;
use rig_harness_test_support::{MemoryStore, app_on, first_agent};
use rig_tui::{Editor, MainEditor, Side, StatusItem, StatusItems, Theme, Tone, TuiTestScreen};
use std::time::Instant;

fn screen_app() -> (App, Entity) {
    let (mut app, _) = app_on(&MemoryStore::default());
    app.insert_resource(Invoked::default())
        .insert_resource(ProjectRoot("C:/fixture/项目".into()))
        .add_plugins((AgentPlugin, SessionLogPlugin, VivyTuiPlugin));
    rig_tui::add_view(&mut app);
    app.finish();
    app.update();
    let agent = first_agent(app.world_mut()).unwrap();
    let mut status = StatusItems::default();
    for item in [
        StatusItem::new("model", Side::Left, 30, u8::MAX).says("org/model-1", Tone::Bold),
        StatusItem::new("status", Side::Left, 50, u8::MAX).says("idle", Tone::Green),
        StatusItem::new("plan", Side::Left, 10, u8::MAX).says("PLAN", Tone::Cyan),
    ] {
        status.show(item);
    }
    app.world_mut()
        .entity_mut(agent)
        .insert((rig_basics::models::ModelLabel("org/model-1".into()), status));
    app.insert_resource(ShownAgent(Some(agent)));
    app.insert_resource(TuiTestScreen::new(100, 24));
    app.insert_resource(TerminalSize(Rect::new(0, 0, 100, 24)));
    app.update();
    (app, agent)
}

fn text(app: &App) -> String {
    app.world().resource::<TuiTestScreen>().rows().join("\n")
}

#[test]
fn workbench_footer_rotates_without_mutating_telemetry_or_input() {
    use crate::workbench_status::{MemoryState, PythonState, WorkbenchStatus};
    let (mut app, agent) = screen_app();
    app.world_mut().entity_mut(agent).insert(WorkbenchStatus {
        python: PythonState::Prepared,
        memory: MemoryState::Unchecked,
    });
    app.world_mut()
        .get_mut::<StatusItems>(agent)
        .unwrap()
        .show(StatusItem::new("context", Side::Right, 40, 5).says("ctx 12%", Tone::Dim));
    app.update();
    assert!(text(&app).lines().last().unwrap().contains("ctx 12%"));
    let footer = app
        .world_mut()
        .query_filtered::<Entity, With<footer::Rotation>>()
        .single(app.world())
        .unwrap();
    assert_eq!(footer::INTERVAL, Duration::from_secs(5));
    assert!(app.world().get::<WakeAt>(footer).is_some());
    app.world_mut()
        .get_mut::<footer::Rotation>(footer)
        .unwrap()
        .next = Instant::now();
    app.update();
    let screen = text(&app);
    let line = screen.lines().last().unwrap();
    assert!(
        line.contains("Py prepared") && line.contains("Mem not init"),
        "{line}"
    );
    assert!(line.contains("PLAN") && line.contains("idle") && !line.contains("ctx 12%"));
    assert!(
        app.world()
            .get::<StatusItems>(agent)
            .unwrap()
            .iter()
            .any(|i| i.text == "ctx 12%")
    );
    // New telemetry while hidden must appear on the next page, not a stale cache.
    app.world_mut()
        .get_mut::<StatusItems>(agent)
        .unwrap()
        .show(StatusItem::new("context", Side::Right, 40, 5).says("ctx 23%", Tone::Dim));
    app.world_mut()
        .get_mut::<footer::Rotation>(footer)
        .unwrap()
        .next = Instant::now();
    app.update();
    assert!(text(&app).lines().last().unwrap().contains("ctx 23%"));
    assert!(!text(&app).lines().last().unwrap().contains("Py "));
    app.world_mut()
        .entity_mut(agent)
        .remove::<WorkbenchStatus>();
    app.update();
    assert!(app.world().get::<WakeAt>(footer).is_none());
}

#[test]
fn workbench_footer_pins_errors_and_preparation_then_holds_recovery() {
    use crate::workbench_status::{MemoryState, PythonState, WorkbenchStatus};
    let (mut app, agent) = screen_app();
    app.world_mut().entity_mut(agent).insert(WorkbenchStatus {
        python: PythonState::Preparing,
        memory: MemoryState::Unchecked,
    });
    app.update();
    let footer = app
        .world_mut()
        .query_filtered::<Entity, With<footer::Rotation>>()
        .single(app.world())
        .unwrap();
    assert!(text(&app).lines().last().unwrap().contains("Py preparing"));
    app.world_mut()
        .get_mut::<footer::Rotation>(footer)
        .unwrap()
        .next = Instant::now();
    app.update();
    assert!(text(&app).lines().last().unwrap().contains("Py preparing"));
    app.world_mut()
        .get_mut::<WorkbenchStatus>(agent)
        .unwrap()
        .python = PythonState::Error;
    app.update();
    assert!(text(&app).lines().last().unwrap().contains("Py error"));
    app.world_mut()
        .get_mut::<WorkbenchStatus>(agent)
        .unwrap()
        .python = PythonState::Ready;
    app.update();
    assert!(text(&app).lines().last().unwrap().contains("Py ready"));
    assert!(
        app.world().get::<footer::Rotation>(footer).unwrap().next
            > Instant::now() + Duration::from_secs(4)
    );
    app.world_mut()
        .get_mut::<WorkbenchStatus>(agent)
        .unwrap()
        .memory = MemoryState::Error;
    app.insert_resource(TuiTestScreen::new(50, 20));
    app.insert_resource(TerminalSize(Rect::new(0, 0, 50, 20)));
    app.update();
    let line = text(&app).lines().last().unwrap().to_owned();
    assert!(
        line.contains("Py ready") && line.contains("PLAN") && line.contains("idle"),
        "{line}"
    );
    // Switching to an agent without Python must not inherit the previous alarm/error.
    app.insert_resource(ShownAgent(None));
    app.update();
    assert!(app.world().get::<WakeAt>(footer).is_none());
    assert!(!text(&app).contains("Mem error"));
}

#[test]
fn welcome_uses_our_identity_and_tracks_model_changes_without_duplicate_headers() {
    let (mut app, agent) = screen_app();
    let before = text(&app);
    assert!(before.contains(">_ Vivy (v0.1.0)"), "{before}");
    assert!(!before.contains("█▙") && !before.contains("rig v"));
    assert!(before.replace(' ', "").contains("项目"), "{before}");
    app.world_mut()
        .entity_mut(agent)
        .insert(rig_basics::models::ModelLabel("org/next-model".into()));
    app.update();
    let after = text(&app);
    assert!(after.contains("org/next-model"), "{after}");
    assert_eq!(after.matches(">_ Vivy").count(), 1);
}

#[test]
fn typing_keeps_plan_and_activity_in_the_footer_and_keeps_the_native_editor() {
    let (mut app, _) = screen_app();
    assert!(text(&app).contains("Ask Vivy to do anything"));
    assert!(!text(&app).contains("Ctrl+V image"), "{}", text(&app));
    let world = app.world_mut();
    let mut editors = world.query_filtered::<&mut Editor, With<MainEditor>>();
    editors
        .single_mut(world)
        .unwrap()
        .set("检查这段代码\nsecond line");
    app.update();
    let rows = app.world().resource::<TuiTestScreen>().rows();
    assert!(rows.last().unwrap().contains("PLAN"), "{}", rows.join("\n"));
    assert!(rows.last().unwrap().contains("idle"));
    assert!(rows.iter().any(|row| row.contains("second line")));
    assert!(!rows.iter().any(|row| row.contains("Ask Vivy")));
}

#[test]
fn terminal_titles_are_rebranded_while_prompt_marks_and_title_restore_survive() {
    let (mut app, _) = screen_app();
    let written = app.world().resource::<TuiTestScreen>().written();
    let first = String::from_utf8_lossy(written);
    assert!(first.contains("\x1b]2;Vivy · idle\x1b\\"), "{first:?}");
    assert!(!first.contains("\x1b]2;rig ·"));
    assert!(first.contains("\x1b]133;A"));
    app.world_mut().write_message(rig_harness::AppExit::Success);
    app.update();
    assert!(
        app.world()
            .resource::<TuiTestScreen>()
            .written()
            .ends_with(b"\x1b[23;0t")
    );
}

#[test]
fn plain_theme_and_narrow_screens_keep_the_editor_and_plan_visible() {
    let (mut app, _) = screen_app();
    app.insert_resource(Theme::default().without_color());
    app.world_mut()
        .resource_mut::<TuiTestScreen>()
        .resize(32, 12);
    app.insert_resource(TerminalSize(Rect::new(0, 0, 32, 12)));
    app.update();
    let output = text(&app);
    assert!(output.contains("PLAN"), "{output}");
    assert!(output.contains('›'));
    let screen = app.world().resource::<TuiTestScreen>();
    assert!(screen.buffer().content().iter().all(|cell| {
        cell.fg == rig_tui::ratatui::style::Color::Reset
            && cell.bg == rig_tui::ratatui::style::Color::Reset
    }));
}

#[test]
fn long_unicode_directory_and_model_are_refit_after_resize() {
    let (mut app, agent) = screen_app();
    app.insert_resource(ProjectRoot(format!("C:/{}", "测试/".repeat(60)).into()));
    app.world_mut()
        .resource_mut::<TuiTestScreen>()
        .resize(32, 16);
    app.insert_resource(TerminalSize(Rect::new(0, 0, 32, 16)));
    app.update();
    let small = text(&app);
    assert!(small.contains('…'), "{small}");
    assert!(!small.contains('\u{fffd}'));
    app.world_mut()
        .entity_mut(agent)
        .insert(rig_basics::models::ModelLabel("org/new-model".into()));
    app.world_mut()
        .resource_mut::<TuiTestScreen>()
        .resize(100, 24);
    app.insert_resource(TerminalSize(Rect::new(0, 0, 100, 24)));
    app.update();
    assert!(text(&app).contains("org/new-model · reasoning default"));
    assert_eq!(text(&app).matches(">_ Vivy").count(), 1);
}

#[test]
fn terminal_branding_leaves_arbitrary_text_and_other_control_sequences_untouched() {
    let (mut app, _) = screen_app();
    let start = app.world().resource::<TuiTestScreen>().written().len();
    let untouched = b"literal rig text\x1b]52;c;cmln\x1b\\\x1b]133;C\x1b\\";
    app.world_mut()
        .write_message(TerminalWrite(untouched.to_vec()));
    app.update();
    assert_eq!(
        &app.world().resource::<TuiTestScreen>().written()[start..],
        untouched
    );
}

#[test]
fn theme_changes_preserve_diff_colors_and_adapt_the_input_surface() {
    let (mut app, _) = screen_app();
    let mut inputs = Vec::new();
    for background in [(0x1c, 0x1c, 0x1c), (0xfa, 0xfa, 0xfa)] {
        let native = Theme::on(background, true);
        app.insert_resource(native);
        app.world_mut().write_message(RequestRedraw);
        app.update();
        let actual = app.world().resource::<Theme>();
        assert_eq!(actual.added, native.added);
        assert_eq!(actual.removed, native.removed);
        assert_eq!(actual.call_failed, native.call_failed);
        assert_eq!(actual.tool, Color::Reset);
        inputs.push(actual.input);
    }
    assert_ne!(inputs[0], inputs[1]);
}

#[test]
fn coding_look_marks_the_state_and_keeps_every_upstream_row_and_cause() {
    use rig_harness::rig_core::message::{ToolCall, ToolFunction, ToolName};
    use rig_harness::rig_core::tool::ToolResult;
    use rig_tui::{CallState, ToolCallView};
    let call = ToolCall::from_wire(
        "read-1",
        ToolFunction::new(
            ToolName::new("read").unwrap(),
            serde_json::json!({"path":"file.rs"}),
        ),
    );
    let output = (0..80)
        .map(|n| format!("line {n}"))
        .collect::<Vec<_>>()
        .join("\n");
    let mut result = call.answer(&ToolResult::success(output.into()));
    // What rig-basics' own look drew: our look edits it, it does not redraw it.
    let drawn = |failed: bool| {
        let mut lines = vec![Line::from("read file.rs 80 lines")];
        lines.extend((0..80).map(|n| Line::from(format!("line {n}"))));
        if failed {
            lines.push(Line::from("read failed: permission denied"));
        }
        lines
            .into_iter()
            .map(rig_tui::LookLine::from)
            .collect::<Vec<_>>()
    };
    let upstream = drawn(false);
    let view = ToolCallView::new(&call, Some(&result), CallState::Done);
    let after = looks::call_look(&view, upstream.clone());
    assert!(after[0].to_string().starts_with("• "));
    assert_eq!(after[1..], upstream[1..]);
    assert!(after.last().unwrap().to_string().contains("line 79"));

    result.is_error = true;
    let failed = looks::call_look(
        &ToolCallView::new(&call, Some(&result), CallState::Done),
        drawn(true),
    );
    assert!(failed[0].to_string().starts_with("× "));
    assert!(
        failed
            .last()
            .unwrap()
            .to_string()
            .contains("permission denied")
    );
}

#[test]
fn local_shell_search_and_list_are_readable_instead_of_raw_json() {
    use rig_harness::rig_core::message::{ToolCall, ToolFunction, ToolName};
    use rig_tui::{CallState, ToolCallView};
    let call = ToolCall::from_wire(
        "shell-1",
        ToolFunction::new(
            ToolName::new("shell").unwrap(),
            serde_json::json!({"command":"printf visible", "timeout_secs":60}),
        ),
    );
    let mut view = ToolCallView::new(&call, None, CallState::Running(None));
    view.output = Some("ACTUAL-LIVE-OUTPUT\nlast live line");
    let text = looks::call_look(&view, Vec::new())
        .into_iter()
        .map(|line| line.to_string())
        .collect::<Vec<_>>()
        .join("\n");
    assert!(
        text.contains("$ printf visible") && !text.contains("\"command\""),
        "{text}"
    );
    assert!(text.contains("ACTUAL-LIVE-OUTPUT") && text.contains("last live line"));
    for (tool, argument) in [("search", "pattern"), ("list", "path")] {
        let call = ToolCall::from_wire(
            "c",
            ToolFunction::new(
                ToolName::new(tool).unwrap(),
                serde_json::json!({argument: "src"}),
            ),
        );
        let view = ToolCallView::new(&call, None, CallState::Done);
        let text = looks::call_look(&view, Vec::new())
            .into_iter()
            .map(|line| line.to_string())
            .collect::<Vec<_>>()
            .join("\n");
        assert!(
            text.starts_with("• ") && text.contains(tool),
            "{tool}: {text}"
        );
        assert!(text.contains("src"), "{tool}: {text}");
    }
}

#[test]
fn a_wheel_notch_scrolls_the_transcript_without_typing_or_sending() {
    use rig_harness::rig_core::completion::Message;
    use rig_harness::rig_ecs::agent::Conversation;
    let (mut app, agent) = screen_app();
    let mut conversation = app
        .world_mut()
        .get_mut::<Conversation>(agent)
        .expect("the agent holds a conversation");
    for row in 0..40 {
        conversation.push(Message::user(format!("transcript row {row}")));
    }
    app.update();
    let bottom = text(&app);
    assert!(bottom.contains("transcript row 39"), "{bottom}");
    wheel(&mut app, MouseEventKind::ScrollUp);
    let scrolled = text(&app);
    assert_ne!(bottom, scrolled, "the wheel looks further back");
    assert!(!scrolled.contains("transcript row 39"), "{scrolled}");
    assert!(
        scrolled.contains("more below") && scrolled.contains("Ask Vivy to do anything"),
        "reading happens above an untouched input: {scrolled}"
    );
    let world = app.world_mut();
    let mut editors = world.query_filtered::<&Editor, With<MainEditor>>();
    assert!(
        editors.single(world).unwrap().is_empty(),
        "a wheel notch types nothing"
    );
    // Down again reaches the newest output, as following the work does.
    for _ in 0..20 {
        wheel(&mut app, MouseEventKind::ScrollDown);
    }
    let followed = text(&app);
    assert!(followed.contains("transcript row 39"), "{followed}");
}

/// Puts a wheel notch of `kind` among the frame's keys, then runs a frame.
fn wheel(app: &mut App, kind: MouseEventKind) {
    use rig_tui::crossterm::event::{KeyModifiers, MouseEvent};
    app.world_mut()
        .resource_mut::<PendingInput>()
        .0
        .push(KeyInput::Event(Event::Mouse(MouseEvent {
            kind,
            column: 10,
            row: 5,
            modifiers: KeyModifiers::NONE,
        })));
    app.update();
}

#[test]
fn wheel_events_become_scrolls_and_other_mouse_events_stay_dropped() {
    use rig_tui::crossterm::event::{KeyModifiers, MouseButton, MouseEvent};
    let mouse = |kind| {
        KeyInput::Event(Event::Mouse(MouseEvent {
            kind,
            column: 10,
            row: 5,
            modifiers: KeyModifiers::NONE,
        }))
    };
    assert!(asks_to_scroll(&mouse(MouseEventKind::ScrollUp)));
    assert!(asks_to_scroll(&mouse(MouseEventKind::ScrollDown)));
    for ignored in [
        MouseEventKind::Moved,
        MouseEventKind::Down(MouseButton::Left),
        MouseEventKind::Up(MouseButton::Left),
    ] {
        assert!(!asks_to_scroll(&mouse(ignored)), "{ignored:?}");
    }
}

#[test]
fn the_wheel_is_asked_for_with_the_screen_and_given_back_on_the_way_out() {
    let (mut app, _) = screen_app();
    let asked = app.world().resource::<TuiTestScreen>().written().to_vec();
    assert!(
        asked.windows(WHEEL_ON.len()).any(|bytes| bytes == WHEEL_ON),
        "{asked:?}"
    );
    app.world_mut().write_message(rig_harness::AppExit::Success);
    app.update();
    let written = app.world().resource::<TuiTestScreen>().written();
    assert!(
        written
            .windows(WHEEL_OFF.len())
            .any(|bytes| bytes == WHEEL_OFF),
        "{written:?}"
    );
    assert!(
        written.ends_with(b"\x1b[23;0t"),
        "the title restore still ends the output: {written:?}"
    );
}

#[test]
fn a_screen_drawn_at_another_size_is_drawn_again() {
    // Windows' ConPTY reports the console buffer's size in the resize event,
    // so the frame that learned of a resize can have been laid out at the old
    // size while the terminal is already at the new one.
    assert_eq!(
        moved_screen(Some((120, 40)), Some((104, 32))),
        Some(Rect::new(0, 0, 104, 32))
    );
    assert_eq!(
        moved_screen(Some((104, 32)), Some((120, 40))),
        Some(Rect::new(0, 0, 120, 40))
    );
    assert_eq!(
        moved_screen(Some((120, 40)), Some((120, 40))),
        None,
        "the screen already has that size"
    );
    assert_eq!(
        moved_screen(Some((120, 40)), None),
        None,
        "without a terminal the frame stands"
    );
    assert_eq!(
        moved_screen(None, Some((120, 40))),
        None,
        "nothing drawn yet is not a resize"
    );
}

#[test]
fn the_size_a_frame_was_laid_out_at_is_its_panels_extent() {
    let mut frame = TuiFrame::default();
    assert_eq!(laid_out(&frame), None, "nothing drawn yet");
    frame.screen = Rect::new(0, 0, 120, 40);
    frame.areas = vec![
        (Entity::PLACEHOLDER, Rect::new(0, 0, 120, 36)),
        (Entity::PLACEHOLDER, Rect::new(0, 36, 120, 3)),
        (Entity::PLACEHOLDER, Rect::new(0, 39, 120, 1)),
        (Entity::PLACEHOLDER, Rect::default()),
    ];
    assert_eq!(laid_out(&frame), Some((120, 40)));
    frame.areas = vec![
        (Entity::PLACEHOLDER, Rect::new(0, 0, 80, 20)),
        (Entity::PLACEHOLDER, Rect::new(0, 20, 80, 3)),
        (Entity::PLACEHOLDER, Rect::new(0, 23, 80, 1)),
    ];
    assert_eq!(
        laid_out(&frame),
        Some((80, 24)),
        "a frame laid out before a resize keeps that size"
    );
}

#[test]
fn native_paste_preserves_unicode_and_newlines_without_submitting() {
    use rig_tui::{KeyInput, PendingInput, crossterm::event::Event};
    let (mut app, _) = screen_app();
    app.world_mut()
        .resource_mut::<PendingInput>()
        .0
        .push(KeyInput::Event(Event::Paste(
            "检查代码\nsecond line".into(),
        )));
    app.update();
    let world = app.world_mut();
    let mut editors = world.query_filtered::<&Editor, With<MainEditor>>();
    assert_eq!(
        editors.single(world).unwrap().text(),
        "检查代码\nsecond line"
    );
}

#[test]
fn narrow_welcome_keeps_the_project_name_and_hints_end_on_whole_items() {
    let (mut app, _) = screen_app();
    app.insert_resource(ProjectRoot(
        "C:/a-very-long-parent-folder/another-parent/Vivy".into(),
    ));
    app.world_mut()
        .resource_mut::<TuiTestScreen>()
        .resize(32, 16);
    app.insert_resource(TerminalSize(Rect::new(0, 0, 32, 16)));
    app.update();
    let rows = app.world().resource::<TuiTestScreen>().rows();
    let directory = rows
        .iter()
        .find(|row| row.contains("C:/") || row.contains("/Vivy"))
        .unwrap();
    assert!(directory.trim_end().ends_with("Vivy"), "{directory}");
    let hint = rows
        .iter()
        .find(|row| row.contains("Enter send"))
        .unwrap()
        .trim();
    assert_eq!(hint, "Enter send");
}
