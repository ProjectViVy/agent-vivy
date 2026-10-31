//! One catalog and Connector for every configured instance.
#[cfg(any(feature = "openai", feature = "anthropic"))]
use super::config::Protocol;
use super::config::{Credential, Instance, ProvidersConfig};
use rig_harness::prelude::*;
use rig_harness::rig_core::{
    catalog::{Catalog, Connector, ModelFacts, ModelSpec, Transport},
    providers::registry::{ConnectError, ProviderConfig, ProviderId},
};
use rig_harness::services::RigHome;
use std::{collections::BTreeSet, ffi::OsString};

#[derive(Resource, Clone, Debug, Default)]
pub struct ProvidersState {
    pub config: ProvidersConfig,
    pub issue: Option<String>,
    pub generation: u64,
}

#[derive(Component)]
struct PendingReconnect;
#[derive(Resource)]
struct BootstrapPending(bool);

#[derive(Default)]
pub struct ProvidersPlugin;
impl Plugin for ProvidersPlugin {
    fn build(&self, app: &mut App) {
        app.add_setting::<ProvidersConfig>("providers");
        app.add_setting::<super::GatewaySettings>("gateway");
        let settings = app.world().resource::<Settings>();
        let mut config = app.world().resource::<ProvidersConfig>().clone();
        config.add_legacy(app.world().resource::<super::GatewaySettings>());
        let bootstrap = std::env::var("VIVY_BOOTSTRAP_GATEWAY_URL")
            .ok()
            .zip(std::env::var("VIVY_BOOTSTRAP_GATEWAY_MODEL").ok())
            .is_some_and(|(url, model)| {
                config.bootstrap(
                    &url,
                    &model,
                    std::env::var("VIVY_BOOTSTRAP_GATEWAY_DIALECT").ok(),
                )
            });
        let issue = settings
            .get::<ProvidersConfig>("providers")
            .err()
            .or_else(|| settings.get::<super::GatewaySettings>("gateway").err())
            .or_else(|| {
                settings
                    .problems()
                    .iter()
                    .find(|s| s.contains("not TOML") || s.contains("could not read"))
                    .cloned()
            })
            .or_else(|| config.validate().err());
        let config = if issue.is_some() {
            ProvidersConfig::default()
        } else {
            config
        };
        let connector = connector(&config, app.world());
        app.insert_resource(Models(connector));
        app.insert_resource(ProvidersState {
            config,
            issue,
            ..Default::default()
        });
        let path = RigHome::of(app.world()).settings();
        let snapshot = super::store::read(&path).unwrap_or_default();
        app.insert_resource(super::store::SettingsFile { path, snapshot });
        app.insert_resource(BootstrapPending(bootstrap));
        protect_credentials(app.world_mut());
        super::setup::install(app);
        app.add_systems(PostStartup, (persist_bootstrap, defaults).chain())
            .add_systems(PostUpdate, reconnect);
        #[cfg(feature = "tui")]
        super::ui::add(app);
    }
}

fn persist_bootstrap(world: &mut World) {
    if !world.resource::<BootstrapPending>().0 || world.resource::<ProvidersState>().issue.is_some()
    {
        return;
    }
    world.resource_mut::<BootstrapPending>().0 = false;
    let config = world.resource::<ProvidersState>().config.clone();
    if let Err(why) = world
        .resource_mut::<super::store::SettingsFile>()
        .save(&config)
    {
        world.resource_mut::<ProvidersState>().issue = Some(why);
    }
}

fn protect_credentials(world: &mut World) {
    let names: Vec<OsString> = world
        .resource::<ProvidersState>()
        .config
        .instances
        .values()
        .filter_map(|instance| match &instance.credential {
            Credential::Env { name } => Some(name.into()),
            _ => None,
        })
        .collect();
    vivy_process::register_credential_variables(&names);
    let processes = world.get_resource_or_init::<Processes>();
    processes.set(processes.launcher().without_env(names));
}

pub(crate) fn apply_config(world: &mut World, config: ProvidersConfig, force: &[String]) {
    let previous = &world.resource::<ProvidersState>().config;
    let affected: BTreeSet<String> = previous
        .instances
        .keys()
        .chain(config.instances.keys())
        .filter(|id| previous.instances.get(*id) != config.instances.get(*id))
        .cloned()
        .chain(force.iter().cloned())
        .collect();
    let connector = connector(&config, world);
    world.insert_resource(Models(connector));
    {
        let mut state = world.resource_mut::<ProvidersState>();
        state.config = config;
        state.issue = None;
        state.generation += 1;
    }
    protect_credentials(world);
    let targets: Vec<Entity> = world
        .query::<(Entity, &ModelChoice)>()
        .iter(world)
        .filter(|(_, choice)| {
            choice
                .0
                .split_once('/')
                .is_some_and(|(id, _)| affected.contains(id))
        })
        .map(|(entity, _)| entity)
        .collect();
    for entity in targets {
        world.entity_mut(entity).insert(PendingReconnect);
    }
    reconnect(world);
}

fn reconnect(world: &mut World) {
    let targets: Vec<_> = world
        .query_filtered::<(Entity, &ModelChoice), (With<PendingReconnect>, Without<ActiveTurn>)>()
        .iter(world)
        .map(|(e, m)| (e, m.0.clone()))
        .collect();
    for (entity, model) in targets {
        let connection = world.resource::<Models>().connect(&model);
        world.entity_mut(entity).remove::<PendingReconnect>();
        match connection {
            Ok(connection) => {
                world.entity_mut(entity).insert(connection);
            }
            Err(why) => {
                world.entity_mut(entity).remove::<Connection>();
                if let Some(mut notices) = world.get_resource_mut::<Messages<Notice>>() {
                    notices.write(Notice::error(entity, why));
                }
            }
        }
    }
}

pub fn reload(world: &mut World) -> Result<(), String> {
    let path = world.resource::<super::store::SettingsFile>().path.clone();
    let result = super::store::read(&path)
        .and_then(|text| super::config::parse(&text).map(|config| (text, config)));
    match result {
        Ok((text, config)) => {
            let changed = text != world.resource::<super::store::SettingsFile>().snapshot
                || world.resource::<ProvidersState>().issue.is_some();
            world.resource_mut::<super::store::SettingsFile>().snapshot = text;
            let stored: Vec<String> = config
                .instances
                .iter()
                .filter(|(_, instance)| instance.credential == Credential::Stored)
                .map(|(id, _)| id.clone())
                .collect();
            if changed || !stored.is_empty() {
                apply_config(world, config, &stored);
            }
            Ok(())
        }
        Err(why) => {
            world.resource_mut::<ProvidersState>().issue = Some(why.clone());
            Err(why)
        }
    }
}

pub fn save_config(world: &mut World, config: ProvidersConfig) -> Result<(), String> {
    world
        .resource_mut::<super::store::SettingsFile>()
        .save(&config)?;
    apply_config(world, config, &[]);
    Ok(())
}

pub fn save_key(world: &mut World, id: &str, key: &str) -> Result<(), String> {
    if !world
        .resource::<ProvidersState>()
        .config
        .instances
        .contains_key(id)
    {
        return Err(format!("unknown provider `{id}`"));
    }
    let mut config = world.resource::<ProvidersState>().config.clone();
    config.instances.get_mut(id).unwrap().credential = Credential::Stored;
    let home = RigHome::of(world);
    world
        .resource_mut::<super::store::SettingsFile>()
        .save_with_key(&config, Some((&home, id, Some(key))))?;
    apply_config(world, config, &[id.into()]);
    Ok(())
}

pub fn remove_key(world: &mut World, id: &str) -> Result<(), String> {
    if !world
        .resource::<ProvidersState>()
        .config
        .instances
        .contains_key(id)
    {
        return Err(format!("unknown provider `{id}`"));
    }
    let config = world.resource::<ProvidersState>().config.clone();
    let home = RigHome::of(world);
    world
        .resource_mut::<super::store::SettingsFile>()
        .save_with_key(&config, Some((&home, id, None)))?;
    apply_config(world, config, &[id.into()]);
    Ok(())
}

pub(crate) fn remove_provider(world: &mut World, id: &str) -> Result<(), String> {
    let mut config = world.resource::<ProvidersState>().config.clone();
    if config.instances.remove(id).is_none() {
        return Err(format!("unknown provider `{id}`"));
    }
    let home = RigHome::of(world);
    world
        .resource_mut::<super::store::SettingsFile>()
        .save_with_key(&config, Some((&home, id, None)))?;
    apply_config(world, config, &[]);
    Ok(())
}

pub(crate) fn rows(id: &str, instance: &Instance) -> Vec<ModelSpec> {
    let vendor = ProviderId::declare(id).expect("validated instance ID");
    let preset = instance.preset_id().ok().flatten();
    let mut rows: Vec<ModelSpec> = Catalog::builtin()
        .iter()
        .filter(|row| Some(row.provider) == preset)
        .cloned()
        .map(|mut row| {
            row.provider = vendor;
            row
        })
        .collect();
    for model in &instance.models {
        if let Some(row) = rows.iter_mut().find(|row| row.id == model.id) {
            model.apply(row);
        } else {
            let mut row = ModelSpec::new(vendor, &model.id).with_tools(true);
            model.apply(&mut row);
            rows.push(row);
        }
    }
    rows.retain(|row| !instance.disabled_models.contains(&row.id));
    rows
}

pub(crate) fn connector(config: &ProvidersConfig, world: &World) -> Connector {
    let mut connector = Connector::new(Catalog::default());
    let home = RigHome::of(world);
    for (id, instance) in &config.instances {
        let vendor = ProviderId::declare(id).expect("validated instance ID");
        for row in rows(id, instance) {
            connector.catalog_mut().insert(row);
        }
        let key = match &instance.credential {
            Credential::None => Ok(String::new()),
            Credential::Env { name } => world
                .get_resource::<Secrets>()
                .and_then(|s| {
                    name.strip_prefix("RIG_SECRET_")
                        .and_then(|n| s.get(&n.to_ascii_lowercase()))
                })
                .map(str::to_owned)
                .or_else(|| std::env::var(name).ok())
                .filter(|key| !key.is_empty())
                .ok_or_else(|| name.clone()),
            Credential::Stored => {
                let path = home.auth().join("provider-keys").join(format!("{id}.json"));
                match std::fs::read(&path) {
                    Ok(data) => serde_json::from_slice::<SavedKey>(&data)
                        .map(|s| s.key)
                        .map_err(|e| format!("saved key for {id}: {e}")),
                    Err(e) if e.kind() == std::io::ErrorKind::NotFound => {
                        Err(format!("/login {id}"))
                    }
                    Err(e) => Err(format!("saved key for {id}: {e}")),
                }
            }
        };
        let id = id.clone();
        let instance = instance.clone();
        connector.provide(
            vendor,
            move |spec: &ModelSpec, _: &Catalog, transport: &Transport| {
                let key = key.as_ref().map_err(|name| ConnectError::MissingKey {
                    vendor: id.clone(),
                    tried: vec![name.clone().into()],
                })?;
                let config = wire_config(&instance, key).map_err(ConnectError::Provider)?;
                #[cfg(feature = "anthropic")]
                let config = match (config, spec.max_output_tokens) {
                    (ProviderConfig::Anthropic(mut config), Some(cap)) => {
                        config.dialect.quirks.max_tokens =
                            rig_harness::rig_core::providers::anthropic::wire::MaxTokens::Fixed(
                                cap.into(),
                            );
                        ProviderConfig::Anthropic(config)
                    }
                    (config, _) => config,
                };
                let http = super::redact::RedactingClient::wrap(
                    transport.shared().ok_or(ConnectError::NoHttpClient)?,
                    key,
                );
                let facts = wire_facts(spec, &config);
                Ok(config.completion_handler(&spec.reference(), &spec.id, http, facts))
            },
        );
    }
    connector
}

#[derive(serde::Deserialize, serde::Serialize)]
pub(crate) struct SavedKey {
    pub key: String,
}

pub(crate) fn wire_config(instance: &Instance, key: &str) -> Result<ProviderConfig, String> {
    let preset = instance.preset_id()?;
    #[cfg(not(any(feature = "openai", feature = "anthropic")))]
    let _ = (key, preset);
    match instance.protocol {
        #[cfg(feature = "openai")]
        Protocol::Openai => {
            use rig_harness::rig_core::providers::openai::{OpenAIConfig, Route, wire};
            static GENERIC: wire::Dialect = wire::Dialect::gateway("compatible", "", "");
            let mut config = if let Some(preset) = preset {
                preset.config(key).unwrap()
            } else {
                ProviderConfig::OpenAi(OpenAIConfig::with_key(&GENERIC, key))
            };
            if let Some(base) = &instance.base_url {
                config = config.with_base_url(base.trim());
            }
            if let ProviderConfig::OpenAi(mut config) = config {
                config = config.with_route(Route::Chat);
                if instance.credential == Credential::None {
                    config = config.with_auth(wire::Auth::OptionalBearer);
                }
                Ok(ProviderConfig::OpenAi(config))
            } else {
                Err("not an OpenAI-format preset".into())
            }
        }
        #[cfg(feature = "anthropic")]
        Protocol::Anthropic => {
            use rig_harness::rig_core::providers::anthropic::wire;
            static GENERIC: wire::Dialect = wire::compatible("compatible", "", "", None);
            let mut config = if let Some(preset) = preset {
                preset.config(key).unwrap()
            } else {
                ProviderConfig::Anthropic(wire::AnthropicConfig::with_key(&GENERIC, key))
            };
            if let Some(base) = &instance.base_url {
                config = config.with_base_url(base.trim());
            }
            if let ProviderConfig::Anthropic(mut config) = config {
                if instance.credential == Credential::None {
                    config.dialect.quirks.auth = wire::Auth::OptionalBearer;
                }
                Ok(ProviderConfig::Anthropic(config))
            } else {
                Err("not an Anthropic-format preset".into())
            }
        }
        #[allow(unreachable_patterns)]
        _ => Err(format!("{} was not compiled", instance.protocol.name())),
    }
}

fn wire_facts(spec: &ModelSpec, config: &ProviderConfig) -> ModelFacts {
    let mut bound = spec.clone();
    bound.provider = config.id().unwrap_or_else(|| {
        ProviderId::declare(config.vendor()).expect("static compatible dialect name")
    });
    ModelFacts::new(bound)
}

#[cfg(all(test, feature = "anthropic"))]
mod fact_tests {
    use super::*;
    #[test]
    fn a_native_instance_name_does_not_override_the_actual_wire_identity() {
        let spec = ModelSpec::new(ProviderId::declare("openai").unwrap(), "fixture-model")
            .with_context_window(1234);
        let mut instance = Instance::generic(Protocol::Anthropic);
        instance.preset = Some("anthropic".into());
        let wire = wire_config(&instance, "fixture").unwrap();
        let facts = wire_facts(&spec, &wire);
        assert_eq!(
            facts
                .for_model("anthropic", "fixture-model")
                .unwrap()
                .context_window,
            Some(1234)
        );
        assert_eq!(spec.reference(), "openai/fixture-model");
    }
}

fn defaults(
    state: Res<ProvidersState>,
    invoked: Option<Res<Invoked>>,
    agents: Query<Entity, (With<Agent>, Without<ModelChoice>, Without<SpawnedBy>)>,
    mut commands: Commands,
) {
    if invoked.is_some_and(|v| v.args.model.is_some()) {
        return;
    }
    let Some(model) = &state.config.default_model else {
        return;
    };
    for agent in &agents {
        commands.entity(agent).insert(ModelChoice(model.clone()));
        if let Some(level) = &state.config.default_thinking {
            commands.queue({
                let level = level.clone();
                let model = model.clone();
                move |world: &mut World| {
                    let reasoning = world
                        .resource::<Models>()
                        .0
                        .catalog()
                        .resolve(&model)
                        .ok()
                        .and_then(|row| row.spec.reasoning.named(&level).ok())
                        .map(|r| ReasoningSetting(r.reasoning));
                    if let Some(reasoning) = reasoning {
                        world.entity_mut(agent).insert(reasoning);
                    }
                }
            });
        }
    }
}
