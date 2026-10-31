//! Bounded admission, adapted from Rig corpus/base's SubagentPolicy queue.
//! Follow-up turns use the same slots as newly spawned children.
use rig_basics::subagents::{
    Lifecycle, OpenRequests, Report, Role, SpawnSubagent, SubagentSystems,
};
use rig_harness::prelude::*;
use std::collections::HashSet;

#[derive(Component)]
struct Queued(u64);

pub(super) fn add(app: &mut App) {
    app.add_systems(
        AgentPass,
        queue
            .in_set(Stage::Gate)
            .after(SubagentSystems::Gate)
            .after(super::workflows::WorkflowSystems),
    );
    app.add_observer(cancel_pending);
}

type AdmissionQuery<'w, 's, F> = Query<
    'w,
    's,
    (
        Entity,
        Option<&'static SpawnSubagent>,
        Option<&'static Incoming>,
        &'static ForAgent,
        &'static Decision,
    ),
    F,
>;

fn queue(
    new: AdmissionQuery<ReadyToStart>,
    held: Query<(Entity, &Queued, &HoldOf)>,
    requests: AdmissionQuery<Without<Outcome>>,
    children: Query<(Entity, &Lifecycle), With<SpawnedBy>>,
    lineage: Query<&SpawnedBy>,
    mut next: Local<u64>,
    mut commands: Commands,
) {
    let mut occupied: HashSet<_> = children
        .iter()
        .filter(|(_, state)| matches!(state, Lifecycle::Working | Lifecycle::WaitingOn(_)))
        .map(|(entity, _)| entity)
        .collect();
    let mut count = occupied.len();
    let mut waiting: Vec<_> = held.iter().collect();
    waiting.sort_by_key(|(_, queued, _)| queued.0);
    for (hold, _, &HoldOf(request)) in waiting {
        let Ok((_, spawn, incoming, &ForAgent(target), decision)) = requests.get(request) else {
            commands.entity(hold).despawn();
            continue;
        };
        if decision.is_decided() {
            commands.entity(hold).despawn();
            continue;
        }
        let needs = needs_slot(spawn, incoming, target, &children, &occupied);
        if needs != Some(true) || count < 4 {
            commands.entity(hold).despawn();
            if needs == Some(true) {
                count += 1;
                if spawn.is_none() {
                    occupied.insert(target);
                }
            }
        }
    }
    for (request, spawn, incoming, &ForAgent(target), decision) in &new {
        if decision.is_decided() {
            continue;
        }
        if needs_slot(spawn, incoming, target, &children, &occupied) != Some(true) {
            continue;
        }
        if count < 4 {
            count += 1;
            if spawn.is_none() {
                occupied.insert(target);
            }
        } else if spawn.is_some() && lineage.get(target).is_ok() {
            // A nested parent retaining a slot must not wait forever for a
            // descendant's slot. It can retry after sibling work completes.
            commands.entity(request).insert(Outcome::refused(
                "Subagent capacity is full; finish sibling work before delegating further",
            ));
        } else {
            *next += 1;
            commands.spawn((
                Queued(*next),
                HoldOf(request),
                Name::new("waiting for a subagent slot"),
            ));
        }
    }
}

fn needs_slot(
    spawn: Option<&SpawnSubagent>,
    incoming: Option<&Incoming>,
    target: Entity,
    children: &Query<(Entity, &Lifecycle), With<SpawnedBy>>,
    occupied: &HashSet<Entity>,
) -> Option<bool> {
    if let Some(spawn) = spawn {
        return (spawn.role == Role::Task).then_some(true);
    }
    if incoming.is_some() && children.contains(target) {
        return Some(!occupied.contains(&target));
    }
    None
}

/// Stop also covers not-yet-started follow-ups. Native StopTurn handles active
/// turns; these messages have not started a turn and otherwise could revive it.
fn cancel_pending(
    stopped: On<StopTurn>,
    descendants: Query<&Spawned>,
    ids: Query<&AgentId>,
    pending: Query<
        (Entity, &ForAgent, Option<&Incoming>, Option<&SpawnSubagent>),
        (Without<Started>, Without<Outcome>, Without<Report>),
    >,
    mut open: Query<&mut OpenRequests>,
    mut commands: Commands,
) {
    let stopping: HashSet<_> = std::iter::once(stopped.entity)
        .chain(descendants.iter_descendants::<Spawned>(stopped.entity))
        .collect();
    let senders: HashSet<_> = stopping
        .iter()
        .filter_map(|agent| ids.get(*agent).ok())
        .map(|id| id.0.as_str())
        .collect();
    for (effect, &ForAgent(target), incoming, spawn) in &pending {
        if !stopping.contains(&target) {
            continue;
        }
        let delegated = spawn.is_some_and(|request| request.role == Role::Task);
        let follow_up = incoming.is_some_and(|message| {
            message
                .author
                .from
                .as_deref()
                .is_some_and(|id| senders.contains(id))
        });
        if !delegated && !follow_up {
            continue;
        }
        if let Some(message) = incoming
            && let Some(request) = &message.author.request
            && let Ok(mut requests) = open.get_mut(target)
        {
            requests.0.retain(|owed| {
                owed.id != *request || message.author.from.as_deref() != Some(owed.asker.0.as_str())
            });
        }
        commands.entity(effect).insert(Outcome::refused(
            "Delegation stopped before admission; no queued work was started",
        ));
    }
}
