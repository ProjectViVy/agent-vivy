//! Pi's names for the model and reasoning controls, on Rig's own:
//!
//! - `/thinking [level]` is Rig's `/effort` (no level opens its picker), and
//!   `--thinking <level>` sets the main agent's level once it has a model.
//! - `/next-model` moves the agent to the next reachable model, and
//!   `/next-thinking` to its model's next reasoning level. In the terminal
//!   view they are Alt+P and Shift+Tab: Pi cycles models with Ctrl+P, which
//!   Rig's view keeps for the prompt history, and with Alt+P on Windows.

use rig_harness::prelude::*;

const THINKING_FLAG: &str = "thinking";

/// `/thinking`, `--thinking`, `/next-model`, `/next-thinking` and their keys.
#[derive(Default)]
pub struct PiModelControlsPlugin;

impl Plugin for PiModelControlsPlugin {
    fn build(&self, app: &mut App) {
        app.add_flag(
            Flag::new(
                THINKING_FLAG,
                "Start with this reasoning level: off, minimal, low, medium, high, xhigh, max",
            )
            .value("LEVEL"),
        )
        .add_command(
            "thinking",
            "Pi's name for /effort: /thinking [level]",
            thinking,
        )
        .add_command(
            "next-model",
            "Move to the next model that can be reached",
            next_model,
        )
        .add_command(
            "next-thinking",
            "Move to the model's next reasoning level",
            next_thinking,
        )
        .add_systems(Update, thinking_at_start);
        #[cfg(feature = "tui")]
        {
            use rig_tui::{AppKeyBindingsExt, KeyBinding, KeyPress, run_command};
            app.add_key_binding(
                KeyBinding::new(KeyPress::alt('p'), "Next model"),
                run_command("next-model"),
            );
            let back_tab = KeyPress::key(rig_tui::crossterm::event::KeyCode::BackTab);
            app.add_key_binding(
                KeyBinding::new(back_tab, "Next reasoning level"),
                run_command("next-thinking"),
            );
        }
    }
}

fn thinking(In(args): In<CommandArgs>, mut commands: Commands) {
    let line = format!("effort {}", args.args.trim());
    commands.trigger(RunCommand::new(args.agent, line.trim_end()));
}

/// `--thinking <level>`: once, when the main agent first has a model.
fn thinking_at_start(
    invoked: Option<Res<Invoked>>,
    agents: MainAgentQuery,
    connected: Query<(), With<Connection>>,
    mut done: Local<bool>,
    mut commands: Commands,
) {
    if *done {
        return;
    }
    let Some(level) = invoked
        .as_ref()
        .and_then(|invoked| invoked.flag_value(THINKING_FLAG))
    else {
        *done = true;
        return;
    };
    let Some(agent) = main_agent(&agents) else {
        return;
    };
    if connected.contains(agent) {
        *done = true;
        commands.trigger(RunCommand::new(agent, format!("effort {level}")));
    }
}

/// The item after `current` in `items`, wrapping; the first when `current`
/// is not among them.
pub fn next_after<'a, T: PartialEq>(items: &'a [T], current: Option<&T>) -> Option<&'a T> {
    let at = current.and_then(|current| items.iter().position(|item| item == current));
    match at {
        Some(at) => items.get((at + 1) % items.len()),
        None => items.first(),
    }
}

fn next_model(
    In(args): In<CommandArgs>,
    agents: Query<Option<&ModelChoice>>,
    models: Res<Models>,
    mut commands: Commands,
    mut notices: MessageWriter<Notice>,
) {
    let current = agents
        .get(args.agent)
        .ok()
        .flatten()
        .map(|choice| choice.0.clone());
    let reachable: Vec<String> = models
        .0
        .reachable()
        .iter()
        .map(|spec| spec.reference())
        .collect();
    match next_after(&reachable, current.as_ref()) {
        Some(model) if Some(model) != current.as_ref() => {
            commands.trigger(SetModel {
                entity: args.agent,
                model: model.clone(),
            });
            notices.write(Notice::info(args.agent, format!("Model: {model}.")));
        }
        _ => {
            notices.write(Notice::info(args.agent, "No other model can be reached."));
        }
    }
}

fn next_thinking(
    In(args): In<CommandArgs>,
    agents: Query<(&Connection, Option<&ReasoningSetting>)>,
    mut commands: Commands,
    mut notices: MessageWriter<Notice>,
) {
    let Ok((connection, setting)) = agents.get(args.agent) else {
        notices.write(Notice::info(args.agent, "Pick a model first."));
        return;
    };
    let choices = connection.spec.reasoning.choices();
    let current = setting.and_then(|setting| setting.0);
    let at = choices
        .iter()
        .position(|choice| choice.reasoning == current);
    let next = at.map_or(0, |at| (at + 1) % choices.len());
    match choices.get(next) {
        Some(choice) if choices.len() > 1 => {
            commands.trigger(SetReasoning {
                entity: args.agent,
                effort: ReasoningSetting(choice.reasoning),
            });
            notices.write(Notice::info(
                args.agent,
                format!("Reasoning: {}.", choice.name),
            ));
        }
        _ => {
            notices.write(Notice::info(
                args.agent,
                "This model takes no reasoning levels.",
            ));
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn cycles_and_wraps() {
        let items = ["a", "b", "c"];
        assert_eq!(next_after(&items, Some(&"a")), Some(&"b"));
        assert_eq!(next_after(&items, Some(&"c")), Some(&"a"));
        assert_eq!(next_after(&items, Some(&"z")), Some(&"a"));
        assert_eq!(next_after(&items, None), Some(&"a"));
        assert_eq!(next_after::<&str>(&[], None), None);
    }
}
