---
title: inl 第 5 步开发计划 — DCP 相关命令
tags: [inl, development, plan, dcp, profinet, device-discovery]
created: 2026-06-02
status: draft
---

# inl — 第 5 步：DCP 相关命令开发计划

## 目标

基于 io-controller C++ 源码 [`pndcp.cpp`](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/io-controller/src/pnconfiglib/pndcp.cpp) 中已实现的 DCP 功能，在 inl 中新增 **4 条 DCP 相关命令** + **1 个端口列表命令**，覆盖工作流设计中的 Phase 2（网络发现）和 Phase 7.3（DCP 参数分配）。

完成后的命令形态：

```bash
# Phase 2.0 — 端口发现
$ inl --target 192.168.3.15 interface list
🔌 已连接 192.168.3.15:6000
📤 发送 DataType=14, Function=4 (interface-list)
📥 收到端口列表:
  - enp4s0  | MAC: 68:ed:a6:0b:c4:3b | IP: 192.168.3.15
  - eth0    | MAC: 00:1b:21:ab:cd:ef | IP: 10.0.0.100

# Phase 2.1 — 设备发现
$ inl --target 192.168.3.15 topology scan --interface enp4s0
📤 发送 DataType=14, Function=1, Portname=enp4s0
📥 发现 3 台在线设备:
  [1] heron-weld     MAC: 00:11:22:33:44:55  IP: 192.168.2.10
      VendorID: 0x038A  DeviceID: 0x0030  Role: PN设备
  [2] smc-valve-01   MAC: aa:bb:cc:dd:ee:ff  IP: 192.168.2.20
      VendorID: 0x0083  DeviceID: 0x0011  Role: PN设备

# Phase 2.2 — GSD 匹配
$ inl --target 192.168.3.15 gsd match --interface enp4s0
📤 发送 DataType=16, Portname=enp4s0
📥 GSD 匹配结果:
  heron-weld   → OBARA SIV31-40 ✅
  smc-valve-01 → SMC EX245-SPN  ✅

# Phase 7.3 — DCP 参数分配
$ inl --target 192.168.3.15 device setup --interface enp4s0 \
    --mac 00:11:22:33:44:55 --name welder-01
📤 发送 DataType=14, Function=2 (set name)
✅ 设备名称已分配

$ inl --target 192.168.3.15 device setup --interface enp4s0 \
    --mac 00:11:22:33:44:55 --ip 192.168.2.20 --mask 255.255.255.0
📤 发送 DataType=14, Function=3 (set IP)
✅ 网络参数已分配
```

---

## 背景上下文

### C++ 源码分析

DCP 功能入口在 [`pndcp.cpp:117-182`](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/io-controller/src/pnconfiglib/pndcp.cpp#L117-L182) 的 `PerformOnlineAccess` 函数：

| Function.Value | 行为 | 必需参数 | 响应方式 | 对应 inl 命令 |
|:---:|------|---------|---------|------------|
| `4` | 获取本地网络接口列表 | 无 | NRC `0x9271` + JSON (DataType=14) | `interface list` |
| `1` | DCP 发现网络中所有 PROFINET 设备 | `Portname` | NRC `0x9271` + JSON (DataType=14) | `topology scan` |
| `2` | 设置设备名称 | `Portname` + `TargetMAC` + `Newdevicename` | 仅错误报告（通过 BYD_TriggerErrorReport），无 JSON 响应 | `device setup --name` |
| `3` | 设置设备 IP + 子网掩码 | `Portname` + `TargetMAC` + `Newipaddress` + `Newsubnetmask` | 仅错误报告，无 JSON 响应 | `device setup --ip` |

此外，GSD 匹配在 `FilterGSDCompatibleDevices` ([pndcp.cpp:L1348-L1390](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/io-controller/src/pnconfiglib/pndcp.cpp#L1348-L1390))：

| DataType | 行为 | 必需参数 | 响应方式 | 对应 inl 命令 |
|:---:|------|---------|---------|------------|
| `16` | 发现 + GSD 匹配 | `Portname` | NRC `0x9271` + JSON (DataType=16) | `gsd match` |

> ⚠️ **注意**：Function=2/3（set name/IP）是副作用操作——C++ 端直接收发 DCP 原始帧，**不通过 NRC 返回 JSON 响应**。成功时无显式确认，失败时通过 `BYD_TriggerErrorReport` 报告。inl 端的"成功"判断依据为：NRC 通信正常 + 无后续错误报告。

### Discovery 响应 JSON 结构（DataType=14, Function=1）

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

数据块类型映射（DCP Option/Suboption → 字段）：

| DCP Block (O,S) | 字段 | 说明 |
|:---:|------|------|
| (2, 1) | `DeviceVendorValue` | 设备厂商名称字符串 |
| (2, 2) | `DeviceName` | DCP 设备名称（经过 TransformDeviceNameBack 反转义） |
| (2, 3) | `VendorID`, `DeviceID` | PI 分配的 16-bit ID |
| (2, 4) | `DeviceRole` | 1=PN设备, 2=PN控制器, 4=PN多设备, 8=PN监视器 |
| (1, 2) | `IPAddress`, `SubNetMask`, `GateWay` | 设备网络参数 |

### Interface List 响应 JSON 结构（DataType=14, Function=4）

```json
{
  "DataType": 14,
  "PortName": ["enp4s0", "eth0"],
  "Mac":      ["68:ed:a6:0b:c4:3b", "00:1b:21:ab:cd:ef"],
  "IP":       ["192.168.3.15", "10.0.0.100"]
}
```

> 三个数组长度一致，按索引对应。过滤条件：IP 非空且 MAC ≠ `00:00:00:00:00:00`。

### GSD Match 响应 JSON 结构（DataType=16）

```json
{
  "DataType": 16,
  "Devices": [
    {
      "Mac": "00:11:22:33:44:55",
      "DeviceVendorValue": "OBARA Corporation",
      "DeviceName": "heron-weld",
      "VendorID": "0x038A",
      "DeviceID": "0x0030",
      "DeviceRole": "PN设备"
    }
  ]
}
```

> 与 Function=1 的结构相同，但只包含与 GSD 库中 VendorID+DeviceID 匹配的设备。

---

## 命令设计

### 新增 3 个 CommandGroup

```go
const (
    GroupInterface  CommandGroup = "interface"
    GroupTopology   CommandGroup = "topology"
)
```

- `interface` — 工业 PC 网络端口（仅 `list`）
- `topology` — 拓扑扫描（仅 `scan`）
- `gsd` — 已有，新增 `match` 子命令
- `device` — 已有，新增 `setup` 子命令

### 命令参数设计

#### `interface list`

| 参数 | 类型 | 必需 | 说明 |
|------|------|:---:|------|
| `--target` | string | ✅ | 工业 PC IP（全局 PersistentFlag） |

#### `topology scan`

| 参数 | 类型 | 必需 | 说明 |
|------|------|:---:|------|
| `--target` | string | ✅ | 全局 |
| `--interface` | string | ✅ | DCP 扫描端口名（如 `enp4s0`） |

#### `gsd match`

| 参数 | 类型 | 必需 | 说明 |
|------|------|:---:|------|
| `--target` | string | ✅ | 全局 |
| `--interface` | string | ✅ | DCP 扫描端口名 |

#### `device setup`

| 参数 | 类型 | 必需 | 说明 |
|------|------|:---:|------|
| `--target` | string | ✅ | 全局 |
| `--interface` | string | ✅ | DCP 操作的端口名 |
| `--mac` | string | ✅ | 目标设备 MAC 地址 (格式 `XX:XX:XX:XX:XX:XX`) |
| `--name` | string | 二选一 | 新设备名称（触发 Function=2） |
| `--ip` | string | 与 `--mask` 配套 | 新 IP 地址（触发 Function=3） |
| `--mask` | string | 与 `--ip` 配套 | 新子网掩码（触发 Function=3） |

> `--name` 和 `--ip` 二选一。同时提供时，先执行 setName（Function=2），再执行 setIP（Function=3）。

---

## 文件清单

```
inl/
├── main.go                          ← 改: buildGroupCmd 新增 interface, topology 分支
├── internal/
│   ├── nrc/
│   │   └── commands.go              ← 改: Registry 新增 5 条 CommandSpec
│   ├── interface/                   ← 新增 package
│   │   ├── types.go                 ←   InterfaceListResponse struct
│   │   └── types_test.go            ←
│   ├── topology/                    ← 已有, 扩展
│   │   ├── types.go                 ←   新增 ScanResponse struct
│   │   └── types_test.go            ←
│   ├── device/                      ← 新增 package (或扩展现有 devicestatus/)
│   │   ├── types.go                 ←   DeviceInfo struct (DCP 发现结果)
│   │   └── types_test.go            ←
│   └── gsd/                         ← 已有, 扩展
│       ├── types.go                 ←   新增 MatchResponse struct
│       └── types_test.go            ←
└── testdata/                        ← 新增实机样本
    ├── interface-list_response_*.json
    ├── topology-scan_response_*.json
    ├── gsd-match_response_*.json
    └── (device-setup 无响应文件)
```

**新增文件**：3 个（interface/types.go + types_test.go, device/types.go + types_test.go 或扩展现有包）
**修改文件**：3 个（commands.go + main.go + topology/types.go + gsd/types.go）
**新增三方依赖**：无

---

## Step 1：Registry 新增 5 条命令

### 文件位置

`inl/internal/nrc/commands.go`（修改 Registry）

### 新增条目

```go
// === DataType=14: DCP 操作 (PerformOnlineAccess) ===
{
    Name:        "interface-list",
    Code:        0x9275,
    DataType:    14,
    Function:    "4",
    Direction:   DirectionRequest,
    Group:       GroupInterface,
    Description: "列出工业 PC 所有可用的网络端口",
    Risk:        RiskRead,
    BodyBuilder: DefaultBodyBuilder,
},
{
    Name:        "topology-scan",
    Code:        0x9275,
    DataType:    14,
    Function:    "1",
    Direction:   DirectionRequest,
    Group:       GroupTopology,
    Description: "DCP 发现网络中所有 PROFINET 设备",
    Risk:        RiskRead,
    BodyBuilder: topologyScanBody,
    Args:        []ArgumentSpec{{Name: "interface", Description: "DCP 扫描端口名", Required: true}},
},
{
    Name:        "gsd-match",
    Code:        0x9275,
    DataType:    16,
    Function:    "",
    Direction:   DirectionRequest,
    Group:       GroupGsd,
    Description: "匹配在线设备与 GSD 驱动库",
    Risk:        RiskRead,
    BodyBuilder: gsdMatchBody,
    Args:        []ArgumentSpec{{Name: "interface", Description: "DCP 扫描端口名", Required: true}},
},
{
    Name:        "device-setup-name",
    Code:        0x9275,
    DataType:    14,
    Function:    "2",
    Direction:   DirectionRequest,
    Group:       GroupDevice,
    Description: "通过 DCP 设置设备名称",
    Risk:        RiskWrite,
    BodyBuilder: deviceSetupBody,
    Args:        []ArgumentSpec{
        {Name: "interface", Description: "端口名", Required: true},
        {Name: "mac", Description: "目标 MAC", Required: true},
        {Name: "name", Description: "新设备名称", Required: true},
    },
},
{
    Name:        "device-setup-ip",
    Code:        0x9275,
    DataType:    14,
    Function:    "3",
    Direction:   DirectionRequest,
    Group:       GroupDevice,
    Description: "通过 DCP 设置设备 IP 和子网掩码",
    Risk:        RiskWrite,
    BodyBuilder: deviceSetupIPBody,
    Args:        []ArgumentSpec{
        {Name: "interface", Description: "端口名", Required: true},
        {Name: "mac", Description: "目标 MAC", Required: true},
        {Name: "ip", Description: "新 IP 地址", Required: true},
        {Name: "mask", Description: "新子网掩码", Required: true},
    },
},
```

### Function 字段说明

CommandSpec 的 `Function` 字段当前为 `string` 类型。DataType=14 的 DCP 命令中，C++ 端通过 `root["Function"] == 1/2/3/4` 做整数比较。有两种方案：

**方案 A（推荐）**：扩展现有 DefaultBodyBuilder

```go
func topologyScanBody(spec CommandSpec, args map[string]string) (string, error) {
    port := args["interface"]
    return fmt.Sprintf(
        `{"DataType":14,"Function":1,"Portname":"%s"}`, port), nil
}

func deviceSetupBody(spec CommandSpec, args map[string]string) (string, error) {
    port := args["interface"]
    mac := args["mac"]
    name := args["name"]
    return fmt.Sprintf(
        `{"DataType":14,"Function":2,"Portname":"%s","TargetMAC":"%s","Newdevicename":"%s"}`,
        port, mac, name), nil
}
```

**方案 B**：修改 DefaultBodyBuilder 支持整数 Function

两种方案都可以，方案 A 更简单，不需要改现有逻辑。

### 验收标准

- [ ] Registry 从 17 条扩展到 22 条
- [ ] GroupInterface=1, GroupTopology=1, GroupGsd=2, GroupDevice=7, GroupConfig=11
- [ ] `init()` 唯一性检查通过（DataType+Function 组合键无冲突）
- [ ] `go test ./internal/nrc/` 所有测试 PASS

---

## Step 2：领域模型

### 2.1 `internal/interface/types.go` — 端口列表

```go
package netiface

// ListResponse 是 DataType=14, Function=4 的响应结构。
//
// C++ 端 GeneralFunction::getInterfaceInfo() 返回 PortName/Mac/IP 三个对齐数组。
type ListResponse struct {
    DataType int      `json:"DataType"`
    PortName []string `json:"PortName"`
    Mac      []string `json:"Mac"`
    IP       []string `json:"IP"`
}

// Port 描述一个网络接口。
type Port struct {
    Name string `json:"name"`
    Mac  string `json:"mac"`
    IP   string `json:"ip"`
}

// Flatten 将对齐数组展开为 []Port。
func (r *ListResponse) Flatten() []Port {
    ports := make([]Port, len(r.PortName))
    for i := range r.PortName {
        ports[i] = Port{
            Name: r.PortName[i],
            Mac:  r.Mac[i],
            IP:   r.IP[i],
        }
    }
    return ports
}
```

### 2.2 `internal/device/types.go` — DCP 发现设备

```go
package device

// DCPDevice 是 DCP 发现返回的单台设备信息。
type DCPDevice struct {
    Mac              string `json:"Mac"`
    DeviceVendorValue string `json:"DeviceVendorValue"`
    DeviceName       string `json:"DeviceName"`
    VendorID         string `json:"VendorID"`
    DeviceID         string `json:"DeviceID"`
    DeviceRole       string `json:"DeviceRole"`
    IPAddress        string `json:"IPAddress"`
    SubNetMask       string `json:"SubNetMask"`
    GateWay          string `json:"GateWay"`
}
```

### 2.3 扩展现有包

- `internal/topology/types.go`：新增 `ScanResponse`（含 `Devices []device.DCPDevice`）
- `internal/gsd/types.go`：新增 `MatchResponse`（与 ScanResponse 同结构，DataType=16）

### 验收标准

- [ ] 3 个新 model struct 含中文注释
- [ ] 每个包至少 2 个 round-trip 测试 PASS
- [ ] `go build ./...` 通过

---

## Step 3：main.go 注册新 Group

### 文件位置

`inl/main.go`（修改 `buildGroupCmd` 和 `runNrcCommand`）

### 新增 Group 分支

```go
case nrc.GroupInterface:
    use = "interface"
    short = "工业 PC 网络端口"
    long = "工业 PC 网络端口管理 (只读)。"
    pureGroup = true
case nrc.GroupTopology:
    use = "topology"
    short = "PROFINET 拓扑管理"
    long = "PROFINET 拓扑扫描与管理 (只读)。"
    pureGroup = true
```

### main() 遍历新增

```go
for _, g := range []nrc.CommandGroup{
    nrc.GroupInterface, nrc.GroupGsd, nrc.GroupDevice, nrc.GroupConfig, nrc.GroupTopology,
} {
    rootCmd.AddCommand(buildGroupCmd(g))
}
```

### buildSubCmd 适配参数

当前的 `buildSubCmd` 自动从 `spec.Args` 构建 Positional Args：

```go
// 已有逻辑：Args: cobra.NoArgs → 改为支持 spec.Args
if len(spec.Args) > 0 {
    subCmd.Args = cobra.ExactArgs(len(spec.Args))
}
```

但 DCP 命令的参数更适合作 flags 而非 positional args。建议在 `buildSubCmd` 中增加 flag 注册：

```go
for _, arg := range spec.Args {
    if arg.Name == "interface" {
        subCmd.Flags().String("interface", "", "DCP 操作端口名 (如 enp4s0)")
        subCmd.MarkFlagRequired("interface")
    }
    // ... 其他 arg
}
```

### runNrcCommand 适配 args

```go
// 构建 args map 供 BodyBuilder 使用
args := make(map[string]string)
if port, _ := cmd.Flags().GetString("interface"); port != "" {
    args["interface"] = port
}
if mac, _ := cmd.Flags().GetString("mac"); mac != "" {
    args["mac"] = mac
}
// ...
body, err := nrc.RequestBody(spec, args)
```

### 验收标准

- [ ] `inl --help` 显示 `interface`, `gsd`, `device`, `config`, `topology` 五个 Group
- [ ] `inl interface --help` 显示 `list` 子命令
- [ ] `inl topology --help` 显示 `scan` 子命令（含 `--interface` 必填标志）
- [ ] `inl gsd --help` 显示 `list`, `match` 两个子命令
- [ ] `inl device --help` 显示 7 个子命令（5 原有 + 2 setup）
- [ ] `go build -o inl.exe` 成功

---

## Step 4：实机验证

### 环境

- 工业 PC：`192.168.3.15:6000`
- 至少 1 台 PROFINET 设备在线（如 `heron-weld`）
- 已知 PROFINET 总线端口名（如 `enp4s0`）

### 跑测命令

```bash
# 4.1 端口列表
inl --target 192.168.3.15 interface list

# 4.2 设备发现
inl --target 192.168.3.15 topology scan --interface enp4s0

# 4.3 GSD 匹配
inl --target 192.168.3.15 gsd match --interface enp4s0

# 4.4 设备名称设置（⚠️ 写操作，需 --yes）
inl --target 192.168.3.15 device setup --interface enp4s0 \
    --mac 00:11:22:33:44:55 --name test-device --yes

# 4.5 设备 IP 设置（⚠️ 写操作，需 --yes）
inl --target 192.168.3.15 device setup --interface enp4s0 \
    --mac 00:11:22:33:44:55 --ip 192.168.2.50 --mask 255.255.255.0 --yes

# ⚠️ 测试后将名称/IP 恢复原值
```

### 验收标准

- [ ] `inl interface list` 返回至少 2 个端口（含 enp4s0）
- [ ] `inl topology scan` 返回 ≥1 台设备，每台含 MAC/Name/IP/VendorID/DeviceID/Role
- [ ] `inl gsd match` 返回设备列表，已匹配的含 GSD 信息
- [ ] `inl device setup --name` 带 `--yes` 后无错误
- [ ] `inl device setup --ip` 带 `--yes` 后无错误
- [ ] 实机样本 JSON 归档到 `inl/testdata/`

---

## 完整验收清单

### 离线验收

- [ ] Registry 22 条（原 17 + 新 5）
- [ ] GroupInterface=1, GroupTopology=1, GroupGsd=2, GroupDevice=7, GroupConfig=11
- [ ] `go test ./internal/nrc/` 全 PASS
- [ ] `go test ./...` 全 PASS（新增 interface/device 包测试）
- [ ] `go vet ./...` 无警告
- [ ] `go build -o inl.exe` 成功
- [ ] `inl --help` 显示 5 个 Group
- [ ] `inl topology scan --help` 底部显示 Risk: read
- [ ] `inl device setup --help` 底部显示 Risk: write

### 实机验收

- [ ] `interface list` 返回端口列表（含 enp4s0）
- [ ] `topology scan` 返回在线设备列表
- [ ] `gsd match` 返回匹配结果
- [ ] `device setup --name --yes` 成功
- [ ] `device setup --ip --yes` 成功
- [ ] 实机样本 5 份 JSON 归档

---

## 风险与备注

| 风险 | 概率 | 应对 |
|------|:---:|------|
| `device setup` 无 JSON 响应，无法判断成功 | 高 | 命令返回后 AI 通过 `topology scan` 验证名称/IP 是否变更 |
| `gsd match` 在当前 nrc2.out 版本可能未注册 DataType=16 回调 | 中 | 先发请求测试，若不支持则降级为 "AI 客户端手动匹配"（已在工作流 §2.2 中有备用算法） |
| DCP 广播在部分网络环境下被交换机过滤 | 低 | 在拓扑 scan 失败时提示用户检查交换机设置 |

---

## 执行节奏

| Step | 内容 | 预计耗时 |
|------|------|---------|
| Step 1 | Registry 新增 5 条 + BodyBuilder | 45 分钟 |
| Step 2 | 4 个领域模型包 | 30 分钟 |
| Step 3 | main.go 适配 | 30 分钟 |
| Step 4 | 实机验证 | 30 分钟 |

**总共约 2 小时代码 + 30 分钟车间**。

---

## 相关文档

- [inl-workflow-design.md](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/docs/inl/inl-workflow-design.md) — 工作流设计（Phase 2, Phase 7.3）
- [inl-architecture.md](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/docs/inl/inl-architecture.md) — 架构设计
- [inl/AGENTS.md](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/inl/AGENTS.md) — 协议契约
- [pndcp.cpp](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/io-controller/src/pnconfiglib/pndcp.cpp) — C++ DCP 实现
- [pndcp.h](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/io-controller/src/pnconfiglib/pndcp.h) — C++ DCP 头文件
