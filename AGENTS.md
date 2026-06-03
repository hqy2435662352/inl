# AGENTS.md

## Project

`inl` 是工业 PC NRC Socket 协议的 CLI 调试工具。通过 TCP:6000 与运行 `nrc2.out` 的工业 PC 通信，收发 NRC 帧，解析 PROFINET GSD 设备列表、拓扑、配置。基于 **Cobra 命令树**（`inl gsd list` / `inl device list` / `inl topology scan` / `inl device setup`），全部命令元数据集中在 `internal/nrc/commands.go` 的 `Registry` 中（共 **22 条**：2 gsd + 7 device + 11 config + 1 interface + 1 topology）。Cobra 树按 `Group` 自动遍历 Registry 构建；写命令按 **Risk 等级**（`RiskRead` / `RiskWrite` / `RiskHighRiskWrite`）自动追加 `--yes` 标志，DCP 命令按 `spec.Args` 注册 `--interface` / `--mac` / `--name` / `--ip` / `--mask` 等 flag。AI Agent 可基于 `--help` 末行的 `Risk:` 标识自动决策是否需要确认。

## Source Layout

| Path | Purpose |
|------|---------|
| `main.go` | **Cobra 命令树入口**：rootCmd + `--target` / `--format` / `--output` 三个 PersistentFlags。`main()` 按 `[GroupInterface, GroupGsd, GroupDevice, GroupConfig, GroupTopology]` 顺序遍历 nrc.Registry 自动构建 cobra.Command 节点；`buildGroupCmd(group)` / `buildSubCmd(spec)` 两个泛化工厂 + `collectDCPArgs(cmd)` 辅助函数(DCP 命令参数提取);`runNrcCommand` 集成 **Risk 检查**（非 read 需 `--yes`，high-risk-write 额外二次确认提示）；`installRiskHelpFunc` 在 help 文本底部追加 `Risk: <level>` 行。**新增命令边际成本**：只在 Registry 追加一行 + 设置 `Group` 字段，main.go **无需任何修改** |
| `internal/nrc/commands.go` | **命令元数据集中层** — 11 字段 `CommandSpec` struct（`Name` / `Code` / `DataType` / `Direction` / `Description` / `Risk` / `Response` / `Function` / `Group` / `Args` / `BodyBuilder`）+ `CommandGroup`(5 个常量) + `ArgumentSpec` + 22 条 `Registry` + 5 个 DataType=14/16 专用 `BodyBuilder`(`interfaceListBody` / `topologyScanBody` / `gsdMatchBody` / `deviceSetupNameBody` / `deviceSetupIPBody`) + `LookupByName` / `LookupByDataType` / `ExpectedResponseCode` / `RequestBody`(新签名 `(spec, args) (string, error)`) / `DefaultBodyBuilder`。`init()` 检查 Name 唯一 + DataType:Function 组合键唯一, 重复则 panic |
| `internal/nrc/commands_test.go` | Registry 唯一性、Lookup 行为、RequestBody 正确性、风险分布、Group 分布、拼写陷阱、5 个新 DCP 命令的 Lookup + BodyBuilder 测试等的单元测试（**26 个**） |
| `internal/nrc/annotation.go` | **Cobra Annotations 常量定义** — `AnnotationRisk` / `AnnotationPureGroup` / `AnnotationDataType` / `AnnotationFunction`，供 main.go 标注 + AI 调度器读取 |
| `internal/nrc/frame.go` | NRC Socket 帧编解码：`BuildFrame`（构建）、`ReadFrame`（读取+校验）。CRC32 多项式为 IEEE 802.3 (`crc32.IEEE`) |
| `internal/nrc/frame_test.go` | 帧编解码单元测试，含 PDF 已知正确帧的 CRC 验证 |
| `internal/nrc/client.go` | TCP 客户端：`Connect`(5s 超时)、`SendReceive`(10s 超时)、中文错误提示 |
| `internal/output/errors.go` | **结构化错误输出**（借鉴 lark-cli `output.Errorf` 模式）— `Error` struct（`Type` / `Code` / `Message` / `Hint` / `Detail`）+ `WriteError` 函数。AI 可通过 `code` 字段识别错误类型并自动决策 |
| `internal/gsd/types.go` | **GSD 领域模型** — 8 个 struct 完整覆盖 DataType=13 响应的 JSON 结构,每个字段均有中文注释说明含义、枚举值、存在条件。`MatchResponse` struct 覆盖 DataType=16(GSD 匹配) 响应 |
| `internal/gsd/types_test.go` | 序列化往返测试，覆盖 HMS/Siemens/SMC 三种设备结构变体 + MatchResponse round-trip |
| `internal/topology/types.go` | **拓扑响应领域模型** — `CallbackJsonResponse` / `CallbackActivatedJsonResponse`(type alias) + `Station` + `Device` struct,涵盖 DataType=12 + Function.Value=`CallBackJson` / `CallBackActivatedJson` 两种响应;`ScanResponse` struct 覆盖 DataType=14 + Function=1(DCP 发现) 响应 |
| `internal/dcpdevice/types.go` | **DCP 设备信息模型** — `DCPDevice` struct(9 字段:Mac / DeviceVendorValue / DeviceName / VendorID / DeviceID / DeviceRole / IPAddress / SubNetMask / GateWay),对应 C++ 端 processResponseFilteredFrames 解析结果。**DCP Block (Option, Suboption) → 字段**映射的唯一权威来源 |
| `internal/dcpdevice/types_test.go` | DCP 设备 round-trip 测试(基础解析 + 9 字段反射检查 + 部分 JSON 解析) |
| `internal/netiface/types.go` | **网络端口列表响应模型** — `ListResponse` struct(4 字段:DataType / PortName / Mac / IP) + `Port` struct + `Flatten()` 方法,对应 DataType=14 + Function=4 响应(三个对齐数组) |
| `internal/netiface/types_test.go` | 端口列表 round-trip 测试(基础解析 + Flatten 顺序 + 空数组处理) |
| `internal/devicestatus/types.go` | **GetActRun 响应领域模型** — `Response` / `Function` / `Device` struct,覆盖 DataType=12 + Function.Value=`GetActRun` 响应(活动设备焊机状态) |
| `internal/gsdfile/types.go` | **GSDML 文件响应领域模型** — `Response` / `Function` / `GSDFile` struct,覆盖 DataType=12 + Function.Value=`GetGSDFileNetwork` / `GetGSDFileActivated` 响应(与 GSDML 全文两种可能位置兼容:`GSDFiles[]` 数组 / `GSDFile` 单字符串) |
| `docs/protocol/field-verification.md` | **实机响应反向核对记录** — 每个读命令的"假设字段 ↔ 实机字段"对照表,发现差异立刻登记。inl 第 3 步起的"实机 JSON 响应 → 反向核对领域模型"反馈循环的**唯一权威记录** |

## Key Protocol Knowledge

### NRC Socket 帧格式

```
┌──────────┬──────────┬──────────┬───────────────┬──────────┐
│ SyncByte │  Length  │ Command  │  Data (JSON)  │   CRC32  │
│  2 Byte  │  2 Byte  │  2 Byte  │   Length−2字节  │  4 Byte  │
│  0x4E66  │  BigEnd  │  BigEnd  │    UTF-8       │  BigEnd  │
└──────────┴──────────┴──────────┴───────────────┴──────────┘
```

- CRC32 计算范围：Length + Command + Payload（经实验确认，与 PDF 已知 CRC 值 `0x53DDEB72` / `0x6B926DFF` 一致）
- 发送 Command = `0x9275`，接收 Command = `0x9271`
- TCP 端口 `6000`，连接超时 5s，读写超时 10s

### GSD 响应 JSON 结构

```
Response
└── Device[]
    ├── 元数据: VendorID, VendorName, DeviceID, GSDName, MainFamily, ProductFamily
    ├── DAP[]                          ← 设备接入点变体（铜口/光口）
    │   ├── UseableModules[]           ← 模式A(FixedInSlots) 或 模式B(AllowedInSlots*)
    │   └── VirtualSubmoduleList？     ← 仅 PLC 类设备非空（Siemens CPU SR40）
    ├── Module[]                       ← 可插入模块，PLC 类为空
    │   ├── UseableSubmodules[]        ← 子模块槽位分配（目前仅 SMC EX245）
    │   └── VirtualSubmoduleList[]
    │       └── IOData[] {Length 必存在}
    └── Submodules[]                   ← Shared 子模块输出镜像（仅 SMC EX245）
        └── IOData[] {Length 不存在}
```

- `IOData.Length` 仅存在于 Module.VirtualSubmoduleList 内，DAP 层和 Submodules 层无此字段
- `UseableModules` 双模式（FixedInSlots vs AllowedInSlots*）互斥，由 GSDML 源文件写法决定
- 详细字段说明见 `internal/gsd/types.go` 注释

### Registry 22 命令表

inl 所有支持的 NRC 命令集中在 `internal/nrc/commands.go` 的 `Registry` 切片（22 条）中。每条 `CommandSpec` 含 11 字段。完整命令表：

| # | Name | Function.Value | Group | Risk | DataType |
|---|------|----------------|-------|------|----------|
| 1 | `gsd-list` | `""` | `gsd` | `read` | 13 |
| 2 | `device-list` | `CallBackJson` | `device` | `read` | 12 |
| 3 | `device-list-active` | `CallBackActivatedJson` | `device` | `read` | 12 |
| 4 | `device-run` | `GetActRun` | `device` | `read` | 12 |
| 5 | `device-gsd-config` | `GetGSDFileNetwork` | `device` | `read` | 12 |
| 6 | `device-gsd-active` | `GetGSDFileActivated` | `device` | `read` | 12 |
| 7 | `config-set-driver` | `SetPNDriver` | `config` | `write` | 12 |
| 8 | `config-add-device` | `AddPNDevice` | `config` | `write` | 12 |
| 9 | `config-remove-device` | `UninstallPNDevice` | `config` | `write` | 12 |
| 10 | `config-set-device` | `SetPNDevice` | `config` | `write` | 12 |
| 11 | `config-add-module` | `AddModule` | `config` | `write` | 12 |
| 12 | `config-remove-module` | `UninstallModule` | `config` | `write` | 12 |
| 13 | `config-add-submodule` | `AddSubmodule` | `config` | `write` | 12 |
| 14 | `config-remove-submodule` | `UninstallSubmodule` | `config` | `write` | 12 |
| 15 | `config-shield` | `ShieldDevice` | `config` | `write` | 12 |
| 16 | `config-unshield` | `UNShieldDevice` | `config` | `write` | 12 |
| 17 | `config-compile` | `Compile` | `config` | `high-risk-write` | 12 |
| 18 | `interface-list` | `"4"`(整数) | `interface` | `read` | 14 |
| 19 | `topology-scan` | `"1"`(整数) | `topology` | `read` | 14 |
| 20 | `gsd-match` | `""` | `gsd` | `read` | 16 |
| 21 | `device-setup-name` | `"2"`(整数) | `device` | `write` | 14 |
| 22 | `device-setup-ip` | `"3"`(整数) | `device` | `write` | 14 |

✅ `ShieldDevice` / `UNShieldDevice` 拼写已统一（2026-06-01）。C++ 控制器端已纠正拼写错误（少 'e'），inl 内部命令名用 `shield` / `unshield`，请求体 `Function.Value` 用 `"ShieldDevice"` / `"UNShieldDevice"`（正确拼写）。

⚠️ **第 18-22 条 DCP 命令 Function 字段为整数语义**（`"1"` / `"2"` / `"3"` / `"4"`），与第 2-17 条 DataType=12 命令的字符串 Function.Value（如 `"CallBackJson"`）区分。C++ 端 DataType=14 分发器 [`PerformOnlineAccess`](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/io-controller/src/pnconfiglib/pndcp.cpp#L117-L182) 用 `root["Function"] == 1/2/3/4` 做整数比较,字符串格式会失配。inl 端 5 个 DCP 命令使用专用 BodyBuilder（`interfaceListBody` / `topologyScanBody` / `gsdMatchBody` / `deviceSetupNameBody` / `deviceSetupIPBody`），不复用 `DefaultBodyBuilder`。

**新增命令的步骤**：在 `Registry` 追加一行 + 设置 `Group` 字段(可选用 `Args` 注册 DCP 参数),main.go **无需任何修改**（自动遍历会拾取新命令）。`init()` 会自动校验 Name / DataType+Function 组合键唯一性,重复则 panic。

### Risk 等级与 --yes

inl 借鉴 lark-cli 服务方法风险检查模式，在 `runNrcCommand` 中实现 Risk 检查：

| RiskLevel | 含义 | `--yes` 要求 | high-risk 二次确认 |
|-----------|------|--------------|---------------------|
| `read` | 只读，无副作用 | 不需要 | 不需要 |
| `write` | 写配置，可能影响设备状态 | **必须** `--yes` | 不需要 |
| `high-risk-write` | 高危（重启/擦除/编译） | **必须** `--yes` | stderr 输出 `⚠️  高危操作` 提示行 |

**完整流程**（写命令）：
1. 用户执行 `inl config add-device --target 192.168.3.15`（无 `--yes`）
2. `runNrcCommand` 检查 `spec.Risk != RiskRead`，`--yes` flag 未设
3. 返回 `*output.Error{Code: "yes_required"}`，退出码非 0，stderr 输出结构化 JSON
4. AI Agent 解析 `code: yes_required` → 自动追加 `--yes` 重试

**完整流程**（高危写命令）：
1. 用户执行 `inl config compile --target 192.168.3.15 --yes`
2. `--yes` 已设；`spec.Risk == RiskHighRiskWrite`
3. stderr 输出 `⚠️  高危操作: config-compile (high-risk-write)` 二次确认提示
4. 继续发送请求

**Group 标注**:`gsd` / `device` / `interface` / `topology` 四个 group 在 Annotations 中标 `inl.dev/pure-group: "true"`,AI 调度可识别为"无副作用"整组放行;`config` 不标(其命令含写副作用)。

### Cobra Annotations

`internal/nrc/annotation.go` 定义 4 个 cobra.Annotations 字符串常量：

| 常量 | 字符串值 | 用途 |
|------|----------|------|
| `AnnotationRisk` | `inl.dev/risk` | 命令 Risk 等级（`read` / `write` / `high-risk-write`） |
| `AnnotationPureGroup` | `inl.dev/pure-group` | group 是否纯读取（仅 `gsd` / `device` 标 `"true"`） |
| `AnnotationDataType` | `inl.dev/datatype` | 命令的 DataType 字段 |
| `AnnotationFunction` | `inl.dev/function` | 命令的 Function.Value 字符串 |

`main.go` 在 `buildSubCmd` 中自动把 `AnnotationRisk` / `AnnotationDataType` / `AnnotationFunction` 写入 `cobra.Command.Annotations`；`buildGroupCmd` 写入 `AnnotationPureGroup`。`installRiskHelpFunc` 读取 `AnnotationRisk` 在 help 文本底部追加 `Risk: <level>` 行。

### 结构化错误

`internal/output/errors.go` 提供 `output.Error` struct + `WriteError` 函数，借鉴 lark-cli `output.Errorf` 模式，让 AI Agent 能以结构化方式解析错误。

**Error struct 字段**：

| 字段 | 类型 | 含义 | 示例 |
|------|------|------|------|
| `Type` | `string` | 错误分类 | `validation` / `permission` / `protocol` |
| `Code` | `string` | 错误代码 | `yes_required` / `target_required` / `unknown_command` / `unexpected_response_command` |
| `Message` | `string` | 人类可读描述 | `拒绝执行: config-add-device 是 write 操作, 需加 --yes 标志确认` |
| `Hint` | `string` | 修复建议（可选） | `查看风险: inl config-add-device --yes --help` |
| `Detail` | `map[string]any` | 任意附加上下文（可选） | `{"command": "config-add-device", "target": "192.168.3.15"}` |

**JSON 输出示例**（`output.WriteError(os.Stderr, &Error{...})` 缩进后）：

```json
{
  "type": "validation",
  "code": "yes_required",
  "message": "拒绝执行: config-add-device 是 write 操作, 需加 --yes 标志确认",
  "hint": "查看风险: inl config-add-device --yes --help"
}
```

**`Error.Error()` 方法**返回 JSON 序列化字符串，实现 `error` 接口；`main.go` 在 `rootCmd.Execute()` 错误处理处统一 `WriteError` 缩进输出到 stderr。

### 输出约定（stdout/stderr 分流）

借鉴 lark-cli 的 "stdout 是数据" 原则：

| 流 | 内容 | 示例 |
|---|------|------|
| **stdout** | 数据 / 命令结果（prettified JSON） | `{"DataType":13, "Device":[...]}` |
| **stderr** | 进度 / 警告 / 结构化错误 | `🔌 连接中...`、 `⚠️  高危操作: ...`、 `--target 不能为空` |

验证分流：

```bash
inl gsd list --target 192.168.3.15 > out.json 2> progress.log
# out.json: 仅 prettified JSON
# progress.log: 🔌 / 📤 / 💾 / ⚠️ 等进度行
```

## Build & Test

```bash
go mod tidy              # 同步依赖（首次需拉取 cobra）
go build ./...           # 编译所有包
go vet ./...             # 静态分析
go test ./...            # 全部单元测试（gsd + nrc + topology + devicestatus + gsdfile + output）
go build -o inl.exe .    # 生成可执行文件
```

**包清单**（按依赖方向）：
- `internal/nrc/` — 帧编解码 + TCP 客户端 + 命令元数据 + Annotations（零外部依赖）
- `internal/gsd/` — DataType=13 响应模型（零依赖）
- `internal/topology/` — DataType=12 + CallBackJson/CallBackActivatedJson 响应模型（零依赖）
- `internal/devicestatus/` — DataType=12 + GetActRun 响应模型（零依赖）
- `internal/gsdfile/` — DataType=12 + GetGSDFileNetwork/GetGSDFileActivated 响应模型（零依赖）
- `internal/output/` — 结构化错误（零依赖）

**依赖**：`github.com/spf13/cobra v1.10.2`（inl 第一个非标准库依赖）+ 间接依赖 `pflag` / `mousetrap`。所有 `internal/` 包仍为零依赖。

## Run

```bash
# 1. 读取 GSD 设备驱动列表（只读，无需 --yes）
inl --target 192.168.3.15 gsd list

# 2. 读取所有已配置 PROFINET 设备（CallBackJson，只读）
inl --target 192.168.3.15 device list

# 3. 获取活动运行的设备 / 焊机状态（GetActRun，只读；第 2 步曾误命名为 topology get）
inl --target 192.168.3.15 device run

# 4. 写命令必须加 --yes 确认
inl --target 192.168.3.15 config add-device --yes

# 5. 高危写命令（编译并应用配置，会重启控制器）需 --yes + 二次确认提示
inl --target 192.168.3.15 config compile --yes

# 自定义原始响应保存路径
inl --target 192.168.3.15 gsd list --output /tmp/my.json

# 查看帮助（help 文本底部显示 Risk: <level>）
inl --help
inl gsd --help
inl device --help
inl config --help
inl config compile --help
```

每次运行在 `inl/` 目录生成 `<name>_response_<时间戳>.json`（prettified 原始 JSON）。`<name>` 取自 `CommandSpec.Name`（如 `gsd-list_response_20260601_150405.json` / `device-run_response_*.json`）。

---

## DataType=12 协议契约（C++ 源码真相）

> **本章节是 inl 与 C++ 服务端协议对齐的"单一权威来源"**。当 C++ 源码、本地设计文档、实机响应三者出现冲突时，**以本章节为准**。所有 inl 命令的 `Function.Value` 字符串均直接对应 C++ 源码 `NetWorkTopologyFunction` 分发器分支。

### 完整 17 命令表（DataType=12 协议层真相）

C++ 源分发器 [`NetWorkTopologyFunction`](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/io-controller/src/pnconfiglib/PNConfigLibFileDesign.cpp#L165-L222) 共 17 个 `Function.Value` 分支 + 1 个独立的 DataType=13 命令：

| # | inl 命令名 | Function.Value | Risk | DataType | C++ 分发分支 |
|---|-----------|----------------|------|----------|--------------|
| 1 | `gsd-list` | `""`（无 Function） | `read` | 13 | 独立入口（不在 NetWorkTopologyFunction 内） |
| 2 | `device-list` | `CallBackJson` | `read` | 12 | `CallBackJson()` |
| 3 | `device-list-active` | `CallBackActivatedJson` | `read` | 12 | `CallBackActivatedJson()` |
| 4 | `device-run` | `GetActRun` | `read` | 12 | `GetActRun()` |
| 5 | `device-gsd-config` | `GetGSDFileNetwork` | `read` | 12 | `GetGSDFileNetwork()` |
| 6 | `device-gsd-active` | `GetGSDFileActivated` | `read` | 12 | `GetGSDFileActivated()` |
| 7 | `config-set-driver` | `SetPNDriver` | `write` | 12 | `SetPNDriver()` |
| 8 | `config-add-device` | `AddPNDevice` | `write` | 12 | `AddPNDevice()` |
| 9 | `config-remove-device` | `UninstallPNDevice` | `write` | 12 | `UninstallPNDevice()` |
| 10 | `config-set-device` | `SetPNDevice` | `write` | 12 | `SetPNDevice()` |
| 11 | `config-add-module` | `AddModule` | `write` | 12 | `AddModule()` |
| 12 | `config-remove-module` | `UninstallModule` | `write` | 12 | `UninstallModule()` |
| 13 | `config-add-submodule` | `AddSubmodule` | `write` | 12 | `AddSubmodule()` |
| 14 | `config-remove-submodule` | `UninstallSubmodule` | `write` | 12 | `UninstallSubmodule()` |
| 15 | `config-shield` | `ShieldDevice` | `write` | 12 | `ShieldDevice()` |
| 16 | `config-unshield` | `UNShieldDevice` | `write` | 12 | `UNShieldDevice()` |
| 17 | `config-compile` | `Compile` | `high-risk-write` | 12 | `Compile()` |

### C++ 源码位置

- **分发表**：[`PNConfigLibFileDesign.cpp:165-222`](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/io-controller/src/pnconfiglib/PNConfigLibFileDesign.cpp#L165-L222) — `NetWorkTopologyFunction` 17 个 `Function.Value` 分支
- **DataType 常量**：[`profinet_constants.h:69-85`](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/io-controller/src/pnconfiglib/profinet_constants.h#L69-L85) — DataType=12/13/14 等宏定义
- **主入口**：[`main.cpp:536-538`](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/io-controller/src/pnconfiglib/main.cpp#L536-L538) — TCP:6000 服务端入口

### ✅ 拼写已纠正：ShieldDevice / UNShieldDevice

历史背景（2026-06-01 之前）：C++ 源码常量曾拼写为 `ShildDevice` / `UNShildDevice`（Shield 少 'e'），inl 请求体需原样沿用以通过字符串比较。

**当前状态**：C++ 控制器端已纠正拼写为 `ShieldDevice` / `UNShieldDevice`，inl 客户端 [`internal/nrc/commands.go:263, 276`](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/inl/internal/nrc/commands.go#L263) 已使用正确拼写，**不再有拼写陷阱**。

| 实体 | 拼写 |
|------|------|
| inl 命令名 | `config-shield` / `config-unshield` |
| inl 请求体 `Function.Value` | `"ShieldDevice"` / `"UNShieldDevice"` |
| C++ 端函数名 | `ShieldDevice()` / `UNShieldDevice()` |
| C++ 端常量 | `PNCONFIGLIB_FUNCTION_SHIELD_DEVICE = "ShieldDevice"` |

**注意**：本仓库的 [`io-controller/src/ioc/profinet_constants.h`](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/io-controller/src/ioc/profinet_constants.h) 是**协议参考快照**，可能仍含旧拼写。修改该文件需谨慎（仅作参考，**不是现场跑的代码**）。

### 协议缺陷：工业 PC 端 nrc2.out 可能与本地 C++ 源码不一致

- C++ 服务端 [`NetWorkTopologyFunction:153-156`](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/io-controller/src/pnconfiglib/PNConfigLibFileDesign.cpp#L153-L156) 要求请求必须带 `Function` 字段（且为 object 类型）
- 我们第 2 步发送的 `{"DataType":12}` 没有 `Function.Value`，按源码应该只打 stderr 不发响应
- **但实机仍收到了响应** → 说明工业 PC 端可能跑了与本地源码不同版本的代码
- **规范请求**：所有 DataType=12 命令应发 `{"DataType":12,"Function":{"Value":"<Function>"}}`（即 DefaultBodyBuilder 在 `spec.Function != ""` 时的输出格式）

### 协议澄清：请求 DataType 与响应 DataType 不需要对齐

- **请求 DataType** 供工业 PC 服务端识别请求类型（`CallBackJson` / `GetActRun` / ...）
- **响应 DataType** 供示教器（另一类客户端）识别响应类型，与请求 DataType **无对应关系**
- 服务端 C++ 9 处硬编码 `root["DataType"] = 14;`（见 [`PNConfigLibFileDesign.cpp`](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/io-controller/src/pnconfiglib/PNConfigLibFileDesign.cpp) 301/314/331/348/366 行），所有 DataType=12 响应统一回 14
- inl 客户端**不应**校验响应 DataType 与请求 DataType 一致

## DataType=14 协议契约（DCP 操作）

> 第 5 步新增章节。DataType=14 是与 DataType=12 平行且更早的协议路径，承载 DCP（Discovery and Configuration Protocol）操作，由 C++ 端 [`PerformOnlineAccess`](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/io-controller/src/pnconfiglib/pndcp.cpp#L117-L182) 分发。共 4 个 Function 整数分支 + 1 个独立的 DataType=16 GSD 匹配入口。

### 4 个 Function 分支

| Function.Value (整数) | 行为 | 必需参数 | 响应方式 | 对应 inl 命令 |
|:---:|------|---------|---------|------------|
| `1` | DCP 发现网络中所有 PROFINET 设备 | `Portname` | NRC `0x9271` + JSON (DataType=14, Devices[]) | `topology scan` |
| `2` | 设置设备名称 | `Portname` + `TargetMAC` + `Newdevicename` | **仅错误报告**（通过 `BYD_TriggerErrorReport`），无 JSON 响应 | `device setup-name` |
| `3` | 设置设备 IP + 子网掩码 | `Portname` + `TargetMAC` + `Newipaddress` + `Newsubnetmask` | **仅错误报告**，无 JSON 响应 | `device setup-ip` |
| `4` | 获取本地网络接口列表 | 无 | NRC `0x9271` + JSON (DataType=14, PortName/Mac/IP 三个对齐数组) | `interface list` |

> ⚠️ **注意**：Function=2/3（set name/IP）是**副作用操作**——C++ 端直接收发 DCP 原始帧，**不通过 NRC 返回 JSON 响应**。成功时无显式确认，失败时通过 `BYD_TriggerErrorReport` 报告。inl 端的"成功"判断依据为：NRC 通信正常 + 无后续错误报告。**建议**：命令执行后立即跑 `topology scan` 验证名称/IP 是否变更(闭环验证,留待后续 PR 自动实现)。

### DataType=16 入口（GSD 匹配）

| DataType | 行为 | 必需参数 | 响应方式 | 对应 inl 命令 |
|:---:|------|---------|---------|------------|
| `16` | 发现 + GSD 匹配 (按 VendorID+DeviceID 过滤) | `Portname` | NRC `0x9271` + JSON (DataType=16, Devices[]) | `gsd match` |

> 若 nrc2.out 版本不支持 DataType=16,降级为"AI 客户端手动匹配"算法(已在 `inl-workflow-design.md §2.2` 备用)。

### Discovery 响应 JSON 结构（DataType=14, Function=1 / DataType=16）

C++ 端 `processResponseFilteredFrames` 返回的数据块解析结果：

```json
{
  "DataType": 14,
  "Devices": [
    {
      "Mac": "00:11:22:33:44:55",
      "DeviceVendorValue": "OBARA Corporation",
      "DeviceName": "heron-weld",
      "VendorID": "0x038A",
      "DeviceID": "0x0030",
      "DeviceRole": "PN设备",
      "IPAddress": "192.168.2.10",
      "SubNetMask": "255.255.255.0",
      "GateWay": "192.168.2.1"
    }
  ]
}
```

数据块类型映射（DCP Block / Option,Suboption → 字段）：

| DCP Block (O,S) | 字段 | 说明 |
|:---:|------|------|
| (2, 1) | `DeviceVendorValue` | 设备厂商名称字符串 |
| (2, 2) | `DeviceName` | DCP 设备名称（经过 TransformDeviceNameBack 反转义） |
| (2, 3) | `VendorID`, `DeviceID` | PI 分配的 16-bit ID |
| (2, 4) | `DeviceRole` | 1=PN设备, 2=PN控制器, 4=PN多设备, 8=PN监视器 |
| (1, 2) | `IPAddress`, `SubNetMask`, `GateWay` | 设备网络参数 |

> 完整字段含义与 JSON 解析见 [`internal/dcpdevice/types.go`](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/inl/internal/dcpdevice/types.go) 中 DCP Block 来源注释。

### Interface List 响应 JSON 结构（DataType=14, Function=4）

```json
{
  "DataType": 14,
  "PortName": ["enp4s0", "eth0"],
  "Mac":      ["68:ed:a6:0b:c4:3b", "00:1b:21:ab:cd:ef"],
  "IP":       ["192.168.3.15", "10.0.0.100"]
}
```

> 三个数组长度一致,按索引对应。过滤条件(IP 非空且 MAC ≠ 00:00:00:00:00:00)在 C++ 端完成。

### C++ 源码位置

- **DCP 入口**：[`pndcp.cpp:117-182`](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/io-controller/src/pnconfiglib/pndcp.cpp#L117-L182) — `PerformOnlineAccess` 函数,按 `root["Function"]` 整数分发
- **GSD 匹配**：[`pndcp.cpp:1348-1390`](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/io-controller/src/pnconfiglib/pndcp.cpp#L1348-L1390) — `FilterGSDCompatibleDevices` 函数,按 VendorID+DeviceID 过滤 DCP 发现结果
- **DCP 帧解析**：[`pndcp.cpp` 全文件](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/io-controller/src/pnconfiglib/pndcp.cpp) — DCP 原始帧解析、Block 字段提取、`TransformDeviceNameBack` 反转义

### Function 字段类型冲突解决方案

C++ 端 DataType=12 的 `Function` 是字符串 object（如 `Function: {Value: "CallBackJson"}`），DataType=14 的 `Function` 是**整数**（如 `Function: 1`）。inl 端 `DefaultBodyBuilder` 对非空 Function 总是输出字符串 object 格式,**不匹配** DataType=14 整数语义。

**本步骤的解决方案**：5 个 DCP 命令（`interface-list` / `topology-scan` / `gsd-match` / `device-setup-name` / `device-setup-ip`）**均使用专用 BodyBuilder**（不复用 `DefaultBodyBuilder`），输出整数 Function 格式。`DefaultBodyBuilder` 保持原行为,继续供 DataType=12 命令使用。

### 已知偏差汇总

> 与 [`docs/protocol/field-verification.md`](docs/protocol/field-verification.md) 同步。本节只列"实机 vs 协议契约"不一致的项；所有核对记录详见 `field-verification.md`。

| 命令 | 偏差描述 | 原因 | 影响 | 后续 PR |
|------|---------|------|------|---------|
| `device-run`（第 2 步曾误命名 `topology-get`） | 命令名暗示"PROFINET 拓扑"但实际返回"活动焊机列表" | C++ 端 `GetActRun` = "Get Active Run" 语义与拓扑无关 | 第 2 步文档误导；已在第 3 步**重命名为 `device-run`** 解决 | 已完成（见 tasks.md Task 4.3） |
| `device-list` (CallBackJson) | ① `Function` 是字符串 `"CallBackJson"` 不是对象；② 内容为 `IDevice`+`PNDriver` 配置而非 `Stations`/`Devices` 拓扑；③ 响应 DataType=12 而非 14 | 工业 PC 运行版本与 C++ 源码不一致；`CallBackJson` 语义 = "返回网卡配置"而非"返回设备拓扑" | `topology/types.go` 中 `CallbackJsonResponse` 模型**完全错误**，需重写为 IDevice+PNDriver 结构 | 待 PR（新增 `internal/idevice/types.go` 或重命名 `topology` 包） |
| `device-list-active` (CallBackActivatedJson) | ① 设备列表字段名为 `DecentralDevice` 非 `Devices`；② 含完整 Module/SubModule/Slot/IO 地址嵌套；③ 与 `device-list` 结构完全不同 | C++ 源码命名约定 `DecentralDevice` (单数)；两个 CallBack 响应语义不同 | `topology/types.go` 中 `CallbackActivatedJsonResponse` type alias 错误，需独立 struct | 待 PR（重写为 DecentralDevice + Module + SubModule 完整层级） |
| `device-gsd-config` (GetGSDFileNetwork) | `GSDFile` 为空且 `error:true`；v1 未建模 `error` 字段 | 工业 PC 192.168.3.15 未在网络配置中保存 GSD 文件 | `gsdfile/types.go` 需新增 `Error bool` 字段 | 待 PR（新增 `Error bool \`json:"error"\`` 到 `gsdfile.Function`） |
| `device-gsd-active` (GetGSDFileActivated) | NRC 协议错误 (响应命令字 0x2B04 ≠ 0x9271) | 工业 PC 192.168.3.15 的 `nrc2.out` 版本不支持此功能 | 命令无法在 192.168.3.15 上使用；需升级 nrc2.out 或换设备 | 待 PR（确认 nrc2.out 版本号、升级或找支持设备重测） |
| `device-setup-name` / `device-setup-ip` (DCP Function=2/3) | **副作用操作无 JSON 响应**——C++ 端直接收发 DCP 原始帧,成功时无显式确认,失败时通过 `BYD_TriggerErrorReport` 报告 | C++ 端 PerformOnlineAccess 协议设计如此(不走 NRC JSON 响应) | inl 端无法直接判断成功;只能通过后续 `topology scan` 验证名称/IP 变更(闭环验证) | 待 PR(`inl device setup` 命令执行后自动跑 `topology scan` 验证) |

### 协议层未覆盖的项

> `SetIDevice` 在 C++ 源分发表中**存在**（`PNConfigLibFileDesign.cpp:165-222` 第 N 个分支），但 inl 第 3 步**未在 Registry 中注册**。原因：第 3 步聚焦"协议字段命名核对"读命令，写命令先做最小覆盖；`SetIDevice` 需带复杂参数（设备名/ID 映射）目前 inl 的 `Args []ArgumentSpec` 还没接 `--data` JSON 构造能力，留待后续 PR。决策记录见 `tasks.md` Task 4.3。

### 反馈循环：实机响应 → 反向核对领域模型

第 3 步起强制建立"实机 JSON 响应 → 反向核对领域模型"反馈循环。规则：

1. 取实机 JSON 响应（来自 `<spec.Name>_response_<时间戳>.json`）
2. 与当前 `Response` struct 字段对比，列出"已建模字段 / 实机额外字段 / 建模但实机缺失字段"
3. 若发现偏差，写一行记录到 [`docs/protocol/field-verification.md`](docs/protocol/field-verification.md)
4. 若需要更新 `Response` struct，立即修改对应 `types.go` 并补充 round-trip 测试（用实机 JSON 作 fixture）
5. 在 `field-verification.md` 章节末尾标 `✅ 已修正` 或 `⚠️ 已知偏差，待后续 PR`
6. **本章节"已知偏差汇总"**与 `field-verification.md` 保持同步

**核心反问**：每个字段名如果拼错会怎样？是否需要备份多种拼写兼容（如 `Stations` / `stations`）？在 `field-verification.md` 每个命令章节末尾必须明确回答。

### 实机新发现（历史记录，2026-06-01，工业 PC 192.168.3.15）

> 第 2 步实机验证遗留证据。已被本章节"已知偏差汇总"和 [`field-verification.md`](docs/protocol/field-verification.md) 取代，仅作历史归档。

| 命令 | 请求 DataType | 响应 DataType | 响应结构 | 备注 |
|------|--------------|--------------|---------|------|
| `inl gsd list` | 13 | 13 ✅ | `Device[]` | 与 MVP 等价 |
| `inl topology get`（已重命名为 `device-run`） | 12 | 14（**by design**） | `Function.Devices[]` | 请求未带 `Function.Value`，服务端用默认行为 |

**`topology get` 实机响应**（保存于 `topology-get_response_20260601_103512.json`）：

```json
{
  "DataType": 14,
  "Function": {
    "Devices": [
      { "DeviceName": "heron-weld",    "Status": "连接断开" },
      { "DeviceName": "smc-weldsaver", "Status": "连接断开" }
    ],
    "TotalCount": 2,
    "Value": "GetActRun"
  }
}
```

**`Value: "GetActRun"`** = "Get Active Run"，返回当前活动运行的设备列表（焊机）。与 spec 原假设的"PROFINET 拓扑（Stations/Ports/Connections）"不同。

**`GetGSDFileNetwork` vs `GetGSDFileActivated`**：
- `GetGSDFileNetwork` = 回调当前**配置中**的网络拓扑中的 GSD 文件内容
- `GetGSDFileActivated` = 回调当前**生效**网络拓扑中的 GSD 内容
- 两者都是返回 GSD 文件内容，但来源不同（前者是设计师的配置、后者是运行时激活的）

---

## Skills 体系(借鉴飞书 CLI 架构)

> 第 4 步新增章节。AI Agent 接到 inl 相关任务时**自动加载**对应 Skill。

### Skills 入口

inl 配套 2 个 Skill 文档,位于 `feishu_cli/skills/`:

| Skill | 作用 | 加载时机 |
|-------|------|---------|
| [`inl-shared`](../skills/inl-shared/SKILL.md) | 共享规则入口(`--target` / Risk / `--yes` / `--dry-run` / 错误码) | **所有 inl 相关任务必读** |
| [`inl-workflow-profinet-write`](../skills/inl-workflow-profinet-write/SKILL.md) | 写操作 4 层安全流程(预检 / 备份 / 确认 / 验证 / 显式回滚) | AI 接到"修改 / 添加 / 删除 / 编译"工业 PC PROFINET 配置任务时 |

**CRITICAL**:`inl-workflow-profinet-write` 的 YAML frontmatter `metadata.requires.skills: ["inl-shared"]` 声明了硬依赖,**未读 `inl-shared` 前不要读 workflow skill**。

### inl 与 lark-cli 工作流的关键差异

> **必须了解**:工业 PC 命令与飞书云端 API 在可逆性上有本质区别,AI 调度行为不同。

| 维度 | Lark-CLI | inl |
|------|---------|-----|
| **操作对象** | 飞书云 API(消息 / 文档 / 日历) | 工业 PC 本地 JSON + 实时设备 |
| **可逆性** | 大部分可逆(云端版本历史 / owner 审计) | 写操作**不可逆**(本地 JSON 覆盖,无 undo) |
| **回滚机制** | 云端自动(版本回滚) | **必须手动**(inl 反向命令 / SCP 恢复) |
| **AI 调度** | write 类 AI 可自动追加 `--yes` 重试 | **必须区分** write(AI 可自动) vs high-risk-write(AI **不可自动**) |

**错误码映射**(借鉴 lark-cli 但增加 confirmation 分支):
- `yes_required` → AI 自动追加 `--yes` 重试(对应 lark-cli `RiskLevel=write`)
- **`confirmation_required` → AI 暂停,人工决策**(inl **新增**,lark-cli 无对应)
- `confirmation_required` 错误的 `Detail.ai_auto_yes == false` 是 AI 决策的硬约束

### 错误码总表(6 个)

| 错误码 | 触发条件 | AI 行为 | 错误码来源 |
|--------|---------|---------|----------|
| `target_required` | `--target` 为空 / 非法 IP | AI 提示用户加 `--target <IP>` | inl 第 3 步 |
| `yes_required` | write 命令缺 `--yes` | AI **自动追加** `--yes` 重试 | lark-cli `cli/cmd/service/service.go:182-185` |
| **`confirmation_required`** | high-risk-write 缺 `--yes` | AI **暂停**,**不**自动追加,告知用户 | lark-cli `cli/internal/cmdutil/confirm.go:29-41` |
| `unknown_command` | Registry 找不到命令 | AI 不重试,报错给用户 | inl 第 3 步 |
| `unexpected_response_command` | 响应命令字 ≠ 0x9271 | AI 不重试,可能服务端版本不匹配 | inl 第 3 步 |
| `protocol_error` | TCP 失败 / 帧解码失败 / CRC 校验失败 | AI 不重试,可能是物理连接问题 | inl 第 3 步 |

**工厂函数**(`inl/internal/output/errors.go`):
- `output.YesRequired(action) error` — 生成 `yes_required` 错误
- `output.ConfirmationRequired(action) error` — 生成 `confirmation_required` 错误(含 `ai_auto_yes: false`)

### 后续 PR 候选

> 本步骤(第 5 步)DCP 相关候选:
- **`device-setup` 副作用验证** — `device setup-name` / `device setup-ip` 成功后,自动跑 `topology scan` 验证名称/IP 是否变更(闭环验证,目前 inl 端无任何成功判断依据)
- **GSD 匹配降级算法** — `gsd match` 在 nrc2.out 不支持 DataType=16 时,自动降级为"AI 客户端按 VendorID+DeviceID 手动匹配 GSD 库"算法(已在 `inl-workflow-design.md §2.2` 备用)
- **DCP Block 原始帧调试** — `device setup` 失败时,记录 C++ 端 DCP 原始帧(非 JSON)到 `testdata/`,便于事后分析交换机过滤 / VLAN 隔离等问题
- **`SetIDevice` Registry 补登记** — 第 3 步临时移除,本步骤亦不涉及,按需补回
- **`SetIDevice` 与 `device-setup-name` 协同** — `device-setup-name` 修改 DCP 层名称,`SetIDevice` 修改 INL 配置层名称,两条命令应作为"配对新设备名"工作流的两步

> 前序步骤候选(继续保留):
- **`inl-workflow-profinet-config` Skill**(9 步配网业务流,与 `profinet-network-engineer` 对标)— 当 AI 接到"完整配网"任务时
- C++ 端 `PNConfigLibFileDesign` 在每个写命令处理前自动备份 `networktopology.json`
- 写命令的 `--data` 复杂 JSON 构造(device name / slot / IP 参数化)
- `inl backup` / `inl restore` 子命令(走 NRC 备份)
- 写命令 body 工厂扩展(BodyBuilder 重载)
- AI 自动回滚(低风险,如 `set-driver` / `shield`)
- `references/` 子目录拆分(当 workflow skill 超过 500 行)
