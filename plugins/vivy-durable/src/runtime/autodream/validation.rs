//! Saved ownership and receipt invariants. Invalid state is refused before replay.
use super::*;
pub(in crate::runtime) fn validate(s: &DurableAgentState) -> io::Result<()> {
    let e = &s.evolution;
    let hash = |v: &str| v.len() == 64 && v.bytes().all(|b| b.is_ascii_hexdigit());
    if e.dream == e.foreground.is_empty()
        || (!e.dream && (!e.jobs.is_empty() || !e.windows.is_empty() || !e.evidence.is_empty()))
        || (e.dream && !e.requests.is_empty())
    {
        return Err(invalid("invalid Dream ownership"));
    }
    let mut keys = std::collections::BTreeSet::new();
    for (n, r) in e.requests.iter().enumerate() {
        if r.id != format!("dream:{}", n + 1)
            || r.keys.is_empty()
            || r.keys.len() > 32
            || r.keys.len() != r.reasons.len()
            || r.keys.iter().any(|k| k.is_empty() || !keys.insert(k))
            || r.reasons.iter().map(String::len).sum::<usize>() > 16 * 1024
        {
            return Err(invalid("invalid Dream request identity or size"));
        }
    }
    for (id, item) in &e.evidence {
        if id != &item.id
            || !hash(&item.digest)
            || item.path.is_empty()
            || !["input", "tool"].contains(&item.kind.as_str())
        {
            return Err(invalid("invalid evidence identity"));
        }
    }
    let mut windows = std::collections::BTreeSet::new();
    for w in &e.windows {
        if !windows.insert(&w.request)
            || w.sources.len() > 128
            || w.sources.iter().any(|id| !e.evidence.contains_key(id))
            || !s
                .task(&w.task)
                .is_some_and(|t| t.source == "vivy.autodream" && t.request_key == w.request)
        {
            return Err(invalid("invalid Dream evidence window"));
        }
    }
    if e.jobs.iter().filter(|j| !j.terminal()).count() > 1 {
        return Err(invalid("multiple live validation jobs"));
    }
    let mut updates = std::collections::BTreeSet::new();
    for (n, j) in e.jobs.iter().enumerate() {
        let task = s
            .task(&j.task)
            .ok_or_else(|| invalid("validation job owner is missing"))?;
        if j.id != format!("job:{}", n + 1)
            || j.generation > task.generation
            || j.update.is_empty()
            || !updates.insert(&j.update)
            || !s.operations.iter().any(|op| {
                op.id == j.operation && op.task == j.task && op.generation == j.generation
            })
            || j.evidence.iter().any(|id| !e.evidence.contains_key(id))
        {
            return Err(invalid("invalid validation job ownership or identity"));
        }
        if ![
            "admitted",
            "waiting_setup",
            "dispatched",
            "validated",
            "publishing",
            "published",
            "failed",
            "stale",
            "cancelled",
            "unknown",
        ]
        .contains(&j.phase.as_str())
        {
            return Err(invalid("invalid validation job phase"));
        }
        match j.target.as_str() {
            "skill" => {
                if j.persona.is_some()
                    || !j.skill.as_ref().is_some_and(|s| hash(&s.revision))
                    || !j.environment.as_ref().is_some_and(|p| hash(&p.id))
                    || !hash(&j.expected_base)
                {
                    return Err(invalid("invalid frozen Skill candidate"));
                }
                j.checks.validate()?;
                if matches!(j.phase.as_str(), "validated" | "publishing" | "published") {
                    let receipt: vivy_python::managed::ValidationReceipt =
                        serde_json::from_value(j.result.clone().unwrap_or(Value::Null))
                            .map_err(io::Error::other)?;
                    if !receipt.passed
                        || receipt.checks != j.checks.digest()
                        || receipt.revision != j.skill.as_ref().unwrap().revision
                        || receipt.environment != j.environment.as_ref().unwrap().environment()
                    {
                        return Err(invalid("invalid committed Skill validation receipt"));
                    }
                }
            }
            "persona" => {
                if j.skill.is_some()
                    || j.environment.is_some()
                    || j.file
                        .as_deref()
                        .is_none_or(|file| !vivy_laputa::auto(file))
                    || j.file_digest
                        .as_deref()
                        .is_none_or(|digest| digest.is_empty())
                    || j.persona.as_ref().is_none_or(|p| {
                        p.trim().is_empty() || p.len() > vivy_laputa::MAX_FILE_BYTES
                    })
                    || j.expected_base.parse::<u64>().is_err()
                {
                    return Err(invalid("invalid frozen persona candidate"));
                }
            }
            _ => return Err(invalid("invalid candidate target")),
        }
        if (j.phase == "published" && j.adoption.is_none())
            || (j.notice_step.is_some() && !j.terminal())
            || (j.ready && j.notice_step.is_none())
            || j.delivered
                .as_ref()
                .is_some_and(|d| !j.ready || s.task(&d.task).is_none())
        {
            return Err(invalid("invalid job adoption or delivery receipt"));
        }
    }
    for op in &s.operations {
        if op.publication.as_ref().is_some_and(|p| !hash(p))
            || op.usage.len() > 1024
            || (!op.usage.is_empty() && (op.kind != "tool" || op.publication.is_none()))
        {
            return Err(invalid("invalid pinned publication or callable usage"));
        }
        for event in &op.usage {
            if event["publication"].as_str() != op.publication.as_deref()
                || !event["revision"].as_str().is_some_and(hash)
            {
                return Err(invalid(
                    "usage differs from the operation's selected publication",
                ));
            }
        }
    }
    Ok(())
}
