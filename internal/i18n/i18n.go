// Package i18n holds the words the app shows, in the languages it
// speaks: Chinese by default, English on request.
package i18n

import "fmt"

// Lang is a language the app shows.
type Lang string

const (
	// Chinese is what the app shows unless the user picks otherwise.
	Chinese Lang = "zh"
	// English is the second language.
	English Lang = "en"
)

// Default is the language a fresh install shows.
const Default = Chinese

// Normalize returns the language v names, Default for anything else.
func Normalize(v string) Lang {
	if Lang(v) == English {
		return English
	}
	return Chinese
}

// messages maps a key to its Chinese and English text, in that order.
// Keys name where the text shows, so a missing one is easy to trace.
var messages = map[string][2]string{
	"settings.subtitle": {
		"按住右 Command 键，然后按应用对应的字母。",
		"Hold the right Command key, then press the letter of the app you want.",
	},

	"permissions.title": {
		"权限",
		"Permissions",
	},
	"permissions.detail": {
		"helper 需要辅助功能来监听按键并移动窗口，macOS 10.15 及更高版本还需要输入监控。",
		"The helper needs Accessibility to watch keys and move windows, and Input Monitoring on macOS 10.15 and later.",
	},
	"permissions.accessibility": {
		"辅助功能",
		"Accessibility",
	},
	"permissions.inputMonitoring": {
		"输入监控",
		"Input Monitoring",
	},
	"permissions.granted": {
		"已授权",
		"Granted",
	},
	"permissions.needed": {
		"未授权",
		"Needed",
	},
	"permissions.request": {
		"请求授权",
		"Request Access",
	},
	"permissions.open": {
		"打开系统设置",
		"Open System Settings",
	},
	"permissions.recheck": {
		"重新检查",
		"Recheck",
	},
	"permissions.stale": {
		"系统设置里可能已经勾选了 rcmd-nsgo，这里却仍显示未授权。这通常是因为那个开关属于更早的构建：macOS 把授权绑定到应用的代码签名，而用 ad-hoc 方式（codesign -s -）签名的构建每次都会换掉这个签名。改用项目固定的签名身份即可（make signing-identity，再 make build），或者在系统设置里把本应用关掉再重新打开一次。",
		"System Settings may show rcmd-nsgo switched on while this window still reads Needed. That happens when the switch belongs to an earlier build: macOS ties the grant to the app's code signature, and a build signed ad-hoc (codesign -s -) changes that signature every time. Sign with the project's fixed identity instead (make signing-identity, then make build), or switch the app off and back on in System Settings after each build.",
	},

	"settings.behaviour.title": {
		"行为",
		"Behaviour",
	},
	"behaviour.assignWithOption": {
		"用 Option 分配",
		"Assign with Option",
	},
	"behaviour.assignWithOptionDetail": {
		"按住触发键，再按 Option 加一个字母，把该字母分配给当前前台的应用。",
		"Hold the trigger key and press Option plus a letter to assign the app in front.",
	},
	"behaviour.cycleWindows": {
		"循环窗口",
		"Cycle windows",
	},
	"behaviour.cycleWindowsDetail": {
		"再次按下同一个字母，切换到该应用的下一个窗口。",
		"Pressing a letter a second time moves to the app's next window.",
	},
	"behaviour.launchAtLogin": {
		"登录时启动",
		"Launch at login",
	},
	"behaviour.launchAtLoginDetail": {
		"登录后自动启动 rcmd-nsgo。macOS 只对打包好的应用生效。",
		"Start rcmd-nsgo when you log in. macOS only takes this for a built app.",
	},

	"settings.language.title": {
		"语言",
		"Language",
	},
	"settings.language.detail": {
		"界面语言，改动立即生效。",
		"The language of this window. A change applies at once.",
	},
	"settings.language.chinese": {
		"中文",
		"中文",
	},
	"settings.language.english": {
		"English",
		"English",
	},

	"assign.title": {
		"字母分配",
		"Letter assignments",
	},
	"assign.empty": {
		"还没有分配字母。按住右 ⌘ 在应用上按 Option + 字母，或者在下面挑一个运行中的应用。",
		"No letters assigned yet. Hold right ⌘ and press Option + a letter over an app, or pick a running app below.",
	},
	"assign.help": {
		"在应用后面的字母格里直接按键即可指定该字母；点这一行的其他位置，会分配名字里下一个可用字母，已经有字母的则会移除。",
		"Type a key in an app's slot to set exactly that key, or press the row to give it the next free letter of its name. Pressing a row that already has a key removes it.",
	},
	"assign.search": {
		"搜索运行中的应用…",
		"Search running apps…",
	},
	"assign.noMatch": {
		"没有匹配的运行中应用。请在上面授予辅助功能，或先启动要分配的应用。",
		"No running apps match. Grant Accessibility above, or start the app you want to assign.",
	},
	"assign.assign": {
		"分配",
		"assign",
	},
	"assign.removeHint": {
		"移除",
		"remove",
	},
	"assign.removed": {
		"已移除 %s（%s）",
		"removed %s from %s",
	},
	"assign.set": {
		"%s → %s",
		"%s → %s",
	},
	"assign.noKeysLeft": {
		"所有字母都被占用了，请先移除一个。",
		"every key is taken — remove one first",
	},

	"ignored.title": {
		"忽略的应用",
		"Ignored apps",
	},
	"ignored.detail": {
		"被忽略的应用不会出现在切换面板里。",
		"Ignored apps never appear in the switcher.",
	},
	"ignored.empty": {
		"还没有忽略任何应用。",
		"No ignored apps.",
	},

	"common.remove": {
		"移除",
		"Remove",
	},
	"common.refresh": {
		"刷新",
		"Refresh",
	},
	"common.ignore": {
		"忽略",
		"Ignore",
	},

	"footer": {
		"rcmd-nsgo 0.1.0 · 用 NSGo 复刻的 rcmd。",
		"rcmd-nsgo 0.1.0 · a reverse-engineered rcmd, built with NSGo.",
	},

	"switcher.title": {
		"切换应用",
		"Switch to app",
	},
	"switcher.hint": {
		"松开 ⌘ 切换 · Esc 取消",
		"Release ⌘ to switch · Esc cancels",
	},
	"switcher.empty": {
		"还没有应用。请在上面授予辅助功能与输入监控，然后再次按住右 ⌘。",
		"No apps yet. Grant Accessibility and Input Monitoring above, then hold the right ⌘ again.",
	},
	"switcher.optionHint": {
		"Option + 字母：把该字母分配给前台应用",
		"Option + letter assigns the app in front",
	},
	"switcher.shiftHint": {
		"右 Shift + 字母：在共用同一字母的应用间循环",
		"Right Shift + letter cycles apps that share a letter",
	},
	"switcher.notRunning": {
		"未运行",
		"not running",
	},
	"switcher.windows": {
		"%d 个窗口",
		"%d windows",
	},

	"status.helperNotBuilt": {
		"helper 未构建：%s",
		"helper not built: %s",
	},
	"status.helperFailed": {
		"helper 启动失败：%s",
		"helper failed to start: %s",
	},
	"status.couldNotOpen": {
		"无法打开 %s：%s",
		"could not open %s: %s",
	},
	"status.couldNotList": {
		"无法列出应用：%s",
		"could not list apps: %s",
	},
	"status.noAppInFront": {
		"前台没有可分配的应用",
		"no app in front to assign",
	},

	"tray.tooltip": {
		"rcmd-nsgo — 按住右 Command 键",
		"rcmd-nsgo — hold the right Command key",
	},
	"tray.settings": {
		"设置…",
		"Settings…",
	},
	"tray.pause": {
		"暂停切换",
		"Pause switching",
	},
	"tray.accessibility": {
		"辅助功能设置…",
		"Accessibility settings…",
	},
	"tray.quit": {
		"退出 rcmd-nsgo",
		"Quit rcmd-nsgo",
	},
}

// T returns key's text in lang. A key with no text is returned as is, so
// a missing one shows up in the UI instead of hiding.
func T(lang Lang, key string) string {
	text, ok := messages[key]
	if !ok {
		return key
	}
	if Normalize(string(lang)) == English {
		return text[1]
	}
	return text[0]
}

// TF is T with fmt's verbs filled in.
func TF(lang Lang, key string, args ...any) string {
	return fmt.Sprintf(T(lang, key), args...)
}
