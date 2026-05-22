import { describe, expect, test } from 'vitest';
import {
  ERROR_CONNECTION_LOST,
  ERROR_METHOD_NOT_FOUND,
  RpcClient,
  classifyRpcMessage,
  createFrontendIdGenerator,
  createRpcError,
  rpc,
} from '../rpc/rpc';

class FakeWebSocket {
  static CONNECTING = 0;
  static OPEN = 1;
  static CLOSING = 2;
  static CLOSED = 3;

  readyState = FakeWebSocket.CONNECTING;
  sent: unknown[] = [];
  onopen: (() => void) | null = null;
  onmessage: ((event: { data: string }) => void) | null = null;
  onclose: (() => void) | null = null;
  onerror: (() => void) | null = null;

  constructor(readonly url: string) {}

  send(data: string) {
    this.sent.push(JSON.parse(data));
  }

  close() {
    this.readyState = FakeWebSocket.CLOSED;
    this.onclose?.();
  }

  open() {
    this.readyState = FakeWebSocket.OPEN;
    this.onopen?.();
  }

  receive(message: unknown) {
    this.onmessage?.({ data: JSON.stringify(message) });
  }
}

async function flushMicrotasks() {
  await Promise.resolve();
  await Promise.resolve();
}

async function waitFor(condition: () => boolean) {
  for (let attempt = 0; attempt < 25; attempt += 1) {
    if (condition()) {
      return;
    }
    await new Promise((resolve) => setTimeout(resolve, 0));
  }
  throw new Error('condition was not met');
}

describe('createFrontendIdGenerator', () => {
  test('generates positive ids from one', () => {
    const nextId = createFrontendIdGenerator();

    expect(nextId()).toBe(1);
    expect(nextId()).toBe(2);
  });
});

describe('classifyRpcMessage', () => {
  test('classifies call, success, failure, and notification messages', () => {
    expect(classifyRpcMessage({ id: 1, method: rpc.folder.session.get, payload: {} })).toBe('call');
    expect(classifyRpcMessage({ id: 1, payload: { message: 'pong' } })).toBe('success');
    expect(classifyRpcMessage({ id: 1, error: createRpcError(ERROR_METHOD_NOT_FOUND, 'missing') })).toBe('failure');
    expect(classifyRpcMessage({ method: rpc.folder.children.updated, payload: { path: 'C:\\tmp' } })).toBe('notification');
  });

  test('rejects zero ids and messages without required payload', () => {
    expect(classifyRpcMessage({ id: 0, method: rpc.folder.session.get, payload: {} })).toBe('invalid');
    expect(classifyRpcMessage({ id: 1, method: rpc.folder.session.get })).toBe('invalid');
    expect(classifyRpcMessage({ method: rpc.folder.children.updated })).toBe('invalid');
    expect(classifyRpcMessage({ id: 1, method: 'folder.session.get', payload: {} })).toBe('invalid');
  });
});

describe('RpcClient', () => {
  test('runs app.hello before ready, matches completions, and dispatches notifications', async () => {
    let socket: FakeWebSocket | undefined;
    let fetchCalls = 0;
    const client = new RpcClient({
      serviceUrl: 'http://127.0.0.1:5173',
      createWebSocket: (url) => {
        socket = new FakeWebSocket(url);
        return socket;
      },
      fetch: async () => {
        fetchCalls += 1;
        throw new Error('boot should not be fetched');
      },
      reconnectIntervalMs: 1,
      reconnectFailureThreshold: 3,
    });

    const ticks: unknown[] = [];
    client.onNotification(rpc.folder.children.updated, (payload) => ticks.push(payload));

    const ready = client.start();
    await waitFor(() => socket !== undefined);
    expect(socket?.url).toBe('ws://127.0.0.1:5173/ws');
    expect(fetchCalls).toBe(0);
    socket?.open();
    await waitFor(() => (socket?.sent.length ?? 0) > 0);
    expect(socket?.sent[0]).toEqual({ id: 1, method: rpc.app.hello, payload: {} });
    socket?.receive({ id: 1, payload: { app: 'superfolder', headless: true } });

    await expect(ready).resolves.toEqual({ app: 'superfolder', headless: true, rpcUrl: 'ws://127.0.0.1:5173/ws' });
    expect(client.status).toBe('connected');

    const session = client.call(rpc.folder.session.get, {});
    expect(socket?.sent[1]).toEqual({ id: 2, method: rpc.folder.session.get, payload: {} });
    socket?.receive({ id: 2, payload: { session: {} } });
    socket?.receive({ method: rpc.folder.children.updated, payload: { path: 'C:\\tmp' } });

    await expect(session).resolves.toEqual({ session: {} });
    expect(ticks).toEqual([{ path: 'C:\\tmp' }]);
    client.stop();
  });

  test('fails pending calls with connection_lost when reconnect threshold is reached', async () => {
    let socket: FakeWebSocket | undefined;
    let socketAttempts = 0;
    const client = new RpcClient({
      serviceUrl: 'http://127.0.0.1:5173',
      createWebSocket: (url) => {
        socketAttempts += 1;
        if (socketAttempts > 1) {
          throw new Error('offline');
        }
        socket = new FakeWebSocket(url);
        return socket;
      },
      fetch: async () => {
        throw new Error('boot should not be fetched');
      },
      reconnectIntervalMs: 1,
      reconnectFailureThreshold: 1,
      callTimeoutMs: 1000,
    });

    const ready = client.start();
    await waitFor(() => socket !== undefined);
    socket?.open();
    await waitFor(() => (socket?.sent.length ?? 0) > 0);
    socket?.receive({ id: 1, payload: { app: 'superfolder', headless: true } });
    await ready;

    const pending = client.call(rpc.folder.session.get, {});
    socket?.close();

    await expect(pending).rejects.toMatchObject({ code: ERROR_CONNECTION_LOST });
    expect(client.status).toBe('disconnected');
    client.stop();
  });
});
