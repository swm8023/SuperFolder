# SuperFolder

SuperFolder 是一个聚焦**文件管理**的复杂工具，定位为同时支持 **Native 桌面模式** 与 **Browser 浏览器模式** 的统一工作台。

## 当前状态

当前仓库仅完成项目初始化，暂不包含具体功能实现。

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

先把项目骨架、定位和协作基础搭好，再逐步补充架构、功能和实现细节。

## 常用脚本

Windows 日常入口使用 `script\*.bat`：

```bat
script\setup.bat
script\dev.bat
script\build.bat
script\test.bat
```

`.bat` 是 Windows 日常入口；复杂 JSON/codegen 和 dev 进程管理由无依赖 Node helper 完成。

headless 单独启动入口：

```bat
bin\start-headless.bat 18080
```
