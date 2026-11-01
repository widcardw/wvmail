---
name: email-fetch-and-display
description: How wvmail fetches mail over IMAP and renders it — every fallback/degradation path, the cache rules, the security boundary, and the known gaps
type: project
---

# 邮件获取与显示：链路与全部 fallback

本文记录「取信（IMAP）」和「显示（Vue）」两条链路上所有的**回退/降级方式**，以及哪些是刻意设计、哪些是可疑之处。

发信、撰写窗口、回复/转发和 `mailto:` 是独立链路，见
`.ai-docs/compose-and-sending.md`。

> 行号会漂移，**函数名 / 组件名才是稳定锚点**。撰写时基线：Wails `v3.0.0-beta.25`，macOS 26。

---

## 1. 总览

```
打开某文件夹                   打开某封邮件
  │                              │
ListEmails（缓存优先）          FetchEmail（缓存优先）
  │  miss                        │  miss
  └─→ RefreshEmails ──┐          └─→ RefreshEmailBody ──┐
                      │                                 │
              dialIMAP（连接阶梯 + ID + 登录看门狗）   同一套 dialIMAP
                      │                                 │
        SELECT → 重试0封 → ENVELOPE失败重取    BodyStructure 遍历 → 抓正文/inline
              → 日期/主题/发件人兜底            → CTE 解码 → 字符集兜底 → CID 替换
                      │                                 │
                  写缓存（best-effort）              写缓存（best-effort）
                      │                                 │
              前端 store（失败保留旧数据）       EmailDetail → sanitize → Shadow DOM
```

---

## 2. 获取层（Go / `services/email*.go`、`imapdial.go`）

### 2.1 入口一览

| 方法 | 缓存优先？ | 超时 | 说明 |
|---|---|---|---|
| `TestConnection` | 否 | **15s** | 只做连接+登录，用于设置页「测试连接」 |
| `ListFolders` | **是**（缓存非空即返回，**无 TTL**） | 60s（走 Refresh） | 只有缓存为空才 `RefreshFolders` |
| `RefreshFolders` | 否 | 60s | `LIST "" *`，成功后写缓存 |
| `ListEmails` | **是**（仅 page ≤ 1） | 60s（走 Refresh） | 缓存缺 folder meta 或 total==0 时强制刷新；命中缓存的返回带 `from_cache: true`，前端据此决定「刚打开的文件夹还要不要再刷一次」 |
| `RefreshEmails` | 否 | 60s | 抓摘要页 + 合并缓存 + 校验缓存 UID 是否仍存在 |
| `SearchEmails` | **仅缓存** | — | `db.SearchEmails`，**没有 IMAP SEARCH 兜底** |
| `FetchEmail` | **是** | — | 命中 `LoadEmailBody` 直接返回 |
| `RefreshEmailBody` | 否 | 60s | 两趟抓取：先 BodyStructure，再正文/inline 段 |
| `DownloadAttachment` | 否 | 60s | 独立连接，只抓指定 part |
| `UnseenCount` | 否 | 60s | 单个文件夹的 `STATUS UNSEEN`（托盘红点用） |
| `FolderStatuses` | **否（必须问服务器）** | 60s | 一次连接、**流水线**发出全部 `STATUS MESSAGES UIDNEXT UNSEEN`，返回 `map[文件夹]{messages,uid_next,unseen}`；既供侧边栏 badge，也供「打开的文件夹到底变没变」判断 |

**缓存策略要点**：`SaveEmails` 按 UID 合并、不删旧行，所以翻到更早的页不会丢掉已缓存的邮件；`RefreshEmails` 的返回页可能来自「服务器新抓的页 + 缓存旧行」合并结果（page 1 总是如此）。合并前会用一次 `UID SEARCH` 只查询**已缓存 UID**，把服务器已经不存在的摘要和正文删掉；否则网页版/其他客户端删除邮件后，本地旧 UID 会永远留在列表里，甚至点击删除时 `MOVE` 失败。

**未读计数不能来自缓存**：`cached_emails.is_read` 只覆盖 UI 已经翻到过的那些页，用它统计会让任何大于一页的邮箱**少算**，所以侧边栏 badge 走 `FolderStatuses`（服务器 `STATUS`）。成本控制：只连一次、把所有 `STATUS` 先发出去再逐个 `Wait`（go-imap 在同一连接上流水线化命令，见其 `Client` 文档），因此整个文件夹列表是**一个 RTT**而不是每文件夹一个。`\Noselect`/`\NonExistent` 的容器文件夹跳过（`STATUS` 对它们是报错而不是 0），服务器拒绝回答的文件夹**不出现在 map 里** —— 前端把「缺键」读作「没有要 badge 的」。

**`UIDNEXT` 是「有没有新邮件」的判据**：同一次 STATUS 顺带取 `MESSAGES` 与 `UIDNEXT`，与「上一次真正抓取时看到的值」比较即可判断邮箱有没有动过 —— 只看 `UNSEEN` 不行（三个常年未读的订阅邮件会让它永远等于 3），而 `UIDNEXT` 单调递增，任何新邮件都会推高它。上一次抓取的值来自 `EmailPageResponse.uid_next`，并且**随文件夹 meta 一起落库**（`CachedFolderMeta.UIDNext`），所以重启后第一次打开文件夹也不必盲抓。见 §4 最后一行。

触发时机（前端 `stores/accounts.ts`）：文件夹列表加载后、切换文件夹时、后台 `emails-synced` 事件到达时；每个账号 **30s 内只刷新一次**并有 in-flight 去重（「打开文件夹要不要刷」这一个判断用的是 5s 容忍度，同一个 sweep 复用），避免同步 tick 与点击堆叠成多个 IMAP 连接。打开一封未读邮件时**乐观地**把该文件夹计数 −1（失败回滚 +1）；只在「这封邮件确实来自当前已加载页且未读」时这么做 —— 搜索/聚合视图里点开的邮件不在 `emailsByFolder` 里，读状态未知，不猜，等下一次刷新。

### 2.2 连接阶段：`dialIMAP` 的 fallback 阶梯

1. **加密方式**
   - `UseTLS || port == 993` → 隐式 TLS（`tls.Client` + `Handshake`）。
   - 否则 → 先试 STARTTLS（`imapclient.NewStartTLS`，机会性升级）；**失败则重新拨号走明文**（`imapclient.New`），并记 `warn`。
   - 注意：该明文回退只对「用户自己配置为未启用 TLS 的非 993 端口」账号生效，但一旦回退，**密码是明文发送**的。
   - 这三条不再调用库的 `DialTLS`/`DialStartTLS`/`DialInsecure`：`imapclient.Options.Dialer` 是具体的 `*net.Dialer`，塞不进代理拨号器，所以 `dialIMAPWith` 自己拨 TCP 再调 `imapclient.New`。代价是 TLS 握手改为受 `timeout` 约束（原来只约束 TCP 连接），详见 `.ai-docs/proxy.md`。
2. **RFC 2971 ID 命令**：`providerNeedsIDCommand` 判断（provider 标签 `163`/`126`/`qq`，**或主机后缀** `qq.com`/`163.com`/`126.com`，覆盖 `imap.exmail.qq.com` 这类 `provider: custom` 的企业邮）。
   - 需要时：`WaitGreeting()` → `ID`。
   - **ID 被拒不再致命**：只记 warn 后继续 LOGIN，让真正的失败由登录阶段报出（原先直接失败，报错是难懂的 `id command failed: BAD`）。
3. **登录看门狗**：`Login` 放在 goroutine，外面 `select` + `time.After(timeout)`；超时则 `client.Close()` 并返回 `login timed out after ...`。
4. **登录后补 `CAPABILITY`**（部分服务端如 163 要求），失败即判为连接失败。
5. **库自带的 30s 响应读超时**（go-imap 在每次读响应前设置）——所以「协议层卡住」通常在 ~30s 结束，而 **60s 只可能来自 TCP+TLS 握手**（`dialIMAPWith` 在握手前后各设/清一次 deadline，用调用方的 `timeout`）。
6. **错误翻译**：`wrapIMAPError` 把 `Unsafe Login`、`authentication failed` 等翻译成带排查提示的中文/英文文案。
7. **分阶段日志**：`phase=connect|greeting|login|capability` + `elapsed`，失败用 `slog.Error`，成功用 `slog.Debug`。

普通邮件操作统一从 `connectIMAP` 进入，它负责把拨号失败包成稳定的
`connection failed` 文案；`selectMailbox` 则统一 SELECT/EXAMINE 与
`select folder failed` 的错误翻译。新增邮件操作不要再复制
`dialIMAP` + `client.Select(...).Wait()` 这两段。

### 2.3 列表/摘要阶段：`RefreshEmails` 的兜底

| # | 触发 | 回退行为 |
|---|---|---|
| 1 | QQ/Foxmail 刚变更后瞬时 `NumMessages==0` | 等 300ms 后**重试 SELECT 一次** |
| 2 | 重试后仍为 0，且此前缓存过非零总数 | **返回错误**「keeping previous list」——故意不返回空列表，让上层保留旧数据 |
| 3 | ENVELOPE 解析失败（腾讯 WeCom 等畸形响应） | **去掉 Envelope 重取一次**，主题/发件人/日期改用默认值 |
| 4 | `Envelope.Date` 为空、早于 1970 年，或比当前时间超前 24 小时以上 | 退回 `InternalDate`；两者都不可信则空字符串 |
| 5 | 主题为空 | `"(No Subject)"`（硬编码英文，未走 i18n） |
| 6 | 发件人为空 | `name <addr>` → `addr` → 空 |
| 7 | 缓存写入失败 | `slog.Warn`，**不影响返回**（缓存是 best-effort） |

### 2.3b 列表排序：`date` 是字符串，必须解析成时刻再比

`cached_emails.date` 存的是 RFC3339 **带各自时区偏移**的字符串（`2026-10-24T21:32:01+08:00`、`2026-10-24T14:51:46Z`、`2026-10-24T09:03:25-04:00` 混在同一列里）。原先三个查询用 `ORDER BY date DESC`，比的是**钟面**而不是时刻：

> 真实病例：青云（`qingyuntop@126.com`）21:32 发的登录提醒（= 13:32 UTC）排在两条 22:49 / 22:51 本地时间的 Jira 通知（= 14:49 / 14:51 UTC）**上面**，因为 `"21:32" > "14:49"`。它在列表顶端赖了整整一天——直到第二天日期前缀变大才被压下去。这和「有没有别的客户端读过这封信」毫无关系（`\Seen` 只影响加粗，不参与排序）。

改法：`services/emailtime.go` 提供 `parseEmailTime` / `sortEmailsNewestFirst`，**排序一律在 Go 里按解析后的 `time.Time` 比较**；SQL 只负责 `ORDER BY uid DESC` 给出确定的基础顺序（同秒的多封通知按 UID 落序，后到者在上）。三处入口：

| 位置 | 用途 |
|---|---|
| `DBManager.LoadEmails` | 文件夹列表（含分页前的全量） |
| `DBManager.SearchEmails` | 搜索结果（同一个列表组件渲染） |
| `DBManager.SuggestEmailContacts` | 发件人补全「最近的优先」，`limit` 取前 N 行，顺序错了就补错人 |

同时两处**写入**改成 `UTC().Format(time.RFC3339)`（`fetchEmailsOn` 与 `RefreshEmailBody`），让新行不再携带偏移、整列趋于同构；老行仍带偏移，读侧排序已能正确处理，所以不做数据回填。附带修掉一个真 bug：`Envelope.Date` 为空时原先会被格式化成 `0001-01-01T00:00:00Z`（非空字符串），导致下面第 4 条的 `InternalDate` 兜底**永远不会触发**。

**畸形未来日期**：真实 Gmail 通知出现过 `Date: Sat, 18 Jan 2612 15:55:30 GMT`，但同一封邮件的 `Received` 与 IMAP `InternalDate` 都是 2026 年。`resolveEmailDate` 现在会校验发件人日期；明显不合理的未来值改取服务器 `InternalDate`。读取旧缓存时 `sanitizeCachedEmailDate` 会先隐藏畸形值，下一次列表刷新会把修正后的日期同时写回摘要和正文缓存。

### 2.4 正文/附件阶段：`RefreshEmailBody` 的兜底

| # | 触发 | 回退行为 |
|---|---|---|
| 1 | BodyStructure 为 nil，或走完没发现任何正文 part / 附件 | **抓整封**（`FetchItemBodySection{Peek:true}`）当纯文本，并走 `decodeBodyPart`（本次修正：原先直接 `string(raw)`，会漏掉 CTE/字符集解码） |
| 1b | `BODYSTRUCTURE` 的叶子 part 没给 disposition / filename / Content-ID | 对这类 metadata 不完整的 part 再取 `BODY[part.MIME]`，从**真实 MIME header** 恢复 `Content-Disposition`、`filename` 和 `Content-Type.name`，再走附件/正文分类（腾讯 QQ / Exmail 会省略这些字段） |
| 2 | 只预取正文 part + inline(CID) 图片 part | 下载型附件**不预取**，点下载时才单独连一次 |
| 3 | base64 解码失败 | 去掉换行/空白前后再试一次；再失败 → **用原始字节** |
| 4 | quoted-printable 解码中途出错（`invalid unescaped byte`） | **保留已解出的部分**（`mime/quotedprintable` 是流式的，出错点之前的内容是好的）；只有一字未解才回退原始字节 |
| 4b | 正文尾部有 NUL 填充字节（`…=20\r\n\x00`） | 解码前先剥掉 NUL —— 它就是上一条里那个「出错点」的常见来源 |
| 5 | 其它 CTE（7bit/8bit/binary）或**根本没声明** CTE | 先按原样用，但**若内容明显是 quoted-printable 就先解码**（见下） |
| 6 | 声明的 charset 未知/转换失败，且字节不是合法 UTF-8 | **依次**：声明 charset → 合法 UTF-8 则原样 → **GB18030 兜底**（本次新增，针对中文邮件常见的 GBK 无 charset 声明） |
| 7 | 附件名为空 | `Content-Disposition.filename` → `Content-Type.name` → 生成 `attachment-N` / `inline-N` |
| 8 | 附件/正文分类 | **`Content-Disposition: attachment` 优先判定**（本次修正：原先 `text/plain`/`text/html` 判定在前，导致带 `.txt`/`.html` 附件的邮件**把附件当正文渲染**、甚至覆盖真实正文）；其后依次是：无 disposition 的 text 正文 → 有文件名的非文本 → `disposition: inline` → 有 `Content-ID` |
| 9 | CID 内联图解码失败 | 用原始字节直接 base64，仍生成 data URI（宁可图错也不要空图） |
| 9b | 内联图片（正文已经用 data URI 显示出来） | **不再进附件列表**（`visibleAttachments`）：正文里已经看得见的东西不需要再当附件列一遍，附件数也随之变准。只对**图片**生效——真实病例：一封已发送邮件里 25 MB 的 `andromeld.log`（`application/octet-stream`）也带 `Content-ID`，按「inline 就不列」会把它藏掉，用户再也下载不到。判据是「inline **且** 是图片 **且** 字节真的被替换进 HTML（`rendered`）」；正文没引用到的内联图仍然列出（否则它在正文和列表里都消失）。 |
| 10 | 缓存写入失败 | `slog.Warn`，不影响返回 |
| 11 | 缓存的正文是「被误标的 quoted-printable」 | 读缓存时发现即解码并**写回缓存**（`repairCachedEncoding`），否则老邮件会一直显示 `=E8=AE=A1` |
| 12 | 命中正文缓存（`FetchEmail` 早退） | `IsSuspicious` 是**派生**值、不落库，必须在缓存路径重算 `isSuspiciousMail(folder, cached.Flags)` —— 否则第一次打开有垃圾邮件门禁，之后每次（含重启后）都悄悄没了 |

### 2.4b 派生字段与缓存的坑（`is_suspicious`）

垃圾邮件门禁是 `EmailDetail.IsSuspicious`，它**不存**在 `cached_email_bodies` 里，规则是「文件夹名含 spam/junk/垃圾 **或** 服务端打了 `\Junk` 标记」（`isSuspiciousMail`）。存进缓存的是 `flags`，所以规则必须在两条路径上都跑：

- 首次打开 → `RefreshEmailBody`（IMAP）→ 算一次；
- 之后打开 → `FetchEmail` 命中缓存 → **也要算一次**。

原先只在 IMAP 路径算，后果是**第一次打开有门禁、第二次就没有了**：正文直接渲染、附件直接列出，只剩「已拦截 N 个远程资源」那条 bar（用户就是这样发现的：SZU 的 Junk 邮件只剩拦截资源提示）。`EmailDetail` 的其它字段都进库了，只有这个派生值没有，属于单点疏漏。

**谁决定要不要拦**：判定（`IsSuspicious`）始终由服务端算，客户端只决定「要不要先折叠」。这个决定是 `Settings → 安全` 的「可疑邮件智能拦截」（配置键 `scan_external_safety`，默认开）：`EmailDetail.vue` 用 `isSpamGated(isSuspicious, intercept, revealed)`（`composables/mail-safety.ts`）算一次 `spamGated`，门禁条 / 正文 / 附件 / 翻译结果 / 两条净化提示条全读它——原先这六处各自重复 `detail.is_suspicious && !suspiciousRevealed`，开关只要漏掉一处就会出现「正文放行了但附件还藏着」。**这个开关此前是死的**：`ScanExternalSafety` 只被写进 YAML、没有任何代码读过，所以门禁实际上是「无条件」的（开关放在 `Settings → 关于` 里，看起来像在工作）。关掉它 = 垃圾邮件直接显示，远程内容开关仍然独立生效。

### 2.5 已知的获取层缺陷（尚未改）

- **`SearchEmails` 只搜本地缓存**：没有 IMAP `SEARCH` 兜底，未缓存过的邮件搜不到，UI 也没说明这一点。
- **缓存没有 TTL**：`ListFolders` 永不刷新；`ListEmails` page 1 一律优先返回缓存。打开文件夹看到的是上次缓存的内容，需要手动刷新。这是**刻意**的（避免每操作一条新连接加剧腾讯侧限流），但用户无感知。
- **`text/*` 命名附件**（有 filename、无 `Content-Disposition: attachment`）仍可能被当正文；本次只修了显式 attachment 的情形。
- **明文回退**（2.2-1）在账号被配置为不用 TLS 时仍会自动发生，仅记 warn。

---

## 3. 显示层（Vue）

**误标 CTE 的判定**（`tryQuotedPrintable`，只对正文，附件不碰）必须同时过两关，避免把正常散文改坏：

1. **便宜的闸门**：正文含 `=3D`（任何被编码的 HTML 都会有），或至少一个软换行 `=\n`，或至少 3 个 `=XX`；
2. **解出来必须像文本**：QP 解码后不允许出现除 `\t`/`\n`/`\r` 之外的控制字符 —— `a=1f b=2c c=3d` 这种散文能过第 1 关，但解出来是单元分隔符，于是被拒。

两道关都过才替换；解码结果照旧走 `decodeCharset` 的三级兜底（QP 解出来是 GBK 字节也能救）。

### 3.1 打开一封邮件的顺序

1. `EmailList.vue` `handleClick` → 解析 account/folder（聚合/搜索视图 vs 普通）→ `accountsStore.selectEmail`。
2. `stores/accounts.ts` `selectEmail`：置 uid、清空 detail、`loadingDetail=true`、清 `detailError` → `EmailService.FetchEmail`。
3. 成功写 `selectedEmailDetail`；失败写 `detailError`（详情保持为空）。
4. `EmailDetail.vue`：loading 转圈 → 错误横幅 → **垃圾邮件门禁横幅** → 详情占位。
5. 门禁：`is_suspicious` 时隐藏 **HTML 正文 + 附件列表**，需点「显示全部」。
6. 头部：`senderDisplay`（name → addr → from → `'Unknown'`）、`formatDate`、主题、to/cc（`RecipientList.vue`，见 §3.1b）。
7. 附件区 `EmailAttachments.vue`。
8. 正文：`body_html` → `sanitizeHtml` → `EmailShadowBody`；否则 `body_text` → `linkifyPlainText` → `EmailShadowBody`；都没有 → `"(No content)"`。
9. `EmailShadowBody.vue`：暗色时 `adaptHtmlForDarkMode` → `shadowRoot.innerHTML = content`。
10. 链接点击拦截 → `EmailLinkDialog` → `OsService.OpenURL`。
11. 可选翻译：`translateHtmlWithLayout`（保留布局）→ 失败则降级为纯文本翻译。

### 3.1b 收件人 / 抄送：超过一行就折叠（`RecipientList.vue`）

企业邮件会带几十上百个地址（邮件列表、工单系统把所有 watcher 都加进 To、reply-all 链条）。原先 `to.join(', ')` 直接铺开，一封这样的邮件能把主题和正文整体顶出屏幕。

**判据是「第一行真的满了」，不是「超过 N 个」**：一个地址可能顶四个短的，同一个列表在窄窗里放得下、宽窗里放不下。所以 `RecipientList.vue` 先在一份**离屏副本**（`.probe`，`visibility:hidden` + `position:absolute; left:-9999px`，仍然是真实布局，否则 `offsetWidth` 为 0）里量出每个地址与分隔符的宽度，再由纯函数 `layoutRecipients(widths, available, chipWidth, separatorWidth)`（`composables/recipients.ts`，有 `tests/recipients.test.mts` 钉住边界）决定第一行能放几个：

- 放得下的地址照常显示，剩下的折成 chip「等 N 个收件人」，点一下展开成全部、chip 变「收起」；
- **最后一个地址不需要给 chip 留位置**——它放得下就说明全都放得下，此时根本不出现 chip；
- chip 的宽度按**总数**（最坏情况）预留，避免「chip 多宽 → 藏几个 → chip 多宽」的循环依赖；
- 单个地址比整行还宽时仍然显示它并靠 `overflow:hidden` 裁掉，而不是渲染一个只剩 chip 的空行；
- 只认**宽度**变化（`ResizeObserver` 里比对 `clientWidth`）：折叠后行高会变，跟着高度重算会自激。

未测量时（首帧、或面板宽度为 0 的隐藏状态）按「全部显示」渲染，量到宽度后立刻收敛——比先画一行 chip 再跳变好。

**密送显示不了**：IMAP `ENVELOPE` 里没有 Bcc 字段（投递前就被服务器剥掉，这是协议设计）。要显示只能解析原始头 `BODY.PEEK[HEADER]`，且只对自己发的、服务器仍保留该头的副本有意义——目前没做。

### 3.2 显示层 fallback 清单（按严重度）

**安全相关**

| 位置 | 触发 | 行为 | 判定 |
|---|---|---|---|
| `EmailDetail.vue` + `EmailTranslatedBody.vue` 翻译分支 | 垃圾邮件 + 点翻译 | **翻译结果原先绕过门禁**直接渲染整段 HTML | 已修（见 §4） |
| `html/sanitize.ts` schema | 始终 | 允许 `<style>` 与**所有元素**的 `style` 属性 → CSS 未净化（`url()`/`@import`/`position:fixed` 都保留） | **已修**（见 §3.4） |
| `html/sanitize.ts` schema | 始终 | 远程 `img src` 的 `http(s)` 保留 → **跟踪像素默认会加载**；没有「拦截远程内容」开关 | **已修**（默认拦截 + 白名单，见 §3.4） |
| `EmailShadowBody.vue` | 暗色转换抛错 | 原先无 try/catch → `innerHTML` 不赋值，正文空白/残留 | 已修 |
| `EmailDetail.vue` 纯文本分支 | 垃圾邮件 | 纯文本正文仍会渲染（安全，但原先与「已拦截 N 个」横幅措辞不符） | 已修（措辞改为准确描述） |
| 纯文本兜底 `linkifyPlainText` | 始终 | 转义正确（`<` → `&#x3C;`，autolink 正则排除引号/尖括号） | 合理 |

**可见性 / 静默失败**

| 位置 | 触发 | 行为 | 判定 |
|---|---|---|---|
| `html/translation.ts` 分隔符数量不匹配 | LLM 改动 `————` | 返回 `[]`/`''` → 原先**静默无反应**，按钮回弹 | 已修（报错提示） |
| `composables/mail-translation.ts` 翻译目标语言缺省 | 配置为空 | 静默用 `zh-CN`；引擎静默用 `llm`；指定 LLM 配置缺失则静默用第一个 | 记录 |
| `html/translation.ts` 无文本节点 | 无可用文本 | 返回 `null` → 静默降级为纯文本翻译（**布局丢失**） | 合理但无提示 |
| `stores/search.ts` 搜索失败 | `SearchEmails` 抛错 | 原先仅 console → 空结果，与「确实没有」无法区分 | 已修（toast） |
| `stores/accounts.ts` 聚合视图失败 | smart folder 抛错 | 原先清空聚合列表 | 已修（保留旧数据 + toast） |
| `stores/accounts.ts` 同步后重取失败 | 后台事件 | 原先 `.catch(() => {})` 完全吞掉 | 已修（记日志，不弹 toast 以免刷屏） |
| `stores/accounts.ts` 取信/取文件夹失败 | 列表加载抛错 | 原先把 `emailsByFolder`/`foldersByAccount` **清空**，与后端「保留旧列表」的意图相反 | 已修（保留旧数据 + toast） |
| `EmailList.vue` 空列表 | `length === 0` | 一律显示「暂无邮件」 | 部分改善（现在至少会弹失败 toast） |
| `EmailList.vue` 聚合邮件缺 account/folder | 点击 | 回落到当前选中账号+文件夹；都没有则**点击无任何反应** | 记录（缺用户反馈） |
| `formats.ts` 日期不可解析 | 脏数据 | 渲染字面量 `"Invalid Date"` | 记录 |
| `formats.ts` 文件大小 | ≥1TB / 负数 / NaN | `"1 undefined"` / `"NaN undefined"`（未 clamp） | 记录 |
| `composables/mail-translation.ts` 翻译无可译内容 | 空正文 | 硬编码英文错误串（未走 i18n） | 记录 |

**重复 / 冗余**

- `formats.ts` 的 `senderDisplay` 与 `EmailDetail.vue` 的 `senderDisplay` 是**两套不同实现**（前者剥掉域名，后者保留完整地址）。
- 主题为空有两处 `"(No Subject)"`；i18n 里没有任何对应 key（另有 7 处硬编码英文兜底串）。
- `EmailDetail.vue` 原先在模板表达式里 `sanitizeHtml(...)`，每次重渲染都重新解析整封 HTML → 本次改为 `computed` 缓存。
- 已删除死代码 `composables/sanitize-html.ts`（`export { sanitizeHtmlRehype as sanitizeHtml }`，全项目 0 引用；真正在用的是 `html/sanitize.ts` / `html/index.ts`）。

### 3.3 DOM 与安全边界

- 全前端**唯一**的 DOM sink 是 `EmailShadowBody.vue` 的 `shadowRoot.innerHTML`（`v-html` 全项目 0 处）。
- 它的三个输入分别是：`sanitizeEmailHtml(body_html)`、`linkifyPlainText(body_text)`、`sanitizeEmailHtml(translatedHtml)` —— 都经过净化或由代码生成。
- 远程内容默认拦截、CSS 已净化（见 §3.4）。
- **CSS 画出来的「图片」**：垃圾邮件会用一个 127 KB 的 `box-shadow`（每个 `x y #color` 就是一个像素）在正文里画图 —— 没有 `<img>`、没有 URL、没有附件，所以远程拦截和 URL 过滤都看不见它（真实样本：`cy@zzbh.org` 那封，128 KB 正文里 127 KB 是这一条声明）。`sanitizeCss` 现在会丢掉**单条超过 2048 字符的声明**（计数为 `EmailSanitizeResult.cssArt`），那封正文因此从 128 KB 降到 2.6 KB、图不再渲染；正常邮件（`box-shadow: 0 1px 2px`）不受影响。丢掉之后正文基本是空的，所以阅读区顶部会显示一条告警条（`EmailCssArtNotice.vue` → `.body-bar`）：「正文用 CSS 画图显示（已拦截），可能是垃圾邮件」—— 否则用户只会看到一片空白，不知道发生过什么。
- 正文里的日期/时间会被标成可点击的 `<span class="date-mention">`（净化**之后**的独立 pass，邮件本身伪造不出这种 chip），点击后弹确认框加入提醒事项 —— 细节见 `mail-date-reminders.md`。
- **颜色主题归我们管**：邮件自带 `@media (prefers-color-scheme: …)` 由 OS 外观触发，与应用主题无关 —— 已在净化时按 `darkMode` 改写成常量，见 §3.5。
- **正文区顶部只有一条 bar 栈**（`styles/ui.css` 的 `.body-bar*`，位于 `EmailShadowBody` 之前）：垃圾邮件门禁 `EmailSecurityNotice.vue`（`title` 是完整说明 + 「显示所有内容」按钮）、`EmailCssArtNotice.vue`、`EmailRemoteNotice.vue` 共用同一套壳，不再各写一份 CSS。标签一律 `white-space: nowrap` —— 否则 CJK 标题会在任意两字之间折行（窄窗时「不安 / 全」）；带按钮的 bar 用 `.body-bar--with-actions`：`flex-wrap` + 标签 `min-width: min-content`，空间不够时**按钮换到第二行**（右对齐），而不是把标题截成省略号。

### 3.4 远程内容拦截与 CSS 净化

**默认行为**：远程内容（`http(s)` 图片、`srcset`、CSS `url()`）默认**拦截**；`data:`（后端已把 `cid:` 内联图解析成 data URI）与相对路径保留。放行有三条途径：

| 粒度 | 入口 | 持久化 |
|---|---|---|
| 全局 | 设置 → 通用 → 远程内容 → 「始终加载…」 | `config.yaml: remote_content.allow_remote_content` |
| 按发件人 | 邮件横幅 → 「总是允许此发件人」 | `config.yaml: remote_content.allowed_senders`（地址小写；`@domain` 匹配整域） |
| 单封邮件 | 邮件横幅 → 「显示远程内容」 | 不持久化，切换邮件即复位 |

**净化管道**（`composables/html/sanitize.ts`）：

```
rehype-parse → rehypeAutoLink → rehype-sanitize(白名单)
   → rehypeSanitizeEmailCss   (重写 <style> 文本与 style 属性)
   → rehypeBlockRemoteContent (删除并计数远程 src/srcSet/poster/background/data)
   → rehype-stringify → 追加受信任的 THEME_LINK_CSS + BLOCKED_IMAGE_CSS
```

CSS 处理是**务实黑名单**（不是完整 CSS 解析器）：永远剥离 `@import`、`expression()`、`behavior`/`-moz-binding`；`position: fixed|sticky` 改写为 `static`；拦截模式下把远程 `url(...)` 替换为 `none`；`prefers-color-scheme` 按当前主题改写为常量（见 §3.5）。`<style>` 与 `style=` 两处都过同一函数。

**计数语义**：`sanitizeEmailHtml()` 返回 `{ html, blockedRemote }`，`blockedRemote` 只统计**拦截模式下真正移除**的远程资源；放行模式恒为 0（否则提示条放行后会再次出现）。它驱动 `EmailRemoteNotice.vue` 的提示条：条上显示短文案「已拦截 N 个远程资源」（`mail.remote.blocked_short`），完整说明（跟踪像素/远程图片/样式）作为 `title` 提示。

**踩过的坑（已加测试）**：hast 里属性名是**驼峰** `srcSet`，只写小写 `srcset` 会漏掉 `<picture><source srcset=…>` —— 实测确认这是一个真实可用的追踪向量。`URL_ATTRS` 因此同时列出两种拼写；`frontend/tests/html-sanitize.test.mts` 固化了这条回归。

**测试**：`cd frontend && pnpm test`（Node ≥ 23 原生剥离 TS 类型，无需新依赖）：`html-sanitize.test.mts` 61 条断言 + `date-mentions.test.mts` 90 条。净化侧覆盖远程拦截/放行两态、`data:` 保留、`<picture><source>` 驼峰属性、`@import` 在两种模式下都剥离、CSS 注释绕过、`position:fixed`、脚本/`onerror`/`</style>` 逃逸、超长声明丢弃、`prefers-color-scheme` 主题改写。Go 侧 `services/configservice_remotecontent_test.go` 验证配置往返与「旧配置默认=拦截」。

**仍然存在（有意保留）**：相对路径 `src`/`url()` 不拦截（只会打到本地资源服务，非追踪）；`class` 属性仍被 schema 剥掉（版式问题，见 §5）；CSS 黑名单挡不住未来新语法（例如某些新函数形式的远程加载），要彻底解决需引入真正的 CSS 解析器做声明级白名单。

### 3.5 主题：邮件自带的 `prefers-color-scheme` 必须听我们的

**症状**：macOS 系统外观是**深色**、应用主题切成 **Light** 时，ChatGPT/OpenAI 的营销邮件（真实样本：收件箱 uid 3780，`noreply@email.openai.com` → `xxx@foxmail.com`）在**日间模式下正文变成白色 → 看不见**；切到 Dark 反而正常。

**成因（已实测确认）**：

1. 那封邮件自带 4 个 `@media (prefers-color-scheme: dark)` 块，里面是 `color:#FFFFFF !important` + `-webkit-text-fill-color:#FFFFFF !important`（以及 `h1,h2,p { color:#ffffff }`）。
2. `prefers-color-scheme` 由引擎按**操作系统外观**求值，**不受**我们写在 `:root`/`[data-theme]` 上的 `color-scheme` 影响 —— 我们设了 `color-scheme: light` 也照样命中。WebKit 自己的 tracker 记着这条（[bug 319500](https://bugs.webkit.org/show_bug.cgi?id=319500)，CSSWG 已决议要改，目前仍是 NEW），原文就是「for any website with an in-page dark mode toggle, the media query is unusable」。
3. 同一批暗色规则里**背景**那一半没能生效：它们打在 `body` / `.wrapperMod` / `.email-shell` / `.content-td` 上，而正文是 `rehype-parse` **fragment** 模式解析的 —— `<body>` 被丢掉、那几个 class 在真实 DOM 里也不存在（实测 0 处）。于是白字留下了、白底没跟上，正好落在邮件内联的 `style="background-color:#FFFFFF"` 表格上。

**所以不是我们注入的 CSS 造成的**：Light 模式我们注入的只有 `.email-auto-link`（主色）和 `.remote-blocked`（`var(--color-text-3)`，浅色下是 #999），没有白色；`html/dark-mode.ts` 的 `* { color: … !important }` 只在暗色路径注入。

**修法**：`sanitizeCss` 把媒体条件本身换成常量 —— `(prefers-color-scheme: dark)` 与当前主题一致时改写为 `(min-width: 0px)`（恒真）、不一致时改写为 `(max-width: 0px)`（恒假，媒体查询按**视口**求值，窗口宽度不可能是 0）。`light` 同理取反；`screen and (…)` 这类复合条件只替换括号那一段，其余保留。于是**主题由 `data-theme` 唯一决定，OS 外观彻底出局**。

- 选项：`sanitizeEmailHtml(html, { darkMode })`，`EmailDetail.vue` 的 `sanitizeOptions()` 传入 `isDarkMode.value`（读它同时也让 `sanitized` computed 在切换主题时重算）。
- 默认值 `darkMode: false` = 走邮件自带的浅色设计：调用方漏传时得到的是**能看清**的那一版，而不是白字。
- 前向兼容：即便 WebKit 以后按决议让 `color-scheme` 影响该媒体查询，改写后的常量仍然正确。
- 暗色路径不变：邮件自带暗色块（如 `table.whats-new-card { background:#2F2F2F }`）会被打开，`adaptHtmlForDarkMode` 继续在其上做兜底适配。

**已知遗留（未改，属暗色路径）**：`-webkit-text-fill-color` 会压过 `color`，而邮件常带 `!important` 的类选择器，`DM_RESET` 里 `* { color: … !important }` 的权重赢不了它 —— 暗色下若某封邮件在**非媒体查询**里写死 `-webkit-text-fill-color:#191919`，那行字仍可能看不见。要修得连带把 `DM_RESET`/`DM_CSS` 的权重整体抬到 ID 级（`:not(#x)` 之类），影响面是所有邮件，故未并入本次。

---

## 4. 本次微调（已改）

| 改动 | 位置 | 原因 |
|---|---|---|
| 附件/正文判定顺序：`Content-Disposition: attachment` 优先 | `services/emailattachments.go` `hasAttachmentDisposition` + 遍历分支 | 原先带 `.txt`/`.html` 附件的邮件会把附件当正文渲染，甚至覆盖真实正文 |
| **从 MIME part header 恢复缺失的附件 metadata** | `services/emailbody.go` `RefreshEmailBody` → `enrichBodyStructureMIMEHeaders`；新 `services/emailmimeheaders.go`；测试 `services/emailmimeheaders_test.go` | 腾讯 Exmail 返回的 `BODYSTRUCTURE` 把 `README.md` 描述成普通的 `text/plain` 叶子，省略了 `Content-Disposition: attachment; filename="README.md"`，于是正文遍历把它当正文候选、附件列表为空。现在对 metadata 不明确的叶子额外取 `BODY[part.MIME]` 并合并真实 header；真实病例重新获取后恢复为 `README.md`（1576 字节）。没有 disposition 的普通正文不会被误标成附件 |
| 字符集三级兜底（声明 → UTF-8 校验 → GB18030） | `decodeCharset`（新） | 原先 charset 缺失/未知时直接原样返回字节中文邮件变乱码 |
| 整封兜底正文也走 `decodeBodyPart` | `RefreshEmailBody` 兜底分支 | 原先 `string(raw)` 漏掉 CTE/字符集解码 |
| **误标 CTE 的自愈解码**：声明 7bit/8bit（或没声明）但正文明显是 quoted-printable 时先解码 | `services/emaildecode.go` `tryQuotedPrintable` | 批量邮件把 QP 内容当 `8bit` 发时界面会直接显示 `style=3D"…"`、`=09`、`=E8=AE=A1`（保险措施，不是下面那条的实际成因） |
| **超长 CSS 声明直接丢弃**（单条 > 2048 字符） | `sanitizeCss`（`dropOversizedDeclarations`） | 垃圾邮件用 `box-shadow` 逐像素画图：无 `<img>`/URL/附件，远程拦截无从下手；正常样式值远小于阈值 |
| **QP 解码不再「全有或全无」**：出错时保留已解出的部分；decode 前剥掉尾部 NUL | `services/emaildecode.go` `decodeBodyPart` | 真实案例（SZU 收的 Microsoft CMT 邮件，uid 274）：part **正确声明**了 `QUOTED-PRINTABLE`，但 section 尾部有一个 `\x00`；`mime/quotedprintable` 在它那里报错，而旧代码 `if err != nil { decoded = raw }` 把已经解好的 9123 字节全丢了，整封正文以未解码状态写进缓存 |
| 老缓存正文同样自愈：读缓存时解码并写回 | `FetchEmail` + `repairCachedEncoding` | 修好抓取路径不够 —— 已经缓存的邮件仍会一直显示转义符 |
| 缓存失败改用 `slog`，删除 4 处调试 `fmt.Println` | `services/emailservice.go`（现拆为 `emailbody.go` 等） | `fmt.Printf/Println` 在 GUI 里无处可见（stdout 未接），且是遗留调试输出 |
| `TestConnection` 记录实际入参（host/user/provider/`has_password`）与耗时 | `services/imapdial.go` | 让「列表测试超时、编辑页测试成功」这类问题可被日志直接判定 |
| 翻译结果纳入垃圾邮件门禁 | `composables/mail-translation.ts`（`start` 早退）+ `EmailDetail.vue` 模板条件 | 原先翻译会绕过门禁把未揭示的正文渲染出来 |
| 翻译返回空内容时报错而非静默 | `composables/mail-translation.ts`（i18n key `mail.translation.empty`） | 分隔符不匹配导致「点了翻译没反应」 |
| 暗色转换失败回退原始 HTML | `EmailShadowBody.vue` try/catch | 原先抛错会让正文空白/残留旧内容 |
| 取信/取文件夹/搜索/聚合失败：保留旧数据 + toast | `stores/accounts.ts` | 原先静默清空，用户误以为「没有邮件」 |
| 取消保存对话框不再报错 | `stores/accounts.ts` `downloadAttachment` | 取消是正常操作，原先会弹错误横幅 |
| **垃圾邮件门禁改为正文区 bar**（从滚动区最顶端的 `.alert--warn` 移到 `EmailShadowBody` 上方，与「已拦截远程资源」同一条栈） | 新 `EmailSecurityNotice.vue` + `EmailDetail.vue`，`.body-bar--with-actions`（`ui.css`） | 顶端那条横幅里 CJK 标题在窄窗会在「安全」中间折行；bar 的 `nowrap` + 按钮换行既解决折行、又和另外两条告警位置统一。顺手删掉已无引用的 `.alert--warn` |
| **内联图片不再出现在附件列表** | `services/emailattachments.go` `visibleAttachments`（`RefreshEmailBody` 结尾 + `FetchEmail` 缓存路径），测试 `emailservice_attachments_test.go` | 正文已经用 data URI 显示过的内容，再列一遍是噪音、还把「N 个附件」的计数撑大。实测缓存里 9 封邮件 60 条附件记录 → 只剩 1 条（就是那个 `.log`），Proton 那几封的 11 个页脚/社交图标全部消失 |
| **`is_suspicious` 在缓存命中路径上重算**（抽出 `isSuspiciousMail`） | `services/emailbody.go` `FetchEmail` + `RefreshEmailBody`，测试 `emailservice_spam_test.go` | 派生值不落库，原先只在 IMAP 路径算：**第一次打开有门禁、之后每次都没有**，垃圾邮件直接渲染正文+附件，只剩拦截资源提示（见 §2.4b） |
| **邮件自带的 `prefers-color-scheme` 改为跟随应用主题**（改写为 `(min-width: 0px)` / `(max-width: 0px)`） | `sanitizeCss` + `alignColorSchemeMediaQueries`，`EmailDetail.vue` 传 `darkMode` | 系统深色 + 应用 Light 时，OpenAI 那封营销邮件（uid 3780）的暗色块照旧命中：`color:#fff !important` 覆盖了它自己的内联白底表格 → 日间模式正文全白看不见。详见 §3.5 |
| 正文净化结果用 `computed` 缓存 | `EmailDetail.vue` | 原先每次重渲染都重新解析整封 HTML |
| 删除死文件 `composables/sanitize-html.ts` | frontend | 0 引用 |
| **默认拦截远程内容**（`src`/`srcSet`/`poster`/`background`/CSS `url()`）并计数 | `html/sanitize.ts` | 跟踪像素原先默认加载 |
| **CSS 净化**：`@import`/`expression()`/`behavior` 永久剥离；拦截模式下远程 `url()` → `none`；`position:fixed\|sticky` → `static` | `html/sanitize.ts` | 原先 `<style>` 与 `style=` 原样放行：可外连、可覆盖应用 UI |
| **放行入口**：全局开关 / 按发件人白名单 / 单封邮件 | `stores/preferences.ts`、`EmailDetail.vue`、`views/settings/Security.vue` + Go `RemoteContentConfig` | 拦截必须有出路，否则用户看不了图 |
| **横幅措辞改为准确描述**，并新增「已拦截 N 个远程资源」横幅与放行按钮 | `EmailDetail.vue` + i18n | 原「已拦截 {count} 个附件/资源」的 count 是「附件数 + 有 HTML ? 1 : 0」，与实际隐藏内容不符 |
| **提示条从正文上方移到附件折叠栏那条带子上**：`EmailRemoteNotice.vue` 与 `.attach-head` 同几何（8px 16px、13px/600、`border-bottom`），底色用新增 token `--color-warning-bg`，左侧图标用 `--color-warning`；警告/错误 alert 也改为复用 token（原先硬编码 `rgba(240,160,32,.14)`/`#f0a020`） | `components/detail/EmailRemoteNotice.vue`（新）、`EmailDetail.vue`、`styles/global.css` | 顶部不再堆叠一块大警告框；拦截提示与附件同为「可操作条」，位置靠近它所影响的正文 |
| **附件列表默认折叠** | `EmailDetail.vue`（`attachmentsOpen = ref(false)`，切邮件时也重置为 `false`） | 长附件列表不再默认占满正文上方 |
| **打开邮件即标记已读**：写服务器 `\Seen`（UID `STORE +FLAGS.SILENT`，读-写 `SELECT`）+ 更新本地缓存 `is_read`，并在同一条连接上重读 `STATUS UNSEEN` | `services/emailfolders.go` `MarkEmailRead`、`services/databaseemail.go` `MarkEmailRead`、`stores/accounts.ts` `markEmailRead` | 之前从不写 `\Seen`：列表的未读样式与托盘徽标永远不会因为在这里读信而更新；失败只回滚列表项，不影响正文展示 |
| **回归测试**：`pnpm test`（26 条断言）+ Go 配置往返/默认值测试 | `frontend/tests/html-sanitize.test.mts`、`services/configservice_remotecontent_test.go` | 安全代码需可重复验证；实测已抓出 `<picture><source srcset>` 驼峰属性漏洞 |
| **打开文件夹即刷新（stale-while-revalidate）**：`selectFolder` 先用缓存画出列表，若这一页来自缓存（`EmailPageResponse.from_cache`）再在后台 `RefreshEmails` 一次，保持当前页、不弹 toast；同一文件夹 20s 内不重复拉、同文件夹同时只有一个请求 | `stores/accounts.ts` `selectFolder` / `revalidateOpenFolder` / `refreshFolder`；`services/emailservice.go` 新增 `FromCache` | 原先点开文件夹只看缓存，新邮件必须手动按刷新按钮。`from_cache` 让「刚从服务器取过的页」不再二次拨号（缓存为空时 `ListEmails` 自己就会走 `RefreshEmails`）。刷新按钮 = 同一函数的 `announce: true` 分支 |
| **打开文件夹不再无条件刷新**：先用一次 `STATUS`（`MESSAGES`/`UIDNEXT`）比对上次抓取时的值，一致就**什么都不做**（不拨号抓列表、刷新图标不转）；只有真变了才 `RefreshEmails`。没有参照点（该文件夹从未抓过、或服务器不返回 `UIDNEXT`）时退回原来的 20s 时间窗 | `stores/accounts.ts` `revalidateOpenFolder` / `fetchSnapshot` / `loadFolderStatuses`；`services/emailfolders.go` `FolderStatuses`（取代 `FolderUnread`）；`services/databaseemail.go` `FolderMeta`（`CachedFolderMeta.UIDNext` 落库） | 上一版「点开就刷」在什么都没变时也要重下一遍 20 封摘要，并让刷新图标每次都转一下 —— 用户能直接看到这个无意义动作。`UNSEEN` 判不了「有没有新邮件」（常年未读的邮件会让它恒定），`UIDNEXT` 单调递增才是可靠信号。刷新图标同时从全局计数改成按 `account:folder` 判断，后台给别的文件夹做的刷新不再让当前列表的头转圈 |
| **列表/搜索/联系人补全改按「时刻」排序**（新增 `services/emailtime.go`，SQL 只保留 `ORDER BY uid DESC`） | `databaseemail.go` `LoadEmails` / `SearchEmails` / `SuggestEmailContacts`，`emailservice.go` `fetchEmailsOn`；测试 `services/emailtime_test.go` | `date` 是带各自时区偏移的字符串，`ORDER BY date DESC` 比的是钟面：青云 21:32(+08:00) 的邮件压在 22:51(Z) 的两条 Jira 通知上面一整天。详见 §2.3b |
| 日期落库统一转 UTC；`Envelope.Date` 为零值时不再格式化成 `0001-01-01T00:00:00Z` | `emailservice.go` `fetchEmailsOn`、`emailbody.go` `RefreshEmailBody`（并补取 `InternalDate`） | 前者让新行同构；后者是**真 bug**：非空的「公元 1 年」让 `InternalDate` 兜底永不触发 |
| **`parseCachedContact` 的 cutset 从 raw string 改成解释字符串** | `services/databaseemail.go` | `` `<> \t\r\n` `` 在 raw 字面量里是**字面**的反斜杠+t/r/n，于是 `n`/`r`/`t`/`\` 都在裁剪集合里 —— `noreply@example.com` 被削成 `oreply@example.com`。测试 `TestParseCachedContactKeepsLeadingLetters` |
| **收件人/抄送超过一行折叠**：「等 N 个收件人」chip 双向切换 | 新 `components/detail/RecipientList.vue` + `composables/recipients.ts`，`EmailHeader.vue` 改用它；i18n `mail.recipients.more` / `.less` | 企业邮件几十个地址原先整段铺开，把主题和正文顶出屏幕。判据是实测第一行能放几个（见 §3.1b），不是固定个数 |
| 抄送标签从硬编码 `"Cc:"` 改为 i18n `search.cc`；`to`/`cc` 为空时整行不渲染 | `EmailHeader.vue` | 英文界面里 `Cc:` 与中文混排；空行只剩一个孤零零的标签 |
| **邮件列表长主题单行省略** | `components/list/EmailListItem.vue` | `.email-subject` 改用 WebKit 单行 clamp 并补齐宽度约束；原先长主题会在列表边缘被硬裁、没有 `…`，还会把右侧日期挤出可视区域 |
| **详情页删除邮件** | `services/emaildelete.go`、`EmailService.DeleteEmail`、`stores/accounts.ts`、`EmailDetail.vue`、`EmailActionBar.vue` | 普通文件夹使用 IMAP `MOVE` 移到垃圾箱；邮件已在垃圾箱时永久删除。删除前先确认 UID 仍在源文件夹；如果已被网页版/其他客户端移走，则只清理本地陈旧缓存并视为成功。成功后清理摘要/正文缓存、星标快照、聚合/搜索列表和角标，并清空阅读区，不自动打开相邻邮件。前端先弹现有 `ConfirmDialog`，删除期间锁定按钮与对话框。`services/emaildelete_test.go` 用内存 IMAP 服务器覆盖移动到 Trash、Trash 内永久删除，以及陈旧 UID 清理 |
| **刷新时清理服务器已删除的缓存行** | `services/emailservice.go` `pruneStaleCachedEmails`、`services/databaseemail.go` `PruneEmails`、测试 `emaildelete_test.go` | `SaveEmails` 仍按 UID upsert 以保留翻页缓存，但刷新后会检查已缓存 UID；网页版删除邮件后，下一次刷新会移除本地旧行，不再出现「网页已删除但列表仍在」 |

---

## 5. 建议的后续项（需产品决策，本次未改）

1. **放开 `class` 属性**：现在 `class` 被 schema 剥掉但 `<style>` 里的 `.class` 规则保留 → 邮件版式半失效，且与暗色模式互相放大。允许 `className` 是成本最低的改善。
2. **搜索兜底**：接 IMAP `SEARCH`，或在搜索框明确提示「仅搜索已缓存邮件」。
3. ~~**缓存 TTL / stale-while-revalidate**：打开文件夹先出缓存、后台再刷新。~~ **已做**（见 §4 最后两行）：先一次 `STATUS` 判断有没有变，变了才抓；20s 时间窗 + 单文件夹 in-flight 去重 + 5s 状态复用。**仍待解决**的是连接数——每次真正抓取都是一条新连接，与已知的腾讯企业邮限流冲突，根治仍需连接复用（长连接 + IDLE，见第 6 项）。
4. **统一发件人显示与文案**：合并两套 `senderDisplay`，把 `(No Subject)`/`(No content)`/`Unknown` 等硬编码兜底串纳入 i18n。
5. **`text/*` 命名附件**：无 disposition 但有 filename 的文本 part 仍会被当正文，可考虑按 filename 存在与否再判一次。
6. **连接复用**：目前「每个操作一条新连接」是限流与握手超时的根因（见 `email-fetch` 相关排查记录）。**已部分解决**：开启自动同步的账号走 IDLE，一条常驻连接负责收信并在其上完成抓取（见 `real-time-sync.md`）；但手动操作（打开文件夹、翻页、取正文、点刷新）仍然各拨一条。
7. **CSS 净化升级为解析器**：当前是务实黑名单（挡 `@import`/`url()`/`expression`/`position:fixed`），无法穷举未来语法。若要更强保证，需引入 CSS 解析器做声明级白名单（代价：新依赖 + 可能改变邮件版式）。
8. **`data:` 图片体积**：内联图允许 `data:`（后端 CID 解析需要），恶意邮件可用超大 base64 撑内存；可加尺寸上限并在超限时降级为附件。
9. **黑名单/发件人白名单的 UI 增强**：目前白名单只能在设置页移除、在邮件里添加；可考虑支持 `@domain` 手工添加与批量清理。

---

## 6. 排查手册

**日志**：`~/Library/Application Support/wvmail/logs/app.log`（slog，10MB 轮转，保留 30 天）。

**关键日志行**

| 关键字 | 含义 |
|---|---|
| `testing IMAP connection` | 设置页测试连接，含 `host/port/user/provider/use_tls/has_password` |
| `imap connect failed (tcp+tls)` `phase=connect` | TCP/TLS 握手失败；`error="tls: DialWithDialer timed out"` = 连上了但握手没完成（限流/链路） |
| `imap greeting failed` `phase=greeting` | 连上但服务器没发 greeting |
| `imap login failed` `phase=login` | 凭据/风控问题（会附带 `Unsafe Login` 等提示） |
| `imap login timed out` | 登录无响应 |
| `envelope fetch failed ... retrying without envelope` | 服务端 ENVELOPE 畸形，已降级重取 |
| `failed to cache ...` | 缓存写失败（不影响功能） |

**超时语义速查**

- **15s** = `TestConnection` 的预算（TCP+TLS+登录整体）。
- **60s** = 其它操作（同样是 TCP+TLS 握手整体，或登录看门狗）。
- **~30s** = go-imap 库对单次响应读取的超时（协议层卡住时先于上面两者触发）。
