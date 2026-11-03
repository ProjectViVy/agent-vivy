//! First-run persona extraction: one completion turns an introduction into the
//! six Laputa files, with a neutral fallback when the description is unclear.
use crate::PersonaStore;
use crate::persona::{
    ASSEMBLY_FILES, IDENTITY, MAX_FILE_BYTES, MAX_PERSONA_BYTES, MISSION, REDLINE, default_body,
    defaults,
};
use rig_harness::prelude::*;
use rig_harness::rig_core::{
    completion::{CompletionRequest, Message},
    effect::{Answered, EffectId, EffectKind},
    message::AssistantMessage,
    serve::{Dispatch, Reply},
};
use serde_json::Value;
use std::{
    collections::BTreeMap,
    io,
    time::{SystemTime, UNIX_EPOCH},
};

pub const PROMPT: &str = include_str!("../../../prompts/ONBOARDING.md");

const NEUTRAL_NAMES: [&str; 12] = [
    "Aster", "Lumen", "Mira", "Nova", "Orin", "Sable", "Tessa", "Wren", "Iris", "Kestrel", "Juno",
    "Vela",
];

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Draft {
    pub name: String,
    pub files: BTreeMap<String, String>,
    /// The model could not describe a usable persona; defaults were supplied.
    pub fallback: bool,
}

/// One tool-free, session-free completion over the connected model.
pub async fn extract(connection: Connection, description: &str) -> Result<Draft, String> {
    let mut request =
        CompletionRequest::from(vec![Message::system(PROMPT), Message::user(description)]);
    request.max_tokens = Some(4096);
    let reply = connection
        .handler
        .0
        .handle(
            EffectKind::Completion {
                request: request.into(),
                stream: false,
            },
            Dispatch::new(EffectId::from_raw(1), false),
        )
        .await;
    let response = match reply {
        Reply::Outcome(Ok(Answered::Completion(response))) => response,
        Reply::Outcome(Err(error)) => return Err(error.to_string()),
        Reply::Stream(_) => return Err("provider returned an unexpected stream".into()),
        Reply::Outcome(Ok(_)) => return Err("provider returned no completed draft".into()),
    };
    let text = AssistantMessage::new(response.choice.clone()).text();
    let draft = parse(&text).unwrap_or_else(fallback);
    let mut total = 0;
    for (name, body) in &draft.files {
        if body.len() > MAX_FILE_BYTES {
            return Err(format!("{name} exceeds the per-file byte budget"));
        }
        total += body.len();
    }
    if total > MAX_PERSONA_BYTES {
        return Err("the persona draft exceeds the assembled byte budget".into());
    }
    Ok(draft)
}

/// Tolerates code fences and `.MD`/case variations; unusable drafts fall back.
pub fn parse(reply: &str) -> Option<Draft> {
    let start = reply.find('{')?;
    let end = reply.rfind('}')?;
    let value: Value = serde_json::from_str(reply.get(start..=end)?).ok()?;
    let object = value.as_object()?;
    if object.get("sufficient").and_then(Value::as_bool) == Some(false) {
        return Some(fallback());
    }
    let name = object
        .get("name")
        .and_then(Value::as_str)
        .map(str::trim)
        .filter(|name| !name.is_empty())
        .map(|name| name.chars().take(64).collect::<String>())?;
    let mut files = defaults();
    for (key, body) in object {
        let Some(name) = normalize(key) else {
            continue;
        };
        let Some(body) = body.as_str() else {
            continue;
        };
        files.insert(name, body.to_string());
    }
    if files.get(MISSION).is_none_or(|text| text.trim().is_empty())
        || files
            .get(IDENTITY)
            .is_none_or(|text| text.trim().is_empty())
    {
        return Some(fallback());
    }
    files.insert(REDLINE.into(), default_body(REDLINE).into());
    Some(Draft {
        name,
        files,
        fallback: false,
    })
}

fn normalize(key: &str) -> Option<String> {
    let upper = key.trim().to_ascii_uppercase();
    let stem = upper.strip_suffix(".MD").unwrap_or(&upper);
    let name = format!("{stem}.MD");
    ASSEMBLY_FILES
        .contains(&name.as_str())
        .then_some(name)
        .filter(|name| name != REDLINE)
}

/// A random neutral name with the compiled default persona.
pub fn fallback() -> Draft {
    let name = random_name();
    let mut files = defaults();
    files.insert(
        IDENTITY.into(),
        default_body(IDENTITY).replacen("Vivy", name, 1),
    );
    Draft {
        name: name.into(),
        files,
        fallback: true,
    }
}

fn random_name() -> &'static str {
    let nanos = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|elapsed| elapsed.as_nanos())
        .unwrap_or_default();
    NEUTRAL_NAMES[(nanos as usize ^ std::process::id() as usize) % NEUTRAL_NAMES.len()]
}

/// Publishes revision one, or returns an identical earlier attempt for the same
/// update ID. An existing persona is never overwritten without `force`.
pub fn write(
    store: &PersonaStore,
    draft: &Draft,
    id: &str,
    force: bool,
) -> io::Result<crate::Persona> {
    if let Some(prior) = store.update(id)? {
        return Ok(prior);
    }
    if store.initialized()? && !force {
        return Err(io::Error::new(
            io::ErrorKind::AlreadyExists,
            "a persona is already initialized",
        ));
    }
    let current = store.read()?;
    store.save_all(
        current.revision,
        &current.last_update_id,
        id,
        &draft.files,
        vec!["onboarding".into()],
        Some(&draft.name),
    )
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::persona::{DARK, DREAM, RELATIONSHIP};

    #[test]
    fn fenced_flat_draft_keeps_default_redline_and_optional_files() {
        let reply = r#"Here you go:
```json
{"sufficient": true, "name": "Nova",
 "mission": "Help me think.",
 "IDENTITY.MD": "I am Nova.",
 "relationship": "We work side by side."}
```"#;
        let draft = parse(reply).unwrap();
        assert!(!draft.fallback);
        assert_eq!(draft.name, "Nova");
        assert_eq!(draft.files[MISSION], "Help me think.");
        assert_eq!(draft.files[IDENTITY], "I am Nova.");
        assert_eq!(draft.files[RELATIONSHIP], "We work side by side.");
        assert_eq!(draft.files[REDLINE], default_body(REDLINE));
        assert!(draft.files[DREAM].is_empty());
        assert!(draft.files[DARK].is_empty());
    }

    #[test]
    fn insufficient_or_unusable_replies_fall_back() {
        for reply in [
            r#"{"sufficient": false}"#,
            r#"{"sufficient": true, "name": "", "MISSION.MD": "M", "IDENTITY.MD": "I"}"#,
            r#"{"sufficient": true, "name": "Nova", "MISSION.MD": " ", "IDENTITY.MD": "I"}"#,
            r#"not json"#,
        ] {
            assert!(parse(reply).is_none_or(|draft| draft.fallback), "{reply}");
        }
    }

    #[test]
    fn fallback_uses_a_neutral_name_and_the_default_mission() {
        let draft = fallback();
        assert!(draft.fallback);
        assert!(NEUTRAL_NAMES.contains(&draft.name.as_str()));
        assert_eq!(draft.files[MISSION], default_body(MISSION));
        assert!(draft.files[IDENTITY].contains(&draft.name));
        assert!(!draft.files[IDENTITY].contains("Vivy"));
        assert_eq!(draft.files[REDLINE], default_body(REDLINE));
    }
}
