# AGENTS.md

## Project

`inl` 是面向 AI Agent 的无头工业以太网 CLI 工具。通过 TCP:6000 与运行 `nrc2.out` 的工业 PC 通信，基于 **Cobra 命令树** + `internal/nrc/commands.go` 的 25 条 Registry 实现 PROFINET GSD、拓扑、DCP 设备发现与参数分配。

**AI Agent 推荐入口**：先用 `inl schema list` 自发现所有命令元数据，再读 `skills/inl-shared/SKILL.md` 了解共享规则。

---

## 🔴 配置态 vs 运行态（AI 必读）

inl 面向两种完全不同的数据平面，**混用会导致不可逆的配置错误**：

| 维度 | 配置态（Configuration State） | 运行态（Runtime State） |
|------|-----------------------------|------------------------|
| **命令组** | `config` 组、`device list`、`device list-active` | `topology scan`、`device setup-name/ip` |
| **数据存储** | `networktopology.json`（工业 PC 端文件） | DCP Layer 2 广播（设备固件内） |
| **生效方式** | 需 `config compile` 推入运行时 | **直接生效**，跳过编译 |
| **验证命令** | `device list`（配置中） / `device list-active`（已激活） | `topology scan` |
| **典型操作** | 添加/删除/修改设备、设置驱动、设置 IDevice | 改设备名、改 IP、DCP 发现 |

### 核心规则

1. **`device list` 只看配置态** — 它返回 `networktopology.json` 的内容，不反映设备实际 DCP 参数
2. **`topology scan` 只看运行态** — 它通过 DCP Layer 2 广播扫描物理网络，不反映未编译的配置
3. **DCP 写命令（`device setup-name` / `setup-ip`）直接改运行态** — 写入后 `device list` 看不到变化，**必须用 `topology scan` 验证**
4. **`config compile` 将配置态推入运行态** — 编译后 `device list-active` 应与 `device list` 一致
5. **`config shield` / `unshield` 例外** — 属 config 命令组但运行时生效，不走 compile

> **严禁**用 `device list` 验证 DCP 写操作，**严禁**用 `topology scan` 判断配置是否已编译完成。
>
> 混用配置态/运行态命令验证对方的数据，是此项目最高频的 AI 错误类型，没有之一。

## Source Layout

| Path | Purpose |
|------|---------|
| `main.go` | Cobra 命令树入口：7 个 Group 自动遍历 Registry，Risk 检查，schema 短路，Envelope 输出，`--format table/csv/ndjson` 多格式分支 + `device setup` 组合命令 |
| `internal/nrc/commands.go` | 命令元数据中心：25 条 Registry + 5 个 DCP BodyBuilder + 1 个 raw-send + 1 个 configSetIDeviceBody + 10 个 config 写命令 BodyBuilder |
| `internal/configresp/` | DataType 写命令的响应模型: `WriteResponse` (Error 用 `json.RawMessage` 容忍 string/[]string) + `ShieldDeviceResponse` (v3 CallbackNTJson 模板: DataType/Function(string 标签)/DeviceName/Result(bool) 全在 root 顶层) + 12 单测 |
| `internal/nrc/config_body.go` | 10 个 config 写命令 BodyBuilder + 通用 `configBodyBuilder` 辅助 + `fetchTopologyForConfig` (可注入 mock) + `validateRequiredFields` 必填校验 + v3 CallbackNTJson 模板 (拼写已纠正: ShieldDevice/UNShieldDevice) |
| `internal/nrc/client.go` | TCP 客户端: 通过 `reliability.Policy.Do` 注入重试逻辑, 默认 Connect 3 次 / DCP 1 次, `WithConnectPolicy` / `WithReceivePolicy` 可注入, `NewReceivePolicy(n)` 公开 API (供 `--retry` flag) |
| `internal/nrc/frame.go` | NRC 帧编解码：SyncByte 0x4E66 + CRC32 IEEE 802.3 |
| `internal/reliability/` | 重试策略子包: `Policy{ MaxRetries, BaseBackoff, MaxBackoff, Jitter, ShouldRetry }` + `Default()` + `ShouldRetryNetwork` |
| `internal/output/envelope.go` | stdout 统一信封：`{ok, data, _notice}` |
| `internal/output/errors.go` | 结构化错误：`type/code/message/hint/detail`，AI 可解析 |
| `internal/output/dryrun.go` | DryRunFrame 预览：构建 NRC 帧不发送（支持 DCP 写 `--dry-run`） |
| `internal/output/table.go` | `--format table` 表格渲染：`ExtractTableRows` 探测首个非空对象数组 + `FormatTable` 对齐输出 |
| `internal/output/csv.go` | `--format csv` CSV 渲染: RFC 4180 标准, 自动转义逗号/引号/换行 |
| `internal/output/ndjson.go` | `--format ndjson` NDJSON 渲染: 适合 jq / 管道处理, 第一行为 header |
| `internal/gsd/` | DataType=13/16 响应模型 |
| `internal/topology/` | DataType=12/14 (CallBackJson / Activated / Scan) 响应模型 |
| `internal/dcpdevice/` | DCP 发现设备模型（9 字段 + Block 映射） |
| `internal/netiface/` | 网络端口列表模型 |
| `internal/devicestatus/` | GetActRun 焊机状态模型 |
| `internal/gsdfile/` | GSDML 文件响应模型（`Response.Error` 字段已建模） |
| `docs/protocol/field-verification.md` | 实机响应反向核对记录（唯一权威偏差登记处） |
| `npm/` | npm 薄壳包: 1 入口包 `inl-cli` + 6 平台子包 `@inl/cli-{linux,darwin,win32}-{x64,arm64}` |
| `.goreleaser.yaml` | 跨平台二进制构建配置: 6 平台 × {tar.gz, zip} |
| `.github/workflows/` | `release.yml` (GoReleaser 自动发布) + `npm-publish.yml` (npm 7 包发布) |
| `skills/` | AI Agent Skill 文档（inl-shared + inl-workflow-profinet-config + inl-workflow-profinet-dcp） |

## Code Conventions

### stdout is data, stderr is everything else

All successful commands output Envelope-wrapped JSON to stdout. Progress, warnings, hints go to stderr.

```bash
inl gsd list --target 192.168.3.15 > out.json 2> progress.log
# out.json:     {ok, data, _notice} Envelope
# progress.log: 🔌 / 📤 / 💾 / ⚠️
```

### Structured errors

`RunE` functions must return `*output.Error` (type/code/message/hint/detail) — never bare `fmt.Errorf`. AI agents parse stderr as JSON.

### Adding a new command

1. Append one `CommandSpec` to Registry in `internal/nrc/commands.go`
2. Set `Group` to route it to the right Cobra tree node
3. `main.go` **needs no changes** (auto-picks from Registry)
4. `init()` validates Name and DataType+Function uniqueness

### Tests

- Every behavior change needs a test
- Run `go test ./...` before committing
- **Test data files must go in `testdata/`** — no `.json` / `.log` / response dumps in the project root

## Build & Test

```bash
go build -o inl.exe .                              # Build
go vet ./...                                       # Static analysis
go test ./...                                      # All unit tests (10 packages)
goreleaser check                                   # 验证 .goreleaser.yaml 语法
goreleaser release --snapshot --clean --skip publish  # 本地 snapshot 构建 (不推送)
```

Dependency: `github.com/spf13/cobra v1.10.2` (only external dependency). All `internal/` packages are zero-dependency.

## Where to Find More

| Topic | Document |
|-------|---------|
| Architecture & design | [`docs/inl-architecture.md`](docs/inl-architecture.md) |
| Product requirements | [`docs/inl-prd.md`](docs/inl-prd.md) |
| Workflow design | [`docs/inl-workflow-design.md`](docs/inl-workflow-design.md) |
| Development status | [`docs/inl-development-status.md`](docs/inl-development-status.md) |
| DataType=12 protocol contract | [`docs/inl-architecture.md`](docs/inl-architecture.md#44-命令--datatype-映射) |
| DataType=14 DCP protocol | [`docs/inl-architecture.md`](docs/inl-architecture.md#44-命令--datatype-映射) |
| Known deviations | [`docs/protocol/field-verification.md`](docs/protocol/field-verification.md) |
| AI Skills | [`skills/inl-shared/SKILL.md`](skills/inl-shared/SKILL.md) |
| NRC frame spec | [`internal/nrc/frame.go`](internal/nrc/frame.go) — BuildFrame/ReadFrame |
| GSD response model | [`internal/gsd/types.go`](internal/gsd/types.go) — 8 structs with Chinese comments |
| Reliability subpackage | [`internal/reliability/`](internal/reliability/) — Policy / Default / ShouldRetryNetwork |
