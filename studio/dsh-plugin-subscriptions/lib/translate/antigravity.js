/**
 * DeepSeek Harness message/tool vocabulary to Antigravity's Gemini-shaped
 * v1internal request envelope, plus response/SSE translation back to the
 * harness streaming contract.
 */
import { randomUUID } from 'node:crypto';
import { CallId, EMPTY_RESPONSE_CODE, LlmError } from '@deepseek-ai/dsh-llm';
import { parseSse } from './sse.js';
/** Flatten a harness tool result to the JSON value Antigravity receives. */
function toolResultValue(block) {
    const text = block.content.map(part => part.type === 'text' ? part.text : '').join('');
    try {
        return JSON.parse(text);
    }
    catch {
        return { output: text, ...block.isError === true ? { isError: true } : {} };
    }
}
/** Safely read per-block replay metadata emitted by this adapter. */
function replayBlocks(message) {
    const source = message.source;
    if (source?.kind !== 'model' || typeof source.replayState !== 'object' || source.replayState === null)
        return [];
    const envelope = source.replayState;
    const response = envelope.response;
    if (response?.kind !== 'antigravity' || response.version !== 1 || !Array.isArray(envelope.blocks))
        return [];
    return envelope.blocks.map((entry) => {
        if (typeof entry !== 'object' || entry === null)
            return {};
        const signature = entry.thoughtSignature;
        return typeof signature === 'string' && signature.length > 0 ? { thoughtSignature: signature } : {};
    });
}
/** Map harness tool schemas to Gemini function declarations. */
export function toAntigravityTools(tools) {
    if (tools.length === 0)
        return [];
    return [{
            functionDeclarations: tools.map(tool => ({
                name: tool.name,
                description: tool.description,
                parameters: tool.parameters,
            })),
        }];
}
/**
 * Convert resolved harness messages into Gemini contents. Function response
 * names are recovered from prior tool calls because DSH correlates results by
 * id while the Gemini wire requires both id and name.
 */
export function toAntigravityContents(messages) {
    const out = [];
    const callNames = new Map();
    for (const message of messages) {
        if (message.role === 'system')
            continue;
        const role = message.role === 'assistant' ? 'model' : 'user';
        const metadata = replayBlocks(message);
        const parts = [];
        for (let index = 0; index < message.content.length; index++) {
            const block = message.content[index];
            switch (block.type) {
                case 'text':
                    // Antigravity's Claude-backed models reject empty text parts.
                    parts.push({ text: block.text.trim().length > 0 ? block.text : '.' });
                    break;
                case 'image':
                    if ('dataBase64' in block) {
                        parts.push({ inlineData: { mimeType: block.mediaType, data: block.dataBase64 } });
                    }
                    break;
                case 'tool-call': {
                    callNames.set(String(block.id), block.name);
                    let args;
                    try {
                        args = JSON.parse(block.arguments);
                    }
                    catch {
                        args = {};
                    }
                    parts.push({
                        functionCall: { id: String(block.id), name: block.name, args },
                        ...metadata[index]?.thoughtSignature === undefined
                            ? {}
                            : { thoughtSignature: metadata[index].thoughtSignature },
                    });
                    break;
                }
                case 'tool-result': {
                    const id = String(block.toolCallId);
                    parts.push({
                        functionResponse: {
                            id,
                            name: callNames.get(id) ?? '',
                            response: toolResultValue(block),
                        },
                    });
                    break;
                }
                default:
                    // Reasoning is not replayed without its provider signature. The
                    // signature-bearing metadata remains attached to tool-call blocks.
                    break;
            }
        }
        if (parts.length === 0)
            continue;
        const previous = out.at(-1);
        if (previous?.role === role)
            previous.parts.push(...parts);
        else
            out.push({ role, parts });
    }
    return out;
}
/** Build one v1internal generateContent/streamGenerateContent request. */
export function toAntigravityRequest(options, messages, projectId) {
    const tools = toAntigravityTools(options.tools ?? []);
    const generationConfig = {
        ...options.maxTokens === undefined ? {} : { maxOutputTokens: options.maxTokens },
        ...options.temperature === undefined ? {} : { temperature: options.temperature },
        ...options.stop === undefined || options.stop.length === 0 ? {} : { stopSequences: options.stop },
        ...options.reasoningEffort === undefined ? {} : {
            thinkingConfig: { thinkingLevel: String(options.reasoningEffort), includeThoughts: true },
        },
    };
    const systemTexts = messages.flatMap(message => message.role === 'system'
        ? message.content.filter((block) => block.type === 'text').map(block => block.text)
        : []);
    const system = options.system ?? (systemTexts.length > 0 ? systemTexts.join('\n\n') : undefined);
    const sessionId = options.sessionId === undefined ? randomUUID() : String(options.sessionId);
    return {
        project: projectId,
        requestId: `agent/${String(Date.now())}/${randomUUID()}/4`,
        model: options.model,
        userAgent: 'antigravity',
        requestType: 'agent',
        request: {
            contents: toAntigravityContents(messages),
            sessionId,
            ...system === undefined || system.length === 0 ? {} : { systemInstruction: { parts: [{ text: system }] } },
            ...tools.length === 0 ? {} : {
                tools,
                toolConfig: { functionCallingConfig: { mode: 'VALIDATED' } },
            },
            ...Object.keys(generationConfig).length === 0 ? {} : { generationConfig },
        },
    };
}
/** Map Gemini usage metadata to the harness's disjoint counters. */
export function mapAntigravityUsage(metadata) {
    const cached = metadata.cachedContentTokenCount ?? 0;
    return {
        inputTokens: Math.max(0, (metadata.promptTokenCount ?? 0) - cached),
        outputTokens: metadata.candidatesTokenCount ?? 0,
        ...cached > 0 ? { cacheReadTokens: cached } : {},
        ...metadata.thoughtsTokenCount === undefined ? {} : { reasoningTokens: metadata.thoughtsTokenCount },
    };
}
/** Push translator for both parsed SSE events and one non-stream response. */
export class AntigravityStreamTranslator {
    blocks = new Map();
    closed = [];
    nextIndex = 0;
    sawContent = false;
    sawToolCall = false;
    terminated = false;
    open(key, kind, chunks, values = {}) {
        const block = { index: this.nextIndex++, kind, text: '', ...values };
        this.blocks.set(key, block);
        chunks.push({ type: 'block-start', index: block.index, blockType: kind });
        return block;
    }
    close(key, chunks) {
        const block = this.blocks.get(key);
        if (block === undefined)
            return;
        this.blocks.delete(key);
        let content;
        if (block.kind === 'text')
            content = { type: 'text', text: block.text };
        else if (block.kind === 'reasoning')
            content = { type: 'reasoning', text: block.text };
        else
            content = {
                type: 'tool-call',
                id: CallId(block.id ?? `call_${randomUUID().replaceAll('-', '')}`),
                name: block.name ?? '',
                arguments: block.text,
            };
        this.closed[block.index] = block.thoughtSignature === undefined ? {} : { thoughtSignature: block.thoughtSignature };
        chunks.push({ type: 'block-end', index: block.index, block: content });
    }
    closeAll(chunks) {
        for (const key of [...this.blocks.keys()])
            this.close(key, chunks);
    }
    finish(reason) {
        const replayState = {
            response: { kind: 'antigravity', version: 1 },
            blocks: this.closed,
        };
        if (!this.sawContent) {
            return {
                type: 'finish',
                reason: { kind: 'error', failure: { message: 'Antigravity returned no content', code: EMPTY_RESPONSE_CODE } },
            };
        }
        if (reason === 'MAX_TOKENS')
            return { type: 'finish', reason: { kind: 'max-tokens' }, replayState };
        if (reason === 'SAFETY' || reason === 'RECITATION' || reason === 'BLOCKLIST') {
            return {
                type: 'finish',
                reason: { kind: 'error', failure: { message: `Antigravity blocked the response (${reason})`, code: 'CONTENT_FILTER' } },
            };
        }
        return { type: 'finish', reason: { kind: this.sawToolCall ? 'tool-calls' : 'stop' }, replayState };
    }
    /** Process one decoded Antigravity response frame. */
    push(event) {
        if (this.terminated)
            return [];
        const chunks = [];
        const candidate = event.response?.candidates?.[0];
        for (const [partIndex, part] of (candidate?.content?.parts ?? []).entries()) {
            if (part.thought === true && typeof part.text === 'string' && part.text.length > 0) {
                const block = this.blocks.get('reasoning') ?? this.open('reasoning', 'reasoning', chunks);
                block.text += part.text;
                if (part.thoughtSignature !== undefined)
                    block.thoughtSignature = part.thoughtSignature;
                this.sawContent = true;
                chunks.push({ type: 'reasoning-delta', index: block.index, text: part.text });
            }
            else if (typeof part.text === 'string' && part.text.length > 0) {
                const block = this.blocks.get('text') ?? this.open('text', 'text', chunks);
                block.text += part.text;
                if (part.thoughtSignature !== undefined)
                    block.thoughtSignature = part.thoughtSignature;
                this.sawContent = true;
                chunks.push({ type: 'text-delta', index: block.index, text: part.text });
            }
            else if (part.functionCall !== undefined) {
                const call = part.functionCall;
                const id = typeof call.id === 'string' && call.id.length > 0
                    ? call.id
                    : `call_${randomUUID().replaceAll('-', '')}`;
                const key = `call:${id}:${String(partIndex)}`;
                const args = JSON.stringify(call.args ?? {});
                const block = this.open(key, 'tool-call', chunks, {
                    id,
                    name: call.name ?? '',
                    ...part.thoughtSignature === undefined ? {} : { thoughtSignature: part.thoughtSignature },
                });
                block.text = args;
                this.sawContent = true;
                this.sawToolCall = true;
                chunks.push({
                    type: 'tool-call-delta',
                    index: block.index,
                    id: CallId(id),
                    name: call.name ?? '',
                    argumentsDelta: args,
                });
                this.close(key, chunks);
            }
        }
        if (candidate?.finishReason !== undefined) {
            this.closeAll(chunks);
            const usage = event.response?.usageMetadata;
            if (usage !== undefined)
                chunks.push({ type: 'usage', usage: mapAntigravityUsage(usage) });
            chunks.push(this.finish(candidate.finishReason));
            this.terminated = true;
        }
        return chunks;
    }
}
/** Consume Antigravity's SSE response into the DSH streaming contract. */
export async function* streamAntigravity(stream, onActivity) {
    const translator = new AntigravityStreamTranslator();
    for await (const event of parseSse(stream, onActivity)) {
        if (event.data === '[DONE]')
            break;
        let parsed;
        try {
            parsed = JSON.parse(event.data);
        }
        catch {
            throw new LlmError(`malformed Antigravity SSE payload: ${event.data.slice(0, 120)}`, 'MALFORMED_RESPONSE');
        }
        yield* translator.push(parsed);
        if (translator.terminated)
            return;
    }
    if (!translator.terminated) {
        throw new LlmError('Antigravity SSE stream ended before a finish chunk', 'STREAM_CLOSED');
    }
}
/** Translate a non-stream generateContent response using the same state machine. */
export function parseAntigravityResponse(event) {
    return new AntigravityStreamTranslator().push(event);
}
