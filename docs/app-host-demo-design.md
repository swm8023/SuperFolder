# APP Host Demo 设计

## 定位

APP Host Demo 是第一个可运行的最小框架验证项目。它用于验证通用 App Host、RPC、Transport、前端连接、后端方法处理和后端主动推送能力。

Demo 的目标是用最少代码跑通这条链路：

```text
React/Vite 前端 -> TypeScript RPC Lib -> WebSocket/HTTP -> Go Service -> Go RPC Lib
```

## 范围

Demo 聚焦通用框架能力：

- Go service 作为本地可执行入口。
- Go service 能选择可用本地端口。
- Go service 能在本地拉起 Web UI。
- 非 headless 模式由 Go exe 创建 WebView2 native window 并加载本地 Web URL。
- 前端启动阶段显示全屏 loading，并主动等待 service/RPC ready。
- React/Vite 作为前端 UI。
- WebSocket 作为第一阶段 RPC transport。
- HTTP endpoint 用于健康检查和开发辅助。
- Boot endpoint 用于向前端暴露运行期端口和 RPC 地址。
- 最小 JSON message 用于消息交换。
- 双向 RPC 统一使用同一种 message 模型。
- 前后端都可以发起带整数 `id` 的调用。
- 通知消息使用整数 `method` 和 `payload`。
- `.build/` 存放构建产物。
- `bin/` 存放二进制产物。
- `script/` 存放开发、构建和验证脚本。

## 技术栈

- 前端：TypeScript、React、Vite。
- Node.js：v25.9.0。
- 前端包管理器：npm 11.12.1。
- 前端测试：Vitest。
- 后端：Go 1.26。
- Go WebSocket：`github.com/gorilla/websocket`。
- 通信：开发阶段使用 localhost HTTP + WebSocket。
- RPC 格式：手写最小 JSON message，后续沉淀为正式 schema/codegen。
- UI surface：非 headless 模式使用 WebView2 native window；开发阶段 headless service 可配合普通浏览器访问 Vite dev server。

## 架构分层

### Go Service

Go Service 是 Demo 的后端入口和通用框架雏形。它负责：

- 启动 HTTP server。
- 选择并绑定本地可用端口。
- 暴露 health endpoint。
- 暴露 boot endpoint。
- 在普通模式服务内嵌 Web UI。
- 在非 headless 模式创建 WebView2 native window 并加载 Web UI 地址。
- 接收 WebSocket 连接。
- 管理 RPC connection。
- 处理 `app.hello` handshake。
- 注册 demo capability。
- 处理 `demo.ping` 调用。
- 发送 `demo.tick` 通知。
- 为后续模块提供 capability 注册接口。

### TypeScript RPC Lib

TypeScript RPC Lib 是前端访问后端的统一入口。它负责：

- 建立 WebSocket 连接。
- 发送 RPC message。
- 生成前端发起的整数 `id`。
- 按 `id` 匹配完成消息。
- 接收后端发起的调用或通知。
- 处理 WebSocket 断开后的自动重连。
- 暴露连接状态。
- 提供取消和超时能力，调用默认超时 30 秒，可由调用方覆盖。

React 组件通过 RPC Lib 表达意图，由 RPC Lib 处理底层连接细节。

### React Demo App

React Demo App 展示框架状态：

- 启动阶段全屏 loading。
- 顶部连接状态：`loading`、`connected`、`reconnecting`、`disconnected`。
- `/boot` 返回的 `app` 和 `headless`。
- `app.hello` 是否成功。
- `demo.ping` 调用按钮和完成 payload。
- 最近 5 条 `demo.tick` 通知。
- 最近错误信息。

运行期 UI 状态保存在前端内存中。

## RPC Message 模型

RPC 统一使用 message 模型。连接上的每条数据都是一个 message。一个 message 可以是发起调用、完成调用、错误完成或通知。
message 不使用 `type` 字段，消息语义通过字段组合判断。

### Method ID

`method` 是全局唯一整数。method id 一经发布不得复用；废弃能力只能标记 deprecated，不能回收 id。

```text
1_000_000 - 1_999_999   App Host / RPC 内建能力
2_000_000 - 9_999_999   当前应用业务能力
10_000_000+             未来插件/扩展/动态分配能力
```

第一阶段定义：

```text
1000001  app.hello
2000001  demo.ping
2000002  demo.tick
```

Go 和 TypeScript 侧都必须使用生成的 method 引用，不在业务代码里手写数字 method。新增 method 在 `app/src/rpc/methods.json` 中声明；值为 `0` 时由 codegen 在构建期分配稳定 id 并回写。业务调用和 handler 注册都使用生成结果。构建和测试入口在执行前运行 `script/codegen-methods.bat`，生成 Go/TypeScript 两端 method 定义与名称表。

TypeScript 侧生成嵌套 const 对象，调用方使用 `rpc.app.hello`、`rpc.demo.ping`、`rpc.demo.tick` 这样的路径引用。Go 侧生成 `App.Hello`、`Demo.Ping`、`Demo.Tick` 这样的命名空间变量；在 `backend` 包外使用时路径是 `backend.App.Hello`。

method id 的稳定源是：

```text
app/src/rpc/methods.json
```

`app/src/rpc/methods.json` 记录 method name、已分配 method id 和各区间的 next 值。method name 固定为 `namespace.action` 两段小写格式。构建期读取该 JSON：

1. 已存在 method 保持原 id。
2. id 为 `0` 的新 method 按区间分配 next id。
3. 更新 `app/src/rpc/methods.json` 的 method id 和 next 值。
4. 生成 `service/backend/methods_gen.go`。
5. 生成 `app/src/rpc/methods_gen.ts`。
6. 发现非法 method name、重复 id、区间耗尽时构建失败。

生成文件提交到仓库，保证 IDE、typecheck 和测试在未运行完整 build 时仍可工作。调试日志和错误消息可以通过 `MethodName(method)` / `methodName(method)` 把数字转成人类可读名称。

### ID 规则

`id` 是单个整数。

```ts
type RpcID = number;
```

每个连接维护两个方向的递增序列：

- frontend 发起的调用使用正整数，从 `1` 开始递增。
- backend 发起的调用使用负整数，从 `-1` 开始递减。
- `0` 是非法 `id`，用于捕捉未初始化或错误生成的调用 ID。

这样 `id` 仍然是单个 int，同时可以通过正负号判断最初发起方，并且同一连接内不会冲突。

示例：

```text
1    frontend 发起的第 1 个调用
2    frontend 发起的第 2 个调用
-1   backend 发起的第 1 个调用
-2   backend 发起的第 2 个调用
```

唯一性边界：

- `id` 只要求在单条 RPC connection 生命周期内唯一。
- reconnect 后可以重新从 `1` 和 `-1` 开始。
- 如果未来需要跨连接追踪，另加 `connectionId` 或 `traceId`，不放进 `id`。

### 调用消息

带 `id` 和 `method` 的 message 表示一次调用。调用必须带 `payload`，空 payload 使用 `{}`。

```json
{
  "id": 1,
  "method": 2000001,
  "payload": {}
}
```

### 完成消息

带 `id` 和 `payload` 的 message 表示对应调用完成。
成功完成必须带 `payload`，空 payload 使用 `{}`。

```json
{
  "id": 1,
  "payload": {
    "message": "pong"
  }
}
```

### 错误完成消息

带 `id` 和 `error` 的 message 表示对应调用失败。错误对象保留独立字段，便于调用方区分成功 payload 和失败信息。
失败完成只带 `error`，不带 `payload`。成功完成和失败完成形状互斥。

```json
{
  "id": 1,
  "error": {
    "code": 1001,
    "message": "method not found: 2999999"
  }
}
```

错误码使用整数。RPC/App Host 提供内建错误码，业务 capability 可以定义自己的错误码区间，从而统一 error 结构。

错误码区间：

```text
1-9999   RPC/App Host 内建错误码
10000+   应用/业务 capability 错误码
```

第一阶段内建错误码：

```text
1001 method_not_found
1002 invalid_message
1003 timeout
1004 connection_lost
```

### 非法消息

收到非法 message 时：

- 如果原 message 带合法整数 `id`，返回同 `id` 的错误完成，错误码为 `1002`。
- 如果原 message 没有合法整数 `id`，关闭 WebSocket 连接。

收到未知 method 时，接收方返回同 `id` 的错误完成，错误码为 `1001`。前端收到后端发起的负数 `id` 调用时也遵循同一规则。

RPC 调用默认超时 30 秒。调用方可以为单次调用覆盖超时时间。超时从调用发起时开始计时，包含断线和重连等待时间。超时后 pending 调用失败，并返回内建错误码 `1003`。

### 通知消息

只有 `method` 和 `payload` 的 message 表示通知。通知消息是单向消息。通知也必须带 `payload`，空 payload 使用 `{}`。

```json
{
  "method": 2000002,
  "payload": {
    "count": 1,
    "message": "tick"
  }
}
```

### 后端发起调用

后端发起调用时使用负整数 `id`。

```json
{
  "id": -1,
  "method": 10000001,
  "payload": {
    "detail": true
  }
}
```

前端完成该调用时返回相同 `id`。

```json
{
  "id": -1,
  "payload": {
    "visible": true
  }
}
```

第一阶段 Demo 实现前端正整数调用和后端通知。协议设计同时覆盖后端负整数调用的模型。

## Demo 能力

### `GET /healthz`

返回 service ready 状态。开发脚本、前端和未来本地入口都可以用它判断后端是否可连接。

响应：

```json
{
  "ok": true,
  "app": "app-host-demo"
}
```

### `GET /boot`

返回前端启动所需的运行期信息，包括 app 标识、headless 状态和 RPC WebSocket URL。

普通模式下，前端在 WebView2 native window 中与 Go service 同源加载，再读取 `/boot` 获取 RPC 地址。headless 调试下，Vite 与 Go service 分属两个端口，前端通过配置的 service 地址读取 `/boot`。

示例：

```json
{
  "app": "app-host-demo",
  "headless": false,
  "rpcUrl": "ws://127.0.0.1:53123/ws"
}
```

### `GET /ws`

与 HTTP service 使用同一个端口，通过 `/ws` 路径升级为 WebSocket，承载 RPC message。

### `app.hello`

WebSocket 建立后，前端先发起 `app.hello` handshake。`app.hello` 是 backend 内建 RPC 调用，占用前端正整数 `id` 序列。`app.hello` 成功完成后，RPC Lib 才进入 ready 状态。backend 在 `app.hello` 成功时触发 `OnSessionReady`，业务层通过该 hook 挂载 session 级 push 任务。

调用：

```json
{
  "id": 1,
  "method": 1000001,
  "payload": {}
}
```

完成：

```json
{
  "id": 1,
  "payload": {
    "app": "app-host-demo",
    "headless": false
  }
}
```

### Web UI 静态资源

普通模式服务内嵌 Web UI。HTTP 路由优先级为：

1. `/healthz`
2. `/boot`
3. `/ws`
4. 内嵌静态资源
5. SPA fallback 到 `index.html`

headless 模式只提供 `/healthz`、`/boot`、`/ws`。

### `demo.ping`

前端发起正整数 `id` 调用，调用 payload 为空对象，后端返回 `{ "message": "pong" }`。

调用：

```json
{
  "id": 1,
  "method": 2000001,
  "payload": {}
}
```

完成：

```json
{
  "id": 1,
  "payload": {
    "message": "pong"
  }
}
```

### `demo.tick`

业务层在 `OnSessionReady` 中启动 `demo.tick` session task，每 2 秒发送一次通知。payload 为 `{ "count": number, "message": "tick" }`。前端展示最近若干条通知。

## 目录设计

Demo 使用简化目录，先把同类能力收敛到一级模块中。

```text
SuperFolder/
  app/
    src/
      rpc/
        methods.json
        methods_gen.ts
        rpc.ts
      App.tsx
      main.tsx
    index.html
    package.json
    vite.config.ts
    tsconfig.json
  service/
    backend/
      host.go
      methods_gen.go
      backend_test.go
      rpc.go
    main.go
    go.mod
  script/
    codegen-methods.bat
    codegen-methods.mjs
    setup.bat
    dev.bat
    dev.mjs
    build.bat
    smoke-headless.mjs
    test.bat
  .build/
  bin/
  docs/
    app-host-demo-design.md
```

### `app`

Demo 前端应用。`app/src/rpc/rpc.ts` 单文件封装前端 RPC 相关内容，包括 message 类型、WebSocket 连接、调用匹配、通知分发和连接状态。

### `service`

Go service。`service/backend/rpc.go` 单文件封装后端通用能力，包括 App Host HTTP、WebSocket connection、handler map、内建 `app.hello` handshake、`OnSessionReady`、session task 和通知发送。`NewServer` 返回 `*backend.Server`，扩展方可以在不修改分发逻辑的情况下用生成的 method 定义调用 `RegisterHandler`。Demo 的 `demo.ping` 注册和 `demo.tick` session task 放在 `service/main.go` 业务层。

### `script`

开发、构建、测试脚本目录。Windows 日常入口使用 `.bat`。复杂 JSON/codegen 和 dev 进程管理由无依赖 Node helper 完成。

`script/setup.bat` 是依赖安装入口。它在 `app` 下运行 `npm install`，并检查 Go、Node.js、npm 是否满足文档版本。

`script/codegen-methods.bat` 是 RPC method 生成入口，内部调用 `script/codegen-methods.mjs`。它读取并维护 `app/src/rpc/methods.json`，并生成 Go/TypeScript method 定义。TypeScript 生成结果统一通过 `rpc.<namespace>.<action>` 访问；Go 生成结果统一通过 `backend.<Namespace>.<Action>` 访问。

`script/dev.bat` 是开发总入口，内部调用 `script/dev.mjs`。它同时启动 Go service 和 Vite dev server，并把 Go service 地址注入给前端。

`script/test.bat` 是验证总入口。它运行 Go 测试、前端类型检查/测试、正式构建，并通过 `script/smoke-headless.mjs` 对生成的单文件 exe 做 smoke test。
它不自动安装依赖；如果 `app/node_modules` 不存在，则提示先运行 `script/setup.bat` 并退出。

`script/build.bat` 是正式构建入口。它不自动安装依赖；如果 `app/node_modules` 不存在，则提示先运行 `script/setup.bat` 并退出。

`bin/start-headless.bat` 是 headless 单独启动入口。它默认使用端口 `18080`，也可以用第一个参数指定端口，例如 `bin\start-headless.bat 18082`。

### `.build`

构建中间产物目录。该目录进入 `.gitignore`，用于前端静态资源、临时 package、测试产物等。

### `bin`

二进制输出目录。该目录进入 `.gitignore`。正式发布产物是单文件 Go exe，输出到 `bin/`。

## 构建产物规则

开发阶段使用 Vite dev server。

构建产物进入：

```text
.build/
bin/
```

`.build/` 和 `bin/` 都由 `.gitignore` 忽略。源码目录内不长期保留构建输出。

正式发布时，`bin/` 中的目标产物应是单文件 Go exe。前端资源需要在构建阶段打入该 exe，或以等价方式合并进单个可执行文件；发布目录不应依赖 exe 旁边散落的静态资源文件。

`script/build.bat` 的目标产物是：

```text
bin/app-host-demo.exe
```

构建流程为：

1. 前端构建输出到 `.build/embedweb/app/`。
2. 构建脚本生成 `.build/embedweb/go.mod` 和 `.build/embedweb/embedweb.go` 临时 Go package。
3. `.build/embedweb/` 使用 `go:embed` 嵌入自身 `app/` 目录下的前端资源。
4. 构建脚本复制 `service/` 到 `.build/service/` 临时构建目录。
5. 构建脚本在 `.build/service/` 中生成 release-only embed bridge，并在临时 `go.mod` 中引用 `.build/embedweb/`。
6. Go 构建阶段把前端资源合并进单文件 exe。
7. 最终只输出 `bin/app-host-demo.exe`。

源码目录不产生 `dist/`，发布运行不依赖 `.build/`。

## 开发运行方式

Demo 开发期使用两个进程：

```text
Go Service:       http://127.0.0.1:18080
Vite dev server:  http://127.0.0.1:5173
RPC WebSocket:    ws://127.0.0.1:18080/ws
```

Vite 通过环境变量或配置读取 Go RPC 地址。Go service 允许本地开发来源连接 WebSocket。

开发调试使用 headless service 模式。headless 模式只提供 `/healthz`、`/boot`、`/ws`，不服务内嵌 Web UI，也不自动打开 UI。前端页面由 Vite dev server 提供。

`script/dev.bat` 负责：

0. 检查 `app/node_modules` 是否存在；不存在则提示先运行 `script/setup.bat` 并退出。
1. 使用固定 dev 端口 `18080` 和 `--headless --port 18080` 启动 Go service。
2. 使用固定 dev 端口 `5173` 启动 Vite dev server。
3. 将固定 Go service 地址提供给前端，用于读取 `/boot` 和连接同端口 `/ws`。
4. 打开 Vite URL。
5. 任一进程退出时，清理另一个子进程。

`script/dev.bat` 需要捕获 `Ctrl+C` 和脚本退出事件，停止 Go service 与 Vite 两个子进程。如果任一子进程提前退出，脚本停止另一个子进程，并以非零退出码结束。

## WebSocket 稳定性

RPC Lib 需要把 WebSocket 连接视为可恢复 transport。运行期连接断开后，前端保持当前界面和运行期状态，不回到主界面，也不立即清空状态。

断线后 RPC Lib 每 200ms 尝试重连。重连成功后恢复 connected 状态。只有连续重试 50 次仍无法连接时，才把连接判定为 disconnected，并触发状态重置。

pending 调用在断线期间进入等待恢复状态。已经发送但尚未完成的调用，重连后不自动重发；断线期间新发起的调用进入队列，重连成功后再发送。所有 pending 调用的超时计时继续运行。

连续重试 50 次仍无法连接时，所有 pending 调用失败，错误码为 `1004`，然后触发状态重置。状态重置后 RPC Lib 继续自动重连；重连成功后重新读取 `/boot` 并建立新的 RPC session。

WebSocket 重连成功后必须重新执行 `app.hello`。重连后的 RPC session 是新 session，`demo.tick.count` 从 1 重新开始。

## 生产运行方式

Demo 生产期使用单文件 Go exe：

```text
bin/app-host-demo.exe
```

命令行参数：

- `--headless`：只启动 service，不自动打开 UI。
- `--port <int>`：指定 service 端口；普通模式不传则使用动态端口；headless 模式必须传。

启动流程：

1. Go exe 选择并绑定本地可用端口。
2. Go exe 启动 HTTP/WebSocket service。
3. Go exe 服务内嵌 Web UI。
4. Go exe 创建 WebView2 native window 并加载本地 Web URL，例如 `http://127.0.0.1:<port>/`。
5. 前端加载后请求 `/boot`。
6. 前端使用 `/boot` 返回的同端口 `/ws` RPC URL 建立 WebSocket 连接。
7. 前端发送 `app.hello` handshake。
8. `app.hello` 成功完成后，RPC ready。

普通模式默认使用系统分配的可用端口，服务内嵌 Web UI，并创建 WebView2 native window。headless 模式必须通过 `--port` 指定端口，只提供 `/healthz`、`/boot`、`/ws`，不服务内嵌 Web UI，不拉起 GUI；显式指定的端口被占用时，service 返回明确错误并退出。

WebView2 Runtime 使用本机已安装版本。第一阶段不随 exe 分发 WebView2 Runtime，也不自动下载或安装 Runtime。若本机缺少 WebView2 Runtime，exe 返回明确错误。

Go exe 生成本地 URL 后直接加载到 WebView2 native window。窗口加载期间由前端显示全屏 loading 动画，并每 200ms 尝试读取 `/boot`、建立 WebSocket 连接并完成 `app.hello` handshake。`app.hello` 成功后退出 loading。启动 loading 不设置超时，页面保持等待并持续重试，直到连接成功、用户关闭窗口或进程退出。

普通模式下，用户关闭 WebView2 主窗口后，Go exe 关闭 HTTP/WebSocket service 并退出进程，不保留常驻后台 agent。

控制台日志保持克制。默认输出监听地址、headless 状态、RPC URL 和关键错误，例如：

```text
app-host-demo listening on http://127.0.0.1:53123
headless=false
rpc=ws://127.0.0.1:53123/ws
```

默认不打印每条 RPC message，也不打印每次 `demo.tick`。

## 测试设计

`script/test.bat` 验证范围：

1. 在 `service` 下运行 Go 测试。
2. 在 `app` 下运行 TypeScript 类型检查和前端测试。
3. 运行 `script/build.bat`。
4. 检查 `bin/app-host-demo.exe` 存在。
5. 使用 `--headless --port <test-port>` 启动 `bin/app-host-demo.exe`。
6. 请求 `/boot` 并验证返回 `app`、`headless`、`rpcUrl`。
7. 关闭 smoke test 启动的 exe 进程。

### Go 测试

Go 侧优先测试：

- message JSON 编解码。
- 正整数和负整数 `id` 生成规则。
- 本地端口选择和 boot 信息生成。
- `/healthz` 返回 `ok` 和 `app`。
- `app.hello` 返回 `app` 和 `headless`。
- unknown method 返回 `code + message` 结构化错误，错误码为 `1001`。
- 非法 message 带合法 `id` 时返回错误码 `1002`；无合法 `id` 时关闭连接。
- 调用超时返回错误码 `1003`。
- `demo.ping` 返回 `{ "message": "pong" }`。
- WebSocket connection 能接收调用并返回完成消息。

### TypeScript 测试

前端侧使用 Vitest，优先测试 `app/src/rpc/rpc.ts`：

- RPC Lib 生成递增的正整数 `id`。
- RPC Lib 按 `id` 匹配完成消息。
- RPC Lib 能分发通知消息。
- 启动 loading 能等待 `/boot`、WebSocket 和 `app.hello` ready。
- WebSocket 断开后 RPC Lib 每 200ms 自动重连。
- 连续重试 50 次前 UI 状态保持。
- 连续重试 50 次后连接判定为 disconnected 并触发状态重置。
- 连续重试 50 次后所有 pending 调用以 `1004 connection_lost` 失败。
- 状态重置后继续自动重连；成功后重新 `/boot` 并建立新的 RPC session。
- WebSocket 重连成功后重新执行 `app.hello`，`demo.tick.count` 从 1 开始。
- 已发送未完成的调用重连后不自动重发。
- 断线期间新发起的调用排队，重连后发送。

### 手动验证

最小手动验证看三件事：

- 页面显示 connected。
- 点击 ping 后显示 pong。
- 页面持续收到 tick 通知。
- 页面展示 `/boot` 的 `app`、`headless` 和 `app.hello` 状态。

## 与正式框架的关系

APP Host Demo 是正式 App Host/RPC 的第一条垂直切片。它产生的可复用部分逐步沉淀到后续正式包中。

第一阶段用简化目录降低启动成本。前端先把 RPC 相关内容集中到 `app/src/rpc/rpc.ts`，method ledger 放在 `app/src/rpc/methods.json`；Go 侧把 App Host/RPC 通用能力集中到 `service/backend`，内建 `app.hello`，业务 method 注册放在 `service/main.go`，并通过 handler map 和 `OnSessionReady` 暴露扩展点；当 message、connection、router 和 transport 边界稳定后再拆分。

## 当前决策

- Demo 使用 Go service。
- Go 版本使用 1.26。
- Go WebSocket 使用 `github.com/gorilla/websocket`。
- Node.js 使用 v25.9.0，前端包管理器使用 npm 11.12.1。
- Demo 非 headless 使用 WebView2 native window 作为 UI surface。
- 开发调试使用固定端口，并由 `script/dev.bat` 打开 Vite URL；该浏览器路径只属于 headless 开发链路。
- 开发调试下 Go service 使用 `127.0.0.1:18080`，Vite 使用 `127.0.0.1:5173`。
- 开发调试使用 headless 模式：`--headless --port 18080`。
- Demo 正式产物是 `bin/app-host-demo.exe` 单文件 Go exe。
- Go exe 支持 `--headless` 和 `--port <int>`。
- headless 模式必须指定 `--port`，只提供 `/healthz`、`/boot`、`/ws`，不服务内嵌 Web UI，不打开 UI。
- 普通模式 HTTP 路由 API 优先，其余路径服务内嵌 Web UI，并 fallback 到 `index.html`。
- Go exe 负责选择本地端口、服务 Web UI、创建 WebView2 native window 并加载本地 Web URL。
- 普通模式下关闭 WebView2 主窗口会结束 Go exe。
- Go exe 不使用系统 URL handler 打开外部浏览器。
- WebView2 窗口加载后由前端全屏 loading 每 200ms 等待 `/boot`、WebSocket 和 `app.hello` ready；启动 loading 不设置超时。
- 控制台默认输出监听地址、headless 状态、RPC URL 和关键错误，不打印每条 RPC message 或 `demo.tick`。
- WebSocket 运行期断开后每 200ms 自动重连，连续失败 50 次前保持当前界面和状态。
- `script/dev.bat` 同时启动 Go service 和 Vite，并注入 service 地址。
- `script/codegen-methods.bat` 从 `app/src/rpc/methods.json` 生成 Go/TypeScript method 定义。
- `script/setup.bat`、`script/dev.bat`、`script/build.bat`、`script/test.bat` 都会先运行 method codegen。
- `script/setup.bat` 安装前端依赖并检查 Go/Node/npm 版本。
- `script/dev.bat` 不自动安装依赖；缺少 `app/node_modules` 时提示运行 `script/setup.bat`。
- `script/build.bat` 不自动安装依赖；缺少 `app/node_modules` 时提示运行 `script/setup.bat`。
- `script/test.bat` 不自动安装依赖；缺少 `app/node_modules` 时提示运行 `script/setup.bat`。
- `script/dev.bat` 捕获退出并清理 Go service 与 Vite；任一子进程提前退出则整体失败。
- 默认使用动态端口；显式 `--port` 被占用时失败。
- 前端通过 `/boot` 获取最终 RPC 地址。
- RPC WebSocket 与 HTTP service 同端口，路径为 `/ws`。
- WebSocket 建立后必须完成 `app.hello` handshake，成功后才算 RPC ready。
- `app.hello` 是普通 RPC 调用，占用前端正整数 `id` 序列。
- Demo 第一阶段手写最小 message envelope。
- Demo 使用单个整数 `id`。
- `id = 0` 非法。
- Message 不使用 `type` 字段，通过字段组合判断调用、完成、错误完成和通知。
- frontend 发起调用使用正整数 `id`。
- backend 发起调用使用负整数 `id`。
- Demo message 成功完成使用 `payload`。
- Demo message 失败完成使用 `error`，格式为整数 `code` + `message`。
- 错误码区间为 `1-9999` 内建，`10000+` 应用/业务 capability。
- RPC 调用默认超时 30 秒，可由调用方覆盖；超时包含断线和重连等待时间；超时错误码为 `1003`。
- 已发送未完成调用重连后不自动重发；断线期间新调用排队，重连后发送。
- 连续重连失败 50 次后，所有 pending 调用以错误码 `1004` 失败。
- disconnected 状态后继续自动重连，成功后重新 boot/session。
- 调用和通知都必须带 `payload`；空 payload 使用 `{}`。
- 成功完成必须带 `payload`；失败完成只带 `error`，不带 `payload`。
- 非法 message 带合法 `id` 时返回错误码 `1002`；无合法 `id` 时关闭连接。
- 未知 method 返回错误码 `1001`；前后端处理规则对称。
- Demo 第一阶段实现前端发起调用和后端通知。
- Demo UI 展示连接状态、boot 信息、hello 状态、ping 结果、最近 5 条 tick 和最近错误。
- `demo.tick` 每 2 秒发送一次通知。
- `demo.tick` 由业务层通过 backend `OnSessionReady` 在 `app.hello` 成功后开始推送。
- WebSocket 重连后的新 session 中，`demo.tick.count` 从 1 开始。
- `demo.tick` payload 为 `{ "count": number, "message": "tick" }`。
- method 使用全局唯一整数 id，永不复用；当前定义 `1000001 app.hello`、`2000001 demo.ping`、`2000002 demo.tick`。
- `.build/` 存放构建中间产物。
- `.build/embedweb/` 是正式构建时生成的临时 Go embed package。
- `.build/embedweb/app/` 存放正式构建时的前端静态资源。
- `.build/service/` 是正式构建时生成的 Go service 临时副本，用于加入 release-only embed bridge。
- `bin/` 存放单文件 Go exe 产物。
- `.build/` 和 `bin/` 都由 `.gitignore` 忽略。
- `script/test.bat` 覆盖 Go 测试、前端检查、正式构建和 exe smoke test。
- smoke test 使用 `--headless --port <test-port>` 启动 exe，避免测试时打开浏览器。


