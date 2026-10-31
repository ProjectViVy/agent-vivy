//! Persistent one-shot waits. Rig owns the clock, inbox and execution loop.
use super::*;
use rig_harness::{
    clock::{Due, WakeAt},
    rig_core::conversation::{Author, AuthorKind, RequestId},
};
use std::time::{Duration, SystemTime};

#[derive(Clone, Debug, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum Phase {
    Scheduled,
    Fired,
    Cancelled,
}

#[derive(Clone, Debug, Serialize, Deserialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct Delivery {
    pub task: String,
    pub step: u64,
}

#[derive(Clone, Debug, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct WakeRecord {
    pub id: String,
    pub generation: u64,
    pub task: String,
    pub task_generation: u64,
    pub deadline_ms: u64,
    pub created_ms: u64,
    pub reason: String,
    pub phase: Phase,
    pub fired_ms: Option<u64>,
    pub cancelled_ms: Option<u64>,
    /// The target task's operation boundary, frozen when the wake fires.
    pub step: Option<u64>,
    pub ready: bool,
    pub delivered: Option<Delivery>,
}
impl WakeRecord {
    fn key(&self) -> String {
        format!("{}/{}", self.id, self.generation)
    }
    fn outstanding(&self) -> bool {
        self.phase != Phase::Cancelled && self.delivered.is_none()
    }
    pub(super) fn input(&self) -> Incoming {
        let mut author = Author::plugin("vivy.wake");
        author.request = Some(RequestId(self.key()));
        Incoming::new(format!("Scheduled wake {} generation {} for task {} generation {}. Reason: {}. Scheduled UTC milliseconds: {}. Fired UTC milliseconds: {}. Decide whether to continue this task, wait again, defer or finish silently.",self.id,self.generation,self.task,self.task_generation,self.reason,self.deadline_ms,self.fired_ms.unwrap_or(0)),DeliveryMode::Steer).with_author(author)
    }
}

#[derive(Clone, Debug, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct Proposal {
    pub deadline_ms: u64,
    pub created_ms: u64,
    pub previous: Option<u64>,
}

#[derive(Component)]
struct Alarm;
#[derive(Deserialize, schemars::JsonSchema)]
#[serde(deny_unknown_fields)]
struct StatusArgs {}

pub(super) fn install(app: &mut App) {
    app.add_systems(First, tick.after(storage::initialize).before(super::recover_inputs))
        .add_observer(delivered)
        .add_open_tool("wake_status", "Read committed wake identities, generations and delivery state before cancelling, replacing or reconciling an uncertain schedule mutation.", ToolOptions::default(),
            |event: On<ToolCalled<StatusArgs>>, mut commands: Commands| {
                let (call,agent)=(event.call,event.agent);
                commands.queue(move |world: &mut World| {
                    let value=status(world,agent);
                    world.entity_mut(call).insert(Outcome::from(ToolResult::success(rig_harness::rig_core::tool::ToolOutput::text(value.to_string()))));
                });
            })
        .add_command("wake", "wake list | cancel <id> <generation> | reschedule <id> <generation> <seconds>", command);
}

fn latest<'a>(state: &'a DurableAgentState, task: &str) -> Option<&'a WakeRecord> {
    state.wakes.iter().rev().find(|w| w.task == task)
}
fn deadline(now: u64, seconds: u64) -> io::Result<u64> {
    seconds
        .checked_mul(1000)
        .and_then(|s| now.checked_add(s))
        .filter(|ms| {
            SystemTime::UNIX_EPOCH
                .checked_add(Duration::from_millis(*ms))
                .is_some()
        })
        .ok_or_else(|| invalid("wait deadline is out of range"))
}
pub(super) fn propose(
    state: &DurableAgentState,
    control: &Control,
) -> io::Result<Option<Proposal>> {
    if matches!(control.action.as_str(), "continue" | "switch")
        && state
            .task(&control.task)
            .is_some_and(|t| t.status == Status::Waiting)
    {
        return Err(invalid("cancel the scheduled wait before resuming early"));
    }
    match control.action.as_str() {
        "wait" => {
            if uncertain(state, &control.task) {
                return Err(invalid(
                    "inspect and resolve uncertain operations before waiting",
                ));
            }
            if control.reason.as_ref().is_none_or(|s| s.trim().is_empty()) {
                return Err(invalid("wait requires a reason"));
            }
            let now = now_ms();
            Ok(Some(Proposal {
                deadline_ms: deadline(
                    now,
                    control
                        .seconds
                        .ok_or_else(|| invalid("wait requires seconds"))?,
                )?,
                created_ms: now,
                previous: latest(state, &control.task).map(|w| w.generation),
            }))
        }
        "cancel_wake" => {
            if !cancel_valid(
                state,
                control.wake_id.as_deref().unwrap_or(""),
                control.wake_generation.unwrap_or(u64::MAX),
                Some(&control.task),
            ) {
                return Err(invalid("stale or delivered wake"));
            }
            Ok(None)
        }
        _ => Ok(None),
    }
}
pub(super) fn control_valid(state: &DurableAgentState, pending: &PendingControl) -> bool {
    let c = &pending.request;
    match c.action.as_str() {
        "wait" => {
            !uncertain(state, &c.task)
                && pending
                    .wake
                    .as_ref()
                    .is_some_and(|p| latest(state, &c.task).map(|w| w.generation) == p.previous)
        }
        "cancel_wake" => cancel_valid(
            state,
            c.wake_id.as_deref().unwrap_or(""),
            c.wake_generation.unwrap_or(u64::MAX),
            Some(&c.task),
        ),
        "continue" | "switch" => state
            .task(&c.task)
            .is_some_and(|t| t.status != Status::Waiting),
        _ => true,
    }
}

fn schedule(
    state: &mut DurableAgentState,
    task: &str,
    proposal: &Proposal,
    reason: &str,
) -> WakeRecord {
    for w in state
        .wakes
        .iter_mut()
        .filter(|w| w.task == task && w.outstanding())
    {
        w.phase = Phase::Cancelled;
        w.cancelled_ms = Some(proposal.created_ms);
    }
    let record = state.task_mut(task).unwrap();
    record.status = Status::Waiting;
    let wake = WakeRecord {
        id: format!("wake:{task}"),
        generation: proposal.previous.map_or(0, |n| n + 1),
        task: task.into(),
        task_generation: record.generation,
        deadline_ms: proposal.deadline_ms,
        created_ms: proposal.created_ms,
        reason: reason.into(),
        phase: Phase::Scheduled,
        fired_ms: None,
        cancelled_ms: None,
        step: None,
        ready: false,
        delivered: None,
    };
    state.wakes.push(wake.clone());
    state.bump();
    wake
}
fn cancel_valid(state: &DurableAgentState, id: &str, generation: u64, task: Option<&str>) -> bool {
    state
        .wakes
        .iter()
        .find(|w| w.id == id && w.generation == generation)
        .is_some_and(|w| {
            w.outstanding()
                && task.is_none_or(|t| w.task == t)
                && state.task(&w.task).is_some_and(|t| {
                    t.generation == w.task_generation
                        && !matches!(
                            t.status,
                            Status::Cancelled | Status::Completed | Status::Failed
                        )
                })
        })
}
fn cancel_state(state: &mut DurableAgentState, id: &str, generation: u64) {
    let wake = state
        .wakes
        .iter_mut()
        .find(|w| w.id == id && w.generation == generation)
        .unwrap();
    wake.phase = Phase::Cancelled;
    wake.cancelled_ms = Some(now_ms());
    let task = wake.task.clone();
    if let Some(t) = state.task_mut(&task) {
        t.status = Status::Suspended;
    }
    state.bump();
}
pub(super) fn apply(state: &mut DurableAgentState, pending: &PendingControl) {
    let c = &pending.request;
    if c.action == "wait" {
        schedule(
            state,
            &c.task,
            pending.wake.as_ref().unwrap(),
            c.reason.as_deref().unwrap(),
        );
    } else {
        cancel_state(
            state,
            c.wake_id.as_deref().unwrap(),
            c.wake_generation.unwrap(),
        );
    }
}

/// Schedule an inactive waiting/suspended task using absolute UTC milliseconds.
/// `previous` is None only for the first schedule, otherwise the latest generation.
/// The returned record is pending until inspect_committed contains it.
pub fn schedule_wake(
    world: &mut World,
    agent: Entity,
    task: &str,
    task_generation: u64,
    previous: Option<u64>,
    deadline_ms: u64,
    reason: &str,
) -> io::Result<WakeRecord> {
    if !healthy(world) {
        return Err(io::Error::other("durable session is not writable"));
    }
    let mut s = world
        .get_mut::<DurableAgentState>(agent)
        .ok_or_else(|| invalid("not a durable agent"))?;
    if reason.trim().is_empty()
        || SystemTime::UNIX_EPOCH
            .checked_add(Duration::from_millis(deadline_ms))
            .is_none()
    {
        return Err(invalid("invalid reason or deadline"));
    }
    if s.active.as_deref() == Some(task)
        || s.control.as_ref().is_some_and(|c| c.request.task == task)
        || uncertain(&s, task)
        || !s.task(task).is_some_and(|t| {
            t.generation == task_generation
                && (matches!(t.status, Status::Waiting | Status::Suspended)
                    || (t.status == Status::Queued && t.delivered))
        })
    {
        return Err(invalid(
            "task is active, ended, uncertain or has a stale generation",
        ));
    }
    if latest(&s, task).map(|w| w.generation) != previous || previous == Some(u64::MAX) {
        return Err(invalid("stale wake generation"));
    }
    let result = schedule(
        &mut s,
        task,
        &Proposal {
            deadline_ms,
            created_ms: now_ms(),
            previous,
        },
        reason,
    );
    drop(s);
    world.resource::<Wake>().wake();
    Ok(result)
}

/// Cancel only the selected outstanding wake. The revision needs a receipt.
pub fn cancel_wake(world: &mut World, agent: Entity, id: &str, generation: u64) -> io::Result<u64> {
    if !healthy(world) {
        return Err(io::Error::other("durable session is not writable"));
    }
    let mut s = world
        .get_mut::<DurableAgentState>(agent)
        .ok_or_else(|| invalid("not a durable agent"))?;
    if !cancel_valid(&s, id, generation, None) {
        return Err(invalid("stale, cancelled or delivered wake"));
    }
    cancel_state(&mut s, id, generation);
    let revision = s.revision;
    drop(s);
    world.resource::<Wake>().wake();
    Ok(revision)
}

pub fn inspect_committed(world: &World, agent: Entity) -> Option<&DurableAgentState> {
    committed(world, agent)
}
fn status(world: &World, agent: Entity) -> Value {
    committed(world,agent).map_or(json!({"committed":false}),|s|json!({"committed":true,"revision":s.revision,"wakes":s.wakes,"pending_control":s.control}))
}
fn command(In(args): In<CommandArgs>, world: &mut World) {
    let parts: Vec<_> = args.args.split_whitespace().collect();
    let result = (|| -> io::Result<String> {
        match parts.as_slice() {
            [] | ["list"] => Ok(status(world, args.agent).to_string()),
            ["cancel", id, generation] => {
                let n = generation
                    .parse()
                    .map_err(|_| invalid("invalid generation"))?;
                Ok(format!(
                    "Cancellation pending at revision {}.",
                    cancel_wake(world, args.agent, id, n)?
                ))
            }
            ["reschedule", id, generation, seconds] => {
                let n = generation
                    .parse::<u64>()
                    .map_err(|_| invalid("invalid generation"))?;
                let secs = seconds.parse().map_err(|_| invalid("invalid seconds"))?;
                let wake = committed(world, args.agent)
                    .and_then(|s| s.wakes.iter().find(|w| w.id == *id && w.generation == n))
                    .cloned()
                    .ok_or_else(|| invalid("unknown committed wake"))?;
                let w = schedule_wake(
                    world,
                    args.agent,
                    &wake.task,
                    wake.task_generation,
                    Some(n),
                    deadline(now_ms(), secs)?,
                    &wake.reason,
                )?;
                Ok(format!(
                    "Reschedule pending: {} generation {} at UTC milliseconds {}.",
                    w.id, w.generation, w.deadline_ms
                ))
            }
            _ => Err(invalid(
                "Use /wake list, /wake cancel <id> <generation>, or /wake reschedule <id> <generation> <seconds>.",
            )),
        }
    })();
    world.write_message(match result {
        Ok(s) => Notice::info(args.agent, s),
        Err(e) => Notice::error(args.agent, e.to_string()),
    });
}

fn input_key(input: &Incoming) -> Option<&str> {
    if input.author.kind != AuthorKind::Plugin("vivy.wake".into()) {
        return None;
    }
    input.author.request.as_ref().map(|id| id.0.as_str())
}
pub(super) fn pending(s: &DurableAgentState) -> bool {
    s.wakes
        .iter()
        .any(|w| w.phase == Phase::Fired && w.delivered.is_none())
}
pub(super) fn accepted(world: &mut World, agent: Entity) -> bool {
    let keys: Vec<_> = world
        .query_filtered::<(&ForAgent, &Incoming), With<Started>>()
        .iter(world)
        .filter(|(a, _)| a.0 == agent)
        .filter_map(|(_, i)| input_key(i).map(str::to_owned))
        .collect();
    world.get::<DurableAgentState>(agent).is_none_or(|s| {
        s.wakes
            .iter()
            .filter(|w| w.phase == Phase::Fired && w.delivered.is_none())
            .all(|w| keys.contains(&w.key()))
    })
}

fn tick(world: &mut World) {
    tick_at(world, now_ms());
}
fn tick_at(world: &mut World, now: u64) {
    if !healthy(world) {
        return;
    }
    let agents: Vec<_> = world
        .query::<(Entity, &DurableAgentState)>()
        .iter(world)
        .map(|(a, s)| (a, s.clone()))
        .collect();
    for (agent, snapshot) in agents {
        for w in snapshot.wakes.iter().filter(|w| w.outstanding()) {
            let valid = snapshot.task(&w.task).is_some_and(|t| {
                t.generation == w.task_generation
                    && !matches!(
                        t.status,
                        Status::Cancelled | Status::Completed | Status::Failed
                    )
            });
            if !valid {
                let mut s = world.get_mut::<DurableAgentState>(agent).unwrap();
                let wake = s.wakes.iter_mut().find(|v| v.key() == w.key()).unwrap();
                wake.phase = Phase::Cancelled;
                wake.cancelled_ms = Some(now);
                s.bump();
                continue;
            }
            // A zero-delay wait must finish its old turn before becoming queued.
            if w.phase == Phase::Scheduled
                && w.deadline_ms <= now
                && snapshot.active.as_deref() != Some(&w.task)
                && committed(world, agent)
                    .is_some_and(|s| s.wakes.contains(w) && s.active.as_deref() != Some(&w.task))
            {
                let mut s = world.get_mut::<DurableAgentState>(agent).unwrap();
                let step = s.task(&w.task).unwrap().next_step;
                let wake = s.wakes.iter_mut().find(|v| v.key() == w.key()).unwrap();
                wake.phase = Phase::Fired;
                wake.fired_ms = Some(now);
                wake.step = Some(step);
                s.task_mut(&w.task).unwrap().status = Status::Queued;
                s.bump();
                probe("wake_fired", &s);
            }
        }
        // Restored Incoming is already the delivery; never manufacture a second.
        let existing: Vec<_> = world
            .query::<(&ForAgent, &Incoming)>()
            .iter(world)
            .filter(|(a, _)| a.0 == agent)
            .filter_map(|(_, i)| input_key(i).map(str::to_owned))
            .collect();
        let fire: Vec<_> = committed(world, agent)
            .into_iter()
            .flat_map(|s| s.wakes.iter())
            .filter(|w| w.phase == Phase::Fired && w.delivered.is_none())
            .cloned()
            .collect();
        for w in fire {
            if !existing.contains(&w.key())
                && world.get::<DurableAgentState>(agent).is_some_and(|s| {
                    s.wakes.iter().any(|v| {
                        v.key() == w.key() && v.phase == Phase::Fired && v.delivered.is_none()
                    })
                })
            {
                world.spawn((w.input(), ForAgent(agent)));
            }
        }
    }
    // Withdraw stale notifications even after they entered Rig's inbox.
    let inputs: Vec<_> = world
        .query_filtered::<(Entity, &ForAgent, &Incoming), (Without<Outcome>, Without<Settled>)>()
        .iter(world)
        .filter_map(|(e, a, i)| input_key(i).map(|k| (e, a.0, k.to_owned())))
        .collect();
    for (e, a, key) in inputs {
        let valid = world.get::<DurableAgentState>(a).is_some_and(|s| {
            s.wakes
                .iter()
                .any(|w| w.key() == key && w.phase == Phase::Fired)
        });
        if !valid {
            decide(
                world,
                e,
                Outcome::refused("wake cancelled or replaced before delivery"),
            );
            release(world, e);
        }
    }
    let earliest = world
        .query::<&DurableAgentState>()
        .iter(world)
        .flat_map(|s| s.wakes.iter())
        .filter(|w| w.phase == Phase::Scheduled)
        .map(|w| w.deadline_ms)
        .min();
    let alarms: Vec<_> = world
        .query_filtered::<Entity, With<Alarm>>()
        .iter(world)
        .collect();
    if let Some(ms) = earliest {
        let at = WakeAt(SystemTime::UNIX_EPOCH + Duration::from_millis(ms));
        let entity = alarms
            .first()
            .copied()
            .unwrap_or_else(|| world.spawn(Alarm).id());
        // Re-arm even when Due was marked before the wall clock moved backwards.
        if world.get::<WakeAt>(entity) != Some(&at)
            || (ms > now && world.get::<Due>(entity).is_some())
        {
            world.entity_mut(entity).insert(at);
        }
    } else {
        for e in alarms {
            world.despawn(e);
        }
    }
}

pub(super) fn admit(world: &mut World, call: Entity, agent: Entity, input: &Incoming) -> bool {
    if input
        .command()
        .is_some_and(|s| s.split_whitespace().next() == Some("wake"))
    {
        release(world, call);
        return true;
    }
    let Some(key) = input_key(input) else {
        return false;
    };
    hold(world, call);
    let wake = world
        .get::<DurableAgentState>(agent)
        .and_then(|s| s.wakes.iter().find(|w| w.key() == key))
        .cloned();
    let Some(w) = wake.filter(|w| {
        w.phase == Phase::Fired && w.delivered.is_none() && digest(&w.input()) == digest(input)
    }) else {
        decide(world, call, Outcome::refused("stale or invalid wake input"));
        release(world, call);
        return true;
    };
    if !healthy(world)
        || !committed(world, agent).is_some_and(|s| {
            s.wakes
                .iter()
                .any(|v| v.key() == key && v.phase == Phase::Fired)
        })
    {
        return true;
    }
    let mut s = world.get_mut::<DurableAgentState>(agent).unwrap();
    if s.active.is_none() {
        s.active = Some(w.task.clone());
        let task = s.task_mut(&w.task).unwrap();
        task.status = Status::Running;
        task.outcome = None;
        s.decision_pending = false;
        s.bump();
    }
    let wake = s.wakes.iter_mut().find(|w| w.key() == key).unwrap();
    if !wake.ready {
        wake.ready = true;
        s.bump();
    }
    drop(s);
    if committed(world, agent).is_some_and(|s| {
        s.active.is_some()
            && s.wakes
                .iter()
                .any(|v| v.key() == key && v.ready && v.phase == Phase::Fired)
    }) {
        release(world, call);
    }
    true
}
fn delivered(
    event: On<Add<Outcome>>,
    inputs: Query<(&ForAgent, &Incoming, &Outcome)>,
    mut agents: Query<&mut DurableAgentState>,
) {
    let Ok((agent, input, result)) = inputs.get(event.entity) else {
        return;
    };
    let Some(key) = input_key(input) else {
        return;
    };
    if result.0.is_err() {
        return;
    }
    let Ok(mut s) = agents.get_mut(agent.0) else {
        return;
    };
    let Some(task) = s.active.as_ref().and_then(|id| s.task(id)) else {
        return;
    };
    let step = s
        .operations
        .iter()
        .filter(|o| o.task == task.id && o.kind == "model" && o.outcome.is_none() && !o.published)
        .map(|o| o.step)
        .min()
        .unwrap_or(task.next_step);
    let delivery = Delivery {
        task: task.id.clone(),
        step,
    };
    if let Some(w) = s
        .wakes
        .iter_mut()
        .find(|w| w.key() == key && w.phase == Phase::Fired && w.delivered.is_none())
    {
        w.delivered = Some(delivery);
        s.bump();
    }
}

pub(super) fn notices(state: &DurableAgentState, task: &str) -> Vec<(u64, String, Incoming)> {
    let mut events: Vec<_> = state
        .wakes
        .iter()
        .filter_map(|w| {
            let delivered = w.delivered.as_ref()?;
            let step = if delivered.task == task {
                delivered.step
            } else if w.task == task {
                w.step?
            } else {
                return None;
            };
            Some((step, format!("{}/input", w.key()), w.input()))
        })
        .collect();
    events.sort_by_key(|e| e.0);
    events
}

/// Each lifecycle source uses only immutable occurrence fields, never live state.
pub(super) fn sources(state: &DurableAgentState) -> Vec<(String, String)> {
    let mut events = vec![];
    for w in &state.wakes {
        events.push((
            format!("{}/scheduled", w.key()),
            format!(
                "Task {} waits: {}. Created UTC ms {}, deadline UTC ms {}.",
                w.task, w.reason, w.created_ms, w.deadline_ms
            ),
        ));
        if let Some(ms) = w.fired_ms {
            events.push((
                format!("{}/fired", w.key()),
                format!(
                    "Wake {} fired at UTC ms {} for task {}.",
                    w.key(),
                    ms,
                    w.task
                ),
            ));
        }
        if let Some(ms) = w.cancelled_ms {
            events.push((
                format!("{}/cancelled", w.key()),
                format!(
                    "Wake {} cancelled at UTC ms {} for task {}.",
                    w.key(),
                    ms,
                    w.task
                ),
            ));
        }
    }
    events
}

pub(super) fn validate(s: &DurableAgentState) -> io::Result<()> {
    let mut generations = BTreeMap::new();
    let mut live = std::collections::BTreeSet::new();
    for w in &s.wakes {
        let expected = generations
            .get(&w.task)
            .map_or(0, |n: &u64| n.saturating_add(1));
        if w.id != format!("wake:{}", w.task)
            || w.generation != expected
            || s.task(&w.task).is_none()
            || w.reason.trim().is_empty()
            || (w.fired_ms.is_some() != w.step.is_some())
            || (w.phase == Phase::Fired && w.fired_ms.is_none())
            || (w.phase == Phase::Scheduled
                && (w.fired_ms.is_some() || w.ready || w.delivered.is_some()))
            || (w.phase == Phase::Cancelled) != w.cancelled_ms.is_some()
            || (w.delivered.is_some() && (!w.ready || w.fired_ms.is_none()))
            || (w.outstanding() && !live.insert(&w.task))
            || SystemTime::UNIX_EPOCH
                .checked_add(Duration::from_millis(w.deadline_ms))
                .is_none()
        {
            return Err(invalid(
                "invalid durable wake identity, phase or generation",
            ));
        }
        if w.delivered
            .as_ref()
            .is_some_and(|d| s.task(&d.task).is_none())
        {
            return Err(invalid("wake recipient missing"));
        }
        generations.insert(w.task.clone(), w.generation);
    }
    if s.tasks.iter().any(|t| {
        t.status == Status::Waiting
            && !s.wakes.iter().any(|w| {
                w.task == t.id && w.task_generation == t.generation && w.phase == Phase::Scheduled
            })
    }) {
        return Err(invalid("waiting task has no matching schedule"));
    }
    if let Some(p) = &s.control {
        if p.request.action == "wait"
            && (p.wake.is_none()
                || p.request.seconds.is_none()
                || p.request
                    .reason
                    .as_ref()
                    .is_none_or(|r| r.trim().is_empty()))
        {
            return Err(invalid("wait control lacks a frozen deadline"));
        }
        if p.request.action == "wait"
            && p.wake.as_ref().is_some_and(|w| {
                deadline(w.created_ms, p.request.seconds.unwrap_or(0)).ok() != Some(w.deadline_ms)
            })
        {
            return Err(invalid("wait deadline does not match its frozen origin"));
        }
    }
    Ok(())
}

#[cfg(test)]
mod tests;
