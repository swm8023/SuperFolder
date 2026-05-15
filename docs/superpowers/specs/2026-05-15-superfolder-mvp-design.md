# SuperFolder 初版文件管理器设计

## 背景

SuperFolder 已经完成最小 App Host Demo：Go 后端启动 HTTP/WebSocket service，React 前端通过 RPC 连接，构建脚本可以输出单文件 exe。下一步不再继续扩展 demo，而是把产品切到正式文件管理器雏形。

本设计定义第一版可运行 SuperFolder 文件管理器。它必须使用现有 App Host/RPC 底座，保留 Web 与本地入口同功能的原则，并开始接入真实文件系统能力。

## 产品身份

第一版从 `APP Host Demo` 切换为 `SuperFolder`：

- 应用名：`superfolder`。
- 窗口标题：`SuperFolder`。
- 构建产物：`bin\superfolder.exe`。
- UI 文案不再出现 `APP Host Demo`。
- `demo.*` method 移除，替换为真实业务 method。

## 目标

- 做出可运行的真实文件管理器雏形。
- 支持左侧收藏树、左右双文件浏览 pane、pane 内 tab、底部 Utility Panel。
- 支持详情列表、平铺/图标、pane 内树状视图三种浏览方式。
- 支持真实目录懒加载、排序、名称过滤和目录 hash 校验。
- 支持复制、移动、删除、重命名等基础文件操作。
- 支持内置右键菜单、核心快捷键、应用内文件剪贴板。
- 支持 Git 状态列与 Git Utility Tab 的真实简版能力。
- 支持文本和图片预览。
- 支持 session/config 后端持久化。

## 非目标

- 不做真实 Terminal shell，只保留 Terminal tab 壳。
- 不做 P4 真实能力，只保留 P4 tab 壳。
- 不做拖拽。
- 不做用户自定义右键菜单配置 UI。
- 不做 Windows 系统文件剪贴板互通。
- 不做远程访问权限模型。
- 不做大目录 range loading 和 watch 增量推送的完整实现。

## 代码边界

`service/backend` 继续作为通用 App Host/RPC 包，不写入 SuperFolder 文件浏览业务。

新增 `service/superfolder` 包承载业务能力：

- 配置和 session 持久化。
- 收藏树。
- 目录浏览。
- 文件操作 job 队列。
- 右键菜单注册和执行。
- 应用内文件剪贴板。
- Git 状态和 Git tab 数据。
- Preview 数据。

`service/main.go` 只负责组装：

```text
backend.NewServer(...)
superfolder.Register(server, options)
backend.Run(...)
```

前端继续位于 `app/`。`app/src/rpc` 保持 RPC client 与 method 常量。文件管理器 UI 可以按功能拆分到 `app/src/superfolder`，但前端不直接访问本机文件系统，不持久化业务状态。

## RPC Method 分组

method id 仍由 `app/src/rpc/methods.json` 作为 source of truth，构建期 codegen 生成 Go/TypeScript 常量。业务代码只使用生成常量，不手写数字。

第一版建议 method 分组：

- `app.hello`：保留通用 handshake。
- `folder.session.get`：获取或初始化 SuperFolder session。
- `folder.session.update`：保存 pane/tab/视图/Utility Panel 等 session 状态。
- `folder.favorites.list`：读取收藏树。
- `folder.favorites.update`：更新收藏配置。
- `folder.children.list`：按路径懒加载直接子项。
- `folder.open`：用 OS 默认程序打开文件，或请求目录导航。
- `folder.menu.list`：按上下文返回内置右键菜单。
- `folder.menu.execute`：执行内置右键菜单命令。
- `folder.clipboard.set`：设置应用内文件剪贴板。
- `folder.clipboard.paste`：粘贴到当前目录并创建 job。
- `job.list`：获取 job 队列快照。
- `job.cancel`：取消等待中或进行中的 job。
- `job.resolveConflict`：解决复制/移动冲突并继续 job。
- `git.status.refresh`：异步刷新当前路径所属 repo 状态。
- `git.summary.get`：获取 Git tab 简版数据。
- `preview.get`：获取文本或图片预览数据。

后端 push 建议：

- `folder.children.updated`：目录子项变化或 hash 更新。
- `job.updated`：job 状态、进度、冲突等待、完成或失败。
- `git.status.updated`：Git 状态列和 Git tab 数据更新。
- `preview.updated`：预览结果更新。

## 主界面

第一版界面采用正式文件管理器结构：

- 左侧 Sidebar：收藏目录树。
- 主区域上方：左右两个 File Browser Pane。
- 每个 pane 支持多个 tab。
- 每个 tab 有当前路径、视图模式、排序、过滤、树展开路径。
- 底部 Utility Panel 可折叠、可拖拽调整高度、支持大 tab 切换。

Utility Panel 第一版包含：

- Terminal：壳。
- Git：真实简版信息。
- P4：壳。
- Preview：文本和图片预览。

Utility Panel 的高度、折叠状态、active tab 由后端 session 保存。

## 默认启动状态

启动时优先恢复上次 session。

没有 session 时：

- 左 pane 打开用户 Home。
- 右 pane 打开 Downloads。
- 默认收藏包含 Home、Desktop、Downloads、Documents。
- 默认布局为左右双 pane，底部 Utility Panel 展开到默认高度。

配置和 session 默认写入 Windows 用户配置目录：

```text
%APPDATA%\SuperFolder\config.json
%APPDATA%\SuperFolder\session.json
```

写入使用临时文件加 rename，避免崩溃写坏文件。仓库目录、exe 目录和用户浏览的项目目录都不写入 SuperFolder 临时状态。

## 目录加载协议

所有文件浏览视图统一使用目录懒加载加 hash 协议。包括：

- 详情列表。
- 平铺/图标。
- pane 内树状视图。
- 左侧收藏树展开到真实目录时的子目录加载。

前端只维护当前可见树和列表的内存状态。展开一个目录时发起 RPC 请求，请求包含：

- `path`：目录路径。
- `knownHash`：前端当前已知的直接子项 hash，可以为空。
- `viewMode`：`details`、`tiles` 或 `tree`。
- `sortKey`：`name`、`kind`、`size`、`mtime`。
- `sortDirection`：`asc` 或 `desc`。
- `filterText`：名称过滤文本。

后端只读取该目录的直接子项，不递归扫描整棵树。

如果 `knownHash` 仍然有效，返回：

```json
{
  "path": "C:\\Users\\name",
  "unchanged": true,
  "childrenHash": "..."
}
```

如果目录发生变化，返回：

```json
{
  "path": "C:\\Users\\name",
  "unchanged": false,
  "childrenHash": "...",
  "entries": []
}
```

目录 hash 只覆盖直接子项的文件系统事实，不包含 Git/P4/Preview 状态。hash 输入字段包括 name、kind、size、mtime 和文件属性。Git/P4/Preview 作为独立状态层异步合并到 UI。

## 目录项字段

目录直接子项返回字段：

- `name`
- `path`
- `kind`：`file` 或 `directory`
- `size`
- `mtime`
- `readonly`
- `hidden`
- `system`
- `hasChildren`

排序和名称过滤由后端执行。前端只渲染结果，不自行维护持久排序结果。

## 视图模式

详情列表：

- 支持基础列：名称、类型、大小、修改时间、Git 状态。
- 支持 Ctrl/Shift 多选。
- 支持 inline rename。

平铺/图标：

- 支持文件和目录图标。
- 支持图片类文件的预览缩略表示，第一版可以先用通用图标加 Preview tab。
- 支持 Ctrl/Shift 多选。
- 支持 inline rename。

pane 内树状视图：

- 支持按层展开目录。
- 展开状态由后端 session 按 Browser Tab 保存。
- 初版只支持单选。

滚动位置、hover、focus、临时选择框等仍属于前端内存状态，重启后可以重置。

## 路径栏

每个 pane/tab 有自己的路径栏。

- 默认显示面包屑。
- 点击路径栏或按 `Ctrl+L` 进入文本路径编辑。
- 提交后通过后端校验路径。
- 路径是目录时导航到该目录。
- 路径不存在或不可访问时返回错误并保持原路径。

## 文件打开

双击目录：在当前 pane/tab 进入目录。

双击文件或右键“打开”：后端调用 Windows 默认关联程序打开该文件。

Preview 是辅助能力，不替代“打开”。

## 文件操作 Job

复制、移动、删除、重命名都通过后端 job 执行。即使小操作也统一进入 job 模型。

job 队列默认串行执行：

- 可以有多个等待中的 job。
- 进行中的 job 支持取消。
- 等待中的 job 支持取消。
- UI 显示队列、状态、进度、错误和完成结果。

job 状态建议：

- `queued`
- `running`
- `waiting_conflict`
- `cancelling`
- `completed`
- `failed`
- `cancelled`

文件操作错误由后端转换为结构化 RPC/job 错误。权限不足、路径不存在、文件被占用、目标冲突都需要返回明确 message。

## 删除语义

删除行为按 Windows 资源管理器语义：

- 普通 Delete 或菜单删除：进入 Windows 回收站。
- `Shift+Delete`：永久删除。
- 永久删除必须有确认弹窗。
- 回收站删除如果 Windows API 失败，不自动降级为永久删除，直接返回错误。

## 复制/移动冲突

复制/移动遇到同名冲突时，job 进入 `waiting_conflict`：

1. 后端通过 `job.updated` push 当前冲突信息。
2. 前端弹冲突对话框。
3. 用户选择覆盖、跳过、保留两者。
4. 用户可以选择“应用到全部”。
5. 前端通过 `job.resolveConflict` 返回选择。
6. 后端继续 job。

冲突不采用预扫描全量冲突的方式。冲突在执行过程中发现并处理，避免大目录或网络路径上预扫描过慢，也避免执行时状态已经变化。

## 重命名

重命名采用前端 inline edit。

用户提交新名称后：

- 前端发起 rename job。
- 后端校验名称、同目录冲突和权限。
- 成功后刷新对应目录。
- 失败时前端恢复原名并显示错误。

## 应用内文件剪贴板

`Ctrl+C` 和 `Ctrl+X` 不直接写 Windows 系统文件剪贴板，而是设置 SuperFolder session 内的应用内文件剪贴板：

- 选中路径列表。
- 操作类型：copy 或 cut。
- 来源 pane/tab。

`Ctrl+V` 在当前目录创建复制或移动 job。

右键“复制路径”写入系统文本剪贴板。

## 快捷键

初版支持 Windows 资源管理器核心快捷键：

- `Enter`：打开。
- `F2`：重命名。
- `Delete`：删除到回收站。
- `Shift+Delete`：永久删除并确认。
- `Ctrl+C`：复制到应用内文件剪贴板。
- `Ctrl+X`：剪切到应用内文件剪贴板。
- `Ctrl+V`：粘贴到当前目录。
- `Ctrl+L`：聚焦路径栏。
- `Ctrl+T`：新 tab。
- `Ctrl+W`：关闭 tab。
- `Alt+Left`：后退。
- `Alt+Right`：前进。
- `Backspace`：上一级。

## 右键菜单

初版只做内置右键菜单，不做用户自定义配置 UI。

内置菜单包括：

- 打开。
- 在新 tab 打开。
- 复制。
- 剪切。
- 粘贴。
- 重命名。
- 删除。
- 永久删除。
- 属性。
- 复制路径。

后端提供菜单注册机制，后续可以通过代码注册新的菜单项。前端只根据上下文请求菜单项并展示，不硬编码业务能力。

右键“属性”初版调用 Windows 文件属性对话框。

## Git 与 P4

Git 初版做真实简版能力：

- 文件列表显示 Git status 列。
- 后端识别当前目录所属 Git repo。
- 后端异步按需刷新 repo 状态。
- 切换路径、切换 tab 或进入目录时可以触发按需刷新。
- Git tab 提供手动刷新按钮。
- Git tab 显示 repo root、branch、状态摘要、最近 log 简版。

Git 刷新不能阻塞目录浏览：

- 目录 listing 先返回文件系统结果。
- Git status 独立后台刷新。
- 刷新完成后通过 push 更新状态列和 Git tab。
- Git 命令失败或超时只影响 Git 信息。
- 同一路径或同一 repo 的刷新请求需要合并，避免重复运行 Git 命令。

P4 初版只做 Utility Panel tab 壳，不接真实 P4 命令。

## Preview

Preview 初版支持文本和图片。

加载策略：

- 选中文件后异步加载。
- Preview tab 激活时优先展示当前选中文件。
- 文本文件只读取前 256KB。
- 图片由后端提供安全引用或数据引用，前端显示。
- 大文件返回“过大无法预览”。
- 加载失败只影响 Preview，不影响目录浏览。
- 快速切换文件时旧的 preview 请求可以取消或忽略结果。

## 性能原则

- 目录按层懒加载，不递归扫树。
- 前端对列表和平铺使用虚拟化渲染。
- 后端目录 hash 减少重复传输。
- Git 和 Preview 异步，不阻塞目录浏览。
- 文件操作统一 job，避免长操作阻塞 UI。
- 高频状态更新通过 push 合并到前端，避免前端轮询。
- 大目录 range loading 和 watch 增量推送作为后续扩展点保留。

## 错误处理

RPC 错误继续使用整数 `code` 和字符串 `message`。

`1-9999` 保留给 App Host/RPC 内建错误。SuperFolder 业务错误使用 `10000+`。

第一版业务错误建议覆盖：

- 路径不存在。
- 路径不可访问。
- 不是目录。
- 目标已存在。
- 权限不足。
- 文件被占用。
- 操作已取消。
- 预览文件过大。
- Git 命令失败。

## 测试策略

自动化测试覆盖核心协议和后端能力：

- Go 测试覆盖目录懒加载 hash。
- Go 测试覆盖 config/session 持久化。
- Go 测试覆盖 job 队列、取消和冲突状态。
- Go 测试使用临时目录覆盖复制、移动、重命名、非永久删除路径的可测试部分。
- Go 测试覆盖 Git 刷新不阻塞目录 listing 的行为。
- Vitest 覆盖 RPC method 调用封装。
- Vitest 覆盖前端文件树状态 reducer。
- Vitest 覆盖快捷键映射。

手动验收覆盖完整 UI：

- 启动非 headless exe。
- 启动 headless service 并用浏览器访问。
- 浏览真实目录。
- 切换左右 pane、tab 和三种视图。
- 展开树状视图并重启验证恢复。
- 复制/移动并处理冲突。
- 重命名。
- 删除到回收站。
- `Shift+Delete` 永久删除确认。
- Git 状态列和 Git tab。
- 文本/图片 Preview。
- Utility Panel 折叠、拖高和 tab 切换。

## 验收标准

第一版完成时应满足：

- `script\test.bat` 通过。
- `script\build.bat` 输出 `bin\superfolder.exe`。
- 非 headless 模式可以打开 SuperFolder 窗口并显示文件管理器 UI。
- headless 模式可以由浏览器访问同一套功能。
- 用户可以通过 UI 浏览本机目录。
- 用户可以完成复制、移动、重命名、删除、粘贴操作。
- 文件操作通过 job 队列反馈状态。
- session 重启后恢复左右 pane、tab、路径、视图模式、树展开和 Utility Panel 状态。
- 配置和 session 写入 `%APPDATA%\SuperFolder`，不污染项目目录。
