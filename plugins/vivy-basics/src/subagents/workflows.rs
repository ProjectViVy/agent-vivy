//! Finite batches and chains over native SpawnSubagent effects.

use super::roles::{PreparedTask, RoleContext};
use rig_basics::subagents::{Report, SpawnSubagent, SpawnedAgent};
use rig_basics::telemetry::usage::AgentUsage;
use rig_harness::commands::CommandSystems;
use rig_harness::prelude::*;
use rig_harness::rig_tools::Live;
use schemars::JsonSchema;
use serde::{Deserialize, Serialize};
use serde_json::json;

pub(super) const PARALLEL: &str = "task_parallel";
pub(super) const CHAIN: &str = "task_chain";
const INLINE_BYTES: usize = 50 * 1024;

#[derive(SystemSet, Debug, Clone, PartialEq, Eq, Hash)]
pub(super) struct WorkflowSystems;

#[derive(Clone, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
struct Task {
    description: String,
    prompt: String,
    profile: Option<String>,
    model: Option<String>,
    effort: Option<String>,
    tools: Option<Vec<String>>,
    cwd: Option<String>,
    environment: Option<String>,
}

impl Task {
    fn request(&self) -> SpawnSubagent {
        let mut request = SpawnSubagent::task(&self.description, &self.prompt);
        request.model.clone_from(&self.model);
        request.effort.clone_from(&self.effort);
        request.tools.clone_from(&self.tools);
        request.environment.clone_from(&self.environment);
        if let Some(profile) = &self.profile {
            request.extra.insert("profile".into(), json!(profile));
        }
        if let Some(cwd) = &self.cwd {
            request.extra.insert("cwd".into(), json!(cwd));
        }
        request
    }
}

#[derive(Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
struct ParallelArgs {
    tasks: Vec<Task>,
}
#[derive(Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
struct ChainArgs {
    steps: Vec<Task>,
}

struct Step {
    request: SpawnSubagent,
    prepared: PreparedTask,
}

#[derive(Component)]
struct Workflow {
    chain: bool,
    steps: Vec<Step>,
    results: Vec<Option<ResultRow>>,
    next: usize,
    shown: usize,
    finished: bool,
    live: Live,
}

#[derive(Component, Clone, Copy)]
struct WorkflowRequest {
    call: Entity,
    index: usize,
}
#[derive(Component, Clone, Copy)]
struct WorkflowChild {
    call: Entity,
    index: usize,
}

/// A settled turn is only a candidate: native Rig may wait for descendants
/// instead of reporting it. The matching native Report commits this result.
#[derive(Component)]
struct WorkflowAnswer(ResultRow);

#[derive(Clone, Serialize)]
struct ResultRow {
    description: String,
    agent: Option<String>,
    status: &'static str,
    answer: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    usage: Option<rig_harness::rig_core::completion::UsageTotals>,
}

pub(super) fn add(app: &mut App) {
    app.configure_sets(
        AgentPass,
        WorkflowSystems
            .in_set(Stage::Gate)
            .after(CommandSystems)
            .after(AskSystems)
            .after(rig_basics::print::PrintSystems)
            .after(rig_basics::subagents::SubagentSystems::Gate),
    );
    let independent = || ToolOptions {
        footprint: Some(Touches::Independent),
    };
    app.add_open_tool(PARALLEL, "Delegate 1-8 independent tasks to native Vivy agents, up to four running children. Returns all results, including failures. Use distinct files for parallel edits.", independent(), parallel)
        .add_open_tool(CHAIN, "Delegate a finite sequence of native Vivy agents. {previous} in a prompt receives the prior bounded final answer. Stops at the first failure.", independent(), chain)
        .add_observer(child_spawned)
        .add_observer(child_finished)
        .add_observer(spawn_failed)
        .add_observer(cancel_removed)
        .add_systems(AgentPass, (claim_reports, advance, stop_restored).chain().in_set(WorkflowSystems));
}

fn parallel(
    called: On<ToolCalled<ParallelArgs>>,
    context: RoleContext,
    wake: Res<Wake>,
    mut commands: Commands,
) {
    start(
        called.call,
        called.agent,
        called.effect,
        false,
        &called.args.tasks,
        &context,
        &wake,
        &mut commands,
    );
}
fn chain(
    called: On<ToolCalled<ChainArgs>>,
    context: RoleContext,
    wake: Res<Wake>,
    mut commands: Commands,
) {
    start(
        called.call,
        called.agent,
        called.effect,
        true,
        &called.args.steps,
        &context,
        &wake,
        &mut commands,
    );
}

fn start(
    call: Entity,
    parent: Entity,
    effect: rig_harness::rig_core::effect::EffectId,
    chain: bool,
    tasks: &[Task],
    context: &RoleContext,
    wake: &Wake,
    commands: &mut Commands,
) {
    let steps = (|| {
        if tasks.is_empty() {
            return Err("Provide at least one task".to_owned());
        }
        if !chain && tasks.len() > 8 {
            return Err("A parallel call accepts at most 8 tasks".to_owned());
        }
        tasks
            .iter()
            .map(|task| {
                let mut request = task.request();
                context
                    .prepare(parent, &mut request)
                    .map(|prepared| Step { request, prepared })
            })
            .collect::<Result<Vec<_>, _>>()
    })();
    let steps = match steps {
        Ok(steps) => steps,
        Err(why) => {
            commands.entity(call).insert(Outcome::refused(why));
            return;
        }
    };
    let (output, live) = CallOutput::new(wake);
    let count = steps.len();
    let mut workflow = Workflow {
        chain,
        steps,
        results: vec![None; count],
        next: 0,
        shown: 0,
        finished: false,
        live,
    };
    let launch = if chain { 1 } else { count };
    for index in 0..launch {
        spawn(
            call,
            parent,
            effect,
            index,
            &workflow.steps[index],
            None,
            commands,
        );
    }
    workflow.next = launch;
    workflow.live.line(&format!(
        "{}: 0/{count} completed",
        if chain { "Chain" } else { "Parallel" }
    ));
    commands.entity(call).insert((workflow, output));
}

fn spawn(
    call: Entity,
    parent: Entity,
    effect: rig_harness::rig_core::effect::EffectId,
    index: usize,
    step: &Step,
    previous: Option<&str>,
    commands: &mut Commands,
) {
    let mut request = step.request.clone();
    if let Some(previous) = previous {
        request.brief = request.brief.replace("{previous}", previous);
    }
    commands.spawn((
        Effect::with_parent_id(Some(effect)),
        request,
        step.prepared.clone(),
        ForAgent(parent),
        ChildOf(call),
        WorkflowRequest { call, index },
    ));
}

fn child_spawned(
    event: On<Add<SpawnedAgent>>,
    requests: Query<(&SpawnedAgent, &WorkflowRequest)>,
    mut commands: Commands,
) {
    if let Ok((SpawnedAgent(child), request)) = requests.get(event.entity) {
        commands.entity(*child).insert(WorkflowChild {
            call: request.call,
            index: request.index,
        });
    }
}

fn child_finished(
    settled: On<Add<Settled>>,
    turns: Query<(&TurnOutcome, &ForAgent)>,
    children: Query<(&WorkflowChild, &AgentId, Option<&AgentUsage>)>,
    workflows: Query<&Workflow>,
    mut commands: Commands,
) {
    let Ok((outcome, ForAgent(agent))) = turns.get(settled.entity) else {
        return;
    };
    let Ok((child, id, usage)) = children.get(*agent) else {
        return;
    };
    let Ok(workflow) = workflows.get(child.call) else {
        return;
    };
    let (status, answer) = match outcome {
        TurnOutcome::Answered(message) => match message.final_answer() {
            Some(answer) => ("completed", answer),
            None => ("failed", "Child ended without a final answer".into()),
        },
        TurnOutcome::Failed(why) => ("failed", why.clone()),
        TurnOutcome::Stopped(why) => (
            "cancelled",
            format!(
                "Stopped: {}. Effects may have partly run.",
                why.as_deref().unwrap_or("cancelled")
            ),
        ),
    };
    let description = workflow.steps[child.index].request.title.clone();
    commands.entity(*agent).insert(WorkflowAnswer(ResultRow {
        description,
        agent: Some(id.0.clone()),
        status,
        answer: bounded(&answer, &id.0),
        usage: usage.map(|usage| usage.total),
    }));
}

fn spawn_failed(
    settled: On<Add<Settled>>,
    requests: Query<(&WorkflowRequest, &Outcome), With<SpawnSubagent>>,
    mut workflows: Query<&mut Workflow>,
) {
    let Ok((request, outcome)) = requests.get(settled.entity) else {
        return;
    };
    let Err(error) = &outcome.0 else {
        return;
    };
    if let Ok(mut workflow) = workflows.get_mut(request.call) {
        let description = workflow.steps[request.index].request.title.clone();
        workflow.results[request.index] = Some(ResultRow {
            description,
            agent: None,
            status: "failed",
            answer: preview(
                &error.message,
                "\n[truncated; full launch error remains in the session effect log]",
            ),
            usage: None,
        });
    }
}

/// Native reports steer their parent. Workflow results instead arrive exactly once
/// through their owning tool; claim the matching report as an ordinary effect.
fn claim_reports(
    mut reports: Query<(&mut Incoming, &mut Decision, &ForAgent), (With<Report>, ReadyToStart)>,
    children: Query<(&WorkflowChild, &AgentId, &WorkflowAnswer)>,
    mut workflows: Query<(&mut Workflow, &ForAgent)>,
) {
    for (mut report, mut decision, target) in &mut reports {
        let child = children
            .iter()
            .find(|(_, id, _)| report.author.from.as_deref() == Some(id.0.as_str()));
        if let Some((child, _, answer)) = child
            && let Ok((mut workflow, parent)) = workflows.get_mut(child.call)
            && parent.0 == target.0
        {
            workflow.results[child.index] = Some(answer.0.clone());
            decision.decide(
                Decider::plugin("vivy-subagents"),
                Outcome::custom(json!({"collected_by":"workflow","step":child.index})),
            );
        } else if let Some(id) = report.author.from.clone() {
            *report.text_mut() = bounded(report.text(), &id);
        }
    }
}

fn advance(
    mut workflows: Query<
        (Entity, &mut Workflow, &mut Decision, &ForAgent, &Effect),
        (ToolCalls, With<Started>, Without<Outcome>),
    >,
    children: Query<(Entity, &WorkflowChild)>,
    mut commands: Commands,
) {
    for (call, mut workflow, mut decision, &ForAgent(parent), effect) in &mut workflows {
        let completed = workflow.results.iter().flatten().count();
        if completed != workflow.shown {
            workflow.live.line(&format!(
                "{}: {completed}/{} completed",
                if workflow.chain { "Chain" } else { "Parallel" },
                workflow.steps.len()
            ));
            workflow.shown = completed;
        }
        if workflow.chain
            && completed == workflow.next
            && workflow.next < workflow.steps.len()
            && workflow
                .results
                .iter()
                .flatten()
                .all(|row| row.status == "completed")
        {
            let previous = workflow.results[workflow.next - 1]
                .as_ref()
                .unwrap()
                .answer
                .clone();
            spawn(
                call,
                parent,
                effect.id(),
                workflow.next,
                &workflow.steps[workflow.next],
                Some(&previous),
                &mut commands,
            );
            workflow.next += 1;
        }
        let failed = workflow
            .results
            .iter()
            .flatten()
            .any(|row| row.status != "completed");
        let finished = completed == workflow.steps.len() || (workflow.chain && failed);
        if !finished {
            continue;
        }
        let results: Vec<_> = workflow.results.iter().flatten().cloned().collect();
        let text = serde_json::to_string(&json!({"mode":if workflow.chain {"chain"} else {"parallel"},"completed":completed,"total":workflow.steps.len(),"results":results})).unwrap();
        let result = if failed {
            Outcome::refused(text)
        } else {
            ToolResult::success(text.into()).into()
        };
        decision.decide(Decider::plugin("vivy-subagents"), result);
        workflow.finished = true;
        // Keep ownership through this Gate: report claiming is ordered first.
        for (agent, owned) in &children {
            if owned.call == call {
                commands
                    .entity(agent)
                    .remove::<(WorkflowChild, WorkflowAnswer)>();
            }
        }
    }
}

fn cancel_removed(
    removed: On<Remove<Workflow>>,
    workflows: Query<&Workflow>,
    children: Query<(Entity, &WorkflowChild), With<ActiveTurn>>,
    mut commands: Commands,
) {
    if workflows
        .get(removed.entity)
        .is_ok_and(|workflow| workflow.finished)
    {
        return;
    }
    for (child, owner) in &children {
        if owner.call == removed.entity {
            commands.trigger(StopTurn::new(child));
        }
    }
}

fn stop_restored(
    mut calls: Query<
        (&ToolRequest, Option<&Interrupted>, &mut Decision),
        (With<Restored>, ReadyToStart),
    >,
) {
    for (request, interrupted, mut decision) in &mut calls {
        if ["task", PARALLEL, CHAIN].contains(&request.call.function.name.as_str())
            && !matches!(interrupted, Some(Interrupted::Settled(_)))
        {
            decision.decide(Decider::plugin("vivy-subagents"), Outcome::refused("Delegation was interrupted by restart; no tasks were replayed. Start a new request if still needed."));
        }
    }
}

fn bounded(answer: &str, agent: &str) -> String {
    preview(
        answer,
        &format!("\n[truncated; full answer remains in child {agent}; /agents {agent}]"),
    )
}

fn preview(answer: &str, suffix: &str) -> String {
    if answer.len() <= INLINE_BYTES {
        return answer.to_owned();
    }
    let mut end = INLINE_BYTES.saturating_sub(suffix.len());
    while !answer.is_char_boundary(end) {
        end -= 1;
    }
    format!("{}{suffix}", &answer[..end])
}

#[cfg(test)]
mod truncation_tests {
    use super::*;
    #[test]
    fn unicode_answer_including_its_reference_stays_inside_the_inline_budget() {
        let text = bounded(&"界".repeat(INLINE_BYTES), "child-123");
        assert!(
            text.len() <= INLINE_BYTES,
            "preview plus reference exceeds its byte budget: {}",
            text.len()
        );
        assert!(text.contains("truncated") && text.contains("/agents child-123"));
    }
}
