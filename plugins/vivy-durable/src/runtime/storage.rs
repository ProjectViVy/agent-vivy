use super::*;
use rig_harness::host::session::{SessionPaths, StoreOpener};
use rig_memory::CheckpointStore;
use std::{
    path::PathBuf,
    sync::{Arc, Mutex},
};

#[derive(Resource, Default, Clone)]
pub(super) struct Checkpoints {
    stores: Arc<Mutex<BTreeMap<PathBuf, CheckpointStore>>>,
    pub committed: BTreeMap<String, DurableAgentState>,
    #[cfg(feature = "assistant-context")]
    contexts: BTreeMap<String, assistant::AssistantContextState>,
    seen: BTreeMap<String, (rig_harness::rig_core::conversation::Lineage, u64)>,
    initialized: bool,
}

pub(super) fn install(app: &mut App) {
    let checkpoints = Checkpoints::default();
    if let Some(SessionStore(inner)) = app.world_mut().remove_resource::<SessionStore>() {
        let store = CheckpointStore::new(inner);
        checkpoints
            .stores
            .lock()
            .unwrap()
            .insert(PathBuf::new(), store.clone());
        app.insert_resource(SessionStore::new(store));
    } else {
        let opener = app
            .world()
            .get_resource::<StoreOpener>()
            .cloned()
            .unwrap_or_default();
        let stores = checkpoints.stores.clone();
        app.insert_resource(StoreOpener::new(move |dir| {
            let store = CheckpointStore::new(opener.open(dir)?);
            stores
                .lock()
                .unwrap()
                .insert(dir.path().to_path_buf(), store.clone());
            Ok(Arc::new(store))
        }));
    }
    app.insert_resource(checkpoints);
}

fn store(world: &World) -> Option<CheckpointStore> {
    let key = world
        .get_resource::<SessionPaths>()
        .map(|p| p.path().to_path_buf())
        .unwrap_or_default();
    let stores = world.resource::<Checkpoints>().stores.lock().unwrap();
    stores
        .get(&key)
        .or_else(|| stores.get(&PathBuf::new()))
        .cloned()
}

pub(super) fn healthy(world: &World) -> bool {
    world
        .get_resource::<SessionLog>()
        .is_some_and(|log| log.failure().is_none())
        && store(world).is_some_and(|s| s.failure().is_none())
}

pub(super) fn initialize(world: &mut World) {
    if world.resource::<Checkpoints>().initialized {
        return;
    }
    world.resource_mut::<Checkpoints>().initialized = true;
    let Some(store) = store(world) else {
        return;
    };
    let restored: Vec<_> = world
        .query::<(&AgentId, &DurableAgentState)>()
        .iter(world)
        .filter(|(id, _)| store.receipt(&id.0).is_some())
        .map(|(id, s)| (id.0.clone(), s.clone()))
        .collect();
    world
        .resource_mut::<Checkpoints>()
        .committed
        .extend(restored);
    #[cfg(feature = "assistant-context")]
    {
        let contexts: Vec<_> = world
            .query::<(&AgentId, &assistant::AssistantContextState)>()
            .iter(world)
            .filter(|(id, _)| store.receipt(&id.0).is_some())
            .map(|(id, s)| (id.0.clone(), s.clone()))
            .collect();
        world
            .resource_mut::<Checkpoints>()
            .contexts
            .extend(contexts);
    }
}

pub(super) fn receipts(world: &mut World) {
    if !healthy(world) {
        return;
    }
    let Some(store) = store(world) else {
        return;
    };
    let ids: Vec<_> = world
        .query::<&AgentId>()
        .iter(world)
        .map(|id| id.0.clone())
        .collect();
    for id in ids {
        let Some(receipt) = store.receipt(&id) else {
            continue;
        };
        let identity = (receipt.lineage, receipt.sequence);
        if world.resource::<Checkpoints>().seen.get(&id) == Some(&identity) {
            continue;
        }
        for line in receipt
            .bytes
            .split(|b| *b == b'\n')
            .filter(|l| !l.is_empty())
        {
            let Ok(value) = serde_json::from_slice::<Value>(line) else {
                continue;
            };
            if value["type"] == "state" && value["key"] == KEY {
                if let Ok(state) =
                    serde_json::from_value::<DurableAgentState>(value["value"].clone())
                {
                    probe("committed", &state);
                    world
                        .resource_mut::<Checkpoints>()
                        .committed
                        .insert(id.clone(), state);
                }
            }
            #[cfg(feature = "assistant-context")]
            if value["type"] == "state" && value["key"] == assistant::CONTEXT_KEY {
                if let Ok(state) = serde_json::from_value::<assistant::AssistantContextState>(
                    value["value"].clone(),
                ) {
                    if let Some(job) = state.jobs.last() {
                        if job.phase == "received" {
                            assistant::probe("after_result", &state);
                        }
                        if job.phase == "published" {
                            assistant::probe("after_publish", &state);
                        }
                    }
                    world
                        .resource_mut::<Checkpoints>()
                        .contexts
                        .insert(id.clone(), state);
                }
            }
        }
        world
            .resource_mut::<Checkpoints>()
            .seen
            .insert(id, identity);
        world.resource::<Wake>().wake();
    }
}

#[cfg(feature = "assistant-context")]
pub(super) fn committed_context(
    world: &World,
    agent: Entity,
) -> Option<&assistant::AssistantContextState> {
    let id = &world.get::<AgentId>(agent)?.0;
    world.resource::<Checkpoints>().contexts.get(id)
}

pub(super) fn committed<'a>(world: &'a World, agent: Entity) -> Option<&'a DurableAgentState> {
    let id = &world.get::<AgentId>(agent)?.0;
    world.resource::<Checkpoints>().committed.get(id)
}
