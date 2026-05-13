# APP Host Demo Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 构建一个可运行的最小 APP Host Demo，验证 Go exe 启动 service、非 headless 创建 WebView2 native window、React/Vite 前端连接 `/boot` 和 `/ws`、整数 method RPC、headless 调试、单文件 exe 构建和 smoke test。

**Architecture:** 前端放在 `app/`，使用 TypeScript/React/Vite；前端 RPC 全部先收敛到 `app/src/rpc/rpc.ts`。后端放在 `service/`，使用 Go；后端 RPC、HTTP 路由、WebSocket、session task 和通知发送收敛到 `service/backend/rpc.go`，demo capability 在 `service/main.go` 注册。构建脚本把前端产物写入 `.build/embedweb/app/`，生成临时 embed package 和 `.build/service/` 构建副本，最终输出 `bin/app-host-demo.exe`。

**Tech Stack:** TypeScript、React、Vite、Vitest、npm、Go 1.26、`github.com/gorilla/websocket`、Windows Batch。

---

## File Structure

Create:

- `app/package.json`：前端依赖和 npm scripts。
- `app/index.html`：Vite HTML entry。
- `app/tsconfig.json`：TypeScript 配置。
- `app/vite.config.ts`：Vite/Vitest 配置，正式构建输出 `.build/embedweb/app/`。
- `app/src/main.tsx`：React entry。
- `app/src/App.tsx`：Demo UI。
- `app/src/styles.css`：Demo UI 样式。
- `app/src/rpc/methods.json`：method id 稳定分配源。
- `app/src/rpc/rpc.ts`：前端 RPC Lib。
- `app/src/rpc/methods_gen.ts`：生成的前端 method const 和名称表。
- `app/src/tests/rpc.test.ts`：RPC Lib 单元测试。
- `service/go.mod`：Go module。
- `service/main.go`：应用入口、demo capability 注册、`OnSessionReady` push 挂载。
- `service/backend/host.go`：CLI、端口选择、service 启动、普通模式创建 WebView2 native window。
- `service/backend/rpc.go`：后端 RPC Lib、HTTP handlers、WebSocket server、handler map、session task。
- `service/backend/methods_gen.go`：生成的后端 method 命名空间变量和名称表。
- `service/backend/backend_test.go`：后端 RPC/HTTP 测试。
- `script/setup.bat`：依赖安装和版本检查。
- `script/codegen-methods.bat`：RPC method 生成入口。
- `script/codegen-methods.mjs`：维护 `app/src/rpc/methods.json`，生成 Go/TS method 定义。
- `script/dev.bat`：headless Go service + Vite 开发入口。
- `script/dev.mjs`：dev 进程管理 helper。
- `script/build.bat`：前端构建、embed package、临时 service 副本、单文件 exe 构建。
- `script/test.bat`：Go 测试、前端类型检查和测试、正式构建、exe smoke test。
- `script/smoke-headless.mjs`：正式 exe headless smoke test helper。
- `bin/start-headless.bat`：从已有单文件 exe 启动 headless service。

Modify:

- `.gitignore`：确认 `.build/`、`bin/`、`node_modules/`、`dist/`、`coverage/` 被忽略。

No automatic commits in this plan. Commit only when the user explicitly asks.

---

### Task 1: Frontend Project Scaffold

**Files:**
- Create: `app/package.json`
- Create: `app/index.html`
- Create: `app/tsconfig.json`
- Create: `app/vite.config.ts`
- Create: `app/src/main.tsx`
- Create: `app/src/styles.css`

- [ ] **Step 1: Create package and config files**

`app/package.json` must define:

```json
{
  "name": "app-host-demo",
  "private": true,
  "version": "0.1.0",
  "type": "module",
  "scripts": {
    "dev": "vite --host 127.0.0.1 --port 5173 --strictPort",
    "build": "tsc --noEmit && vite build",
    "typecheck": "tsc --noEmit",
    "test": "vitest run"
  },
  "dependencies": {
    "@vitejs/plugin-react": "latest",
    "vite": "latest",
    "typescript": "latest",
    "vitest": "latest",
    "react": "latest",
    "react-dom": "latest",
    "@types/react": "latest",
    "@types/react-dom": "latest"
  },
  "devDependencies": {}
}
```

`app/vite.config.ts` must set `build.outDir` to `../.build/embedweb/app` and `emptyOutDir` to `true`.

- [ ] **Step 2: Add minimal React entry**

`app/src/main.tsx` imports `React`, `createRoot`, `App`, and `styles.css`, then renders `<App />` into `#root`.

- [ ] **Step 3: Verify scaffold compiles after dependencies are installed**

Run after `script/setup.bat` exists and dependencies are installed:

```bat
Set-Location app
npm run typecheck
```

Expected: TypeScript exits with code `0`.

---

### Task 2: Frontend RPC Lib, Test First

**Files:**
- Create: `app/src/tests/rpc.test.ts`
- Create: `app/src/rpc/rpc.ts`

- [ ] **Step 1: Write RED tests for RPC message behavior**

Add tests that define the desired API before implementation:

```ts
import { describe, expect, test, vi } from 'vitest';
import {
  ERROR_CONNECTION_LOST,
  ERROR_METHOD_NOT_FOUND,
  RpcClient,
  createFrontendIdGenerator,
  classifyRpcMessage,
  createRpcError,
  rpc,
} from '../rpc/rpc';

describe('createFrontendIdGenerator', () => {
  test('generates positive ids from one', () => {
    const nextId = createFrontendIdGenerator();
    expect(nextId()).toBe(1);
    expect(nextId()).toBe(2);
  });
});

describe('classifyRpcMessage', () => {
  test('classifies call, success, failure, and notification messages', () => {
    expect(classifyRpcMessage({ id: 1, method: rpc.demo.ping, payload: {} })).toBe('call');
    expect(classifyRpcMessage({ id: 1, payload: { message: 'pong' } })).toBe('success');
    expect(classifyRpcMessage({ id: 1, error: createRpcError(ERROR_METHOD_NOT_FOUND, 'missing') })).toBe('failure');
    expect(classifyRpcMessage({ method: rpc.demo.tick, payload: { count: 1 } })).toBe('notification');
  });
});
```

- [ ] **Step 2: Run tests and verify RED**

Run:

```bat
Set-Location app
npm test -- src/tests/rpc.test.ts
```

Expected: fails because `rpc.ts` exports do not exist.

- [ ] **Step 3: Implement pure RPC helpers**

`rpc.ts` must export:

```ts
export const ERROR_METHOD_NOT_FOUND = 1001;
export const ERROR_INVALID_MESSAGE = 1002;
export const ERROR_TIMEOUT = 1003;
export const ERROR_CONNECTION_LOST = 1004;
export const rpc = {
  app: { hello: 1000001 },
  demo: { ping: 2000001, tick: 2000002 },
} as const;
export type RpcMessageKind = 'call' | 'success' | 'failure' | 'notification' | 'invalid';
export function createFrontendIdGenerator(): () => number;
export function createRpcError(code: number, message: string): RpcError;
export function classifyRpcMessage(message: unknown): RpcMessageKind;
```

- [ ] **Step 4: Add RED tests for matching completions and notifications**

Add a test with a fake WebSocket factory:

```ts
test('matches call completion by id and dispatches notifications', async () => {
  const socket = new FakeWebSocket('ws://127.0.0.1/ws');
  const client = new RpcClient({
    serviceUrl: 'http://127.0.0.1:18080',
    createWebSocket: () => socket,
    fetch: async () =>
      new Response(JSON.stringify({ app: 'app-host-demo', headless: true, rpcUrl: 'ws://127.0.0.1/ws' })),
    reconnectIntervalMs: 1,
    reconnectFailureThreshold: 3,
  });

  const ticks: unknown[] = [];
  client.onNotification(rpc.demo.tick, (payload) => ticks.push(payload));
  const ready = client.start();
  socket.open();
  socket.receive({ id: 1, payload: { app: 'app-host-demo', headless: true } });
  await ready;

  const ping = client.call(rpc.demo.ping, {});
  socket.receive({ id: 2, payload: { message: 'pong' } });
  socket.receive({ method: rpc.demo.tick, payload: { count: 1, message: 'tick' } });

  await expect(ping).resolves.toEqual({ message: 'pong' });
  expect(ticks).toEqual([{ count: 1, message: 'tick' }]);
});
```

- [ ] **Step 5: Implement `RpcClient` minimal behavior**

`RpcClient` must:

- poll `/boot` every `reconnectIntervalMs` until it receives `{ app, headless, rpcUrl }`;
- open WebSocket using `rpcUrl`;
- send `1000001 app.hello` as the first call;
- expose status `loading`, `connected`, `reconnecting`, `disconnected`;
- match `id + payload` and `id + error` completions;
- dispatch `method + payload` notifications;
- queue calls made while disconnected;
- fail pending calls with code `1004` after `reconnectFailureThreshold` failed reconnect attempts;
- apply default call timeout `30000` ms unless overridden.

- [ ] **Step 6: Run frontend RPC tests**

Run:

```bat
Set-Location app
npm test -- src/tests/rpc.test.ts
```

Expected: all tests in `src/tests/rpc.test.ts` pass.

---

### Task 3: React Demo UI

**Files:**
- Create: `app/src/App.tsx`
- Modify: `app/src/styles.css`

- [ ] **Step 1: Implement UI against `RpcClient`**

`App.tsx` must:

- create one `RpcClient`;
- show full-screen loading until `app.hello` completes;
- show connection status;
- show `/boot` app/headless;
- show hello status;
- provide a `demo.ping` button;
- show latest ping payload;
- show recent 5 `demo.tick` payloads;
- show latest error.

- [ ] **Step 2: Verify frontend typecheck**

Run:

```bat
Set-Location app
npm run typecheck
```

Expected: exits with code `0`.

---

### Task 4: Go RPC/HTTP Tests, Test First

**Files:**
- Create: `service/go.mod`
- Create: `service/backend/backend_test.go`
- Create: `service/backend/rpc.go`

- [ ] **Step 1: Create Go module**

`service/go.mod` must use module path `apphostdemo/service` and require `github.com/gorilla/websocket`.

- [ ] **Step 2: Write RED tests for message validation and error shape**

Add Go tests:

```go
func TestClassifyMessage(t *testing.T) {
  cases := []struct {
    name string
    raw  string
    want MessageKind
  }{
    {"call", `{"id":1,"method":2000001,"payload":{}}`, MessageCall},
    {"success", `{"id":1,"payload":{"message":"pong"}}`, MessageSuccess},
    {"failure", `{"id":1,"error":{"code":1001,"message":"missing"}}`, MessageFailure},
    {"notification", `{"method":2000002,"payload":{"count":1}}`, MessageNotification},
  }

  for _, tc := range cases {
    t.Run(tc.name, func(t *testing.T) {
      msg, err := DecodeMessage([]byte(tc.raw))
      if err != nil {
        t.Fatalf("DecodeMessage returned error: %v", err)
      }
      if got := ClassifyMessage(msg); got != tc.want {
        t.Fatalf("ClassifyMessage() = %v, want %v", got, tc.want)
      }
    })
  }
}
```

- [ ] **Step 3: Write RED tests for HTTP handlers**

Add tests that use `httptest.NewServer(NewServer(ServerOptions{AppName: "app-host-demo", Headless: true}))` and assert:

- `GET /healthz` returns `{"ok":true,"app":"app-host-demo"}`;
- `GET /boot` returns `app`, `headless:true`, and `rpcUrl` ending with `/ws`;
- unknown method sent over `/ws` returns error code `1001`;
- `app.hello` returns `{ "app": "app-host-demo", "headless": true }`;
- `demo.ping` returns `{ "message": "pong" }`.

- [ ] **Step 4: Run Go tests and verify RED**

Run:

```bat
Set-Location service
go test ./...
```

Expected: fails because `rpc.go` behavior is not implemented.

---

### Task 5: Go Service Implementation

**Files:**
- Create: `service/backend/rpc.go`
- Create: `service/main.go`

- [ ] **Step 1: Implement `rpc.go`**

`rpc.go` must include:

- `RPCError`, `Message`, `MessageKind`;
- built-in error constants `1001` through `1004`;
- `DecodeMessage`, `ClassifyMessage`, `ErrorMessage`, `SuccessMessage`;
- `ServerOptions` with `AppName`, `Headless`, `StaticFS`;
- built-in `app.hello` handshake and `OnSessionReady`;
- `NewServer(options ServerOptions) *Server`;
- `RegisterHandler(method Method, handler HandlerFunc)` for generated method definitions;
- `StartSessionTask` and `NotifyFunc` for connection-scoped push work;
- `/healthz`, `/boot`, `/ws` handlers;
- WebSocket upgrade using `gorilla/websocket`;
- unknown method handling through the handler map.

- [ ] **Step 2: Implement `main.go`**

`main.go` must:

- parse `--headless` and `--port <int>`;
- require `--port` in headless mode;
- choose a dynamic local port in normal mode when `--port` is absent;
- register `demo.ping` in the business layer and attach `demo.tick` through `OnSessionReady`;
- start HTTP service on `127.0.0.1:<port>`;
- in normal mode create a WebView2 native window and load `http://127.0.0.1:<port>/`;
- when the WebView2 window closes, shutdown the HTTP service and exit the process;
- keep running until process termination;
- log listening URL, headless flag, and RPC URL.

- [ ] **Step 3: Run Go tests**

Run:

```bat
Set-Location service
go test ./...
```

Expected: all Go tests pass.

---

### Task 6: Scripts

**Files:**
- Create: `script/setup.bat`
- Create: `script/dev.bat`
- Create: `script/dev.mjs`
- Create: `script/build.bat`
- Create: `script/test.bat`
- Create: `script/smoke-headless.mjs`
- Create: `bin/start-headless.bat`

- [ ] **Step 1: Implement `setup.bat`**

The script must:

- print Go, Node, and npm versions;
- run `npm install` inside `app`;
- run `go mod download` inside `service`.

- [ ] **Step 2: Implement `dev.bat`**

The script must:

- check `app/node_modules`;
- start `go run . --headless --port 18080` in `service`;
- start `npm run dev` in `app` with `VITE_SERVICE_URL=http://127.0.0.1:18080`;
- open `http://127.0.0.1:5173/`;
- terminate both child processes when the script exits.

- [ ] **Step 3: Implement `build.bat`**

The script must:

- check `app/node_modules`;
- clear only `.build/embedweb`, `.build/service`, and `bin/app-host-demo.exe`;
- run `npm run build` inside `app`;
- generate `.build/embedweb/go.mod`;
- generate `.build/embedweb/embedweb.go` with `//go:embed all:app`;
- copy `service/` to `.build/service/`;
- generate `.build/service/backend/embedweb_release.go` that sets the package-level embedded FS;
- add `require apphostdemo/embedweb v0.0.0` and `replace apphostdemo/embedweb => ../embedweb` to `.build/service/go.mod`;
- run `go build -o ../../bin/app-host-demo.exe .` inside `.build/service`.

- [ ] **Step 4: Implement `test.bat`**

The script must:

- check `app/node_modules`;
- run `go test ./...` inside `service`;
- run `npm run typecheck` and `npm test` inside `app`;
- run `script/build.bat`;
- start `bin/app-host-demo.exe --headless --port 18081`;
- request `http://127.0.0.1:18081/boot`;
- assert `app == "app-host-demo"`, `headless == true`, and `rpcUrl` ends with `/ws`;
- stop the smoke test process.

---

### Task 7: End-to-End Verification

**Files:**
- No new files.

- [ ] **Step 1: Install dependencies**

Run:

```bat
script\setup.bat
```

Expected: `npm install` and `go mod download` exit with code `0`.

- [ ] **Step 2: Run full verification**

Run:

```bat
script\test.bat
```

Expected:

- Go tests pass.
- TypeScript typecheck passes.
- Vitest tests pass.
- `bin/app-host-demo.exe` exists.
- smoke test `/boot` passes.

- [ ] **Step 3: Optional manual dev run**

Run:

```bat
script\dev.bat
```

Expected:

- browser opens `http://127.0.0.1:5173/`;
- UI leaves loading after `app.hello`;
- status is `connected`;
- `demo.ping` returns `pong`;
- `demo.tick` updates every 2 seconds.

---

## Self-Review

- Spec coverage: covers docs, frontend scaffold, TypeScript RPC, React UI, Go service, scripts, build, and smoke test.
- Placeholder scan: no `TBD`, no `TODO`, no unspecified implementation bucket.
- Type consistency: RPC method ids are `1000001 app.hello`, `2000001 demo.ping`, `2000002 demo.tick`; message fields are `id`, integer `method`, `payload`, `error`; errors use integer `code`.


