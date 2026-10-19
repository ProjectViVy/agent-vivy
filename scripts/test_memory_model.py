"""Tests for the reference contract and fail-closed trace transport."""

import copy
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
import subprocess

import memory_model as mm


def create(key="a", operation="create-a", text="needle original"):
    return {"op": "create", "id": key, "operation_id": operation, "text": text}


class ReferenceTests(unittest.TestCase):
    def test_bml_replay_retains_original_outcome_after_update_and_delete(self):
        model = mm.BMLModel()
        first = model.step(create())
        self.assertEqual(first, {"error": "", "revision": 1, "store_revision": 1})
        update = dict(create(operation="update-a", text="needle new"),
                      op="update", revision=1)
        self.assertEqual(model.step(update)["revision"], 2)
        self.assertEqual(model.step(create()), first)
        self.assertEqual(model.step({"op": "delete", "id": "a",
                                     "operation_id": "delete-a", "revision": 2})["store_revision"], 3)
        model.step({"op": "restart"})
        self.assertEqual(model.step({"op": "snapshot"}),
                         {"error": "", "store_revision": 3, "records": []})
        self.assertEqual(model.step(create()), first)

    def test_bml_stale_and_changed_replay_do_not_mutate_state(self):
        model = mm.BMLModel()
        model.step(create())
        before = model.step({"op": "snapshot"})
        self.assertEqual(model.step(dict(create(operation="bad"), op="update", revision=9))["error"],
                         "record_revision_conflict")
        self.assertEqual(model.step(create(text="needle changed"))["error"], "operation_conflict")
        self.assertEqual(model.step({"op": "snapshot"}), before)

    def test_recap_cancel_before_replace_can_retry_without_reopen(self):
        model = mm.RecapModel()
        request = {"op": "commit", "operation_id": "one", "text": "kept", "revision": 0}
        model.step(request)
        model.step({"op": "reopen"})
        self.assertEqual(model.step({"op": "cancel_recovery", "boundary": "recovery_before_replace"}),
                         {"error": "context_canceled"})
        self.assertEqual(model.step({"op": "read"}), {"error": "", "revision": 1, "text": "kept"})
        self.assertEqual(model.step(request), {"error": "", "revision": 1, "text": "kept"})

    def test_recap_cancel_after_replace_requires_reopen(self):
        model = mm.RecapModel()
        model.step({"op": "commit", "operation_id": "one", "text": "kept", "revision": 0})
        model.step({"op": "reopen"})
        self.assertEqual(model.step({"op": "cancel_recovery", "boundary": "recovery_after_replace"}),
                         {"error": "recap_commit_unknown"})
        self.assertEqual(model.step({"op": "read"}), {"error": "recap_commit_unknown"})
        model.step({"op": "restart"})
        self.assertEqual(model.step({"op": "read"})["text"], "kept")

    def test_generation_is_deterministic_and_contains_lifecycle_checks(self):
        for target in ("bml", "recap"):
            trace = mm.generate_trace(target, 17, 50)
            self.assertEqual(trace, mm.generate_trace(target, 17, 50))
            self.assertNotEqual(trace, mm.generate_trace(target, 18, 50))
            self.assertIn("crash", {q["op"] for q in trace})
            model = mm.BMLModel() if target == "bml" else mm.RecapModel()
            for q in trace:
                model.step(q)

    def test_generated_traces_reject_changed_operation_identity(self):
        for target, cls, error in (("bml", mm.BMLModel, "operation_conflict"),
                                   ("recap", mm.RecapModel, "recap_operation_conflict")):
            model = cls()
            results = [model.step(q) for q in mm.generate_trace(target, 17, 100)]
            self.assertTrue(any(result["error"] == error for result in results), target)

    def test_bad_trace_is_rejected_not_ignored(self):
        for trace in ([{"op": "made-up"}], [{"op": "create", "id": "a"}], [],
                      [{"op": "snapshot", "foreign": "yes"}],
                      [dict(create(), foreign=True)],
                      [dict(create(), op="update", revision=0)]):
            with self.assertRaises(ValueError):
                mm.validate_trace("bml", trace)

    def test_replay_envelope_is_validated_before_execution(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "trace.json"
            valid = {"version": 1, "target": "bml", "seed": 7, "trace": [create()]}
            path.write_text(json.dumps(valid))
            self.assertEqual(mm.load_receipt(path), valid)
            for invalid in ([], {}, dict(valid, version=2), dict(valid, seed=True),
                            dict(valid, target="unknown"), dict(valid, trace=[])):
                path.write_text(json.dumps(invalid))
                with self.assertRaises(ValueError):
                    mm.load_receipt(path)

    def test_source_guard_rejects_wrong_pin_and_dirty_sibling(self):
        pin = json.loads((mm.ROOT / "laputa-source.lock.json").read_text())["commit"]
        for outputs in (("wrong-sha",), (pin, " M laputa/go.mod")):
            with patch.object(mm, "checked", side_effect=outputs):
                with self.assertRaises(ValueError):
                    mm.source_versions()


class TransportTests(unittest.TestCase):
    def test_mismatch_receipt_is_self_contained_and_replayable(self):
        with tempfile.TemporaryDirectory() as directory:
            trace = [create(), {"op": "snapshot"}]
            expected = [{"error": "", "revision": 1, "store_revision": 1}, {"error": "", "store_revision": 1, "records": []}]
            actual = copy.deepcopy(expected)
            actual[0]["revision"] = 0
            with self.assertRaises(mm.ModelMismatch) as error:
                mm.compare("bml", trace, expected, actual)
            path = Path(directory) / "failure.json"
            mm.write_receipt(path, "bml", 12, trace, error.exception, {"vivy": "abc"})
            receipt = json.loads(path.read_text())
            self.assertEqual(receipt["trace"], trace)
            self.assertEqual(receipt["failure"]["index"], 0)
            self.assertEqual(receipt["sources"]["vivy"], "abc")

    def test_missing_and_extra_results_fail(self):
        for actual in ([], [{"error": ""}, {"error": ""}]):
            with self.assertRaises(mm.ModelMismatch):
                mm.compare("bml", [{"op": "reopen"}], [{"error": ""}], actual)

    def test_malformed_worker_output_fails(self):
        for output in ("PASS\n", '{"error":""}\nnot-json', "[]\n",
                       '{"error":"","revision":true}', '{"error":"","unexpected":0}'):
            with self.assertRaises(ValueError):
                mm.decode_results(output)

    def test_worker_exit_and_incomplete_output_are_not_passes(self):
        for code, output in ((1, ''), (23, ''), (0, ''), (0, '{}\n')):
            result = subprocess.CompletedProcess([], code, output, "synthetic failure")
            with patch.object(mm, "command", return_value=result):
                with self.assertRaises((RuntimeError, ValueError)):
                    mm.run_trace(Path("fake-worker"), [{"op": "snapshot"}], Path("."))

    def test_crash_exit_is_accepted_only_at_an_explicit_crash_step(self):
        result = subprocess.CompletedProcess([], 23, '{"error":""}\n', '')
        with patch.object(mm, "command", return_value=result):
            self.assertEqual(mm.run_trace(Path("fake"), [{"op": "crash"}], Path(".")), [{"error": ""}])

    def test_worker_timeout_propagates_instead_of_becoming_empty_success(self):
        with patch.object(mm, "command", side_effect=subprocess.TimeoutExpired("fake", 30)):
            with self.assertRaises(subprocess.TimeoutExpired):
                mm.run_trace(Path("fake"), [{"op": "snapshot"}], Path("."))


if __name__ == "__main__":
    unittest.main()
