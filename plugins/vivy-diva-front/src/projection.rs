use crate::protocol::*;
use rig_harness::prelude::*;
use vivy_basics::workbench_status::{MemoryState, PythonState, WorkbenchStatus};

pub fn snapshot(world: &mut World, watermark: u64) -> Snapshot {
    let session = world
        .get_resource::<SessionPaths>()
        .and_then(|paths| paths.0.id())
        .map(|id| id.as_str().to_owned());
    let mut plugins: Vec<_> = world
        .query::<(&Name, &PluginSource)>()
        .iter(world)
        .map(|(name, source)| PluginInfo {
            name: name.as_str().into(),
            package: source.package.clone(),
            compiled: true,
        })
        .collect();
    plugins.sort_by(|a, b| a.name.cmp(&b.name));
    let foreground = vivy_durable::runtime::foreground(world);
    let assistant = foreground
        .and_then(|e| world.get::<AgentId>(e))
        .map(|id| id.0.clone());
    let agents = world
        .query::<(&AgentId, Has<ActiveTurn>)>()
        .iter(world)
        .map(|(id, working)| AgentInfo {
            id: id.0.clone(),
            working,
        })
        .collect();
    let configured = world
        .get_resource::<vivy_gateway::ProvidersState>()
        .is_some_and(|s| !s.config.instances.is_empty());
    let ready = foreground.is_some_and(|e| world.get::<Connection>(e).is_some());
    let model = Readiness {
        compiled: world.contains_resource::<Models>(),
        configured,
        ready,
        state: if ready {
            "ready"
        } else if configured {
            "configured"
        } else {
            "unconfigured"
        }
        .into(),
        issue: world
            .get_resource::<vivy_gateway::ProvidersState>()
            .and_then(|s| s.issue.clone()),
    };
    let status = foreground
        .and_then(|e| world.get::<WorkbenchStatus>(e))
        .copied();
    let python_compiled = plugins.iter().any(|p| p.package == "vivy-python");
    let python_state = status.map(|s| s.python).unwrap_or(PythonState::Unprepared);
    let python = Readiness {
        compiled: python_compiled,
        configured: matches!(
            python_state,
            PythonState::Prepared
                | PythonState::Starting
                | PythonState::Running
                | PythonState::Ready
        ),
        ready: matches!(python_state, PythonState::Ready),
        state: format!("{python_state:?}").to_lowercase(),
        issue: None,
    };
    let memory_state = status.map(|s| s.memory).unwrap_or_default();
    let memory = Readiness {
        compiled: plugins.iter().any(|p| p.name.contains("MemoryTools")),
        configured: memory_state != MemoryState::Off,
        ready: memory_state == MemoryState::Ready,
        state: format!("{memory_state:?}").to_lowercase(),
        issue: None,
    };
    let durable = foreground.and_then(|e| vivy_durable::runtime::inspect_committed(world, e));
    let shell_status = vivy_basics::shell::availability(world);
    let shell = Readiness {
        compiled: shell_status.is_some(),
        configured: shell_status.as_ref().is_some_and(|r| r.is_ok()),
        ready: shell_status.as_ref().is_some_and(|r| r.is_ok()),
        state: if shell_status.as_ref().is_some_and(|r| r.is_ok()) {
            "available"
        } else {
            "unavailable"
        }
        .into(),
        issue: shell_status.and_then(Result::err),
    };
    let revision = durable.map_or(0, |s| s.revision);
    let active_task = durable.and_then(|s| s.active.clone());
    let tasks = durable
        .map(crate::conversation::task_info)
        .unwrap_or_default();
    let tools = durable
        .map(crate::conversation::tool_info)
        .unwrap_or_default();
    let questions = crate::conversation::questions(world);
    Snapshot {
        revision,
        active_task,
        tasks,
        questions,
        tools,
        session,
        assistant,
        plugins,
        agents,
        core_version: env!("CARGO_PKG_VERSION").into(),
        protocol_version: VERSION,
        watermark,
        model,
        python,
        shell,
        memory,
    }
}
