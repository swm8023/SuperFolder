import { useEffect, useMemo, useState } from 'react';
import { RpcClient, RpcClientSnapshot, RpcError } from './rpc/rpc';

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

    client.start().catch((error) => {
      setLatestError(asRpcError(error));
    });

    return () => {
      stopState();
      client.stop();
    };
  }, [client]);

  if (!hasConnectedOnce) {
    return (
      <main className="loading-screen">
        <div className="loading-mark" aria-hidden="true" />
        <div className="loading-title">SuperFolder</div>
        <div className="loading-subtitle">Connecting</div>
      </main>
    );
  }

  return (
    <main className="shell">
      <header className="topbar">
        <div>
          <h1>SuperFolder</h1>
          <p>File workspace</p>
        </div>
        <span className={`status status-${snapshot.status}`}>{snapshot.status}</span>
      </header>

      <section className="grid">
        <div className="panel wide">
          <h2>Session</h2>
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
              <dt>rpc</dt>
              <dd>{snapshot.helloReady ? 'ready' : 'waiting'}</dd>
            </div>
          </dl>
        </div>

        <div className="panel wide">
          <h2>Workspace</h2>
          <p className="muted">SuperFolder workspace UI is being enabled.</p>
        </div>

        <div className="panel wide">
          <h2>Error</h2>
          <pre>{latestError ? JSON.stringify(latestError, null, 2) : '-'}</pre>
        </div>
      </section>
    </main>
  );
}
