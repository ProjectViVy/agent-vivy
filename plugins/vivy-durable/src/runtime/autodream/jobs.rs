//! One host-owned validation job, with receipt-gated activation and native inbox delivery.
use super::*;
use rig_harness::rig_core::conversation::{Author, RequestId};
use vivy_python::managed::{
    Publication, PublicationStore, SkillRevision, ValidationJob, ValidationReceipt, ValidationSpec,
};

#[derive(Clone, Debug, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct Job {
    pub id: String,
    pub operation: String,
    pub task: String,
    pub generation: u64,
    pub target: String,
    pub expected_base: String,
    pub evidence: Vec<String>,
    pub skill: Option<SkillRevision>,
    pub persona: Option<String>,
    #[serde(default)]
    pub file: Option<String>,
    #[serde(default)]
    pub file_digest: Option<String>,
    pub environment: Option<Publication>,
    pub checks: ValidationSpec,
    pub phase: String,
    pub attempt: u32,
    pub unknown_attempts: Vec<u32>,
    pub result: Option<Value>,
    pub adoption: Option<Value>,
    pub error: Option<String>,
    pub notice_step: Option<u64>,
    pub ready: bool,
    pub delivered: Option<wake::Delivery>,
    pub skipped: bool,
    pub update: String,
}
impl Job {
    pub fn terminal(&self) -> bool {
        matches!(
            self.phase.as_str(),
            "published" | "failed" | "stale" | "cancelled" | "unknown"
        )
    }
    fn input(&self) -> Incoming {
        let mut author = Author::plugin("vivy.autodream.job");
        author.request = Some(RequestId(self.id.clone()));
        Incoming::new(format!("AutoDream job {} for task {} completed: {}. Validation: {}. Adoption: {}. Error: {}. Decide whether to inspect, revise, wait or finish silently.",self.id,self.task,self.phase,json!(self.result),json!(self.adoption),json!(self.error)),DeliveryMode::Steer).with_author(author)
    }
}
#[derive(Resource, Default)]
struct Workers {
    running: BTreeMap<String, ValidationJob>,
    initialized: std::collections::BTreeSet<String>,
}
#[derive(Component)]
struct PublicationRetry(String);
#[derive(Serialize, Deserialize, schemars::JsonSchema)]
#[serde(deny_unknown_fields)]
struct CandidateArgs {
    target: String,
    source: String,
    expected_base: String,
    #[serde(default)]
    evidence: Vec<String>,
    #[serde(default)]
    checks: Value,
    #[serde(default)]
    name: Option<String>,
    #[serde(default)]
    file: Option<String>,
}
#[derive(Deserialize, schemars::JsonSchema)]
#[serde(deny_unknown_fields)]
struct JobArgs {
    job: String,
}

pub(super) fn install(app: &mut App) {
    app.init_resource::<Workers>()
        .add_systems(First,tick.after(super::tick).before(crate::runtime::recover_inputs))
        .add_systems(Last,stop_workers.in_set(OnAppExitSystems).run_if(on_message::<rig_harness::AppExit>))
        .add_observer(delivered)
        .add_open_tool("candidate_submit","Freeze one persona core file (file must be IDENTITY.MD, RELATIONSHIP.MD, DREAM.MD or DARK.MD) or a pure-Python Skill directory, admit background checks, and automatically adopt the exact passing version. expected_base is the persona revision or workbench publication from dream_status. name is required for Skills. Submission is not validation success.",ToolOptions::default(),
            |e:On<ToolCalled<CandidateArgs>>,mut commands:Commands| {
                let (call,agent,args)=(e.call,e.agent,serde_json::to_value(&e.args).unwrap_or(Value::Null));
                commands.queue(move |world:&mut World| {
                    let result=serde_json::from_value(args).map_err(io::Error::other).and_then(|args|submit_candidate(world,agent,call,args));
                    answer(world,call,result);
                });
            })
        .add_open_tool("validation_status","Inspect committed job admission, validation, publication and delivery; a job ID is not successful validation.",ToolOptions::default(),
            |e:On<ToolCalled<JobArgs>>,mut commands:Commands| {
                let (call,agent,id)=(e.call,e.agent,e.args.job.clone());
                commands.queue(move |world:&mut World| {let result=committed(world,agent).and_then(|s|s.evolution.jobs.iter().find(|j|j.id==id)).map(|j|json!(j)).ok_or_else(||invalid("no committed job with this identity"));answer(world,call,result);});
            })
        .add_open_tool("validation_wait","Suspend the owning Dream task until this job completes. Already completed jobs return their saved result. Cancelling the wait alone does not cancel the job.",ToolOptions::default(),
            |e:On<ToolCalled<JobArgs>>,mut commands:Commands| {
                let (call,agent,id)=(e.call,e.agent,e.args.job.clone());
                commands.queue(move |world:&mut World| {let result=wait(world,agent,&id);answer(world,call,result);});
            })
        .add_open_tool("validation_cancel","Cancel this owned job and fence its publication; cancelling a waiter is separate. Inspect the result for any publication that already won the race.",ToolOptions::default(),
            |e:On<ToolCalled<JobArgs>>,mut commands:Commands| {
                let (call,agent,id)=(e.call,e.agent,e.args.job.clone());
                commands.queue(move |world:&mut World| {let result=cancel(world,agent,&id);answer(world,call,result);});
            })
        .add_open_tool("validation_detach","Cancel only this job's completion wait. The admitted job continues and its result remains available through validation_status.",ToolOptions::default(),
            |e:On<ToolCalled<JobArgs>>,mut commands:Commands| {
                let (call,agent,id)=(e.call,e.agent,e.args.job.clone());
                commands.queue(move |world:&mut World| {let result=detach(world,agent,&id);answer(world,call,result);});
            });
}
fn publication_store(world: &World) -> PublicationStore {
    PublicationStore::new(&root(world), RigHome::of(world).root())
}
fn submit_candidate(
    world: &mut World,
    agent: Entity,
    call: Entity,
    args: CandidateArgs,
) -> io::Result<Value> {
    acquire(world)?;
    let remaining = world
        .resource::<Settings>()
        .staging_budget_bytes
        .checked_sub(directory_bytes(&root(world).join(".vivy/evolution"))?)
        .filter(|n| *n > 0)
        .ok_or_else(|| invalid("AutoDream staging budget exhausted"))?;
    let state = world
        .get::<DurableAgentState>(agent)
        .ok_or_else(|| invalid("not a durable agent"))?;
    if !state.evolution.dream {
        return Err(invalid(
            "candidate submission belongs to the internal Dream role",
        ));
    }
    let bound = world
        .get::<Bound>(call)
        .cloned()
        .ok_or_else(|| invalid("candidate submission requires a durable operation"))?;
    if let Some(job) = state
        .evolution
        .jobs
        .iter()
        .find(|j| j.operation == bound.operation)
    {
        return Ok(json!({"job":job.id,"status":job.phase}));
    }
    if state.evolution.jobs.iter().any(|j| !j.terminal()) {
        return Err(invalid(
            "one validation job is already active; inspect or wait",
        ));
    }
    if args
        .evidence
        .iter()
        .any(|id| !state.evolution.evidence.contains_key(id))
    {
        return Err(invalid("candidate refers to unknown evidence"));
    }
    let source = PathBuf::from(&args.source);
    let source = if source.is_absolute() {
        source
    } else {
        root(world).join(".vivy/evolution").join(source)
    };
    let mut job = Job {
        id: format!("job:{}", state.evolution.jobs.len() + 1),
        operation: bound.operation,
        task: bound.task,
        generation: bound.generation,
        target: args.target.clone(),
        expected_base: args.expected_base,
        evidence: args.evidence,
        skill: None,
        persona: None,
        file: None,
        file_digest: None,
        environment: None,
        checks: ValidationSpec::default(),
        phase: "admitted".into(),
        attempt: 0,
        unknown_attempts: vec![],
        result: None,
        adoption: None,
        error: None,
        notice_step: None,
        ready: false,
        delivered: None,
        skipped: false,
        update: String::new(),
    };
    job.update = format!(
        "autodream:{}",
        digest(&(world.get::<AgentId>(agent).unwrap().0.clone(), &job.id))
    );
    match args.target.as_str() {
        "persona" => {
            let file = args
                .file
                .as_deref()
                .ok_or_else(|| invalid("persona candidates must name a core file"))?;
            if !vivy_laputa::auto(file) {
                return Err(invalid(
                    "only IDENTITY.MD, RELATIONSHIP.MD, DREAM.MD and DARK.MD may be evolved automatically",
                ));
            }
            let store = persona_store(world);
            if !store.initialized()? {
                return Err(invalid(
                    "persona candidates require an initialized Laputa persona",
                ));
            }
            let current = store.read()?;
            if current.revision.to_string() != job.expected_base {
                return Err(invalid("stale persona base"));
            }
            let bytes = fs::read(source)?;
            if bytes.len() > vivy_laputa::MAX_FILE_BYTES {
                return Err(invalid(
                    "persona candidate exceeds the per-file byte budget",
                ));
            }
            let text = String::from_utf8(bytes).map_err(io::Error::other)?;
            if text.trim().is_empty() {
                return Err(invalid("persona candidate must not be blank"));
            }
            job.file = Some(file.to_string());
            job.file_digest = Some(vivy_laputa::digest_text(current.file(file)));
            job.persona = Some(text);
        }
        "skill" => {
            let store = publication_store(world);
            let current = store.current()?;
            if current.id != job.expected_base {
                return Err(invalid("stale Skill publication base"));
            }
            let mut spec: ValidationSpec = serde_json::from_value(if args.checks.is_null() {
                json!({})
            } else {
                args.checks
            })?;
            spec.output_budget_bytes = spec.output_budget_bytes.min(remaining);
            spec.validate()?;
            let skill = store.freeze(
                args.name
                    .as_deref()
                    .ok_or_else(|| invalid("Skill name is required"))?,
                &source,
                world.resource::<Settings>().staging_budget_bytes,
            )?;
            if world
                .get_resource::<vivy_basics::skills::TrustedSkillInventory>()
                .is_some_and(|i| {
                    i.0.iter().any(|s| {
                        s.name == skill.name
                            && !current
                                .skills()
                                .unwrap_or_default()
                                .iter()
                                .any(|published| published.file == s.file.to_string_lossy())
                    })
                })
            {
                return Err(invalid("candidate collides with ordinary Skill discovery"));
            }
            job.skill = Some(skill);
            job.environment = Some(current);
            job.checks = spec;
        }
        _ => return Err(invalid("candidate target must be persona or skill")),
    }
    let id = job.id.clone();
    let mut state = world.get_mut::<DurableAgentState>(agent).unwrap();
    state.evolution.jobs.push(job);
    state.bump();
    Ok(json!({"job":id,"status":"admitted_pending_commit"}))
}
fn wait(world: &mut World, agent: Entity, id: &str) -> io::Result<Value> {
    let saved = committed(world, agent)
        .and_then(|s| s.evolution.jobs.iter().find(|j| j.id == id))
        .cloned()
        .ok_or_else(|| invalid("job admission is not committed"))?;
    if saved.terminal() {
        return Ok(json!(saved));
    }
    let mut state = world.get_mut::<DurableAgentState>(agent).unwrap();
    if state.active.as_deref() != Some(&saved.task) || state.control.is_some() {
        return Err(invalid("only the active owner can wait for this job"));
    }
    let control = Control {
        action: "defer".into(),
        task: saved.task.clone(),
        generation: saved.generation,
        seconds: None,
        reason: None,
        wake_id: None,
        wake_generation: None,
    };
    state.control = Some(PendingControl {
        request: control,
        origin: saved.task,
        origin_generation: saved.generation,
        wake: None,
    });
    state.bump();
    state
        .evolution
        .jobs
        .iter_mut()
        .find(|j| j.id == id)
        .unwrap()
        .skipped = false;
    Ok(json!({"job":id,"status":"waiting_pending_commit"}))
}
pub(super) fn detach(world: &mut World, agent: Entity, id: &str) -> io::Result<Value> {
    let mut state = world
        .get_mut::<DurableAgentState>(agent)
        .ok_or_else(|| invalid("no owning Dream"))?;
    let job = state
        .evolution
        .jobs
        .iter_mut()
        .find(|j| j.id == id)
        .ok_or_else(|| invalid("job not found"))?;
    if job.delivered.is_none() {
        job.skipped = true;
    }
    let result = json!({"job":job.id,"phase":job.phase,"wait":"detached","job_cancelled":false});
    state.bump();
    Ok(result)
}
pub(super) fn persona_target(job: &Job) -> io::Result<(&str, &str)> {
    let file = job
        .file
        .as_deref()
        .ok_or_else(|| invalid("legacy persona candidate has no target file"))?;
    let file_digest = job
        .file_digest
        .as_deref()
        .ok_or_else(|| invalid("persona candidate is missing its file digest"))?;
    Ok((file, file_digest))
}
fn reconcile(world: &World, job: &Job) -> io::Result<Option<Value>> {
    if job.target == "skill" {
        return publication_store(world)
            .activated(&job.update)
            .map(|p| p.map(|p| json!({"publication":p.id})));
    }
    let store = persona_store(world);
    // update is idempotent over activated ancestry; call it only when the active
    // lineage proves the ID or at the publication step below.
    store
        .activated(&job.update)
        .map(|p| p.map(|p| json!({"persona_revision":p.revision})))
}
pub(super) fn cancel(world: &mut World, agent: Entity, id: &str) -> io::Result<Value> {
    let snapshot = world
        .get::<DurableAgentState>(agent)
        .and_then(|s| s.evolution.jobs.iter().find(|j| j.id == id))
        .cloned()
        .ok_or_else(|| invalid("job not found"))?;
    if let Some(active) = world
        .resource_mut::<Workers>()
        .running
        .remove(&snapshot.update)
    {
        active.cancel();
    }
    let adopted = if snapshot.phase == "publishing" {
        reconcile(world, &snapshot)?
    } else {
        snapshot.adoption.clone()
    };
    let mut state = world.get_mut::<DurableAgentState>(agent).unwrap();
    let job = state
        .evolution
        .jobs
        .iter_mut()
        .find(|j| j.id == id)
        .unwrap();
    if let Some(adopted) = adopted {
        job.phase = "published".into();
        job.adoption = Some(adopted);
    } else if !job.terminal() {
        job.phase = "cancelled".into();
        job.error = Some("job cancelled".into());
    }
    let value = json!(job);
    state.bump();
    Ok(value)
}
fn tick(world: &mut World) {
    if !healthy(world) {
        world.resource_mut::<Workers>().running.clear();
        return;
    }
    let agents: Vec<_> = world
        .query::<(Entity, &AgentId, &DurableAgentState)>()
        .iter(world)
        .filter(|(_, _, s)| s.evolution.dream)
        .map(|(e, id, _)| (e, id.0.clone()))
        .collect();
    for (agent, id) in agents {
        if acquire(world).is_err() {
            continue;
        }
        let first = world.resource_mut::<Workers>().initialized.insert(id);
        let Some(saved) = committed(world, agent).cloned() else {
            continue;
        };
        let jobs = world
            .get::<DurableAgentState>(agent)
            .unwrap()
            .evolution
            .jobs
            .clone();
        for mut job in jobs {
            let retry = world
                .query::<(Entity, &PublicationRetry, Option<&rig_harness::clock::Due>)>()
                .iter(world)
                .find(|(_, r, _)| r.0 == job.update)
                .map(|(e, _, due)| (e, due.is_some()));
            if let Some((entity, due)) = retry {
                if !due && !job.terminal() {
                    continue;
                }
                world.despawn(entity);
            }
            let Some(record) = saved.evolution.jobs.iter().find(|j| j.id == job.id) else {
                continue;
            };
            if &job != record {
                continue;
            }
            let valid = world
                .get::<DurableAgentState>(agent)
                .and_then(|s| s.task(&job.task))
                .is_some_and(|t| t.generation == job.generation && t.status != Status::Cancelled);
            if !valid && !job.terminal() {
                let _ = cancel(world, agent, &job.id);
                continue;
            }
            let result: io::Result<()> = (|| {
                match job.phase.as_str() {
                    "admitted" => {
                        if !saved.operations.iter().any(|o| {
                            o.id == job.operation && (o.outcome.is_some() || o.resolution.is_some())
                        }) {
                            return Ok(());
                        }
                        if let Some(skill) = &job.skill {
                            if vivy_python::managed::request_dependencies(
                                world,
                                &skill.dependencies,
                            )? {
                                job.phase = "waiting_setup".into();
                            } else {
                                job.phase = "dispatched".into();
                            }
                        } else {
                            job.phase = "dispatched".into();
                        }
                    }
                    "waiting_setup" => {
                        let current = publication_store(world).current()?;
                        if current.id != job.expected_base {
                            return Err(invalid("stale candidate while waiting for setup"));
                        }
                        vivy_python::managed::request_dependencies(
                            world,
                            &job.skill
                                .as_ref()
                                .ok_or_else(|| invalid("missing pending Skill"))?
                                .dependencies,
                        )?;
                        if let Some(staged) =
                            vivy_python::managed::staged_environment(world, &job.expected_base)?
                        {
                            job.environment = Some(staged);
                            job.phase = "dispatched".into();
                        }
                    }
                    "dispatched" => {
                        if first {
                            job.unknown_attempts.push(job.attempt);
                            if job.target == "persona"
                                || (job.checks.replay_safe
                                    && job.attempt
                                        < rig_harness::rig_ecs::turn::RETRY_POLICY.max_retries)
                            {
                                job.attempt += 1;
                                job.phase = "admitted".into();
                            } else {
                                job.phase = "unknown".into();
                                job.error=Some("host exited after dispatch intent without committed validation outcome".into());
                            }
                        } else if job.target == "persona" {
                            job.result = Some(
                                json!({"passed":true,"assertions":1,"file":job.file,"file_digest":job.file_digest,"persona_digest":digest(&job.persona)}),
                            );
                            job.phase = "validated".into();
                        } else if let Some(running) =
                            world.resource::<Workers>().running.get(&job.update)
                        {
                            if let Some(result) = running.poll() {
                                world.resource_mut::<Workers>().running.remove(&job.update);
                                match result {
                                    Ok(receipt) => {
                                        job.phase = if receipt.passed {
                                            "validated"
                                        } else {
                                            "failed"
                                        }
                                        .into();
                                        job.result = Some(json!(receipt));
                                    }
                                    Err(error) => {
                                        job.phase = "failed".into();
                                        job.error = Some(error);
                                    }
                                }
                            }
                        } else {
                            let scratch = root(world)
                                .join(".vivy/evolution/validation")
                                .join(digest(&(&job.update, job.attempt)));
                            let runner = ValidationJob::start(
                                publication_store(world),
                                job.environment
                                    .clone()
                                    .ok_or_else(|| invalid("missing validation environment"))?,
                                job.skill
                                    .clone()
                                    .ok_or_else(|| invalid("missing frozen Skill"))?,
                                job.checks.clone(),
                                scratch,
                                Some(world.resource::<Wake>().clone()),
                            )?;
                            world
                                .resource_mut::<Workers>()
                                .running
                                .insert(job.update.clone(), runner);
                        }
                    }
                    "validated" => {
                        job.phase = "publishing".into();
                    }
                    "publishing" => {
                        let adopted = if job.target == "persona" {
                            let expected =
                                job.expected_base.parse::<u64>().map_err(io::Error::other)?;
                            let (file, file_digest) = persona_target(&job)?;
                            let store = persona_store(world);
                            let current = store.read()?;
                            let persona = store.save_file_checked(
                                expected,
                                &current.last_update_id,
                                file_digest,
                                &job.update,
                                file,
                                job.persona
                                    .as_deref()
                                    .ok_or_else(|| invalid("missing frozen persona"))?,
                                job.evidence.clone(),
                            )?;
                            json!({"persona_revision":persona.revision,"file":file})
                        } else {
                            let receipt: ValidationReceipt = serde_json::from_value(
                                job.result
                                    .clone()
                                    .ok_or_else(|| invalid("missing validation receipt"))?,
                            )?;
                            let publication = publication_store(world).publish_in(
                                &job.expected_base,
                                job.environment
                                    .as_ref()
                                    .ok_or_else(|| invalid("missing validation environment"))?,
                                job.skill
                                    .as_ref()
                                    .ok_or_else(|| invalid("missing frozen Skill"))?,
                                &receipt,
                                &job.update,
                            )?;
                            json!({"publication":publication.id})
                        };
                        probe("dream_activated", &saved);
                        job.adoption = Some(adopted);
                        job.phase = "published".into();
                    }
                    _ => (),
                }
                Ok(())
            })();
            if let Err(e) = result {
                if e.kind() == io::ErrorKind::WouldBlock {
                    world.spawn((
                        PublicationRetry(job.update.clone()),
                        rig_harness::clock::WakeAt(
                            std::time::SystemTime::now() + std::time::Duration::from_millis(100),
                        ),
                    ));
                    continue;
                }
                if job.phase == "publishing" {
                    match reconcile(world, &job) {
                        Ok(Some(adopted)) => {
                            job.adoption = Some(adopted);
                            job.phase = "published".into();
                        }
                        Ok(None) => {
                            job.phase = if e.to_string().contains("stale")
                                || e.to_string().contains("revision conflict")
                            {
                                "stale"
                            } else {
                                "failed"
                            }
                            .into();
                        }
                        Err(cause) => {
                            job.error =
                                Some(format!("Publication requires reconciliation: {e}; {cause}"));
                        }
                    }
                } else {
                    job.phase = if e.to_string().contains("stale") {
                        "stale"
                    } else {
                        "failed"
                    }
                    .into();
                }
                if job.error.is_none() {
                    job.error = Some(e.to_string());
                }
            }
            if job != *record {
                let job_id = job.id.clone();
                let mut s = world.get_mut::<DurableAgentState>(agent).unwrap();
                *s.evolution
                    .jobs
                    .iter_mut()
                    .find(|j| j.id == job_id)
                    .unwrap() = job;
                s.bump();
                probe("dream_job", &s);
            }
        }
        prepare_notices(world, agent);
    }
    let pending = world
        .query::<&DurableAgentState>()
        .iter(world)
        .any(|s| s.evolution.jobs.iter().any(|j| j.phase == "waiting_setup"));
    if !pending
        && world
            .get_resource::<vivy_python::managed::PendingRequirements>()
            .is_some_and(|p| !p.0.is_empty())
    {
        world.insert_resource(vivy_python::managed::PendingRequirements::default());
    }
}
fn stop_workers(workers: Res<Workers>) {
    for job in workers.running.values() {
        job.cancel();
    }
}
fn prepare_notices(world: &mut World, agent: Entity) {
    let Some(saved) = committed(world, agent).cloned() else {
        return;
    };
    for record in saved
        .evolution
        .jobs
        .iter()
        .filter(|j| j.terminal() && j.delivered.is_none() && !j.skipped)
    {
        let local = world.get::<DurableAgentState>(agent).unwrap();
        if !local.evolution.jobs.contains(record) {
            continue;
        }
        let Some(task) = local.task(&record.task) else {
            continue;
        };
        if task.generation != record.generation
            || matches!(
                task.status,
                Status::Completed | Status::Failed | Status::Cancelled
            )
        {
            let mut s = world.get_mut::<DurableAgentState>(agent).unwrap();
            s.evolution
                .jobs
                .iter_mut()
                .find(|j| j.id == record.id)
                .unwrap()
                .skipped = true;
            s.bump();
            continue;
        }
        if record.notice_step.is_none() && local.active.as_ref() != Some(&record.task) {
            let step = task.next_step;
            let mut s = world.get_mut::<DurableAgentState>(agent).unwrap();
            s.evolution
                .jobs
                .iter_mut()
                .find(|j| j.id == record.id)
                .unwrap()
                .notice_step = Some(step);
            s.task_mut(&record.task).unwrap().status = Status::Queued;
            s.bump();
            continue;
        }
        if record.notice_step.is_some() {
            if local
                .active
                .as_ref()
                .is_some_and(|active| active != &record.task)
            {
                continue;
            }
            let exists = world
                .query::<(&ForAgent, &Incoming)>()
                .iter(world)
                .any(|(a, i)| a.0 == agent && input_key(i) == Some(&record.id));
            if !exists {
                world.spawn((record.input(), ForAgent(agent)));
            }
        }
    }
}
fn input_key(input: &Incoming) -> Option<&str> {
    if input.author.kind
        != rig_harness::rig_core::conversation::AuthorKind::Plugin("vivy.autodream.job".into())
    {
        return None;
    }
    input.author.request.as_ref().map(|r| r.0.as_str())
}
pub(in crate::runtime) fn pending(state: &DurableAgentState) -> bool {
    state
        .evolution
        .jobs
        .iter()
        .any(|j| j.notice_step.is_some() && j.delivered.is_none() && !j.skipped)
}
pub(in crate::runtime) fn accepted(world: &mut World, agent: Entity) -> bool {
    let keys: Vec<_> = world
        .query_filtered::<(&ForAgent, &Incoming), With<Started>>()
        .iter(world)
        .filter(|(a, _)| a.0 == agent)
        .filter_map(|(_, i)| input_key(i).map(str::to_owned))
        .collect();
    world.get::<DurableAgentState>(agent).is_none_or(|s| {
        s.evolution
            .jobs
            .iter()
            .filter(|j| j.notice_step.is_some() && j.delivered.is_none() && !j.skipped)
            .all(|j| keys.contains(&j.id))
    })
}
pub(in crate::runtime) fn admit(
    world: &mut World,
    call: Entity,
    agent: Entity,
    input: &Incoming,
) -> bool {
    if input.command().is_some_and(|s| {
        matches!(
            s.split_whitespace().next(),
            Some("dream" | "python" | "setup")
        )
    }) {
        release(world, call);
        return true;
    }
    let Some(id) = input_key(input) else {
        return false;
    };
    hold(world, call);
    let valid = world.get::<DurableAgentState>(agent).is_some_and(|s| {
        s.evolution.jobs.iter().any(|j| {
            j.id == id
                && !j.skipped
                && j.delivered.is_none()
                && s.task(&j.task).is_some_and(|t| {
                    t.generation == j.generation
                        && !matches!(
                            t.status,
                            Status::Completed | Status::Failed | Status::Cancelled
                        )
                })
        })
    });
    if !valid {
        decide(
            world,
            call,
            Outcome::refused("stale or cancelled job result input"),
        );
        release(world, call);
        return true;
    }
    let Some(saved) = committed(world, agent)
        .and_then(|s| s.evolution.jobs.iter().find(|j| j.id == id))
        .cloned()
    else {
        return true;
    };
    if !healthy(world) {
        return true;
    }
    if saved.notice_step.is_none()
        || saved.delivered.is_some()
        || saved.skipped
        || digest(&saved.input()) != digest(input)
    {
        decide(world, call, Outcome::refused("stale job result input"));
        release(world, call);
        return true;
    }
    let mut state = world.get_mut::<DurableAgentState>(agent).unwrap();
    if state.active.is_none() {
        state.active = Some(saved.task.clone());
        let task = state.task_mut(&saved.task).unwrap();
        task.status = Status::Running;
        task.outcome = None;
        state.decision_pending = false;
        state.bump();
    }
    let job = state
        .evolution
        .jobs
        .iter_mut()
        .find(|j| j.id == id)
        .unwrap();
    if !job.ready {
        job.ready = true;
        state.bump();
    }
    drop(state);
    if committed(world, agent).is_some_and(|s| {
        s.active.is_some() && s.evolution.jobs.iter().any(|j| j.id == id && j.ready)
    }) {
        release(world, call);
    }
    true
}
fn delivered(
    e: On<Add<Outcome>>,
    inputs: Query<(&ForAgent, &Incoming, &Outcome)>,
    mut agents: Query<&mut DurableAgentState>,
) {
    let Ok((of, input, result)) = inputs.get(e.entity) else {
        return;
    };
    let Some(id) = input_key(input) else { return };
    if result.0.is_err() {
        return;
    }
    let Ok(mut s) = agents.get_mut(of.0) else {
        return;
    };
    let Some(task) = s.active.as_ref().and_then(|id| s.task(id)) else {
        return;
    };
    let delivery = wake::Delivery {
        task: task.id.clone(),
        step: s
            .operations
            .iter()
            .filter(|o| {
                o.task == task.id && o.kind == "model" && o.outcome.is_none() && !o.published
            })
            .map(|o| o.step)
            .min()
            .unwrap_or(task.next_step),
    };
    if let Some(job) = s
        .evolution
        .jobs
        .iter_mut()
        .find(|j| j.id == id && j.delivered.is_none())
    {
        job.delivered = Some(delivery);
        s.bump();
    }
}
pub(in crate::runtime) fn notices(
    state: &DurableAgentState,
    task: &str,
) -> Vec<(u64, String, Incoming)> {
    state
        .evolution
        .jobs
        .iter()
        .filter_map(|j| {
            j.delivered
                .as_ref()
                .filter(|d| d.task == task)
                .map(|d| (d.step, format!("{}/result", j.id), j.input()))
        })
        .collect()
}
