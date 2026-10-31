import json
from pathlib import Path
import sys
import tempfile
import unittest
import hashlib
import subprocess
import zipfile

sys.path.insert(0, str(Path(__file__).parents[1] / 'python'))
from vivy_workbench import prepare


class PreparationTests(unittest.TestCase):
    def test_managed_dependency_stage_handles_spaces_and_preserves_current(self):
        with tempfile.TemporaryDirectory(prefix='d4 staged dependencies ') as directory:
            root=Path(directory);lock=root/'empty.lock';lock.write_text('',encoding='utf-8')
            wheel=root/'vivyd4_fixturedep-1.0-py3-none-any.whl'
            with zipfile.ZipFile(wheel,'w') as archive:
                archive.writestr('vivyd4_fixturedep/__init__.py','VALUE=42\n')
                archive.writestr('vivyd4_fixturedep-1.0.dist-info/METADATA','Metadata-Version: 2.1\nName: vivyd4-fixturedep\nVersion: 1.0\n')
                archive.writestr('vivyd4_fixturedep-1.0.dist-info/WHEEL','Wheel-Version: 1.0\nRoot-Is-Purelib: true\nTag: py3-none-any\n')
                archive.writestr('vivyd4_fixturedep-1.0.dist-info/RECORD','')
            prepare.prepare(root,lock,offline=True)
            before=(root/'current.json').read_bytes()
            staged=prepare.prepare(root,lock,offline=True,publish=False,scope='dependency',managed_requirements=['vivyd4-fixturedep @ '+wheel.as_uri()])
            result=subprocess.run([staged['executable'],'-I','-c','import vivyd4_fixturedep;assert vivyd4_fixturedep.VALUE==42'],capture_output=True)
            self.assertEqual(result.returncode,0,result.stderr)
            self.assertEqual((root/'current.json').read_bytes(),before)

    def test_stage_only_never_replaces_current_environment(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            lock = root / 'empty.lock'
            lock.write_text('', encoding='utf-8')
            first = prepare.prepare(root, lock, offline=True, scope='first')
            before = (root / 'current.json').read_bytes()
            staged = prepare.prepare(root, lock, offline=True, scope='candidate', publish=False)
            self.assertTrue(Path(staged['executable']).is_file())
            self.assertNotEqual(first['executable'], staged['executable'])
            self.assertEqual((root / 'current.json').read_bytes(), before)

    def test_unvalidated_publication_is_not_executed_when_reuse_is_disabled(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            lock = root/'empty.lock'
            lock.write_text('', encoding='utf-8')
            first = prepare.prepare(root, lock, offline=True)
            site = Path(subprocess.check_output([first['executable'], '-I', '-c', 'import sysconfig;print(sysconfig.get_path("purelib"))'], text=True).strip())
            marker = root/'untrusted-started'
            (site/'untrusted.pth').write_text('import pathlib;pathlib.Path(' + repr(str(marker)) + ').touch()\n', encoding='utf-8')
            second = prepare.prepare(root, lock, offline=True, reuse=False)
            self.assertNotEqual(first['executable'], second['executable'])
            self.assertFalse(marker.exists())

    def test_memory_detection_rejects_equal_size_asset_corruption(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            lock = root/'empty.lock'
            lock.write_text('', encoding='utf-8')
            current = prepare.prepare(root, lock, offline=True, scope='native-fixture')
            current['memory_ready'] = True
            (root/'current.json').write_text(json.dumps(current), encoding='utf-8')
            memory = root/'native-memory'
            memory.mkdir()
            asset = memory/'fixture.onnx'
            asset.write_bytes(b'original')
            (memory/'embedding.json').write_text(json.dumps({'files':[{'path':str(asset),'bytes':8,'sha256':hashlib.sha256(b'original').hexdigest()}]}), encoding='utf-8')
            self.assertEqual(prepare.check(root, scope='native-fixture', native_memory=True)['outcome'], 'ready')
            asset.write_bytes(b'modified')
            result = prepare.check(root, scope='native-fixture', native_memory=True)
            self.assertEqual(result['outcome'], 'error')
            self.assertIn('hash mismatch', result['detail'])

    def test_import_failure_repairs_instead_of_republishing_broken_environment(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            lock = root / 'empty.lock'
            lock.write_text('', encoding='utf-8')
            first = prepare.prepare(root, lock, offline=True)
            damaged = dict(first, imports=['vivy_missing_fixture_module'])
            (root/'current.json').write_text(json.dumps(damaged), encoding='utf-8')
            self.assertEqual(prepare.check(root)['outcome'], 'error')
            repaired = prepare.prepare(root, lock, offline=True)
            self.assertNotEqual(first['executable'], repaired['executable'])
            self.assertEqual(prepare.check(root)['outcome'], 'ready')

    def test_detection_is_read_only_and_scope_changes_require_reprepare(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            lock = root / 'empty.lock'
            lock.write_text('', encoding='utf-8')
            self.assertEqual(prepare.check(root, scope='approved')['outcome'], 'not_init')
            first = prepare.prepare(root, lock, offline=True, scope='approved')
            self.assertEqual(prepare.check(root, scope='approved')['outcome'], 'ready')
            self.assertEqual(prepare.check(root, scope='changed')['outcome'], 'not_init')
            self.assertEqual(json.loads((root / 'current.json').read_text()), first)
            (root / 'current.json').write_text('{bad', encoding='utf-8')
            self.assertEqual(prepare.check(root, scope='approved')['outcome'], 'error')

    def test_corrupt_pointer_can_be_repaired_explicitly(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            lock = root / 'empty.lock'
            lock.write_text('', encoding='utf-8')
            (root / 'current.json').write_text('{broken', encoding='utf-8')
            self.assertTrue(prepare.prepare(root, lock, offline=True)['executable'])

    def test_failed_install_preserves_the_published_environment(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            lock = root / 'empty.lock'
            lock.write_text('', encoding='utf-8')
            first = prepare.prepare(root, lock, python=sys.executable, offline=True)
            self.assertTrue(Path(first['executable']).is_file())
            broken = root / 'broken.lock'
            broken.write_text('vivy-nonexistent-fixture-7a192==0.0.1\n', encoding='utf-8')
            with self.assertRaises(RuntimeError):
                prepare.prepare(root, broken, python=sys.executable, offline=True)
            self.assertEqual(json.loads((root / 'current.json').read_text()), first)
            second = prepare.prepare(root, lock, python=sys.executable, offline=True)
            self.assertEqual(first, second)

    def test_competing_preparer_gets_busy_without_publishing(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            lock = root / 'empty.lock'
            lock.write_text('', encoding='utf-8')
            with prepare.environment_lock(root):
                with self.assertRaisesRegex(RuntimeError, 'busy'):
                    prepare.prepare(root, lock, python=sys.executable, offline=True)
            self.assertFalse((root / 'current.json').exists())
            self.assertTrue(prepare.prepare(root, lock, python=sys.executable, offline=True))


if __name__ == '__main__':
    unittest.main()
