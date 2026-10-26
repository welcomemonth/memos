# 授权服务集成到 memos 实施方案

> 版本：2.0（2026-10-25，结构性修订）
> 依据：`授权服务端契约.md` v1.0（已按本次修订更新）、`授权服务端设计.md` v1.1（历史参考）。
> 本文档是把授权服务端**合并进 memos 主项目**的实施蓝图，替代设计文档中“独立 Go 仓库 + Gin + 独立 SQLite + 独立管理页”的方案。
> 客户端代码与契约路径保持不变；服务端把客户端接口挂载在 `/xiaohualu` 前缀下，客户端只需把 `XHL_LICENSE_SERVER` 配置成带前缀的 base URL。

## Context

桌面端软件「小花录」已有独立授权服务端设计，原方案是独立 Go 仓库 + Gin + 独立 SQLite + 独立管理页。用户认为独立服务太分散，改为合并进当前 memos 项目。

本次修订后的核心业务模型：

- **严格一对一**：一张卡密最多绑定一台设备，一台设备同时最多绑定一张卡密。没有 `max_devices` 字段。
- **首次激活锁定到期**：非 `lifetime` 卡密在第一次成功激活时，按 `duration_days` 计算 `expires_at = 激活时间 + duration_days`，之后不再重算；`lifetime` 没有到期时间。
- **卡密首次使用后锁定设备**：客户端 `deactivate` 只删除当前绑定，不释放卡密；同一设备可以重新激活，其他设备需要客服在管理端释放。
- **续期由客服完成**：客户付款后，管理员在管理页手动延长卡密的 `expires_at`；不提供客户端自助续费，也不自动换卡。
- **客户端接口前缀 `/xiaohualu`**：`/xiaohualu/v1/activate` 等；管理接口仍在 `/admin/api/license`。
- **创建结果只在网页展示并复制**：不生成 CSV。
- **更新包保留当前版本与上一版本**，更早的删除。
- **V1 单实例、单密钥**：不考虑多副本和密钥轮换。

## 已确认决策

| 决策项 | 结论 | 影响 |
|---|---|---|
| 卡密与设备 | 严格一对一，删除 `max_devices` | `license_bindings`：`device_id` 主键 + `card_id` 唯一 |
| 有效期 | 首次激活时 `expires_at = now + duration_days`，之后不重算 | 卡密新增 `activated_at`、`expires_at`；客户端解绑/重复激活不重置 |
| 卡密锁定 | 首次激活写入 `bound_device_id`；客户端解绑不清除 | 其他设备在客服释放前一直 `409 device_limit` |
| 续期 | 客服在管理页按天数延长 `expires_at` | 管理端新增 `POST /cards/:id/extend`；refresh 可返回更晚的 exp |
| 更新包 | 保留当前版 + 上一版 | 上传新版本后删除更早的 `.exe` |
| 明文卡密 | 创建响应返回一次，网页复制 | 不生成 CSV；关闭弹窗前必须复制 |
| 客户端路由 | `/xiaohualu/v1/...` | 客户端 `XHL_LICENSE_SERVER` 带 `/xiaohualu`；契约路径不变 |
| 部署 | 单实例、单密钥 | 内存限流、本地更新包目录；不做轮换/多副本 |

## 方案对比

| | 方案 A（推荐） | 方案 B | 方案 C |
|---|---|---|---|
| 客户端契约接口 | 手写 Echo handler，路径/JSON/错误码 100% 按契约 | 仍手写（gateway 错误体不兼容） | 手写 |
| 管理端 API | 手写 Echo JSON + 复用 memos Bearer 认证 | 新增 proto `LicenseAdminService`，走 Connect + gateway | 独立 Gin 服务 |
| 管理界面 | 现有 `Setting` 页新增 admin section | 同 A，但用生成的 TS 客户端 | 独立 React 应用 + go:embed |
| 存储 | memos 主库，现有迁移体系，三库同步 | 同 A | 独立 SQLite |
| 上传安装包 | 原生 multipart，简单 | Connect 传 bytes（base64 膨胀、大文件不友好） | 原生 multipart |
| 与现状契合 | 高：同进程、共用 admin 体系、运维最简单 | 中：多一次 buf 生成和 Connect 适配 | 低：与「不要分散」目标冲突 |
| 结论 | **采用** | 不采用 | 不采用 |

方案 B 只在“管理端 API 自动生成 TS 类型”上有优势，但代价是 proto/gateway/Connect 三处注册、每个 RPC 写 Connect 适配，还要为上传单独开一条原生路由，整体更碎。

---

## 推荐方案 A：详细设计

### 1. 目录与文件

新增：

```
internal/license/
├── config.go        # 读取 XHL_* 环境变量、加载/校验 Ed25519 私钥、派生公钥
├── token.go         # 紧凑令牌签发/验签（base64url(payload).base64url(sig)）
├── cardkey.go       # 卡密生成、规范化（Trim+ToUpper）、SHA-256
├── errors.go        # 契约错误码常量 + 中文 message + HTTP 状态映射
├── service.go       # Activate / Refresh / Deactivate 领域逻辑
├── repo.go          # 卡密/绑定/撤销 SQL（三库占位符与事务差异集中在此）
├── ratelimit.go     # 每 IP 令牌桶
└── update.go        # 更新包落盘、sha256、windows.json 原子读写、版本号校验

server/router/license/
├── service.go       # /xiaohualu/v1/* 客户端契约路由
├── admin.go         # /admin/api/license/* 管理路由 + ADMIN 中间件
└── *_test.go        # httptest 契约回归、错误码、限流、admin 权限

store/migration/{sqlite,mysql,postgres}/0.31/00__license.sql
```

修改：

- `store/migration/{sqlite,mysql,postgres}/LATEST.sql`：追加三张表 DDL
- `server/server.go`：构造 license 服务并注册两组路由
- `server/router/frontend/frontend.go`：`shouldSkipFrontendStatic` 增加 `/xiaohualu`、`/admin`
- `web/vite.config.mts`：开发代理增加 `^/xiaohualu`、`^/admin`
- `web/src/components/Settings/settingSections.ts`：新增 `license` section（scope: admin）
- `web/src/components/Settings/LicenseSection.tsx`（新增）
- `web/src/components/Settings/license/*`（新增：卡密面板、创建弹窗、延长弹窗、绑定/释放弹窗、版本面板）
- `web/src/utils/license-api.ts`（新增，带 Bearer + 401 刷新重试的 fetch 封装）
- `web/src/hooks/useLicenseQueries.ts`（新增）
- `web/src/locales/{en,zh-Hans,zh-Hant,en-GB}.json`：新增 `setting.license.*`
- `.env.example`：补充 `XHL_*` 变量说明

不修改 `proto/`，不影响 MCP/OpenAPI。不新增 `acl_config.go` 条目（手写路由不经过 Connect 鉴权）。

### 2. 数据库

表名加 `license_` 前缀，避免与 memos 表冲突。三套迁移 + LATEST 保持一致。

```sql
CREATE TABLE IF NOT EXISTS license_cards (
    id               VARCHAR(64) PRIMARY KEY,
    key_hash         VARCHAR(64) NOT NULL UNIQUE,
    plan             VARCHAR(16) NOT NULL,
    duration_days    INTEGER     NOT NULL,
    revoked          INTEGER     NOT NULL DEFAULT 0,
    activated_at     VARCHAR(32) NOT NULL DEFAULT '',
    expires_at       VARCHAR(32) NOT NULL DEFAULT '',
    bound_device_id  VARCHAR(64) NOT NULL DEFAULT '',
    created_at       VARCHAR(32) NOT NULL
);

-- 严格一对一：device_id 主键保证一台设备最多一条绑定；
-- card_id 唯一保证一张卡密最多一条绑定。
CREATE TABLE IF NOT EXISTS license_bindings (
    device_id     VARCHAR(64) PRIMARY KEY,
    card_id       VARCHAR(64) NOT NULL UNIQUE,
    activated_at  VARCHAR(32) NOT NULL,
    last_seen_at  VARCHAR(32) NOT NULL,
    token_nonce   VARCHAR(64) NOT NULL
);

CREATE TABLE IF NOT EXISTS license_revocations (
    nonce      VARCHAR(64) PRIMARY KEY,
    reason     VARCHAR(32) NOT NULL,
    revoked_at VARCHAR(32) NOT NULL
);
```

字段语义：

- `duration_days`：卡密的原始有效天数；`lifetime` 为 0。
- `activated_at`：**第一次成功激活**的时间；空字符串 = 从未使用。
- `expires_at`：非 `lifetime` 卡密在首次激活时写入 `activated_at + duration_days*86400`；`lifetime` 为空表示永久。客服延长时只改这个字段。
- `bound_device_id`：首次激活的设备 ID；客户端 `deactivate` **不清除**，只有管理端“释放设备”才清除。它保证卡密被使用后不会漂移到其他设备。
- `license_bindings`：当前活跃绑定。`device_id` 主键 + `card_id` 唯一，两个约束共同保证一对一。
- 时间统一 RFC3339 UTC 文本；对外令牌用 Unix 秒。memos 现有表多用 `BIGINT`，这里按契约用文本时间，`repo.go` 内统一转换。
- `key_hash` 内联 `UNIQUE`，避免 MySQL 不支持 `CREATE INDEX IF NOT EXISTS`；`card_id` 的 `UNIQUE` 同时充当查询索引。
- 迁移目录用 `0.31/00__license.sql`，对应 schema `0.31.1`；LATEST.sql 与增量必须等价。
- 不再有 `max_devices`；`409 device_limit` 的语义保留为“同一张卡密已绑定其他设备”。

`internal/license/repo.go` 通过 `store.GetDriver().GetDB()` 和 `Profile.Driver` 执行 SQL，不扩 `store.Driver` 接口。理由是授权是独立 bounded context、查询少，避免在三套 store 驱动里各加十几个方法；迁移仍走 memos 现有迁移系统。

### 3. 核心流程与并发

**卡密生成**

- 格式 `XHL-XXXX-XXXX-XXXX-XXXX`，16 位取自无歧义字母表 `23456789ABCDEFGHJKMNPQRSTUVWXYZ`。
- 入库前规范化：`strings.ToUpper(strings.TrimSpace(key))` → SHA-256 十六进制；只存哈希。
- 明文只在创建响应中出现一次，不落库、不写日志、不生成 CSV。
- 批量创建在一个事务里完成；`key_hash`/`card_id` 冲突时重试。

**Activate（`POST /xiaohualu/v1/activate`）**

1. 校验 `key`、`deviceId`（32 位小写十六进制），否则 `400 bad_request`。
2. 规范化 + 哈希查卡：不存在 → `401 invalid_key`；撤销 → `403 revoked`。
3. 如果卡密已被使用（`activated_at != ''`）且 `expires_at` 已到 → `403 card_expired`。
4. 事务内按 `device_id` 查当前绑定：
   - **同卡**：幂等。沿用卡密已有的 `expires_at`，不重新计算；更新 `last_seen_at`，签发新 nonce/iat 的令牌。
   - **不同卡**：不在本接口自动换卡，返回 `403 device_mismatch`；客服先解绑旧卡，客户再激活新卡。
   - **无绑定**：
     - 如果 `card.bound_device_id != '' && card.bound_device_id != deviceId` → `409 device_limit`（卡密首次使用时已锁定到另一台设备）。
     - 如果卡密当前已被其他设备绑定（`card_id` 唯一）→ `409 device_limit`。
     - 如果 `activated_at == ''`（从未使用）：写 `activated_at = now`；非 `lifetime` 写 `expires_at = now + duration_days*86400`；`lifetime` 保持空；写 `bound_device_id = deviceId`。
     - 如果 `activated_at != ''` 且 `bound_device_id == deviceId`（客户端解绑后同设备重新激活）：直接复用原 `expires_at`，不重算。
     - 如果 `activated_at != ''` 且 `bound_device_id == ''`（客服已释放旧设备）：写 `bound_device_id = deviceId`，复用原 `expires_at`，不重算。
     - 插入 `license_bindings`。
5. 令牌：`iat = now`；`exp = expires_at 对应的 Unix 秒`（`lifetime` 为 0）；非 `lifetime` 必须满足 `exp > now`，否则在签名前返回 `403 card_expired`。
6. 并发控制：
   - SQLite 用 `BEGIN IMMEDIATE`（`*sql.Conn` 显式开启）串行化写事务。
   - MySQL/Postgres 对 card 行 `SELECT ... FOR UPDATE`。
   - 唯一约束是最终防线：`device_id` 主键冲突 → 重新读取绑定，返回幂等或 `device_mismatch`；`card_id` 唯一冲突 → `409 device_limit`。
7. 事务成功后用 Ed25519 签发令牌，返回 `{token,status:"active"}`。

**Refresh（`POST /xiaohualu/v1/refresh`）**

1. 先验签、校验结构，失败 → `401 invalid_token`。
2. `sub != deviceId` → `403 device_mismatch`。
3. 按 `device_id` 查绑定；不存在 → `403 revoked`。
4. 请求令牌的 `nonce` 在 `license_revocations`，或绑定当前 `token_nonce` 已被撤销 → `403 revoked`。
5. 卡密不存在或 `revoked` → `403 revoked`。
6. 到期判断：
   - 令牌 `exp != 0 && now >= exp` 时，通常返回 `401 token_expired`。
   - **客服续期例外**：如果卡密 `expires_at` 非空且 `now < expires_at`（管理员已经延长），继续签发新令牌，而不是返回 `token_expired`。这样客户不需要重新输入卡密即可恢复。
7. 签发前再确认：非 `lifetime` 卡密的 `expires_at` 必须非空且 `now < expires_at`，否则返回 `403 card_expired`；`lifetime` 的 `exp` 为 0。
8. 新令牌：`plan` 取自卡密；`exp = expires_at` 的 Unix 秒（可能在客服延长后变晚）；`iat = now`；`nonce` 重新生成。更新绑定 `token_nonce`、`last_seen_at`。
9. 正常刷新不改变到期时间；只有管理员延长会改变 `expires_at`。
10. 严格一对一下，刷新只轮换 nonce，**不强制旧令牌失效**：旧令牌在解绑/撤销前仍可通过 Refresh。这是为了避免“服务端已刷新但响应丢失，客户端拿旧令牌重试被永久 block”的可靠性问题。

**Deactivate（`POST /xiaohualu/v1/deactivate`）**

- 验签；`sub != deviceId` → `403 device_mismatch`。
- 事务内按 `device_id` 查绑定：存在则把绑定 `token_nonce`（以及请求令牌 nonce，若不同）写入 `license_revocations`，删除绑定；不存在则直接返回 `{status:"ok"}`（幂等）。
- **不清理卡密的 `bound_device_id`、`activated_at`、`expires_at`**。因此同一设备可以重新激活并沿用原到期时间；其他设备仍然 `409 device_limit`，直到管理端释放。
- 解绑后该设备旧令牌 refresh 必然 `403 revoked`。

**管理端释放设备（`POST /admin/api/license/bindings/unbind`）**

- 请求 `{cardId, deviceId}`，敏感值走 body。
- 事务内：
  - 如果存在 `(cardId, deviceId)` 的当前绑定：删除绑定 + 把绑定 `token_nonce` 写入 `license_revocations`。
  - 如果 `card.bound_device_id == deviceId`：清空 `bound_device_id`，允许卡密绑定到新设备。
  - 如果两者都不匹配：幂等返回 `{status:"ok"}`；如果卡密当前绑定的是另一个设备，返回 `409 device_mismatch`，避免误释放。
- 卡密的 `activated_at`/`expires_at` 不变，新设备只能使用剩余有效期。
- 释放后旧设备如果仍在线并重新激活，可能抢回 `bound_device_id`；客服应先让旧设备停止使用。若无法协调，直接撤销旧卡并给客户发新卡更稳妥。

**客服续期（`POST /admin/api/license/cards/:id/extend`）**

- 请求 `{days}`（1~3650），管理端按“延长天数”操作；UI 可展示计算后的日期。
- 仅允许：非 `lifetime`、未撤销、已激活（`activated_at != ''`）的卡密。
- 计算：`base = max(now, expires_at)`；`newExpiresAt = base + days*86400`；更新 `expires_at`。
- 返回 `{expiresAt}`（RFC3339 UTC）。
- 卡密当前绑定的设备在下一次 refresh 时拿到新的 `exp`；如果旧令牌已经过期，refresh 走上面的“客服续期例外”直接恢复。
- 未激活的卡密不能延长（有效期从首次激活开始算）；如需更长有效期，创建时选择更大的 `duration_days`。

**到期时间的双向记录**

- **服务端权威**：`license_cards.expires_at` 是最终到期时间；`activated_at` 记录首次激活；`license_bindings.last_seen_at` 在每次 activate/refresh 时更新，即“设备使用卡密的时间”。refresh 使用服务端时钟判断，不信任客户端时间。
- **客户端本地记录**：激活成功后客户端把令牌（`exp` 就是卡密到期时间）持久化到 DPAPI，并记录 `last_verified_at`；本地只做展示和 1 小时宽限判断，服务端返回的 `token_expired` / `card_expired` 才是终态。
- **定时校验**：客户端启动时和每日调用 refresh；服务端校验卡密状态、`expires_at` 和绑定后签发新令牌。网络失败时客户端进入 1 小时宽限，超过则本地判为过期；恢复后以服务端结果为准。
- **续期后的恢复**：管理员延长 `expires_at` 后，即使客户端本地已经显示过期，下一次 refresh 只要设备绑定仍然存在，就会拿到新令牌并解除过期状态。

**令牌**

- `payload = {v:1, sub, plan, iat, exp, nonce}`，JSON 序列化后 `base64.RawURLEncoding`。
- `sig = ed25519.Sign(priv, []byte(payloadB64))`，再 RawURLEncoding。
- 客户端校验规则必须对齐：`v == 1`、`sub`/`nonce` 非空、`plan` 白名单且不为 `trial`、`iat > 0`、`lifetime` 的 `exp == 0`、非 `lifetime` 的 `exp > iat`；到期判断用 `now.Unix() >= exp`。
- 私钥加载：
  - `XHL_PRIVATE_KEY`：base64 文本（标准/无填充/URL 编码均可）。
  - `XHL_PRIVATE_KEY_FILE`：OpenSSL PEM（`BEGIN PRIVATE KEY`）或 base64 文本；两者同时配置时文件优先。
  - 32 字节按 seed 处理，用 `ed25519.NewKeyFromSeed` 扩展；64 字节直接作为私钥；其他长度视为配置错误。
  - 两者都未配置：服务正常启动，但 `/xiaohualu/v1/*`（health 除外）返回 `500 internal`；管理页显示“未配置”。
  - 启动时记录错误但不得打印密钥内容。
- 暴露给客户端内置的公钥使用标准 base64（带填充）；管理页 `/status` 可同时返回 base64 与 hex。
- V1 单密钥，不做轮换；客户端内置公钥与部署私钥必须成对。

### 4. 客户端接口（`/xiaohualu` 前缀）

| 方法 | 实际路径 | 契约相对路径 | 说明 |
|---|---|---|---|
| POST | `/xiaohualu/v1/activate` | `/v1/activate` | 激活/同设备幂等 |
| POST | `/xiaohualu/v1/refresh` | `/v1/refresh` | 刷新令牌 |
| POST | `/xiaohualu/v1/deactivate` | `/v1/deactivate` | 解绑当前设备（保留卡密设备锁定） |
| GET | `/xiaohualu/v1/health` | `/v1/health` | `{status:"ok",time:<unix秒>}`，UTC，不限流 |
| GET | `/xiaohualu/v1/update/windows.json` | `/v1/update/windows.json` | 读取更新清单 |
| GET | `/xiaohualu/v1/update/files/:filename` | `/v1/update/files/:filename` | 下载安装包，支持 Range |

- **客户端配置**：`XHL_LICENSE_SERVER=https://<host>/xiaohualu`；客户端仍然追加 `/v1/activate` 等相对路径，所以客户端代码不需要修改。
- 统一响应头 `application/json; charset=utf-8`；错误体固定 `{code,message}`，HTTP 状态按契约第 4 节。未配置私钥统一返回 `500 internal`。
- `GET .../windows.json` 不存在时返回 `404 {code:"not_found"}`；该状态码不在契约表中，客户端按通用错误处理。
- 请求体限制 64 KiB；`/xiaohualu/v1/*`（health 除外）每 IP 10 次/分钟，命中 `429 rate_limited`。限流用**令牌桶**，map 定期清理过期桶。
- `XHL_TRUST_PROXY=true` 时从 `X-Forwarded-For` 取客户端 IP，否则用 `RemoteAddr`；只有确认反代会覆盖 XFF 时才可开启。单实例部署，内存限流即可。
- 日志只记路径、状态码、card id；不记明文卡密、令牌、deviceId，也不记录完整 URL/query string。
- Handler 使用 Echo v5 签名 `func(c *echo.Context) error`（不是 v4 的 `echo.Context` 接口）。

### 5. 管理接口（`/admin/api/license`，仅 ADMIN）

统一前缀 `/admin/api/license`，Echo group 上挂认证中间件：

```go
user, err := authenticator.AuthenticateToUser(ctx, authHeader, cookieHeader)
// user == nil            -> 401 {code:"unauthenticated"}
// user.Role != ADMIN     -> 403 {code:"permission_denied"}
```

`AuthenticateToUser` 的角色来自 `store.GetUser`，比 JWT 的 15 分钟快照新，但 `store.GetUser` 有 10 分钟进程内缓存；同进程改角色会即时失效，多实例/直接改库时最多有 10 分钟延迟。管理接口不要读 JWT 的 `claims.Role`。

| 方法 | 路径 | 请求/响应 |
|---|---|---|
| GET | `/status` | `{configured, publicKey, updateBaseUrl}` |
| GET | `/cards?page=&pageSize=&status=&plan=` | `{cards:[{id,keyHash,plan,durationDays,revoked,activatedAt,expiresAt,boundDeviceId,createdAt,deviceCount}],total,counts:{unused,active,expired,revoked}}`；`deviceCount` 只可能是 0 或 1；`counts` 按当前 `plan` 过滤计算 |
| POST | `/cards/lookup` | `{key}` → 按规范化哈希精确匹配，返回单张卡或 404；**卡密明文只走 body** |
| POST | `/cards` | `{plan,count,durationDays}` → `{cards:[{id,key}]}`，明文仅此一次；不生成 CSV |
| POST | `/cards/:id/revoke` | `{status:"ok"}` |
| POST | `/cards/:id/extend` | `{days}` → `{expiresAt}`；客服续期 |
| GET | `/cards/:id/bindings` | `{boundDeviceId, binding:{deviceId,activatedAt,lastSeenAt} \| null}` |
| POST | `/bindings/unbind` | `{cardId, deviceId}` → 删除当前绑定、撤销 nonce、清除 `bound_device_id` → `{status:"ok"}`；**deviceId 只走 body** |
| GET | `/update` | `{release, currentFile:{name,size}, previousFile:{name,size}\|null}`；保留当前+上一版 |
| POST | `/update` | multipart：`version`、`releaseNotes`、`file` → 生成并返回 windows.json |

- **敏感值不得进 URL**：卡密明文、`deviceId` 都不能出现在 query string 或路径参数里，否则会被 nginx/宝塔等反代的 access log、浏览器历史完整记录。
- 列表筛选只保留 `plan` / `status`；按卡密查找走 `POST /cards/lookup`。
- 状态定义：
  - `unused`：`activated_at` 为空。
  - `active`：已激活、未撤销，且 `expires_at` 为空（lifetime）或晚于当前时间。
  - `expired`：已激活、未撤销，`expires_at` 已到。
  - `revoked`：`revoked = 1`。
- 输入边界：`count` 1~1000，`pageSize` 1~200，`durationDays` 1~3650（`lifetime` 为 0）；`days` 1~3650；`releaseNotes` 限制长度。
- `plan` 与 `durationDays` 必须一致：`daily=1`、`monthly=30`、`yearly=365`、`lifetime=0`；不要允许 `yearly + 30 天` 这种组合。
- 创建响应只含明文卡密一次；前端展示后由管理员点击“复制全部”，**不提供 CSV**。关闭弹窗前提示“关闭后无法再次查看，只能撤销重建”。
- 客服续期只改变 `expires_at`；`duration_days`、`plan`、`bound_device_id` 不变。UI 显示“设备将在下次刷新后看到新到期时间”。未激活、`lifetime`、已撤销的卡密拒绝延长，返回 `400 bad_request`。
- 客服释放设备后，卡密可以绑定到新设备，但到期时间不变；如果卡密已过期，先延长再释放/激活。

### 6. 更新包与 windows.json

- 目录：`<Profile.Data>/license/updates/`（启动时创建，0700/0750）。
- 上传流程：
  1. `http.MaxBytesReader` 限制大小（`XHL_UPDATE_MAX_BYTES`，默认 512 MiB）；`ParseMultipartForm` 的 `maxMemory` 单独设置。
  2. `version` 必须与客户端 `parseVersion` 兼容：`^\d+\.\d+\.\d+([-+][0-9A-Za-z.-]+)?$`，并拒绝低于当前 `latestVersion` 的版本；`releaseNotes` 必填且限制长度（如 8 KiB）；文件必须 `.exe`。
  3. 流式写入临时文件，同时算 SHA-256；`fsync` 后 `rename` 为 `xiaohualu-<version>.exe`。
  4. **保留当前版本与上一版本**：上传成功后保留新文件和紧邻的上一版，删除更早的 `.exe`；删除失败（Windows 上文件被下载占用）降级为 warning，不影响本次上传。
  5. 原子写 `windows.json`：`latestVersion`、`releaseNotes`、`downloadUrl`、`publishedAt`（RFC3339 UTC）、`sha256`（小写十六进制）。
- 并发上传：管理端对上传/清理操作加互斥，避免两个请求同时 rename/清理。
- `downloadUrl = url.JoinPath(<licenseBase>, "v1/update/files/xiaohualu-<version>.exe")`，其中 `<licenseBase>` 已经包含 `/xiaohualu`。
- base URL 解析顺序：`XHL_UPDATE_BASE_URL` → `Profile.InstanceURL` + `/xiaohualu` → 管理请求的 scheme+host + `/xiaohualu`。拼接时去除尾部斜杠，并避免重复追加 `/xiaohualu`。
- 校验：除 `http://localhost`、`http://127.0.0.1`、`http://[::1]` 外，`downloadUrl` 必须是 HTTPS；否则拒绝上传并提示配置 `XHL_UPDATE_BASE_URL`。
- 下载路由只允许目录内单层文件名（拒绝 `/`、`..`，先 `filepath.Base` 再校验），用 `c.File`/`http.ServeFile` 获得 Range 支持。
- `windows.json` 响应加 `Cache-Control: no-cache`；文件不存在时返回 404 `{code:"not_found",message:"暂无更新版本"}`。

### 7. 配置项（只读环境变量，不改 Viper 前缀）

| 变量 | 必填 | 默认 | 说明 |
|---|---|---|---|
| `XHL_PRIVATE_KEY` | 二选一 | — | base64 私钥 |
| `XHL_PRIVATE_KEY_FILE` | 二选一 | — | PEM 或 base64 私钥文件 |
| `XHL_TRUST_PROXY` | 否 | false | 是否信任 `X-Forwarded-For` |
| `XHL_UPDATE_BASE_URL` | 否 | `Profile.InstanceURL + /xiaohualu` | 对外下载基础 URL，**已包含 `/xiaohualu`**，必须 HTTPS（回环除外） |
| `XHL_UPDATE_MAX_BYTES` | 否 | 536870912 | 安装包大小上限 |
| `XHL_RATE_LIMIT_PER_MINUTE` | 否 | 10 | 每 IP 限流；0 = 关闭 |

直接 `os.Getenv`，与仓库现有 `MEMOS_LOG_DIR`、`MILVUS_*` 等做法一致；私钥同时配置时 `XHL_PRIVATE_KEY_FILE` 优先。

> **`.env` 是硬依赖**：`cmd/memos/main.go` 的 `init()` 在 `godotenv.Load(".env")` 失败时会 `panic("not found .env file")`，`scripts/compose.yaml` 也依赖 `env_file: .env`。因此本地 `go run` / 部署前必须先准备 `.env`（可从 `.env.example` 复制），否则服务不会启动；`XHL_*` 可以写在 `.env`，也可以用进程环境变量覆盖。
>
> 私钥文件建议权限 0600，且不要提交到仓库。V1 单密钥，不提供密钥轮换；单实例部署，不考虑多副本。

### 8. 前端

- `settingSections.ts` 新增 `{ key: "license", scope: "admin", icon: KeyRound, labelKey: "setting.license.label", component: LicenseSection }`，放在 Admin 分组，并把 `"license"` 加进 `SettingSectionKey` 联合类型。`SettingSectionDefinition` 要求 `component` 字段。
- `LicenseSection.tsx` 两个面板，用本地二段切换（两个 `Button` + `aria-selected`）或 `RadioGroup`；**不新增 Tabs 依赖**。
  - **卡密管理**：
    - 状态条：未使用/使用中/已过期/已撤销数量（来自列表接口的 `counts`）。
    - 筛选：`unused` / `active` / `expired` / `revoked`，以及套餐 `plan`。
    - 按卡密精确查找：输入卡密，走 `POST /cards/lookup`，不要把卡密拼进 URL。
    - 表格列：卡密 ID/哈希前缀、套餐、原始天数、状态、首次激活时间、当前到期时间、锁定设备、当前绑定、操作。
    - 操作：查看绑定、延长有效期、撤销、释放设备。
    - 批量创建弹窗：套餐、数量、有效天数（按套餐预填）；创建后弹窗展示明文卡密和“复制全部”按钮；**不下载 CSV**；关闭前提示不可再次查看。
    - 延长弹窗：输入延长天数，展示计算后的新到期时间；提交后 invalidate 列表。
    - 释放确认：展示卡密 ID 和锁定设备；确认后清除绑定，允许新设备激活。
  - **版本发布**：当前版本信息（版本、sha256、downloadUrl、发布时间、上一版文件）、上传表单（版本号、更新说明、.exe 文件）。
- `license-api.ts` 复用 `getRequestToken()`；收到 401 时调用 `refreshAccessToken()` 后重新 `getRequestToken()` 再重试一次（不复用旧 token）；非 JSON 错误体回退到通用文案；用 `react-hot-toast` 提示。查找、延长、释放都用 POST body。
- `uploadUpdate` 用 `FormData`，不要手写 `Content-Type`；上传大文件时给用户进度/等待提示。
- React Query hooks 放 `web/src/hooks/useLicenseQueries.ts`，query key 形如 `["license","cards",...]`，变更后 invalidate。
- 复用 `SettingTable`、`Dialog`、`ConfirmDialog`、`Input`、`Select`、`Button`、`Badge`、`SettingGroup`。
- 开发代理：`web/vite.config.mts` 增加 `^/xiaohualu`（联调用）和 `^/admin`（管理页）指向 `devProxyServer`。
- locale：以 `en.json` 为类型源，至少补 `en`、`zh-Hans`；`zh-Hant`、`en-GB` 一起补更完整。
- 明文卡密复制：前端使用 `navigator.clipboard.writeText()`，一次复制全部卡密（每行一个 `XHL-...`）；不调用后端 CSV。

### 9. 实施阶段

1. **迁移 + repo**：三套 `0.31/00__license.sql`、LATEST 追加、`repo.go`；`license_bindings` 以 `device_id` 为主键、`card_id` 唯一；`license_cards` 含 `activated_at/expires_at/bound_device_id`；测试建表、唯一约束、三库占位符。
2. **token + cardkey**：签发/验签 roundtrip、篡改、lifetime、`now >= exp` 边界、规范化与批量生成测试。
3. **配置与更新包**：私钥加载、版本号三段式、当前+上一版保留、sha256、Windows 文件占用、并发上传。
4. **领域服务**：Activate/Refresh/Deactivate；首次激活锁定到期、同设备幂等、解绑后同设备重激活不延长、不同设备 `device_limit`、客服续期例外。
5. **客户端路由**：`/xiaohualu/v1/*`、错误码映射（未配置 500、windows.json 404）、令牌桶限流、health。
6. **管理路由**：ADMIN 中间件、列表/查找/创建/撤销/延长/绑定/释放；创建只回一次明文、无 CSV；敏感值不在 URL。
7. **服务装配**：`server/server.go`、`shouldSkipFrontendStatic` 增加 `/xiaohualu`、`/admin`、数据目录、`.env` 前置。
8. **前端 API 与 hooks**：`license-api.ts`、`useLicenseQueries.ts`、Vite 代理、401 重试。
9. **管理页面与 i18n**：section、卡密面板、创建/延长/释放弹窗、版本面板、复制功能。
10. **端到端与安全复核**：建卡 → 激活 → 同设备重复激活 → 换设备 409 → 解绑 → 同设备重激活 → 换设备仍 409 → 客服释放 → 新设备激活 → 客服延长 → refresh 新 exp → 上传安装包 → 拉取 windows.json → 检查日志。

### 10. 验证

```bash
# cmd/memos 的 init() 在 .env 缺失时会 panic，先准备
cp .env.example .env

go test ./internal/license/... ./server/router/license/... ./store/...
go test -v -race ./server/... ./internal/...
golangci-lint run

cd web && pnpm lint && pnpm test && pnpm release
```

本地手工验证：

```bash
# 用 OpenSSL 生成私钥（生成的 xhl.key 不要提交）
openssl genpkey -algorithm ED25519 -out ./xhl.key
XHL_PRIVATE_KEY_FILE=./xhl.key \
XHL_UPDATE_BASE_URL=http://127.0.0.1:8081/xiaohualu \
go run ./cmd/memos --port 8081

curl localhost:8081/xiaohualu/v1/health

# 管理页 /setting#license 建卡后：
curl -X POST localhost:8081/xiaohualu/v1/activate \
  -H 'Content-Type: application/json' \
  -d '{"key":"XHL-....","deviceId":"3f2c9d8a1b7e4f60a5c3d2e1f0b9a876","appVersion":"0.1.0","os":"windows"}'

curl localhost:8081/xiaohualu/v1/update/windows.json
```

关键断言：

- 首次激活：卡密 `activated_at`/`expires_at` 写入，令牌 `exp = 激活时间 + duration`；`bound_device_id` 等于设备 ID。
- 同设备重复激活：`expires_at`/`exp` 不变，绑定数不变。
- 同一张卡换设备：`409 device_limit`。
- 客户端 `deactivate` 后：同设备可重新激活且 `exp` 不变；另一台设备仍 `409 device_limit`。
- 管理端释放设备后：新设备可以激活，`exp` 仍是原到期时间。
- 管理端延长 `days` 后：已绑定设备下一次 refresh 返回更晚的 `exp`；旧令牌已过期也能通过 refresh 恢复。
- 未激活卡密不能延长；lifetime 卡密不能延长。
- 更新包：连续上传两个版本，保留当前+上一版；非法版本号被拒；`windows.json` 的 sha256 与 `sha256sum` 一致。
- 管理端查找/延长/释放请求的 URL 中不出现明文卡密/deviceId。
- 日志和反代 access log 中没有明文卡密、令牌、deviceId。

### 11. 风险与注意

- **严格一对一**：`license_bindings.device_id` 主键 + `card_id` 唯一；同一张卡第二台设备返回 `409 device_limit`。
- **首次激活锁定**：`activated_at`/`expires_at` 只在第一次成功激活时写入；客户端解绑、同设备重激活、管理员释放都不能重算，否则会出现无限续期。
- **卡密锁定设备**：`bound_device_id` 记录首次激活设备；客户端 `deactivate` 不清除；只有管理端释放才清除。这样“卡密被使用后”不会漂移到其他设备。
- **客服续期**：`POST /cards/:id/extend` 只改 `expires_at`；refresh 的“过期令牌 + 已延长卡密”例外必须实现，否则客户延长后仍会被 `token_expired` 卡住。
- **不再自动换卡**：同设备换另一张卡返回 `device_mismatch`；客服先释放旧卡，再让客户激活新卡。
- **客户端接口前缀**：所有客户端接口在 `/xiaohualu` 下；`shouldSkipFrontendStatic` 必须跳过 `/xiaohualu` 和 `/admin`，否则 SPA fallback 会把 404 伪装成 `index.html`。
- **敏感值**：卡密查找、设备释放、延长请求都用 POST body；不得把明文卡密/deviceId 放进 URL。
- **单实例**：内存限流 + 本地更新包目录；512 MiB 上传需要反代 `client_max_body_size`、超时和 CDN 限制。
- **`.env` 硬依赖**：`cmd/memos` 的 `init()` 缺失 `.env` 会 panic。
- **前端无 Tabs 原语**：不要引入新依赖，用按钮/RadioGroup 切换；Vite 需代理 `/admin`、`/xiaohualu`。
- **迁移冲突**：`0.31/00` 对应 schema `0.31.1`；如果未来上游也使用 0.31，合并会冲突，需要接受 fork 维护成本。
- **三库差异**：占位符、事务锁、`ONLY_FULL_GROUP_BY` 集中在 `repo.go`；T1 至少用 Testcontainers 跑 MySQL/Postgres。
- **管理端认证**：`AuthenticateToUser` 是 10 分钟缓存读，不是严格实时；不要用 JWT `claims.Role`。
- **备份**：授权数据与 memos 共库，备份主库即包含卡密；`Profile.Data` 同时包含安装包，备份体积要预估。
- **密钥/部署**：V1 单密钥、单实例；不做轮换和多副本。

---

## 12. 具体实施计划（任务分解）

### 12.1 核心类型与函数签名

`internal/license`：

```go
// token.go
type Payload struct {
    V     int    `json:"v"`
    Sub   string `json:"sub"`
    Plan  string `json:"plan"`
    Iat   int64  `json:"iat"`
    Exp   int64  `json:"exp"`
    Nonce string `json:"nonce"`
}
type Signer struct { priv ed25519.PrivateKey }
func NewSigner(priv ed25519.PrivateKey) *Signer
func (s *Signer) Sign(p *Payload) (string, error)
func (s *Signer) Verify(token string, now time.Time) (*Payload, error)
func (s *Signer) PublicKeyBase64() string
func (s *Signer) PublicKeyHex() string

// cardkey.go
func GenerateCardKey() (string, error)       // XHL-XXXX-XXXX-XXXX-XXXX
func NormalizeCardKey(key string) string     // TrimSpace + ToUpper
func HashCardKey(key string) string          // sha256 hex
func LooksLikeCardKey(s string) bool

// errors.go
type Error struct { Code, Message string; HTTP int }
func (e *Error) Error() string
var ErrNotConfigured = &Error{Code: "internal", Message: "授权服务暂时不可用", HTTP: 500}

// models.go（可并入 repo.go）
type Card struct {
    ID             string
    KeyHash        string
    Plan           string
    DurationDays   int
    Revoked        bool
    ActivatedAt    string // RFC3339 UTC，空 = 未使用
    ExpiresAt      string // RFC3339 UTC，空 = lifetime/未使用
    BoundDeviceID  string
    CreatedAt      string
}
type Binding struct {
    DeviceID    string
    CardID      string
    ActivatedAt string
    LastSeenAt  string
    TokenNonce  string
}
type CardCounts struct { Unused, Active, Expired, Revoked int }

// service.go
type Service struct { repo *Repo; signer *Signer; now func() time.Time }
func (s *Service) Activate(ctx context.Context, req ActivateRequest) (*ActivateResponse, error)
func (s *Service) Refresh(ctx context.Context, req RefreshRequest) (*RefreshResponse, error)
func (s *Service) Deactivate(ctx context.Context, req DeactivateRequest) error
func (s *Service) Extend(ctx context.Context, cardID string, days int) (string, error) // 返回新 expiresAt
func (s *Service) Release(ctx context.Context, cardID, deviceID string) error

// repo.go
func NewRepo(db *sql.DB, driver string) *Repo
func (r *Repo) CreateCards(ctx context.Context, cards []*Card) error
func (r *Repo) ListCards(ctx context.Context, f *FindCard) ([]*Card, int, *CardCounts, error)
func (r *Repo) GetCard(ctx context.Context, f *FindCard) (*Card, error)
func (r *Repo) LookupCardByKeyHash(ctx context.Context, keyHash string) (*Card, error)
func (r *Repo) RevokeCard(ctx context.Context, cardID string) error
func (r *Repo) ExtendCard(ctx context.Context, cardID string, newExpiresAt string) error
func (r *Repo) ListBindings(ctx context.Context, cardID string) ([]*Binding, error)
func (r *Repo) GetBindingByDevice(ctx context.Context, deviceID string) (*Binding, error)
func (r *Repo) Activate(ctx context.Context, p ActivateParams) (*ActivationResult, error) // 首次激活写 activated_at/expires_at/bound_device_id
func (r *Repo) Refresh(ctx context.Context, p RefreshParams) (*Card, error)
func (r *Repo) Deactivate(ctx context.Context, p DeactivateParams) error // 保留 bound_device_id
func (r *Repo) Release(ctx context.Context, cardID, deviceID string) error // 清除 bound_device_id

// update.go
type UpdateStore struct { dir string; maxBytes int64 }
type Release struct {
    LatestVersion string `json:"latestVersion"`
    ReleaseNotes  string `json:"releaseNotes"`
    DownloadURL   string `json:"downloadUrl"`
    PublishedAt   string `json:"publishedAt"`
    SHA256        string `json:"sha256"`
}
func (u *UpdateStore) Read() (*Release, error)
func (u *UpdateStore) Save(ctx context.Context, version, notes, licenseBase string, file io.Reader) (*Release, error)
func (u *UpdateStore) FilePath(name string) (string, error)
func ValidateReleaseVersion(raw string) error
```

`server/router/license`：

```go
type Service struct {
    Profile    *profile.Profile
    License    *license.Service
    Update     *license.UpdateStore
    Limiter    *license.RateLimiter
    Configured bool
}
func (s *Service) RegisterRoutes(e *echo.Echo) // 内部 Group("/xiaohualu")

type AdminService struct {
    Profile *profile.Profile
    Store   *store.Store
    Repo    *license.Repo
    Update  *license.UpdateStore
    Auth    *auth.Authenticator
    Signer  *license.Signer
}
func (s *AdminService) RegisterRoutes(e *echo.Echo)
```

> 所有原生 handler 使用 Echo v5 的 `func(c *echo.Context) error`；`AdminService` 的 `Store` 用于构造 `auth.NewAuthenticator(store, secret)`，`Signer` 可能为 nil（未配置私钥），`/status` 要能处理这种状态。

### 12.2 任务清单

按顺序实施，每个任务结束都能独立编译、测试。

#### T1 数据层（迁移 + Repo）

- 文件：`store/migration/{sqlite,mysql,postgres}/0.31/00__license.sql`、三份 `LATEST.sql`、`internal/license/repo.go`、`internal/license/repo_test.go`
- 实现：
  - 三张表；`license_bindings` 以 `device_id` 为主键、`card_id` 唯一；`license_cards` 含 `activated_at/expires_at/bound_device_id`，无 `max_devices`
  - `key_hash` 内联 `UNIQUE`
  - `Repo` 用 `driver` 字段切换占位符：postgres 重写 `?` 为 `$n`；统一 `rebind()`
  - `Activate` 的事务：sqlite 用 `*sql.Conn` + `BEGIN IMMEDIATE`；mysql/postgres 用 `BeginTx` + `SELECT ... FOR UPDATE` 锁卡密行；分别捕获 `device_id` 与 `card_id` 唯一键冲突
  - `Deactivate` 只删绑定、保留 `bound_device_id`；`Release` 删绑定并清空 `bound_device_id`
- 验收：
  - 内存 SQLite 建表、唯一 hash 冲突、CRUD 通过
  - 首次激活写入 `activated_at`/`expires_at`/`bound_device_id`
  - 同设备重复激活不新增绑定、不重算到期
  - 客户端解绑后同设备重激活复用原到期；`Release` 后新设备可激活
  - 并发 20 个 goroutine 用不同 deviceId 激活同一张卡，只有 1 个成功，其余 `409 device_limit`；同设备并发激活两张卡，只有一张成功
  - Testcontainers 跑通 MySQL/Postgres（Docker 不可用时跳过并标注）
  - `go test ./internal/license/... ./store/...`

#### T2 令牌与卡密

- 文件：`internal/license/token.go`、`cardkey.go` 及测试
- 实现：
  - `RawURLEncoding` 无填充；签名对象是 payload 的 base64 字符串
  - `Verify` 校验结构、签名、`v==1`、`plan` 白名单、`iat>0`、`exp==0 || exp>iat`；到期判断用 `now.Unix() >= exp`
  - 非 `lifetime` 的 `exp` 必须 `> iat`；`lifetime` 的 `exp` 必须为 0
  - 卡密字母表去掉 `0/O/1/I/L`，16 位分 4 组
  - 私钥解析：PEM PKCS8 / 标准 base64 / URL base64；32 字节 seed 用 `ed25519.NewKeyFromSeed`，64 字节直接使用
- 验收：
  - roundtrip、篡改 payload/签名被拒、lifetime `exp=0`、过期返回可区分错误
  - 规范化 `" xhl-abcd-... "` 与 `"XHL-ABCD-..."` 哈希一致
  - 用同一对密钥与客户端 `internal/license/token_test.go` 做交叉验签

#### T3 配置与更新包存储

- 文件：`internal/license/config.go`、`update.go` 及测试
- 实现：
  - 私钥：PEM `BEGIN PRIVATE KEY` → `x509.ParsePKCS8PrivateKey`；否则 base64 解码，接受 32 字节 seed 或 64 字节私钥；错误信息不含密钥内容
  - 版本号：`^\d+\.\d+\.\d+([-+][0-9A-Za-z.-]+)?$`，与客户端 `parseVersion` 一致，并拒绝低于当前 `latestVersion`
  - `UpdateStore.Save`：临时文件与目标同目录 → 流式 `io.Copy` 到 `io.MultiWriter(file, hasher)` → `Sync` → `Rename`；保留当前+上一版，删除更早的 `.exe`（失败只记 warning）；原子写 `windows.json`；上传/清理加互斥
  - `downloadUrl` 由调用方传入已包含 `/xiaohualu` 的 licenseBase，用 `url.JoinPath` 拼接，校验 HTTPS/回环（含 `[::1]`）
- 验收：
  - PEM、标准 base64、URL base64 三种私钥输入都能加载；32 字节 seed 签发可验签
  - 上传后 sha256 与 `sha256sum` 一致；`windows.json` 字段完整、`Cache-Control: no-cache`
  - `FilePath("../x")`、`"/etc/passwd"` 被拒
  - `1.2`、`latest` 被拒；`1.2.3`、`v1.2.3`、`1.2.3-beta.1` 按预期处理
  - 连续上传两个版本，保留当前+上一版；Windows 上删除占用文件失败不影响发布

#### T4 领域服务

- 文件：`internal/license/service.go`、`internal/license/service_test.go`
- 实现：按 §3 的 Activate/Refresh/Deactivate/Extend/Release 流程；错误统一返回 `*license.Error`
- 实现要点：
  - 首次激活写 `activated_at/expires_at/bound_device_id`；之后只读，不重算
  - 同卡幂等；同设备换卡 → `device_mismatch`；不同设备使用已锁定卡 → `device_limit`
  - 客户端解绑保留 `bound_device_id`；管理端释放清除
  - 服客续期后 refresh 返回新 exp；过期令牌 + 已延长卡密可恢复
- 验收：
  - 契约 §8 场景 1~8 在领域层全部通过
  - 首次激活到期时间 = 激活时间 + duration
  - 同设备重激活/解绑后重激活不改变到期
  - 同设备换卡（旧卡仍锁定）→ `device_mismatch`
  - 同一张卡换设备 → `device_limit`；管理端释放后新设备可激活
  - 客服延长后 refresh 新 exp；过期令牌 + 已延长卡密恢复
  - lifetime 卡密 `exp=0`，不能延长
  - 未激活卡密不能延长

#### T5 客户端契约路由

- 文件：`server/router/license/service.go`、`service_test.go`
- 实现：
  - `e.Group("/xiaohualu")` 下 6 条路由；JSON 响应/错误按契约，`application/json; charset=utf-8`
  - `http.MaxBytesReader` 64 KiB；令牌桶限流，health 豁免，map 定期清理
  - 未配置私钥时除 health 外返回 `500 internal`
  - `windows.json` 缺失返回 404 `not_found`；响应 `Cache-Control: no-cache`
  - 到期边界统一 `now >= exp`
- 验收：
  - 表驱动测试覆盖全部 error code 与 HTTP 状态
  - 连续第 11 次请求返回 429；`XHL_TRUST_PROXY` 关闭时伪造 XFF 不影响限流
  - `GET /xiaohualu/v1/health` 返回 UTC Unix 秒
  - 未配置私钥时 health 正常、其余返回 500
  - 未注册的 `/xiaohualu/*` 返回 404/JSON，不被 SPA fallback 伪装

#### T6 管理路由

- 文件：`server/router/license/admin.go`、`admin_test.go`
- 实现：
  - group 中间件 `requireAdmin`：`AuthenticateToUser` → nil 401 → `Role != store.RoleAdmin` 403；不要用 JWT `claims.Role`
  - 列表/状态筛选；`POST /cards/lookup`（卡密走 body）；创建批次事务（无 CSV）；撤销；`POST /cards/:id/extend`；绑定列表；`POST /bindings/unbind`（deviceId 走 body，删绑定 + 撤销 nonce + 清 `bound_device_id`）
  - 输入边界：`count`、`pageSize`、`durationDays`、`days`、`releaseNotes`；`plan` 与 `durationDays` 一致
  - 上传：`ParseMultipartForm` + 字段校验 + `UpdateStore.Save`
- 验收：
  - 匿名 401、普通用户 403、admin 200
  - 创建响应含明文卡密，二次查询不再返回明文；响应不含 CSV
  - 解绑/释放后对应令牌 refresh 失败；新设备可以激活；到期时间不变
  - 客服延长后 `expires_at` 更新；未激活/lifetime/撤销卡密拒绝延长
  - 同一张卡第二台设备激活 → `409 device_limit`
  - 上传后 `GET /xiaohualu/v1/update/windows.json` 可读到新内容
  - 测试断言请求 URL 中不含明文卡密/deviceId

#### T7 服务装配

- 文件：`server/server.go`、`server/router/frontend/frontend.go`
- 实现：
  - `NewServer` 中加载配置、构造 repo/service/update store、注册两组路由；`AdminService` 用 `auth.NewAuthenticator(store, secret)` 构造认证器
  - `shouldSkipFrontendStatic` 增加 `hasPathPrefix(path, "/xiaohualu") || hasPathPrefix(path, "/admin")`
  - 数据目录 `<Profile.Data>/license/updates` 启动时 `MkdirAll`
  - `go run` 前必须存在 `.env`
- 验收：
  - `go run ./cmd/memos` 后 curl `/xiaohualu/v1/health`；`/setting` 前端不受影响
  - 不配置私钥也能正常启动，health 正常、其余客户端接口返回 500
  - 未注册的 `/xiaohualu/*`、`/admin/*` 返回 404/JSON，不被 SPA fallback 伪装

#### T8 前端 API 与 hooks

- 文件：`web/src/utils/license-api.ts`、`web/src/hooks/useLicenseQueries.ts`、`web/vite.config.mts`
- 实现：
  - `request()` 自动取 token；401 时 `refreshAccessToken()` 后重新 `getRequestToken()` 再重试一次
  - 错误类 `LicenseAPIError { code, message, status }`；非 JSON 错误体回退到通用文案
  - 卡密查找、延长、释放用 POST body；`uploadUpdate` 用 `FormData`，不手写 Content-Type
  - React Query key 工厂 + query/mutation；mutation 后 invalidate
  - `vite.config.mts` 增加 `^/admin`、`^/xiaohualu` 代理
- 验收：`cd web && pnpm lint`；401 重试与非 JSON 错误用 Vitest 覆盖；`pnpm dev` 下管理页请求能到达后端

#### T9 管理页面与 i18n

- 文件：`web/src/components/Settings/settingSections.ts`、`LicenseSection.tsx`、`license/LicenseCardsPanel.tsx`、`CreateLicenseCardsDialog.tsx`、`ExtendLicenseCardDialog.tsx`、`LicenseBindingsDialog.tsx`、`LicenseUpdatePanel.tsx`、`web/src/locales/*.json`
- 页面：
  - 卡密面板：状态条、筛选、按卡密查找、表格、批量创建、创建结果复制、延长、撤销、绑定查看/释放
  - 版本面板：当前版本、上一版、上传表单
  - 两个面板用本地二段切换；不新增 Tabs 依赖
- `settingSections.ts` 的定义必须带 `component: LicenseSection`，并把 `"license"` 加进 `SettingSectionKey`
- 验收：
  - 仅 ADMIN 能在 `/setting#license` 看到入口
  - 全流程可走通：建卡 → 复制明文 → 激活 → 解绑 → 同设备重激活 → 换设备 409 → 释放 → 新设备激活 → 延长 → refresh 新 exp → 上传版本
  - 创建结果没有 CSV 按钮，只有复制
  - `pnpm lint && pnpm test && pnpm build`

#### T10 端到端与安全复核

- 用临时私钥启动服务，跑通：
  1. admin 建卡（记录明文）
  2. `/xiaohualu/v1/activate` 拿令牌 → 本地用公钥验签 → 核对 `exp = 激活时间 + duration`
  3. 同设备再次 activate：绑定数不变、`exp` 不变
  4. 换设备 activate：`409 device_limit`
  5. `/xiaohualu/v1/deactivate` 后同设备重新 activate：成功且 `exp` 不变
  6. 解绑后另一台设备 activate：仍 `409 device_limit`
  7. 管理端释放设备 → 新设备 activate：成功，`exp` 不变
  8. 管理端延长 `days` → 已绑定设备 refresh：返回新 `exp`；构造已过期令牌再 refresh：恢复
  9. `/xiaohualu/v1/refresh`：正常刷新 `exp` 不变、`nonce` 变化
  10. 上传两个版本：保留当前+上一版；非法版本号被拒
  11. 连续请求触发 429
- 检查日志与反代 access log 中无明文卡密/token/deviceId；管理端查找/延长/释放 URL 不含敏感参数
- 启动前确认 `.env` 存在
- 运行：
  ```bash
  go test ./internal/license/... ./server/router/license/... ./store/...
  go test -v -race ./server/... ./internal/...
  golangci-lint run
  cd web && pnpm lint && pnpm test && pnpm build
  ```

### 12.3 契约场景 → 测试映射

| 场景 | 测试位置 |
|---|---|
| 1 首次激活令牌可验签、`sub==deviceId` | `internal/license/service_test.go` |
| 2 同设备重复激活不增绑定 | 同上 |
| 3 同一张卡换设备 `409 device_limit` | 同上 + repo 并发测试 |
| 4 卡密过期 `403 card_expired` | 同上 |
| 5 撤销后激活/刷新失败 | 同上 |
| 6 refresh 同 exp、新 iat/nonce | 同上 |
| 7 篡改令牌 `401 invalid_token` | `token_test.go` + HTTP 测试 |
| 8 health | `server/router/license/service_test.go` |
| 9 首次激活写入 `expires_at`/`bound_device_id` | `internal/license/service_test.go` |
| 10 客户端解绑后同设备可重激活、其他设备 409 | 同上 |
| 11 管理端释放后新设备可激活 | 同上 |
| 12 客服延长后 refresh 新 exp；过期令牌恢复 | 同上 |
| 13 管理端查找/延长/释放 URL 不含敏感值 | `server/router/license/admin_test.go` |
| 14 版本号三段式校验 | `internal/license/update_test.go` |
| 15 `/xiaohualu` 前缀与 SPA fallback | `server/router/license/service_test.go` |

### 12.4 交付顺序建议

先合入 T1~T4（纯后端、可单测），再合入 T5~T7（接口可用、前端尚未接入），最后 T8~T10（页面与端到端）。每阶段单独提交，提交信息用简洁英文，例如：

- `feat: add license card storage and Ed25519 token`
- `feat: add xiaohualu license client endpoints`
- `feat: add license admin API with renewal`
- `feat(web): add license admin settings section`

---

## 13. 剩余待确认项（不阻塞 T1）

1. **`durationDays` 与 `plan` 的关系**：当前要求一致（daily=1/monthly=30/yearly=365/lifetime=0）。如果需要任意天数（例如 7 天、90 天），需要扩展契约的 plan 取值或允许“自定义套餐”。

**确认**： 可以任意天数，但是也支持按天/月/年

2. **客服续期的输入**：当前是“延长 N 天”，从 `max(now, 当前 expires_at)` 起算。如果你更希望直接选择新的到期日期，改成绝对日期输入即可。

**确认**：直接改成绝对日期

3. **客户端解绑后的锁定**：当前设计是“解绑只删绑定，卡密仍锁定原设备；同设备可重激活，其他设备需客服释放”。如果你的意思是解绑后卡密立即对任何设备可用，需要去掉 `bound_device_id` 的留置行为。

**确认**：客户端没有解绑功能，已在激活中就一直用，只有服务端有解绑功能，相当于客户要换设备要跟客服说，客服来操作，并把原来这个卡密再给客户，让他来重新激活，继承卡密的剩余时间

4. **管理端释放策略**：当前允许客服对任意卡密释放设备（包括未过期的），用于换机；如果你只允许过期/撤销卡密释放，需要加限制。

**确认**：允许对任意卡密操作


5. **`XHL_UPDATE_BASE_URL` 是否包含 `/xiaohualu`**：当前定义为包含（例如 `https://host/xiaohualu`）。如果部署习惯填 host 根地
址，需要改成由服务端自动补前缀。

**确认**： 我的客户端会自动携带这个/xiaohualu 这个前缀

6. **未使用的卡密是否会过期**：当前不会，`expires_at` 在首次激活时才计算。如果希望“发卡后 N 天内必须激活”，需要额外字段。

**确认**：不会过期

7. **过期卡密是否允许客服延长**：当前允许，从 `now` 重新起算 N 天。如果希望只允许在到期前延长，需要加限制。

**确认**：允许，延长相当于续费，又重新可用了

8. **过期令牌 + 已延长卡密的 refresh 恢复**：当前允许（不返回 `token_expired`，直接签发新令牌）。这会让客户续期后无需重新输入卡密；如果你希望客户端必须重新激活，去掉该例外。

**确认**：允许，不需要重新激活

9. **“保留上一版”的定义**：当前是保留当前版 + 上一版（两个 `.exe`）。如果你只想保留当前版，上传时删除上一版即可。

**确认**：保留最新版和最新版的上一版

## 14. 本次修订摘要（2026-10-25）

- **严格一对一**：删除 `max_devices`；`license_bindings` 用 `device_id` 主键 + `card_id` 唯一。
- **首次激活锁定到期**：卡密新增 `activated_at`、`expires_at`；非 lifetime 首次激活按 `duration_days` 计算，之后不重算。
- **卡密锁定设备**：新增 `bound_device_id`；客户端 `deactivate` 保留，管理端释放才清除。
- **客服续期**：新增 `POST /cards/:id/extend`；refresh 支持返回更晚的 exp，并允许过期令牌在卡密已延长时恢复。
- **不再自动换卡**：同设备换另一张卡返回 `device_mismatch`，由客服先释放旧卡。
- **客户端前缀**：所有客户端接口挂到 `/xiaohualu`；客户端只改 `XHL_LICENSE_SERVER` base URL。
- **明文卡密**：不生成 CSV，创建结果在网页展示并复制。
- **更新包**：保留当前版 + 上一版。
- **部署**：明确单实例、单密钥，不考虑多副本和密钥轮换。
- **移除**：`max_devices`、按绝对日期创建、CSV 导出、自动续费换卡、旧版“多设备共享 term”设计。
