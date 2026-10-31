//! `/session`, `/copy` and `/export`, as Pi has them, on the conversation
//! of the agent the command was typed for. Tokens and cost are Rig's
//! `/usage`; Pi's `/clone` and `/tree` are Rig's `/fork`, `/agents` and
//! rewind.

use std::fmt::Write as _;
use std::path::Path;

use rig_harness::harness_protocol::SessionId;
use rig_harness::host::cli::Cli;
use rig_harness::prelude::*;
use rig_harness::rig_core::completion::message::{
    AssistantContent, Message, ToolResultContent, UserContent,
};
use rig_harness::rig_core::conversation::{self, AuthorKind, EntryRef};
use serde::{Deserialize, Serialize};

/// `/session`: the session's name, id and directory, the agent's model, and
/// how many messages and tool calls its conversation holds.
#[derive(Default)]
pub struct SessionInfoPlugin;

impl Plugin for SessionInfoPlugin {
    fn build(&self, app: &mut App) {
        app.add_command(
            "session",
            "Show this session's name, id, model and message counts",
            session,
        );
    }
}

/// `/copy`: the agent's last reply to the clipboard.
#[derive(Default)]
pub struct CopyPlugin;

impl Plugin for CopyPlugin {
    fn build(&self, app: &mut App) {
        app.add_command("copy", "Copy the last reply to the clipboard", copy);
    }
}

/// `/export [path]`: the conversation as HTML, or JSONL for a `.jsonl`
/// path; by default `session-<id>.html` in the agent's directory.
#[derive(Default)]
pub struct ExportPlugin;

impl Plugin for ExportPlugin {
    fn build(&self, app: &mut App) {
        app.add_command(
            "export",
            "Save the conversation as HTML, or JSONL for a .jsonl path: /export [path]",
            export,
        );
    }
}

/// `/clone`: this session copied into a new one, which the agent switches
/// to, as Pi's `/clone` duplicates the session at its current position.
#[derive(Default)]
pub struct ClonePlugin;

impl Plugin for ClonePlugin {
    fn build(&self, app: &mut App) {
        app.add_command(
            "clone",
            "Copy this session into a new one and switch to it",
            clone,
        );
    }
}

/// `/import <file.jsonl>`: the messages of an `/export` JSONL file into
/// this session's shown agent, which must have none yet (`/new` first).
#[derive(Default)]
pub struct ImportPlugin;

impl Plugin for ImportPlugin {
    fn build(&self, app: &mut App) {
        app.add_command(
            "import",
            "Load an /export JSONL file into this empty session: /import <file.jsonl>",
            import,
        );
    }
}

/// `/tree`: Pi's session tree is Rig's agents of this session (each fork
/// and rewind makes one), so it opens `/agents`.
#[derive(Default)]
pub struct TreePlugin;

impl Plugin for TreePlugin {
    fn build(&self, app: &mut App) {
        app.add_command(
            "tree",
            "Show the branches of this session (its agents)",
            tree,
        );
    }
}

fn tree(In(args): In<CommandArgs>, mut commands: Commands) {
    commands.trigger(RunCommand::new(
        args.agent,
        format!("agents {}", args.args).trim_end(),
    ));
}

/// Copies the session directory `from` to `to`, without what belongs to
/// the process running it: locks, a pending switch, its name.
pub fn copy_session(from: &Path, to: &Path) -> std::io::Result<()> {
    vivy_process::session::require_ordinary_history(from)?;
    const LEFT_OUT: [&str; 5] = [".rig-memory.lock", "lock", "switch", "deleting", "name"];
    std::fs::create_dir_all(to)?;
    for entry in std::fs::read_dir(from)? {
        let entry = entry?;
        let name = entry.file_name();
        if LEFT_OUT.iter().any(|left| name == *left) {
            continue;
        }
        let target = to.join(&name);
        if entry.file_type()?.is_dir() {
            copy_session(&entry.path(), &target)?;
        } else {
            std::fs::copy(entry.path(), target)?;
        }
    }
    Ok(())
}

fn clone(
    In(args): In<CommandArgs>,
    turns: Query<(), With<TurnOf>>,
    paths: Option<Res<SessionPaths>>,
    home: Res<RigHome>,
    mut commands: Commands,
    mut notices: MessageWriter<Notice>,
) {
    if !turns.is_empty() {
        notices.write(Notice::info(
            args.agent,
            "A turn is running. Press Esc to stop it first.",
        ));
        return;
    }
    let Some(paths) = paths else {
        notices.write(Notice::error(
            args.agent,
            "This run keeps no session to clone.",
        ));
        return;
    };
    let copy = SessionId::generate();
    let to = home.0.session(&copy);
    match copy_session(paths.0.path(), to.path()) {
        Ok(()) => {
            notices.write(Notice::info(
                args.agent,
                format!("Cloned into session {copy}."),
            ));
            commands.trigger(Exit(ExitReason::Switch(Some(copy))));
        }
        Err(why) => {
            let _ = std::fs::remove_dir_all(to.path());
            notices.write(Notice::error(
                args.agent,
                format!("Could not clone the session: {why}."),
            ));
        }
    }
}

/// One line of an `/export` JSONL file, as [`jsonl`] writes it.
#[derive(Deserialize)]
struct ImportedLine {
    #[serde(default)]
    author: Option<conversation::Author>,
    message: Message,
}

/// The messages of an `/export` JSONL `text`, or the line that does not read.
pub fn read_jsonl(text: &str) -> Result<Vec<conversation::Entry>, String> {
    text.lines()
        .enumerate()
        .filter(|(_, line)| !line.trim().is_empty())
        .map(|(at, line)| {
            let line: ImportedLine =
                serde_json::from_str(line).map_err(|why| format!("line {}: {why}", at + 1))?;
            let entry = conversation::Entry::new(line.message);
            Ok(match line.author {
                Some(author) => entry.by(author),
                None => entry,
            })
        })
        .collect()
}

fn import(
    In(args): In<CommandArgs>,
    mut agents: Query<(&mut Conversation, Option<&WorkDir>)>,
    paths: Option<Res<SessionPaths>>,
    root: Option<Res<ProjectRoot>>,
    mut notices: MessageWriter<Notice>,
) {
    if let Some(paths) = paths {
        if let Err(error) = vivy_process::session::require_ordinary_history(paths.path()) {
            notices.write(Notice::error(args.agent, error.to_string()));
            return;
        }
    }
    let Ok((mut conversation, workdir)) = agents.get_mut(args.agent) else {
        return;
    };
    let given = args.args.trim();
    if given.is_empty() {
        notices.write(Notice::error(
            args.agent,
            "/import needs a file: /import <file.jsonl>",
        ));
        return;
    }
    if conversation.entries().len() > 0 {
        notices.write(Notice::error(
            args.agent,
            "/import goes into an empty session: run /new first.",
        ));
        return;
    }
    let base = workdir
        .map(|dir| dir.0.clone())
        .or_else(|| root.map(|root| root.0.clone()))
        .unwrap_or_default();
    let path = base.join(given);
    let entries = std::fs::read_to_string(&path)
        .map_err(|why| why.to_string())
        .and_then(|text| read_jsonl(&text));
    match entries {
        Ok(entries) => {
            let count = entries.len();
            for entry in entries {
                conversation.push(entry);
            }
            notices.write(Notice::info(
                args.agent,
                format!("Imported {count} messages from {}.", path.display()),
            ));
        }
        Err(why) => {
            notices.write(Notice::error(
                args.agent,
                format!("Could not import {}: {why}.", path.display()),
            ));
        }
    }
}

/// How many messages of each kind a conversation holds.
#[derive(Debug, Default, PartialEq, Eq)]
pub struct Counts {
    pub user: usize,
    pub assistant: usize,
    pub tool_calls: usize,
    pub tool_results: usize,
    pub total: usize,
}

impl Counts {
    pub fn of(conversation: &conversation::Conversation) -> Self {
        let mut counts = Self::default();
        for entry in conversation.entries() {
            counts.total += 1;
            match entry.message {
                Message::User { content } => {
                    // The user's, typed or sent through a template or skill.
                    let users = entry.author.is_none_or(|a| a.kind == AuthorKind::User);
                    let typed = content
                        .iter()
                        .any(|c| !matches!(c, UserContent::ToolResult(_)));
                    if users && typed {
                        counts.user += 1;
                    }
                    counts.tool_results += content
                        .iter()
                        .filter(|c| matches!(c, UserContent::ToolResult(_)))
                        .count();
                }
                Message::Assistant(reply) => {
                    counts.assistant += 1;
                    counts.tool_calls += reply.tool_calls().count();
                }
                Message::System { .. } => {}
            }
        }
        counts
    }
}

fn session(
    In(args): In<CommandArgs>,
    agents: Query<(&Conversation, Option<&ModelChoice>)>,
    paths: Option<Res<SessionPaths>>,
    mut notices: MessageWriter<Notice>,
) {
    let Ok((conversation, model)) = agents.get(args.agent) else {
        return;
    };
    let mut lines = Vec::new();
    match paths {
        Some(paths) => {
            let dir = &paths.0;
            if let Some(name) = dir.name() {
                lines.push(format!("Name: {name}"));
            }
            if let Some(id) = dir.id() {
                lines.push(format!("ID: {id}"));
            }
            lines.push(format!("Directory: {}", dir.path().display()));
        }
        None => lines.push("Session: not saved".to_owned()),
    }
    let model = model.map_or("none chosen", |model| model.0.as_str());
    lines.push(format!("Model: {model}"));
    let c = Counts::of(conversation);
    lines.push(format!(
        "Messages: {} ({} from you, {} replies); tools: {} calls, {} results",
        c.total, c.user, c.assistant, c.tool_calls, c.tool_results
    ));
    lines.push("Tokens and cost: /usage".to_owned());
    notices.write(Notice::info(args.agent, lines.join("\n")));
}

/// The text of the last reply that has any.
pub fn last_reply(conversation: &conversation::Conversation) -> Option<String> {
    conversation
        .entries()
        .rev()
        .find_map(|entry| match entry.message {
            Message::Assistant(reply) => Some(reply.text()).filter(|text| !text.trim().is_empty()),
            _ => None,
        })
}

fn copy(
    In(args): In<CommandArgs>,
    agents: Query<&Conversation>,
    cli: Option<Res<Cli>>,
    processes: Option<Res<Processes>>,
    #[cfg(feature = "tui")] mut terminal: MessageWriter<rig_tui::CopyToClipboard>,
    mut notices: MessageWriter<Notice>,
) {
    let Some(text) = agents.get(args.agent).ok().and_then(|c| last_reply(c)) else {
        notices.write(Notice::error(args.agent, "No reply to copy yet."));
        return;
    };
    let chars = text.chars().count();
    // The terminal view copies by OSC 52 as well as to this machine's
    // clipboard, so it works over SSH too.
    #[cfg(feature = "tui")]
    if cli.as_deref().and_then(Cli::front) == Some(rig_tui::FRONT) {
        terminal.write(rig_tui::CopyToClipboard(text));
        notices.write(Notice::info(
            args.agent,
            format!("Copied the last reply ({chars} characters)."),
        ));
        return;
    }
    #[cfg(not(feature = "tui"))]
    let _ = cli;
    let launcher = processes.map(|p| p.launcher()).unwrap_or_default();
    match rig_harness::rig_tools::clipboard::write_text(&launcher, &text) {
        Ok(()) => notices.write(Notice::info(
            args.agent,
            format!("Copied the last reply ({chars} characters)."),
        )),
        Err(why) => notices.write(Notice::error(args.agent, format!("Could not copy: {why}."))),
    };
}

fn export(
    In(args): In<CommandArgs>,
    agents: Query<(&Conversation, Option<&ModelChoice>, Option<&WorkDir>)>,
    root: Option<Res<ProjectRoot>>,
    paths: Option<Res<SessionPaths>>,
    mut notices: MessageWriter<Notice>,
) {
    let Ok((conversation, model, workdir)) = agents.get(args.agent) else {
        return;
    };
    let id = paths
        .as_ref()
        .and_then(|paths| paths.0.id())
        .map_or_else(|| "unsaved".to_owned(), |id| id.to_string());
    let base = workdir
        .map(|dir| dir.0.clone())
        .or_else(|| root.map(|root| root.0.clone()))
        .unwrap_or_default();
    let path = match args.args.trim() {
        "" => base.join(format!("session-{id}.html")),
        given => base.join(given),
    };
    let model = model.map(|model| model.0.as_str());
    let written = if path.extension().is_some_and(|ext| ext == "jsonl") {
        jsonl(conversation).and_then(|text| write(&path, &text))
    } else {
        write(&path, &html(conversation, &id, model))
    };
    match written {
        Ok(()) => notices.write(Notice::info(
            args.agent,
            format!("Exported the session to {}.", path.display()),
        )),
        Err(why) => notices.write(Notice::error(
            args.agent,
            format!("Could not export to {}: {why}.", path.display()),
        )),
    };
}

fn write(path: &Path, text: &str) -> Result<(), String> {
    if let Some(parent) = path.parent().filter(|p| !p.as_os_str().is_empty()) {
        std::fs::create_dir_all(parent).map_err(|error| error.to_string())?;
    }
    std::fs::write(path, text).map_err(|error| error.to_string())
}

/// One line per entry: its id, when it was made, who wrote it when not
/// the user or the model, and the message as rig-core serializes it.
pub fn jsonl(conversation: &conversation::Conversation) -> Result<String, String> {
    #[derive(Serialize)]
    struct Line<'a> {
        id: String,
        made: u64,
        #[serde(skip_serializing_if = "Option::is_none")]
        author: Option<&'a conversation::Author>,
        message: &'a Message,
    }
    let mut text = String::new();
    for entry in conversation.entries() {
        let line = Line {
            id: entry.id.to_string(),
            made: entry.made,
            author: entry.author,
            message: entry.message,
        };
        text.push_str(&serde_json::to_string(&line).map_err(|error| error.to_string())?);
        text.push('\n');
    }
    Ok(text)
}

/// A self-contained page of the conversation: the user's messages, the
/// replies, tool calls with their arguments and results, in order.
pub fn html(conversation: &conversation::Conversation, id: &str, model: Option<&str>) -> String {
    let mut body = String::new();
    for entry in conversation.entries() {
        entry_html(&mut body, &entry);
    }
    let model = model
        .map(|m| format!(" · {}", escape(m)))
        .unwrap_or_default();
    format!(
        "<!doctype html>\n<html lang=\"en\"><head><meta charset=\"utf-8\">\
         <meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\
         <title>Session {id}</title><style>{STYLE}</style></head>\n<body>\
         <header><h1>Session {id}</h1><p>{count} messages{model}</p></header>\n{body}</body></html>\n",
        id = escape(id),
        count = conversation.entries().len(),
    )
}

const STYLE: &str = "body{font:15px/1.5 system-ui,sans-serif;max-width:860px;margin:0 auto;padding:16px;\
color:#1d1d1f;background:#fff}header p{color:#666}section{margin:12px 0;padding:10px 14px;\
border-radius:8px;border:1px solid #e3e3e3}.user{background:#f2f6ff}.tool{background:#f7f7f7}\
h2{font-size:12px;text-transform:uppercase;letter-spacing:.05em;color:#666;margin:0 0 6px}\
pre{white-space:pre-wrap;word-break:break-word;margin:6px 0;font:13px/1.45 ui-monospace,monospace}\
.error h2{color:#b00020}@media(prefers-color-scheme:dark){body{color:#e8e8e8;background:#161616}\
section{border-color:#333}.user{background:#1c2433}.tool{background:#202020}header p,h2{color:#999}}";

fn entry_html(out: &mut String, entry: &EntryRef<'_>) {
    let by = entry.author.and_then(|author| match &author.kind {
        AuthorKind::User => author.title.clone(),
        AuthorKind::Plugin(name) => Some(format!("plugin {name}")),
        AuthorKind::Agent => Some("agent".to_owned()),
        AuthorKind::Rig => Some("rig".to_owned()),
    });
    let label = |base: &str| match &by {
        Some(by) => format!("{base} · {}", escape(by)),
        None => base.to_owned(),
    };
    match entry.message {
        Message::System { content } => section(out, "tool", &label("System"), content),
        Message::User { content } => {
            for part in content {
                match part {
                    UserContent::Text(text) => section(out, "user", &label("User"), &text.text),
                    UserContent::ToolResult(result) => {
                        let text: Vec<String> = result
                            .content
                            .iter()
                            .map(|c| match c {
                                ToolResultContent::Text(text) => text.text.clone(),
                                ToolResultContent::Json { value } => value.to_string(),
                                _ => "[image]".to_owned(),
                            })
                            .collect();
                        let class = if result.is_error {
                            "tool error"
                        } else {
                            "tool"
                        };
                        let title = format!("Result · {}", escape(result.name.as_str()));
                        section(out, class, &title, &text.join("\n"));
                    }
                    _ => section(out, "user", &label("User"), "[attachment]"),
                }
            }
        }
        Message::Assistant(reply) => {
            for part in &reply.content {
                match part {
                    AssistantContent::Text(text) => {
                        section(out, "assistant", &label("Assistant"), &text.text)
                    }
                    AssistantContent::ToolCall(call) => {
                        let args = serde_json::to_string_pretty(&call.function.arguments)
                            .unwrap_or_default();
                        let title = format!("Tool call · {}", escape(call.function.name.as_str()));
                        section(out, "tool", &title, &args);
                    }
                    _ => {}
                }
            }
        }
    }
}

fn section(out: &mut String, class: &str, title: &str, text: &str) {
    let _ = writeln!(
        out,
        "<section class=\"{class}\"><h2>{title}</h2><pre>{}</pre></section>",
        escape(text)
    );
}

fn escape(text: &str) -> String {
    let mut out = String::with_capacity(text.len());
    for c in text.chars() {
        match c {
            '&' => out.push_str("&amp;"),
            '<' => out.push_str("&lt;"),
            '>' => out.push_str("&gt;"),
            '"' => out.push_str("&quot;"),
            '\'' => out.push_str("&#39;"),
            c => out.push(c),
        }
    }
    out
}

#[cfg(test)]
mod tests {
    use super::*;
    use rig_harness::rig_core::completion::message::AssistantMessage;

    fn conversation() -> conversation::Conversation {
        let mut conversation = conversation::Conversation::default();
        conversation.push(Message::user("Make <b>hello</b> & go"));
        conversation.push(
            conversation::Entry::new(Message::user("From a template"))
                .by(conversation::Author::user().titled("template t")),
        );
        conversation.push(Message::Assistant(AssistantMessage::new(vec![
            AssistantContent::text("Done: it's <ok>."),
        ])));
        conversation
    }

    #[test]
    fn counts_and_last_reply() {
        let conversation = conversation();
        let counts = Counts::of(&conversation);
        assert_eq!((counts.total, counts.user, counts.assistant), (3, 2, 1));
        assert_eq!(
            last_reply(&conversation).as_deref(),
            Some("Done: it's <ok>.")
        );
        assert_eq!(last_reply(&conversation::Conversation::default()), None);
    }

    #[test]
    fn exports_escaped_html() {
        let page = html(&conversation(), "1-2", Some("gateway/m<x>"));
        assert!(page.contains("Make &lt;b&gt;hello&lt;/b&gt; &amp; go"));
        assert!(page.contains("Done: it&#39;s &lt;ok&gt;."));
        assert!(page.contains("gateway/m&lt;x&gt;"));
        assert!(!page.contains("<b>hello"));
    }

    #[test]
    fn an_export_imports_back() {
        let text = jsonl(&conversation()).unwrap();
        let entries = read_jsonl(&text).unwrap();
        assert_eq!(entries.len(), 3);
        assert_eq!(
            entries[1].author.as_ref().and_then(|a| a.title.as_deref()),
            Some("template t")
        );
        let mut back = conversation::Conversation::default();
        for entry in entries {
            back.push(entry);
        }
        assert_eq!(jsonl(&back).unwrap().lines().count(), 3);
        assert!(
            read_jsonl("{\"message\": 3}")
                .unwrap_err()
                .starts_with("line 1")
        );
    }

    #[test]
    fn exports_one_json_line_per_entry() {
        let text = jsonl(&conversation()).unwrap();
        let lines: Vec<serde_json::Value> = text
            .lines()
            .map(|line| serde_json::from_str(line).unwrap())
            .collect();
        assert_eq!(lines.len(), 3);
        assert!(lines[0]["message"].to_string().contains("hello"));
    }
}
