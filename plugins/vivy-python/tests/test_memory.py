"""Real native engine acceptance. Assets must be explicitly prepared first."""
import asyncio
import os
from pathlib import Path
import sys
import subprocess
import threading
import unittest
import uuid

sys.path.insert(0, str(Path(__file__).parents[1] / 'python'))
from vivy_workbench.memory import Memory, MemoryConflict, MemoryIncomplete


@unittest.skipUnless(os.environ.get('VIVY_MEMORY_TEST_ROOT'), 'explicit native-memory acceptance environment required')
class MemoryTests(unittest.IsolatedAsyncioTestCase):
    async def test_preparation_probe_searches_reopens_and_cleans_its_palace(self):
        from vivy_workbench.memory import probe
        directory = Path(os.environ['VIVY_MEMORY_TEST_ROOT']) / 'probes'
        before = set(directory.iterdir()) if directory.exists() else set()
        await asyncio.to_thread(probe, os.environ['VIVY_MEMORY_TEST_ROOT'])
        self.assertEqual(set(directory.iterdir()), before)

    async def asyncSetUp(self):
        self.memory = Memory(os.environ['VIVY_MEMORY_TEST_ROOT'])
        self.source = 'synthetic-' + uuid.uuid4().hex

    async def asyncTearDown(self):
        await self.memory.close()

    async def test_identity_conflict_and_distinct_sources(self):
        import mempalace.paths as paths
        self.assertTrue(Path(paths.lock_dir()).is_relative_to(self.memory.root))
        first = await self.memory.remember('工作台的总和是四十二。', source_id=self.source, event_id='one')
        again = await self.memory.remember('工作台的总和是四十二。', source_id=self.source, event_id='one')
        self.assertEqual(first['id'], again['id'])
        self.assertTrue(again['deduplicated'])
        with self.assertRaises(MemoryConflict):
            await self.memory.remember('changed', source_id=self.source, event_id='one')
        with self.assertRaises(MemoryConflict):
            await self.memory.remember('工作台的总和是四十二。', source_id=self.source, event_id='one', room='other')
        second = await self.memory.remember('工作台的总和是四十二。', source_id=self.source+'-other', event_id='one')
        self.assertNotEqual(first['id'], second['id'])
        self.assertEqual((await self.memory.get(first['id']))['content'], '工作台的总和是四十二。')
        results = await self.memory.search('工作台总和', source_id=self.source)
        self.assertEqual(results[0]['id'], first['id'])

    async def test_first_write_creates_a_fresh_palace(self):
        self.memory.palace = str(self.memory.root / ('fresh-' + uuid.uuid4().hex))
        record = await self.memory.remember('Fresh native database.', source_id=self.source, event_id='fresh')
        self.assertEqual((await self.memory.get(record['id']))['content'], 'Fresh native database.')

    async def test_partitioned_search_and_incomplete_read(self):
        record = await self.memory.remember('Partitioned synthetic content.' * 200, source_id=self.source, event_id='chunks', wing='coding', room='tests')
        self.assertFalse(await self.memory.search('content', source_id=self.source, wing='daily'))
        self.assertEqual((await self.memory.search('content', source_id=self.source, wing='coding', room='tests'))[0]['id'], record['id'])
        await self.memory._call(lambda collection: collection.delete(ids=[record['id'] + '_chunk_000001']))
        with self.assertRaises(MemoryIncomplete):
            await self.memory.get(record['id'])
        with self.assertRaises(MemoryIncomplete):
            await self.memory.remember('Partitioned synthetic content.' * 200, source_id=self.source, event_id='chunks', wing='coding', room='tests')

    async def test_reads_do_not_create_palaces(self):
        self.memory.palace = str(self.memory.root / ('absent-' + uuid.uuid4().hex))
        with self.assertRaises(Exception):
            await self.memory.get(source_id=self.source, event_id='absent')
        self.assertFalse(Path(self.memory.palace).exists())

    async def test_two_process_writers_share_one_palace(self):
        code = "import sys,asyncio;sys.path.insert(0,sys.argv[1]);from vivy_workbench.memory import Memory\nasync def run():\n m=Memory(sys.argv[2])\n try: await m.remember('Competing synthetic writer.',source_id=sys.argv[3],event_id='concurrent')\n finally: await m.close()\nasyncio.run(run())"
        children = [subprocess.Popen([sys.executable, '-I', '-c', code, str(Path(__file__).parents[1]/'python'), str(self.memory.root), self.source+str(i)], stdout=subprocess.DEVNULL, stderr=subprocess.PIPE) for i in range(2)]
        try:
            for child in children:
                _, error = await asyncio.to_thread(child.communicate, timeout=45)
                self.assertEqual(child.returncode, 0, error.decode(errors='replace'))
            for i in range(2):
                self.assertEqual((await self.memory.get(source_id=self.source+str(i), event_id='concurrent'))['content'], 'Competing synthetic writer.')
        finally:
            for child in children:
                if child.poll() is None:
                    child.kill()
                    child.wait()

    async def test_cancelled_waiter_does_not_replay_native_operation(self):
        started, release = threading.Event(), threading.Event()
        count = []
        def operation(collection):
            started.set()
            release.wait(5)
            count.append('once')
        task = asyncio.create_task(self.memory._call(operation))
        self.assertTrue(await asyncio.to_thread(started.wait, 10))
        task.cancel()
        with self.assertRaises(asyncio.CancelledError):
            await task
        release.set()
        await self.memory.close()
        self.assertEqual(count, ['once'])

    async def test_stored_dimension_mismatch_refuses_operations(self):
        from mempalace.backends.base import EmbedderIdentity
        self.memory.palace = str(self.memory.root / ('wrong-dimension-' + uuid.uuid4().hex))
        record = await self.memory.remember('Dimension fixture.', source_id=self.source, event_id='dimension')
        await self.memory._call(lambda c: c.set_embedder_identity(EmbedderIdentity(model_name='embeddinggemma', dimension=123)), write=True)
        with self.assertRaisesRegex(RuntimeError, 'dimension'):
            await self.memory.get(record['id'])

    async def test_foreign_lock_is_bounded_before_writing(self):
        code = "import sys,time;sys.path.insert(0,sys.argv[1]);from vivy_workbench.memory import configure;configure(sys.argv[2]);from mempalace.palace import mine_palace_lock\nwith mine_palace_lock(sys.argv[3]):\n print('LOCKED',flush=True)\n time.sleep(30)"
        child = subprocess.Popen([sys.executable,'-I','-c',code,str(Path(__file__).parents[1]/'python'),str(self.memory.root),self.memory.palace], stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
        try:
            self.assertEqual((await asyncio.to_thread(child.stdout.readline)).strip(), 'LOCKED')
            start = asyncio.get_running_loop().time()
            with self.assertRaisesRegex(RuntimeError, 'busy; no write started'):
                await self.memory.remember('Must not write.', source_id=self.source, event_id='busy')
            self.assertLess(asyncio.get_running_loop().time()-start, 7)
        finally:
            child.kill()
            child.wait(5)
            child.stdout.close()
            child.stderr.close()
        with self.assertRaises(KeyError):
            await self.memory.get(source_id=self.source, event_id='busy')

    async def test_complete_long_text_reopen_and_responsive_event_loop(self):
        text = '这是完整保留的合成测试内容。\n' * 700
        operation = asyncio.create_task(self.memory.remember(text, source_id=self.source, event_id='long'))
        await asyncio.sleep(.01)
        # A blocking embed/write on the protocol event loop would miss this bound.
        marker = asyncio.get_running_loop().time()
        await asyncio.sleep(.01)
        self.assertLess(asyncio.get_running_loop().time() - marker, .5)
        record = await operation
        await self.memory.close()
        self.memory = Memory(os.environ['VIVY_MEMORY_TEST_ROOT'])
        self.assertEqual((await self.memory.get(record['id']))['content'], text)
        self.assertNotIn('mempalace.mcp_server', sys.modules)


if __name__ == '__main__':
    unittest.main()
