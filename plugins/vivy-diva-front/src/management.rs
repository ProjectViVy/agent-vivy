use crate::protocol::*;
use rig_harness::prelude::*;

pub fn providers(world: &World) -> Reply {
    let state = world.resource::<vivy_gateway::ProvidersState>();
    Reply::Providers(ProvidersView {
        presets: vivy_gateway::desktop_presets::presets(),
        config: state.config.clone(),
        generation: state.generation,
        issue: state.issue.clone(),
        protocols: [
            vivy_gateway::config::Protocol::Openai,
            vivy_gateway::config::Protocol::Anthropic,
        ]
        .into_iter()
        .filter(|p| p.compiled())
        .map(|p| p.name().into())
        .collect(),
    })
}
fn fault(message: String) -> Fault {
    Fault {
        code: ErrorCode::Conflict,
        message: rig_harness::rig_core::redact::text(&message).into_owned(),
    }
}
fn check(world: &World, generation: u64) -> Result<(), Fault> {
    if world.resource::<vivy_gateway::ProvidersState>().generation != generation {
        return Err(fault(
            "provider configuration changed; reload before saving".into(),
        ));
    }
    Ok(())
}
pub fn save(world: &mut World, input: ProvidersSave) -> Result<Reply, Fault> {
    check(world, input.generation)?;
    input.config.validate().map_err(|message| Fault {
        code: ErrorCode::InvalidRequest,
        message,
    })?;
    if input
        .config
        .default_model
        .as_ref()
        .is_some_and(|m| !input.config.contains_model(m))
    {
        return Err(fault(
            "default model must be an enabled configured model".into(),
        ));
    }
    vivy_gateway::save_config(world, input.config).map_err(fault)?;
    if let Some(agent) = vivy_durable::runtime::foreground(world) {
        vivy_gateway::select_default(world, agent);
    }
    Ok(providers(world))
}
pub fn key(world: &mut World, input: ProviderKey) -> Result<Reply, Fault> {
    check(world, input.generation)?;
    match input.key {
        Some(key) => vivy_gateway::save_key(world, &input.provider, &key),
        None => vivy_gateway::remove_key(world, &input.provider),
    }
    .map_err(fault)?;
    if let Some(agent) = vivy_durable::runtime::foreground(world) {
        vivy_gateway::select_default(world, agent);
    }
    Ok(providers(world))
}
pub fn reload(world: &mut World) -> Result<Reply, Fault> {
    vivy_gateway::reload(world).map_err(fault)?;
    Ok(providers(world))
}
