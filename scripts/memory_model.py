#!/usr/bin/env python3
"""Reference traces against native BML and pinned ACTMEM; no storage emulation."""

import argparse
import copy
import hashlib
import json
import os
from pathlib import Path
import random
import subprocess
import sys
import tempfile


ROOT = Path(__file__).resolve().parent.parent
ASSETS = ROOT / "scripts" / "testdata" / "memory-model"
CONTROL = {"reopen", "restart", "crash"}


class BMLModel:
    def __init__(self):
        self.records = {}
        self.operations = {}
        self.revision = 0

    def step(self, q):
        op = q["op"]
        if op in CONTROL:
            return {"error": ""}
        if op == "snapshot":
            visible = [] if q.get("foreign") else [
                {"id": key, "text": value["text"], "revision": value["revision"]}
                for key, value in sorted(self.records.items()) if not value["deleted"]
            ]
            return {"error": "", "store_revision": self.revision, "records": visible}
        identity = q["operation_id"]
        if identity in self.operations:
            request, outcome = self.operations[identity]
            return copy.deepcopy(outcome) if request == q else {"error": "operation_conflict"}
        key = q["id"]
        old = self.records.get(key)
        if op == "create":
            if old:
                return {"error": "record_revision_conflict"}
            revision = 1
        elif not old or old["revision"] != q["revision"]:
            return {"error": "record_revision_conflict"}
        else:
            revision = old["revision"] + 1
        if op == "delete":
            old["deleted"] = True
            revision = 1  # The new tombstone, not the retained target row.
        else:
            self.records[key] = {"text": q["text"], "revision": revision,
                                 "deleted": bool(old and old["deleted"])}
        self.revision += 1
        outcome = {"error": "", "revision": revision, "store_revision": self.revision}
        self.operations[identity] = (copy.deepcopy(q), outcome)
        return copy.deepcopy(outcome)


class RecapModel:
    def __init__(self):
        self.revision = 0
        self.text = ""
        self.operations = {}
        self.unknown = False
        self.confirmed = False

    def step(self, q):
        op = q["op"]
        if op in CONTROL:
            self.unknown = self.confirmed = False
            return {"error": ""}
        if self.unknown:
            return {"error": "recap_commit_unknown"}
        if op == "cancel_recovery":
            if self.confirmed or not self.revision:
                raise ValueError("recovery injection requires reopen of a committed head")
            if q["boundary"] == "recovery_before_replace":
                return {"error": "context_canceled"}
            self.unknown = True
            return {"error": "recap_commit_unknown"}
        self.confirmed = True
        if op == "read":
            return {"error": "", "revision": self.revision, "text": self.text}
        identity = q["operation_id"]
        if identity in self.operations:
            request, outcome = self.operations[identity]
            return copy.deepcopy(outcome) if request == q else {"error": "recap_operation_conflict"}
        if q["revision"] != self.revision:
            return {"error": "recap_conflict"}
        self.revision += 1
        self.text = q["text"]
        outcome = {"error": "", "revision": self.revision, "text": self.text}
        self.operations[identity] = (copy.deepcopy(q), outcome)
        return copy.deepcopy(outcome)


def validate_trace(target, trace):
    if target not in ("bml", "recap") or not isinstance(trace, list) or not trace:
        raise ValueError("nonempty bml/recap trace required")
    fields = {
        "create": {"id", "operation_id", "text"},
        "update": {"id", "operation_id", "text", "revision"},
        "delete": {"id", "operation_id", "revision"},
        "commit": {"operation_id", "text", "revision"},
        "cancel_recovery": {"boundary"},
        "snapshot": set(), "read": set(), **{op: set() for op in CONTROL},
    }
    allowed = CONTROL | ({"create", "update", "delete", "snapshot"} if target == "bml"
                         else {"commit", "read", "cancel_recovery"})
    for q in trace:
        if not isinstance(q, dict) or q.get("op") not in allowed:
            raise ValueError(f"invalid {target} operation: {q!r}")
        required = fields[q["op"]]
        optional = {"foreign"} if q["op"] == "snapshot" else set()
        if not required <= q.keys() or q.keys() - required - {"op"} - optional:
            raise ValueError(f"invalid operation fields: {q!r}")
        if "foreign" in q and type(q["foreign"]) is not bool:
            raise ValueError("foreign must be boolean")
        for key in required - {"revision"}:
            if not isinstance(q[key], str) or (not q[key] and key != "text"):
                raise ValueError(f"invalid {key}")
        if "revision" in q and (type(q["revision"]) is not int or q["revision"] < 0):
            raise ValueError("revision must be a nonnegative integer")
        if q["op"] in {"update", "delete"} and q["revision"] == 0:
            raise ValueError("BML mutation requires a positive record revision")
        if q["op"] == "cancel_recovery" and q["boundary"] not in {
                "recovery_before_replace", "recovery_after_replace"}:
            raise ValueError("unsupported recovery boundary")


def generate_trace(target, seed, steps):
    rng = random.Random(seed)
    model = BMLModel() if target == "bml" else RecapModel()
    trace = []

    def add(q):
        trace.append(copy.deepcopy(q))
        model.step(q)

    first = ({"op": "create", "id": "first", "operation_id": "first", "text": "needle 初始"}
             if target == "bml" else {"op": "commit", "operation_id": "first", "text": "初始", "revision": 0})
    add(first)
    add({"op": "crash"})
    add({"op": "snapshot" if target == "bml" else "read"})
    for i in range(steps):
        choice = rng.randrange(6)
        identity = f"seed-{seed}-step-{i}"
        if choice == 0:
            add({"op": rng.choice(sorted(CONTROL))})
        elif choice == 1:
            replay = copy.deepcopy(rng.choice(list(model.operations.values()))[0])
            if rng.choice((False, True)):
                if "text" in replay:
                    replay["text"] += " needle changed"
                else:
                    replay["revision"] += 1
            add(replay)
        elif target == "bml":
            live = [key for key, value in model.records.items() if not value["deleted"]]
            if choice == 2 and live:
                key = rng.choice(live)
                add({"op": "delete", "id": key, "operation_id": identity,
                     "revision": model.records[key]["revision"]})
            elif choice in (3, 4) and live:
                key = rng.choice(live)
                add({"op": "update", "id": key, "operation_id": identity,
                     "text": f"needle 更新 {rng.randrange(10000)}", "revision":
                     model.records[key]["revision"] + (5 if choice == 4 else 0)})
            else:
                add({"op": "create", "id": identity, "operation_id": identity,
                     "text": f"needle 内容 {rng.randrange(10000)}"})
        else:
            add({"op": "commit", "operation_id": identity, "text": f"内容 {rng.randrange(10000)}",
                 "revision": model.revision + (5 if choice == 4 else 0)})
        add({"op": "snapshot" if target == "bml" else "read"})
    if target == "bml":
        add({"op": "snapshot", "foreign": True})
    else:
        for boundary in ("recovery_after_replace", "recovery_before_replace"):
            add({"op": "reopen"})
            add({"op": "cancel_recovery", "boundary": boundary})
            add({"op": "read"})
            add({"op": "restart"})
            add({"op": "read"})
    validate_trace(target, trace)
    return trace


class ModelMismatch(Exception):
    def __init__(self, index, expected, actual):
        self.index, self.expected, self.actual = index, expected, actual
        super().__init__(f"step {index}: expected {expected!r}, got {actual!r}")


def compare(target, trace, expected, actual):
    for i in range(max(len(expected), len(actual))):
        want = expected[i] if i < len(expected) else "<no result>"
        got = actual[i] if i < len(actual) else "<missing result>"
        if want != got:
            raise ModelMismatch(i, want, got)


def decode_results(output):
    results = []
    for line in output.splitlines():
        value = json.loads(line)
        if not isinstance(value, dict) or not isinstance(value.get("error"), str):
            raise ValueError("worker response must be an object with an error code")
        if value.keys() - {"error", "revision", "store_revision", "text", "records"}:
            raise ValueError("unknown worker response field")
        for key in ("revision", "store_revision"):
            if key in value and (type(value[key]) is not int or value[key] < 0):
                raise ValueError("worker revisions must be nonnegative integers")
        if "text" in value and not isinstance(value["text"], str):
            raise ValueError("worker text must be a string")
        if "records" in value:
            if not isinstance(value["records"], list):
                raise ValueError("worker records must be a list")
            for record in value["records"]:
                if (not isinstance(record, dict) or record.keys() != {"id", "text", "revision"}
                        or not isinstance(record["id"], str) or not isinstance(record["text"], str)
                        or type(record["revision"]) is not int or record["revision"] <= 0):
                    raise ValueError("invalid worker record")
        results.append(value)
    return results


def write_receipt(path, target, seed, trace, failure, sources):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps({"version": 1, "target": target, "seed": seed,
                               "sources": sources, "trace": trace,
                               "failure": {"index": failure.index, "expected": failure.expected,
                                           "actual": failure.actual}}, ensure_ascii=False, indent=2) + "\n",
                    encoding="utf-8")


def load_receipt(path):
    receipt = json.loads(path.read_text(encoding="utf-8"))
    if (not isinstance(receipt, dict) or receipt.get("version") != 1
            or receipt.get("target") not in ("bml", "recap")
            or type(receipt.get("seed")) is not int):
        raise ValueError("replay requires version=1, target=bml/recap, and integer seed")
    validate_trace(receipt["target"], receipt.get("trace"))
    return receipt


def command(args, cwd, *, input=None, env=None, timeout=120):
    return subprocess.run(args, cwd=cwd, input=input, capture_output=True, text=True,
                          encoding="utf-8", timeout=timeout, env=env, check=False)


def checked(args, cwd):
    result = command(args, cwd)
    if result.returncode:
        raise RuntimeError(f"{args[0]} failed: {result.stderr or result.stdout}")
    return result.stdout.strip()


def source_versions():
    laputa = ROOT.parent / "laputa"
    lock = json.loads((ROOT / "laputa-source.lock.json").read_text())
    sha = checked(["git", "rev-parse", "HEAD"], laputa)
    if sha != lock["commit"] or checked(["git", "status", "--porcelain"], laputa):
        raise ValueError("Laputa must be clean at laputa-source.lock.json; run the official bootstrap")
    module = (laputa / "laputa" / "go.mod").read_text()
    if not module.startswith("module github.com/ProjectViVy/laputa/laputa\n"):
        raise ValueError("unexpected Laputa module identity")
    digest = hashlib.sha256()
    for path in (Path(__file__), ASSETS / "bml_test.go", ASSETS / "recap_test.go"):
        digest.update(path.read_bytes())
    return {"vivy": checked(["git", "rev-parse", "HEAD"], ROOT), "laputa": sha,
            "vivy_dirty": bool(checked(["git", "status", "--porcelain"], ROOT)),
            "harness_sha256": digest.hexdigest(), "go": checked(["go", "version"], ROOT)}


def build_bridge(target, work, race=False):
    module = ROOT / "bml" if target == "bml" else ROOT.parent / "laputa" / "laputa"
    package = module if target == "bml" else module / "actmem"
    overlay = work / f"{target}-overlay.json"
    overlay.write_text(json.dumps({"Replace": {str(package / "memory_model_test.go"):
                                              str(ASSETS / f"{target}_test.go")}}))
    binary = work / (target + (".exe" if os.name == "nt" else ""))
    args = ["go", "test", "-mod=readonly", "-p=1", "-overlay", str(overlay), "-c", "-o", str(binary)]
    if race:
        args.append("-race")
    args.append("." if target == "bml" else "./actmem")
    result = command(args, module, timeout=300)
    if result.returncode:
        raise RuntimeError(f"Go bridge build failed: {result.stderr or result.stdout}")
    return binary


def run_trace(binary, trace, root):
    actual, segment = [], []
    for i, q in enumerate(trace):
        segment.append(q)
        if q["op"] not in {"restart", "crash"} and i != len(trace) - 1:
            continue
        result = command([str(binary), "-test.run=^TestMemoryModelBridge$"], root,
                         input="".join(json.dumps(q, ensure_ascii=False) + "\n" for q in segment),
                         env={**os.environ, "VIVY_MEMORY_MODEL_ROOT": str(root)}, timeout=30)
        expected_exit = 23 if segment[-1]["op"] == "crash" else 0
        if result.returncode != expected_exit:
            raise RuntimeError(f"worker exited {result.returncode}, expected {expected_exit}: {result.stderr}")
        output = decode_results(result.stdout)
        if len(output) != len(segment):
            raise ValueError(f"worker returned {len(output)} results for {len(segment)} operations")
        actual.extend(output)
        segment = []
    return actual


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--target", choices=("bml", "recap", "all"), default="all")
    parser.add_argument("--seed", type=int, default=17)
    parser.add_argument("--cases", type=int, default=5)
    parser.add_argument("--steps", type=int, default=40)
    parser.add_argument("--replay", type=Path)
    parser.add_argument("--output", type=Path, default=ROOT / ".workspace" / "memory-model")
    parser.add_argument("--go-regressions", action="store_true")
    parser.add_argument("--race", action="store_true")
    args = parser.parse_args(argv)
    if not 1 <= args.cases <= 100 or not 1 <= args.steps <= 500:
        parser.error("cases must be 1..100 and steps must be 1..500")
    receipt = load_receipt(args.replay) if args.replay else None
    sources = source_versions()
    targets = [receipt["target"]] if receipt else (["bml", "recap"] if args.target == "all" else [args.target])
    failed = False
    with tempfile.TemporaryDirectory(prefix="vivy-memory-model-") as temporary:
        work = Path(temporary)
        for target in targets:
            if target not in ("bml", "recap"):
                raise ValueError("invalid replay target")
            binary = build_bridge(target, work, args.race)
            if args.go_regressions:
                result = command([str(binary), "-test.run=^TestMemoryModelNative", "-test.v"], ROOT)
                print(result.stdout, end="")
                print(result.stderr, end="", file=sys.stderr)
                failed |= result.returncode != 0
                continue
            for case in range(1 if receipt else args.cases):
                seed = receipt["seed"] if receipt else args.seed + case
                trace = receipt["trace"] if receipt else generate_trace(target, seed, args.steps)
                validate_trace(target, trace)
                model = BMLModel() if target == "bml" else RecapModel()
                expected = [model.step(q) for q in trace]
                try:
                    with tempfile.TemporaryDirectory(prefix=f"{target}-", dir=work) as store:
                        actual = run_trace(binary, trace, Path(store))
                    compare(target, trace, expected, actual)
                except (OSError, ValueError, RuntimeError, subprocess.TimeoutExpired) as error:
                    failure = ModelMismatch(-1, "complete valid worker trace", str(error))
                    path = args.output / f"{target}-{seed}.json"
                    write_receipt(path, target, seed, trace, failure, sources)
                    print(f"ERROR {target} seed={seed}: {error}; replay: --replay {path}")
                    failed = True
                except ModelMismatch as error:
                    path = args.output / f"{target}-{seed}.json"
                    write_receipt(path, target, seed, trace, error, sources)
                    print(f"FAIL {target} seed={seed}: {error}; replay: --replay {path}")
                    failed = True
                else:
                    print(f"PASS {target} seed={seed} operations={len(trace)}")
    return int(failed)


if __name__ == "__main__":
    try:
        sys.exit(main())
    except (OSError, ValueError, RuntimeError, subprocess.TimeoutExpired) as error:
        print(f"memory-model: {error}", file=sys.stderr)
        sys.exit(2)
