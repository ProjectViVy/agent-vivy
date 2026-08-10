export interface RpcRequest {
  jsonrpc: "2.0";
  id?: string;
  method: string;
  params?: unknown;
}

export interface RpcErrorPayload {
  code: number;
  message: string;
}

export class RpcClientError extends Error {
  constructor(public readonly code: number, message: string) {
    super(message);
    this.name = "RpcClientError";
  }
}

interface RpcResponse<T> {
  jsonrpc: string;
  id: string;
  result?: T;
  error?: RpcErrorPayload;
}

interface Bootstrap {
  protocol_version: string;
  websocket_path: string;
  token: string;
}

export type RpcNotification = (params: unknown) => void;
export type RpcCloseListener = () => void;

export class RpcClient {
  private readonly socket: WebSocket;
  private nextID = 0;
  private readonly pending = new Map<string, {
    resolve: (value: unknown) => void;
    reject: (reason: unknown) => void;
  }>();
  private readonly notificationListeners = new Map<string, Set<RpcNotification>>();
  private readonly closeListeners = new Set<RpcCloseListener>();

  private constructor(socket: WebSocket) {
    this.socket = socket;
    socket.onmessage = (message) => this.receive(message.data);
    socket.onclose = () => {
      clientPromise = null;
      const error = new RpcClientError(-32098, "control plane disconnected");
      for (const waiter of this.pending.values()) waiter.reject(error);
      this.pending.clear();
      for (const listener of this.closeListeners) listener();
    };
  }

  static async connect(): Promise<RpcClient> {
    const response = await fetch("/rpc/bootstrap", { cache: "no-store" });
    if (!response.ok) throw new RpcClientError(-32098, `cannot bootstrap control plane (${response.status})`);
    const bootstrap = (await response.json()) as Bootstrap;
    const scheme = window.location.protocol === "https:" ? "wss:" : "ws:";
    const url = `${scheme}//${window.location.host}${bootstrap.websocket_path}?token=${encodeURIComponent(bootstrap.token)}`;
    const socket = await new Promise<WebSocket>((resolve, reject) => {
      const candidate = new WebSocket(url);
      candidate.onopen = () => resolve(candidate);
      candidate.onerror = () => reject(new RpcClientError(-32098, "cannot connect to control plane"));
    });
    const client = new RpcClient(socket);
    await client.call("initialize", { protocol_version: bootstrap.protocol_version });
    return client;
  }

  call<T>(method: string, params?: unknown): Promise<T> {
    const id = String(++this.nextID);
    const request: RpcRequest = { jsonrpc: "2.0", id, method, params };
    return new Promise<T>((resolve, reject) => {
      this.pending.set(id, { resolve: resolve as (value: unknown) => void, reject });
      try {
        this.socket.send(JSON.stringify(request));
      } catch (error) {
        this.pending.delete(id);
        reject(error);
      }
    });
  }

  notify(method: string, params?: unknown): void {
    this.socket.send(JSON.stringify({ jsonrpc: "2.0", method, params }));
  }

  onNotification(method: string, listener: RpcNotification): () => void {
    let listeners = this.notificationListeners.get(method);
    if (!listeners) {
      listeners = new Set();
      this.notificationListeners.set(method, listeners);
    }
    listeners.add(listener);
    return () => listeners?.delete(listener);
  }

  onClose(listener: RpcCloseListener): () => void {
    this.closeListeners.add(listener);
    return () => this.closeListeners.delete(listener);
  }

  close(): void {
    this.socket.close();
  }

  private receive(data: unknown): void {
    let message: RpcResponse<unknown> & { method?: string; params?: unknown };
    try {
      message = JSON.parse(String(data)) as typeof message;
    } catch {
      return;
    }
    if (message.method) {
      for (const listener of this.notificationListeners.get(message.method) ?? []) listener(message.params);
      return;
    }
    const waiter = this.pending.get(message.id);
    if (!waiter) return;
    this.pending.delete(message.id);
    if (message.error) {
      waiter.reject(new RpcClientError(message.error.code, message.error.message));
    } else {
      waiter.resolve(message.result);
    }
  }
}

let clientPromise: Promise<RpcClient> | null = null;

export function getRpcClient(): Promise<RpcClient> {
  if (!clientPromise) {
    clientPromise = RpcClient.connect().catch((error) => {
      clientPromise = null;
      throw error;
    });
  }
  return clientPromise;
}

export function resetRpcClient(): void {
  clientPromise?.then((client) => client.close()).catch(() => undefined);
  clientPromise = null;
}
