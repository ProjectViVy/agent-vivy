use super::*;
use rig_memory::{DirFiles, Files};
use std::{
    path::PathBuf,
    sync::atomic::{AtomicU64, Ordering},
};

static NEXT: AtomicU64 = AtomicU64::new(0);
struct Scratch(PathBuf);
impl Scratch {
    fn new() -> Self {
        Self(std::env::temp_dir().join(format!(
            "vivy-persona-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        )))
    }
    fn store(&self) -> PersonaStore {
        PersonaStore::new(&self.0)
    }
    fn files(&self) -> DirFiles {
        DirFiles::new(self.0.join(".vivy/laputa"))
    }
}
impl Drop for Scratch {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.0);
    }
}
fn save(store: &PersonaStore, current: &Persona, id: &str, name: &str, text: &str) -> Persona {
    store
        .save_file(
            current.revision,
            &current.last_update_id,
            id,
            name,
            text,
            vec!["human".into()],
        )
        .unwrap()
}
fn write_history(scratch: &Scratch, id: &str, persona: &Persona) {
    scratch
        .files()
        .write_file(
            &format!("revisions/{}.json", digest_text(id)),
            &serde_json::to_vec_pretty(persona).unwrap(),
        )
        .unwrap();
}

#[test]
fn absence_is_a_seed_and_saving_publishes_six_files() {
    let dir = Scratch::new();
    let store = dir.store();
    let seed = store.read().unwrap();
    assert_eq!((seed.persona_id.as_str(), seed.revision), (PERSONA_ID, 0));
    assert!(seed.text().contains("You are Vivy"));
    assert!(seed.text().starts_with("Your mission is"));
    assert!(!dir.0.join(".vivy").exists(), "reading must not write");

    let one = store
        .save_file(0, "", "one", IDENTITY, "IDENTITY_ONE", vec!["human".into()])
        .unwrap();
    assert_eq!(one.revision, 1);
    assert_eq!(one.file(IDENTITY), "IDENTITY_ONE");
    assert_eq!(one.file(MISSION), seed.file(MISSION));
    for name in ASSEMBLY_FILES {
        assert!(store.directory().join(name).is_file(), "{name} is missing");
    }
    assert_eq!(
        store
            .save_file(0, "", "one", IDENTITY, "IDENTITY_ONE", vec!["human".into()])
            .unwrap(),
        one
    );
    assert!(
        store
            .save_file(0, "", "one", IDENTITY, "CHANGED", vec!["human".into()])
            .is_err()
    );
    assert!(
        store
            .save_file(0, "stale", "three", IDENTITY, "X", vec![])
            .is_err()
    );

    let two = save(&store, &one, "two", RELATIONSHIP, "RELATIONSHIP_TWO");
    let restored = store.rollback(two.revision, 1, "rollback").unwrap();
    assert_eq!(restored.revision, 3);
    assert_eq!(restored.file(IDENTITY), "IDENTITY_ONE");
    assert_eq!(restored.file(RELATIONSHIP), seed.file(RELATIONSHIP));
    assert_eq!(store.read().unwrap(), restored);
    assert_eq!(
        store.rollback(restored.revision, 1, "rollback").unwrap(),
        restored
    );
    assert_eq!(
        store.read().unwrap().sources,
        vec!["rollback:1".to_string()]
    );
}

#[test]
fn assembled_text_follows_fixed_order_and_skips_empty_optional_files() {
    let dir = Scratch::new();
    let store = dir.store();
    let mut current = store.read().unwrap();
    for (name, text) in [
        (MISSION, "M"),
        (IDENTITY, "I"),
        (RELATIONSHIP, "R"),
        (REDLINE, "L"),
        (DREAM, "D"),
        (DARK, "K"),
    ] {
        current = save(&store, &current, name, name, text);
    }
    assert_eq!(current.text(), "M\n\nI\n\nR\n\nL\n\nD\n\nK");
    assert_eq!(current.revision, 6);
    let cleared = save(&store, &current, "dream", DREAM, "");
    assert_eq!(cleared.text(), "M\n\nI\n\nR\n\nL\n\nK");
}

#[test]
fn unknown_blank_and_oversized_files_are_refused() {
    let dir = Scratch::new();
    let store = dir.store();
    let one = save(&store, &store.read().unwrap(), "one", IDENTITY, "I");
    let unknown = store.save_file(1, &one.last_update_id, "x", "OTHER.MD", "x", vec![]);
    assert!(unknown.is_err());
    let blank = store.save_file(1, &one.last_update_id, "y", MISSION, " \n ", vec![]);
    assert!(blank.is_err());
    let oversized = store.save_file(
        1,
        &one.last_update_id,
        "z",
        IDENTITY,
        &"x".repeat(MAX_FILE_BYTES + 1),
        vec![],
    );
    assert!(oversized.is_err());
    assert_eq!(store.read().unwrap().revision, 1);

    let mut current = one;
    for (i, name) in ASSEMBLY_FILES.iter().enumerate().take(5) {
        let text = if i == 0 {
            format!("M{}", "x".repeat(11 * 1024))
        } else {
            "x".repeat(11 * 1024)
        };
        current = save(&store, &current, name, name, &text);
    }
    let overflow = store.save_file(
        current.revision,
        &current.last_update_id,
        "overflow",
        DARK,
        &"x".repeat(11 * 1024),
        vec![],
    );
    assert!(overflow.is_err());
    assert_eq!(store.read().unwrap().revision, 6);
}

#[test]
fn v1_persona_migrates_into_identity_and_can_be_rolled_back() {
    let dir = Scratch::new();
    let files = dir.files();
    let old = serde_json::json!({
        "schema_version": 1,
        "persona_id": "legacy-hash",
        "revision": 1,
        "text": "LEGACY_IDENTITY",
        "last_update_id": "one",
        "sources": ["human"],
        "parent_update_id": "",
    });
    let bytes = serde_json::to_vec_pretty(&old).unwrap();
    files
        .write_file(&format!("revisions/{}.json", digest_text("one")), &bytes)
        .unwrap();
    files.write_file("persona.json", &bytes).unwrap();

    let store = dir.store();
    let migrated = store.read().unwrap();
    assert_eq!(
        (migrated.persona_id.as_str(), migrated.revision),
        (PERSONA_ID, 1)
    );
    assert_eq!(migrated.file(IDENTITY), "LEGACY_IDENTITY");
    assert!(!migrated.file(MISSION).is_empty());
    assert!(!migrated.unpublished);
    assert_eq!(store.activated("one").unwrap(), Some(migrated.clone()));

    let two = save(&store, &migrated, "two", IDENTITY, "NEW_IDENTITY");
    assert_eq!(two.revision, 2);
    for name in ASSEMBLY_FILES {
        assert!(store.directory().join(name).is_file(), "{name} is missing");
    }
    let back = store.rollback(2, 1, "back").unwrap();
    assert_eq!(back.revision, 3);
    assert_eq!(back.file(IDENTITY), "LEGACY_IDENTITY");
    assert_eq!(back.file(RELATIONSHIP), migrated.file(RELATIONSHIP));
}

#[test]
fn external_edits_are_used_immediately_and_marked_unpublished() {
    let dir = Scratch::new();
    let store = dir.store();
    let one = save(&store, &store.read().unwrap(), "one", IDENTITY, "PUBLISHED");
    assert!(!store.read().unwrap().unpublished);
    std::fs::write(store.directory().join(IDENTITY), "EDITED_OUTSIDE").unwrap();

    let published = store.read().unwrap();
    assert!(published.unpublished);
    assert_eq!(published.file(IDENTITY), "EDITED_OUTSIDE");
    assert_eq!(published.revision, 1);

    let two = save(&store, &published, "two", RELATIONSHIP, "R2");
    assert!(!two.unpublished);
    assert_eq!(two.file(IDENTITY), "EDITED_OUTSIDE");
    assert_eq!(two.revision, 2);
    assert_eq!(one.file(IDENTITY), "PUBLISHED");
}

#[test]
fn checked_saves_refuse_a_file_changed_after_preparation() {
    let dir = Scratch::new();
    let store = dir.store();
    let one = save(&store, &store.read().unwrap(), "one", IDENTITY, "PREPARED");
    let digest = digest_text("PREPARED");
    std::fs::write(store.directory().join(IDENTITY), "MOVED").unwrap();

    let stale = store.save_file_checked(
        one.revision,
        &one.last_update_id,
        &digest,
        "stale",
        IDENTITY,
        "CANDIDATE",
        vec!["dream".into()],
    );
    assert!(stale.is_err());
    assert_eq!(store.read().unwrap().revision, 1);
    let current = store.read().unwrap();
    let saved = store
        .save_file_checked(
            current.revision,
            &current.last_update_id,
            &digest_text("MOVED"),
            "fresh",
            IDENTITY,
            "CANDIDATE",
            vec!["dream".into()],
        )
        .unwrap();
    assert_eq!(saved.revision, 2);
    assert_eq!(saved.file(IDENTITY), "CANDIDATE");
}

#[test]
fn rollback_to_zero_restores_the_default_seed() {
    let dir = Scratch::new();
    let store = dir.store();
    let seed = store.read().unwrap();
    let one = save(&store, &seed, "one", IDENTITY, "CUSTOM");
    let zero = store.rollback(one.revision, 0, "reset").unwrap();
    assert_eq!(zero.revision, 2);
    assert_eq!(zero.file(IDENTITY), seed.file(IDENTITY));
    assert_eq!(zero.text(), seed.text());
    assert_eq!(zero.sources, vec!["rollback:0".to_string()]);
}

#[test]
fn prepared_history_is_reconciled_after_a_failed_current_publication() {
    let dir = Scratch::new();
    let store = dir.store();
    let one = save(&store, &store.read().unwrap(), "one", IDENTITY, "OLD");
    let mut two = one.clone();
    two.revision = 2;
    two.last_update_id = "two".into();
    two.parent_update_id = one.last_update_id.clone();
    two.sources = vec![];
    two.files.insert(IDENTITY.into(), "NEW".into());
    two.sources_digest = digest_files(&two.files);
    write_history(&dir, "two", &two);
    assert_eq!(store.read().unwrap(), one);
    assert!(
        store
            .save_file(1, &one.last_update_id, "two", IDENTITY, "DIFFERENT", vec![])
            .is_err()
    );
    assert_eq!(
        store
            .save_file(1, &one.last_update_id, "two", IDENTITY, "NEW", vec![])
            .unwrap(),
        two
    );
    assert_eq!(store.read().unwrap(), two);
}

#[test]
fn current_metadata_must_match_its_immutable_revision() {
    let dir = Scratch::new();
    let store = dir.store();
    save(&store, &store.read().unwrap(), "one", IDENTITY, "COMMITTED");
    dir.files()
        .write_file("persona.json.part", b"{broken")
        .unwrap();
    assert_eq!(store.read().unwrap().revision, 1);
    dir.files().write_file("persona.json", b"{broken").unwrap();
    assert!(store.read().is_err());
    assert!(
        store
            .save_file(1, "one", "two", IDENTITY, "REPLACEMENT", vec![])
            .is_err()
    );
    assert_eq!(dir.files().read_file("persona.json").unwrap(), b"{broken");
}

#[test]
fn concurrent_updates_have_exactly_one_winner() {
    let dir = Scratch::new();
    std::fs::create_dir_all(&dir.0).unwrap();
    let barrier = std::sync::Arc::new(std::sync::Barrier::new(2));
    let handles: Vec<_> = (0..2)
        .map(|n| {
            let root = dir.0.clone();
            let b = barrier.clone();
            std::thread::spawn(move || {
                b.wait();
                PersonaStore::new(&root).save_file(
                    0,
                    "",
                    &n.to_string(),
                    IDENTITY,
                    &n.to_string(),
                    vec![],
                )
            })
        })
        .collect();
    assert_eq!(
        handles
            .into_iter()
            .filter_map(|h| h.join().unwrap().ok())
            .count(),
        1
    );
    assert_eq!(PersonaStore::new(&dir.0).read().unwrap().revision, 1);
}

#[test]
fn composed_request_has_managed_persona_before_custom_role_and_new_operations_reload_it() {
    use rig_core::test_utils::MockCompletionModel;
    use rig_harness_test_support::{MemoryStore, app_on, connect, first_agent, reply, run_until};
    let dir = Scratch::new();
    let store = dir.store();
    let one = save(
        &store,
        &store.read().unwrap(),
        "one",
        MISSION,
        "PERSONA_FIRST",
    );
    let model = MockCompletionModel::from_stream_turns([reply("one"), reply("two")]);
    let (mut app, wakes) = app_on(&MemoryStore::default());
    app.insert_resource(PersonaSource(dir.0.clone()))
        .add_plugins((AgentPlugin, SessionLogPlugin, LaputaPlugin));
    app.finish();
    app.update();
    let agent = first_agent(app.world_mut()).unwrap();
    app.world_mut()
        .entity_mut(agent)
        .insert((connect(&model).unwrap(), SystemPrompt("CUSTOM_ROLE".into())));
    for (i, text) in ["FIRST_INPUT", "SECOND_INPUT"].into_iter().enumerate() {
        if i == 1 {
            save(&store, &one, "two", MISSION, "PERSONA_SECOND");
        }
        app.world_mut()
            .trigger(Deliver::new(agent, text, DeliveryMode::Queue));
        run_until(&mut app, &wakes, |w| {
            !w.entity(agent).contains::<ActiveTurn>() && model.request_count() == i + 1
        });
    }
    let requests = model.requests();
    let first = format!("{:?}", requests[0].chat_history);
    let second = format!("{:?}", requests[1].chat_history);
    assert!(first.find("PERSONA_FIRST").unwrap() < first.find("CUSTOM_ROLE").unwrap());
    assert!(second.contains("PERSONA_SECOND"));
    assert!(!second.contains("PERSONA_FIRST"));
    assert!(second.contains("<persona>"));
}
