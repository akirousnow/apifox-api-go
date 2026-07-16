# CLAUDE.md — apifox-api-go

Agent / 协作者入口：项目是什么、怎么开发、怎么发版。  
细节以 `README.md`（CLI 用法）和 `RELEASE.md`（构建与产物）为准；本文是精简操作手册。

---

## 项目介绍

| 项 | 值 |
|----|-----|
| 仓库 | https://github.com/akirousnow/apifox-api-go |
| 模块路径 | `github.com/akirousnow/apifox-api-go` |
| 二进制 | `apifox-api` |
| 语言 | Go（module min `go 1.25.0`；**发布构建钉 Go 1.26.5**） |
| 运行时 | 纯原生可执行文件，**不依赖 Node / Bun** |

**做什么：** Apifox OpenAPI CLI 的 Go 实现。

- Project Binding：把工作区目录绑到 Apifox `projectId`（全局注册表 `~/.apifox-api.json`）
- 离线优先：OpenAPI 快照缓存在 `<binding>/.cache/apifox-api/`
- 搜索接口、`get` 生成 TypeScript 类型、`refresh` 强制拉远程

**命令树：**

```text
apifox-api
├── version
├── init <projectId> [--moduleIds ...] [--authKey ...]
├── config set-auth-key <token>
├── module [moduleId]
├── search [keywords...] [--method] [--mode] [--limit] [--json] [--moduleId]
├── get [method] [path] [--method] [--moduleId]
└── refresh
```

**包布局（概要）：**

```text
main.go                 # 入口
internal/cli/           # Cobra 命令与进程契约
internal/binding/       # 注册表、auth、current module
internal/snapshot/      # 缓存路径、加载、远程 export
internal/openapi/       # 索引、搜索、排序
internal/typesgen/      # OpenAPI → TypeScript
internal/buildinfo/     # Version / Commit（ldflags 注入）
scripts/release.sh      # 六平台交叉编译
scripts/smoke.sh        # --version / version / --help
.github/workflows/release.yml  # 推 release 分支自动发版
```

**安全：** 永远不要把 `APIFOX_AUTH_KEY` / Access Token 写进源码、ldflags、SBOM、provenance 或 CI 日志。

完整用户文档见 [README.md](./README.md)。

---

## 本地开发

```bash
# 测试
go test ./...

# 本地构建
go build -o apifox-api .

# 带版本信息
go build -o apifox-api \
  -ldflags "-X github.com/akirousnow/apifox-api-go/internal/buildinfo.Version=0.1.0 -X github.com/akirousnow/apifox-api-go/internal/buildinfo.Commit=$(git rev-parse --short HEAD)" \
  .
```

常用环境变量：

| 变量 | 说明 |
|------|------|
| `APIFOX_AUTH_KEY` | 运行时 Auth（最高优先级） |
| `APIFOX_MCP_OPENAPI_TTL_MS` | 快照 TTL（毫秒），默认 24h |

---

## 后续发版（推荐流程）

自动化入口：[`.github/workflows/release.yml`](./.github/workflows/release.yml)。

### 一句话

**在 `release` 分支写好根目录 `VERSION`（无 `v` 前缀）并 push → Actions 构建六平台包并创建 GitHub Release。**

### 步骤

```bash
# 1) 确保 main 已包含要发布的改动
git checkout main
git pull

# 2) 切到 / 更新 release 分支
git checkout -B release
# 或: git checkout release && git merge main

# 3) 写入版本号（不要带 v）
echo '0.1.1' > VERSION
git add VERSION
# 若有其它发版相关改动一并 add
git commit -m "release: v0.1.1"

# 4) 推送触发 CI
git push -u origin release
```

完成后查看：

- Actions: https://github.com/akirousnow/apifox-api-go/actions  
- Releases: https://github.com/akirousnow/apifox-api-go/releases  

### 版本如何解析（CI）

优先级：

1. `workflow_dispatch` 输入的 `version`
2. 推送的 tag `v*` → 去掉 `v`
3. 根目录 `VERSION` 文件（推 `release` 分支时的主路径）
4. HEAD 恰好等于某个 tag
5. 否则 `dev+<shortsha>`：**只构建 + artifact，不创建 Release**

含 `-` 的版本（如 `0.1.0-rc.1`）会标成 **prerelease**。

### CI 会做什么

1. Go **1.26.5**
2. `./scripts/release.sh` → `dist/release/`（6 平台 + checksums + SBOM + provenance）
3. `./scripts/smoke.sh dist/release/linux-amd64/apifox-api`
4. 打包到 `dist/assets/`（tar.gz / zip + checksums-archives）
5. 上传 workflow artifact
6. 有明确版本时：`gh release create/upload` → `v{VERSION}`  
   - notes：`--generate-notes`（上一 tag → 当前 target；**不必**改成手写 main commit list，除非以后有痛点）
   - **不要**在 workflow 里 `git push` tag（避免二次触发）；`gh release create --target` 会处理 tag

### 备选触发

```bash
# 打 tag 推送（也可发版）
git tag v0.1.1
git push origin v0.1.1

# 或 GitHub UI: Actions → Release → Run workflow → 填 version
```

### 本地发版构建（不上传 GitHub）

```bash
export GOTOOLCHAIN=go1.26.5
export VERSION=0.1.1
export COMMIT="$(git rev-parse --short HEAD)"
./scripts/release.sh
./scripts/smoke.sh dist/release/linux-amd64/apifox-api
```

产物说明与校验见 [RELEASE.md](./RELEASE.md)。

### npm 过渡

首个稳定 Go 周期内仍保留 npm/npx 的 TS 包 `apifox-api` 作兼容桥；细节见 `RELEASE.md`。本仓库只负责 **原生二进制** 发版。

---

## Agent 约定（改代码时）

- 模块 import 路径必须是 `github.com/akirousnow/apifox-api-go/...`
- 改 CLI 行为时同步考虑：stdout 成功 / stderr 错误与 stale 警告 / 退出码 0|1
- 发布相关改动：优先改 `scripts/release.sh` + workflow，并更新 `RELEASE.md` / 本文「后续发版」
- 不要提交 secrets；`.cache/`、`dist/` 已在 `.gitignore`
