"""Offline tests for the D0 evidence checker (python -m unittest discover -s scripts -p test_durable_probe.py)."""
import copy
import unittest

from durable_probe import analyze, probe_environment


def fixture():
    events = []
    for number, kind in enumerate(["input", "model", "tool"]):
        cp = {"operation": str(number), "kind": kind, "digest": "digest",
              "revision": number, "generation": 0}
        events.extend([
            {"event": "checkpoint_prepared", "agent": "a", "checkpoint": cp},
            {"event": "append_observed", "log": "a", "checkpoints": [cp]},
            {"event": "write_gate_released", "agent": "a", "checkpoint": cp},
            {"event": "started", "agent": "a", "operation": str(number)},
        ])
    for event in events:
        event["durable"] = False
    return events


class EvidenceTest(unittest.TestCase):
    def test_inherited_rig_protocol_and_probe_variables_are_removed(self):
        original = {name: "outside-fixture" for name in [
            "RIG_HOME", "RIG_SESSION", "RIG_RUN", "RIG_START", "RIG_LAUNCHER",
            "RIG_NOTICE", "RIG_BUILD_FAILURE", "RIG_PROCESS_LEDGER", "RIG_DESCRIBE",
            "RIG_SOURCE", "RIG_SECRET_GATEWAY", "VIVY_D0_TRACE", "rig_run"]}
        original.update(PATH="toolchain", CARGO_TARGET_DIR="cache")
        expected = {"PATH": "toolchain", "CARGO_TARGET_DIR": "cache"}
        self.assertEqual(probe_environment(original), expected)
        self.assertIn("RIG_RUN", original)

    def test_complete_sequence(self):
        self.assertEqual(analyze(fixture())["observed_starts"], 3)

    def test_arbitrary_or_failed_write_is_not_a_receipt(self):
        for event in [{"event": "append_failed"},
                      {"event": "append_observed", "log": "another", "checkpoints": []}]:
            with self.subTest(event=event):
                events = fixture()
                events[1] = dict(event, durable=False)
                with self.assertRaises(AssertionError):
                    analyze(events)

    def test_changed_request_invalidates_previous_release(self):
        events = fixture()
        changed = copy.deepcopy(events[0])
        changed["checkpoint"]["revision"] += 1
        changed["checkpoint"]["digest"] = "different"
        events.insert(3, changed)
        with self.assertRaises(AssertionError):
            analyze(events)

    def test_duplicate_dispatch_or_claimed_disk_receipt_is_rejected(self):
        events = fixture()
        events.insert(4, copy.deepcopy(events[3]))
        with self.assertRaises(AssertionError):
            analyze(events)
        events = fixture()
        events[1]["durable"] = True
        with self.assertRaises(AssertionError):
            analyze(events)


if __name__ == "__main__":
    unittest.main()
