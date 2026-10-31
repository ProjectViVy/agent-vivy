import asyncio
import os
import pathlib
import socket
import subprocess
import sys
import unittest
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from unittest.mock import patch

ROOT = pathlib.Path(__file__).parents[1]
sys.path.insert(0, str(ROOT / 'python'))
from vivy_workbench import mcp


class McpTests(unittest.IsolatedAsyncioTestCase):
    async def asyncSetUp(self):
        self.environment = patch.dict(os.environ, {'NO_PROXY': '*', 'no_proxy': '*'})
        self.environment.start()
        self.addCleanup(self.environment.stop)
        mcp._registry = mcp._Registry()
        self.config = {'type': 'stdio', 'command': sys.executable,
                       'args': [str(ROOT / 'tests/mcp_fixture.py')]}
        async def host(kind, payload):
            if kind == 'mcp.config':
                return self.config.copy()
            if kind == 'mcp.list_connections':
                return {'connections': [{'connectionId': 'fixture', 'name': 'fixture'}]}
            raise AssertionError(kind)
        mcp.host_request = host

    async def asyncTearDown(self):
        await mcp.close()

    async def test_stdio_full_envelope_error_and_reload(self):
        self.assertEqual((await mcp.list_connections())[0]['connectionId'], 'fixture')
        tools = await mcp.list_tools('fixture')
        self.assertEqual({t['name'] for t in tools}, {'dataset', 'failure'})
        described = await mcp.describe_tool('fixture', 'dataset')
        self.assertEqual(described['title'], 'Synthetic dataset')
        self.assertTrue(described['annotations']['readOnlyHint'])
        result = await mcp.call_tool('fixture', 'dataset')
        self.assertEqual(result['structuredContent']['rows'][1]['value'], 22)
        self.assertTrue(result['content'])
        self.assertFalse(result['isError'])
        with self.assertRaises(mcp.McpToolError) as failure:
            await mcp.call_tool('fixture', 'failure')
        self.assertTrue(failure.exception.result['isError'])
        await mcp.reload('fixture')
        self.assertEqual((await mcp.call_tool('fixture', 'dataset'))['structuredContent']['rows'][0]['value'], 20)

    async def test_unscoped_search_uses_configured_lazy_connections(self):
        result = await mcp.search_tools('dataset')
        self.assertEqual(result['searched'], ['fixture'])
        self.assertEqual(result['tools'][0]['name'], 'dataset')

    async def test_parallel_first_open_and_close(self):
        results = await asyncio.gather(*(mcp.list_tools('fixture') for _ in range(5)))
        self.assertTrue(all(len(result) == 2 for result in results))
        await mcp.close()
        with self.assertRaises(RuntimeError):
            await mcp.list_tools('fixture')

    async def test_pages_filter_credentials_timeout_and_dead_start(self):
        self.config['args'] = [str(ROOT / 'tests/mcp_pages_fixture.py'), 'pages']
        self.config['env'] = {'SELECTED_FIXTURE': 'selected'}
        self.config['callTimeoutMs'] = 150
        with patch.dict(os.environ, {'RIG_SECRET_FIXTURE': 'must-not-reach-child'}):
            self.assertEqual({t['name'] for t in await mcp.list_tools('fixture')}, {'first', 'second'})
            result = (await mcp.call_tool('fixture', 'second'))['structuredContent']
            self.assertFalse(result['secret_present'])
            self.assertEqual(result['selected'], 'selected')
            with self.assertRaises(TimeoutError):
                await mcp.call_tool('fixture', 'first', {'slow': True})
        await mcp.reload('fixture')
        self.config['args'] = ['-c', 'import sys;sys.exit(7)']
        with self.assertRaises(Exception):
            await mcp.list_tools('fixture')

    async def test_repeated_cursor_and_close_during_open(self):
        self.config['args'] = [str(ROOT / 'tests/mcp_pages_fixture.py'), 'repeat']
        with self.assertRaises(Exception):
            await mcp.list_tools('fixture')
        self.config['args'][-1] = 'slow-open'
        opening = asyncio.create_task(mcp.list_tools('fixture'))
        await asyncio.sleep(.2)
        await asyncio.wait_for(mcp.close(), 5)
        result = await asyncio.gather(opening, return_exceptions=True)
        self.assertIsInstance(result[0], BaseException)

    async def test_real_streamable_http(self):
        with socket.socket() as bound:
            bound.bind(('127.0.0.1', 0))
            port = bound.getsockname()[1]
        p = subprocess.Popen([sys.executable, str(ROOT / 'tests/mcp_fixture.py'), str(port)],
                             stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        try:
            for _ in range(100):
                try:
                    reader, writer = await asyncio.open_connection('127.0.0.1', port)
                    writer.close()
                    await writer.wait_closed()
                    break
                except OSError:
                    await asyncio.sleep(.05)
            self.config = {'type': 'http', 'url': f'http://127.0.0.1:{port}/mcp'}
            result = await mcp.call_tool('fixture', 'dataset')
            self.assertEqual(len(result['structuredContent']['rows']), 2)
            await mcp.close()
        finally:
            p.terminate()
            p.wait(5)

    async def test_http_authentication_failure_preserves_status_and_redacts(self):
        class Rejected(BaseHTTPRequestHandler):
            def do_POST(self):
                self.send_response(401)
                self.send_header('Content-Length', '0')
                self.end_headers()
            def log_message(self, *args):
                pass
        server = ThreadingHTTPServer(('127.0.0.1', 0), Rejected)
        threading.Thread(target=server.serve_forever, daemon=True).start()
        self.config = {'type': 'http', 'url': f'http://127.0.0.1:{server.server_port}/mcp',
                       'headers': {'Authorization': 'Bearer private-fixture-value'}}
        try:
            with self.assertRaisesRegex(mcp.McpStartupError, '401') as failure:
                await mcp.list_tools('fixture')
            self.assertNotIn('private-fixture-value', str(failure.exception))
        finally:
            await asyncio.to_thread(server.shutdown)
            server.server_close()


if __name__ == '__main__':
    unittest.main()
