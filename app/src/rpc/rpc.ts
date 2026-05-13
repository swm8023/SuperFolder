import { methodName, rpc } from './methods_gen';
export { methodName, rpc } from './methods_gen';

export const ERROR_METHOD_NOT_FOUND = 1001;
export const ERROR_INVALID_MESSAGE = 1002;
export const ERROR_TIMEOUT = 1003;
export const ERROR_CONNECTION_LOST = 1004;

export type RpcStatus = 'loading' | 'connected' | 'reconnecting' | 'disconnected';
export type RpcMessageKind = 'call' | 'success' | 'failure' | 'notification' | 'invalid';
export type RpcMethod = number;

export interface RpcError {
  code: number;
  message: string;
}

export interface RpcMessage {
  id?: number;
  method?: RpcMethod;
  payload?: unknown;
  error?: RpcError;
}

export interface BootInfo {
  app: string;
  headless: boolean;
  rpcUrl: string;
}

export interface RpcClientSnapshot {
  status: RpcStatus;
  boot: BootInfo | null;
  helloReady: boolean;
  latestError: RpcError | null;
}

interface WebSocketLike {
  readyState: number;
  onopen: (() => void) | null;
  onmessage: ((event: { data: string }) => void) | null;
  onclose: (() => void) | null;
  onerror: ((event?: unknown) => void) | null;
  send(data: string): void;
  close(): void;
}

export interface RpcClientOptions {
  serviceUrl?: string;
  reconnectIntervalMs?: number;
  reconnectFailureThreshold?: number;
  callTimeoutMs?: number;
  fetch?: typeof fetch;
  createWebSocket?: (url: string) => WebSocketLike;
}

interface PendingCall {
  id: number;
  method: RpcMethod;
  payload: unknown;
  sent: boolean;
  timer: ReturnType<typeof setTimeout>;
  resolve: (payload: unknown) => void;
  reject: (error: RpcError) => void;
}

type NotificationHandler = (payload: unknown) => void;
type StateHandler = (snapshot: RpcClientSnapshot) => void;

const WS_OPEN = 1;

export function createFrontendIdGenerator(): () => number {
  let id = 0;
  return () => {
    id += 1;
    return id;
  };
}

export function createRpcError(code: number, message: string): RpcError {
  return { code, message };
}

export function classifyRpcMessage(message: unknown): RpcMessageKind {
  if (!isRecord(message)) {
    return 'invalid';
  }

  const hasId = Object.prototype.hasOwnProperty.call(message, 'id');
  const hasMethod = Object.prototype.hasOwnProperty.call(message, 'method');
  const hasPayload = Object.prototype.hasOwnProperty.call(message, 'payload');
  const hasError = Object.prototype.hasOwnProperty.call(message, 'error');
  const validId = typeof message.id === 'number' && Number.isInteger(message.id) && message.id !== 0;
  const validMethod = typeof message.method === 'number' && Number.isInteger(message.method) && message.method > 0;
  const validError = isRpcError(message.error);

  if (hasId && !validId) {
    return 'invalid';
  }

  if (validId && validMethod && hasPayload && !hasError) {
    return 'call';
  }

  if (validId && !hasMethod && hasPayload && !hasError) {
    return 'success';
  }

  if (validId && !hasMethod && !hasPayload && validError) {
    return 'failure';
  }

  if (!hasId && validMethod && hasPayload && !hasError) {
    return 'notification';
  }

  return 'invalid';
}

export function resolveDefaultServiceUrl(): string {
  const configured = import.meta.env.VITE_SERVICE_URL as string | undefined;
  if (configured && configured.trim().length > 0) {
    return configured.replace(/\/$/, '');
  }
  return window.location.origin;
}

export class RpcClient {
  private readonly serviceUrl: string;
  private readonly reconnectIntervalMs: number;
  private readonly reconnectFailureThreshold: number;
  private readonly callTimeoutMs: number;
  private readonly fetchImpl: typeof fetch;
  private readonly createWebSocketImpl: (url: string) => WebSocketLike;
  private readonly nextId = createFrontendIdGenerator();
  private readonly pending = new Map<number, PendingCall>();
  private readonly notificationHandlers = new Map<RpcMethod, Set<NotificationHandler>>();
  private readonly stateHandlers = new Set<StateHandler>();

  private socket: WebSocketLike | null = null;
  private readyPromise: Promise<BootInfo> | null = null;
  private reconnecting = false;
  private stopped = false;
  private bootInfo: BootInfo | null = null;
  private helloReady = false;
  private latestError: RpcError | null = null;
  private currentStatus: RpcStatus = 'loading';

  constructor(options: RpcClientOptions = {}) {
    this.serviceUrl = (options.serviceUrl ?? resolveDefaultServiceUrl()).replace(/\/$/, '');
    this.reconnectIntervalMs = options.reconnectIntervalMs ?? 200;
    this.reconnectFailureThreshold = options.reconnectFailureThreshold ?? 50;
    this.callTimeoutMs = options.callTimeoutMs ?? 30000;
    this.fetchImpl = options.fetch ?? fetch.bind(globalThis);
    this.createWebSocketImpl =
      options.createWebSocket ??
      ((url: string) => {
        return new WebSocket(url) as unknown as WebSocketLike;
      });
  }

  get status(): RpcStatus {
    return this.currentStatus;
  }

  get boot(): BootInfo | null {
    return this.bootInfo;
  }

  getSnapshot(): RpcClientSnapshot {
    return {
      status: this.currentStatus,
      boot: this.bootInfo,
      helloReady: this.helloReady,
      latestError: this.latestError,
    };
  }

  onState(handler: StateHandler): () => void {
    this.stateHandlers.add(handler);
    handler(this.getSnapshot());
    return () => this.stateHandlers.delete(handler);
  }

  onNotification(method: RpcMethod, handler: NotificationHandler): () => void {
    let handlers = this.notificationHandlers.get(method);
    if (!handlers) {
      handlers = new Set();
      this.notificationHandlers.set(method, handlers);
    }
    handlers.add(handler);
    return () => handlers?.delete(handler);
  }

  start(): Promise<BootInfo> {
    if (this.readyPromise) {
      return this.readyPromise;
    }

    this.stopped = false;
    this.setStatus('loading');
    this.readyPromise = this.connectUntilReady();
    return this.readyPromise;
  }

  stop(): void {
    this.stopped = true;
    for (const pending of this.pending.values()) {
      clearTimeout(pending.timer);
    }
    this.pending.clear();
    if (this.socket?.readyState === WS_OPEN) {
      this.socket.close();
    }
    this.socket = null;
  }

  call<TPayload = unknown>(method: RpcMethod, payload: unknown = {}, options: { timeoutMs?: number } = {}): Promise<TPayload> {
    const id = this.nextId();
    const timeoutMs = options.timeoutMs ?? this.callTimeoutMs;

    return new Promise<TPayload>((resolve, reject) => {
      const timer = setTimeout(() => {
        this.pending.delete(id);
        const error = createRpcError(ERROR_TIMEOUT, `rpc timeout: ${methodName(method)}`);
        this.setLatestError(error);
        reject(error);
      }, timeoutMs);

      const pending: PendingCall = {
        id,
        method,
        payload,
        sent: false,
        timer,
        resolve: (value) => resolve(value as TPayload),
        reject,
      };
      this.pending.set(id, pending);
      this.sendPendingCall(pending);
    });
  }

  private async connectUntilReady(): Promise<BootInfo> {
    let failures = 0;

    while (!this.stopped) {
      try {
        const boot = await this.connectOnce();
        this.bootInfo = boot;
        this.helloReady = true;
        this.setStatus('connected');
        this.flushQueuedCalls();
        return boot;
      } catch (error) {
        failures += 1;
        this.setStatus(failures >= this.reconnectFailureThreshold ? 'disconnected' : 'loading');
        if (failures >= this.reconnectFailureThreshold) {
          this.failAllPending(createRpcError(ERROR_CONNECTION_LOST, 'rpc connection lost'));
          failures = 0;
        }
        await this.delay();
      }
    }

    throw createRpcError(ERROR_CONNECTION_LOST, 'rpc client stopped');
  }

  private async reconnectLoop(): Promise<void> {
    if (this.reconnecting || this.stopped) {
      return;
    }

    this.reconnecting = true;
    let failures = 0;
    this.setStatus('reconnecting');

    while (!this.stopped) {
      try {
        const boot = await this.connectOnce();
        this.bootInfo = boot;
        this.helloReady = true;
        this.setStatus('connected');
        this.flushQueuedCalls();
        this.reconnecting = false;
        return;
      } catch (error) {
        failures += 1;
        if (failures >= this.reconnectFailureThreshold) {
          this.failAllPending(createRpcError(ERROR_CONNECTION_LOST, 'rpc connection lost'));
          this.setStatus('disconnected');
          failures = 0;
        }
        await this.delay();
      }
    }

    this.reconnecting = false;
  }

  private async connectOnce(): Promise<BootInfo> {
    const boot = await this.fetchBoot();
    const socket = this.createWebSocketImpl(boot.rpcUrl);
    this.socket = socket;
    this.bindSocket(socket);
    await this.waitForOpen(socket);
    this.bindSocket(socket);
    await this.call(rpc.app.hello, {});
    return boot;
  }

  private async fetchBoot(): Promise<BootInfo> {
    const response = await this.fetchImpl(`${this.serviceUrl}/boot`, { cache: 'no-store' });
    if (!response.ok) {
      throw new Error(`boot failed: ${response.status}`);
    }
    const payload = await response.json();
    if (!isRecord(payload) || typeof payload.app !== 'string' || typeof payload.headless !== 'boolean' || typeof payload.rpcUrl !== 'string') {
      throw new Error('invalid boot payload');
    }
    return { app: payload.app, headless: payload.headless, rpcUrl: payload.rpcUrl };
  }

  private bindSocket(socket: WebSocketLike): void {
    socket.onmessage = (event) => this.handleMessage(event.data);
    socket.onclose = () => this.handleSocketClose(socket);
    socket.onerror = () => this.handleSocketClose(socket);
  }

  private waitForOpen(socket: WebSocketLike): Promise<void> {
    if (socket.readyState === WS_OPEN) {
      return Promise.resolve();
    }

    return new Promise((resolve, reject) => {
      socket.onopen = () => resolve();
      socket.onclose = () => reject(new Error('websocket closed before open'));
      socket.onerror = () => reject(new Error('websocket failed before open'));
    });
  }

  private handleMessage(data: string): void {
    let message: unknown;
    try {
      message = JSON.parse(data);
    } catch (error) {
      this.socket?.close();
      return;
    }

    const kind = classifyRpcMessage(message);
    if (!isRecord(message)) {
      this.socket?.close();
      return;
    }

    if (kind === 'success') {
      this.completeCall(message.id as number, message.payload);
      return;
    }

    if (kind === 'failure') {
      this.failCall(message.id as number, message.error as RpcError);
      return;
    }

    if (kind === 'notification') {
      this.dispatchNotification(message.method as RpcMethod, message.payload);
      return;
    }

    if (kind === 'call') {
      this.send({
        id: message.id as number,
        error: createRpcError(ERROR_METHOD_NOT_FOUND, `method not found: ${methodName(message.method as RpcMethod)}`),
      });
      return;
    }

    if (typeof message.id === 'number' && Number.isInteger(message.id) && message.id !== 0) {
      this.send({
        id: message.id,
        error: createRpcError(ERROR_INVALID_MESSAGE, 'invalid rpc message'),
      });
      return;
    }

    this.socket?.close();
  }

  private handleSocketClose(socket: WebSocketLike): void {
    if (this.socket !== socket || this.stopped) {
      return;
    }

    this.socket = null;
    this.helloReady = false;
    this.setStatus('reconnecting');
    void this.reconnectLoop();
  }

  private sendPendingCall(pending: PendingCall): void {
    if (!this.canSend() || pending.sent) {
      return;
    }

    this.send({ id: pending.id, method: pending.method, payload: pending.payload });
    pending.sent = true;
  }

  private flushQueuedCalls(): void {
    for (const pending of this.pending.values()) {
      this.sendPendingCall(pending);
    }
  }

  private send(message: RpcMessage): void {
    if (!this.canSend()) {
      return;
    }
    this.socket?.send(JSON.stringify(message));
  }

  private canSend(): boolean {
    return this.socket?.readyState === WS_OPEN;
  }

  private completeCall(id: number, payload: unknown): void {
    const pending = this.pending.get(id);
    if (!pending) {
      return;
    }
    clearTimeout(pending.timer);
    this.pending.delete(id);
    pending.resolve(payload);
  }

  private failCall(id: number, error: RpcError): void {
    const pending = this.pending.get(id);
    if (!pending) {
      return;
    }
    clearTimeout(pending.timer);
    this.pending.delete(id);
    this.setLatestError(error);
    pending.reject(error);
  }

  private failAllPending(error: RpcError): void {
    for (const pending of this.pending.values()) {
      clearTimeout(pending.timer);
      pending.reject(error);
    }
    this.pending.clear();
    this.setLatestError(error);
  }

  private dispatchNotification(method: RpcMethod, payload: unknown): void {
    const handlers = this.notificationHandlers.get(method);
    if (!handlers) {
      return;
    }
    for (const handler of handlers) {
      handler(payload);
    }
  }

  private setStatus(status: RpcStatus): void {
    if (this.currentStatus === status) {
      return;
    }
    this.currentStatus = status;
    this.emitState();
  }

  private setLatestError(error: RpcError): void {
    this.latestError = error;
    this.emitState();
  }

  private emitState(): void {
    const snapshot = this.getSnapshot();
    for (const handler of this.stateHandlers) {
      handler(snapshot);
    }
  }

  private delay(): Promise<void> {
    return new Promise((resolve) => {
      setTimeout(resolve, this.reconnectIntervalMs);
    });
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function isRpcError(value: unknown): value is RpcError {
  return isRecord(value) && typeof value.code === 'number' && Number.isInteger(value.code) && typeof value.message === 'string';
}
