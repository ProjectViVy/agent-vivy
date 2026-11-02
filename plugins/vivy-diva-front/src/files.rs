//! Bounded filesystem jobs, outside the ECS schedule. Storage remains domain-owned.
use crate::{emit, protocol::*};
use rig_harness::prelude::*;
use std::{
    path::{Path, PathBuf},
    sync::{Mutex, mpsc},
};

#[derive(Resource)]
pub struct Jobs {
    sender: mpsc::SyncSender<ManagementCompleted>,
    receiver: Mutex<mpsc::Receiver<ManagementCompleted>>,
    active: usize,
}
impl Default for Jobs {
    fn default() -> Self {
        let (sender, receiver) = mpsc::sync_channel(4);
        Self {
            sender,
            receiver: Mutex::new(receiver),
            active: 0,
        }
    }
}

pub fn start(world: &mut World, id: String, method: Method) -> Result<Reply, Fault> {
    if world.resource::<Jobs>().active >= 4 {
        return Err(failure(
            "management queue is full; retry after active jobs finish",
        ));
    }
    let root = world.resource::<ProjectRoot>().0.clone();
    let home = RigHome::of(world).root().to_path_buf();
    let trusted = world
        .get_resource::<vivy_basics::trust::ProjectTrust>()
        .is_none_or(|t| t.trusted);
    let managed = world
        .resource::<vivy_python::managed::desktop::Desktop>()
        .clone();
    if let Method::ManagedDependencies(input) = &method {
        vivy_python::managed::desktop::dependencies(world, &input.id).map_err(failure)?;
    }
    let staged = if matches!(method, Method::ManagedValidate(_)) {
        let store = vivy_python::managed::PublicationStore::new(&root, &home);
        let current = store.current().map_err(failure)?;
        vivy_python::managed::staged_environment(world, &current.id).map_err(failure)?
    } else {
        None
    };
    let target = if matches!(method, Method::McpControl(_) | Method::McpSave(_)) {
        let agent = vivy_durable::runtime::foreground(world)
            .ok_or_else(|| failure("assistant is not ready"))?;
        Some(vivy_python::control::target(world, agent))
    } else {
        None
    };
    let sender = world.resource::<Jobs>().sender.clone();
    let wake = world.resource::<Wake>().clone();
    world.resource_mut::<Jobs>().active += 1;
    let operation = id.clone();
    std::thread::spawn(move || {
        let result = run(root, home, trusted, method, target, managed, staged);
        let (result, error) = match result {
            Ok(result) => (Some(result), None),
            Err(error) => (None, Some(error)),
        };
        let _ = sender.send(ManagementCompleted {
            operation,
            result,
            error,
        });
        wake.wake();
    });
    Ok(Reply::Operation(PendingOperation {
        operation: id,
        committed: false,
    }))
}
fn failure(message: impl ToString) -> Fault {
    Fault {
        code: ErrorCode::Conflict,
        message: rig_harness::rig_core::redact::text(&message.to_string()).into_owned(),
    }
}
#[cfg(feature = "persistent")]
fn persona(value: vivy_laputa::Persona, directory: &Path) -> PersonaView {
    let text = value.text();
    let files = vivy_laputa::ASSEMBLY_FILES
        .iter()
        .map(|name| {
            let name = *name;
            PersonaFileView {
                name: name.into(),
                text: value.file(name).into(),
                required: vivy_laputa::required(name),
                auto: vivy_laputa::auto(name),
            }
        })
        .collect();
    PersonaView {
        revision: value.revision,
        update_id: value.last_update_id,
        directory: directory.display().to_string(),
        text,
        files,
        unpublished: value.unpublished,
    }
}
#[cfg(feature = "persistent")]
fn persona_version(value: vivy_laputa::Persona, truncated: bool) -> PersonaVersionView {
    let mut text = value.text();
    let mut truncated = truncated;
    if text.len() > 4096 {
        let mut end = 4096;
        while !text.is_char_boundary(end) {
            end -= 1;
        }
        text.truncate(end);
        truncated = true;
    }
    PersonaVersionView {
        revision: value.revision,
        update_id: value.last_update_id,
        text,
        sources: value.sources,
        truncated,
    }
}
fn run(
    root: PathBuf,
    home: PathBuf,
    trusted: bool,
    method: Method,
    target: Option<vivy_python::control::Target>,
    managed: vivy_python::managed::desktop::Desktop,
    staged: Option<vivy_python::managed::Publication>,
) -> Result<ManagementResult, Fault> {
    #[cfg(feature = "persistent")]
    {
        let store = vivy_laputa::PersonaStore::new(&root);
        match &method {
            Method::PersonaSaveFile(input) => {
                let current = store.read().map_err(failure)?;
                if current.last_update_id != input.expected_update_id {
                    return Err(failure("persona update ID changed; reload before saving"));
                }
                store
                    .save_file(
                        input.revision,
                        &input.expected_update_id,
                        &input.update_id,
                        &input.name,
                        &input.text,
                        vec!["diva".into()],
                    )
                    .map_err(failure)?;
            }
            Method::PersonaRollback(input) => {
                store
                    .rollback(input.revision, input.target, &input.update_id)
                    .map_err(failure)?;
            }
            _ => {}
        }
        if matches!(
            method,
            Method::PersonaRead | Method::PersonaSaveFile(_) | Method::PersonaRollback(_)
        ) {
            let revisions = store.revisions().map_err(failure)?;
            let truncated = revisions.len() > 32;
            return Ok(ManagementResult::Persona {
                current: persona(store.read().map_err(failure)?, &store.directory()),
                history: revisions
                    .into_iter()
                    .take(32)
                    .map(|value| persona_version(value, truncated))
                    .collect(),
            });
        }
    }
    match method {
        Method::ManagedOpen(p) => Ok(ManagementResult::ManagedCandidate(
            managed
                .open(
                    &vivy_python::managed::PublicationStore::new(&root, &home),
                    &p.id,
                )
                .map_err(failure)?,
        )),
        Method::SkillRead(p) => {
            let s = vivy_basics::skills::local::list(&root, &home, trusted)
                .map_err(failure)?
                .into_iter()
                .find(|s| s.name == p.name)
                .ok_or_else(|| failure("skill not found"))?;
            Ok(ManagementResult::Skill(SkillDocument {
                name: s.name,
                description: s.description,
                text: s.text,
                revision: s.revision,
                editable: s.editable,
            }))
        }
        Method::ManagedRead
        | Method::ManagedImport(_)
        | Method::ManagedEdit(_)
        | Method::ManagedValidate(_)
        | Method::ManagedCancel(_)
        | Method::ManagedDependencies(_)
        | Method::ManagedPublish(_)
        | Method::ManagedRollback(_) => {
            let store = vivy_python::managed::PublicationStore::new(&root, &home);
            match method {
                Method::ManagedImport(p) => managed
                    .import(&store, &p.base, &p.name, std::path::Path::new(&p.source))
                    .map_err(failure)?,
                Method::ManagedEdit(p) => managed.edit(&store, &p.id, &p.files).map_err(failure)?,
                Method::ManagedValidate(p) => {
                    let checks = serde_json::from_str(&p.checks).map_err(failure)?;
                    managed
                        .validate(&store, &p.id, checks, staged)
                        .map_err(failure)?;
                }
                Method::ManagedCancel(p) => managed.cancel(&p.id).map_err(failure)?,
                Method::ManagedPublish(p) => managed
                    .publish(&store, &p.id, &p.update_id)
                    .map_err(failure)?,
                Method::ManagedRollback(p) => {
                    store
                        .rollback(&p.base, &p.target, &p.update_id)
                        .map_err(failure)?;
                }
                _ => {}
            }
            Ok(ManagementResult::Managed(
                managed.read(&store).map_err(failure)?,
            ))
        }
        Method::McpRead => Ok(ManagementResult::Mcp(
            vivy_python::control::read(&home).map_err(failure)?,
        )),
        Method::McpSave(input) => {
            let old = vivy_python::control::read(&home).map_err(failure)?;
            let view = vivy_python::control::save(&home, &input.revision, &input.connections)
                .map_err(failure)?;
            if let Some(target) = target {
                for server in old
                    .connections
                    .keys()
                    .chain(view.connections.keys())
                    .collect::<std::collections::BTreeSet<_>>()
                {
                    target.run("apply", server).map_err(failure)?;
                }
            }
            Ok(ManagementResult::Mcp(view))
        }
        Method::McpControl(input) => {
            let action = match input.action {
                McpAction::Inspect => "inspect",
                McpAction::Reconnect => "reconnect",
                McpAction::Close => "close",
            };
            let result = target
                .ok_or_else(|| failure("MCP worker target is unavailable"))?
                .run(action, &input.server)
                .map_err(failure)?;
            let tools = result["tools"]
                .as_array()
                .into_iter()
                .flatten()
                .map(|t| McpTool {
                    name: t["name"].as_str().unwrap_or_default().into(),
                    description: t["description"].as_str().unwrap_or_default().into(),
                    schema: rig_harness::rig_core::redact::text(&t["inputSchema"].to_string())
                        .into_owned(),
                })
                .collect();
            Ok(ManagementResult::McpResult(McpResult {
                server: input.server,
                state: result["state"].as_str().unwrap_or_default().into(),
                tools,
            }))
        }
        Method::SkillsRead
        | Method::SkillsRefresh
        | Method::SkillImport(_)
        | Method::SkillSave(_)
        | Method::SkillRemove(_) => {
            use vivy_basics::skills::local;
            match method {
                Method::SkillImport(input) => local::import(
                    &root,
                    &home,
                    trusted,
                    std::path::Path::new(&input.source),
                    &input.name,
                )
                .map_err(failure)?,
                Method::SkillSave(input) => {
                    local::save(&home, &input.name, &input.revision, &input.text)
                        .map_err(failure)?
                }
                Method::SkillRemove(input) => {
                    local::remove(&home, &input.name, &input.revision).map_err(failure)?
                }
                _ => {}
            }
            Ok(ManagementResult::Skills {
                skills: local::list(&root, &home, trusted)
                    .map_err(failure)?
                    .into_iter()
                    .map(|s| SkillDocument {
                        name: s.name,
                        description: s.description,
                        text: String::new(),
                        revision: s.revision,
                        editable: s.editable,
                    })
                    .collect(),
            })
        }
        Method::PromptsRead | Method::PromptSave(_) => {
            if let Method::PromptSave(input) = method {
                vivy_basics::prompt::save_file(
                    &root,
                    &home,
                    trusted,
                    &input.name,
                    &input.revision,
                    &input.text,
                )
                .map_err(failure)?;
            }
            let prompts = ["RECAP.md", "AUTODREAM.md"]
                .into_iter()
                .map(|name| {
                    let (source, text, revision) =
                        vivy_basics::prompt::inspect_file(&root, &home, trusted, name)
                            .map_err(failure)?;
                    Ok(PromptView {
                        name: name.into(),
                        source,
                        text,
                        revision,
                    })
                })
                .collect::<Result<Vec<_>, Fault>>()?;
            Ok(ManagementResult::Prompts { prompts })
        }
        _ => Err(Fault {
            code: ErrorCode::NotReady,
            message: "requested component was not compiled".into(),
        }),
    }
}
pub fn poll(world: &mut World) {
    loop {
        let result = world.resource::<Jobs>().receiver.lock().unwrap().try_recv();
        let Ok(result) = result else {
            break;
        };
        world.resource_mut::<Jobs>().active -= 1;
        if matches!(&result.result, Some(ManagementResult::Skills { .. })) {
            vivy_basics::skills::refresh_inventory(world);
        }
        emit(world, None, Payload::ManagementCompleted(result));
    }
}

pub fn recap(world: &mut World, refresh: bool) -> Result<Reply, Fault> {
    #[cfg(feature = "persistent")]
    {
        use vivy_durable::runtime::{self, assistant};
        let agent = runtime::foreground(world).ok_or_else(|| failure("assistant is not ready"))?;
        if refresh {
            assistant::request_refresh(world, agent).map_err(failure)?;
        }
        let state = assistant::inspect_committed(world, agent);
        Ok(Reply::Recap(RecapView {
            revision: state.map_or(0, |s| s.recap_revision),
            summary: state.map(|s| s.summary.clone()).unwrap_or_default(),
            covered: state.map(|s| s.covered.clone()).unwrap_or_default(),
            error: state.and_then(|s| s.error.clone()),
            refresh_pending: refresh || state.is_some_and(|s| s.refresh),
            jobs: state
                .map(|s| {
                    s.jobs
                        .iter()
                        .map(|j| RecapJob {
                            id: j.id.to_string(),
                            phase: j.phase.clone(),
                            spans: j.spans.len(),
                        })
                        .collect()
                })
                .unwrap_or_default(),
        }))
    }
    #[cfg(not(feature = "persistent"))]
    {
        let _ = (world, refresh);
        Err(failure("Recap adapter is not compiled"))
    }
}
