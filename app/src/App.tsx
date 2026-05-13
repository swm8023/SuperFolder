import { useEffect, useMemo, useState } from 'react';
import { RpcClient, RpcClientSnapshot, RpcError, rpc } from './rpc/rpc';

interface TickPayload {
  count: number;
  message: string;
}

type PingState = 'idle' | 'pending' | 'done' | 'failed';

function asTickPayload(payload: unknown): TickPayload | null {
  if (typeof payload !== 'object' || payload === null) {
    return null;
  }
  const value = payload as Record<string, unknown>;
  if (typeof value.count !== 'number' || typeof value.message !== 'string') {
    return null;
  }
  return { count: value.count, message: value.message };
}

function asRpcError(error: unknown): RpcError {
  if (typeof error === 'object' && error !== null) {
    const value = error as Record<string, unknown>;
    if (typeof value.code === 'number' && typeof value.message === 'string') {
      return { code: value.code, message: value.message };
    }
  }
  return { code: 10000, message: error instanceof Error ? error.message : String(error) };
}

export default function App() {
  const client = useMemo(() => new RpcClient(), []);
  const [snapshot, setSnapshot] = useState<RpcClientSnapshot>(() => client.getSnapshot());
  const [hasConnectedOnce, setHasConnectedOnce] = useState(false);
  const [ticks, setTicks] = useState<TickPayload[]>([]);
  const [pingState, setPingState] = useState<PingState>('idle');
  const [pingPayload, setPingPayload] = useState<unknown>(null);
  const [latestError, setLatestError] = useState<RpcError | null>(null);

  useEffect(() => {
    const stopState = client.onState((next) => {
      setSnapshot(next);
      if (next.helloReady) {
        setHasConnectedOnce(true);
      }
      if (next.latestError) {
        setLatestError(next.latestError);
      }
    });
    const stopTick = client.onNotification(rpc.demo.tick, (payload) => {
      const tick = asTickPayload(payload);
      if (!tick) {
        return;
      }
      setTicks((current) => [tick, ...current].slice(0, 5));
    });

    client.start().catch((error) => {
      setLatestError(asRpcError(error));
    });

    return () => {
      stopTick();
      stopState();
      client.stop();
    };
  }, [client]);

  async function handlePing() {
    setPingState('pending');
    setLatestError(null);
    try {
      const payload = await client.call(rpc.demo.ping, {});
      setPingPayload(payload);
      setPingState('done');
    } catch (error) {
      setLatestError(asRpcError(error));
      setPingState('failed');
    }
  }

  if (!hasConnectedOnce) {
    return (
      <main className="loading-screen">
        <div className="loading-mark" aria-hidden="true" />
        <div className="loading-title">APP Host Demo</div>
        <div className="loading-subtitle">Connecting</div>
      </main>
    );
  }

  return (
    <main className="shell">
      <header className="topbar">
        <div>
          <h1>APP Host Demo</h1>
          <p>Go Service + React RPC</p>
        </div>
        <span className={`status status-${snapshot.status}`}>{snapshot.status}</span>
      </header>

      <section className="grid">
        <div className="panel">
          <h2>Boot</h2>
          <dl className="kv">
            <div>
              <dt>app</dt>
              <dd>{snapshot.boot?.app ?? '-'}</dd>
            </div>
            <div>
              <dt>headless</dt>
              <dd>{String(snapshot.boot?.headless ?? '-')}</dd>
            </div>
            <div>
              <dt>hello</dt>
              <dd>{snapshot.helloReady ? 'ready' : 'waiting'}</dd>
            </div>
          </dl>
        </div>

        <div className="panel">
          <h2>Ping</h2>
          <div className="action-row">
            <button type="button" onClick={handlePing} disabled={snapshot.status !== 'connected' || pingState === 'pending'}>
              {pingState === 'pending' ? 'Pinging' : 'Ping'}
            </button>
            <span className="muted">{pingState}</span>
          </div>
          <pre>{pingPayload ? JSON.stringify(pingPayload, null, 2) : '-'}</pre>
        </div>

        <div className="panel wide">
          <h2>Tick</h2>
          <ol className="ticks">
            {ticks.length === 0 ? (
              <li className="empty">-</li>
            ) : (
              ticks.map((tick) => (
                <li key={`${tick.count}-${tick.message}`}>
                  <span>#{tick.count}</span>
                  <strong>{tick.message}</strong>
                </li>
              ))
            )}
          </ol>
        </div>

        <div className="panel wide">
          <h2>Error</h2>
          <pre>{latestError ? JSON.stringify(latestError, null, 2) : '-'}</pre>
        </div>
      </section>
    </main>
  );
}
