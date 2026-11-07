# Gmail / Outlook 在线验证（OAuth）可行性调研

> **Archived research.** The phase-one implementation described below has
> landed; use `services/oauthservice.go` and the account settings flow as the
> current reference.

> 2026-10-18 调研。目标：在现有「手填 IMAP/SMTP」之外，增加网页授权式添加账户，覆盖 Gmail、Outlook.com（个人）、Microsoft 365（组织），并回答「Exchange 怎么办」。
>
> **状态更新（同日）：阶段一已实现。** 见文末「已实现」一节。

## 结论先行

| 提供商 | 推荐方案 | 可行性 | 门槛 |
|---|---|---|---|
| Outlook.com / Microsoft 365 | OAuth2 + XOAUTH2 (IMAP/SMTP) | ✅ 必须做 | 低（Azure 免费注册，用户自同意） |
| Gmail | 同一套 OAuth/XOAUTH2 | ✅ 技术可行 | 发布需 Google 审核 + CASA 安全评估；过渡期用 App Password |
| Exchange 自建（on-prem） | 保持手动 IMAP/SMTP | ✅ 现状已支持 | 无（EWS/Graph 不建议做） |

关键事实（2026-10 核实）：
- **微软官方文档**已把 Outlook.com 的 IMAP/SMTP 认证方法标为 **OAuth2/Modern Auth**；Microsoft Q&A（2026-08）确认 basic auth 的 SMTP 已被拒收。**对 Outlook，OAuth 不是可选项而是唯一正道**（应用密码路线在新安全策略下也不可靠）。
- **Google**：`https://mail.google.com/` 是 restricted scope，应用发布到 production 需要 verification + 年度 CASA 安全评估；未验证应用（testing 模式）限 100 名测试用户，且 **refresh token 7 天过期**（每要重新登录）。Gmail 的应用专用密码（需 2FA）目前仍可用，可作为过渡。
- 同类开源客户端 Pebble（Tauri）已走通这条路：Outlook 用 OAuth + Graph API 收发信；其 issue #78 的教训是 **Graph `/me` 取邮箱字段必须 `mail` > `userPrincipalName` > `email`**（微软允许 QQ 邮箱作为登录名，`email` 可能不是收件地址）。

## 方案对比

### A. OAuth2 + XOAUTH2（推荐，一套机制通吃两家）

IMAP/SMTP 层不用改协议逻辑，只是把「密码」换成「访问令牌」：
- IMAP：SASL `XOAUTH2`，payload 为 `base64(user=<邮箱>\x01auth=Bearer <token>\x01\x01)`（`\x01` 为 Control+A / SOH）。go-imap v2 的 `imapclient.Client.Authenticate` 会处理 Base64 编码。
- SMTP：实现 `smtp.Auth` 的 XOAUTH2 变体，同样简单。
- OAuth 流程（桌面端三选一）：
  1. **Authorization Code + PKCE + Loopback (RFC 8252)** ✅ 推荐：打开系统浏览器授权，应用内起临时 HTTP server 接 code。Google 使用 `http://127.0.0.1:<随机端口>/callback`；Microsoft 使用 `http://localhost:<随机端口>/callback`，因为 Entra 仅对 `localhost` 忽略动态端口，且路径仍需匹配注册值。
  2. Device Code Flow：Microsoft 完全支持（可做兜底）；Google 的设备码面向受限输入设备、scope 受限，不推荐。
  3. 内嵌 WebView：**Google 明确禁止**，不可行。
- 令牌端点：
  - Google: `https://oauth2.googleapis.com/token`，scope `https://mail.google.com/`（IMAP+SMTP 共用）。
  - Microsoft: `https://login.microsoftonline.com/common/oauth2/v2.0/token`。授权请求收集 `offline_access`、Graph `User.Read`、Outlook `IMAP.AccessAsUser.All` 和 `SMTP.Send`；兑换授权码时显式只取 Graph `User.Read`（用于 `/me`），刷新邮件访问令牌时显式只取 Outlook IMAP/SMTP scopes，因为 access token 按资源受众隔离。`common` tenant 同时覆盖个人与组织账户。
- Token 刷新：`golang.org/x/oauth2` 的 `TokenSource` 自动刷新；refresh token 存 config，复用现有 AES-256-GCM 加密（`imap_pass` 同款）。

### B. Microsoft Graph API（Pebble 的路线）

用 REST 拉邮件代替 IMAP。优点是现代、不依赖 IMAP 开关；缺点是**要重写整个收发信/缓存层**，与本项目已有的 IMAP 管线（分页、缓存、解码、附件）完全重复。**不建议**；只用 Graph 的 `GET /v1.0/me?$select=mail,userPrincipalName` 做授权后取用户邮箱这一小步。

### C. 应用专用密码（App Password）

Gmail 可用（需先开 2FA，myaccount.google.com/apppasswords 生成 16 位密码，填进现有表单即可，零开发量、只需文档引导 + 预设提示文案）。Microsoft 个人账户的应用密码在新认证策略下不可依赖。

### D. Exchange 自建

自建 Exchange 通常仍开 IMAP（或用户自己有凭据），现有手动表单已覆盖。EWS 协议工作量大、受众小，不做。

## 发布门槛（决定节奏的差异点）

- **Microsoft**：Azure 应用注册免费；选「个人 + 组织」multi-tenant；IMAP/SMTP scope 属委托权限，**用户自己点同意即可，无需微软审核**。refresh token 90 天滚动续期（在用就不过期）。国内访问 login.microsoftonline.com 一般可达；世纪互联租户（login.partner.microsoftonline.cn）endpoint 不同，可后续加。
- **Google**：这是唯一真正的门槛。路线：
  1. 过渡（推荐首发）：UI 里 Gmail/Outlook 都给「在线验证」，Gmail 另配「应用专用密码」引导文档；
  2. 进阶：允许高级用户填自己的 OAuth client id（自建 Google Cloud 项目，无审核负担）；
  3. 长期：做正式 verification + CASA（年审 + 费用），把内置 client id 发布给所有人。
- 中国网络访问 Google 需代理，属用户环境问题，UI 上给出明确报错提示即可。

## 落地设计（对应现有代码）

### Go 侧（services/，均为新文件或小改）
- 新增 `services/oauthservice.go`：
  - `BeginOAuth(provider)` → 生成 PKCE + state，起 loopback listener，返回浏览器 URL（`OsService.OpenURL` 打开）；
  - 等回调 → 换 token → （Microsoft：Graph `/me`，注意 `mail` 优先级；Google：oauth2 userinfo）→ 返回 {email, provider}；
  - `RefreshToken` 托管：提供 `TokenSource(account)` 给 IMAP/SMTP 用。
- `configservice.go`：`EmailAccount` 增加 `AuthType string`（`password` | `oauth2`）、`OAuthProvider`、`OAuthRefreshToken`（AES-GCM 加密，复用 `imap_pass` 机制）。
- `imapdial.go`：`AuthType == oauth2` 时取 access token，`imapclient.Options` 注入 `sasl.NewXoauth2Client(user, token)`。
- SMTP 发信路径：XOAUTH2 `smtp.Auth`。

### 前端（AccountForm.vue 双 tab）
- 顶部两个 tab：**「服务器配置」**（现「手动」的更名建议；英文 Manual → "Server settings"）与**「在线验证」**。
- 「在线验证」流程：选 Google / Microsoft → 打开系统浏览器 → 界面显示"等待授权完成…" → 回调成功自动填入 email + provider + token → 常规「测试连接」→ 保存。
- provider 预设里 Gmail/Outlook 自动切换到「在线验证」tab（检测到 `gmail.com`/`outlook.com` 等）；App Password 的引导文案放在 Gmail 的手动表单提示里。

### 阶段划分
1. **阶段一**：Microsoft OAuth 全链路 + 双 tab UI + token 加密存储（约 1-2 天）。
2. **阶段二**：Gmail 复用同一链路（client id 配置 + App Password 引导文案 + 报错提示）。
3. **阶段三（可选）**：Google 正式 verification/CASA、世纪互联 endpoint、Device Code 兜底。

## 参考链接
- https://github.com/QingJ01/Pebble/issues/78 （Outlook OAuth userinfo 邮箱字段优先级坑）
- https://github.com/WRZ1014/Pebble_Misc/blob/main/Gmail.md （Gmail App Password 图文流程）
- Microsoft 官方 POP/IMAP/SMTP 设置（认证=OAuth2）：support.microsoft.com/office/pop-imap-and-smtp-settings-8361e398-8af4-4e97-b147-6c6c4ac95353
- Google 受限范围验证：developers.google.cn/identity/protocols/oauth2/production-readiness/restricted-scope-verification
- Gmail API scopes（restricted 定义）：developers.google.com/gmail/api/auth/scopes

## 已实现（2026-10-18，阶段一）

- `services/oauthservice.go`：PKCE(S256)+state、loopback 回调 server（Google 使用 127.0.0.1，Microsoft 使用 localhost，动态端口，5 分钟超时）、token 交换（同步在回调内完成，避免竞态）、Graph/Google userinfo 取邮箱（Microsoft 按 `mail` > `userPrincipalName` > `email`）、`oauthAccessToken` 刷新 + 内存缓存、`xoauth2Client` SASL。
- 绑定方法：`OAuthService.BeginOAuth(provider, clientID)` / `PollOAuth(state)` / `CancelOAuth(state)`（`main.go` 已注册）。
- `EmailAccount` 新字段：`AuthType`("password"|"oauth2")、`OAuthProvider`、`OAuthClientID`、`OAuthRefreshToken`（AES-GCM 加密落盘）；`dialIMAP` 在 oauth2 模式走 `client.Authenticate(xoauth2)`。XOAUTH2 payload 需用 SOH(`\x01`)分隔，并在 token 后附两个 SOH；误用 NUL(`\x00`) 会令 Outlook IMAP 返回 `BAD Command Argument Error. 12`，现已修正。
- 前端 `AccountForm.vue` 双 tab：「服务器配置」（原手动）与「在线验证」（选 Google/Microsoft → 可选 Client ID → 打开浏览器 → 1.5s 轮询 → 自动填表）；编辑 oauth 账户默认落在在线验证 tab；Gmail 手动表单显示 App Password 提示；`Account.vue` 校验按 AuthType 分支。
- 测试：`services/oauthservice_test.go` 覆盖 httptest 全流程、拒绝路径、刷新缓存、Microsoft localhost 回调可达性、内置 Client ID 和 Graph/Outlook token 受众分离；单测用 overrideProvider 把 token/userinfo 指到 httptest。
- Microsoft 已内置公开 Client ID `63884bde-31f3-4d9b-854d-097342c1d1c1`，普通用户无需自行填写；Google Client ID 仍为空，发布前需注册 Google Cloud 应用，Gmail 生产化需 verification+CASA。
