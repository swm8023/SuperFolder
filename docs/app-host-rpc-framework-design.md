# 通用 App Host 与 RPC 框架设计

## 定位

App Host 是一个项目无关的本地运行框架，用于承载 SuperFolder 以及未来其他应用。它负责启动会话后端、建立通信通道、加载前端 UI、管理窗口生命周期，并通过统一 RPC 系统向前端暴露能力。

本框架不绑定 SuperFolder 的文件浏览业务。SuperFolder 只是运行在 App Host 之上的一个应用。

## 核心目标

- 本地使用时交付一个 `.exe` 入口。
- `.exe` 启动 App Host/Session Backend 后，再加载前端 UI。
- `.exe` 选择本地可用端口，启动 HTTP/WebSocket service，并创建 native UI surface 加载本地 Web UI。
- 本地入口使用 Go `.exe` 启动 App Host，再创建 WebView2 native window 加载前端 UI。浏览器/Vite 只用于 headless 开发链路。
- 前端能力通过 RPC 调用后端，由 App Host 提供本地能力边界。
- RPC 合约统一，底层 Transport 可插拔。
- 本地优先使用低开销 IPC，远程使用 HTTPS/WSS，localhost HTTPS/WSS 保留给调试和兼容。
- RPC 支持双向调用、完成消息、通知、流、取消、超时、版本和结构化错误。
- 本地调试可使用 headless 运行方式，只监听 service 端口，不加载或打开 UI。

## 关键术语

- Desktop Executable：本地安装后的 `.exe` 入口，负责启动 App Host 并加载 UI。
- Desktop UI Surface：App Host 启动后加载的前端页面。Windows 产品路径使用 WebView2 native window；浏览器或 Vite dev server 只用于开发调试。
- App Host：项目无关的本地运行时，负责进程、窗口、通信和应用装载。
- Session Backend：随应用会话启动和停止的后端进程，不是常驻机器代理。
- Client：任何前端渲染入口，包括 WebView2 native window、本地浏览器开发页面或远程 Web 页面。
- RPC System：项目无关的通信系统，对上提供统一 API，对下适配不同 Transport。
- Transport：RPC 底层通信机制，例如本地 IPC、HTTPS、WSS。
- Protocol Envelope：RPC 消息外壳，承载整数 method id、整数 id、payload、版本、能力上下文、deadline、cancellation、stream id 和错误信息。
- Stream Payload：终端输出、目录变化、仓库日志、预览数据、大文件内容等增量或大块数据。

## 启动链路

本地启动链路为：

1. 用户启动 Desktop Executable。
2. Desktop Executable 选择并绑定本地可用端口。
3. Desktop Executable 启动 App Host。
4. App Host 初始化 Session Backend 和 RPC System。
5. App Host 打开本地 HTTP/WebSocket surface。
6. Desktop Executable 创建 WebView2 native window 并加载前端 UI surface。
7. 前端读取 boot 信息并建立 RPC 连接。
8. 前端通过 RPC handshake 获取应用能力、版本、权限和初始会话状态。

浏览器不会负责启动本地后端。浏览器访问只连接已经存在的 App Host/Backend 或远程服务入口。

headless 运行链路为：

1. 启动 Desktop Executable 时传入 headless 参数和显式端口。
2. App Host 只启动 HTTP/WebSocket service。
3. App Host 提供 boot、health 和 RPC endpoint。
4. App Host 不服务内嵌 UI，也不打开 UI surface。
5. 调试前端或自动化测试连接该显式端口。

## 技术边界

前端使用 TypeScript、React 和 Vite。Desktop Executable、App Host、Session Backend、RPC runtime、Transport 和通用能力注册使用 Go。

当前本地入口由 Go 实现。Windows 非 headless 模式通过 WebView2 native window 承载前端页面，作为 UI 适配层接入现有 Go App Host/RPC，业务能力仍由 Go App Host 提供。

## RPC 模型

RPC 系统采用自研框架和能力模型，但消息格式应保持可调试和可演进。第一版控制消息使用 JSON envelope，大块 payload 可使用二进制 chunk 或独立 stream。

基础消息需要支持：

- call/completion：调用和完成消息共享同一套 message envelope，通过整数 `id` 关联；成功完成统一使用 `payload`。
- server push：后端主动推送目录变化、进度、终端输出、仓库状态等事件。
- bidirectional call：连接双方都可以发起调用。
- streaming：支持长连接、增量结果和大块数据传输。
- cancellation：前端切换上下文或改变条件时可以取消旧调用。
- deadline/timeout：避免慢调用无限占用资源。默认调用超时 30 秒，可由调用方覆盖；超时从调用发起时开始计时，包含断线和重连等待时间。
- structured error：统一整数错误码、错误消息、可恢复性和调试信息。`1-9999` 为 RPC/App Host 内建错误码，`10000+` 为应用/业务 capability 错误码。
- versioning：协议版本、应用版本和能力版本可协商。

第一版基础 message 不使用 `type` 字段，通过字段组合判断语义：

- `id + method + payload` 表示调用。
- `id + payload` 表示成功完成。
- `id + error` 表示失败完成。
- `method + payload` 表示通知。

调用和通知必须带 `payload`，空 payload 使用 `{}`。成功完成必须带 `payload`。失败完成只带 `error`，不带 `payload`。

`method` 使用全局唯一整数 id，发布后永不复用。业务代码注册 method name，构建期生成 Go 和 TypeScript const。调试工具可以用 method name registry 把整数 id 显示成人类可读名称。

`id` 使用单个整数。前端发起调用使用正整数，后端发起调用使用负整数，`0` 为非法值。`id` 唯一性限定在单条 RPC connection 生命周期内。

第一版内建错误码：

```text
1001 method_not_found
1002 invalid_message
1003 timeout
1004 connection_lost
```

## Transport 策略

RPC 合约与 Transport 分离。

第一阶段本地开发使用 localhost HTTP/WebSocket。后续本地桌面场景可增加低开销 IPC Transport，例如命名管道或其他 Windows IPC。远程浏览器使用 HTTPS/WSS。

应用功能代码不应该感知当前 Transport。Transport 差异由 RPC System 屏蔽。

## 能力注册

App Host 应提供通用能力注册机制。应用模块向 App Host 注册 method id、method name、stream、权限要求、版本和生命周期钩子。SuperFolder 可以注册文件浏览、预览、Git、P4、Terminal、配置等能力；其他应用也可以复用同一套机制注册自己的能力。

method id 按全局区间分配。method name 只用于调试、日志和文档，不作为 wire dispatch key。建议 method name 仍使用两段式 `<namespace>.<action>`，例如：

- `app.session`
- `window.lifecycle`
- `config.store`
- `fs.browser`
- `terminal.session`
- `git.log`
- `p4.log`

namespace 只是调试名称组织方式，不应把 SuperFolder 业务写死进通用框架。

method id 分配需要稳定 registry 文件作为 source of truth。构建期扫描能力注册代码，发现新 method 时分配 id 并生成 Go/TypeScript 常量；已经发布的 id 不回收、不复用。

## 状态与持久化边界

前端可以维护运行期 View State，例如滚动位置、可见范围、hover、focus 等。这些状态可以在重启后重置。

需要跨重启恢复的状态由后端/App Host 持久化，例如窗口布局、打开的文件/目录 tab、当前路径、视图偏好、收藏目录、自定义右键菜单、Git/P4 配置和会话恢复数据。

前端可以编辑配置，但不作为 durable source of truth。

## 性能原则

- App Host 启动后应尽快返回 UI 可加载状态。
- 长耗时初始化应后台化，并通过 RPC 推送进度。
- 本地通信避免无意义 HTTP/WSS 开销，优先低开销 IPC。
- 大块数据使用 stream/chunk，不通过单个 JSON 响应传输。
- 高频事件需要合并、节流或按订阅范围推送。
- RPC 调用需要 cancellation，避免用户快速切换时堆积无效工作。
- 能力模块应按需加载，避免启动时加载所有业务能力。

## 构建产物原则

前端构建产物不应长期落在仓库源码目录中，也不应复制到 Go package 内部只为了 `go:embed`。生产打包可以在 `.build/` 下生成临时 embed package 和临时 service 构建副本，再编入 Go exe。

开发调试优先使用 Vite dev server 加 Go 后端，避免在项目目录生成大体积 `dist`。

正式发布目标是单文件 Go exe。前端资源应在构建阶段合并进该 exe，发布时不依赖 exe 旁边的静态资源目录。

## 安全边界

App Host 是受信任本地能力边界。Web UI 不直接获得文件系统权限，而是通过 RPC 访问受控能力。

远程访问必须考虑认证、授权、传输加密和能力暴露范围。第一版可以先定义协议字段和能力模型，不必立即实现完整远程账号体系，但协议不能假设所有连接都是本机可信连接。

## 当前决策

- 本地交付 `.exe`。
- 本地入口使用 Go `.exe`，非 headless UI surface 使用 WebView2 native window。
- 后端随客户端会话启动和停止。
- App Host/RPC 是项目无关基础设施。
- 前端使用 TypeScript/React/Vite，Desktop Executable、App Host 和后端使用 Go。
- headless 运行方式必须显式指定端口，只启动 service，不加载或打开 UI。
- RPC 框架自研，底层消息保持标准、可调试、可演进。
- RPC 支持双向调用、完成消息和后端通知。
- 第一版 RPC message 使用 `id`、整数 `method`、`payload`、`error` 字段，不使用 `type` 字段。
- method id 通过 registry/codegen 生成，业务代码只注册 method name。
- `id` 使用单个整数；前端调用为正数，后端调用为负数。
- RPC error 使用整数 `code` 和字符串 `message`。
- 统一 RPC 合约，Transport 可插拔。
- 本地优先 IPC，远程使用 HTTPS/WSS。
- 仓库内不保留前端构建产物或大体积临时目录。

## 后续设计主题

- 本地 IPC Transport 的具体实现。
- HTTPS/WSS 证书和本地安全策略。
- 通用能力注册 API 的 TypeScript/Go 接口形态。
- 多应用共用 App Host 时的隔离、安装和版本管理策略。
