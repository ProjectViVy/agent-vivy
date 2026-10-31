"""Build temporary recipe variants and prove delegation/TUI optionality.

Usage: python scripts/subagents_optional.py <vivy.exe>
Uses CARGO_TARGET_DIR, the same as the baseline build. Variants are ignored.
"""
import json
import os
import pathlib
import re
import subprocess
import sys
import tempfile
import threading
from http.server import ThreadingHTTPServer

from mock_gateway import Handler

ROOT = pathlib.Path(__file__).resolve().parent.parent


def variant(builder, name, excluded, extra="", env=None):
    original = (ROOT / "recipes/coding-agent.yaml").read_text(encoding="utf-8")
    header, components = original.split("components:\n", 1)
    header = re.sub(r"(?m)^name: coding-agent$", "name: " + name, header)
    pieces = re.split(r"(?=^  - name:)", components, flags=re.MULTILINE)
    selected = [piece for piece in pieces if not any(piece.startswith("  - name: " + item + "\n") for item in excluded)]
    text = header + "components:\n" + "".join(selected) + extra
    text = re.sub(r"path: ../plugins/([^\s]+)", lambda m: "path: " + json.dumps(str(ROOT / "plugins" / m[1])), text)
    directory = ROOT / ".workspace/subagents/recipes"
    directory.mkdir(parents=True, exist_ok=True)
    recipe = directory / (name + ".yaml")
    recipe.write_text(text, encoding="utf-8")
    recipe.with_suffix(".Cargo.lock").write_bytes((ROOT / "recipes/coding-agent.Cargo.lock").read_bytes())
    subprocess.run([builder, "build", str(recipe)], check=True, cwd=ROOT, env=env)
    binary = pathlib.Path(os.environ["CARGO_TARGET_DIR"]) / "debug" / (name + (".exe" if os.name == "nt" else ""))
    return binary


class WithoutDelegation(Handler):
    def do_POST(self):
        body = json.loads(self.rfile.read(int(self.headers["content-length"])))
        names = {tool["function"]["name"] for tool in body["tools"]}
        assert not names & {"task", "task_parallel", "task_chain", "message", "wait"}, names
        self.server.checked = True
        self.reply(body, text="NO_SUBAGENTS_ACCEPTED")


def main():
    builder = str(pathlib.Path(sys.argv[1]).resolve())
    headless = variant(builder, "subagents-headless", ["terminal"])
    graph = subprocess.check_output(["cargo", "tree", "--manifest-path", str(ROOT / ".generated/subagents-headless/Cargo.toml"), "-e", "normal", "--prefix", "none"], text=True, encoding="utf-8")
    assert "rig-tui " not in graph, "headless normal dependency graph includes TUI"
    subprocess.run([sys.executable, str(ROOT / "scripts/subagents_smoke.py"), str(headless)], check=True, cwd=ROOT)
    print("PASS: headless delegation, no rig-tui in normal dependencies", flush=True)
    without = variant(builder, "without-subagents", ["subagents"])
    server = ThreadingHTTPServer(("127.0.0.1", 0), WithoutDelegation)
    server.checked = False
    threading.Thread(target=server.serve_forever, daemon=True).start()
    try:
        with tempfile.TemporaryDirectory(prefix="vivy optional subagents ") as directory:
            root = pathlib.Path(directory)
            (root / "settings.toml").write_text(f'[gateway]\nbase_url = "http://127.0.0.1:{server.server_port}/v1"\nmodel = "model"\n[trust]\ndefault = "never"\n', encoding="utf-8")
            result = subprocess.run([str(without), "-p", "List capabilities"], cwd=root, env=dict(os.environ, RIG_HOME=str(root), RIG_SECRET_GATEWAY="test-key"), text=True, encoding="utf-8", capture_output=True, timeout=30)
            assert result.returncode == 0 and "NO_SUBAGENTS_ACCEPTED" in result.stdout and server.checked, (result.returncode, result.stdout, result.stderr)
        print("PASS: recipe without subagents exposes no delegation tools", flush=True)
    finally:
        server.shutdown()
        server.server_close()


if __name__ == "__main__":
    main()
