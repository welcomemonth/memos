# 授权服务集成到 memos 实施方案

> 版本：1.0（2026-10-25）
> 依据：`授权服务端契约.md` v1.0、`授权服务端设计.md` v1.1。
> 本文档是把授权服务端**合并进 memos 主项目**的实施蓝图，替代设计文档中“独立 Go 仓库 + Gin + 独立 SQLite + 独立管理页”的方案。对外客户端契约不变。

## Context

桌面端软件「小花录」已有独立授权服务端设计，原方案是独立 Go 仓库 + Gin + 独立 SQLite + 独立管理页。用户认为独立服务太分散，改为合并进当前 memos 项目：

- 客户端契约接口保持原样（桌面客户端不修改）：`/v1/activate`、`/v1/refresh`、`/v1/deactivate`、`/v1/health`、`/v1/update/windows.json`
- 管理端（卡密创建/列表/撤销/绑定查看/解绑/版本发布）只允许 memos 的 `ADMIN` 角色访问
- 管理界面集成进 memos 现有前端（设置页 admin section），不单独维护 React 应用
- 同进程、同端口、同一二进制、复用 memos 主数据库
- 私钥严格使用 `XHL_PRIVATE_KEY` / `XHL_PRIVATE_KEY_FILE`，不提供 keygen 命令
- 管理页支持上传新版本安装包，自动计算 sha256 并生成 `windows.json`；版本号、版本说明手填
- 更新包存本地目录，只保留当前版本

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
├── repo.go          # 卡密/绑定/撤销 SQL（sqlite/mysql/postgres 占位符与事务差异集中在此）
├── ratelimit.go     # 每 IP 令牌桶
└── update.go        # 更新包落盘、sha256、windows.json 原子读写

server/router/license/
├── service.go       # /v1/* 客户端契约路由
├── admin.go         # /admin/api/license/* 管理路由 + ADMIN 中间件
└── *_test.go        # httptest 契约回归、错误码、限流、admin 权限

store/migration/{sqlite,mysql,postgres}/0.31/00__license.sql
```

修改：

- `store/migration/{sqlite,mysql,postgres}/LATEST.sql`：追加三张表 DDL
- `server/server.go`：构造 license 服务并注册路由
- `server/router/frontend/frontend.go`：`shouldSkipFrontendStatic` 增加 `/v1`、`/admin`
- `web/src/components/Settings/settingSections.ts`：新增 `license` section（scope: admin）
- `web/src/components/Settings/LicenseSection.tsx`（新增）
- `web/src/utils/license-api.ts`（新增，带 Bearer + 401 刷新重试的 fetch 封装）
- `web/src/hooks/useLicenseQueries.ts`（新增）
- `web/src/locales/{en,zh-Hans,zh-Hant,en-GB}.json`：新增 `setting.license.*`
- `.env.example`：补充 `XHL_*` 变量说明

不修改 `proto/`，不影响 MCP/OpenAPI。不新增 `acl_config.go` 条目（手写路由不经过 Connect 鉴权）。

### 2. 数据库

表名加 `license_` 前缀，避免与 memos 表冲突。三套迁移 + LATEST 保持一致。

```sql
CREATE TABLE IF NOT EXISTS license_cards (
    id            VARCHAR(64) PRIMARY KEY,
    key_hash      CHAR(64)    NOT NULL UNIQUE,
    plan          VARCHAR(16) NOT NULL,
    duration_days INTEGER     NOT NULL,
    max_devices   INTEGER     NOT NULL DEFAULT 1,
    revoked       INTEGER     NOT NULL DEFAULT 0,
    expires_at    VARCHAR(32) NOT NULL DEFAULT '',
    created_at    VARCHAR(32) NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_license_cards_key_hash ON license_cards(key_hash);

CREATE TABLE IF NOT EXISTS license_bindings (
    card_id       VARCHAR(64) NOT NULL,
    device_id     VARCHAR(64) NOT NULL,
    activated_at  VARCHAR(32) NOT NULL,
    last_seen_at  VARCHAR(32) NOT NULL,
    token_nonce   VARCHAR(64) NOT NULL,
    PRIMARY KEY (card_id, device_id)
);

CREATE TABLE IF NOT EXISTS license_revocations (
    nonce      VARCHAR(64) PRIMARY KEY,
    reason     VARCHAR(32) NOT NULL,
    revoked_at VARCHAR(32) NOT NULL
);
```

- 时间统一 RFC3339 UTC 文本；对外 Unix 秒。
- `expires_at` 空字符串 = 卡密本身不过期（与绑定到期时间 `exp` 不同）。
- SQLite/MySQL/Postgres 均使用上述类型；Postgres 的 `INTEGER`、`VARCHAR`、`TEXT` 均可接受。

`internal/license/repo.go` 通过 `store.GetDriver().GetDB()` 和 `Profile.Driver` 执行 SQL，不扩 `store.Driver` 接口。理由：授权是独立 bounded context，查询少；这样避免在三套 store 驱动里各加十几个方法。迁移仍走 memos 现有迁移系统。

### 3. 核心流程与并发

**卡密生成**
- 格式 `XHL-XXXX-XXXX-XXXX-XXXX`，16 位取自无歧义字母表 `23456789ABCDEFGHJKMNPQRSTUVWXYZ`。
- 入库前规范化：`strings.ToUpper(strings.TrimSpace(key))` → SHA-256 十六进制；只存哈希。
- 明文只在创建响应中出现一次，不落库、不写日志。
- 批量创建在一个事务里完成。

**Activate（`POST /v1/activate`）**
1. 校验 `key`、`deviceId`（32 位小写十六进制），否则 `400 bad_request`。
2. 规范化 + 哈希查卡：不存在 → `401 invalid_key`；撤销 → `403 revoked`；卡密自身过期 → `403 card_expired`。
3. 事务内：
   - 查同设备绑定；存在 → 幂等，`exp` 保持不变，更新 `last_seen_at`、签发新 nonce/iat 的令牌。
   - 不存在 → 统计绑定数；`>= max_devices` → `409 device_limit`；否则插入绑定，`exp = now + duration_days*86400`（`lifetime` → `0`）。
   - SQLite 用 `BEGIN IMMEDIATE`（`*sql.Conn` 显式开启）避免并发超额；MySQL/Postgres 对 card 行 `SELECT ... FOR UPDATE`。
4. 事务成功后用 Ed25519 签发令牌，返回 `{token,status:"active"}`。

**Refresh（`POST /v1/refresh`）**
1. 验签失败/结构损坏 → `401 invalid_token`；令牌 `exp != 0 && now > exp` → `401 token_expired`。
2. `sub != deviceId` → `403 device_mismatch`。
3. nonce 在 `license_revocations` 或绑定不存在 → `403 revoked`。
4. 卡密撤销 → `403 revoked`；卡密自身过期 → `403 card_expired`。
5. 新令牌：`plan`、`exp` 不变，`iat = now`，`nonce` 重新生成；更新绑定 `token_nonce`、`last_seen_at`。

**Deactivate（`POST /v1/deactivate`）**
- 验签；`sub != deviceId` → `403 device_mismatch`。
- 事务内删除绑定 + 当前 nonce 写入 `license_revocations`；重复调用返回 `{status:"ok"}`。
- 解绑后旧令牌 refresh 必然 `403 revoked`。

**令牌**
- `payload = {v:1, sub, plan, iat, exp, nonce}`，JSON 序列化后 `base64.RawURLEncoding`。
- `sig = ed25519.Sign(priv, []byte(payloadB64))`，再 RawURLEncoding。
- 私钥加载：
  - `XHL_PRIVATE_KEY`：base64 文本（标准/无填充/URL 编码均可）。
  - `XHL_PRIVATE_KEY_FILE`：文件内容可以是 OpenSSL PEM（`BEGIN PRIVATE KEY`）或 base64 文本。
  - 两者都未配置：服务正常启动，但 `/v1/*`（health 除外）返回 `503 internal`；管理页显示“未配置”。
  - 验签失败/长度不是 64 字节私钥或 32 字节 seed → 启动时记录 error 并禁用授权接口。

### 4. 客户端接口（手写 Echo）

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/v1/activate` | 激活/同设备幂等 |
| POST | `/v1/refresh` | 刷新令牌 |
| POST | `/v1/deactivate` | 解绑当前设备 |
| GET | `/v1/health` | `{status:"ok",time:<unix秒>}`，UTC，不限流 |
| GET | `/v1/update/windows.json` | 读取更新包目录下 windows.json |
| GET | `/v1/update/files/:filename` | 下载安装包，支持 Range |

- 统一响应头 `application/json; charset=utf-8`；错误体固定 `{code,message}`，HTTP 状态严格按契约第 4 节。
- 请求体限制 64 KiB；`/v1/*`（health 除外）每 IP 10 次/分钟，命中 `429 rate_limited`。
- `XHL_TRUST_PROXY=true` 时从 `X-Forwarded-For` 取首个 IP，否则用 `RemoteAddr`。
- 日志只记路径、状态码、card id；绝不记明文卡密、令牌、deviceId。

### 5. 管理接口（仅 ADMIN）

统一前缀 `/admin/api/license`，Echo group 上挂认证中间件：

```go
user, err := authenticator.AuthenticateToUser(ctx, authHeader, cookieHeader)
// user == nil            -> 401 {code:"unauthenticated"}
// user.Role != ADMIN     -> 403 {code:"permission_denied"}
```

`AuthenticateToUser` 会重新查库，角色是最新值，不受 JWT 15 分钟快照影响。

| 方法 | 路径 | 请求/响应 |
|---|---|---|
| GET | `/status` | `{configured, publicKey, updateBaseUrl}` |
| GET | `/cards?page=&pageSize=&q=&status=` | `{cards:[{id,keyHash,plan,durationDays,maxDevices,revoked,expiresAt,createdAt,deviceCount}],total}` |
| POST | `/cards` | `{plan,count,maxDevices,durationDays,expiresAt}` → `{cards:[{id,key}]}`，明文仅此一次 |
| POST | `/cards/:id/revoke` | `{status:"ok"}` |
| GET | `/cards/:id/bindings` | `{bindings:[{deviceId,activatedAt,lastSeenAt}]}` |
| DELETE | `/bindings/:cardId/:deviceId` | 删除绑定 + nonce 入撤销集合 → `{status:"ok"}` |
| GET | `/update` | 当前 windows.json 内容 + 文件名/大小 |
| POST | `/update` | multipart：`version`、`releaseNotes`、`file` → 生成并返回 windows.json |

搜索 `q`：若形如卡密（可规范化后哈希），按 `key_hash` 精确匹配；否则按 `id`、`plan` 模糊匹配。

### 6. 更新包与 windows.json

- 目录：`<Profile.Data>/license/updates/`（启动时创建，0700/0750）。
- 上传流程：
  1. `http.MaxBytesReader` 限制大小（`XHL_UPDATE_MAX_BYTES`，默认 512 MiB）。
  2. `version` 校验 `^[0-9A-Za-z._-]+$`；`releaseNotes` 必填；文件必须 `.exe`。
  3. 流式写入临时文件，同时算 SHA-256；`fsync` 后 `rename` 为 `xiaohualu-<version>.exe`；删除上一个版本文件（只保留当前版）。
  4. 构造 `downloadUrl = <base>/v1/update/files/xiaohualu-<version>.exe`。
  5. 原子写 `windows.json`：`latestVersion`、`releaseNotes`、`downloadUrl`、`publishedAt`（RFC3339 UTC）、`sha256`（小写十六进制）。
- base URL 解析顺序：`XHL_UPDATE_BASE_URL` → `Profile.InstanceURL` → 管理请求的 scheme+host（含反向代理头）。
- 校验：除 `http://localhost`、`http://127.0.0.1` 外，`downloadUrl` 必须是 HTTPS；否则拒绝上传并提示配置 `XHL_UPDATE_BASE_URL`。
- 下载路由只允许目录内单层文件名（拒绝 `/`、`..`），用 `c.File`/`http.ServeFile` 获得 Range 支持。
- `windows.json` 不存在时返回 404 `{code:"not_found",message:"暂无更新版本"}`。

### 7. 配置项（只读环境变量，不改 Viper 前缀）

| 变量 | 必填 | 默认 | 说明 |
|---|---|---|---|
| `XHL_PRIVATE_KEY` | 二选一 | — | base64 私钥 |
| `XHL_PRIVATE_KEY_FILE` | 二选一 | — | PEM 或 base64 私钥文件 |
| `XHL_TRUST_PROXY` | 否 | false | 是否信任 `X-Forwarded-For` |
| `XHL_UPDATE_BASE_URL` | 否 | `Profile.InstanceURL` | 对外下载基础 URL，必须 HTTPS（回环地址除外） |
| `XHL_UPDATE_MAX_BYTES` | 否 | 536870912 | 安装包大小上限 |
| `XHL_RATE_LIMIT_PER_MINUTE` | 否 | 10 | 每 IP 限流 |

直接 `os.Getenv`，与仓库现有 `MEMOS_LOG_DIR`、`MILVUS_*` 等做法一致；`.env` 由 godotenv 自动加载。私钥同时配置时 `XHL_PRIVATE_KEY_FILE` 优先。

### 8. 前端

- `settingSections.ts` 新增 `{ key: "license", scope: "admin", icon: KeyRound, labelKey: "setting.license.label" }`，放在 Admin 分组。
- `LicenseSection.tsx` 两个 tab：
  - **卡密管理**：状态提示、搜索/状态筛选、分页表格、批量创建弹窗、创建结果（复制全部 + CSV 下载）、撤销确认、绑定查看与解绑。
  - **版本发布**：当前版本信息（版本、sha256、downloadUrl、发布时间）、上传表单（版本号、更新说明、.exe 文件）。
- `license-api.ts` 复用 `getRequestToken()`；收到 401 时调用 `refreshAccessToken()` 重试一次；错误解析 `{code,message}` 并用 `react-hot-toast` 提示。
- React Query hooks 放 `web/src/hooks/useLicenseQueries.ts`，query key 形如 `["license","cards",...]`，变更后 invalidate。
- 复用 `SettingTable`、`Dialog`、`ConfirmDialog`、`Input`、`Select`、`Button`、`Badge`。
- locale：以 `en.json` 为类型源，至少补 `en`、`zh-Hans`；`zh-Hant`、`en-GB` 一起补更完整。

### 9. 实施阶段

1. **迁移 + repo**：三套 `0.31/00__license.sql`、LATEST 追加、`repo.go`；测试建表/CRUD/唯一索引。
2. **token + cardkey**：签发/验签 roundtrip、篡改、lifetime、规范化与批量生成测试。
3. **领域服务**：Activate/Refresh/Deactivate，覆盖契约 §8 场景 1~7（幂等、设备上限、卡过期、撤销、刷新换 nonce 同 exp、解绑后 refresh 失败）。
4. **客户端路由**：`/v1/*`、错误码映射、限流、health、update.json 读取；httptest 全错误码。
5. **管理路由**：ADMIN 中间件、卡密 CRUD、绑定/解绑；测试未登录 401、非 admin 403、创建只回一次明文。
6. **更新包**：上传、sha256、原子写、下载、HTTPS 校验；测试非法版本号/超大文件/回环地址。
7. **前端**：section、API 封装、hooks、页面、i18n；`pnpm lint && pnpm test`。
8. **端到端**：管理员登录 → 建卡 → 客户端 activate → refresh → 解绑 → refresh 失败 → 上传安装包 → 拉取 windows.json。

### 10. 验证

```bash
go test ./internal/license/... ./server/router/license/... ./store/...
go test -v -race ./server/... ./internal/...
golangci-lint run

cd web && pnpm lint && pnpm test && pnpm release
```

本地手工验证：

```bash
# 用 OpenSSL 生成私钥（无需仓库内置 keygen）
openssl genpkey -algorithm ED25519 -out /tmp/xhl.key
XHL_PRIVATE_KEY_FILE=/tmp/xhl.key XHL_UPDATE_BASE_URL=http://127.0.0.1:8081 go run ./cmd/memos --port 8081

curl localhost:8081/v1/health
# 管理页 /setting#license 建卡后：
curl -X POST localhost:8081/v1/activate \
  -H 'Content-Type: application/json' \
  -d '{"key":"XHL-....","deviceId":"3f2c9d8a1b7e4f60a5c3d2e1f0b9a876","appVersion":"0.1.0","os":"windows"}'
curl localhost:8081/v1/update/windows.json
```

### 11. 风险与注意

- `/v1`、`/admin` 必须加入 `shouldSkipFrontendStatic`，否则可能被静态资源中间件/SPA fallback 干扰。
- 不把 `/v1/*` 加进 `acl_config.go`（那是 Connect/gateway 的白名单，与手写路由无关）。
- 迁移版本用 `0.31/00`（实际 schema 版本 `0.31.1`）；三套 LATEST 必须与增量迁移等价。
- 三库占位符差异（`?` vs `$n`）和事务锁差异集中在 `repo.go`，不要在 handler 里写 SQL。
- 管理接口不要复用 JWT 里的 `claims.Role`，必须用 `AuthenticateToUser` 的查库结果。
- 上传文件名和下载路径做严格校验，避免目录穿越；临时文件与目标文件必须同目录以保证 rename 原子性。
- 日志、错误响应中不得出现明文卡密、令牌、deviceId 原文。
- 授权数据与 memos 共库：备份 memos 主库即包含卡密；私钥仍只在环境变量/文件中，不落库。

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
var ErrNotConfigured = &Error{Code: "internal", Message: "授权服务暂时不可用", HTTP: 503}

// service.go
type Service struct { repo *Repo; signer *Signer; now func() time.Time }
func (s *Service) Activate(ctx context.Context, req ActivateRequest) (*ActivateResponse, error)
func (s *Service) Refresh(ctx context.Context, req RefreshRequest) (*RefreshResponse, error)
func (s *Service) Deactivate(ctx context.Context, req DeactivateRequest) error

// repo.go
func NewRepo(db *sql.DB, driver string) *Repo
func (r *Repo) CreateCards(ctx context.Context, cards []*Card) error
func (r *Repo) ListCards(ctx context.Context, f *FindCard) ([]*CardListItem, int, error)
func (r *Repo) GetCard(ctx context.Context, f *FindCard) (*Card, error)
func (r *Repo) RevokeCard(ctx context.Context, cardID string) error
func (r *Repo) ListBindings(ctx context.Context, cardID string) ([]*Binding, error)
func (r *Repo) Activate(ctx context.Context, p ActivateParams) (*ActivationResult, error)
func (r *Repo) Refresh(ctx context.Context, p RefreshParams) (*Card, error)
func (r *Repo) Deactivate(ctx context.Context, p DeactivateParams) error

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
func (u *UpdateStore) Save(ctx context.Context, version, notes, baseURL string, file io.Reader) (*Release, error)
func (u *UpdateStore) FilePath(name string) (string, error)
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
func (s *Service) RegisterRoutes(e *echo.Echo)

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

### 12.2 任务清单

按顺序实施，每个任务结束都能独立编译、测试。

#### T1 数据层（迁移 + Repo）
- 文件：`store/migration/{sqlite,mysql,postgres}/0.31/00__license.sql`、三份 `LATEST.sql`、`internal/license/repo.go`、`internal/license/repo_test.go`
- 实现：
  - 三张表，`key_hash` 内联 `UNIQUE`（避免 MySQL 不支持 `CREATE INDEX IF NOT EXISTS`）
  - `Repo` 用 `driver` 字段切换占位符：postgres 重写 `?` 为 `$n`；统一 `rebind()` helper
  - `Activate` 的事务：sqlite 用 `*sql.Conn` + `BEGIN IMMEDIATE`；mysql/postgres 用 `BeginTx` + `SELECT ... FOR UPDATE`
  - `ListCards` 用 `LEFT JOIN license_bindings` 返回 `deviceCount`
- 验收：
  - 内存 SQLite 建表、唯一 hash 冲突、CRUD 通过
  - 并发 20 个 goroutine 激活 `max_devices=1` 的卡，只有 1 个成功
  - `go test ./internal/license/... ./store/...`

#### T2 令牌与卡密
- 文件：`internal/license/token.go`、`cardkey.go` 及测试
- 实现：
  - `RawURLEncoding` 无填充；签名对象是 payload 的 base64 字符串
  - `Verify` 校验结构、签名、`v==1`、`plan` 白名单、`iat>0`、`exp==0 || exp>iat`；到期判断用 `now.Unix() >= exp`
  - 卡密字母表去掉 `0/O/1/I/L`，16 位分 4 组
- 验收：
  - roundtrip、篡改 payload/签名被拒、lifetime `exp=0`、过期返回可区分错误
  - 规范化 `" xhl-abcd-... "` 与 `"XHL-ABCD-..."` 哈希一致

#### T3 配置与更新包存储
- 文件：`internal/license/config.go`、`update.go` 及测试
- 实现：
  - 私钥：PEM `BEGIN PRIVATE KEY` → `x509.ParsePKCS8PrivateKey`；否则 base64 解码，接受 32 字节 seed 或 64 字节私钥
  - `UpdateStore.Save`：临时文件 → 流式 `io.Copy` 到 `io.MultiWriter(file, hasher)` → `Sync` → `Rename`；删除旧 `.exe`；原子写 `windows.json`
  - `downloadUrl` 由调用方传入 baseURL，校验 HTTPS/回环
- 验收：
  - PEM、标准 base64、URL base64 三种私钥输入都能加载
  - 上传后 sha256 与 `sha256sum` 一致；`windows.json` 字段完整
  - `FilePath("../x")`、`"/etc/passwd"` 被拒

#### T4 领域服务
- 文件：`internal/license/service.go`、`internal/license/service_test.go`
- 实现：按 §3 的 Activate/Refresh/Deactivate 流程；错误统一返回 `*license.Error`
- 验收：契约 §8 场景 1~7 在领域层全部通过，另加：
  - 解绑幂等
  - 解绑后 refresh → `revoked`
  - refresh 保留 `plan`/`exp`，只更新 `iat`/`nonce`
  - 卡密自身过期与令牌过期分别映射 `card_expired` / `token_expired`

#### T5 客户端契约路由
- 文件：`server/router/license/service.go`、`service_test.go`
- 实现：
  - 6 条路由；JSON 响应/错误严格按契约
  - `http.MaxBytesReader` 64 KiB；`XHL_RATE_LIMIT_PER_MINUTE` 固定窗口限流，health 豁免
  - 未配置私钥时除 health 外返回 503
- 验收：
  - 表驱动测试覆盖全部 error code 与 HTTP 状态
  - 连续第 11 次请求返回 429
  - `GET /v1/health` 返回 UTC Unix 秒

#### T6 管理路由
- 文件：`server/router/license/admin.go`、`admin_test.go`
- 实现：
  - group 中间件 `requireAdmin`：`AuthenticateToUser` → nil 401 → `Role != store.RoleAdmin` 403
  - 卡密分页/搜索/状态筛选；创建批次事务；撤销；绑定列表；解绑（删绑定 + nonce 入撤销）
  - 上传：`ParseMultipartForm` + 字段校验 + `UpdateStore.Save`
- 验收：
  - 匿名 401、普通用户 403、admin 200
  - 创建响应含明文卡密，二次查询不再返回明文
  - 解绑后对应令牌 refresh 失败
  - 上传后 `GET /v1/update/windows.json` 可读到新内容

#### T7 服务装配
- 文件：`server/server.go`、`server/router/frontend/frontend.go`
- 实现：
  - `NewServer` 中加载配置、构造 repo/service/update store、注册两组路由
  - `shouldSkipFrontendStatic` 增加 `hasPathPrefix(path, "/v1") || hasPathPrefix(path, "/admin")`
  - 数据目录 `<Profile.Data>/license/updates` 启动时 `MkdirAll`
- 验收：
  - `go run ./cmd/memos` 后 curl `/v1/health`；访问 `/setting` 前端不受影响
  - 不配置私钥也能正常启动

#### T8 前端 API 与 hooks
- 文件：`web/src/utils/license-api.ts`、`web/src/hooks/useLicenseQueries.ts`
- 实现：
  - `request()` 自动取 token；401 时 `refreshAccessToken()` 后重试一次
  - 错误类 `LicenseAPIError { code, message, status }`
  - `uploadUpdate` 用 `FormData`，不手写 Content-Type
  - React Query key 工厂 + query/mutation；mutation 后 invalidate
- 验收：`cd web && pnpm lint`；401 重试用 Vitest 单测覆盖

#### T9 管理页面与 i18n
- 文件：
  - `web/src/components/Settings/settingSections.ts`
  - `web/src/components/Settings/LicenseSection.tsx`
  - `web/src/components/Settings/license/LicenseCardsPanel.tsx`
  - `web/src/components/Settings/license/CreateLicenseCardsDialog.tsx`
  - `web/src/components/Settings/license/LicenseBindingsDialog.tsx`
  - `web/src/components/Settings/license/LicenseUpdatePanel.tsx`
  - `web/src/locales/{en,zh-Hans,zh-Hant,en-GB}.json`
- 页面：
  - 卡密 tab：状态条、筛选（全部/有效/已撤销/已过期）、搜索、分页表格、批量创建、结果复制/CSV、撤销、绑定查看/解绑
  - 版本 tab：当前版本卡片（版本/说明/sha256/downloadUrl/发布时间）、上传表单
- 验收：
  - 仅 ADMIN 能在 `/setting#license` 看到入口
  - 全流程可走通：建卡 → 撤销 → 查看绑定 → 解绑 → 上传版本
  - `pnpm lint && pnpm test && pnpm build`

#### T10 端到端与安全复核
- 用临时私钥启动服务，跑通：
  1. admin 建卡
  2. `/v1/activate` 拿令牌 → 本地用公钥验签
  3. 同设备再次 activate，绑定数不变
  4. 换设备触发 `device_limit`
  5. `/v1/refresh` 的 `exp` 不变、`nonce` 变化
  6. `/v1/deactivate` 后 refresh 返回 `revoked`
  7. 上传安装包，`windows.json` 的 sha256 与 `sha256sum` 一致
  8. 连续请求触发 429
- 检查日志中无明文卡密/token/deviceId
- 运行：
  ```bash
  go test ./internal/license/... ./server/router/license/... ./store/...
  go test -v -race ./server/... ./internal/...
  golangci-lint run
  cd web && pnpm lint && pnpm test && pnpm build
  ```

### 12.3 契约场景 → 测试映射

| 契约 §8 场景 | 测试位置 |
|---|---|
| 1 首次激活令牌可验签、`sub==deviceId` | `internal/license/service_test.go` |
| 2 同设备重复激活不增绑定 | 同上 |
| 3 新设备超限 `409 device_limit` | 同上 + repo 并发测试 |
| 4 卡密过期 `403 card_expired` | 同上 |
| 5 撤销后激活/刷新失败 | 同上 |
| 6 refresh 同 exp、新 iat/nonce | 同上 |
| 7 篡改令牌 `401 invalid_token` | `token_test.go` + HTTP 测试 |
| 8 health | `server/router/license/service_test.go` |

### 12.4 交付顺序建议

先合入 T1~T4（纯后端、可单测），再合入 T5~T7（接口可用、前端尚未接入），最后 T8~T10（页面与端到端）。每阶段单独提交，提交信息用简洁英文，例如：

- `feat: add license card storage and Ed25519 token`
- `feat: add license client contract endpoints`
- `feat: add admin license API`
- `feat(web): add license admin settings section`
