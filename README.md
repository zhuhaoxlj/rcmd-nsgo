# rcmd-nsgo

一个用 [NSGo](https://nsgo.dev/zh/) 复刻 [rcmd](https://lowtechguys.com/rcmd/) 的 macOS 菜单栏应用：按住右 Command 键，按应用对应的字母，松开即切换。

本项目是独立实现，不包含 rcmd 的代码。行为与协议参考见 [docs/REVERSE-ENGINEERING.md](docs/REVERSE-ENGINEERING.md)。

## 功能

- 按住右 Command 显示应用切换面板（字母 + 应用图标 + 窗口数）。
- 按字母选中，松开右 Command 聚焦对应应用；未运行的应用会启动。
- `Option + 字母` 把该字母分配给当前前台应用，并持久化。
- 设置窗口里每个 app 的字母都就地可改：点字母格，直接按你想要的键（按退格取消），不需要先删再分配。下面列出正在运行、且有窗口的应用，每行有「分配 / 移除」按钮和可写的字母格；每行还有一个「忽略」按钮，点一下就把该 app 移进下方的忽略列表（在忽略列表里可再移除）。被忽略的 app 即使有分配字母也不会出现在切换面板。
- 「不显示面板（盲操）」开关：打开后按住触发键不弹面板，直接按字母就切换。它同时让枚举跳过图标与窗口计数，一次列举从约 64 ms / 90 KiB 降到约 18 ms / 1 KiB。
- `Esc` 取消本次切换。
- 动态字母：没有静态分配的应用按名称获得可用字母。
- 窗口数只统计真正的窗口：app 常驻的 36×22 之类辅助对话框不算，避免把单窗口的 app 报成「2 windows」。
- 菜单栏图标：打开设置、暂停切换、跳转辅助功能设置、退出。
- 设置窗口：权限状态、字母分配、忽略列表、登录启动等。
- 中文界面，默认中文；设置窗口里的「语言」可切到 English，立即生效，选择会记住。

## 宣传页

在线地址：<https://zhuhaoxlj.github.io/rcmd-nsgo/>

`site/index.html` 是一个单文件宣传页（无外部资源），可直接在浏览器打开：

```sh
open site/index.html
```

页面里的切换面板是真的可以按的：按住 ⌘ 再按字母，或直接点一个应用。

## 架构

```text
rcmd-nsgo（Go + NSGo）
   │  JSON Lines（stdin/stdout）
   ▼
rcmd-helper（Swift）
   │  CGEventTap / NSWorkspace / AXUIElement
   ▼
macOS
```

- **rcmd-nsgo**：界面、设置、状态机。NSGo 负责原生 AppKit 视图、面板与托盘。
- **rcmd-helper**：监听全局键盘、枚举/激活应用、枚举/聚焦/最小化窗口。与 rcmd 借助 Hammerspoon 的分工相同。

启用应用时用 LaunchServices（`NSWorkspace.openApplication`）而不是 `NSRunningApplication.activate(options:)`：后者在近几版 macOS 上返回 `true` 却不会真的把 app 提到前面。打开已在运行的 app 只会激活它，不会多开一个实例。

## 构建

要求：

- macOS（本项目只构建 macOS 目标）
- Go 1.27.3 或更新（NSGo 的要求）
- Xcode Command Line Tools（`swiftc`）

```sh
make helper   # 编译 Swift helper
make build    # 构建 .app，放入 helper 并重新签名
```

产物：`dist/rcmd-nsgo-0.1.0-macos_arm64/rcmd-nsgo.app`。

开发时可用：

```sh
make run
```

## 使用

1. 启动 app，菜单栏出现 `⌘`。
2. 首次运行会请求辅助功能与输入监控权限；在系统设置中勾选 `rcmd-helper`（以及 `rcmd-nsgo`）。
3. 按住右 Command，面板出现。
4. 按应用对应的字母，松开右 Command。
5. 想固定某个字母：在设置窗口的字母分配区点选运行中的应用即可，或先填字母再点；也可以按住右 Command 按 `Option + 字母` 分配给当前前台应用。

## 权限

| 权限 | 用途 |
| --- | --- |
| 辅助功能 | 全局监听按键、读取/移动其他应用的窗口 |
| 输入监控 | 在 macOS 10.15+ 上接收全局键盘事件 |

权限绑定到二进制路径。重新构建或移动 app 后，可能需要在系统设置里重新授权。

## 签名与权限

macOS 把辅助功能与输入监控的授权绑定到应用的**代码签名**上，而不是路径。

- **ad-hoc 签名（`codesign -s -`）** 的 designated requirement 就是二进制的 cdhash。cdhash 是内容哈希，每次重新编译都会变，于是：系统设置里的开关还亮着（那是旧构建的记录），但新二进制匹配不上，app 里读到的仍是未授权。表现就是「面板里明明开了，app 却说没权限」。
- 本项目的 `make build` 默认用一张固定的自签名证书（`make signing-identity` 一次性创建，放在 gitignore 的 `.signing/` 里）。这样 designated requirement 变成 `identifier "dev.mark.rcmdnsgo" and certificate leaf = H"..."`，重建不再使其失效，授权只需给一次。

```sh
make signing-identity   # 只跑一次；没有它时 make build 会回退到 ad-hoc 并给出警告
make build
```

输入监控必须在系统设置里手动打开（系统只弹一次提示）。macOS 27 上辅助功能一栏已改名为「设备控制和数据访问」。

若某次仍然对不上，清掉旧记录再给一次：

```sh
tccutil reset Accessibility dev.mark.rcmdnsgo
tccutil reset ListenEvent dev.mark.rcmdnsgo
```

设置窗口在权限未就绪时会就地显示上面这段原因，不必再猜。

## 目录

```text
main.go                  NSGo 入口
internal/app/            状态机与界面
internal/helper/         helper 进程桥接
internal/model/          设置模型与持久化
helper/main.swift        Swift 原生 helper
docs/REVERSE-ENGINEERING.md  逆向笔记
```

## 已知限制

- 只复刻了应用级切换；窗口级切换器、每应用设置、Hide/Minimize 键尚未实现。
- 右 Command 是固定触发键，尚未做成可录制。
- 面板不支持鼠标滚轮/键盘上下选择，只用字母。
- 动态字母排序是「当前应用优先，其余按名称」，与 rcmd 未逐项对齐。
- 构建产物使用项目自签名证书，只适合本机使用；换机器后需要重新跑一次 `make signing-identity` 并重新授权。
