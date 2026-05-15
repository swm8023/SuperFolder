# SuperFolder

SuperFolder 是一个聚焦**文件管理**的复杂工具，定位为同时支持 **Native 桌面模式** 与 **Browser 浏览器模式** 的统一工作台。

## 当前状态

当前仓库已经包含 SuperFolder 第一版雏形：

- Go 后端负责 App Host、RPC、会话状态、收藏夹、目录加载、文件操作 Job、Git 摘要和文件预览。
- React/Vite 前端负责 SuperFolder 工作台 UI，包含左右双窗格、tab、收藏栏、右键菜单、下方多功能面板和 Job 面板。
- RPC method 由 `app/src/rpc/methods.json` 统一分配整数 id，并生成 Go/TypeScript 常量。
- 正式构建输出单文件 Go exe：`bin\superfolder.exe`。

## 产品方向

- 面向重度文件管理场景的本地工具
- 支持 Native 应用直接使用本机能力
- 支持浏览器访问同一套能力与工作流
- 后续可扩展目录浏览、批量整理、检索、预览、同步、自动化操作等能力

## 模式设计

### Native

适合直接访问本地文件系统、系统集成能力与高性能交互场景。

### Browser

适合通过网页进行访问、跨设备操作与统一入口管理。

## 仓库目标

先把可运行的 SuperFolder MVP 和可复用 App Host/RPC 基础稳定下来，再逐步补齐 Terminal、P4、目录监听、递归树、预览能力和系统集成细节。

## 常用脚本

Windows 日常入口使用 `script\*.bat`：

```bat
script\setup.bat
script\dev.bat
script\build.bat
script\test.bat
```

`.bat` 是 Windows 日常入口；复杂 JSON/codegen 和 dev 进程管理由无依赖 Node helper 完成。

开发模式：

- `script\dev.bat` 启动 headless Go service 和 Vite dev server，适合前端调试。
- `script\test.bat` 会运行 Go 测试、前端 typecheck/test、正式构建和 headless smoke。

正式构建：

```bat
script\build.bat
bin\superfolder.exe
```

普通模式由 `superfolder.exe` 启动本地后端并打开 WebView2 native window。

headless 单独启动入口：

```bat
bin\start-headless.bat 18080
```

headless 模式只监听指定端口，不打开 native window；它提供 `/boot` 和同端口 `/ws` RPC 入口。
