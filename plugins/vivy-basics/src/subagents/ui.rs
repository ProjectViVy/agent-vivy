use rig_harness::prelude::*;
use rig_tui::ratatui::text::Line;
use rig_tui::{LookLine, RenderBatch, ToolCallView};
use serde_json::Value;

pub(super) fn add(app: &mut App) {
    app.add_systems(
        rig_tui::TranscriptRender,
        draw_calls.in_set(rig_tui::RenderSystems::Render),
    );
}

/// Draws both workflows with the same look.
fn draw_calls(mut batch: ResMut<RenderBatch>) {
    for (call, output) in batch.calls_mut() {
        if !matches!(
            call.call.function.name.as_str(),
            "task_parallel" | "task_chain"
        ) {
            continue;
        }
        output.lines = draw(&call.view());
    }
}

fn draw(view: &ToolCallView<'_>) -> Vec<LookLine> {
    let title = if view.name() == "task_chain" {
        "Chain"
    } else {
        "Parallel"
    };
    let Some(text) = view.result_text() else {
        let mut lines = vec![view.header(format!("Vivy {title}"), "").into()];
        if let Some(output) = view.output {
            lines.extend(
                output
                    .lines()
                    .map(|line| Line::from(line.to_owned()).into()),
            );
        } else {
            lines.extend(view.progress_lines().into_iter().map(Into::into));
        }
        for child in view.children.iter().rev().take(6).rev() {
            let state = if child.failed {
                "failed"
            } else {
                match child.state {
                    rig_tui::CallState::Done => "completed",
                    rig_tui::CallState::Waiting | rig_tui::CallState::Held(_) => "queued",
                    _ => "running",
                }
            };
            lines.push(
                Line::from(format!(
                    "  {} · {} · {state}",
                    child.agent.as_deref().unwrap_or("child"),
                    child.name
                ))
                .into(),
            );
        }
        return lines;
    };
    let Ok(result) = serde_json::from_str::<Value>(&text) else {
        let mut lines = vec![view.header(format!("Vivy {title}"), "").into()];
        lines.extend(view.result_lines());
        return lines;
    };
    let mut lines = vec![
        view.header(
            format!("Vivy {title}"),
            format!("{}/{} completed", result["completed"], result["total"]),
        )
        .into(),
    ];
    for row in result["results"].as_array().into_iter().flatten() {
        let status = row["status"].as_str().unwrap_or("unknown");
        let name = row["description"].as_str().unwrap_or("Task");
        let id = row["agent"].as_str().unwrap_or("not started");
        lines.push(Line::from(format!("{name} · {status} · {id}")).into());
        lines.push(Line::from(usage(row.get("usage"))).into());
        if let Some(answer) = row["answer"].as_str() {
            lines.extend(rig_tui::markdown::render(answer, &view.theme, None));
        }
    }
    lines
}

fn usage(value: Option<&Value>) -> String {
    let Some(usage) = value else {
        return "usage unknown · cost unknown".into();
    };
    let counter = |name: &str| {
        usage["tokens"][name]
            .as_u64()
            .map(|n| n.to_string())
            .unwrap_or_else(|| "?".into())
    };
    let cost = if usage["unpriced"].as_u64() == Some(0)
        && usage["calls"].as_u64().is_some_and(|calls| calls > 0)
    {
        usage["cost"]
            .as_f64()
            .map(|cost| format!("${cost:.4}"))
            .unwrap_or_else(|| "cost unknown".into())
    } else {
        "cost unknown".into()
    };
    format!(
        "↑{} ↓{} tokens · {cost}",
        counter("input_tokens"),
        counter("output_tokens")
    )
}

#[cfg(test)]
mod tests {
    use super::*;
    use rig_harness::rig_core::message::{ToolCall, ToolFunction, ToolName};
    use rig_tui::CallState;
    use serde_json::json;

    #[test]
    fn completed_parallel_result_is_readable_and_renders_markdown() {
        let call = ToolCall::from_wire(
            "p",
            ToolFunction::new(ToolName::new("task_parallel").unwrap(), json!({"tasks":[]})),
        );
        let payload = json!({"mode":"parallel","completed":1,"total":2,"results":[{
            "description":"Scout","agent":"child-123","status":"failed","answer":"# Finding\n**Gateway unavailable**",
            "usage":{"tokens":{"input_tokens":10,"output_tokens":2},"cost":0.0,"unpriced":1,"calls":1}
        }]}).to_string();
        let result = call.answer(&ToolResult::success(payload.into()));
        let view = ToolCallView::new(&call, Some(&result), CallState::Done);
        let text = draw(&view)
            .into_iter()
            .map(|line| line.to_string())
            .collect::<Vec<_>>()
            .join("\n");
        assert!(
            text.contains("Parallel") && text.contains("1/2") && text.contains("Scout"),
            "{text}"
        );
        assert!(text.contains("failed") && text.contains("Gateway unavailable"));
        assert!(
            text.contains("cost unknown"),
            "missing price is not a zero-cost result: {text}"
        );
        assert!(
            !text.contains("\"results\"") && !text.contains("**Gateway"),
            "raw JSON/Markdown leaked: {text}"
        );
    }

    #[test]
    fn running_chain_shows_progress_before_a_result_exists() {
        let call = ToolCall::from_wire(
            "c",
            ToolFunction::new(ToolName::new("task_chain").unwrap(), json!({"steps":[]})),
        );
        let mut view = ToolCallView::new(&call, None, CallState::Running(None));
        view.output = Some("Chain: 1/3 completed");
        let text = draw(&view)
            .into_iter()
            .map(|line| line.to_string())
            .collect::<Vec<_>>()
            .join("\n");
        assert!(text.contains("Chain: 1/3 completed"), "{text}");
    }
}
