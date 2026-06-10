---
title: inl P0 支线修复计划 — 领域模型修正 + SetIDevice 注册
tags: [inl, fix, p0, topology, setidevice, hotfix]
created: 2026-06-02
status: draft
---

# inl — P0 支线修复计划

## 目标

修正 [AGENTS.md](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/inl/AGENTS.md) "已知偏差汇总"中标为 **"完全错误"** 和 **"待 PR"** 的两个阻塞项，补齐第 18 条 DataType=12 命令。

| 偏差 | AGENTS.md 描述 | 影响 |
|------|-------------|------|
| `device-list` (CallBackJson) | `CallbackJsonResponse` 模型**完全错误**，需重写为 IDevice+PNDriver 结构 | Phase 1 `CurrentState.device_instances` 无法正确反序列化 → Phase 3 冲突检测依赖此数据 |
| `device-list-active` (CallBackActivatedJson) | `CallbackActivatedJsonResponse` type alias 错误，需独立 struct | 已有 `ActivatedTopologyResponse` ✅，本修复确认其与实机一致 |
| `SetIDevice` | C++ 源码存在但 inl 未注册 | 工作流中 IDevice 配置能力缺失 |

---

## 实机响应结构分析

### `device list` (CallBackJson) — [实机样本](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/inl/testdata/device-list_response_20260601_164426.json)

```json
{
  "DataType": 12,
  "Error": [],
  "ErrorID": [],
  "Function": "CallBackJson",
  "IDevice": {
    "Activate": false,
    "InputLength": 64,
    "OutputLength": 64
  },
  "PNDriver": {
    "DeviceName": "pndriver",
    "IPAddress": "192.168.2.14",
    "SetInTheProject": true,
    "SubnetMask": "255.255.255.0",
    "iDevice": false
  }
}
```

关键特征：
- `Function` 是**字符串**（不是嵌套对象）
- 无 `DecentralDevice` 数组（配置中拓扑没有设备时为空；理论上有设备时会出现）
- PNDriver 含 `iDevice` 字段（bool）
- 含 `Error` + `ErrorID` 空数组

### `device list-active` (CallBackActivatedJson) — [实机样本](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/inl/testdata/device-list-active_response_20260601_164518.json)

```json
{
  "DataType": 14,
  "DecentralDevice": [
    {
      "DeviceID": "0x0002",
      "DeviceName": "ex245",
      "IPAddress": "192.168.2.10",
      "InputLength": 8,
      "InputStartAddress": 0,
      "OutputLength": 8,
      "OutputStartAddress": 0,
      "ReductionRatio": 16,
      "RefGSD": "...",
      "SetInTheProject": true,
      "SubnetMask": "255.255.255.0",
      "VendorID": "0x0083",
      "Module": [
        {
          "ModuleName": "32 valves",
          "Slot": 2,
          "SubModule": [
            { "InputLength": 0, "InputStartAddress": 0,
              "OutputLength": 1, "OutputStartAddress": 0,
              "SubModuleName": "32 valves" }
          ]
        }
      ]
    }
  ],
  "Function": "CallBackActivatedJson",
  "IDevice": { "Activate": false, "InputLength": 0, "OutputLength": 0 },
  "PNDriver": { "DeviceName": "pndriver", "IPAddress": "192.168.2.14",
                "SetInTheProject": true, "SubnetMask": "255.255.255.0" },
  "TotalInputLength": 0,
  "TotalOutputLength": 8
}
```

关键特征：
- `DataType` 响应为 14（不是 12——C++ 端硬编码）
- 当前 `ActivatedTopologyResponse` 模型**已正确** ✅
- `PNDriver` 无 `iDevice` 字段（与 CallBackJson 不同）

---

## 修复项 1：`topology/types.go` — 新增 CallbackJsonResponse

### 问题

当前 `topology/types.go` 中只有 `ActivatedTopologyResponse`（对应 CallBackActivatedJson），没有 `CallbackJsonResponse`（对应 CallBackJson）。旧的 `Response` struct 含 `Stations []json.RawMessage` 已被标记为 Deprecated。

### 新增模型

```go
// CallbackJsonResponse 是 Function="CallBackJson" 响应 (配置中拓扑视角)。
//
// C++ 端含义: 回调当前网络配置文件中的设备配置信息。
// 顶层结构: DataType + Function + IDevice + PNDriver (+ DecentralDevice[]，当有设备时出现)
// 对应命令: `inl device list`。
type CallbackJsonResponse struct {
    DataType   int              `json:"DataType"`
    Function   string           `json:"Function"`
    Error      []interface{}    `json:"Error"`
    ErrorID    []interface{}    `json:"ErrorID"`
    IDevice    IDeviceInfo      `json:"IDevice"`
    PNDriver   PNDriverConfig   `json:"PNDriver"`
}

// PNDriverConfig 是 CallBackJson 响应中的 PNDriver 信息。
// 与 ActivatedTopologyResponse 中 PNDriverInfo 不同: 多了 iDevice 字段。
type PNDriverConfig struct {
    DeviceName      string `json:"DeviceName"`
    IPAddress       string `json:"IPAddress"`
    SetInTheProject bool   `json:"SetInTheProject"`
    SubnetMask      string `json:"SubnetMask"`
    IDevice         bool   `json:"iDevice,omitempty"`
}
```

> `DecentralDevice` 数组暂不放在此 struct 中——实机样本中该字段不存在。当配置中有设备时，该数组由 C++ 端动态追加，Go `json.Unmarshal` 对未知字段宽容，不会报错。

### 更新注释

- 删除 `Response` 的 Deprecated 标记，改为注释 "历史占位，不再使用"
- 在 package 注释中补充 CallbackJsonResponse 说明

### 单元测试

```go
func TestCallbackJsonResponseRoundTrip(t *testing.T) {
    raw := `{"DataType":12,"Error":[],"ErrorID":[],"Function":"CallBackJson","IDevice":{"Activate":false,"InputLength":64,"OutputLength":64},"PNDriver":{"DeviceName":"pndriver","IPAddress":"192.168.2.14","SetInTheProject":true,"SubnetMask":"255.255.255.0","iDevice":false}}`
    var resp CallbackJsonResponse
    if err := json.Unmarshal([]byte(raw), &resp); err != nil {
        t.Fatalf("Unmarshal failed: %v", err)
    }
    if resp.Function != "CallBackJson" {
        t.Errorf("Function = %q, want CallBackJson", resp.Function)
    }
    if resp.PNDriver.IPAddress != "192.168.2.14" {
        t.Errorf("PNDriver IP = %q", resp.PNDriver.IPAddress)
    }
    if resp.IDevice.InputLength != 64 {
        t.Errorf("IDevice InputLength = %d", resp.IDevice.InputLength)
    }
}

func TestCallbackJsonResponseHasIDeviceField(t *testing.T) {
    raw := `{"DataType":12,"Error":[],"ErrorID":[],"Function":"CallBackJson","IDevice":{"Activate":false,"InputLength":64,"OutputLength":64},"PNDriver":{"DeviceName":"pndriver","IPAddress":"192.168.2.14","SetInTheProject":true,"SubnetMask":"255.255.255.0","iDevice":true}}`
    var resp CallbackJsonResponse
    json.Unmarshal([]byte(raw), &resp)
    if !resp.PNDriver.IDevice {
        t.Error("iDevice should be true")
    }
}
```

---

## 修复项 2：`topology/types.go` — 确认 ActivatedTopologyResponse 正确性

### 现状

`ActivatedTopologyResponse` 字段与实机响应一致 ✅，无需修改。仅需补充注释说明此模型已通过实机验证。

### 新增注释

```go
// ActivatedTopologyResponse 是 Function="CallBackActivatedJson" 响应 (运行时激活视角)。
//
// ✅ 已通过实机验证 (2026-06-02, 工业 PC 192.168.3.15)。
// C++ 端含义: 回调当前运行时已激活的 PROFINET 设备列表, 含完整 Module/SubModule 层级。
// 对应命令: `inl device list-active`。
```

### 可选：提取共享类型

`IDeviceInfo` 在 `CallbackJsonResponse` 和 `ActivatedTopologyResponse` 中共用——保留在当前文件中即可，无需移动。

---

## 修复项 3：`internal/nrc/commands.go` — 注册 SetIDevice

### 背景

`SetIDevice` 在 C++ 源码 `NetWorkTopologyFunction` 中作为第 10 个 `Function.Value` 分支存在。inl 第 3 步时因"参数复杂度高"跳过注册。

### 新增条目

```go
{
    Name:        "config-set-idevice",
    Code:        0x9275,
    DataType:    12,
    Direction:   DirectionRequest,
    Description: "设置 IDevice IO 长度参数",
    Risk:        RiskWrite,
    Response:    nil,
    Function:    "SetIDevice",
    Group:       GroupConfig,
    Args:        nil,
    BodyBuilder: DefaultBodyBuilder,
},
```

> 当前阶段仅注册骨架。`SetIDevice` 的完整参数（IO 长度、Activate 状态）需要 `--data` 支持（规划中）。注册后该命令可参与 help 显示和 Risk 分级，后续 PR 补全参数构造能力。

### Registry 影响

- 17+5 → 23 条（DataType=12 从 16 条扩展到 17 条）
- GroupConfig 从 11 条扩展到 12 条
- 测试更新：`TestRegistryHas23Entries` 替换 `TestRegistryHas22Entries`

---

## 修复项 4：AGENTS.md 偏差表更新

| 偏差 | 旧状态 | 新状态 |
|------|:---:|:---:|
| `device-list` (CallBackJson) — 模型完全错误 | ⚠️ 待 PR | ✅ 已修正 |
| `device-list-active` (CallBackActivatedJson) — type alias 错误 | ⚠️ 待 PR | ✅ 已修正（确认模型正确） |
| `SetIDevice` — 未注册 | ⚠️ 待 PR | ✅ 已注册骨架 |

---

## 文件清单

```
inl/
├── internal/
│   ├── topology/
│   │   ├── types.go              ← 改: 新增 CallbackJsonResponse + PNDriverConfig,
│   │   │                             确认 ActivatedTopologyResponse 正确性,
│   │   │                             删除旧 Response 的误导性注释
│   │   └── types_test.go         ← 改: 新增 2 个回调测试
│   └── nrc/
│       ├── commands.go           ← 改: Registry 新增 SetIDevice (18→23)
│       └── commands_test.go      ← 改: 23 条测试 (原 22)
├── AGENTS.md                     ← 改: 偏差表更新 (3 条 → ✅)
└── docs/protocol/
    └── field-verification.md     ← 改: 标注 CallBackJson/CallBackActivatedJson/SetIDevice 已修正
```

**新增文件**: 0
**修改文件**: 5
**新增三方依赖**: 0

---

## 验收标准

### 离线验收

- [ ] `topology/types.go` 含 `CallbackJsonResponse` struct（6 字段: DataType/Function/Error/ErrorID/IDevice/PNDriver）
- [ ] `topology/types.go` 含 `PNDriverConfig` struct（5 字段，含 `iDevice`）
- [ ] `ActivatedTopologyResponse` 注释标记 "✅ 已通过实机验证"
- [ ] 旧 `Response` struct 注释注明 "历史占位，不再使用"
- [ ] 2 个 round-trip 测试 PASS（用实机 JSON 作 fixture）
- [ ] Registry 含 `config-set-idevice`（RiskWrite, Function="SetIDevice"）
- [ ] `go test ./...` 全部 9 包 PASS
- [ ] `go vet ./...` 零警告
- [ ] AGENTS.md 偏差表 3 项标记为 ✅
- [ ] field-verification.md 对应章节更新

### 实机验证

- [ ] `inl device list` 返回的 JSON 可通过 `CallbackJsonResponse` 反序列化
- [ ] `inl device list-active` 返回的 JSON 可通过 `ActivatedTopologyResponse` 反序列化

---

## 执行节奏

| 修复项 | 内容 | 预计耗时 |
|--------|------|:---:|
| 1 | topology/types.go 新增 CallbackJsonResponse + PNDriverConfig | 15 分钟 |
| 2 | 实机 JSON fixture 测试 | 10 分钟 |
| 3 | Registry 注册 SetIDevice + 测试更新 | 10 分钟 |
| 4 | AGENTS.md + field-verification.md 偏差表更新 | 10 分钟 |
| 5 | 实机验证 | 5 分钟 |

**总共约 45 分钟**。

---

## 相关文档

- [inl/AGENTS.md](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/inl/AGENTS.md#L382-L397) — 已知偏差汇总
- [inl/docs/protocol/field-verification.md](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/inl/docs/protocol/field-verification.md) — 实机响应核对
- [topology/types.go](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/inl/internal/topology/types.go) — 当前模型
- [device-list 实机响应](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/inl/testdata/device-list_response_20260601_164426.json)
- [device-list-active 实机响应](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/inl/testdata/device-list-active_response_20260601_164518.json)
