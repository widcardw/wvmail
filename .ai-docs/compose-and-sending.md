---
name: compose-and-sending
description: Outbound mail architecture in wvmail — SMTP, MIME/CID, compose window lifecycle, attachments, reply/forward and mailto routing
type: project
---

# 写信与发信：从编辑器到 SMTP

本文记录 wvmail 的出站邮件链路。撰写基线为 Wails `v3.0.0-beta.25`、Go 1.25、
Vue 3、Tiptap `3.22.5`。

> 行号会漂移；以 service、method 和组件名为稳定锚点。

## 1. 总览

```
主窗口「写邮件」/ 回复 / 转发 / mailto:
  │
  └─ ComposeService.OpenCompose / OpenMailto
       │
       ├─ 校验账户并创建内存 ComposeDraft
       ├─ 复用或创建名为 compose 的 Wails 子窗口
       └─ URL 只带 draft ID：/#/compose?id=...
            │
            └─ ComposeView
                 ├─ Tiptap 富文本
                 ├─ 收件人 / 抄送 / 密送 / 主题
                 ├─ 附件和 CID 内联图片
                 └─ SendDraft
                      │
                      ├─ Go 把编辑器预览图替换成 cid: 引用
                      ├─ EmailSendService 构建 MIME
                      ├─ SMTP 发送
                      ├─ best-effort IMAP APPEND 到 Sent
                      ├─ 发出 compose:sent
                      └─ 关闭子窗口并删除暂存目录
```

发送链路与 IMAP 取信链路刻意分开：

- `EmailSendService` 只负责 SMTP 和 Sent 副本，不改 IMAP 缓存。
- `ComposeService` 只负责草稿、附件暂存和写信窗口生命周期。
- 发送成功后会额外发出已有的 `emails-synced` 事件，让主窗口刷新邮件列表。

## 2. 关键文件

| 路径 | 职责 |
|---|---|
| `services/emailmime.go` | MIME 构建、地址/附件校验、mailto 解析、纯文本 fallback |
| `services/emailsend.go` | SMTP 连接/认证/发送、Sent 文件夹探测与 APPEND |
| `services/composeservice.go` | 单例写信窗口、内存草稿、附件暂存、关闭确认 |
| `services/composehtml.go` | 发送前把 `data-compose-image-id` 图片替换成 `cid:` |
| `frontend/src/views/ComposeView.vue` | 写信表单、自动保存、拖放、发送/放弃 |
| `frontend/src/components/compose/ComposeEditor.vue` | Tiptap 编辑器与格式工具栏 |
| `frontend/src/composables/compose.ts` | 收件人文本解析与格式化 |
| `frontend/src/composables/reply.ts` | 回复/回复全部/转发的纯函数规则 |
| `frontend/src/App.vue` | 应用级 `mailto:` 点击兜底和 `compose:sent` toast |
| `frontend/src/components/detail/EmailDetail.vue` | 阅读窗格中的回复、回复全部、转发与正文引用 |

## 3. 草稿与写信窗口

### 3.1 草稿只存在内存中

`ComposeDraft` 保存在 `ComposeService.state`，包含：

- 账户、To/Cc/Bcc、主题、纯文本和 HTML；
- 普通附件与内联图片的元数据；
- `InReplyTo` 和 `References`。

窗口 URL 只传 `draftID`。收件人、HTML 和附件不会塞进 URL，避免长度限制和泄漏。
当前版本没有持久化草稿：进程退出或确认放弃后内容即丢失。

### 3.2 只有一个写信窗口

Wails 窗口名固定为 `compose`。`OpenCompose` 的规则是：

1. 没有窗口：创建 920×720 的子窗口。
2. 已有干净窗口：复用窗口并 `SetURL` 到新草稿。
3. 已有脏草稿：不覆盖，聚焦旧窗口并返回
   `Reused=true` / `Warning="another compose draft is still open..."`。

这是第一版有意采用的保守行为：宁可提示用户先处理当前草稿，也不静默丢掉未发送内容。

### 3.3 关闭确认

- 子窗口注册 `WindowClosing` hook。
- 草稿未标记 `allowClose` 时取消关闭，并向前端发
  `compose:confirm-close`。
- `ComposeView` 显示 `ConfirmDialog`；确认放弃后才调用
  `ComposeService.CloseCompose`。
- 发送成功时 Go 先设置 `allowClose`，再关闭窗口，因此不会再触发放弃确认。

### 3.4 自动保存

`ComposeView` 监听表单和 Tiptap 内容，450ms 防抖后调用
`ComposeService.UpdateDraft`。初始化时用 `hydrating` 和 `nextTick` 避免
“刚打开窗口就被标成脏草稿”。

## 4. 附件和内联图片

### 4.1 暂存目录

用户选中的文件先复制到：

```
<os.UserConfigDir>/wvmail/compose/<draftID>/
```

目录权限为 `0700`，文件权限为 `0600`。复制后：

- 普通附件保存为 `ComposeAttachment{Inline:false}`；
- 图片保存为 `Inline:true`，生成稳定 `ContentID` 和仅供编辑器预览的 data URL。

发送、放弃或替换草稿时会删除对应目录。

### 4.2 为什么发送前要在 Go 侧替换 CID

Tiptap 编辑器必须看到 data URL 才能立即预览图片，但发出去的 HTML 不能携带整张
base64 图片。编辑器的 `<img>` 同时带：

```html
data-compose-image-id="..."
src="data:image/png;base64,..."
```

`SendDraft` 调用 `replaceComposeImageSources`：

1. 用 `golang.org/x/net/html` 解析 HTML fragment；
2. 按 `data-compose-image-id` 找到对应图片；
3. 改写为 `src="cid:<ContentID>"`；
4. 删除编辑器专用属性；
5. 只把改写后的副本交给 MIME builder。

内存草稿仍保留 data URL，所以发送失败后可以直接重试，图片预览不会变成破图。

### 4.3 限制

- 单个附件上限 20 MiB；
- 整封 MIME 消息上限 25 MiB；
- 普通附件 + 内联图片合计最多 20 个；
- 文件会完整读入内存后再构建 MIME；
- 目前没有断点续传、压缩或大文件分片。

## 5. SMTP

### 5.1 连接和 TLS

`EmailSendService.connect` 使用 `dialerForAccount(account, 30s)`，因此账号的代理
设置与 IMAP 走同一套规则。

| 配置 | 行为 |
|---|---|
| SMTP 端口 465 | 隐式 TLS，TLS 最低版本 1.2 |
| 非 465 且 `UseTLS=true` | 明文 TCP 连接后立即 `STARTTLS` |
| 非 465 且 `UseTLS=false` | 不升级 TLS |

与 IMAP 不同，SMTP 没有“STARTTLS 失败后静默回退明文”的阶梯。配置为 TLS 时，
握手失败就是发送失败。

### 5.2 认证

- `AuthType=oauth2`：刷新 access token，用 SASL XOAUTH2 认证。
- 其它情况：使用 `SMTPPass`；为空时回退到 `IMAPPass`。
- 密码认证使用 SASL PLAIN。
- `net/smtp.Auth` 在 Go 1.25 的接口由 `smtpSASLAuth` 适配到
  `emersion/go-sasl`，避免维护第二份 XOAUTH2 实现。

### 5.3 Envelope 与 Bcc

SMTP envelope 收件人由 To + Cc + Bcc 去重合并。MIME 头只写 To 和 Cc：

```text
RCPT TO: bcc@example.com   ← 有
Bcc: bcc@example.com       ← 不写
```

这样密送收件人仍能收到邮件，但地址不会出现在邮件头中。

## 6. MIME 结构

`buildEmailMessage` 使用 `go-message`：

### 6.1 纯文本和 HTML

- Text 和 HTML 都有：`multipart/alternative`，plain 在前、HTML 在后。
- 只有 HTML：HTML 正文。
- 只有 Text：纯文本正文。
- 若调用方漏了其中一个，`EmailSendService` 会先从另一份派生：
  HTML → plain，plain → 安全转义的 HTML。

### 6.2 附件与内联图片

| 内容 | 顶层结构 |
|---|---|
| 正文 | `multipart/alternative`（即使只有一个 child 也保持 multipart 容器） |
| 正文 + 内联图 | `multipart/related` → `multipart/alternative` + images |
| 正文 + 附件 | `multipart/mixed` → `multipart/alternative` + files |
| 正文 + 内联图 + 附件 | `multipart/mixed` → (`multipart/related` → alternative + images) + files |

文本 part 使用 quoted-printable；文件和内联图使用 base64。
附件文件名通过标准 `Content-Disposition` 参数编码，中文文件名可往返。

### 6.3 线程头

- 回复和回复全部写 `In-Reply-To` 与原邮件的 `Message-ID`。
- `References` 当前只写这个 `Message-ID`；`EmailDetail` 尚未暴露原始
  References 链，因此不会伪造完整历史链。
- 转发不写线程头。

## 7. Sent 副本

SMTP 成功后，`EmailSendService` 会尝试：

1. `ListFolders`；
2. 先按 `\Sent` 属性找，再按常见名称
   `Sent`、`Sent Mail`、`已发送`、`[Gmail]/Sent Mail` 等回退；
3. 通过 IMAP `APPEND` 写入原始 MIME，并带 `\Seen`。

这是 **best-effort**：

- APPEND 失败不会把已经成功的 SMTP 发送改成失败；
- `SendEmailResult.Warning` 会说明“已发送，但保存到已发送失败”；
- 找不到 Sent 文件夹时直接跳过，不报错。

## 8. 回复、回复全部和转发

规则集中在 `frontend/src/composables/reply.ts`，有
`frontend/tests/reply.test.mts` 覆盖。

### 8.1 回复

- To：原邮件发件人；
- Cc/Bcc：空；
- 主题：没有 `Re:` 时补 `Re:`；
- 正文：顶部留空段落，随后是本地化引用说明和净化后的原正文 blockquote；
- 设置线程头。

### 8.2 回复全部

- To：原发件人；
- Cc：原 To + 原 Cc；
- 去掉当前账户地址和发件人地址；
- 地址按不区分大小写去重；
- 主题和线程头同回复。

### 8.3 转发

- To/Cc/Bcc：空，由用户填写；
- 主题：没有 `Fwd:`/`Forward:` 时补 `Fwd:`；
- 正文包含 From、Date、Subject、To 和引用正文；
- 不设置 `In-Reply-To` / `References`。

### 8.4 当前限制

- 不自动携带原邮件附件；
- 引用正文优先使用已净化的 HTML；垃圾邮件门禁未放行时不引用正文；
- 纯文本 fallback 依赖原邮件的 `body_text`；
- 没有“回复时自动引用到光标位置”或折叠引用 UI。

## 9. `mailto:` 路由

应用内点击 `mailto:` 链接不会交给系统默认邮件客户端：

- `App.vue` 在 document 上做应用级 click 兜底；
- 已由 `EmailDetail` 处理并 `preventDefault` 的点击不会重复打开；
- `ComposeService.OpenMailto` 解析 To/Cc/Bcc/Subject/Body；
- Body 同时生成纯文本和安全转义的 HTML；
- 如果已有脏草稿，仍然遵守单窗口规则，返回 warning 而不是覆盖。

当前 **不注册** `mailto:` 为操作系统级默认邮件客户端。这是应用内路由，不是
macOS/Windows 的 URL scheme 注册。

## 10. 事件和错误语义

### 10.1 `compose:sent`

发送成功后、关闭窗口前，Go 发出：

```ts
{
  accountID: string
  messageID: string
  warning: string
}
```

主窗口 `App.vue` 监听并显示成功 toast；有 warning 时显示警告。

### 10.2 `emails-synced`

发送成功后同时发出已有事件，前端据此刷新当前文件夹，尽快看到 Sent 副本。

### 10.3 失败时

- SMTP 未接受消息：`SendDraft` 返回 error，窗口保持打开，草稿和附件保留。
- SMTP 已接受但 Sent APPEND 失败：发送成功，返回 warning。
- SMTP QUIT 失败：消息已经被接受，只记 debug 日志，不报发送失败。

## 11. 已知缺口

- 草稿不持久化，应用重启后丢失。
- 不支持 Markdown/富文本模式切换；当前只提供 Tiptap 富文本。
- 不支持定时发送、撤销发送、签名、模板。
- 转发不附带原附件，回复不带原附件。
- 收件人输入只做轻量解析，没有地址簿自动补全。
- 发送成功后依赖 IMAP 刷新看到 Sent 副本；没有本地乐观插入。
- 没有 OS 级 `mailto:` 默认客户端注册。

## 12. 验证

### 12.1 自动测试

```bash
go test ./services -count=1
cd frontend
pnpm typecheck
pnpm lint
pnpm test
pnpm build
```

现有覆盖：

- Go：multipart/alternative、中文主题/附件名、内联 CID、Bcc 头不泄漏、
  单纯文本、mailto 解析、Sent 文件夹识别、HTML → plain；
- Go：编辑器预览图 → CID 替换，以及无图片 HTML 保持不变；
- 前端：回复/回复全部/转发收件人规则、去重、线程头、主题前缀、HTML 转义；
- 前端：收件人文本解析和现有全部邮件渲染/安全测试。

### 12.2 手工发送矩阵

准备两个已配置账户 A/B，并用自己创建的测试文件：

1. A → B：只有纯文本正文。
2. A → B：富文本（粗体、列表、链接、代码块）。
3. A → B：普通附件，包括中文文件名。
4. A → B：正文内联 PNG/JPEG。
5. A → B：同时带内联图和普通附件。
6. A → B，B 放入 Cc，另一个地址放入 Bcc；确认 B 的邮件头没有 Bcc。
7. 在 B 中回复 A，再在 A 中回复全部，检查主题和线程显示。
8. 转发一封带正文的邮件，确认原正文被引用。
9. 点击邮件中的 `mailto:`，确认打开 wvmail 写信窗口而不是系统客户端。
10. 断网后发送，确认窗口保留、草稿仍可重试。
11. 发送成功后检查 A 的 Sent 文件夹；模拟 APPEND 失败时确认仍提示发送成功及 warning。

