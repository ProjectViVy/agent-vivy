use super::*;
use vivy_python::managed::{PinnedCall, PublicationStore};

pub(super) fn install(app: &mut App) {
    app.add_systems(
        AgentPass,
        pin.in_set(PrepareSystems::Edit)
            .before(vivy_python::BindWorkbench),
    );
}
fn pin(world: &mut World) {
    let calls: Vec<_> = world
        .query_filtered::<(
            Entity,
            &ForAgent,
            Option<&ToolRequest>,
            Option<&ModelRequest>,
        ), (
            With<Admitted>,
            Without<Started>,
            Without<Outcome>,
            Without<PinnedCall>,
        )>()
        .iter(world)
        .filter(|(_, _, t, m)| t.is_some() || m.is_some())
        .map(|(e, a, t, m)| (e, a.0, t.is_some(), m.is_some()))
        .collect();
    for (call, agent, tool, model) in calls {
        if assistant::is_maintenance(world, call) {
            continue;
        }
        let uncertain = world.query::<&DurableAgentState>().iter(world).any(|s| {
            s.evolution
                .jobs
                .iter()
                .any(|j| j.phase == "publishing" && j.error.is_some())
        });
        if uncertain {
            hold(world, call);
            continue;
        }
        let Some(state) = world.get::<DurableAgentState>(agent) else {
            continue;
        };
        let Some(task) = state.active.as_ref() else {
            continue;
        };
        let prior = world
            .get::<Bound>(call)
            .and_then(|b| state.operations.iter().find(|o| o.id == b.operation));
        let retry = world
            .get::<CallOf>(call)
            .and_then(|of| world.get::<rig_harness::rig_ecs::turn::Recovery>(of.0))
            .is_some_and(|r| r.retries > 0);
        // Rig's model replies (including restored pending replies) give their
        // tools a parent effect. A direct tool may belong to a turn without
        // having been planned by its model, so CallOf alone is insufficient.
        let planned = tool
            && world
                .get::<Effect>(call)
                .is_some_and(|e| e.parent.is_some());
        let source = prior.or_else(|| {
            (planned || retry)
                .then(|| {
                    state
                        .operations
                        .iter()
                        .rev()
                        .find(|o| o.task == *task && o.kind == "model")
                })
                .flatten()
        });
        let selection = if let Some(source) = source {
            Ok(source.publication.clone())
        } else {
            let store = PublicationStore::new(&root(world), RigHome::of(world).root());
            if store.root().join("current.json").exists() {
                store.current().map(|p| Some(p.id))
            } else {
                Ok(None)
            }
        };
        match selection {
            Ok(publication) => {
                if model {
                    if let Err(e) = instructions(world, call, publication.as_deref()) {
                        world.entity_mut(call).insert(Outcome::refused(format!(
                            "Cannot compose selected Skill instructions: {e}"
                        )));
                        continue;
                    }
                }
                world.entity_mut(call).insert(PinnedCall {
                    publication,
                    ..Default::default()
                });
            }
            Err(e) => {
                world.entity_mut(call).insert(Outcome::refused(format!(
                    "Cannot select workbench publication: {e}"
                )));
            }
        }
    }
}

fn instructions(world: &mut World, call: Entity, publication: Option<&str>) -> io::Result<()> {
    use vivy_basics::skills::{
        PublishedSkills, Skill, SkillCollisions, TrustedSkillInventory, listing,
    };
    let mut skills = world
        .get_resource::<TrustedSkillInventory>()
        .map(|i| i.0.clone())
        .unwrap_or_default();
    if let Some(published) = world.get_resource::<PublishedSkills>() {
        skills.retain(|s| !published.0.iter().any(|p| p.file == s.file));
    }
    if let Some(id) = publication {
        let selected =
            PublicationStore::new(&root(world), RigHome::of(world).root()).selected(id)?;
        for s in selected.skills()? {
            if world
                .get_resource::<SkillCollisions>()
                .is_some_and(|c| c.0.contains(&s.name))
            {
                return Err(invalid("managed Skill collides with ordinary discovery"));
            }
            if skills.iter().any(|ordinary| ordinary.name == s.name) {
                return Err(invalid("managed Skill collides with ordinary discovery"));
            }
            skills.push(Skill {
                name: s.name,
                description: s.description,
                file: PathBuf::from(s.file),
                manual: false,
            });
        }
    }
    if let Some(mut request) = world.get_mut::<ModelRequest>(call) {
        if let Some(prompt) = &mut request.prompt {
            prompt.sections.retain(|s| s.tag != "skills");
            prompt
                .sections
                .push(rig_harness::rig_core::completion::Section::new(
                    "skills",
                    listing(&skills),
                ));
        }
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn direct_tool_admission_selects_current_while_planned_tools_keep_the_model_pin() {
        let mut world = World::new();
        let root = std::env::temp_dir().join(format!("vivy-direct-pin-{}", std::process::id()));
        world.insert_resource(ProjectRoot(root.clone()));
        world.insert_resource(RigHome(rig_harness::harness_protocol::Home::at(
            root.join("home"),
        )));
        let old = "a".repeat(64);
        let mut state = DurableAgentState::default();
        state.active = Some("task:1".into());
        state.operations.push(serde_json::from_value(json!({"id":"task:1/0/0","task":"task:1","generation":0,"kind":"model","call_id":null,"request":null,"digest":"fixture","route":"fixture","step":0,"attempt":0,"retry_at_ms":0,"phase":"settled","outcome":null,"published":true,"resolution":null,"publication":old,"usage":[]})).unwrap());
        let agent = world.spawn(state).id();
        let request = || {
            ToolRequest{call:serde_json::from_value(json!({"id":{"provider":"direct"},"function":{"name":"python_repl","arguments":{"code":"pass"}}})).unwrap()}
        };
        let turn = world.spawn_empty().id();
        let direct = world
            .spawn((request(), ForAgent(agent), Admitted, CallOf(turn)))
            .id();
        let parent = Effect::default().id();
        let planned = world
            .spawn((
                request(),
                ForAgent(agent),
                Admitted,
                CallOf(turn),
                Effect::with_parent_id(Some(parent)),
            ))
            .id();
        pin(&mut world);
        assert_eq!(world.get::<PinnedCall>(direct).unwrap().publication, None);
        assert_eq!(
            world.get::<PinnedCall>(planned).unwrap().publication,
            Some(old)
        );
    }
}
