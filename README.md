# rcmd-nsgo

一个用 [NSGo](https://nsgo.dev/zh/) 复刻 [rcmd](https://lowtechguys.com/rcmd/) 的 macOS 菜单栏应用：按住右 Command 键，按应用对应的字母，松开即切换。

本项目是独立实现，不包含 rcmd 的代码。行为与协议参考见 [docs/REVERSE-ENGINEERING.md](docs/REVERSE-ENGINEERING.md)。

## 功能

- 按住右 Command 显示应用切换面板（字母 + 应用图标 + 窗口数）。
- 按字母选中，松开右 Command 聚焦对应应用；未运行的应用会启动。
- `Option + 字母` 把该字母分配给当前前台应用，并持久化。
- 设置窗口的字母分配区列出正在运行、且有窗口的应用：点一下即分配（已分配的再点一下移除），也可先在小输入框里写字母再点，指定想要的键。
- `Esc` 取消本次切换。
- 动态字母：没有静态分配的应用按名称获得可用字母。
- 菜单栏图标：打开设置、暂停切换、跳转辅助功能设置、退出。
- 设置窗口：权限状态、字母分配、忽略列表、登录启动等。

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
- 构建产物使用 ad-hoc 签名，只适合本机使用。
