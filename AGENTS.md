# AGENTS.md

## Project

`inl` 是面向 AI Agent 的无头工业以太网 CLI 工具。通过 TCP:6000 与运行 `nrc2.out` 的工业 PC 通信，基于 **Cobra 命令树** + `internal/nrc/commands.go` 的 24 条 Registry 实现 PROFINET GSD、拓扑、DCP 设备发现与参数分配。

**AI Agent 推荐入口**：先用 `inl schema list` 自发现所有命令元数据，再读 `skills/inl-shared/SKILL.md` 了解共享规则。

## Source Layout

| Path | Purpose |
|------|---------|
| `main.go` | Cobra 命令树入口：6 个 Group 自动遍历 Registry，Risk 检查，schema 短路，Envelope 输出 |
| `internal/nrc/commands.go` | 命令元数据中心：24 条 Registry + 5 个 DCP BodyBuilder（新增命令只需追加一行） |
| `internal/nrc/client.go` | TCP 客户端：5s 连接超时，10s 读写超时，`SendReceiveFiltered` 按 command code 过滤异步推送 |
| `internal/nrc/frame.go` | NRC 帧编解码：SyncByte 0x4E66 + CRC32 IEEE 802.3 |
| `internal/output/envelope.go` | stdout 统一信封：`{ok, data, _notice}` |
| `internal/output/errors.go` | 结构化错误：`type/code/message/hint/detail`，AI 可解析 |
| `internal/output/dryrun.go` | DryRunFrame 预览：构建 NRC 帧不发送 |
| `internal/gsd/` | DataType=13/16 响应模型 |
| `internal/topology/` | DataType=12/14 (CallBackJson / Activated / Scan) 响应模型 |
| `internal/dcpdevice/` | DCP 发现设备模型（9 字段 + Block 映射） |
| `internal/netiface/` | 网络端口列表模型 |
| `internal/devicestatus/` | GetActRun 焊机状态模型 |
| `internal/gsdfile/` | GSDML 文件响应模型 |
| `docs/protocol/field-verification.md` | 实机响应反向核对记录（唯一权威偏差登记处） |
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
go build -o inl.exe .    # Build
go vet ./...              # Static analysis
go test ./...             # All unit tests (9 packages)
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

# 5. Risk display in help
inl config compile --help   # shows "Risk: high-risk-write"
```

Each run saves raw response to `<name>_response_<timestamp>.json`.

## Where to Find More

| Topic | Document |
|-------|---------|
| Architecture & design | [`docs/inl-architecture.md`](docs/inl-architecture.md) |
| Product requirements | [`docs/inl-prd.md`](docs/inl-prd.md) |
| Workflow design | [`docs/inl-workflow-design.md`](docs/inl-workflow-design.md) |
| DataType=12 protocol contract | [`docs/inl-architecture.md`](docs/inl-architecture.md#44-命令--datatype-映射) |
| DataType=14 DCP protocol | [`docs/inl-architecture.md`](docs/inl-architecture.md#44-命令--datatype-映射) |
| Known deviations | [`docs/protocol/field-verification.md`](docs/protocol/field-verification.md) |
| AI Skills | [`skills/inl-shared/SKILL.md`](skills/inl-shared/SKILL.md) |
| NRC frame spec | [`internal/nrc/frame.go`](internal/nrc/frame.go) — BuildFrame/ReadFrame |
| GSD response model | [`internal/gsd/types.go`](internal/gsd/types.go) — 8 structs with Chinese comments |
