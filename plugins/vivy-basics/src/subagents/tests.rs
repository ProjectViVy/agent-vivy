use super::*;
use rig_core::catalog::{Catalog, Connector};
use rig_core::test_utils::MockCompletionModel;
use rig_harness_test_support::{MemoryStore, app_on, connect, reply, start_call};
use serde_json::json;
use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};

static NEXT: AtomicU64 = AtomicU64::new(0);

pub(super) struct Fixture {
    base: PathBuf,
    pub home: PathBuf,
    pub project: PathBuf,
}
impl Fixture {
    pub(super) fn new() -> Self {
        let base = std::env::temp_dir().join(format!(
            "vivy-subagents-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        let home = base.join("home");
        let project = base.join("project");
        std::fs::create_dir_all(&home).unwrap();
        std::fs::create_dir_all(&project).unwrap();
        Self {
            base,
            home,
            project,
        }
    }
    pub(super) fn role(&self, project: bool, text: &str) {
        let dir = if project {
            self.project.join(".rig/agents")
        } else {
            self.home.join("agents")
        };
        std::fs::create_dir_all(&dir).unwrap();
        std::fs::write(dir.join("analyst.md"), text).unwrap();
    }
}
impl Drop for Fixture {
    fn drop(&mut self) {
        assert!(self.base.starts_with(std::env::temp_dir()));
        assert!(
            self.base
                .file_name()
                .unwrap()
                .to_string_lossy()
                .starts_with("vivy-subagents-")
        );
        std::fs::remove_dir_all(&self.base).unwrap();
    }
}

pub(super) fn setup(fixture: &Fixture, trusted: bool) -> (App, Entity) {
    setup_on(fixture, trusted, &MemoryStore::default())
}

pub(super) fn setup_on(fixture: &Fixture, trusted: bool, store: &MemoryStore) -> (App, Entity) {
    let (mut app, _) = app_on(store);
    let model = MockCompletionModel::from_stream_turns([reply("done")]);
    let connection = connect(&model).unwrap();
    let choice = ModelChoice(connection.spec.reference());
    app.insert_resource(RigHome(Home::at(fixture.home.clone())))
        .insert_resource(ProjectRoot(fixture.project.clone()))
        .insert_resource(Models(Connector::new(Catalog::default())))
        .insert_resource(crate::trust::ProjectTrust {
            trusted,
            undecided: false,
            protected: vec![fixture.project.join(".rig/agents")],
        })
        .add_plugins((AgentPlugin, SessionLogPlugin, rig_harness::ask::AskPlugin));
    for name in ["read", "write", "shell", "python_repl"] {
        app.add_open_tool(
            name,
            "Test tool.",
            ToolOptions::default(),
            |_: On<ToolCalled<serde_json::Value>>| {},
        );
    }
    rig_harness::load::<VivySubagentsPlugin>(&mut app, "vivy-basics", "test");
    let parent = app.world_mut().spawn((Agent, connection, choice)).id();
    app.finish();
    app.update();
    (app, parent)
}

fn task(app: &mut App, parent: Entity, args: serde_json::Value) -> (Option<Entity>, String) {
    let before: Vec<_> = app
        .world()
        .get::<Spawned>(parent)
        .map(|s| s.iter().collect())
        .unwrap_or_default();
    let id = format!("test-task-{}", NEXT.fetch_add(1, Ordering::Relaxed));
    let call = start_call(app.world_mut(), parent, ("task", &id), args).unwrap();
    for _ in 0..12 {
        app.world_mut().run_schedule(AgentPass);
    }
    let child = app
        .world()
        .get::<Spawned>(parent)
        .and_then(|s| s.iter().find(|e| !before.contains(e)));
    let result = app
        .world()
        .get::<Outcome>(call)
        .map(|o| o.tool_result().output().render())
        .unwrap_or_default();
    (child, result)
}

fn role_text(app: &mut App, child: Entity) -> String {
    let world = app.world_mut();
    world
        .query::<(&PromptSection, &SectionOf)>()
        .iter(world)
        .filter(|(_, of)| of.0 == child)
        .map(|(section, _)| section.text.clone())
        .collect::<Vec<_>>()
        .join("\n")
}

#[test]
fn selected_component_reuses_native_delegation_tools() {
    let (mut app, _) = app_on(&MemoryStore::default());
    app.add_plugins((AgentPlugin, SessionLogPlugin, VivySubagentsPlugin));
    app.finish();
    app.update();
    let world = app.world_mut();
    let names: Vec<_> = world
        .query::<&ToolDef>()
        .iter(world)
        .map(|definition| definition.0.name.to_string())
        .collect();
    for name in ["task", "message", "wait"] {
        assert!(
            names.iter().any(|registered| registered == name),
            "missing {name}: {names:?}"
        );
    }
}

#[test]
fn project_roles_are_protected_by_existing_trust() {
    let root = std::env::temp_dir().join(format!("vivy-roles-trust-{}", std::process::id()));
    let agents = root.join(".rig/agents");
    std::fs::create_dir_all(&agents).unwrap();
    let found = crate::trust::protected(&root);
    std::fs::remove_dir(&agents).unwrap();
    std::fs::remove_dir(root.join(".rig")).unwrap();
    std::fs::remove_dir(&root).unwrap();
    assert!(
        found.contains(&agents),
        "project agents must participate in the existing trust decision"
    );
}

#[test]
fn role_narrows_tools_and_freezes_its_prompt() {
    let fixture = Fixture::new();
    fixture.role(
        false,
        "---\nname: analyst\ndescription: Inspect code\ntools: read\n---\nUSER ROLE A",
    );
    let (mut app, parent) = setup(&fixture, false);
    let (child, output) = task(
        &mut app,
        parent,
        json!({"description":"Inspect code","prompt":"Investigate.","profile":"analyst"}),
    );
    let child = child.unwrap_or_else(|| panic!("no child: {output}"));
    let access = app.world().get::<ToolAccess>(child).unwrap();
    assert!(access.allows("read"));
    assert!(!access.allows("write") && !access.allows("python_repl"));
    assert!(role_text(&mut app, child).contains("USER ROLE A"));
    fixture.role(false, "---\ntools: [write]\n---\nUSER ROLE B");
    assert!(role_text(&mut app, child).contains("USER ROLE A"));
    let (next, output) = task(
        &mut app,
        parent,
        json!({"description":"Inspect again","prompt":"Investigate.","profile":"analyst"}),
    );
    assert!(
        role_text(
            &mut app,
            next.unwrap_or_else(|| panic!("no child: {output}"))
        )
        .contains("USER ROLE B")
    );
}

#[test]
fn project_role_obeys_trust_and_invalid_override_does_not_fall_back() {
    let fixture = Fixture::new();
    fixture.role(false, "---\ntools: [read]\n---\nHOME ROLE");
    fixture.role(true, "---\ntools: [write]\n---\nPROJECT ROLE");
    let (mut app, parent) = setup(&fixture, false);
    let (child, output) = task(
        &mut app,
        parent,
        json!({"description":"Inspect","prompt":"Work.","profile":"analyst"}),
    );
    assert!(
        role_text(
            &mut app,
            child.unwrap_or_else(|| panic!("no child: {output}"))
        )
        .contains("HOME ROLE")
    );
    app.world_mut()
        .resource_mut::<crate::trust::ProjectTrust>()
        .trusted = true;
    let (child, output) = task(
        &mut app,
        parent,
        json!({"description":"Inspect","prompt":"Work.","profile":"analyst"}),
    );
    assert!(
        role_text(
            &mut app,
            child.unwrap_or_else(|| panic!("no child: {output}"))
        )
        .contains("PROJECT ROLE")
    );
    fixture.role(true, "---\ntools: 123\n---\nINVALID");
    let (child, output) = task(
        &mut app,
        parent,
        json!({"description":"Inspect","prompt":"Work.","profile":"analyst"}),
    );
    assert!(child.is_none(), "invalid override must not widen access");
    assert!(
        output.contains("analyst") && output.contains("invalid"),
        "{output}"
    );
}

#[test]
fn cwd_is_local_to_child_and_rejects_project_escape() {
    let fixture = Fixture::new();
    let subdir = fixture.project.join("nested");
    std::fs::create_dir(&subdir).unwrap();
    let (mut app, parent) = setup(&fixture, false);
    let (child, output) = task(
        &mut app,
        parent,
        json!({"description":"Inspect nested","prompt":"Work.","cwd":"nested"}),
    );
    let child = child.unwrap_or_else(|| panic!("no child: {output}"));
    assert_eq!(
        app.world().get::<WorkDir>(child).unwrap().0,
        subdir.canonicalize().unwrap()
    );
    assert!(app.world().get::<WorkDir>(parent).is_none());
    let (child, output) = task(
        &mut app,
        parent,
        json!({"description":"Escape","prompt":"Work.","cwd":".."}),
    );
    assert!(child.is_none());
    assert!(output.contains("project"), "{output}");
}

#[test]
fn default_depth_is_one_and_explicit_native_setting_is_preserved() {
    let fixture = Fixture::new();
    let (app, _) = setup(&fixture, false);
    assert_eq!(
        app.world()
            .resource::<rig_basics::subagents::SubagentPolicy>()
            .max_depth,
        1
    );
    let (mut app, _) = app_on(&MemoryStore::default());
    app.insert_resource(Settings::from_toml("[subagents]\nmax_depth = 3"))
        .add_plugins(VivySubagentsPlugin);
    assert_eq!(
        app.world()
            .resource::<rig_basics::subagents::SubagentPolicy>()
            .max_depth,
        3
    );
}

#[test]
fn child_at_the_depth_limit_does_not_advertise_batch_delegation() {
    let fixture = Fixture::new();
    let (mut app, parent) = setup(&fixture, false);
    let (child, output) = task(
        &mut app,
        parent,
        json!({"description":"Work","prompt":"Inspect."}),
    );
    let access = app
        .world()
        .get::<ToolAccess>(child.unwrap_or_else(|| panic!("{output}")))
        .unwrap();
    assert!(
        !access.allows("task") && !access.allows("task_parallel") && !access.allows("task_chain")
    );
}

#[test]
fn plugin_stages_have_explicit_order_with_native_commands() {
    let fixture = Fixture::new();
    let (mut app, _) = setup(&fixture, false);
    let conflicts = rig_harness::stage_conflicts(app.world_mut());
    assert!(conflicts.is_empty(), "{}", conflicts.join("\n"));
}

#[test]
fn roles_are_available_through_the_native_command_runner() {
    let fixture = Fixture::new();
    let (mut app, parent) = setup(&fixture, false);
    app.world_mut().trigger(RunCommand::new(parent, "roles"));
    app.update();
    let messages = app.world().resource::<Messages<Notice>>();
    let text = messages
        .iter_current_update_messages()
        .map(|notice| format!("{notice:?}"))
        .collect::<String>();
    assert!(
        text.contains("scout:") && text.contains("worker:"),
        "{text}"
    );
}
