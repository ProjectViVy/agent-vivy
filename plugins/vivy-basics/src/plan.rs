//! Collaborative Plan mode, adapted from Rig's `corpus/shared/plan_mode.rs`
//! at e1cd47d5. Like DSH's plan-mode at 22ddee3, guidance changes while the
//! tool catalog and existing permissions stay unchanged. Session state,
//! model requests and terminal views use the same agent-owned component.

use rig_harness::prelude::*;
use rig_harness::rig_core::completion::Section;
use schemars::JsonSchema;
use serde::Deserialize;

const REVIEW_TITLE: &str = "Plan review";
const PLAN_GUIDANCE: &str = "You are in Plan mode. Investigate the task, clarify material choices, \
    and design a concise implementation-ready plan before carrying it out. Prefer existing code \
    and simple changes. Do not implement the plan until the user approves it through \
    exit_plan_mode or explicitly leaves Plan mode. Ordinary conversational agreement is not a \
    Plan review decision. When ready, call exit_plan_mode with the complete Markdown plan, \
    starting with a # title, as the only and final tool call in your response. Incorporate review \
    feedback and submit again when needed. If review is unavailable, stop and ask the user to \
    leave Plan mode with /plan off. Plan mode supplies collaboration guidance; it does not \
    replace existing tool permissions.";

/// Saved planning state of one agent. Pending changes and reviews are not saved.
#[derive(Component, Reflect, Clone, Debug, Default)]
#[reflect(Component, Saved, Clone, Debug, Default)]
struct PlanState {
    active: bool,
    revision: u64,
}

#[derive(Component, Clone, Copy)]
struct PendingMode(bool);

#[derive(Component, Clone)]
struct PlanReview {
    agent: Entity,
    call: Entity,
    revision: u64,
}

#[derive(Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
struct ExitPlanArgs {
    /// The complete Markdown plan, starting with a # title.
    plan: String,
}

/// `/plan`, persistent guidance, explicit plan submission and terminal review.
#[derive(Default)]
pub struct PlanPlugin;

impl Plugin for PlanPlugin {
    fn build(&self, app: &mut App) {
        app.register_type::<PlanState>()
            .register_required_components::<Agent, PlanState>()
            .add_command(
                "plan",
                "Plan before implementation: /plan [message], /plan off",
                plan,
            )
            .add_open_tool(
                "exit_plan_mode",
                "Submit the complete Markdown plan for user review. Call only in Plan mode, \
                 as the only and final tool in your response. Approval leaves Plan mode for \
                 the next model request; feedback keeps planning. Never infer approval from prose.",
                ToolOptions::default(),
                submit,
            )
            // After request edits, before compaction measures the final prompt;
            // a child's role goes on first, so this guidance follows it.
            .add_systems(
                AgentPass,
                prepare
                    .in_set(Stage::Prepare)
                    .in_set(PrepareSystems::Edit)
                    .after(crate::context::replace_prompt)
                    .after(crate::subagents::SubagentSystems::Prepare),
            )
            .add_systems(
                AgentPass,
                finish_reviews
                    .in_set(Stage::Review)
                    .after(ReviewSystems::Act),
            )
            .add_observer(turn_ended);
        #[cfg(feature = "tui")]
        {
            app.add_systems(
                PostUpdate,
                terminal::show.in_set(rig_tui::StatusSystems::Show),
            )
            .add_systems(
                rig_tui::TranscriptRender,
                terminal::draw_submission.in_set(rig_tui::RenderSystems::Render),
            );
        }
    }
}

fn commit_mode(world: &mut World, agent: Entity, active: bool) {
    if let Some(mut state) = world.get_mut::<PlanState>(agent)
        && state.active != active
    {
        state.active = active;
        state.revision += 1;
    }
    if let Ok(mut entity) = world.get_entity_mut(agent) {
        entity.remove::<PendingMode>();
    }
}

fn apply_pending(world: &mut World, agent: Entity) {
    if let Some(PendingMode(active)) = world.get::<PendingMode>(agent).copied() {
        commit_mode(world, agent, active);
    }
}

fn reviews_of(world: &mut World, agent: Entity) -> Vec<Entity> {
    world
        .query::<(Entity, &PlanReview)>()
        .iter(world)
        .filter_map(|(ask, review)| (review.agent == agent).then_some(ask))
        .collect()
}

fn withdraw_review(world: &mut World, agent: Entity) -> bool {
    let reviews = reviews_of(world, agent);
    if reviews.is_empty() {
        return false;
    }
    world.trigger(StopTurn::new(agent));
    for ask in reviews {
        if let Ok(entity) = world.get_entity_mut(ask) {
            entity.despawn();
        }
    }
    true
}

fn plan(In(args): In<CommandArgs>, world: &mut World) {
    let text = args.args.trim();
    let active = text != "off";
    let message = active.then_some(text).filter(|text| !text.is_empty());
    if world.get::<PlanState>(args.agent).is_none() {
        return;
    }
    if !active || message.is_some() {
        withdraw_review(world, args.agent);
    }
    let running = world.get::<ActiveTurn>(args.agent).is_some();
    let current = world.get::<PlanState>(args.agent).unwrap().active;
    if running && current != active {
        world.entity_mut(args.agent).insert(PendingMode(active));
    } else {
        commit_mode(world, args.agent, active);
    }
    let mode = if active { "on" } else { "off" };
    let pending = if world.get::<PendingMode>(args.agent).is_some() {
        " (applies from the next model request or when this turn ends)"
    } else {
        ""
    };
    world.write_message(Notice::info(
        args.agent,
        format!("Plan mode {mode}{pending}."),
    ));
    if let Some(message) = message {
        world.trigger(Deliver::new(args.agent, message, args.mode).literal());
    }
}

fn prepare(world: &mut World) {
    let calls: Vec<_> = world.query_filtered::<(Entity, &ForAgent),
        (With<ModelRequest>, With<TurnCall>, Added<Admitted>)>()
        .iter(world).map(|(call, agent)| (call, agent.0)).collect();
    for (call, agent) in calls {
        apply_pending(world, agent);
        if world
            .get::<PlanState>(agent)
            .is_some_and(|state| state.active)
            && let Some(mut request) = world.get_mut::<ModelRequest>(call)
            && let Some(prompt) = &mut request.prompt
        {
            prompt
                .sections
                .push(Section::new("vivy_plan", PLAN_GUIDANCE));
        }
    }
}

fn turn_ended(
    ended: On<Add<Settled>>,
    turns: Query<&ForAgent, With<TurnOutcome>>,
    mut commands: Commands,
) {
    if let Ok(agent) = turns.get(ended.entity) {
        let agent = agent.0;
        commands.queue(move |world: &mut World| apply_pending(world, agent));
    }
}

fn decide_call(world: &mut World, call: Entity, result: Outcome) {
    if let Some(mut decision) = world.get_mut::<Decision>(call) {
        decision.decide(Decider::plugin("vivy-plan"), result);
    }
}

fn stop_review(world: &mut World, agent: Entity, reason: &str) {
    world.write_message(Notice::info(agent, reason));
    world.trigger(StopTurn {
        entity: agent,
        reason: Some(reason.into()),
    });
}

fn submit(call: On<ToolCalled<ExitPlanArgs>>, answerers: Answerers, mut commands: Commands) {
    let (agent, entity, effect, markdown) =
        (call.agent, call.call, call.effect, call.args.plan.clone());
    let interactive = answerers.can_answer(Some(agent));
    commands.queue(move |world: &mut World| {
        let Some(state) = world.get::<PlanState>(agent).cloned() else {
            return;
        };
        let problem = if !state.active || world.get::<PendingMode>(agent).is_some() {
            Some("exit_plan_mode requires active Plan mode with no pending mode change.")
        } else if !valid_plan(&markdown) {
            Some("Submit a non-empty Markdown plan starting with a # title.")
        } else if !reviews_of(world, agent).is_empty() {
            Some("A plan is already awaiting review.")
        } else {
            None
        };
        if let Some(problem) = problem {
            decide_call(world, entity, Outcome::refused(problem));
            return;
        }
        if !interactive {
            stop_review(
                world,
                agent,
                "Plan review unavailable. Plan mode remains on; use /plan off to leave.",
            );
            return;
        }
        let ask = Ask::new(REVIEW_TITLE)
            .body(markdown)
            .free_text()
            .choice("approve", "Approve and execute")
            .choice("revise", "Keep planning");
        world.spawn((
            Effect::with_parent_id(Some(effect)),
            ask,
            ForAgent(agent),
            HoldOf(entity),
            PlanReview {
                agent,
                call: entity,
                revision: state.revision,
            },
        ));
    });
}

fn valid_plan(markdown: &str) -> bool {
    markdown
        .trim()
        .strip_prefix('#')
        .is_some_and(|rest| rest.starts_with(char::is_whitespace) && !rest.trim().is_empty())
}

fn finish_reviews(world: &mut World) {
    let answered: Vec<_> = world
        .query::<(Entity, &PlanReview, &Outcome)>()
        .iter(world)
        .map(|(entity, review, outcome)| (entity, review.clone(), outcome.clone()))
        .collect();
    for (ask, review, outcome) in answered {
        let live = world.get::<Outcome>(review.call).is_none()
            && world.get::<CallOf>(review.call).is_some_and(|call_of| {
                world
                    .get::<ActiveTurn>(review.agent)
                    .is_some_and(|turn| turn.turn() == call_of.0)
            });
        let current = world
            .get::<PlanState>(review.agent)
            .is_some_and(|state| state.active && state.revision == review.revision)
            && world.get::<PendingMode>(review.agent).is_none()
            && live;
        if current
            && Answer::of(&outcome)
                .and_then(|answer| answer.text)
                .is_some_and(|text| text.trim() == "/plan off")
        {
            // Rig's modal question receives typed commands as free-text
            // answers. Route this explicit user exit through the same command.
            world.trigger(RunCommand::new(review.agent, "plan off"));
            continue;
        }
        world.entity_mut(ask).remove::<PlanReview>();
        if current {
            let answer = Answer::of(&outcome).unwrap_or_default();
            if answer.choices == ["approve"]
                && answer
                    .text
                    .as_deref()
                    .is_none_or(|text| text.trim().is_empty())
            {
                world.entity_mut(review.agent).insert(PendingMode(false));
                decide_call(world, review.call, Outcome::from(ToolResult::success(
                    "Plan approved. Leave Plan mode on the next model request and carry out the reviewed plan.".into())));
            } else if answer.is("revise")
                || answer
                    .text
                    .as_deref()
                    .is_some_and(|text| !text.trim().is_empty())
            {
                let feedback = answer.text.as_deref().unwrap_or("");
                decide_call(world, review.call, Outcome::from(ToolResult::success(
                    format!("Keep planning. Revise and submit the plan again. User feedback: {feedback}").into())));
            } else {
                stop_review(
                    world,
                    review.agent,
                    "Plan review cancelled. Plan mode remains on.",
                );
            }
        } else if live {
            stop_review(
                world,
                review.agent,
                "Plan review expired. Submit the current plan again.",
            );
        }
        if let Ok(mut entity) = world.get_entity_mut(ask) {
            entity.remove::<HoldOf>();
        }
    }
}

#[cfg(feature = "tui")]
mod terminal {
    use super::*;
    use rig_tui::{RenderBatch, Side, StatusItem, StatusItems, Tone};
    const ITEM: StatusItem = StatusItem::new("vivy-plan", Side::Left, 45, 26);

    /// Draws a submission as its title and plan: the raw JSON behind the
    /// review stays out of the transcript, and once settled the readable plan
    /// is retained in scrollback.
    pub(super) fn draw_submission(mut batch: ResMut<RenderBatch>) {
        for (call, output) in batch.tools_mut("exit_plan_mode") {
            let view = call.view();
            let plan = view.argument("plan").unwrap_or_default();
            let mut lines = vec![
                view.header("Plan submission", plan.lines().next().unwrap_or_default())
                    .into(),
            ];
            if call.result.is_some() {
                lines.extend(
                    plan.lines()
                        .skip(1)
                        .map(|line| rig_tui::ratatui::text::Line::from(line.to_owned()).into()),
                );
            }
            lines.extend(view.result_lines());
            output.lines = lines;
        }
    }

    pub(super) fn show(
        mut agents: Query<(Entity, &PlanState, Option<&PendingMode>, &mut StatusItems)>,
        reviews: Query<&PlanReview, Without<Outcome>>,
    ) {
        for (agent, state, pending, mut items) in &mut agents {
            let text = match pending {
                Some(PendingMode(true)) => "PLAN (next request)",
                Some(PendingMode(false)) => "DEFAULT (next request)",
                None if reviews.iter().any(|review| review.agent == agent) => "PLAN · REVIEW",
                None if state.active => "PLAN",
                None => "",
            };
            items.show(ITEM.says(text, Tone::Yellow));
        }
    }
}

#[cfg(test)]
mod tests;
