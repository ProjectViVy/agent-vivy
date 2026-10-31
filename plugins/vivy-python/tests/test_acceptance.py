"""The real-model gate must prove execution, not match a tool-call argument."""
from pathlib import Path
import sys
import unittest

sys.path.insert(0, str(Path(__file__).parents[3] / 'scripts'))
from workbench_smoke import validate_events


class AcceptanceTests(unittest.TestCase):
    def test_tool_arguments_and_failed_results_cannot_pass_the_gate(self):
        events = [{'type':'tool_call', 'args':{'code':"print('MEMORY_COMPLETE ARTIFACT_COMPLETE')"}},
                  {'type':'tool_result', 'ok':False, 'output':"Traceback: print('MEMORY_COMPLETE ARTIFACT_COMPLETE')"}]
        for native_memory in (False, True):
            with self.assertRaises(AssertionError):
                validate_events(events, native_memory)

    def test_successful_outputs_and_expected_ordinary_failure_are_required(self):
        events = [{'type':'tool_result', 'ok':True, 'output':marker} for marker in ['DISCOVERED','ARTIFACT_COMPLETE','STATE_SURVIVED','MEMORY_COMPLETE']]
        with self.assertRaises(AssertionError):
            validate_events(events, True)
        events.append({'type':'tool_result', 'ok':False, 'output':'ValueError: EXPECTED_ORDINARY_ERROR'})
        validate_events(events, True)


if __name__ == '__main__':unittest.main()
