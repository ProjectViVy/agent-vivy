//! The recipe-selected terminal presentation, on Rig's public widgets.
//! The native editor, scrolling, questions and session lifecycle remain owners
//! of their behavior. This plugin composes their screen and changes its looks.
use bevy_ecs::message::MessageMutator;

use rig_harness::prelude::*;
use rig_tui::ratatui::{
    layout::{Constraint, Rect},
    style::{Color, Style, Stylize},
    text::Line,
    widgets::{Block, Clear, Paragraph, Widget},
};
use rig_tui::{
    Editor, InlineStrip, InputAction, InputContext, InputSystems, KeyFocus, KeyInput, MainEditor,
    MainPane, PaneKeys, PanelCanvas, PendingInput, Placement, RequestRedraw, StatusItems,
    StatusSystems, TerminalSize, TerminalWrite, Theme, TranscriptPane, TranscriptSource, TuiFrame,
    TuiKeys, TuiPanel, TuiSystems, TuiTestScreen, TuiViewPlugin,
    crossterm::event::{Event, MouseEventKind},
};

mod footer;
mod looks;

/// The wheel reported to us, in SGR form, and nothing else about the mouse:
/// no motion, no clicks. The view owns no mouse, so the sequence is a
/// [`TerminalWrite`], and leaving the terminal undoes it.
const WHEEL_ON: &[u8] = b"\x1b[?1000h\x1b[?1006h";
const WHEEL_OFF: &[u8] = b"\x1b[?1000l\x1b[?1006l";

/// Rows one wheel notch moves the transcript: what a terminal's wheel
/// usually does, and a third of Rig's page keys.
const NOTCH: usize = 3;

/// A compact coding screen with Vivy branding and a persistent footer.
#[derive(Default)]
pub struct VivyTuiPlugin;

impl Plugin for VivyTuiPlugin {
    fn build(&self, app: &mut App) {
        app.add_plugins((TuiViewPlugin, rig_basics::welcome::WelcomePlugin));
        let world = app.world_mut();
        world.spawn((
            Name::new("Vivy:footer"),
            footer::Rotation::default(),
            TuiPanel::new(Placement::BelowInput(Constraint::Length(1))),
        ));
        world.spawn((
            Name::new("Vivy:input"),
            Editor::default(),
            MainEditor,
            TuiPanel::new(Placement::Bottom(Constraint::Length(3))),
        ));
        world.spawn((Name::new("Vivy:questions"), InlineStrip));
        world.spawn((
            Name::new("Vivy:transcript"),
            TranscriptPane::new(TranscriptSource::Shown),
            MainPane,
            PaneKeys,
        ));
        footer::install(app);
        app.add_systems(Startup, ask_for_wheel)
            // Written after the frame, before the view lets the terminal go
            // and before the exit-time title restore that ends the output.
            .add_systems(
                Last,
                release_wheel
                    .before(OnAppExitSystems)
                    .run_if(on_message::<AppExit>),
            )
            // Rig reads the mouse and leaves it to nothing; the wheel is
            // the transcript's, in every scope, before any binding.
            .add_systems(
                TuiKeys,
                (wheel_scrolls, resize_redraws).in_set(InputSystems::Filter),
            )
            // Before the frame decides whether to draw: a resize that Rig
            // did not learn of would leave the screen as it was.
            .add_systems(Update, keep_the_screen_size)
            .add_systems(PostUpdate, prioritize_status.in_set(StatusSystems::Edit))
            .add_systems(PostUpdate, prepare.in_set(TuiSystems::Prepare))
            // The native widget has already drawn its text and caret. Change
            // only its chrome before composition; overlays still draw last.
            .add_systems(
                PostUpdate,
                input_chrome
                    .after(TuiSystems::Draw)
                    .before(TuiSystems::Render),
            )
            // Rig sends its state in PostUpdate and writes in Last, after
            // OnAppExitSystems. Its exit-time title pop passes through intact.
            .add_systems(Last, brand_title.before(OnAppExitSystems));
    }

    fn finish(&self, app: &mut App) {
        looks::install(app);
    }
}

/// Keep the same terminal-derived palette, with quieter neutral surfaces.
/// Compare before writing so an idle app does not redraw indefinitely.
fn prepare(mut theme: ResMut<Theme>, mut redraw: MessageWriter<RequestRedraw>) {
    let mut next = *theme;
    next.user = theme.shade(0.06);
    next.input = theme.shade(0.08);
    next.tool = Color::Reset;
    next.call_done = Color::Reset;
    next.call_waiting = Color::Reset;
    next.call_running = theme.shade(0.04);
    if *theme != next {
        *theme = next;
        redraw.write(RequestRedraw);
    }
}

/// Preserve the data supplied by plugins, changing only fitting priorities.
fn prioritize_status(mut statuses: Query<&mut StatusItems>) {
    for mut items in &mut statuses {
        let mut next = items.clone();
        for mut item in items.iter().cloned() {
            match item.name {
                "status" => {
                    item.order = 10;
                    item.keep = u8::MAX;
                    item.min_width = 5;
                }
                "vivy-plan" | "plan" => {
                    item.order = 20;
                    item.keep = u8::MAX;
                    item.min_width = 4;
                }
                "model" => {
                    item.keep = 200;
                    item.min_width = 8;
                }
                "context" => item.keep = 180,
                _ => {}
            }
            next.show(item);
        }
        if *items != next {
            *items = next;
        }
    }
}

/// Asks the terminal for the wheel once the view holds it, so a notch
/// scrolls the transcript instead of leaving the mouse to the terminal.
fn ask_for_wheel(mut writes: MessageWriter<TerminalWrite>) {
    writes.write(TerminalWrite(WHEEL_ON.to_vec()));
}

/// Gives the terminal's mouse back on the way out, so the shell after this
/// one is not asked for reports it never wanted.
fn release_wheel(mut writes: MessageWriter<TerminalWrite>) {
    writes.write(TerminalWrite(WHEEL_OFF.to_vec()));
}

/// Turns each wheel notch into the scroll rows it means, before the keys
/// reach the view; other mouse events, which nothing binds, stay dropped.
fn wheel_scrolls(mut events: ResMut<PendingInput>) {
    if !events.0.iter().any(asks_to_scroll) {
        return;
    }
    let mut next = Vec::with_capacity(events.0.len());
    for item in std::mem::take(&mut events.0) {
        match item {
            KeyInput::Event(event) => match scroll(&event) {
                Some(action) => next.extend(std::iter::repeat_n(KeyInput::Action(action), NOTCH)),
                None => next.push(KeyInput::Event(event)),
            },
            item => next.push(item),
        }
    }
    events.0 = next;
}

/// The scroll a wheel event asks for, else nothing.
fn scroll(event: &Event) -> Option<InputAction> {
    match event {
        Event::Mouse(mouse) => match mouse.kind {
            MouseEventKind::ScrollUp => Some(InputAction::ScrollUp),
            MouseEventKind::ScrollDown => Some(InputAction::ScrollDown),
            _ => None,
        },
        _ => None,
    }
}

fn asks_to_scroll(item: &KeyInput) -> bool {
    matches!(item, KeyInput::Event(event) if scroll(event).is_some())
}

/// Draws again after a resize even when it ends at the size the view already
/// has: the terminal clipped or cleared the screen meanwhile, so a frame that
/// is not drawn leaves it looking wrong.
fn resize_redraws(events: Res<PendingInput>, mut redraw: MessageWriter<RequestRedraw>) {
    if events.0.iter().any(is_resize) {
        redraw.write(RequestRedraw);
    }
}

fn is_resize(item: &KeyInput) -> bool {
    matches!(item, KeyInput::Event(Event::Resize(..)))
}

/// Draws the screen again when the frame it drew is not the size the
/// terminal has. Rig takes its screen size from the resize event, and on
/// Windows ConPTY that event reports the console buffer's size, which stays
/// as the app found it; it can also arrive after the window settled, or not
/// at all. Either way the frame drawn may be laid out at another size, and
/// without a change the view would never draw the frame that fixes it.
fn keep_the_screen_size(
    size: Option<ResMut<TerminalSize>>,
    drawn: Option<Res<TuiFrame>>,
    test: Option<Res<TuiTestScreen>>,
    mut redraw: MessageWriter<RequestRedraw>,
) {
    // A test's screen is the size the test set; the terminal answers nothing.
    if test.is_some() {
        return;
    }
    let (Some(mut size), Some(drawn)) = (size, drawn) else {
        return;
    };
    let Some(next) = moved_screen(laid_out(&drawn), reported_size()) else {
        return;
    };
    if size.0 != next {
        size.0 = next;
    }
    redraw.write(RequestRedraw);
}

/// The screen the terminal reports now, when it reports one.
fn reported_size() -> Option<(u16, u16)> {
    rig_tui::crossterm::terminal::size().ok()
}

/// The size the last frame laid its panels out at: the bottom and right
/// edges of the areas it drew. A frame laid out before a resize and drawn
/// after it reports the panel layout, not the frame, which is what a re-laid
/// frame has to replace.
fn laid_out(frame: &TuiFrame) -> Option<(u16, u16)> {
    let mut size = (0, 0);
    for (_, area) in &frame.areas {
        size = (size.0.max(area.right()), size.1.max(area.bottom()));
    }
    match size {
        (0, _) | (_, 0) => None,
        (width, height) => Some((width, height)),
    }
}

/// The space to draw in again, when the screen last drawn, at `drawn`, is not
/// the size the terminal reports: the next frame lays out and draws at it.
fn moved_screen(drawn: Option<(u16, u16)>, reported: Option<(u16, u16)>) -> Option<Rect> {
    let (width, height) = reported?;
    let (drawn_width, drawn_height) = drawn?;
    ((drawn_width, drawn_height) != (width, height)).then(|| Rect::new(0, 0, width, height))
}

fn input_chrome(
    mut editors: Query<(Entity, &Editor, &mut PanelCanvas), With<MainEditor>>,
    state: Res<InputContext>,
    focus: Res<KeyFocus>,
    shown: Res<ShownAgent>,
    active: Query<(), With<ActiveTurn>>,
    theme: Res<Theme>,
) {
    // Leave the question title, refusal hints and other panels' focus intact.
    if state.answering().is_some() || state.picking().is_some() || focus.0.is_some() {
        return;
    }
    let busy = shown.0.is_some_and(|agent| active.contains(agent));
    for (entity, editor, mut canvas) in &mut editors {
        let area = canvas.area();
        if state.editor() != Some(entity) || area.width < 3 || area.height < 3 {
            continue;
        }
        let background = Style::new().bg(theme.input);
        let hint_area = Rect::new(area.x, area.y, area.width, 1);
        Clear.render(hint_area, canvas.buffer_mut());
        Block::new()
            .style(background)
            .render(hint_area, canvas.buffer_mut());
        if editor.is_empty() {
            let hint = input_hint(busy, usize::from(area.width - 2));
            let inside = Rect::new(area.x + 1, area.y, area.width - 2, 1);
            Paragraph::new(Line::from(hint).dim())
                .style(background)
                .render(inside, canvas.buffer_mut());
            let placeholder = if busy {
                "Steer the current task"
            } else {
                "Ask Vivy to do anything"
            };
            let inside = Rect::new(area.x + 3, area.y + 1, area.width.saturating_sub(4), 1);
            Paragraph::new(Line::from(placeholder).dim())
                .style(background)
                .render(inside, canvas.buffer_mut());
        }
    }
}

/// Fit complete hints rather than cutting a shortcut midway. During work,
/// stopping the turn stays discoverable even in a narrow terminal.
fn input_hint(busy: bool, width: usize) -> String {
    let items: &[&str] = if busy {
        &["Esc stop", "Enter steer", "Tab queue"]
    } else {
        &["Enter send", "Shift+Enter newline", "/ commands", "@ files"]
    };
    let mut shown = String::new();
    for item in items {
        let next = if shown.is_empty() {
            (*item).to_owned()
        } else {
            format!("{shown} · {item}")
        };
        if Line::from(next.as_str()).width() > width {
            break;
        }
        shown = next;
    }
    shown
}

/// Replace only the known upstream application-title sequence. Never rewrite
/// transcript text, arbitrary OSC payloads, prompt marks or clipboard contents.
fn brand_title(mut writes: MessageMutator<TerminalWrite>) {
    const PREFIX: &[u8] = b"\x1b]2;rig \xc2\xb7 ";
    for write in writes.read() {
        let start = if write.0.starts_with(b"\x1b[22;0t") {
            7
        } else {
            0
        };
        if write
            .0
            .get(start..)
            .is_some_and(|tail| tail.starts_with(PREFIX))
        {
            write
                .0
                .splice(start + 4..start + 7, b"Vivy".iter().copied());
        }
    }
}

#[cfg(test)]
mod tests;
