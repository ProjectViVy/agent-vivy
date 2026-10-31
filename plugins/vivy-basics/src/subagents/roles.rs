//! Role snapshots adapt the corpus AgentProfiles pattern to Vivy's existing trust.
//! Reference: Rig e1cd47d5, corpus/base/src/agent_profiles.rs (MIT licensed).

use bevy_ecs::system::SystemParam;
use rig_basics::subagents::SpawnSubagent;
use rig_harness::prelude::*;
use serde::Deserialize;
use std::collections::{BTreeMap, HashSet};
use std::path::{Path, PathBuf};

#[derive(Component, Reflect, Clone)]
#[reflect(Component, Saved, Clone)]
pub(super) struct AppliedRole {
    pub name: String,
    pub prompt: String,
}

#[derive(Component, Clone)]
pub(super) struct PreparedTask {
    pub role: Option<AppliedRole>,
    pub cwd: Option<PathBuf>,
}

#[derive(SystemParam)]
pub(super) struct RoleContext<'w, 's> {
    root: Res<'w, ProjectRoot>,
    home: Res<'w, RigHome>,
    trust: Option<Res<'w, crate::trust::ProjectTrust>>,
    parents: Query<'w, 's, (&'static ToolAccess, Option<&'static WorkDir>)>,
    tools: Query<'w, 's, &'static ToolDef>,
}

impl RoleContext<'_, '_> {
    pub fn prepare(
        &self,
        parent: Entity,
        request: &mut SpawnSubagent,
    ) -> Result<PreparedTask, String> {
        if request.role == rig_basics::subagents::Role::Task
            && (request.brief.trim().is_empty() || request.title.trim().is_empty())
        {
            return Err("Task description and prompt must not be empty".into());
        }
        let roles = load(
            self.home.0.root(),
            &self.root.0,
            self.trust.as_ref().is_some_and(|trust| trust.trusted),
        );
        let known = self
            .tools
            .iter()
            .map(|tool| tool.0.name.to_string())
            .collect();
        let (access, cwd) = self
            .parents
            .get(parent)
            .map_err(|_| "parent is not an agent")?;
        prepare_one(request, access, cwd, &known, &roles, &self.root.0)
    }
}

#[derive(Clone, Default, Deserialize)]
#[serde(default, deny_unknown_fields)]
struct Frontmatter {
    name: Option<String>,
    description: Option<String>,
    model: Option<String>,
    effort: Option<String>,
    tools: Option<Tools>,
}

#[derive(Clone, Deserialize)]
#[serde(untagged)]
enum Tools {
    List(Vec<String>),
    Comma(String),
}
impl Tools {
    fn names(&self) -> Vec<String> {
        match self {
            Self::List(names) => names.clone(),
            Self::Comma(text) => text
                .split(',')
                .map(str::trim)
                .filter(|s| !s.is_empty())
                .map(str::to_owned)
                .collect(),
        }
    }
}

#[derive(Clone)]
struct Role {
    fields: Frontmatter,
    prompt: String,
}

fn builtin() -> BTreeMap<String, Result<Role, String>> {
    [
        ("scout", "Investigate code and return concise, sourced findings. Do not edit files.", Some(vec!["read", "search", "list"])),
        ("planner", "Inspect the task and return a concrete implementation plan. Do not edit files.", Some(vec!["read", "search", "list"])),
        ("reviewer", "Review the change for correctness. Report concrete findings with locations. You may run checks; shell commands are not a read-only sandbox.", Some(vec!["read", "search", "list", "shell"])),
        ("worker", "Complete the delegated task, verify relevant behavior, and report changes, results and remaining limits.", None),
    ].into_iter().map(|(name, prompt, tools)| {
        let fields = Frontmatter { description: Some(prompt.into()), tools: tools.map(|names| Tools::List(names.into_iter().map(str::to_owned).collect())), ..Default::default() };
        (name.to_owned(), Ok(Role { fields, prompt: prompt.to_owned() }))
    }).collect()
}

fn parse(name: &str, text: &str) -> Result<Role, String> {
    let (fields, body) =
        crate::templates::split::<Frontmatter>(text.trim_start_matches('\u{feff}'))?;
    if fields.name.as_deref().is_some_and(|given| given != name) {
        return Err("name must match the role file name".into());
    }
    Ok(Role {
        fields,
        prompt: body.trim().to_owned(),
    })
}

fn load(home: &Path, root: &Path, trusted: bool) -> BTreeMap<String, Result<Role, String>> {
    let mut roles = builtin();
    let dirs = [
        Some(home.join("agents")),
        trusted.then(|| root.join(".rig/agents")),
    ];
    for dir in dirs.into_iter().flatten() {
        let Ok(entries) = std::fs::read_dir(&dir) else {
            continue;
        };
        let mut files: Vec<_> = entries
            .flatten()
            .map(|entry| entry.path())
            .filter(|file| file.extension().is_some_and(|ext| ext == "md") && file.is_file())
            .collect();
        files.sort();
        for file in files {
            let Some(name) = file.file_stem().and_then(|stem| stem.to_str()) else {
                continue;
            };
            let role = std::fs::read_to_string(&file)
                .map_err(|error| error.to_string())
                .and_then(|text| parse(name, &text))
                .map_err(|why| format!("Role {name} is invalid ({}): {why}", file.display()));
            roles.insert(name.to_owned(), role);
        }
    }
    roles
}

pub(super) fn prepare(
    mut requests: Query<
        (Entity, &mut SpawnSubagent, &ForAgent),
        (Added<Admitted>, Without<PreparedTask>),
    >,
    context: RoleContext,
    mut commands: Commands,
) {
    for (entity, mut request, &ForAgent(parent)) in &mut requests {
        let prepared = context.prepare(parent, &mut request);
        match prepared {
            Ok(prepared) => {
                commands.entity(entity).insert(prepared);
            }
            Err(why) => {
                commands.entity(entity).insert(Outcome::refused(why));
            }
        }
    }
}

fn prepare_one(
    request: &mut SpawnSubagent,
    parent: &ToolAccess,
    parent_cwd: Option<&WorkDir>,
    known: &HashSet<String>,
    roles: &BTreeMap<String, Result<Role, String>>,
    root: &Path,
) -> Result<PreparedTask, String> {
    let selected = request
        .extra
        .get("profile")
        .map(|v| v.as_str().ok_or("profile must be a string"))
        .transpose()?
        .map(|name| {
            roles
                .get(name)
                .ok_or_else(|| format!("Unknown role {name}; /roles lists available roles"))
                .and_then(|role| role.clone())
                .map(|role| (name.to_owned(), role))
        })
        .transpose()?;
    let allowed = selected
        .as_ref()
        .and_then(|(_, role)| role.fields.tools.as_ref())
        .map(Tools::names);
    let requested = request.tools.clone().or_else(|| allowed.clone());
    if let Some(names) = &requested {
        for name in names {
            if !known.contains(name)
                || !parent.allows(name)
                || allowed.as_ref().is_some_and(|list| !list.contains(name))
            {
                return Err(format!(
                    "Tool {name} is unavailable or outside the parent/role tool selection"
                ));
            }
        }
    }
    request.tools = requested;
    let role = selected.map(|(name, role)| {
        request.model = request.model.take().or(role.fields.model);
        request.effort = request.effort.take().or(role.fields.effort);
        AppliedRole {
            name,
            prompt: role.prompt,
        }
    });
    let cwd = match request.extra.get("cwd") {
        Some(value) => {
            let given = value.as_str().ok_or("cwd must be a string")?;
            let base = parent_cwd.map_or(root, |cwd| cwd.0.as_path());
            let path = base
                .join(given)
                .canonicalize()
                .map_err(|e| format!("Cannot resolve cwd: {e}"))?;
            let project = root
                .canonicalize()
                .map_err(|e| format!("Cannot resolve project: {e}"))?;
            if !path.is_dir() || !path.starts_with(project) {
                return Err("cwd must be a directory inside the current project".into());
            }
            Some(path)
        }
        None => parent_cwd.map(|cwd| cwd.0.clone()),
    };
    Ok(PreparedTask { role, cwd })
}

pub(super) fn show_prompt(
    inserted: On<Insert<AppliedRole>>,
    roles: Query<&AppliedRole>,
    mut commands: Commands,
) {
    if let Ok(role) = roles.get(inserted.entity) {
        commands.spawn((
            PromptSection::new(PromptSection::ORDER_ROLE, "vivy_role", role.prompt.clone()),
            SectionOf(inserted.entity),
        ));
    }
}

pub(super) fn list_roles(
    In(args): In<CommandArgs>,
    (root, home, trust): (
        Res<ProjectRoot>,
        Res<RigHome>,
        Option<Res<crate::trust::ProjectTrust>>,
    ),
    mut notices: MessageWriter<Notice>,
) {
    let roles = load(
        home.0.root(),
        &root.0,
        trust.is_some_and(|trust| trust.trusted),
    );
    let lines: Vec<_> = roles
        .into_iter()
        .map(|(name, role)| match role {
            Ok(role) => format!(
                "{name}: {}",
                role.fields.description.unwrap_or_else(|| role
                    .prompt
                    .lines()
                    .next()
                    .unwrap_or_default()
                    .to_owned())
            ),
            Err(why) => why,
        })
        .collect();
    notices.write(Notice::info(Some(args.agent), lines.join("\n")));
}
