---
name: real-time-sync
description: 实时收信（IMAP IDLE）的实现：连接生命周期、心跳与看门狗、EXISTS 去重、与兜底轮询的分工，以及 QQ/163 的连接数约束
type: project
---

# 实时收信：IMAP IDLE

对应代码：`services/emailidle.go`（watcher）、`services/emailsyncservice.go`（兜底轮询 + `OnMailArrived`）、`services/emailservice.go` `fetchEmailsOn`（复用连接的抓取）、`services/imapdial.go` `dialIMAPWith`（装 `UnilateralDataHandler`）。

> 行号会漂移，**函数名才是稳定锚点**。

---

## 1. 为什么不用轮询

轮询每 tick 都要：拨号 → TLS → 登录 → `SELECT` → `FETCH` 一页，**不管有没有新邮件**。IDLE 把这件事反过来：一条连接开着不动，服务器在邮件落库的瞬间推 `* N EXISTS`。结果是新邮件约 1 秒内到界面，而且空闲期**零命令**。

代价是那条连接得活着。睡眠唤醒、Wi-Fi 切换、服务商 idle 超时都会静默杀掉它，所以真正的工程量不在 IDLE 本身，而在**发现它死了并重连**。

## 2. 触发条件

只对 `SyncInterval > 0` 的账号开 watcher（也就是设置里「自动同步」打开的账号）。`reconcile` 每 30s 重读一次配置，负责三件事：开新账号、关掉关掉的账号、**连接参数变了就重连**（`fingerprint` 覆盖 host/port/user/auth_type/password/oauth_refresh_token/use_tls；改头像、改显示名、改间隔都不重连）。

**删除账号不等这个 30s tick**：`ConfigService.RemoveAccount` 写盘之后回调 `EmailIdleService.StopWatching`（在 `main.go` 里用 `SetAccountRemovedListener` 接线），立刻关掉那条连接。否则被删的账号在窗口期内收到新邮件仍会被抓取、写缓存、发 `emails-synced`，服务端连接槽也一直占着。同一个回调顺带 `EmailSyncService.ForgetAccount` 清掉 `lastSync` 和托盘徽标里该账号的未读数（`unreadTracker.drop`），不用等 `syncLoop` 的 `retain`。

启动后有 5s 延迟（`idleStartDelay`）才开第一条 IDLE：启动瞬间托盘 seed（`checkUnreadOnce`）和侧边栏徽标（`FolderStatuses`）已经各拨一次号，再加上 IDLE 就是每账号 3 条并发连接，QQ/163 会数这个数。延迟期间到达的邮件由启动那次抓取覆盖。

## 3. 一条连接的生命周期

```
dialIMAPWith(handler)  ← handler 必须在拨号时装，之后无法附加
  → CAPABILITY          没有 IDLE 就 errIdleUnsupported → 该账号本会话退回轮询
  → SELECT INBOX (只读)  拿到 NumMessages 作为基线 total
  → IDLE
      ├─ EXISTS/EXPUNGE → debounce 1.5s → DONE → SELECT+FETCH(page 1) → 回 IDLE
      ├─ 心跳 3min      → DONE → NOOP(30s 超时) → 回 IDLE
      ├─ Client.Closed  → 返回错误 → 退避重连
      └─ watcher stop   → DONE → 返回 nil
```

**复用同一条连接做抓取**（`fetchEmailsOn`）而不是新拨一条：IDLE 期间不允许发其它命令，所以顺序必须是 DONE → 抓 → 再 IDLE；但换连接就意味着每来一封新邮件多一条会话，正好抵消 IDLE 的意义。

## 4. 四个容易踩的坑

| 坑 | 症状 | 处理 |
|---|---|---|
| **重复 EXISTS** | 有些服务器在 IDLE 重启时重发当前 `EXISTS`，若按「收到 EXISTS 就抓」会变成抓取死循环 | 维护 `total`（SELECT 的 `NumMessages`，每次抓取后更新为 `resp.Total`），**只有计数变化才抓** |
| **睡眠唤醒** | 合盖后 socket 在内核里看着还是开的，`Client.Closed` 不触发，服务器那边早已断开 → 邮件再也不来 | 3 分钟心跳：DONE → `NOOP`（30s 超时，超时即 `client.Close()` 解阻塞）→ 重连 |
| **抓取卡死** | 连接在命令中途死掉时，那次读没有任何东西能打断 | `idleFetchTimeout` 90s，超时关 socket |
| **多连接** | 每次重连都新开会话，QQ/163 有并发与频率限制 | 指数退避 30s→1m→2m→4m→8m→15m（封顶），成功建立 IDLE 后重置为 30s |

`errIdleUnsupported` 只认 `NO`/`BAD`（`isIdleUnsupported`）。超时、断连、`BYE` 都算瞬时故障，必须重试——把一次 Wi-Fi 抖动当成「服务器不支持 IDLE」会让这个账号整个会话都退回轮询。

## 5. 与轮询的分工

`EmailSyncService.syncLoop` 每个 tick 对每个账号先问 `idle.Watching(accountID)`：

- **true**（连接正在 idle）→ 跳过。IDLE 已经在做这件事，再轮询只是白花连接。
- **false**（没开自动同步、服务器不支持、或正在退避重连）→ 照旧按 `SyncInterval` 轮询。

所以 `SyncInterval` 现在的语义是「**兜底轮询间隔**」，不是「多久收一次信」。设置页在开关打开时会显示这条说明（i18n key `settings.accounts.sync_hint`）。

`OnMailArrived` 只做两件事：记 `lastSync`、发 `emails-synced` 事件。**不重新抓取**——watcher 已经在自己的连接上抓完并写缓存了；未读数也是那次抓取顺带 `STATUS` 出来的（`reportUnseen`），不需要额外命令。

## 6. 前端如何响应

`emails-synced` 到达后（`stores/accounts.ts`）：节流刷新该账号的未读徽标；若事件账号正好是当前账号，按缓存重读当前页（`ListEmails`，不打网络）。

## 7. 已知限制

1. **只有 INBOX**。其它文件夹靠「打开即刷新」（见 `email-fetch-and-display.md` §4）。给每个文件夹都开 IDLE 意味着每账号 N 条常驻连接，国内服务商不接受。
2. **纯 flag 变化不触发**（`data.NumMessages == nil` 直接 return）。在手机上读了一封，这边的列表要等下次打开文件夹或下一次抓取才更新。要做的话得区分「自己 STORE 引起的」和「别处引起的」，否则会和 `MarkEmailRead` 打架。
3. **没有 `IDLE` 的服务器**退回轮询，行为与改造前一致。
4. **离线判定不参与**：`NetworkService.ProbeConnection` 的结果目前不影响 watcher，靠退避兜住。接进来可以少几次无效重连。
5. **删除账号默认不清理本地缓存**：`RemoveAccount` 只改 `config.yaml`，`CachedEmail` / `CachedEmailBody` / `CachedFolder` / `CachedFolderMeta` 里该 `account_id` 的行会留在库里。缓存按 `account_id` 分区，重新添加同一个邮箱会拿到新 UUID，旧行再也读不到（但一直占着磁盘）。所以删除弹窗带一个「同时删除本地缓存的邮件」勾选项，勾上才调 `EmailService.PurgeAccountCache` → `DBManager.DeleteAccountCache`（一个事务删这四张表）。默认不勾：缓存是离线可读的前提，不该被顺手丢掉。
