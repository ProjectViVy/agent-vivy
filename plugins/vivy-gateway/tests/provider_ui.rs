#![cfg(all(feature = "tui", feature = "openai", feature = "anthropic"))]
use rig_harness::prelude::*;
use rig_harness::{
    commands::{CommandPlugin, RunCommand},
    main_agent::ShownAgent,
    pick::{Chosen, Pick, PickPlugin, Picker},
    services::RigHome,
};
use vivy_gateway::{ProvidersPlugin, ProvidersState};

fn setup() -> (App, Entity, std::path::PathBuf) {
    use std::sync::atomic::{AtomicU64, Ordering};
    static N: AtomicU64 = AtomicU64::new(0);
    let dir = std::env::temp_dir().join(format!(
        "vivy-provider-ui-{}-{}",
        std::process::id(),
        N.fetch_add(1, Ordering::Relaxed)
    ));
    std::fs::create_dir_all(&dir).unwrap();
    let mut app = App::new();
    app.add_message::<Notice>();
    app.insert_resource(RigHome(rig_harness::harness_protocol::Home::at(&dir)));
    app.insert_resource(Settings::from_toml(""));
    app.insert_resource(ShownAgent::default());
    app.add_plugins((CommandPlugin, PickPlugin, ProvidersPlugin));
    let agent = app.world_mut().spawn(Agent).id();
    app.world_mut().resource_mut::<ShownAgent>().0 = Some(agent);
    (app, agent, dir)
}

fn choose(app: &mut App, label: &str) {
    let (entity, index) = app
        .world_mut()
        .query::<(Entity, &Picker)>()
        .iter(app.world())
        .find_map(|(e, p)| {
            p.items
                .iter()
                .position(|item| item.label.starts_with(label))
                .map(|i| (e, i))
        })
        .unwrap_or_else(|| panic!("missing choice: {label}"));
    app.world_mut()
        .trigger(Pick::person(entity, Some(Chosen::item(index))));
    app.update();
}

fn text(app: &mut App, value: &str) {
    let entity = app
        .world_mut()
        .query::<(Entity, &Picker)>()
        .iter(app.world())
        .next()
        .unwrap()
        .0;
    app.world_mut()
        .trigger(Pick::person(entity, Some(Chosen::text(value))));
    app.update();
}

fn command(app: &mut App, agent: Entity, value: &str) {
    app.world_mut().trigger(RunCommand::new(agent, value));
    app.update();
}

#[test]
fn add_cancel_does_not_save_and_login_input_is_hidden() {
    let (mut app, agent, dir) = setup();
    app.world_mut().trigger(RunCommand::new(agent, "provider"));
    app.update();
    choose(&mut app, "Add provider");
    choose(&mut app, "Custom OpenAI");
    assert!(
        app.world()
            .resource::<ProvidersState>()
            .config
            .instances
            .is_empty()
    );
    let picker = app
        .world_mut()
        .query::<(Entity, &Picker)>()
        .iter(app.world())
        .next()
        .unwrap()
        .0;
    app.world_mut().trigger(Pick::person(picker, None));
    app.update();
    assert!(!dir.join("settings.toml").exists());
    std::fs::remove_dir_all(dir).unwrap();
}

#[test]
fn provider_and_model_crud_keys_and_explicit_defaults_persist() {
    let (mut app, agent, dir) = setup();
    command(&mut app, agent, "provider");
    choose(&mut app, "Add provider");
    choose(&mut app, "Custom OpenAI");
    choose(&mut app, "Instance ID:");
    text(&mut app, "company");
    choose(&mut app, "Endpoint:");
    text(&mut app, "http://127.0.0.1:9/v1");
    choose(&mut app, "Models:");
    choose(&mut app, "Add model");
    choose(&mut app, "id:");
    text(&mut app, "org/model-1");
    choose(&mut app, "context_window:");
    text(&mut app, "4000");
    choose(&mut app, "Save model to draft");
    choose(&mut app, "Back to provider");
    choose(&mut app, "Credential:");
    choose(&mut app, "No key required");
    choose(&mut app, "Save only");
    assert!(
        app.world()
            .resource::<Models>()
            .connect("company/org/model-1")
            .is_ok()
    );
    assert!(dir.join("settings.toml").is_file());
    choose(&mut app, "Save API key");
    let picker = app
        .world_mut()
        .query::<&Picker>()
        .iter(app.world())
        .next()
        .unwrap();
    assert!(picker.secret);
    text(&mut app, "fixture-private-key-9482");
    let settings = std::fs::read_to_string(dir.join("settings.toml")).unwrap();
    assert!(!settings.contains("fixture-private-key-9482"));
    assert!(dir.join("auth/provider-keys/company.json").is_file());
    app.world_mut()
        .entity_mut(agent)
        .insert(ModelChoice("company/org/model-1".into()));
    command(&mut app, agent, "model-default");
    assert_eq!(
        app.world()
            .resource::<ProvidersState>()
            .config
            .default_model
            .as_deref(),
        Some("company/org/model-1")
    );
    command(&mut app, agent, "provider company");
    choose(&mut app, "Edit provider and models");
    choose(&mut app, "Name:");
    text(&mut app, "Work account");
    choose(&mut app, "Save only");
    assert_eq!(
        app.world().resource::<ProvidersState>().config.instances["company"]
            .name
            .as_deref(),
        Some("Work account")
    );
    choose(&mut app, "Delete provider");
    choose(&mut app, "Delete provider and saved key");
    assert!(
        app.world()
            .resource::<ProvidersState>()
            .config
            .instances
            .is_empty()
    );
    assert!(!dir.join("auth/provider-keys/company.json").exists());
    assert!(
        app.world()
            .resource::<Models>()
            .connect("company/org/model-1")
            .is_err()
    );
    std::fs::remove_dir_all(dir).unwrap();
}

#[test]
fn restore_removed_models_is_a_draft_action_without_a_text_prompt() {
    let (mut app, agent, dir) = setup();
    let model_id = rig_harness::rig_core::catalog::Catalog::builtin()
        .iter()
        .find(|s| s.provider.vendor() == "deepseek")
        .unwrap()
        .id
        .clone();
    let config=vivy_gateway::config::parse(&format!("[providers.instances.deepseek]\nprotocol='openai'\npreset='deepseek'\ndisabled_models=[{model_id:?}]\ncredential={{source='none'}}\n")).unwrap();
    vivy_gateway::save_config(app.world_mut(), config).unwrap();
    command(&mut app, agent, "provider deepseek");
    choose(&mut app, "Edit provider and models");
    choose(&mut app, "Models:");
    choose(&mut app, "Restore removed preset models");
    assert!(
        app.world_mut()
            .query::<&Picker>()
            .iter(app.world())
            .any(|p| p.title == "Provider models")
    );
    choose(&mut app, "Back to provider");
    choose(&mut app, "Save only");
    assert!(
        app.world()
            .resource::<Models>()
            .0
            .catalog()
            .resolve(&format!("deepseek/{model_id}"))
            .is_ok()
    );
    std::fs::remove_dir_all(dir).unwrap();
}

#[test]
fn saving_a_budget_based_thinking_choice_keeps_its_portable_level_name() {
    use rig_harness::rig_core::catalog::ReasoningSupport;
    let (mut app, agent, dir) = setup();
    let config=vivy_gateway::config::parse("[providers.instances.messages]\nprotocol='anthropic'\nbase_url='http://127.0.0.1:9'\ncredential={source='none'}\n[[providers.instances.messages.models]]\nid='org/model'\n").unwrap();
    vivy_gateway::save_config(app.world_mut(), config).unwrap();
    let mut row = (*app
        .world()
        .resource::<Models>()
        .0
        .catalog()
        .resolve("messages/org/model")
        .unwrap()
        .shared())
    .clone();
    row.reasoning = ReasoningSupport::Listed {
        levels: vec![],
        budget: Some(1024..=16384),
        can_disable: true,
        default: None,
    };
    let thinking = row.reasoning.named("high").unwrap().reasoning;
    app.world_mut()
        .resource_mut::<Models>()
        .0
        .catalog_mut()
        .insert(row);
    app.world_mut().entity_mut(agent).insert((
        ModelChoice("messages/org/model".into()),
        ReasoningSetting(thinking),
    ));
    command(&mut app, agent, "model-default");
    assert_eq!(
        app.world()
            .resource::<ProvidersState>()
            .config
            .default_thinking
            .as_deref(),
        Some("high")
    );
    std::fs::remove_dir_all(dir).unwrap();
}

#[test]
fn presets_with_dotted_native_names_get_a_valid_instance_id() {
    let (mut app, agent, dir) = setup();
    command(&mut app, agent, "provider");
    choose(&mut app, "Add provider");
    choose(
        &mut app,
        &format!(
            "{} · openai",
            rig_harness::rig_core::providers::azure::PROVIDER_NAME
        ),
    );
    let picker = app
        .world_mut()
        .query::<&Picker>()
        .iter(app.world())
        .next()
        .unwrap();
    let label = &picker
        .items
        .iter()
        .find(|item| item.label.starts_with("Instance ID:"))
        .unwrap()
        .label;
    assert!(vivy_gateway::config::valid_id(
        label.strip_prefix("Instance ID:").unwrap().trim()
    ));
    std::fs::remove_dir_all(dir).unwrap();
}

#[test]
fn a_failed_credential_delete_keeps_the_provider_available_for_retry() {
    let (mut app, agent, dir) = setup();
    let config=vivy_gateway::config::parse("[providers.instances.company]\nprotocol='openai'\nbase_url='http://127.0.0.1:9/v1'\ncredential={source='none'}\n[[providers.instances.company.models]]\nid='org/model'\n").unwrap();
    vivy_gateway::save_config(app.world_mut(), config).unwrap();
    std::fs::create_dir_all(dir.join("auth/provider-keys/company.json")).unwrap();
    command(&mut app, agent, "provider company");
    choose(&mut app, "Delete provider");
    choose(&mut app, "Delete provider and saved key");
    assert!(
        app.world()
            .resource::<ProvidersState>()
            .config
            .instances
            .contains_key("company")
    );
    assert!(
        vivy_gateway::config::parse(&std::fs::read_to_string(dir.join("settings.toml")).unwrap())
            .unwrap()
            .instances
            .contains_key("company")
    );
    std::fs::remove_dir_all(dir).unwrap();
}
