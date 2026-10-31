//! A branch summary when the user rewinds, as Pi makes one when it leaves a
//! branch. Rig's rewind (and `/fork` at an earlier message) forks the shown
//! agent just before a message; the messages after it stay with the agent
//! forked and the fork goes on without them. This plugin asks the model for
//! a structured summary of those messages, with Rig's own summarizer as a
//! request of its own, and adds it to the fork as a user message in Pi's
//! words, so the fork keeps what the branch it came back from learned.
//!
//! The summary call is a model-call effect in a turn of the fork's own, so
//! it is recorded, counted and can be stopped. A failed or stopped summary
//! adds nothing.

use std::sync::Arc;

use rig_harness::prelude::*;
use rig_harness::rig_core::completion::Message;
use rig_harness::rig_core::conversation::{Author, Entry};
use rig_harness::rig_ecs::fork::AgentForked;
use rig_harness::rig_memory::{CompactReason, Summarizer, SummaryPrompts};

/// Who adds the summary to the fork.
pub const AUTHOR: &str = "branch-summary";

/// Pi's words around a branch summary (`BRANCH_SUMMARY_PREFIX`/`SUFFIX`).
const PREFIX: &str =
    "The following is a summary of a branch that this conversation came back from:\n\n<summary>\n";
const SUFFIX: &str = "</summary>";

/// What the summarizer is asked, in place of Rig's compaction request.
const INITIAL: &str = "The conversation above is a branch the user left by going back to an \
earlier point. Write a structured checkpoint of it, so the conversation that continues from that \
earlier point keeps what this branch did and learned.";

/// Summarizes what a rewind leaves behind into the fork.
#[derive(Default)]
pub struct BranchSummaryPlugin;

impl Plugin for BranchSummaryPlugin {
    fn build(&self, app: &mut App) {
        app.add_observer(summarize_left_branch)
            .add_observer(take_branch_summary);
    }
}

/// On the summary call: the fork it goes into and how many messages it
/// summarizes.
#[derive(Component, Clone, Debug)]
pub struct BranchSummary {
    pub fork: Entity,
    pub messages: usize,
}

/// The messages of the agent forked that its fork, `kept` messages long,
/// left behind. A fork copies up to a turn boundary at or before the
/// message it was asked through, so its own length is where it cut.
pub fn left_behind<T: Clone>(messages: &[T], kept: usize) -> Vec<T> {
    messages.get(kept..).unwrap_or_default().to_vec()
}

/// The user message that carries `summary` into the fork.
pub fn summary_message(summary: &str) -> String {
    format!("{PREFIX}{}\n{SUFFIX}", summary.trim())
}

fn summarizer() -> Summarizer {
    Summarizer {
        prompts: SummaryPrompts {
            initial: INITIAL.into(),
            ..SummaryPrompts::DEFAULT
        },
        ..Summarizer::DEFAULT
    }
}

fn summarize_left_branch(
    forked: On<AgentForked>,
    agents: Query<(&Conversation, Option<&Connection>)>,
    mut commands: Commands,
    mut notices: MessageWriter<Notice>,
) {
    let (from, fork) = (forked.from, forked.fork);
    let (Ok((conversation, connection)), Ok((kept, _))) = (agents.get(from), agents.get(fork))
    else {
        return;
    };
    let older: Vec<Arc<Message>> = left_behind(conversation.messages(), kept.messages().len());
    if older.is_empty() {
        return;
    }
    let Some(connection) = connection.cloned() else {
        notices.write(Notice::info(
            fork,
            "No branch summary: the agent has no model.",
        ));
        return;
    };
    let request =
        match summarizer().request(&older, "", &CompactReason::Threshold, &connection.spec) {
            Ok(request) => request,
            Err(why) => {
                notices.write(Notice::error(fork, format!("No branch summary: {why}.")));
                return;
            }
        };
    let turn = commands
        .spawn((Name::new("branch summary"), TurnOf(fork)))
        .id();
    commands.spawn((
        Name::new("branch summary call"),
        ModelRequest {
            connection: Some(connection),
            ..ModelRequest::new(request)
        },
        BranchSummary {
            fork,
            messages: older.len(),
        },
        ForAgent(fork),
        CallOf(turn),
        Effect::default().with_purpose("branch summary"),
    ));
}

fn take_branch_summary(
    settled: On<Add<Settled>>,
    calls: Query<(&BranchSummary, &Outcome, &CallOf)>,
    ended: Query<(), With<TurnOutcome>>,
    mut agents: Query<&mut Conversation>,
    mut commands: Commands,
    mut notices: MessageWriter<Notice>,
) {
    let Ok((summary, outcome, &CallOf(turn))) = calls.get(settled.entity) else {
        return;
    };
    let fork = summary.fork;
    let text = match (&outcome.0, outcome.completion()) {
        _ if ended.contains(turn) => Err("it was stopped".to_owned()),
        (Err(report), _) => Err(report.to_string()),
        (Ok(_), None) => Err("the model did not answer with a completion".to_owned()),
        (Ok(_), Some(response)) => {
            Summarizer::summary_text(response).map_err(|why| why.to_string())
        }
    };
    match (text, agents.get_mut(fork)) {
        (Ok(text), Ok(mut conversation)) => {
            let entry =
                Entry::new(Message::user(summary_message(&text))).by(Author::plugin(AUTHOR));
            conversation.push(entry);
            let plural = if summary.messages == 1 { "" } else { "s" };
            let note = format!(
                "Summarized the branch left behind ({} message{plural}) into this one.",
                summary.messages
            );
            notices.write(Notice::info(fork, note));
        }
        (Err(why), _) => {
            notices.write(Notice::error(
                fork,
                format!("Branch summary failed: {why}."),
            ));
        }
        (Ok(_), Err(_)) => {}
    }
    if !ended.contains(turn) {
        let done = TurnOutcome::Stopped(Some("branch summarized".to_owned()));
        commands.entity(turn).try_insert(done);
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn leaves_behind_what_follows_the_fork_point() {
        let messages = ["a", "b", "c", "d"];
        assert_eq!(left_behind(&messages, 2), ["c", "d"]);
        assert!(
            left_behind(&messages, 4).is_empty(),
            "a fork at the end leaves nothing"
        );
        assert!(left_behind(&messages, 9).is_empty());
    }

    #[test]
    fn carries_the_summary_in_pis_words() {
        let text = summary_message("  ## Goal\nFix it.\n ");
        assert_eq!(
            text,
            "The following is a summary of a branch that this conversation came back from:\n\n\
             <summary>\n## Goal\nFix it.\n</summary>"
        );
    }

    #[test]
    fn asks_for_a_branch_checkpoint_on_its_own() {
        use rig_harness::rig_core::catalog::ModelSpec;
        use rig_harness::rig_core::providers::registry::ProviderId;
        let spec = ModelSpec::new(ProviderId::declare("test").unwrap(), "m");
        let older = vec![Arc::new(Message::user("try the other parser"))];
        let request = summarizer()
            .request(&older, "", &CompactReason::Threshold, &spec)
            .unwrap();
        let sent = format!("{request:?}");
        assert!(sent.contains("branch the user left"), "{sent}");
        assert!(sent.contains("try the other parser"));
    }
}
