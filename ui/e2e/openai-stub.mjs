import http from 'node:http';

const PORT = Number(process.env.VIVY_E2E_STUB_PORT || 8800);

function lastUserText(body) {
  const messages = Array.isArray(body?.messages) ? body.messages : [];
  for (let i = messages.length - 1; i >= 0; i -= 1) {
    if (messages[i]?.role === 'user') return String(messages[i].content ?? '');
  }
  return '';
}

function hasToolResult(body) {
  const messages = Array.isArray(body?.messages) ? body.messages : [];
  return messages.some((message) => message?.role === 'tool');
}

function sse(res, payload) {
  res.write(`data: ${JSON.stringify(payload)}\n\n`);
}

function streamText(res, text) {
  sse(res, {
    id: 'stub-1',
    object: 'chat.completion.chunk',
    choices: [{ index: 0, delta: { role: 'assistant', content: text }, finish_reason: null }],
  });
  sse(res, {
    id: 'stub-1',
    object: 'chat.completion.chunk',
    choices: [{ index: 0, delta: {}, finish_reason: 'stop' }],
  });
  res.write('data: [DONE]\n\n');
  res.end();
}

function streamToolCall(res) {
  sse(res, {
    id: 'stub-tool',
    object: 'chat.completion.chunk',
    choices: [{
      index: 0,
      delta: {
        role: 'assistant',
        tool_calls: [{ index: 0, id: 'call_e2e', type: 'function', function: { name: 'write_note', arguments: '' } }],
      },
      finish_reason: null,
    }],
  });
  sse(res, {
    id: 'stub-tool',
    object: 'chat.completion.chunk',
    choices: [{
      index: 0,
      delta: { tool_calls: [{ index: 0, function: { arguments: '{"content":"e2e approval note"}' } }] },
      finish_reason: null,
    }],
  });
  sse(res, {
    id: 'stub-tool',
    object: 'chat.completion.chunk',
    choices: [{ index: 0, delta: {}, finish_reason: 'tool_calls' }],
  });
  res.write('data: [DONE]\n\n');
  res.end();
}

const server = http.createServer((req, res) => {
  if (req.url === '/healthz') {
    res.writeHead(200, { 'content-type': 'application/json' });
    res.end(JSON.stringify({ status: 'ok' }));
    return;
  }
  if (req.method !== 'POST' || !req.url?.includes('/chat/completions')) {
    res.writeHead(404);
    res.end();
    return;
  }
  const chunks = [];
  req.on('data', (chunk) => chunks.push(chunk));
  req.on('end', () => {
    let body = {};
    try { body = JSON.parse(Buffer.concat(chunks).toString('utf8') || '{}'); } catch { body = {}; }
    res.writeHead(200, { 'content-type': 'text/event-stream', 'cache-control': 'no-cache' });
    const text = lastUserText(body);
    if (/e2e approval/i.test(text) && !hasToolResult(body)) {
      streamToolCall(res);
      return;
    }
    streamText(res, `stub reply to: ${text || 'empty'}`);
  });
});

server.listen(PORT, '127.0.0.1', () => {
  process.stdout.write(`openai stub listening on 127.0.0.1:${PORT}\n`);
});
