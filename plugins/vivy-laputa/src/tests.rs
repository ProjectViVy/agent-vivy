use super::*;
use std::sync::atomic::{AtomicU64, Ordering};
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
}
impl Drop for Scratch {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.0);
    }
}

#[test]
fn absence_is_empty_and_cas_preserves_versions_and_idempotency() {
    let dir = Scratch::new();
    let store = PersonaStore::new(&dir.0);
    let empty = store.read().unwrap();
    assert_eq!(empty.revision, 0);
    assert!(empty.text.is_empty());
    assert!(!store.root.exists());
    let one = store
        .update(0, "one", "PERSONA_ONE", vec!["human".into()])
        .unwrap();
    assert_eq!(one.revision, 1);
    assert_eq!(
        store
            .update(0, "one", "PERSONA_ONE", vec!["human".into()])
            .unwrap(),
        one
    );
    assert!(
        store
            .update(0, "one", "CHANGED", vec!["human".into()])
            .is_err()
    );
    assert!(store.update(0, "stale", "bad", vec![]).is_err());
    store.update(1, "two", "PERSONA_TWO", vec![]).unwrap();
    assert_eq!(
        store
            .update(0, "one", "PERSONA_ONE", vec!["human".into()])
            .unwrap(),
        one
    );
    let restored = store.rollback(2, 1, "rollback").unwrap();
    assert_eq!(
        (restored.revision, restored.text.as_str()),
        (3, "PERSONA_ONE")
    );
    assert_eq!(store.read().unwrap(), restored);
    assert_eq!(store.rollback(2, 1, "rollback").unwrap(), restored);
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
                PersonaStore::new(&root).update(0, &n.to_string(), &n.to_string(), vec![])
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
fn incomplete_current_and_unpublished_history_do_not_become_persona() {
    let dir = Scratch::new();
    let store = PersonaStore::new(&dir.0);
    let one = store.update(0, "one", "committed", vec![]).unwrap();
    std::fs::write(store.root.join("persona.json.part"), b"{broken").unwrap();
    assert_eq!(store.read().unwrap(), one);
    std::fs::write(store.root.join("persona.json"), b"{broken").unwrap();
    assert!(store.read().is_err());
    assert!(store.update(1, "two", "replacement", vec![]).is_err());
    assert_eq!(
        std::fs::read(store.root.join("persona.json")).unwrap(),
        b"{broken"
    );
}

#[test]
fn prepared_history_is_reconciled_after_a_failed_current_publication() {
    let dir = Scratch::new();
    let store = PersonaStore::new(&dir.0);
    let one = store.update(0, "one", "OLD", vec![]).unwrap();
    let two = Persona {
        revision: 2,
        last_update_id: "two".into(),
        parent_update_id: "one".into(),
        text: "NEW".into(),
        ..one.clone()
    };
    store
        .files()
        .write_file(
            &history_name("two"),
            &serde_json::to_vec_pretty(&two).unwrap(),
        )
        .unwrap();
    assert_eq!(store.read().unwrap(), one);
    assert!(store.update(1, "two", "DIFFERENT", vec![]).is_err());
    assert_eq!(store.update(1, "two", "NEW", vec![]).unwrap(), two);
    assert_eq!(store.read().unwrap(), two);
}

#[test]
fn composed_request_has_managed_persona_before_custom_role_and_new_operations_reload_it() {
    use rig_core::test_utils::MockCompletionModel;
    use rig_harness_test_support::{MemoryStore, app_on, connect, first_agent, reply, run_until};
    let dir = Scratch::new();
    let store = PersonaStore::new(&dir.0);
    store.update(0, "one", "PERSONA_FIRST", vec![]).unwrap();
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
            store.update(1, "two", "PERSONA_SECOND", vec![]).unwrap();
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
}
