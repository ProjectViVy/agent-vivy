//! Vivy delegation, using the native Rig subagent execution path.

use rig_basics::subagents::{SpawnedAgent, SubagentPolicy};
use rig_harness::prelude::*;
use serde_json::json;

mod admission;
mod roles;
#[cfg(feature = "tui")]
mod ui;
mod workflows;
use roles::{AppliedRole, PreparedTask};

/// The subagents plugin's stage systems, for others to order against.
#[derive(SystemSet, Clone, Copy, Debug, PartialEq, Eq, Hash)]
pub enum SubagentSystems {
    /// In [`Stage::Prepare`], in
    /// [`PrepareSystems::Edit`](rig_harness::PrepareSystems::Edit): a
    /// child's role and work directory go on its request.
    Prepare,
}

#[derive(Default)]
pub struct VivySubagentsPlugin;

impl Plugin for VivySubagentsPlugin {
    fn build(&self, app: &mut App) {
        if !app.is_plugin_added::<rig_basics::subagents::SubagentsPlugin>() {
            app.add_plugins(rig_basics::subagents::SubagentsPlugin);
        }
        if !app.is_plugin_added::<rig_basics::telemetry::usage::UsagePlugin>() {
            app.add_plugins(rig_basics::telemetry::usage::UsagePlugin);
        }
        let policy = app
            .world()
            .resource::<Settings>()
            .get::<Option<SubagentPolicy>>("subagents")
            .ok()
            .flatten()
            .unwrap_or(SubagentPolicy { max_depth: 1 });
        app.insert_resource(policy)
            .init_resource::<ProjectRoot>()
            .keeps_saved::<AppliedRole>()
            .keeps_saved::<WorkDir>()
            .add_observer(apply_prepared)
            .add_observer(roles::show_prompt)
            .configure_sets(
                AgentPass,
                SubagentSystems::Prepare.in_set(PrepareSystems::Edit),
            )
            .add_systems(AgentPass, roles::prepare.in_set(SubagentSystems::Prepare))
            .add_command("roles", "List available subagent roles", roles::list_roles);
        RigHome::of(app.world_mut());
        workflows::add(app);
        admission::add(app);
        #[cfg(feature = "tui")]
        ui::add(app);
    }

    fn finish(&self, app: &mut App) {
        let world = app.world_mut();
        let mut definitions = world.query::<&mut ToolDef>();
        for mut definition in definitions.iter_mut(world) {
            if definition.0.name.as_str() != "task" {
                continue;
            }
            let definition = std::sync::Arc::make_mut(&mut definition.0);
            let properties = definition
                .parameters
                .get_mut("properties")
                .and_then(|v| v.as_object_mut())
                .unwrap();
            properties.insert("profile".into(), json!({"type":"string","description":"Named Vivy role; /roles lists built-in, user and trusted project roles."}));
            properties.insert("cwd".into(), json!({"type":"string","description":"Working directory inside this project, relative to yours. Project context stays shared."}));
        }
    }
}

/// Apply the already validated snapshot before any child model call.
fn apply_prepared(
    spawned: On<Add<SpawnedAgent>>,
    requests: Query<(&SpawnedAgent, &PreparedTask)>,
    access: Query<&ToolAccess>,
    lineage: Query<&SpawnedBy>,
    tools: Query<&ToolDef>,
    policy: Res<SubagentPolicy>,
    mut commands: Commands,
) {
    let Ok((SpawnedAgent(child), prepared)) = requests.get(spawned.entity) else {
        return;
    };
    if let Some(role) = &prepared.role {
        commands.entity(*child).insert(role.clone());
    }
    if let Some(cwd) = &prepared.cwd {
        commands.entity(*child).insert(WorkDir(cwd.clone()));
    }
    if lineage.iter_ancestors::<SpawnedBy>(*child).count() >= policy.max_depth
        && let Ok(access) = access.get(*child)
    {
        let names = tools
            .iter()
            .map(|tool| tool.0.name.as_str())
            .filter(|name| {
                access.allows(name) && !["task", "task_parallel", "task_chain"].contains(name)
            })
            .map(str::to_owned)
            .collect();
        commands.entity(*child).insert(ToolAccess::Only(names));
    }
}

#[cfg(test)]
mod tests;
#[cfg(test)]
mod workflows_tests;
