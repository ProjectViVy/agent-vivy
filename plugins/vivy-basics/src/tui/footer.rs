//! Two display pages; the original status items remain owned by their producers.
use super::*;
use crate::setup::{OutcomeState, ProviderStatus};
use crate::workbench_status::{MemoryState, PythonState, WorkbenchStatus};
use rig_tui::ratatui::layout::Layout;
use rig_tui::{AppStatus, Side, StatusItem, Tone};
use std::time::Instant;

pub(super) const INTERVAL: Duration = Duration::from_secs(5);

#[derive(Component)]
pub(super) struct Rotation {
    agent: Option<Entity>,
    status: Option<WorkbenchStatus>,
    provider: Option<ProviderStatus>,
    workbench: bool,
    pub(super) next: Instant,
}

impl Default for Rotation {
    fn default() -> Self {
        Self {
            agent: None,
            status: None,
            provider: None,
            workbench: false,
            next: Instant::now() + INTERVAL,
        }
    }
}

pub(super) fn install(app: &mut App) {
    app.add_systems(PostUpdate, tick.before(TuiSystems::Prepare))
        .add_systems(PostUpdate, draw.in_set(TuiSystems::Draw));
}

fn tick(
    shown: Res<ShownAgent>,
    statuses: Query<&WorkbenchStatus>,
    providers: Query<&ProviderStatus>,
    mut panels: Query<(Entity, &mut Rotation)>,
    mut commands: Commands,
    mut redraw: MessageWriter<RequestRedraw>,
) {
    let status = shown.0.and_then(|agent| statuses.get(agent).ok()).copied();
    let provider = shown
        .0
        .and_then(|agent| providers.get(agent).ok())
        .filter(|p| p.0.outcome != OutcomeState::Ready || p.0.phase.is_some())
        .cloned();
    let available = status.is_some() || provider.is_some();
    let now = Instant::now();
    for (entity, mut page) in &mut panels {
        let changed = page.agent != shown.0 || page.status != status || page.provider != provider;
        if page.agent != shown.0 || page.status.is_some() != status.is_some() {
            page.workbench = false;
            page.next = now + INTERVAL;
        }
        let provider_urgent =
            |p: &ProviderStatus| p.0.outcome == OutcomeState::Error || p.0.phase.is_some();
        let previous_urgent = page.agent == shown.0
            && (page.status.is_some_and(WorkbenchStatus::urgent)
                || page.provider.as_ref().is_some_and(provider_urgent));
        let urgent = status.is_some_and(WorkbenchStatus::urgent)
            || provider.as_ref().is_some_and(provider_urgent);
        let before = page.workbench;
        if urgent || (previous_urgent && available) {
            page.workbench = true;
            page.next = now + INTERVAL;
        } else if available && now >= page.next {
            page.workbench = !page.workbench;
            page.next = now + INTERVAL;
        }
        if changed || before != page.workbench {
            redraw.write(RequestRedraw);
        }
        if available {
            commands
                .entity(entity)
                .insert(WakeAt::after(page.next.saturating_duration_since(now)));
        } else {
            commands.entity(entity).remove::<(WakeAt, Due)>();
        }
        page.agent = shown.0;
        page.status = status;
        page.provider = provider.clone();
    }
}

fn workbench_items(status: WorkbenchStatus) -> [StatusItem; 2] {
    let python = match status.python {
        PythonState::Unprepared => "Py unprepared",
        PythonState::Prepared => "Py prepared",
        PythonState::Stopped => "Py stopped",
        PythonState::Starting => "Py starting",
        PythonState::Running => "Py running",
        PythonState::Ready => "Py ready",
        PythonState::Preparing => "Py preparing",
        PythonState::Checking => "Py checking",
        PythonState::Error => "Py error",
    };
    let memory = match status.memory {
        MemoryState::Off => "Mem not init",
        MemoryState::Unchecked => "Mem not init",
        MemoryState::Ready => "Mem ready",
        MemoryState::Closed => "Mem closed",
        MemoryState::Error => "Mem error",
    };
    let tone = if status.python == PythonState::Error {
        Tone::Red
    } else if status.python == PythonState::Preparing {
        Tone::Yellow
    } else {
        Tone::Dim
    };
    [
        StatusItem::new("vivy-python", Side::Right, 10, 254)
            .shrinks_to(8)
            .says(python, tone),
        StatusItem::new("vivy-memory", Side::Right, 20, 190).says(
            memory,
            if status.memory == MemoryState::Error {
                Tone::Red
            } else {
                Tone::Dim
            },
        ),
    ]
}

fn draw(
    mut panels: Query<(&Rotation, &mut PanelCanvas)>,
    shown: Res<ShownAgent>,
    items: Query<&StatusItems>,
    active: Query<&ActiveTurn>,
    app: Res<AppStatus>,
) {
    let own = shown.0.and_then(|agent| items.get(agent).ok());
    let turn = shown
        .0
        .and_then(|agent| active.get(agent).ok())
        .and_then(|turn| items.get(turn.turn()).ok());
    for (page, mut canvas) in &mut panels {
        let area = canvas.area();
        if area.is_empty() {
            continue;
        }
        let extra = if page.workbench {
            let mut items = page
                .status
                .map(workbench_items)
                .map(Vec::from)
                .unwrap_or_default();
            if let Some(provider) = &page.provider {
                let (text, tone) = if provider.0.phase.is_some() {
                    ("Provider testing", Tone::Yellow)
                } else if provider.0.outcome == OutcomeState::Error {
                    ("Provider error", Tone::Red)
                } else {
                    ("Provider not init", Tone::Yellow)
                };
                items.push(StatusItem::new("vivy-provider", Side::Right, 5, 255).says(text, tone));
            }
            Some(items)
        } else {
            None
        };
        let mut projected: Vec<_> = [Some(&app.0), own, turn]
            .into_iter()
            .flatten()
            .flat_map(StatusItems::iter)
            .filter(|item| {
                extra.is_none() || !matches!(item.name, "tokens" | "cache" | "cost" | "context")
            })
            .cloned()
            .collect();
        if shown.0.is_none() {
            projected
                .push(StatusItem::new("agent", Side::Left, 30, 255).says("no agent", Tone::Dim));
        }
        if let Some(extra) = extra {
            projected.extend(extra);
        }
        let (left, right) = rig_tui::status::line(projected.iter(), area.width);
        let [left_area, right_area] =
            Layout::horizontal([Constraint::Min(0), Constraint::Length(right.width() as u16)])
                .areas(area);
        Clear.render(area, canvas.buffer_mut());
        left.render(left_area, canvas.buffer_mut());
        right.render(right_area, canvas.buffer_mut());
    }
}
