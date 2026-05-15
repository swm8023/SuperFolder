# 项目目录架构与技术选型

## 定位

本文档定义 SuperFolder 仓库的目录结构、技术选型、包职责、依赖方向和构建产物规则。它服务于下一步基础代码目录搭建，但不描述具体实现步骤。

项目分为三层：

1. 通用框架层：App Host、RPC、Transport、会话生命周期。
2. SuperFolder 业务层：文件浏览、配置、Git/P4、Terminal、Preview 等能力。
3. 前端与本地入口层：React Web UI、Go Desktop Executable。

## 技术选型

### 前端

- 语言：TypeScript。
- UI：React。
- 构建：Vite。
- Node.js：v25.9.0。
- 包管理器：npm 11.12.1。
- 测试：Vitest。
- 本地入口：Go `.exe` 启动 App Host 后创建 WebView2 native window 加载同一套 Web UI。
- 浏览器访问：普通浏览器连接已运行的 App Host/Session Backend。

React/Vite 是主前端技术线。产品选择浏览器/Web UI 与本地入口共享同一套前端能力，确保 Web 侧具备完整功能，本地入口只提供更高效的窗口承载和本机集成。

### 后端

- 语言：Go 1.26。
- WebSocket：`github.com/gorilla/websocket`。
- 职责：App Host、Session Backend、RPC runtime、Transport、配置/会话持久化、SuperFolder 后端能力。
- 关键能力：文件系统访问、目录 watch、子进程、Terminal、Git/P4 调用、Preview 生成、流式 RPC。

Go 是主要后端语言。后端业务逻辑、通用 RPC 和 SuperFolder 能力都应优先放在 Go 中。

### 本地入口

- 语言：Go。
- 职责：生成本地 `.exe`，启动 App Host/Session Backend，等待后端 ready，创建 WebView2 native window 加载 Web UI。

当前本地入口由 Go 实现。未来如果增加嵌入式窗口，应作为可替换 UI adapter 接入 Go App Host/RPC，不改变业务和协议核心。

## 推荐仓库结构

```text
SuperFolder/
  app/
  service/
  script/
  docs/
  .build/
  bin/
  CONTEXT.md
  README.md
```

## 目录职责

### `app`

React/Vite 前端应用。负责：

- 应用启动界面。
- Workspace Window 内的整体布局。
- 文件浏览器 UI。
- Utility Panel UI。
- RPC client 绑定。
- 前端运行期 View State。

该目录不持久化业务状态，不直接访问本机文件系统，不执行 Git/P4/Terminal 操作。

`app/src/rpc/rpc.ts` 封装所有前端 RPC 相关内容，包括 message 类型、WebSocket transport、调用匹配、通知分发和连接状态。

`app/src/superfolder` 是当前产品前端模块，包含业务类型、API 封装、状态 reducer、快捷键映射和 React 工作台组件。

### `service`

Go service。负责：

- 应用会话生命周期。
- 本地端口选择。
- Web UI 服务和 WebView2 native window 启动。
- RPC server/client runtime。
- Transport 抽象与具体实现。
- Capability 注册。
- 配置/会话存储基础设施。
- 静态 Web UI 的服务入口。
- 本地和远程连接的基础安全模型。

`service/backend` 封装后端通用能力，包括 App Host HTTP、WebSocket connection、handler map、内建 `app.hello` handshake、`OnSessionReady`、session task 和通知发送。

`service/superfolder` 封装当前产品后端能力，包括会话状态、收藏夹、目录加载、菜单、文件操作 Job、Git 摘要和预览。

`service/backend/methods_gen.go` 由 `script/codegen-methods.bat` 生成并提交，用于 Go 侧 method 命名空间变量和调试名称映射。

`NewServer` 返回 `*backend.Server`。产品后端通过自身 `Register(server)` 方法集中注册业务 handler，调用方只负责组合 App Host 与产品模块；session ready 后的 push 能力通过 `OnSessionReady` 挂载。

### `app/src/rpc`

RPC 协议级前端共享模块。`app/src/rpc/methods.json` 是 method id 的稳定分配源。method name 使用小写分段命名，例如 `folder.session.get`、`folder.children.list`。新增 method 在 JSON 中以 `"namespace.action": 0` 或更多分段声明，由 codegen 自动分配 id，已分配 id 永不复用。

TypeScript 生成文件导出嵌套 const 对象，例如 `rpc.app.hello`、`rpc.folder.session.get`。Go 生成文件导出命名空间变量，例如 `backend.App.Hello`、`backend.Folder.Session.Get`。业务调用和 handler 注册只能使用生成结果，不手写数字 method。

### `script`

开发、构建、测试脚本目录。Windows 日常入口使用 `.bat`；复杂 JSON/codegen 和 dev 进程管理由无依赖 Node helper 完成。

包含：

- `setup.bat`：安装前端依赖并检查 Go/Node/npm 版本。
- `codegen-methods.bat` + `codegen-methods.mjs`：维护 `app/src/rpc/methods.json` 并生成 Go/TypeScript method 定义。
- `dev.bat` + `dev.mjs`：启动 headless Go service 和 Vite。
- `build.bat`：生成单文件 Go exe；缺依赖时提示运行 `setup.bat`。
- `test.bat` + `smoke-headless.mjs`：运行测试、正式构建和 smoke test；缺依赖时提示运行 `setup.bat`。
- `bin/start-headless.bat`：从已有单文件 exe 启动 headless service，默认端口 `18080`，也支持第一个参数指定端口。

### `.build`

构建中间产物目录。该目录进入 `.gitignore`。

### `bin`

二进制输出目录。该目录进入 `.gitignore`。正式发布产物是单文件 Go exe。

### `docs`

设计文档、上下文和 ADR。当前核心设计文档为：

- `app-host-rpc-framework-design.md`
- `superfolder-file-browser-design.md`
- `project-structure-and-tech-selection.md`
- `app-host-demo-design.md` 保留为早期框架验证记录。

重大、难逆、存在真实 trade-off 的决策应进入 `docs/adr/`。

## 依赖方向

允许的依赖方向：

```text
app -> app/src/rpc
service -> service/backend
script -> app
script -> service
```

禁止的依赖方向：

```text
app -> service source imports
service -> app source imports
.build -> source imports
bin -> source imports
```

核心原则：

- 通用框架不能依赖 SuperFolder 业务。
- 前端不能直接依赖后端源码。
- RPC message 是契约，不是 runtime。
- RPC method 使用全局唯一整数 id，Go 和 TypeScript 侧由 codegen 保持一致；TypeScript 侧以 `rpc.<namespace>.<action>` 访问，Go 侧以 `backend.<Namespace>.<Action>` 访问。
- 本地入口只负责启动和组合，不是业务层。

## 构建产物规则

仓库内构建产物统一进入 `.build/` 和 `bin/`，避免散落在源码目录里导致 AI、搜索和代码索引扫描无意义的大文件。

明确禁止：

- 在 `app/dist` 长期保留构建产物。
- 在 Go package 下复制前端 `dist` 目录用于 embed。
- 在源码目录生成大体积中间文件。
- 提交 `.build/`、`bin/` 下的二进制产物、`dist/`、`coverage/`、`node_modules/`。`bin/*.bat` 可以提交为启动入口。

推荐规则：

- 开发调试：浏览器连接 Vite dev server，Go 后端独立运行。
- 正式构建：构建中间产物进入 `.build/`，其中 `.build/embedweb/app/` 存放前端静态资源，`.build/embedweb/` 是临时 Go embed package，`.build/service/` 是临时 Go service 构建副本。
- 发布前本地二进制进入 `bin/`，正式目标产物是单文件 Go exe。

`.build/` 和 `bin/` 中的二进制产物必须被 `.gitignore`、搜索脚本和 AI 约定排除；`bin/*.bat` 是源码入口，可以保留。

## 当前 MVP 验收目标

当前 SuperFolder MVP 应验证通用框架与产品雏形可以协同工作。

核心能力：

- Go App Host 启动 HTTP/WebSocket 服务。
- Go App Host 在普通模式创建 WebView2 native window 加载 Web UI。
- Go App Host 提供 `/boot` 让前端获取同端口 `/ws` RPC 地址。
- React/Vite 前端连接 Go 后端。
- 开发调试使用 headless service：Go service 固定 `127.0.0.1:18080`，Vite 固定 `127.0.0.1:5173`。
- WebSocket 建立后先完成 `app.hello` handshake。
- RPC method 定义由 `app/src/rpc/methods.json` 生成，TypeScript 调用侧使用 `rpc.app.hello`、`rpc.folder.session.get` 风格的嵌套 const，Go 调用侧使用 `backend.App.Hello`、`backend.Folder.Session.Get` 风格的命名空间变量。
- 后端提供会话状态、收藏夹、目录懒加载、菜单、文件操作 Job、Git 摘要和预览 RPC。
- 前端显示 SuperFolder 工作台，包含收藏栏、双文件窗格、tab、右键菜单、下方多功能窗口和 Job 状态。
- 开发调试不生成前端 `dist`。
- 仓库内不生成大体积临时产物。
- `script/build.bat` 输出 `bin/superfolder.exe` 单文件 Go exe。
- `script/test.bat` 覆盖 Go 测试、前端检查、正式构建和 exe smoke test。

第一版使用浏览器/Vite 验证前端迭代效率，正式运行由 `superfolder.exe` 创建本地窗口承载同一套 Web UI。

## 当前决策补充

- 第一版 RPC schema 手写最小 JSON message。
- 第一版本地 Transport 使用 HTTP/WebSocket。
- 第一版统一命令入口使用 `.bat`。
- 调试使用 headless service；普通模式由 Go exe 服务 Web UI 并创建 WebView2 native window。


