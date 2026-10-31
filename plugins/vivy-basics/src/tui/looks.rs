//! Our own looks for the terminal transcript: the welcome banner and the
//! local shell/search/list calls, which have no upstream look of their own,
//! keep the upstream renderers' output and diffs, and mark each call's state.
use rig_harness::prelude::*;
use rig_tui::highlight::{Code, Language};
use rig_tui::ratatui::{
    style::{Color, Style, Stylize},
    text::{Line, Span},
};
use rig_tui::{
    AsideView, CallState, LookLine, RenderBatch, RenderSystems, ToolCallView, TranscriptRender,
};

/// The calls our own look draws: the local wrappers, which the upstream
/// renderers leave to the default.
const OURS: [&str; 3] = ["shell", "search", "list"];

/// The calls whose first row carries the state marker, after their look drew
/// it.
const MARKED: [&str; 6] = ["read", "edit", "write", "shell", "search", "list"];

pub(super) fn install(app: &mut App) {
    // After the upstream renderers drew their lines, and after decorations.
    app.add_systems(TranscriptRender, look.in_set(RenderSystems::Edit));
}

fn look(mut batch: ResMut<RenderBatch>) {
    // The pane's width, before the requests borrow the batch.
    let width = batch.width;
    for (aside, output) in batch.asides_mut(rig_basics::welcome::WELCOME) {
        output.lines = welcome(&aside.view(), width);
    }
    for (call, output) in batch.calls_mut() {
        output.lines = call_look(&call.view(), std::mem::take(&mut output.lines));
    }
}

/// `drawn`, as the upstream look drew it, as one call shows: our own look for
/// the local wrappers, and the state marker on the first row.
pub(super) fn call_look(view: &ToolCallView<'_>, drawn: Vec<LookLine>) -> Vec<LookLine> {
    let name = view.name();
    let mut lines = if OURS.contains(&name) {
        basic_tool(view)
    } else {
        drawn
    };
    if !MARKED.contains(&name) {
        return lines;
    }
    if let Some(header) = lines.first_mut() {
        let (mark, style) = if view.failed() {
            ("× ", Style::new().red().bold())
        } else {
            match view.state {
                CallState::Done => ("• ", Style::new().dim()),
                _ => ("• ", Style::new().yellow()),
            }
        };
        // Keep every original span (including exit/timeout causes).
        let mut marked = header.colored();
        marked.spans.insert(0, Span::styled(mark, style));
        *header = marked.into();
    }
    lines
}

fn basic_tool(view: &ToolCallView<'_>) -> Vec<LookLine> {
    let mut lines = match view.name() {
        "shell" => {
            let command = view.argument("command").unwrap_or_default();
            let code = Code::new(command, Language::Named("bash"));
            let mut lines = Vec::new();
            for index in 0..code.len() {
                lines.push(if index == 0 {
                    view.header_code("$", &code, index)
                } else {
                    code.line(index, vec![Span::from("    ")])
                });
            }
            if lines.is_empty() {
                lines.push(view.header("$", "").into());
            }
            lines
        }
        "search" => vec![
            view.header("search", view.argument("pattern").unwrap_or_default())
                .into(),
        ],
        _ => vec![
            view.header("list", view.argument("path").unwrap_or("."))
                .into(),
        ],
    };
    lines.extend(view.result_lines());
    if view.result.is_none()
        && let Some(output) = view.output
    {
        lines.extend(
            output
                .lines()
                .map(|line| Line::from(format!("   {line}")).into()),
        );
    }
    lines
}

pub(super) fn welcome(view: &AsideView<'_>, columns: u16) -> Vec<LookLine> {
    let says: rig_basics::welcome::Welcome =
        serde_json::from_value(view.aside.value.clone()).unwrap_or_default();
    let width = usize::from(columns).saturating_sub(3).max(1);
    let model = match says.model {
        Some(model) => format!("model: {} · reasoning {}", model, says.reasoning),
        None => "no model: /model picks one".to_owned(),
    };
    vec![
        Line::default(),
        Line::from(vec![
            Span::from(" >_ ").cyan(),
            Span::from("Vivy").bold(),
            Span::from(format!(" (v{})", env!("CARGO_PKG_VERSION"))).dim(),
        ]),
        Line::from(format!("   {}", fit(&model, width, false))).dim(),
        Line::from(format!("   {}", fit(&says.directory, width, true))).dim(),
        Line::default(),
    ]
    .into_iter()
    .map(|line| line.bg(Color::Reset).into())
    .collect()
}

/// Truncate display columns rather than bytes, leaving a visible ellipsis.
fn fit(text: &str, width: usize, keep_end: bool) -> String {
    if Line::from(text).width() <= width {
        return text.to_owned();
    }
    let mut shown = String::new();
    let mut characters: Vec<_> = text.chars().collect();
    if keep_end {
        characters.reverse();
    }
    for character in characters {
        let next = if keep_end {
            format!("{character}{shown}")
        } else {
            format!("{shown}{character}")
        };
        if Line::from(next.as_str()).width() >= width {
            break;
        }
        shown = next;
    }
    if keep_end {
        shown.insert(0, '…');
    } else {
        shown.push('…');
    }
    shown
}
