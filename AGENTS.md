# AGENTS.md

## Project

`inl` 是面向 AI Agent 的无头工业以太网 CLI 工具。通过 TCP:6000 与运行 `nrc2.out` 的工业 PC 通信，基于 **Cobra 命令树** + `internal/nrc/commands.go` 的 25 条 Registry 实现 PROFINET GSD、拓扑、DCP 设备发现与参数分配。

**AI Agent 推荐入口**：先用 `inl schema list` 自发现所有命令元数据，再读 `skills/inl-shared/SKILL.md` 了解共享规则。

## Source Layout

| Path | Purpose |
|------|---------|
| `main.go` | Cobra 命令树入口：7 个 Group 自动遍历 Registry，Risk 检查，schema 短路，Envelope 输出，`--format table/csv/ndjson` 多格式分支 + `device setup` 组合命令 |
| `internal/nrc/commands.go` | 命令元数据中心：25 条 Registry + 5 个 DCP BodyBuilder + 1 个 raw-send + 1 个 configSetIDeviceBody (Step 9.1) + 10 个 config 写命令 BodyBuilder (Step 10.A) |
| `internal/configresp/` | DataType 写命令的响应模型 (Step 10.A v3): `WriteResponse` (Error 用 `json.RawMessage` 容忍 string/[]string) + `ShieldDeviceResponse` (v3 CallbackNTJson 模板: DataType/Function(string 标签)/DeviceName/Result(bool) 全在 root 顶层) + 12 单测 |
| `internal/nrc/config_body.go` | Step 10.A 10 个 config 写命令 BodyBuilder + 通用 `configBodyBuilder` 辅助 + `fetchTopologyForConfig` (可注入 mock) + `validateRequiredFields` 必填校验 + v3 CallbackNTJson 模板 (拼写已纠正: ShieldDevice/UNShieldDevice) |
| `internal/nrc/client.go` | TCP 客户端: 通过 `reliability.Policy.Do` 注入重试逻辑, 默认 Connect 3 次 / DCP 1 次, `WithConnectPolicy` / `WithReceivePolicy` 可注入, `NewReceivePolicy(n)` 公开 API (供 `--retry` flag) |
| `internal/nrc/frame.go` | NRC 帧编解码：SyncByte 0x4E66 + CRC32 IEEE 802.3 |
| `internal/reliability/` | 抽出的重试策略子包 (Step 9.2): `Policy{ MaxRetries, BaseBackoff, MaxBackoff, Jitter, ShouldRetry }` + `Default()` + `ShouldRetryNetwork` |
| `internal/output/envelope.go` | stdout 统一信封：`{ok, data, _notice}` |
| `internal/output/errors.go` | 结构化错误：`type/code/message/hint/detail`，AI 可解析 |
| `internal/output/dryrun.go` | DryRunFrame 预览：构建 NRC 帧不发送（支持 DCP 写，Step 9.4 验证） |
| `internal/output/table.go` | `--format table` 表格渲染：`ExtractTableRows` 探测首个非空对象数组 + `FormatTable` 对齐输出 |
| `internal/output/csv.go` | `--format csv` CSV 渲染 (Step 9.4): RFC 4180 标准, 自动转义逗号/引号/换行 |
| `internal/output/ndjson.go` | `--format ndjson` NDJSON 渲染 (Step 9.4): 适合 jq / 管道处理, 第一行为 header |
| `internal/gsd/` | DataType=13/16 响应模型 |
| `internal/topology/` | DataType=12/14 (CallBackJson / Activated / Scan) 响应模型 |
| `internal/dcpdevice/` | DCP 发现设备模型（9 字段 + Block 映射） |
| `internal/netiface/` | 网络端口列表模型 |
| `internal/devicestatus/` | GetActRun 焊机状态模型 |
| `internal/gsdfile/` | GSDML 文件响应模型（`Response.Error` 字段已建模，Step 9.2） |
| `docs/protocol/field-verification.md` | 实机响应反向核对记录（唯一权威偏差登记处） |
| `npm/` | npm 薄壳包 (Step 9.3): 1 入口包 `inl-cli` + 6 平台子包 `@inl/cli-{linux,darwin,win32}-{x64,arm64}` |
| `.goreleaser.yaml` | 跨平台二进制构建配置 (Step 9.3): 6 平台 × {tar.gz, zip} |
| `.github/workflows/` | `release.yml` (GoReleaser 自动发布) + `npm-publish.yml` (npm 7 包发布) |
| `skills/` | AI Agent Skill 文档（inl-shared + inl-workflow-profinet-write） |

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
go test ./...                                      # All unit tests (10 packages, Step 9.2 加 reliability)
goreleaser check                                   # 验证 .goreleaser.yaml 语法
goreleaser release --snapshot --clean --skip publish  # 本地 snapshot 构建 (不推送)
```

Dependency: `github.com/spf13/cobra v1.10.2` (only external dependency). All `internal/` packages are zero-dependency.

## Run

```bash
# 0. AI self-discovery
inl schema list

# 1. Read GSD drivers (read, no --yes needed)
inl --target 192.168.3.15 gsd list

# 2. DCP discover online devices
inl --target 192.168.3.15 topology scan --interface enp4s0

# 3. Write config (requires --yes)
inl --target 192.168.3.15 config add-device --yes

# 4. Compile & activate (high-risk, requires --yes)
inl --target 192.168.3.15 config compile --yes

# 5. Set iDevice IO length (Step 9.1 参数化)
inl --target 192.168.3.15 config set-idevice \
    --data '{"Activate":true,"InputLength":64,"OutputLength":64}' --yes

# 6. 12 条 config 写命令 (Step 10.A 参数化)
inl --target 192.168.3.15 config set-driver \
    --data '{"DeviceName":"profinetdriver","IPAddress":"192.168.3.15","SubnetMask":"255.255.255.0","SetInTheProject":true}' --yes

# 6. 一键 DCP 设置设备 (Step 9.4 组合命令)
inl --target 192.168.3.15 device setup \
    --interface enp4s0 --mac 00:11:22:33:44:55 --name heron-weld \
    --ip 192.168.2.10 --mask 255.255.255.0 --yes

# 6.x. 12 条 config 写命令 (Step 10.A 已参数化, Step 10.B 待实机端到端验证)

# 7. Risk display in help
inl config compile --help   # shows "Risk: high-risk-write"
```

Each run saves raw response to `<name>_response_<timestamp>.json`.

### 网络抖动场景: `--retry`

```bash
# 默认 1 次重试; 网络不稳时加大到 3-5
inl --target 192.168.3.15 --retry 3 topology scan --interface enp4s0
```

`--retry N` 全局 flag 控制 DCP 命令的最大重试次数 (Step 9.2 引入)。重试逻辑由 `internal/reliability.Policy` 实现, 指数退避 + 抖动 + 可配置的 `ShouldRetry` 触发条件。

## Output Formats

| Format | Stdout | Use case |
|--------|--------|----------|
| `json` (默认) | `Envelope {ok, data, _notice}` | AI 消费、pipe 链、脚本处理 |
| `table` | 对齐的固定宽度表格 | FAE 现场肉眼读、不走 Envelope |
| `csv` | RFC 4180 CSV | Excel 处理、awk/grep |
| `ndjson` | NDJSON (header 数组 + 对象行) | jq / 管道处理 |

### `--format table/csv/ndjson` 用法

```bash
# 表格 (FAE 现场)
inl topology scan --interface pnio1 --target 192.168.3.15 --format table

# CSV (Excel / awk)
inl topology scan --interface pnio1 --target 192.168.3.15 --format csv > devices.csv

# NDJSON (jq 消费)
inl topology scan --interface pnio1 --target 192.168.3.15 --format ndjson | jq -c 'select(.Mac)'

# 无数组响应 (如 device list) 自动回退 JSON + stderr 警告
inl device list --target 192.168.3.15 --format table
# stderr: ⚠️  --format table 不适用此命令, 回退 JSON 输出
```

探测算法: `output.ExtractTableRows` 深度优先搜索首个非空对象数组, 提取公共字段作为列。
列宽自适应 (min=8, max=40), 超长字符串截断为 `<prefix>...`。
nil / 缺失字段显示为 `-` (table) / `""` (csv/ndjson)。

### `--dry-run` (DCP 写支持)

```bash
# DCP 写也支持 --dry-run: 构造帧 + 打印 + 跳过发送
inl --target 192.168.3.15 device setup-name \
    --interface enp4s0 --mac 00:11:22:33:44:55 --name test --dry-run
# stdout: DryRunFrame JSON (16 进制 payload + CRC32)
# stderr: 🛑 --dry-run 模式: 已跳过连接和发送
```

## Distribution (Step 9.3)

```bash
# 1. GitHub Release: 推送 v* tag 触发 release.yml
git tag v0.1.0 && git push origin v0.1.0

# 2. npm: 入口包 + 6 平台子包
npm install -g inl-cli
inl --version   # 输出: inl version 0.1.0
```

分发渠道:
- **GitHub Release**: `tar.gz` + `zip` 6 平台二进制 + `checksums.txt` (GoReleaser 自动)
- **npm**: 1 入口包 `inl-cli` + 6 平台子包 (用户 `npm install` 自动选平台)
- **本地构建**: `go build` 静态编译单文件 ~5MB (仅依赖 cobra)

## Raw Send (透传)

`inl raw send --data '<json>'` 直接发送任意 JSON payload 到工业 PC, 兜底覆盖 24 条 Registry 外的边缘场景。
**默认 `write` 风险**, 需 `--yes` 才执行; `--data` 必须是合法 JSON。

```bash
# 等价于 `inl topology scan --interface enp4s0`
inl raw send --data '{"DataType":14,"Function":1,"Portname":"enp4s0"}' --yes
```

## Where to Find More

| Topic | Document |
|-------|---------|
| Architecture & design | [`docs/inl-architecture.md`](docs/inl-architecture.md) |
| Product requirements | [`docs/inl-prd.md`](docs/inl-prd.md) |
| Workflow design | [`docs/inl-workflow-design.md`](docs/inl-workflow-design.md) |
| Development plan (current) | [`docs/inl-step9-distribution-plan.md`](docs/inl-step9-distribution-plan.md) — Step 9 生产化与发布 |
| Development status | [`docs/inl-development-status.md`](docs/inl-development-status.md) |
| DataType=12 protocol contract | [`docs/inl-architecture.md`](docs/inl-architecture.md#44-命令--datatype-映射) |
| DataType=14 DCP protocol | [`docs/inl-architecture.md`](docs/inl-architecture.md#44-命令--datatype-映射) |
| Known deviations | [`docs/protocol/field-verification.md`](docs/protocol/field-verification.md) |
| AI Skills | [`skills/inl-shared/SKILL.md`](skills/inl-shared/SKILL.md) |
| NRC frame spec | [`internal/nrc/frame.go`](internal/nrc/frame.go) — BuildFrame/ReadFrame |
| GSD response model | [`internal/gsd/types.go`](internal/gsd/types.go) — 8 structs with Chinese comments |
| Reliability subpackage | [`internal/reliability/`](internal/reliability/) — Policy / Default / ShouldRetryNetwork |
