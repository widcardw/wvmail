---
name: dev-debugging
description: How to debug the wvmail frontend on macOS 26, where the in-app Web Inspector (Wails Cmd+Opt+I) no longer opens
type: project
---

# 前端调试（macOS 26 / Tahoe）

## 问题：Cmd+Opt+I 打不开 DevTools

`wails3 dev` 按 Cmd+Opt+I（View → Open Developer Tools）**没有任何反应**，也没有任何报错。

### 原因：macOS 26 的 WebKit 回归 + Wails 用的是私有 API

Wails 的菜单项最终走到：

```
menuitem_dev.go  NewOpenDevToolsMenuItem()  →  Accelerator "Alt+Command+I"
   → WebviewWindow.OpenDevTools()
   → macosWebviewWindow.openDevTools()
   → mac_private_api_darwin.go: wailsPrivateOpenWebInspector()
   → [webView._inspector show]        ← 私有 API（_WKInspector）
```

macOS 26 上这条私有路径**静默失效**。本机实测（macOS 26.7 / Build 25G229）：

| 探测项 | 结果 |
|---|---|
| `_WKInspector` 类 | 存在 |
| `-[WKWebView _inspector]` / `-show` / `-showConsole` / `-attach` | 都存在，调用**不抛异常** |
| `show` / `showConsole` 之后 `inspector.connected` | **始终为 0**（不渲染） |
| `webView.inspectable = YES` | 生效 |
| `WKPreferences` 的私有 `developerExtrasEnabled` | **getter 已不存在**（KVC setValue 静默无效） |

对应的 WebKit 缺陷（未修复）：[WebKit bug 306964 — "WKWebView internal inspector split view fails to render inspector pane on macOS"](https://bugs.webkit.org/show_bug.cgi?id=306964)（Safari 26 / macOS 26，状态 NEW）。报告里明确写道：**应用内的 inspector 不渲染，但「Going through Safari's Dev Tools works with external window」**——与上面的实测完全一致。

所以：**这不是「macOS 26 禁止调试 WKWebView」，而是「应用内嵌的 inspector（私有 API）在 macOS 26 上坏了」**。公开的 `isInspectable` 仍然有效。

### 版本提示

- **alpha.97**：只设私有 `developerExtrasEnabled`，而该键在 macOS 26 上已失效 → 当时**Safari 也看不到**这个 webview。
- **beta.25（现在）**：优先使用公开 API `webView.inspectable = YES`（macOS 13.3+），私有键仅作为老系统兜底 → **Safari 路径可用**。

## 现在怎么调（两条路）

### 1. Safari 的 Develop 菜单（推荐，唯一官方路径）

1. Safari → 设置 → 高级 → 勾选「显示网页开发者功能」（Show features for web developers）。
2. 菜单栏 **开发 / Develop** → 找到你的 app（dev 模式是 `wvmail.dev`）→ 选中页面。
3. 外部 inspector 窗口打开，DOM / Console / Network 都可用（局部变量、断点都正常）。

> 前提是 dev 构建（`wails3 dev`）。`wails3 task build` 的 `-tags production` 构建里 `enableDevTools()` 是空实现，菜单项也不存在——这是有意为之。

### 2. 在浏览器里调前端（改 UI 最快）

`wails3 dev` 会同时跑 Vite（默认 `http://127.0.0.1:9245`），直接用 Safari/Chrome 打开该地址：

- 可以正常调试 Vue 组件、样式、UnoCSS、DOM；
- Wails 的 Go 调用（`#/wvmail/services`）在浏览器里不可用会报错，但 `App.vue` 的初始化已经 try/catch，界面仍会渲染；
- 需要真实后端数据时再回到应用窗口。

## 开发时部分 UnoCSS 图标突然消失

这是 WKWebView + Vite/UnoCSS HMR 的偶发重绘问题，不是图标 class 没生成，也不是布局遮挡。

排查时已经确认：

- 消失的 `<i>` 有正常的宽高和 `currentColor`；
- `mask-image` 是有效的 `data:image/svg+xml`；
- `document.elementFromPoint()` 命中的就是图标本身，没有其他图层覆盖；
- 在应用窗口按 `Cmd+R` 完整刷新后，标题栏、折叠侧栏和文件夹树里的图标会一起恢复。

因此遇到“只有一部分图标不见”时，先完整刷新页面，不要在图标组件上直接加边框、背景或 `position` 修补。若刷新后仍缺失，再检查 `uno.config.ts` 的 `safelist`；动态拼出来的 `i-${...}` class 必须列入 safelist。

## 什么时候能恢复快捷键

需要 WebKit 修好应用内 inspector，或 Wails 换实现（目前**没有**公开 API 能「以编程方式打开 inspector」，`isInspectable` 只是让 Safari 能看到它）。跟踪点：

- [WebKit bug 306964](https://bugs.webkit.org/show_bug.cgi?id=306964)
- Wails 侧代码：`pkg/application/mac_private_api_darwin.go` 的 `wailsPrivateOpenWebInspector`（beta.25）

## 快速自检脚本

想知道当前系统的私有 inspector 是否可用，编译一个最小 ObjC 探针即可（无需 GUI 交互）：

```objc
Class c = NSClassFromString(@"_WKInspector");
WKWebView *wv = [[WKWebView alloc] initWithFrame:NSMakeRect(0,0,100,100)];
wv.inspectable = YES;
id ins = [wv performSelector:NSSelectorFromString(@"_inspector")];
[ins performSelector:NSSelectorFromString(@"show")];
// 等待 1–3 秒后读 ins.connected：0 = 未连接（即当前问题）
```
