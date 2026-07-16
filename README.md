# apifox-api-go

Apifox OpenAPI 原生 CLI（Go）。支持 Project Binding、离线快照检索、TypeScript 类型生成。

- 仓库：https://github.com/akirousnow/apifox-api-go
- 二进制名：`apifox-api`
- 模块路径：`github.com/akirousnow/apifox-api-go`
- 纯原生可执行文件，**不依赖 Node / Bun**

发布与多平台构建说明见 [RELEASE.md](./RELEASE.md)。

## 安装

### 方式一：`go install`（推荐，需 Go 1.25+）

```bash
# 安装最新已发布版本（有 tag 后）
go install github.com/akirousnow/apifox-api-go@latest

# 或指定版本
go install github.com/akirousnow/apifox-api-go@v0.1.0
```

安装后确保 `$GOPATH/bin` 或 `$HOME/go/bin` 在 `PATH` 中：

```bash
apifox-api version
# apifox-api <semver> (commit <sha>)
```

### 方式二：从源码构建

```bash
git clone https://github.com/akirousnow/apifox-api-go.git
cd apifox-api-go

go build -o apifox-api \
  -ldflags "-X github.com/akirousnow/apifox-api-go/internal/buildinfo.Version=0.1.0 -X github.com/akirousnow/apifox-api-go/internal/buildinfo.Commit=$(git rev-parse --short HEAD)" \
  .
./apifox-api version
```

### 方式三：下载 Release 产物

从 [GitHub Releases](https://github.com/akirousnow/apifox-api-go/releases) 下载对应平台二进制：

| 平台 | 路径 |
|------|------|
| Linux amd64 | `linux-amd64/apifox-api` |
| Linux arm64 | `linux-arm64/apifox-api` |
| macOS amd64 | `darwin-amd64/apifox-api` |
| macOS arm64 | `darwin-arm64/apifox-api` |
| Windows amd64 | `windows-amd64/apifox-api.exe` |
| Windows arm64 | `windows-arm64/apifox-api.exe` |

校验：

```bash
sha256sum -c checksums.txt
```

### npm / npx 兼容（过渡期）

现有 TypeScript 包在 **一个完整稳定周期内** 仍可使用：

```bash
npm install -g apifox-api
npx apifox-api <command>
```

Go 二进制是推荐的新安装路径；npm 入口用于平滑迁移，详见 [RELEASE.md](./RELEASE.md)。

## 快速开始

```bash
# 1) 配置 Auth Key（二选一）
export APIFOX_AUTH_KEY='your-apifox-access-token'
# 或写入全局默认：
apifox-api config set-auth-key 'your-apifox-access-token'

# 2) 在项目根目录绑定 Apifox 项目
apifox-api init <projectId>
# 多模块示例：
apifox-api init 6307449 --moduleIds 5,8,12 --authKey 'your-token'

# 3) 拉取 / 刷新 OpenAPI 快照
apifox-api refresh

# 4) 搜索接口
apifox-api search pets
apifox-api search --method POST
apifox-api search pets dog --mode and --limit 10
apifox-api search pets --json

# 5) 生成 TypeScript 类型（stdout 纯 TS）
apifox-api get GET /users/{id}
apifox-api get /users/{id} --method GET
apifox-api get /users/{id}              # 该 path 下全部 method
```

## 命令一览

```text
apifox-api
├── version
├── init <projectId> [--moduleIds ...] [--authKey ...]
├── config
│   └── set-auth-key <token>
├── module [moduleId]
├── search [keywords...] [--method] [--mode] [--limit] [--json] [--moduleId]
├── get [method] [path] [--method] [--moduleId]
└── refresh
```

全局：

| 选项 | 说明 |
|------|------|
| `-h, --help` | 帮助 |
| `-v, --version` | 版本，与 `apifox-api version` 相同 |

成功输出写 **stdout**，错误 / 过期警告写 **stderr**。退出码：`0` 成功，`1` 失败。

---

### `version`

```bash
apifox-api version
# apifox-api <semver> (commit <sha>)
```

---

### `init`

把 **当前工作目录** 绑定到一个 Apifox `projectId`，写入全局注册表 `~/.apifox-api.json`。

```bash
apifox-api init <projectId> [--moduleIds 5,8,12] [--authKey <token>]
```

| 参数 / 标志 | 说明 |
|-------------|------|
| `<projectId>` | 必填，Apifox 项目 ID |
| `--moduleIds` | 逗号分隔的正整数模块 ID；省略 = 仅默认模块 |
| `--authKey` | 本 binding 的 Access Token；省略则回退环境变量等 |

Auth 优先级（init）：`--authKey` → `APIFOX_AUTH_KEY` → 已有 binding → 全局 key（仅预取，不落盘）。

多模块时会写入 `.current-module`（取第一个 moduleId）。

---

### `config set-auth-key`

设置全局默认 Auth Key（写入 `~/.apifox-api.json`）。输出前后 fingerprint，不打印明文。

```bash
apifox-api config set-auth-key <token>
```

---

### `module`

查看或切换当前模块（多模块 workspace）。

```bash
apifox-api module           # 显示当前 module 与已绑定 moduleIds
apifox-api module 8         # 切换到 module 8（须在 binding 的 moduleIds 内）
```

---

### `search`

从本地 Snapshot 缓存离线检索接口（缓存未命中 / 过期时可联网刷新；失败时允许 stale 结果，警告在 stderr）。

```bash
apifox-api search [keywords...] [flags]
```

| 标志 | 默认 | 说明 |
|------|------|------|
| `--method` | `""` | HTTP method 过滤（`GET` / `POST` / …） |
| `--mode` | `or` | 关键词组合：`or` \| `and` |
| `--limit` | `20` | 结果窗口，有效范围 1–50 |
| `--json` | `false` | 机器可读 JSON |
| `--moduleId` | — | 一次性覆盖当前 module，不改写 `.current-module` |

规则：必须提供 **keywords** 和/或 **`--method`**。

示例：

```bash
apifox-api search pets
apifox-api search --method POST
apifox-api search pets dog --mode and --limit 10
apifox-api search pets --json --moduleId 5
```

JSON 字段概要：`total`, `showing`, `truncated`, `limit`, `module`, `stale`, `items[]`（含 `method`, `path`, `summary`, `tags`, `operationId`）。

---

### `get`

为接口生成 TypeScript 类型（stdout 为纯 TS）。

```bash
apifox-api get <method> <path> [--moduleId N]
apifox-api get <path> --method METHOD [--moduleId N]
apifox-api get <path> [--moduleId N]    # path 下全部 method
```

| 标志 | 说明 |
|------|------|
| `--method` | HTTP method（与位置参数 method 二选一，不可同时用） |
| `--moduleId` | 一次性 module 覆盖 |

示例：

```bash
apifox-api get GET /users/{id}
apifox-api get /users/{id} --method GET
apifox-api get /users/{id}
apifox-api get /users/{id} --moduleId 5
```

找不到接口时，可先用 `apifox-api search <关键词>` 定位。

---

### `refresh`

强制刷新 **所有** 已绑定 module 的 OpenAPI 快照。需要可用 Auth Key；远程失败时 **不** 回退 stale。

```bash
apifox-api refresh
```

## 配置与环境

### 环境变量

| 变量 | 说明 |
|------|------|
| `APIFOX_AUTH_KEY` | 运行时 Auth（最高优先级） |
| `APIFOX_MCP_OPENAPI_TTL_MS` | 快照新鲜度 TTL（毫秒），默认 `86400000`（24h） |

运行时 Auth 优先级：`APIFOX_AUTH_KEY` → binding `authKey` → 全局 registry `authKey`。

### 文件布局

| 路径 | 作用 |
|------|------|
| `~/.apifox-api.json` | 全局注册表（schema v1：全局 authKey + bindings） |
| `<binding-root>/.current-module` | 多模块当前 module |
| `<binding-root>/.cache/apifox-api/` | OpenAPI 快照缓存 |

Binding 解析：从 CWD 向上 walk-up 到 home / 文件系统根。

**请勿**把 Auth Key 写进源码、ldflags、SBOM、CI 日志或 provenance。

## 开发

```bash
# 测试
go test ./...

# 本地构建
go build -o apifox-api .

# 多平台发布构建（见 RELEASE.md）
export VERSION=0.1.0
export COMMIT="$(git rev-parse --short HEAD)"
export GOTOOLCHAIN=go1.26.5
./scripts/release.sh
./scripts/smoke.sh dist/release/linux-amd64/apifox-api
```

## License

见仓库 License 文件（若有）。
