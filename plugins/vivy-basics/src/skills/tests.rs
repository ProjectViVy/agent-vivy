use super::*;
use rig_harness_test_support::{MemoryStore, app_on, first_agent};

#[test]
fn publication_refresh_changes_slash_instructions_without_a_stale_closure() {
    let root = std::env::temp_dir().join(format!(
        "vivy-skill-refresh-{}-{}",
        std::process::id(),
        std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .unwrap()
            .as_nanos()
    ));
    std::fs::create_dir_all(&root).unwrap();
    let old = root.join("old.md");
    let new = root.join("new.md");
    std::fs::write(&old, "OLD_INSTRUCTIONS_CANARY").unwrap();
    std::fs::write(&new, "NEW_INSTRUCTIONS_CANARY").unwrap();
    let skill = Skill {
        name: "fixture".into(),
        description: "Fixture".into(),
        file: old,
        manual: false,
    };
    let (mut app, _) = app_on(&MemoryStore::default());
    app.insert_resource(ProjectRoot(root.clone()));
    app.insert_resource(RigHome(rig_harness::harness_protocol::Home::at(
        root.join("home"),
    )));
    app.insert_resource(PublishedSkills(vec![skill.clone()]));
    app.add_plugins((AgentPlugin, SessionLogPlugin, TrustedSkillsPlugin));
    app.finish();
    app.update();
    let agent = first_agent(app.world_mut()).unwrap();
    app.world_mut()
        .insert_resource(PublishedSkills(vec![Skill { file: new, ..skill }]));
    refresh_inventory(app.world_mut());
    app.world_mut()
        .trigger(rig_harness::commands::RunCommand::new(
            agent,
            "skill:fixture",
        ));
    app.world_mut().flush();
    let texts: Vec<_> = app
        .world_mut()
        .query::<&Incoming>()
        .iter(app.world())
        .map(Incoming::text)
        .collect();
    assert!(
        texts.iter().any(|s| s.contains("NEW_INSTRUCTIONS_CANARY")),
        "{texts:?}"
    );
    assert!(!texts.iter().any(|s| s.contains("OLD_INSTRUCTIONS_CANARY")));
    drop(app);
    std::fs::remove_dir_all(root).unwrap();
}
