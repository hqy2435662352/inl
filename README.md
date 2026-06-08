# inl — Industrial Netline CLI

[![npm version](https://img.shields.io/npm/v/inl-cli.svg)](https://www.npmjs.com/package/inl-cli)
[![GitHub release](https://img.shields.io/github/release/hqy2435662352/inl.svg)](https://github.com/hqy2435662352/inl/releases)

面向 AI Agent 的无头（Headless）工业以太网配置与诊断命令行工具。通过 TCP:6000 与运行 `nrc2.out` 的工业 PC 通信，实现 PROFINET 设备驱动管理、拓扑读写、DCP 设备发现与参数分配。

```bash
$ inl --target 192.168.3.15 topology scan --interface enp4s0
📤 发送 DataType=14, Function=1, Portname=enp4s0
📥 发现 3 台在线设备:
  [1] ex245          MAC: aa:bb:cc:dd:ee:ff  VendorID: 0x0083  Role: PN设备
  [2] heron-weld     MAC: 00:11:22:33:44:55  VendorID: 0x038A  Role: PN设备
  [3] scalance       MAC: 11:22:33:44:55:66  VendorID: 0x002A  Role: PN设备
```

## 安装

### 方式 1: npm（推荐，Node.js 生态）

```bash
npm install -g inl-cli
inl --version
# 输出: inl version 0.1.0
```

入口包 + 6 平台子包 (`@inl/cli-linux-x64`, `@inl/cli-darwin-arm64` 等) 自动按平台装。

### 方式 2: GitHub Release（CI 集成 / 容器镜像）

从 [Releases 页面](https://github.com/hqy2435662352/inl/releases) 下载对应平台的 tar.gz 或 zip：

```bash
# Linux x64 示例
curl -L https://github.com/hqy2435662352/inl/releases/download/v0.1.0/inl_0.1.0_linux_amd64.tar.gz | tar xzf -
./inl --version
```

### 方式 3: go install（开发环境）

```bash
go install github.com/hqy2435662352/inl/inl@latest
inl --version
```

### 方式 4: 本地构建

```bash
git clone https://github.com/hqy2435662352/inl.git
cd inl/inl
go build -o inl.exe .
```

## 设计原则

| 原则 | 说明 |
|------|------|
| **Agent-Native** | stdout = 结构化数据，stderr = 诊断信息。每步操作有明确的 in/out 数据 |
| **安全优先** | 三级 Risk（read/write/high-risk-write）+ `--dry-run` + `--yes` 强制确认 |
| **协议直达** | 25+1 条命令直接对应 C++ 端 `NetWorkTopologyFunction` 分发器 + `PerformOnlineAccess` DCP 操作 |
| **零依赖分发** | `go build` 静态编译单文件 ~5MB，仅依赖 cobra |

## 快速开始

```bash
# 0. AI 自发现
inl schema list

# 1. 读取 GSD 设备驱动列表（只读，无需 --yes）
inl --target 192.168.3.15 gsd list

# 2. 查看当前激活的 PROFINET 拓扑
inl --target 192.168.3.15 device list-active

# 3. DCP 发现在线设备
inl --target 192.168.3.15 topology scan --interface enp4s0

# 4. 写入配置（写命令必须加 --yes）
inl --target 192.168.3.15 config add-device --yes

# 5. 编译激活（高危，需 --yes）
inl --target 192.168.3.15 config compile --yes

# 6. 参数化 config 写命令
inl --target 192.168.3.15 config set-driver \
    --data '{"DeviceName":"profinetdriver","IPAddress":"192.168.3.15","SubnetMask":"255.255.255.0","SetInTheProject":true}' --yes

# 7. 一键 DCP 设置设备（名称+IP+验证）
inl --target 192.168.3.15 device setup \
    --interface enp4s0 --mac 00:11:22:33:44:55 --name heron-weld \
    --ip 192.168.2.10 --mask 255.255.255.0 --yes

# 8. 网络抖动时加大重试
inl --target 192.168.3.15 --retry 3 topology scan --interface enp4s0
```

## 命令总览（25 条 + 1 组合）

| Group | 命令 | 协议 | 风险 | 说明 |
|-------|------|------|:--:|------|
| **gsd** | `list` | DataType=13 | read | 列出 GSDML 设备驱动 |
| | `match` | DataType=16 | read | 匹配在线设备 ↔ GSD 驱动 |
| **device** | `list` | DataType=12, CallBackJson | read | 查看配置中的网络拓扑 |
| | `list-active` | DataType=12, CallBackActivatedJson | read | 查看激活中的网络拓扑 |
| | `run` | DataType=12, GetActRun | read | 查看活动运行设备状态 |
| | `gsd-config` | DataType=12, GetGSDFileNetwork | read | 查看配置中 GSD 文件 |
| | `gsd-active` | DataType=12, GetGSDFileActivated | read | 查看激活中 GSD 文件 |
| | `setup-name` | DataType=14, Func=2 | write | DCP 设置设备名称 |
| | `setup-ip` | DataType=14, Func=3 | write | DCP 设置设备 IP |
| | `setup` | 组合命令 | write | 一键 DCP 设置（名称+IP+验证） |
| **config** | `set-driver` | DataType=12, SetPNDriver | write | 设置主站参数 |
| | `add-device` | DataType=12, AddPNDevice | write | 添加分散设备 |
| | `remove-device` | DataType=12, UninstallPNDevice | write | 卸载分散设备 |
| | `set-device` | DataType=12, SetPNDevice | write | 设置设备参数 |
| | `set-idevice` | DataType=12, SetIDevice | write | 设置 iDevice IO 参数 |
| | `add-module` | DataType=12, AddModule | write | 添加模块 |
| | `remove-module` | DataType=12, UninstallModule | write | 卸载模块 |
| | `add-submodule` | DataType=12, AddSubmodule | write | 添加子模块 |
| | `remove-submodule` | DataType=12, UninstallSubmodule | write | 删除子模块 |
| | `shield` | DataType=12, ShieldDevice | write | 屏蔽设备 |
| | `unshield` | DataType=12, UNShieldDevice | write | 取消屏蔽 |
| | `compile` | DataType=12, Compile | **high-risk-write** | 编译并激活配置 |
| **topology** | `scan` | DataType=14, Func=1 | read | DCP 发现网络设备 |
| **interface** | `list` | DataType=14, Func=4 | read | 列出工业 PC 网络端口 |
| **schema** | `list` | 纯客户端 | read | AI 自发现所有命令元数据 |
| **raw** | `send` | 透传 | write | 直接发送任意 JSON payload |

## 输出格式

| Format | Stdout | 适用场景 |
|--------|--------|----------|
| `json`（默认） | `Envelope {ok, data, _notice}` | AI 消费、pipe 链、脚本处理 |
| `table` | 对齐的固定宽度表格 | FAE 现场肉眼读 |
| `csv` | RFC 4180 CSV | Excel 处理、awk/grep |
| `ndjson` | NDJSON（header 数组 + 对象行） | jq / 管道处理 |

```bash
# 表格输出（FAE 现场）
inl topology scan --interface enp4s0 --target 192.168.3.15 --format table

# CSV（Excel）
inl topology scan --interface enp4s0 --target 192.168.3.15 --format csv > devices.csv

# NDJSON（jq）
inl topology scan --interface enp4s0 --target 192.168.3.15 --format ndjson | jq -c 'select(.Mac)'

# --dry-run（预览请求帧不发送）
inl --target 192.168.3.15 device setup-name \
    --interface enp4s0 --mac 00:11:22:33:44:55 --name test --dry-run

# 透传命令
inl raw send --data '{"DataType":14,"Function":1,"Portname":"enp4s0"}' --yes
```

## 系统架构

```
┌── 上位机：调试 PC (AI 舞台) ─────┐      ┌── 下位机：工业 PC ────────┐
│  inl CLI (Go + Cobra)            │ TCP  │  nrc2.out (io-controller)   │
│     ↓ Registry 25 CommandSpec    │:6000 │     ↓ DataType 分发          │
│  NRC 帧 (0x9275) ──────────────► │─────►│  switchsendmapvarvalue →    │
│                                    │      │  ├─ DataType=12            │
│  响应 (0x9271) ◄─────────────────│◄─────│  │  NetWorkTopologyFunction │
│                                    │      │  ├─ DataType=13            │
│  Envelope / Table / CSV / NDJSON  │      │  │  CallBackGsdFileList     │
│  Structured Errors (type/code)    │      │  ├─ DataType=14            │
│  DryRunFrame + --retry            │      │  │  PerformOnlineAccess    │
│  reliability.Policy               │      │  ├─ DataType=16            │
│                                    │      │  │  FilterGSDCompatible     │
│  AI Skills (SKILL.md × 3)        │      │  └─ DataType=17            │
│     ↓                            │      │                            │
│  inl-shared + profinet-write     │      │                            │
│  + profinet-config               │      │                            │
└────────────────────────────────────┘      └────────────────────────────┘
```

## AI Skills

inl 配套 3 个 AI Agent Skill 文档，位于 `skills/` 目录：

| Skill | 用途 | 加载时机 |
|-------|------|---------|
| [`inl-shared`](skills/inl-shared/SKILL.md) | 共享规则：Risk 等级、`--yes`/`--dry-run`、错误码 | 所有 inl 任务必读 |
| [`inl-workflow-profinet-write`](skills/inl-workflow-profinet-write/SKILL.md) | 写操作 4 层安全流程：预检 → 备份 → 确认 → 回滚 | 接到修改/删除/编译任务时 |
| [`inl-workflow-profinet-config`](skills/inl-workflow-profinet-config/SKILL.md) | 配网 8 阶段编排：评估→发现→规划→验证→执行→编译→检查→文档 | 接到完整配网任务时 |

## 开发

```bash
go test ./...            # 11 个包全部测试
go vet ./...             # 静态分析
go build -o inl.exe .    # 构建
```

**包结构**：

| 包 | 职责 |
|----|------|
| `main.go` | Cobra 命令树入口 + Risk 检查 + `--dry-run` + `--format` + `device setup` 组合命令 |
| `internal/nrc/` | NRC 帧编解码 + TCP 客户端 + 25 命令 Registry + 11 个 BodyBuilder |
| `internal/nrc/config_body.go` | 10 个 config 写命令 BodyBuilder + 通用 configBodyBuilder + fetch/validate |
| `internal/output/` | 结构化错误 + Envelope + DryRunFrame + Table/CSV/NDJSON 渲染 |
| `internal/reliability/` | 重试策略子包：Policy + Default() + ShouldRetryNetwork + `--retry` flag |
| `internal/configresp/` | config 写命令响应模型：WriteResponse + ShieldDeviceResponse |
| `internal/gsd/` | DataType=13/16 响应模型（8 structs） |
| `internal/topology/` | CallBackJson/ActivatedJson + DCP ScanResponse |
| `internal/devicestatus/` | GetActRun 焊机状态模型 |
| `internal/gsdfile/` | GetGSDFile* 响应模型 |
| `internal/dcpdevice/` | DCP 发现设备模型（9 字段 + Block 映射） |
| `internal/netiface/` | 网络端口列表模型 |

## 分发

| 渠道 | 状态 | 说明 |
|------|:--:|------|
| GitHub Release | 就绪 | `tar.gz` + `zip` 6 平台二进制 + checksums（推送 `v*` tag 自动触发） |
| npm | 就绪 | 1 入口包 + 6 平台子包，`npm install -g inl-cli` |
| 本地构建 | ✅ | `go build` 静态编译单文件 ~5MB |

## 协议参考

- NRC Socket 帧：SyncByte `0x4E66` + Length + Command + JSON + CRC32 (IEEE 802.3)
- 请求 Command = `0x9275`，响应 Command = `0x9271`
- CRC32 经验证与 PDF 已知正确帧一致 (`0x53DDEB72` / `0x6B926DFF`)
- 完整协议契约见 [AGENTS.md](AGENTS.md)

## 文档

- [产品需求文档](docs/inl-prd.md)
- [架构设计文档](docs/inl-architecture.md)
- [配网工作流设计](docs/inl-workflow-design.md)
- [开发现状报告](docs/inl-development-status.md)
- [Config 字段参照](docs/inl-config-field-reference.md)
- [实机偏差登记](docs/protocol/field-verification.md)
- [协议契约](AGENTS.md)
