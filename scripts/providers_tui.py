"""Native provider CRUD, key masking and session/default acceptance on ConPTY.

Usage: python scripts/providers_tui.py <vivy>; test-only pyte/pywinpty as plan_tui.
"""
from contextlib import ExitStack
import json
import os
import pathlib
import subprocess
import sys
import tempfile
import threading
import tomllib
from http.server import ThreadingHTTPServer
from branding_tui import View
from plan_tui import command, skip_initialization
from providers_smoke import Gateway
from tui import Terminal, ENTER


def close(terminal):
    if os.name == "nt" and terminal.process.isalive():
        subprocess.run(["taskkill", "/PID", str(terminal.pid), "/T", "/F"], capture_output=True)
    terminal.close()


def scenario(binary, cols, rows):
    server = ThreadingHTTPServer(("127.0.0.1", 0), Gateway)
    server.records = []
    threading.Thread(target=server.serve_forever, daemon=True).start()
    terminal = None
    try:
        with ExitStack() as resources:
            root = pathlib.Path(resources.enter_context(tempfile.TemporaryDirectory(prefix="vivy provider terminal ")))
            home, work = root / "home", root / "work"
            home.mkdir()
            work.mkdir()
            (home / "settings.toml").write_text("# retained\n[trust]\ndefault='never'\n", encoding="utf-8")
            env = dict(os.environ, RIG_HOME=str(home), VIVY_CHAT_KEY="fixture-chat", VIVY_MESSAGES_KEY="fixture-messages")
            env.pop("VIVY_BOOTSTRAP_GATEWAY_URL", None)
            env.pop("VIVY_BOOTSTRAP_GATEWAY_MODEL", None)
            terminal = Terminal([binary], work, env, rows=rows, cols=cols)
            resources.callback(close, terminal)
            view = View(terminal, cols, rows)
            skip_initialization(terminal, view)
            view.wait("empty home starts", lambda s: "Vivy" in s and "›" in s)

            def choose(label, title):
                terminal.type(label)
                view.wait("choose " + label, lambda s: title in s)

            def value(text, title):
                terminal.type(text)
                view.wait("input accepted", lambda s: title in s)

            def add(protocol, instance, endpoint, variable):
                command(terminal, view, "/provider")
                view.wait("provider list", lambda s: "Providers" in s)
                choose("Add provider", "Add provider")
                choose("Custom " + protocol, "Edit provider")
                choose("Instance ID:", "Current:")
                value(instance, "Edit provider")
                choose("Endpoint:", "Current:")
                value(endpoint, "Edit provider")
                choose("Credential:", "Credential source")
                choose("Environment variable", "Current:")
                value(variable, "Edit provider")
                choose("Models:", "Provider models")
                choose("Add model", "Edit model")
                choose("id:", "Current:")
                value("org/model-1", "Edit model")
                choose("max_output_tokens:", "Current:")
                value("1000", "Edit model")
                choose("Save model to draft", "Provider models")
                choose("Back to provider", "Edit provider")
                choose("Save only", "Provider")
                view.wait("provider persisted", lambda _: instance in tomllib.loads((home / "settings.toml").read_text()).get("providers", {}).get("instances", {}))

            add("OpenAI", "chat", f"http://127.0.0.1:{server.server_port}/chat/v1", "VIVY_CHAT_KEY")
            choose("Save API key", "API key for chat")
            terminal.type("CONPTY_PRIVATE_FIXTURE_7845", enter=False)
            terminal.read_for(0.8)
            assert "CONPTY_PRIVATE_FIXTURE_7845" not in terminal.output.decode("utf-8", "replace")
            terminal.type(ENTER, enter=False)
            view.wait("saved key", lambda s: "API key saved" in s or "saved key" in s)
            choose("Choose model", "Choose model")
            choose("org/model-1", "chat/org/model-1")
            command(terminal, view, "/model-default")
            view.wait("default persisted", lambda _: tomllib.loads((home / "settings.toml").read_text()).get("providers", {}).get("default_model") == "chat/org/model-1")
            command(terminal,view,"PYTHON_PRIME")
            view.wait("persistent Python started before the next credential",lambda s:"PYTHON_PRIMED" in s and "idle" in s.splitlines()[-1])
            add("Anthropic", "messages", f"http://127.0.0.1:{server.server_port}/messages", "VIVY_MESSAGES_KEY")
            choose("Choose model", "Choose model")
            choose("org/model-1", "messages/org/model-1")
            command(terminal, view, "TOOLS_WIRE")
            view.wait("Anthropic tool result", lambda s: "SHELL_FILTER_OK" in s and "idle" in s.splitlines()[-1])
            command(terminal,view,"PYTHON_WIRE")
            view.wait("Python policy refresh removes the newly declared credential",lambda s:"PYTHON_FILTER_OK" in s and "idle" in s.splitlines()[-1])
            command(terminal,view,"BLOCK_WIRE")
            view.wait("Anthropic shell call started",lambda s:"BLOCK_READY" in s and "Esc stop" in s)
            terminal.type(b"\x1b",enter=False)
            view.wait("Anthropic tool cancellation returns to idle",lambda s:"idle" in s.splitlines()[-1] and "Esc stop" not in s)
            command(terminal, view, "/provider messages")
            view.wait("provider details", lambda s: "Edit provider and models" in s)
            choose("Edit provider and models", "Edit provider")
            choose("Name:", "Current:")
            value("Message account", "Edit provider")
            choose("Save only", "Provider")
            view.wait("edited provider name persisted", lambda _: tomllib.loads((home / "settings.toml").read_text())["providers"]["instances"]["messages"].get("name") == "Message account")
            assert tomllib.loads((home / "settings.toml").read_text())["providers"]["instances"]["messages"]["name"] == "Message account"
            terminal.type(b"\x1b", enter=False)
            terminal.read_for(0.3)
            command(terminal, view, "/quit")
            terminal.read_for(1)
            close(terminal)
            session = next((home / "sessions").iterdir()).name
            terminal = Terminal([binary, "--resume", session], work, env, rows=rows, cols=cols)
            resources.callback(close, terminal)
            view = View(terminal, cols, rows)
            view.wait("resumed model wins over startup default", lambda s: "messages/org/model-1" in s and "›" in s)
            with (home / "settings.toml").open("a",encoding="utf-8") as file:
                file.write(f'''\n[providers.instances.file_added]
protocol="openai"
base_url="http://127.0.0.1:{server.server_port}/chat/v1"
credential={{source="none"}}
[[providers.instances.file_added.models]]
id="org/model-1"
''')
            command(terminal,view,"/model")
            view.wait("model picker reloads external provider edits",lambda s:"file_added/org/model-1" in s)
            terminal.type(b"\x1b",enter=False)
            terminal.read_for(0.3)
            command(terminal, view, "/provider messages")
            view.wait("resumed provider details", lambda s: "Edit provider and models" in s)
            evidence=os.environ.get("VIVY_PROVIDER_EVIDENCE")
            if evidence:
                view.capture(pathlib.Path(evidence)/f"{cols}x{rows}","provider")
            choose("Delete provider", "Delete provider")
            choose("Delete provider and saved key", "Providers")
            assert "messages" not in tomllib.loads((home / "settings.toml").read_text())["providers"]["instances"]
            assert "CONPTY_PRIVATE_FIXTURE_7845" not in (home / "settings.toml").read_text()
            assert "# retained" in (home / "settings.toml").read_text()
            assert "Agent-loop systems use the same data" not in terminal.output.decode("utf-8","replace")
            terminal.type(b"\x1b", enter=False)
            terminal.read_for(0.3)
            command(terminal, view, "/quit")
            terminal.read_for(1)
            print(f"PASS: native providers {cols}x{rows}, CRUD, hidden key, protocols, tools, defaults and resume", flush=True)
    finally:
        server.shutdown()
        server.server_close()


if __name__ == "__main__":
    for size in [(120, 40), (50, 20)]:
        scenario(str(pathlib.Path(sys.argv[1]).resolve()), *size)
