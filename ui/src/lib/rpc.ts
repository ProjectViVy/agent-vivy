export interface RpcRequest {
  jsonrpc: '2.0';
  id?: string;
  method: string;
  params?: unknown;
}

export class RpcClientError extends Error {
  constructor(public readonly code: number, message: string) {
    super(message);
    this.name = 'RpcClientError';
  }
}

interface RpcResponse<T> {
  jsonrpc: string;
  id: string;
  result?: T;
  error?: { code: number; message: string };
}

export interface RpcCapabilities {
  protocol_version: string;
  capabilities: string[];
}

interface Bootstrap {
  protocol_version: string;
  websocket_path: string;
  token: string;
}

type NotificationListener = (params: unknown) => void;
type CloseListener = () => void;

export class RpcClient {
  private nextId = 0;
  private readonly pending = new Map<string, { resolve: (value: unknown) => void; reject: (reason: unknown) => void }>();
  private readonly notifications = new Map<string, Set<NotificationListener>>();
  private readonly closeListeners = new Set<CloseListener>();

  private constructor(private readonly socket: WebSocket, public readonly capabilities: RpcCapabilities) {
    socket.onmessage = (message) => this.receive(message.data);
    socket.onclose = () => {
      clientPromise = null;
      const error = new RpcClientError(-32098, 'Vivy control plane disconnected');
      for (const waiter of this.pending.values()) waiter.reject(error);
      this.pending.clear();
      for (const listener of this.closeListeners) listener();
    };
  }

  static async connect(): Promise<RpcClient> {
    const response = await fetch('/rpc/bootstrap', { cache: 'no-store' });
    if (!response.ok) throw new RpcClientError(-32098, `无法连接 Vivy control plane（HTTP ${response.status}）`);
    const contentType = response.headers.get('content-type') ?? '';
    if (!contentType.toLowerCase().includes('application/json')) {
      throw new RpcClientError(-32098, '无法连接 Vivy control plane：/rpc/bootstrap 返回了非 JSON 响应，请确认后端已启动且开发代理配置正确');
    }
    let bootstrap: Bootstrap;
    try {
      bootstrap = await response.json() as Bootstrap;
    } catch {
      throw new RpcClientError(-32098, 'Vivy control plane bootstrap 响应不是有效 JSON');
    }
    if (!bootstrap.websocket_path || !bootstrap.token) {
      throw new RpcClientError(-32098, 'Vivy control plane bootstrap 响应缺少连接信息');
    }
    const scheme = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const url = `${scheme}//${window.location.host}${bootstrap.websocket_path}?token=${encodeURIComponent(bootstrap.token)}`;
    const socket = await new Promise<WebSocket>((resolve, reject) => {
      const candidate = new WebSocket(url);
      candidate.onopen = () => resolve(candidate);
      candidate.onerror = () => reject(new RpcClientError(-32098, '无法建立 Vivy control plane WebSocket'));
    });
    const provisional = new RpcClient(socket, { protocol_version: bootstrap.protocol_version, capabilities: [] });
    const capabilities = await provisional.call<RpcCapabilities>('initialize', { protocol_version: bootstrap.protocol_version });
    provisional.capabilities.protocol_version = capabilities.protocol_version;
    provisional.capabilities.capabilities = capabilities.capabilities ?? [];
    return provisional;
  }

  call<T>(method: string, params?: unknown): Promise<T> {
    const id = String(++this.nextId);
    return new Promise<T>((resolve, reject) => {
      this.pending.set(id, { resolve: resolve as (value: unknown) => void, reject });
      try {
        this.socket.send(JSON.stringify({ jsonrpc: '2.0', id, method, params } satisfies RpcRequest));
      } catch (error) {
        this.pending.delete(id);
        reject(error);
      }
    });
  }

  onNotification(method: string, listener: NotificationListener): () => void {
    const listeners = this.notifications.get(method) ?? new Set<NotificationListener>();
    listeners.add(listener);
    this.notifications.set(method, listeners);
    return () => listeners.delete(listener);
  }

  onClose(listener: CloseListener): () => void {
    this.closeListeners.add(listener);
    return () => this.closeListeners.delete(listener);
  }

  close(): void { this.socket.close(); }

  private receive(data: unknown): void {
    let message: RpcResponse<unknown> & { method?: string; params?: unknown };
    try { message = JSON.parse(String(data)) as typeof message; } catch { return; }
    if (message.method) {
      for (const listener of this.notifications.get(message.method) ?? []) listener(message.params);
      return;
    }
    const waiter = this.pending.get(message.id);
    if (!waiter) return;
    this.pending.delete(message.id);
    if (message.error) waiter.reject(new RpcClientError(message.error.code, message.error.message));
    else waiter.resolve(message.result);
  }
}

let clientPromise: Promise<RpcClient> | null = null;

export function getRpcClient(): Promise<RpcClient> {
  if (!clientPromise) clientPromise = RpcClient.connect().catch((error) => { clientPromise = null; throw error; });
  return clientPromise;
}

export function resetRpcClient(): void {
  const current = clientPromise;
  clientPromise = null;
  current?.then((client) => client.close()).catch(() => undefined);
}
