// Disposable OpenAI-compatible endpoint for the masks e2e split pair.
// /healthz answers so Playwright can detect readiness; the chat-completions
// route intentionally never responds, which keeps a run `active` for as long
// as the test needs (mask selection must still queue for the next run).
import http from 'node:http';

const server = http.createServer((req, res) => {
  if (req.method === 'GET' && req.url === '/healthz') {
    res.writeHead(200, { 'content-type': 'text/plain' });
    res.end('ok');
    return;
  }
  // Everything else hangs: the model call never completes.
  req.on('error', () => {});
});

server.listen(9911, '127.0.0.1');
process.on('SIGTERM', () => server.close(() => process.exit(0)));
process.on('SIGINT', () => server.close(() => process.exit(0)));
