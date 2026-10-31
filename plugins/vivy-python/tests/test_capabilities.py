import importlib
import json
import os
from pathlib import Path
import sys
import tempfile
import unittest
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).parents[1] / 'python'))


class CapabilityTests(unittest.TestCase):
    def test_managed_binding_uses_frozen_origin_and_reports_actual_result(self):
        from vivy_workbench import capabilities
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            source = root / 'src' / 'managed_fixture'
            source.mkdir(parents=True)
            (source / '__init__.py').write_text('def run(value): return value + 7\n', encoding='utf-8')
            (root / 'pyproject.toml').write_text('[project]\nname="managed_fixture"\nversion="0.1"\n', encoding='utf-8')
            (root / 'SKILL.md').write_text('frozen instructions', encoding='utf-8')
            row = {'name':'managed_fixture', 'file':str(root/'SKILL.md'), 'revision':'revision-one', 'publication':'publication-one'}
            events = []
            namespace = {}
            try:
                inventory = capabilities.load([row], namespace, usage=events.append)
                self.assertEqual(inventory[0]['state'], 'ready')
                self.assertEqual(namespace['managed_fixture'](3), 10)
                self.assertEqual(events[0]['revision'], 'revision-one')
                self.assertEqual(events[0]['publication'], 'publication-one')
                self.assertEqual(events[0]['result'], '10')
                self.assertEqual(events[0]['outcome'], 'ok')
                import inspect
                self.assertEqual(str(inspect.signature(namespace['managed_fixture'])), '(value)')
            finally:
                sys.modules.pop('managed_fixture',None)
                while str(root/'src') in sys.path: sys.path.remove(str(root/'src'))

    def test_managed_import_cannot_silently_reuse_an_old_module(self):
        from vivy_workbench import capabilities
        import types
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root/'src'/'old_fixture').mkdir(parents=True)
            (root/'src'/'old_fixture'/'__init__.py').write_text('def run(): return "new"',encoding='utf-8')
            (root/'pyproject.toml').write_text('[project]\nname="old_fixture"\nversion="0.1"\n',encoding='utf-8')
            old = types.ModuleType('old_fixture')
            old.__file__ = str(root/'old.py')
            old.run = lambda: 'old'
            with patch.dict(sys.modules, {'old_fixture':old}):
                namespace = {}
                records = capabilities.load([{'name':'old_fixture','file':str(root/'SKILL.md'),'revision':'new'}],namespace)
                self.assertEqual(records[0]['state'],'unavailable')
                self.assertNotIn('old_fixture',namespace)
            while str(root/'src') in sys.path: sys.path.remove(str(root/'src'))

    def test_memory_status_reports_actual_binding_and_close_without_a_probe(self):
        from vivy_workbench.capabilities import memory_status
        from vivy_workbench.memory import Memory
        with patch.dict(os.environ, {'VIVY_WORKBENCH_MEMORY_ROOT': 'unused'}):
            self.assertEqual(memory_status({}), 'error')
            memory = object.__new__(Memory)
            memory.closed = False
            self.assertEqual(memory_status({'memory': memory}), 'ready')
            memory.closed = True
            self.assertEqual(memory_status({'memory': memory}), 'closed')
        with patch.dict(os.environ, {}, clear=True):
            self.assertEqual(memory_status({}), 'off')

    def test_memory_status_never_executes_a_rebound_objects_property(self):
        from vivy_workbench.capabilities import memory_status
        touched = []
        class UserObject:
            @property
            def closed(self):
                touched.append(True)
                raise RuntimeError('must not run during status observation')
        with patch.dict(os.environ, {'VIVY_WORKBENCH_MEMORY_ROOT': 'unused'}):
            self.assertEqual(memory_status({'memory': UserObject()}), 'error')
            self.assertEqual(touched, [])

    def test_complete_artifact_round_trip_and_no_silent_cap(self):
        from vivy_workbench import artifacts
        with tempfile.TemporaryDirectory() as directory, patch.dict(os.environ, {'VIVY_WORKBENCH_ARTIFACTS': directory}):
            value = {'rows': ['中文' * 200000], 'isError': False}
            record = artifacts.save(value, 'application/json')
            self.assertTrue(record['complete'])
            self.assertEqual(json.loads(Path(record['path']).read_text(encoding='utf-8')), value)
            self.assertEqual(record['bytes'], Path(record['path']).stat().st_size)
            with self.assertRaises(ValueError):
                artifacts.save(b'12345', 'application/octet-stream', max_bytes=4)

    def test_trusted_skill_binding_failure_isolation_and_lazy_markdown(self):
        from vivy_workbench import capabilities
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            for name, body in [('fixture_callable', 'def run(value): return value+2'), ('fixture_module', 'answer=42'), ('fixture_bad', 'raise ValueError("broken")')]:
                package = root / name
                package.mkdir()
                (package / '__init__.py').write_text(body, encoding='utf-8')
                (package / 'pyproject.toml').write_text(f'[project]\nname="{name}"\nversion="0.1.0"\n', encoding='utf-8')
            sys.path.insert(0, directory)
            self.addCleanup(sys.path.remove, directory)
            self.addCleanup(lambda: [sys.modules.pop(n, None) for n in ['fixture_callable', 'fixture_module', 'fixture_bad']])
            skills = [{'name': name, 'file': str(root / name / 'SKILL.md')} for name in ['fixture_callable', 'fixture_bad', 'fixture_module']]
            skills.append({'name': 'markdown', 'file': str(root / 'missing-until-read.md')})
            namespace = {}
            inventory = capabilities.load(skills, namespace)
            self.assertEqual(namespace['fixture_callable'](40), 42)
            self.assertEqual(namespace['fixture_module'].answer, 42)
            self.assertNotIn('fixture_bad', namespace)
            self.assertEqual({i['name']: i['state'] for i in inventory}, {'fixture_callable': 'ready', 'fixture_bad': 'unavailable', 'fixture_module': 'ready', 'markdown': 'instructions'})

    def test_reserved_bindings_and_absent_host_requirements_are_unavailable(self):
        from vivy_workbench import capabilities
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / 'pyproject.toml').write_text('[project]\nname="mcp"\nversion="0.1.0"\n', encoding='utf-8')
            namespace = {'mcp': 'existing'}
            records = capabilities.load([{'name': 'mcp', 'file': str(root / 'SKILL.md')}], namespace)
            self.assertEqual(namespace['mcp'], 'existing')
            self.assertEqual(records[0]['state'], 'unavailable')
            (root / 'pyproject.toml').write_text('[project]\nname="example"\nversion="0.1.0"\n[tool.vivy.skill]\nrequires_host=["factory"]\n', encoding='utf-8')
            self.assertEqual(capabilities.load([{'name': 'example', 'file': str(root / 'SKILL.md')}], {})[0]['state'], 'unavailable')


if __name__ == '__main__':
    unittest.main()
