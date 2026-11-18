// rcmd-helper watches the keyboard and moves other applications' windows.
//
// The NSGo app speaks JSON lines to it on stdin and stdout:
//
//   {"id":1,"action":"apps"}
//   {"id":1,"ok":true,"result":[...]}
//   {"event":"hold","on":true}
//
// It is the same split rcmd ships: a small privileged helper (rcmd used
// Hammerspoon) does the Accessibility work and the app draws the UI.

import AppKit
import ApplicationServices
import Foundation

// MARK: - Output

let outLock = NSLock()

func emit(_ object: [String: Any]) {
    guard JSONSerialization.isValidJSONObject(object),
          let data = try? JSONSerialization.data(withJSONObject: object),
          let text = String(data: data, encoding: .utf8) else { return }
    outLock.lock()
    FileHandle.standardOutput.write((text + "\n").data(using: .utf8)!)
    outLock.unlock()
}

func sendEvent(_ name: String, _ extra: [String: Any] = [:]) {
    var object = extra
    object["event"] = name
    emit(object)
}

// MARK: - Permissions

func accessibilityTrusted() -> Bool { AXIsProcessTrusted() }

func inputMonitoringTrusted() -> Bool {
    if #available(macOS 10.15, *) { return CGPreflightListenEventAccess() }
    return true
}

func reportPermissions() {
    sendEvent("status", [
        "accessibility": accessibilityTrusted(),
        "inputMonitoring": inputMonitoringTrusted(),
    ])
}

func requestPermissions() {
    let options = [kAXTrustedCheckOptionPrompt.takeUnretainedValue() as String: true] as CFDictionary
    _ = AXIsProcessTrustedWithOptions(options)
    if #available(macOS 10.15, *) { _ = CGRequestListenEventAccess() }
    reportPermissions()
}

// MARK: - Keyboard

var holding = false
var rightShift = false
var leftShift = false
var eventTap: CFMachPort?

func letter(for event: CGEvent) -> String? {
    guard let nsEvent = NSEvent(cgEvent: event),
          let chars = nsEvent.charactersIgnoringModifiers,
          let first = chars.first else { return nil }
    let lower = String(first).lowercased()
    guard let scalar = lower.unicodeScalars.first,
          (scalar.value >= 97 && scalar.value <= 122) ||
          (scalar.value >= 48 && scalar.value <= 57) else { return nil }
    return lower
}

func eventTapCallback(proxy: CGEventTapProxy, type: CGEventType, event: CGEvent, refcon: UnsafeMutableRawPointer?) -> Unmanaged<CGEvent>? {
    switch type {
    case .tapDisabledByTimeout, .tapDisabledByUserInput:
        if let tap = eventTap { CGEvent.tapEnable(tap: tap, enable: true) }
        return Unmanaged.passUnretained(event)

    case .flagsChanged:
        let code = event.getIntegerValueField(.keyboardEventKeycode)
        switch code {
        case 54: // right Command
            let down = event.flags.contains(.maskCommand)
            if down && !holding {
                holding = true
                sendEvent("hold", ["on": true])
            } else if !down && holding {
                holding = false
                sendEvent("hold", ["on": false])
            }
        case 60: // right Shift
            rightShift = event.flags.contains(.maskShift)
        case 56: // left Shift
            leftShift = event.flags.contains(.maskShift)
        default:
            break
        }
        return Unmanaged.passUnretained(event)

    case .keyDown:
        guard holding else { return Unmanaged.passUnretained(event) }
        let code = event.getIntegerValueField(.keyboardEventKeycode)
        if code == 53 { // Escape
            sendEvent("esc")
            return nil
        }
        if event.getIntegerValueField(.keyboardEventAutorepeat) != 0 {
            return nil
        }
        guard let ch = letter(for: event) else { return Unmanaged.passUnretained(event) }
        var shift = ""
        if rightShift { shift = "right" } else if leftShift { shift = "left" }
        sendEvent("letter", [
            "letter": ch,
            "option": event.flags.contains(.maskAlternate),
            "shift": shift,
        ])
        return nil

    case .keyUp:
        if holding { return nil }
        return Unmanaged.passUnretained(event)

    default:
        return Unmanaged.passUnretained(event)
    }
}

func installTap() -> Bool {
    let mask = (1 << CGEventType.keyDown.rawValue)
        | (1 << CGEventType.keyUp.rawValue)
        | (1 << CGEventType.flagsChanged.rawValue)
        | (1 << CGEventType.tapDisabledByTimeout.rawValue)
        | (1 << CGEventType.tapDisabledByUserInput.rawValue)
    guard let tap = CGEvent.tapCreate(
        tap: .cgSessionEventTap,
        place: .headInsertEventTap,
        options: .defaultTap,
        eventsOfInterest: CGEventMask(mask),
        callback: eventTapCallback,
        userInfo: nil
    ) else { return false }
    eventTap = tap
    let source = CFMachPortCreateRunLoopSource(kCFAllocatorDefault, tap, 0)
    CFRunLoopAddSource(CFRunLoopGetMain(), source, .commonModes)
    CGEvent.tapEnable(tap: tap, enable: true)
    return true
}

func keepTryingTap() {
    if installTap() {
        reportPermissions()
        return
    }
    reportPermissions()
    DispatchQueue.main.asyncAfter(deadline: .now() + 2) { keepTryingTap() }
}

// MARK: - Applications and windows

func windows(of pid: pid_t) -> [AXUIElement] {
    let app = AXUIElementCreateApplication(pid)
    var value: CFTypeRef?
    guard AXUIElementCopyAttributeValue(app, kAXWindowsAttribute as CFString, &value) == .success,
          let windows = value as? [AXUIElement] else { return [] }
    return windows
}

func windowTitle(_ window: AXUIElement) -> String {
    var value: CFTypeRef?
    guard AXUIElementCopyAttributeValue(window, kAXTitleAttribute as CFString, &value) == .success else { return "" }
    return value as? String ?? ""
}

func runningApp(bundle: String) -> NSRunningApplication? {
    NSRunningApplication.runningApplications(withBundleIdentifier: bundle).first
}

// realWindows are the app's ordinary windows, leaving out the tiny
// helper dialogs many apps keep around (a 36x22 AXSystemDialog and the
// like): those are not windows the user would want to switch to, and
// counting them made an app report two windows when it had one.
func realWindows(of pid: pid_t) -> [AXUIElement] {
    windows(of: pid).filter { window in
        let subrole = attribute(window, kAXSubroleAttribute) as? String ?? ""
        if subrole == "AXDialog" || subrole == "AXSystemDialog" { return false }
        let size = windowSize(window)
        return size.width >= 200 && size.height >= 120
    }
}

func attribute(_ element: AXUIElement, _ name: String) -> Any? {
    var value: CFTypeRef?
    guard AXUIElementCopyAttributeValue(element, name as CFString, &value) == .success else { return nil }
    return value
}

func windowSize(_ window: AXUIElement) -> CGSize {
    guard let value = attribute(window, kAXSizeAttribute as String), CFGetTypeID(value as CFTypeRef) == AXValueGetTypeID() else {
        return .zero
    }
    var size = CGSize.zero
    AXValueGetValue(value as! AXValue, .cgSize, &size)
    return size
}

func iconBase64(_ image: NSImage) -> String? {
    let side = 64
    guard let rep = NSBitmapImageRep(bitmapDataPlanes: nil, pixelsWide: side, pixelsHigh: side,
                                     bitsPerSample: 8, samplesPerPixel: 4, hasAlpha: true, isPlanar: false,
                                     colorSpaceName: .deviceRGB, bytesPerRow: 0, bitsPerPixel: 0) else { return nil }
    NSGraphicsContext.saveGraphicsState()
    NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: rep)
    image.draw(in: NSRect(x: 0, y: 0, width: side, height: side))
    NSGraphicsContext.restoreGraphicsState()
    guard let data = rep.representation(using: .png, properties: [:]) else { return nil }
    return data.base64EncodedString()
}

// iconCache keeps an app's encoded icon by bundle id: encoding a 64 pt
// PNG per app per listing cost more than everything else in it, and an
// app's icon does not change while it runs.
var iconCache: [String: String] = [:]

// listApps reports the regular running apps. A caller that will not show
// icons or window counts can ask for neither, which skips the PNG
// encoding and an AX query per app: that is most of the time this call
// takes, and a switcher that shows no panel needs neither.
func listApps(icons: Bool, counts: Bool) -> [[String: Any]] {
    var result: [[String: Any]] = []
    for app in NSWorkspace.shared.runningApplications {
        guard app.activationPolicy == .regular, let bundle = app.bundleIdentifier else { continue }
        var entry: [String: Any] = [
            "bundleID": bundle,
            "name": app.localizedName ?? bundle,
            "pid": Int(app.processIdentifier),
            "active": app.isActive,
        ]
        if counts {
            entry["windows"] = realWindows(of: app.processIdentifier).count
        }
        if icons {
            if let cached = iconCache[bundle] {
                entry["icon"] = cached
            } else if let icon = app.icon, let encoded = iconBase64(icon) {
                iconCache[bundle] = encoded
                entry["icon"] = encoded
            }
        }
        result.append(entry)
    }
    return result
}

// activate brings bundle to the front.
//
// NSRunningApplication.activate(options:) no longer does that on recent
// macOS: it returns true while the front app stays where it was. Going
// through LaunchServices does raise the app, and opening an app that
// already runs activates it rather than starting a second instance.
func activate(bundle: String) {
    if let app = runningApp(bundle: bundle), let url = app.bundleURL {
        raiseStandardWindow(of: app)
        openApp(at: url)
        return
    }
    guard let url = NSWorkspace.shared.urlForApplication(withBundleIdentifier: bundle) else { return }
    openApp(at: url)
}

func openApp(at url: URL) {
    let configuration = NSWorkspace.OpenConfiguration()
    configuration.activates = true
    configuration.createsNewApplicationInstance = false
    NSWorkspace.shared.openApplication(at: url, configuration: configuration, completionHandler: nil)
}

func raiseStandardWindow(of app: NSRunningApplication) {
    guard let window = realWindows(of: app.processIdentifier).first else { return }
    AXUIElementPerformAction(window, kAXRaiseAction as CFString)
}

func focusWindow(bundle: String, index: Int) {
    guard let app = runningApp(bundle: bundle) else { return }
    let list = realWindows(of: app.processIdentifier)
    guard !list.isEmpty else { return }
    let window = list[min(max(index, 0), list.count - 1)]
    AXUIElementPerformAction(window, kAXRaiseAction as CFString)
    openApp(at: app.bundleURL ?? URL(fileURLWithPath: "/"))
}

func setMinimized(_ window: AXUIElement, _ value: Bool) {
    AXUIElementSetAttributeValue(window, kAXMinimizedAttribute as CFString, value as CFBoolean)
}

func minimize(bundle: String, all: Bool) {
    guard let app = runningApp(bundle: bundle) else { return }
    let list = realWindows(of: app.processIdentifier)
    for (i, window) in list.enumerated() {
        if !all && i == 0 { continue }
        setMinimized(window, true)
    }
}

func hide(bundle: String) {
    runningApp(bundle: bundle)?.hide()
}

func hideOthers() {
    let front = NSWorkspace.shared.frontmostApplication?.processIdentifier
    for app in NSWorkspace.shared.runningApplications where app.activationPolicy == .regular {
        if app.processIdentifier != front { app.hide() }
    }
}

func frontmost() -> [String: Any] {
    guard let app = NSWorkspace.shared.frontmostApplication else { return [:] }
    return [
        "bundleID": app.bundleIdentifier ?? "",
        "name": app.localizedName ?? "",
        "pid": Int(app.processIdentifier),
        "active": true,
    ]
}

// MARK: - Requests

func handle(_ request: [String: Any]) {
    let id = request["id"]
    let action = request["action"] as? String ?? ""
    let payload = request["payload"] as? [String: Any] ?? [:]
    let bundle = payload["bundle"] as? String ?? ""

    var result: Any = NSNull()
    var ok = true
    var failure = ""

    switch action {
    case "apps":
        let icons = payload["icons"] as? Bool ?? true
        let counts = payload["counts"] as? Bool ?? true
        result = listApps(icons: icons, counts: counts)
    case "activate":
        activate(bundle: bundle)
    case "focusWindow":
        focusWindow(bundle: bundle, index: payload["index"] as? Int ?? 0)
    case "minimizeOthers":
        minimize(bundle: bundle, all: false)
    case "minimizeAll":
        minimize(bundle: bundle, all: true)
    case "hide":
        hide(bundle: bundle)
    case "hideOthers":
        hideOthers()
    case "frontmost":
        result = frontmost()
    case "permissions":
        result = ["accessibility": accessibilityTrusted(), "inputMonitoring": inputMonitoringTrusted()]
    case "requestPermissions":
        requestPermissions()
    default:
        ok = false
        failure = "unknown action \(action)"
    }

    if let id = id {
        emit(["id": id, "ok": ok, "error": failure, "result": result])
    }
}

// MARK: - Main

func readRequests() {
    DispatchQueue.global(qos: .userInitiated).async {
        while let line = readLine(strippingNewline: true) {
            guard !line.isEmpty,
                  let data = line.data(using: .utf8),
                  let object = try? JSONSerialization.jsonObject(with: data) as? [String: Any] else { continue }
            DispatchQueue.main.async { handle(object) }
        }
        exit(0)
    }
}

signal(SIGTERM) { _ in exit(0) }
signal(SIGINT) { _ in exit(0) }

readRequests()
keepTryingTap()
CFRunLoopRun()
