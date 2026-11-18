# rcmd 逆向笔记

本文记录复刻对象的静态观察、由此得出的推断，以及仍未确认的部分。结论都标了证据来源，方便以后复核。

## 目标

| 项目 | 值 |
| --- | --- |
| 应用 | rcmd.app |
| 版本 | 2.7.0 |
| Bundle ID | `com.lowtechguys.rcmd` |
| 可执行文件 | 通用 Mach-O（x86_64 + arm64），约 11.5 MB |
| 应用包大小 | 约 4.9 MB |
| 分发形态 | Mac App Store 包（含 `_MASReceipt`、`AppReceiptValidator`） |

## 证据

- 应用包结构：`Contents/MacOS/rcmd`、`Contents/Resources/rcmd.lua`、`Contents/Resources/init.lua`、`Assets.car`。
- `rcmd.lua` 全文（336 行）是随包发布的明文脚本，由 Hammerspoon 执行。
- 二进制字符串：设置键、视图类名、菜单文案、错误信息。
- `otool -L`：SwiftUI、AppKit、Combine、ServiceManagement、UserNotifications、Starscream（WebSocket）。
- `swift-demangle`：`Lowtech.ObservableSettings`、`Lowtech.HashidsGenerator`、`rcmd.WindowFocuser` 等类型。

## 架构

**观察**：rcmd 是原生 SwiftUI 应用，窗口操作交给 Hammerspoon。

```text
rcmd.app (SwiftUI)
   │  HTTP + WebSocket, 127.0.0.1:3094
   ▼
Hammerspoon (Lua: rcmd.lua)
   │  hs.window.filter / Accessibility API
   ▼
macOS 窗口
```

`rcmd.lua` 启动一个本地 HTTP 服务并挂载 WebSocket：

- 端口 `3094`，只绑定 `localhost`。
- 路径 `/ws`，握手时校验 `token`（脚本里写作 `HSTOKEN` 占位）。
- 命令（`params.cmd`）：`reload`、`minimize`、`minimizeothers`、`minimizeall`、`launch`、`allwindows`、`windows`、`focus`。
- 窗口过滤角色：`AXFloatingWindow`、`AXStandardWindow`、`AXSystemDialog`、`AXSystemFloatingWindow`。
- 错误码：`NOAPP`、`NOWINDOW`、`Unauthorized`。

**推断**：主程序通过这套协议拿到窗口列表、聚焦/最小化窗口，并用 Hammerspoon 的辅助功能权限完成跨应用操作。选择 Hammerspoon 而不是自带 AX 代码，省去了自己处理辅助功能授权与窗口细节的成本。

## 功能地图

以下名称来自二进制中的 Swift 类型、设置键与界面文案：

- 触发：按住右 Command（可配置的 TriggerKey），显示应用切换器 OSD。
- 字母分配：
  - 动态分配：按应用名生成字母。
  - 静态分配：持久化，应用未运行时也可用字母启动。
  - `Option + 字母`：把字母分配给当前前台应用。
  - `Right Shift + 字母`：在共用同一字母的应用间强制循环。
  - `Left Shift + 字母`：聚焦时隐藏其他应用。
- 多窗口：
  - `cycle same letter apps`：同字母应用循环。
  - `focus specific windows`：聚焦指定窗口。
  - 实验性窗口切换器（`Try experimental window switching`）。
  - 最小化其他窗口 / 全部窗口。
- 行为设置：`whenAlreadyFocusedAction`、`App window focus behaviour`、每应用设置、忽略应用列表、登录启动、隐藏菜单栏图标。
- 相关视图类型：`SettingsView`、`AssignmentsView`、`RecordHotkeyView`、`WindowSwitchingSettingsView`、`MenuView`、`SwitcherRowView`、`WindowRow`、`OSDWindow`、`DynamicCycleWindowView`。
- 交互细节：`Tap detection threshold`、`Waiting for fn depress`、`Focus app after releasing`。

## 复刻映射

| rcmd | 本复刻 |
| --- | --- |
| SwiftUI 主程序 | NSGo（Go + AppKit 原生视图） |
| Hammerspoon + Lua | 自带的 Swift `rcmd-helper` |
| localhost HTTP/WebSocket | 标准输入/输出上的 JSON Lines |
| hs.window.filter | AXUIElement 窗口枚举 |
| 右 Command 触发 | CGEventTap 监听 keycode 54 |
| 字母 OSD | NSGo `desktop.OpenPanel` 面板 |
| 设置窗口 | NSGo `desktop.OpenWindow` |
| 登录启动 | `desktop.SetLaunchAtLogin` |

## 尚未复刻

- 窗口级切换器（只做了应用级切换）。
- 每应用独立设置。
- Menu/Actions 键与 Hide/Minimize 键。
- fn 键触发与敲击阈值。
- 应用图标缓存与动画细节。

## 未确认

- 动态字母的具体排序规则（推测为最近使用优先，未在静态证据中确认）。
- 设置持久化的内部格式（二进制含 `Error encoding settings to JSON`，推测为 JSON/Defaults）。
- 运行时 OSD 的位置、尺寸与动画参数。

## 工具限制

REA 的 Hopper 分析通道在当前机器上不可用：Hopper 启动器在执行 REA 注入的 Python 脚本时报 `The handler some object is not defined.`，`-Y` 脚本没有执行，因此没有得到 Swift 伪代码。上述结论全部来自包内脚本、字符串与符号层面的静态证据。
