//! Dependency scope belongs to setup; candidates only request an expanded scope.
use super::*;
use rig_harness::prelude::*;

#[derive(Resource, Default, Clone)]
pub struct PendingRequirements(pub Vec<String>);

pub fn request_dependencies(world: &mut World, dependencies: &[String]) -> io::Result<bool> {
    let root = world.resource::<ProjectRoot>().0.clone();
    let store = PublicationStore::new(&root, RigHome::of(world).root());
    let current = store.current()?;
    let admitted: Vec<String> = serde_json::from_value(
        current
            .manifest
            .get("managed_requirements")
            .cloned()
            .unwrap_or(json!([])),
    )?;
    if dependencies.iter().all(|d| admitted.contains(d)) {
        return Ok(false);
    }
    // Preserve the previous interpreter's local authority before setup expands its grant.
    let mut required = admitted;
    required.extend_from_slice(dependencies);
    required.sort();
    required.dedup();
    if world
        .get_resource::<PendingRequirements>()
        .is_none_or(|p| p.0 != required)
    {
        store.retain_current()?;
        world.insert_resource(PendingRequirements(required));
        crate::management::initialization::refresh(world);
    }
    Ok(true)
}
pub fn staged_environment(world: &World, base: &str) -> io::Result<Option<Publication>> {
    let store = PublicationStore::new(
        &world.resource::<ProjectRoot>().0,
        RigHome::of(world).root(),
    );
    let scope = crate::management::initialization::scope(world);
    Ok(store
        .staged(&scope)?
        .filter(|p| p.manifest["staged_base"] == base))
}
pub(crate) fn requirements(world: &World) -> Vec<String> {
    let store = PublicationStore::new(
        &world.resource::<ProjectRoot>().0,
        RigHome::of(world).root(),
    );
    let mut requirements: Vec<String> = store
        .current()
        .ok()
        .and_then(|p| {
            serde_json::from_value(
                p.manifest
                    .get("managed_requirements")
                    .cloned()
                    .unwrap_or(json!([])),
            )
            .ok()
        })
        .unwrap_or_default();
    if let Some(pending) = world.get_resource::<PendingRequirements>() {
        requirements.extend_from_slice(&pending.0);
    }
    requirements.sort();
    requirements.dedup();
    requirements
}

/// Manual preparation changes the environment, so recheck every published callable
/// before making that pair visible. Candidate preparation remains staged instead.
pub(crate) fn finish(
    store: &PublicationStore,
    base: &Publication,
    manifest: Value,
    stage_only: bool,
    cancel: &crate::Cancellation,
) -> Result<String, String> {
    let staged = store.stage(&base.id, manifest).map_err(|e| e.to_string())?;
    let mut next = staged.manifest.clone();
    for skill in staged.skills().map_err(|e| e.to_string())? {
        let saved: ValidationReceipt =
            serde_json::from_value(base.manifest["managed_validations"][&skill.name].clone())
                .map_err(|e| format!("No retained callable checks for {}: {e}", skill.name))?;
        let mut spec = saved.spec.ok_or_else(|| {
            format!(
                "No retained callable check specification for {}",
                skill.name
            )
        })?;
        spec.require_regression = false;
        let receipt = validation::run(
            store,
            &staged,
            &skill,
            &spec,
            &store
                .root
                .join("prepare-validation")
                .join(&staged.id)
                .join(&skill.name),
            cancel,
        )?;
        if !receipt.passed {
            return Err(format!(
                "Prepared environment failed the retained checks for {}",
                skill.name
            ));
        }
        next["managed_validations"][&skill.name] = json!(receipt);
    }
    if cancel.cancelled.load(std::sync::atomic::Ordering::SeqCst) {
        return Err("preparation cancelled before activation".into());
    }
    if stage_only {
        store.ready_stage(&staged).map_err(|e| e.to_string())?;
        return Ok("Dependency environment staged; retained Skills validated. The waiting candidate must pass validation before activation.".into());
    }
    let _lock = store.lock().map_err(|e| e.to_string())?;
    let current = store.current().map_err(|e| e.to_string())?;
    if current.id != base.id {
        return Err("stale manual preparation base; current publication preserved".into());
    }
    store
        .activate(&current, next, &format!("prepare:{}", staged.id))
        .map_err(|e| e.to_string())?;
    Ok("Python environment and retained Skills validated and published. New calls adopt them with fresh variables.".into())
}
