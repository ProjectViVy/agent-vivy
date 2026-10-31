"""Run Vivy's D0 acceptance tests against an editable Rig checkout.

Usage: python scripts/rig_fork_probe.py [--rig <checkout>]
Omit --rig to reproduce the acceptance failure against the pinned upstream.
Only the ignored diagnostic manifest/lock are rewritten; production pins stay intact.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tomllib

from durable_probe import probe_environment

ROOT = Path(__file__).resolve().parents[1]


def manifest_for(source: str, plugin: Path, rig: Path | None) -> str:
    parsed = tomllib.loads(source)
    for section in ("dependencies", "dev-dependencies"):
        for name,spec in parsed.get(section,{}).items():
            if isinstance(spec,dict) and "path" in spec:
                replacement=dict(spec,path=(plugin/spec["path"]).resolve().as_posix())
                fields=", ".join(f"{k} = {json.dumps(v)}" for k,v in replacement.items())
                source=re.sub(rf"(?m)^{re.escape(name)} = \{{[^\n]*\}}$",lambda _:f"{name} = {{ {fields} }}",source)
    if rig is not None:
        for section in ("dependencies", "dev-dependencies"):
            for name, spec in parsed.get(section, {}).items():
                if not isinstance(spec, dict) or spec.get("git") != "https://github.com/0xPlaygrounds/rig":
                    continue
                candidates = [rig / folder / name for folder in ("crates", "test-support", "plugins")]
                package = next((p for p in candidates if (p / "Cargo.toml").is_file()), None)
                if package is None:
                    raise ValueError(f"Rig checkout does not contain package {name}")
                data = tomllib.loads((package / "Cargo.toml").read_text(encoding="utf-8"))
                if data["package"]["name"] != name:
                    raise ValueError(f"unexpected package in {package}")
                replacement = {"path": package.as_posix(), **{
                    k: v for k, v in spec.items() if k not in ("git", "rev")}}
                fields = ", ".join(f"{k} = {json.dumps(v)}" for k, v in replacement.items())
                source, count = re.subn(rf"(?m)^{re.escape(name)} = \{{[^\n]*\}}$",
                                       lambda _: f"{name} = {{ {fields} }}", source)
                if count != 1:
                    raise ValueError(f"expected one inline dependency for {name}, found {count}")
    source += f'\n[lib]\npath = {json.dumps((plugin / "src/lib.rs").as_posix())}\n\n[workspace]\n'
    if rig:
        # Optional Vivy gateway dependencies must use the same local Rig graph.
        source += '\n[patch."https://github.com/0xPlaygrounds/rig"]\n'
        metadata=json.loads(subprocess.check_output(["cargo","metadata","--offline","--no-deps","--format-version","1"],cwd=rig,text=True))
        for package in metadata["packages"]:
            name=package["name"]
            if name=="rig" or name.startswith("rig-"):
                path=Path(package["manifest_path"]).parent.as_posix()
                source += f'{name} = {{ path = {json.dumps(path)} }}\n'
    return source


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--rig", type=Path)
    parser.add_argument("--runtime", action="store_true", help="run D0 checkpoint and D1 runtime acceptance (requires --rig)")
    parser.add_argument("--assistant-context", action="store_true", help="also run D2 persona/Recap projection and maintenance tests (requires --rig)")
    parser.add_argument("--wake", action="store_true", help="also run D3 durable wait/wake tests (requires --rig)")
    parser.add_argument("--autodream", action="store_true", help="also run D4 AutoDream tests (requires --rig)")
    args = parser.parse_args()
    args.wake = args.wake or args.autodream
    args.assistant_context = args.assistant_context or args.wake
    args.runtime = args.runtime or args.assistant_context
    rig = args.rig.resolve(strict=True) if args.rig else None
    if args.runtime and not rig:
        parser.error("--runtime requires --rig")
    plugin = ROOT / "plugins/vivy-durable"
    output = ROOT / ".workspace/d0-d1/consumer" if args.runtime else ROOT / ".workspace/d0" / ("fork-probe" if rig else "stock-fork-probe")
    if args.assistant_context:
        output = ROOT / ".workspace/d2/consumer"
    if args.wake:
        output = ROOT / ".workspace/d3/consumer"
    if args.autodream:
        output = ROOT / ".workspace/d4/consumer"
    output.mkdir(parents=True, exist_ok=True)
    manifest = output / "Cargo.toml"
    manifest.write_text(manifest_for((plugin / "Cargo.toml").read_text(encoding="utf-8"),
                                     plugin, rig), encoding="utf-8")
    shutil.copyfile(plugin / "Cargo.lock", output / "Cargo.lock")
    report = {"status": "running", "rig": str(rig) if rig else "pinned upstream",
              "scope": "D0/D1 + D2 assistant context; process-crash only" if args.assistant_context else "D0 checkpoint + D1 runtime; process-crash only" if args.runtime else "serialization failure and existing D0 characterizations; no sync receipt"}
    if rig:
        report["revision"] = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=rig, text=True).strip()
        diff = subprocess.check_output(["git", "diff", "HEAD", "--"], cwd=rig)
        report["diff_sha256"] = hashlib.sha256(diff).hexdigest()
        report["dirty"] = bool(subprocess.check_output(["git", "status", "--porcelain"], cwd=rig))
    report_path = output / "report.json"
    if args.wake:
        report['scope'] = 'D0/D1/D2 + D3 waiting/wake; process-crash only'
    if args.autodream:
        report['scope'] = 'D0/D1/D2/D3 + D4 AutoDream; process-crash only'
    report_path.write_text(json.dumps(report, indent=2), encoding="utf-8")
    environment = probe_environment(os.environ)
    command = ["cargo", "test", "--offline", "--manifest-path", str(manifest),
               "--features", "autodream,checkpoint-probe" if args.autodream else "wake,checkpoint-probe" if args.wake else "assistant-context,checkpoint-probe" if args.assistant_context else "runtime,checkpoint-probe" if args.runtime else "rig-fork-probe"]
    with (output / "tests.log").open("w", encoding="utf-8") as log:
        result = subprocess.run(command, cwd=output, env=environment, stdout=log,
                                stderr=subprocess.STDOUT, timeout=600)
    report.update(status="passed" if result.returncode == 0 else "failed", exit_code=result.returncode)
    report_path.write_text(json.dumps(report, indent=2), encoding="utf-8")
    print(f"{report['status']}: {report_path}")
    if result.returncode:
        print("\n".join((output / "tests.log").read_text(encoding="utf-8").splitlines()[-35:]))
    return result.returncode


if __name__ == "__main__":
    raise SystemExit(main())
