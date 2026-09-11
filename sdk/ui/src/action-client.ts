/** The only JSON-RPC method exposed by the UI SDK for Module Control Actions. */
export const MODULE_ACTION_METHOD = "module.action.invoke" as const;

/** Keep UI-side serialization bounded before a request reaches the transport. */
export const MODULE_ACTION_MAX_INPUT_BYTES = 1 << 20;
export const MODULE_ACTION_MAX_DEPTH = 32;
export const MODULE_ACTION_MAX_IDENTIFIER_BYTES = 256;

export interface ModuleActionRPC {
  call<T = unknown>(method: string, params?: unknown): Promise<T>;
}

export interface ModuleActionRequest<Input = unknown> {
  readonly moduleId: string;
  readonly actionId: string;
  readonly input: Input;
}

export interface ModuleActionClientOptions {
  readonly maxInputBytes?: number;
  readonly maxDepth?: number;
}

/** A bounded client-side error for malformed input or transport errors. */
export class ModuleActionClientError extends Error {
  constructor(public readonly code: string | number, message: string) {
    super(message);
    this.name = "ModuleActionClientError";
  }
}

/**
 * Typed transport adapter for `module.action.invoke`.
 *
 * The client knows only how to encode a bounded request. It does not inspect
 * active Generations, Grants, approval, Trust, or any other authority state;
 * those decisions remain exclusively on the server ActionHost.
 */
export class ModuleActionClient {
  private readonly maxInputBytes: number;
  private readonly maxDepth: number;

  constructor(
    private readonly rpc: ModuleActionRPC,
    options: ModuleActionClientOptions = {},
  ) {
    this.maxInputBytes = normalizeBound(options.maxInputBytes, MODULE_ACTION_MAX_INPUT_BYTES);
    this.maxDepth = normalizeBound(options.maxDepth, MODULE_ACTION_MAX_DEPTH);
  }

  invoke<Input = unknown, Result = unknown>(request: ModuleActionRequest<Input>): Promise<Result>;
  invoke<Input = unknown, Result = unknown>(moduleId: string, actionId: string, input: Input): Promise<Result>;
  async invoke<Input = unknown, Result = unknown>(
    requestOrModule: ModuleActionRequest<Input> | string,
    actionOrUndefined?: string,
    inputOrUndefined?: Input,
  ): Promise<Result> {
    const request: ModuleActionRequest<Input> = typeof requestOrModule === "string"
      ? { moduleId: requestOrModule, actionId: actionOrUndefined ?? "", input: inputOrUndefined as Input }
      : requestOrModule;
    const moduleId = normalizeIdentifier(request?.moduleId);
    const actionId = normalizeIdentifier(request?.actionId);
    const input = serializeBoundedInput(request?.input, this.maxInputBytes, this.maxDepth);
    return this.rpc.call<Result>(MODULE_ACTION_METHOD, {
      module_id: moduleId,
      action_id: actionId,
      input,
    });
  }

  /** Alias for callers that name this operation after the transport call. */
  call<Input = unknown, Result = unknown>(request: ModuleActionRequest<Input>): Promise<Result>;
  call<Input = unknown, Result = unknown>(moduleId: string, actionId: string, input: Input): Promise<Result>;
  call<Input = unknown, Result = unknown>(
    requestOrModule: ModuleActionRequest<Input> | string,
    actionOrUndefined?: string,
    inputOrUndefined?: Input,
  ): Promise<Result> {
    if (typeof requestOrModule === "string") {
      return this.invoke<Input, Result>(requestOrModule, actionOrUndefined ?? "", inputOrUndefined as Input);
    }
    return this.invoke<Input, Result>(requestOrModule);
  }
}

export function createModuleActionClient(
  rpc: ModuleActionRPC,
  options?: ModuleActionClientOptions,
): ModuleActionClient {
  return new ModuleActionClient(rpc, options);
}

/** Descriptive aliases used by UI Modules that call this an Action client. */
export const ActionClient = ModuleActionClient;
export type ActionClient = ModuleActionClient;
export const createActionClient = createModuleActionClient;

function normalizeBound(value: number | undefined, fallback: number): number {
  if (value === undefined) return fallback;
  if (!Number.isSafeInteger(value) || value <= 0 || value > fallback) {
    throw new ModuleActionClientError("ACTION_INPUT_BOUNDED", "action client bounds are invalid");
  }
  return value;
}

function normalizeIdentifier(value: unknown): string {
  if (typeof value !== "string") {
    throw new ModuleActionClientError("ACTION_REQUEST_INVALID", "module and action IDs are required");
  }
  const normalized = value.trim();
  if (normalized.length === 0 || new TextEncoder().encode(normalized).byteLength > MODULE_ACTION_MAX_IDENTIFIER_BYTES) {
    throw new ModuleActionClientError("ACTION_REQUEST_INVALID", "module and action IDs are invalid");
  }
  return normalized;
}

function serializeBoundedInput(value: unknown, maxBytes: number, maxDepth: number): unknown {
  let serialized: string | undefined;
  try {
    serialized = JSON.stringify(value);
  } catch {
    throw new ModuleActionClientError("ACTION_INPUT_BOUNDED", "action input is not JSON serializable");
  }
  if (serialized === undefined) {
    throw new ModuleActionClientError("ACTION_INPUT_BOUNDED", "action input is not JSON serializable");
  }
  if (new TextEncoder().encode(serialized).byteLength > maxBytes) {
    throw new ModuleActionClientError("ACTION_INPUT_BOUNDED", "action input exceeds the client bound");
  }
  let parsed: unknown;
  try {
    parsed = JSON.parse(serialized) as unknown;
  } catch {
    throw new ModuleActionClientError("ACTION_INPUT_BOUNDED", "action input is not valid JSON");
  }
  if (jsonDepth(parsed, maxDepth) > maxDepth) {
    throw new ModuleActionClientError("ACTION_INPUT_BOUNDED", "action input is too deeply nested");
  }
  return parsed;
}

function jsonDepth(value: unknown, limit: number): number {
  const pending: Array<{ readonly value: unknown; readonly depth: number }> = [{ value, depth: 0 }];
  let maximum = 0;
  while (pending.length > 0) {
    const current = pending.pop();
    if (!current) break;
    if (current.value === null || typeof current.value !== "object") {
      maximum = Math.max(maximum, current.depth);
      continue;
    }
    const next = current.depth + 1;
    maximum = Math.max(maximum, next);
    if (next > limit) return next;
    const children = Array.isArray(current.value)
      ? current.value
      : Object.values(current.value as Record<string, unknown>);
    for (const child of children) pending.push({ value: child, depth: next });
  }
  return maximum;
}
