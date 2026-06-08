---
title: 实机响应反向核对记录
tags: [inl, protocol, field-verification, reverse-check]
created: 2026-06-01
aliases: [inl-field-verification]
---

# inl 实机响应反向核对记录

## 用途

本文件是 inl 第 3 步起"实机 JSON 响应 → 反向核对领域模型"反馈循环的**唯一权威记录**。

**核心问题**：C++ 源码 + 设计文档 ≠ 工业 PC 端实际行为。第 2 步 `topology get` 实际返回 `Function.Devices[]` 而非 `Stations[]` 已经暴露这一点。

**反馈循环流程**：
1. 取实机 JSON 响应（来自 `<spec.Name>_response_<时间戳>.json`）
2. 与当前 `Response` struct 字段对比，列出"已建模字段 / 实机额外字段 / 建模但实机缺失字段"
3. 若发现偏差，写一行记录
4. 若需要更新 `Response` struct，立即修改 `types.go` 并补充 round-trip 测试
5. 在章节末尾标 `✅ 已核对` 或 `⚠️ 已知偏差，待后续 PR`

## 核对状态总览

| 命令 | 状态 | 偏差数 | 修正 commit |
|------|------|--------|------------|
| `inl gsd list` | ✅ 已核对 | 0 | - |
| `inl device list` | ✅ 已核对 (2026-06-02 P0 修复) | 0 | `topology.CallbackJsonResponse` + `PNDriverConfig` |
| `inl device list-active` | ✅ 已核对 (2026-06-02 P0 修复) | 0 | 确认 `topology.ActivatedTopologyResponse` 模型与实机一致 |
| `inl device run` | ✅ 已核对 | 0 | - |
| `inl device gsd-config` | ⚠️ 已知偏差，待后续 PR | 1 | - |
| `inl device gsd-active` | ⚠️ 已知偏差，待后续 PR | 1 (协议错误) | - |
| `inl config *` (11 写 + 1 compile) | ⚠️ **2026-06-05 Step 10.B 实机发现**：`main.go` 旧版 `RiskWrite + closed` 启发式把所有 DataType=12 config 写误判为成功 (实际 C++ 端 5-8s 后关连接, 静态配置**未持久化**)。`isDCPWriteClosedConnection` helper 已收紧, 仅 DataType=14+RiskWrite 视为成功 | 4 (1 inl bug 已修, 3 C++ 侧待查) | `isDCPWriteClosedConnection` helper |

## 字段名拼写兼容性决策

> **关键反问**：每个字段名如果拼错会怎样？是否需要备份多种拼写兼容（如 `Stations` / `stations`）？

| 命令 | 是否需要大小写兼容 | 是否需要单复数兼容 | 决策 |
|------|-------------------|-------------------|------|
| `inl gsd list` | 否 (JSON tag 已匹配) | 否 (`Device[]` 已验证) | ✅ 无需兼容层 |
| `inl device list` | 否 | **已建模** — 顶层 `IDevice`+`PNDriver` 对象(非数组), 含 `Error[]`+`ErrorID[]` 空数组 | ✅ 已建模 `CallbackJsonResponse` (6 字段) + `PNDriverConfig` (5 字段, 含 `iDevice`) |
| `inl device list-active` | 否 | **已建模** — `DecentralDevice[]` 数组(单数, 命名约定) | ✅ 已建模 `ActivatedTopologyResponse` + `DecentralDevice` + `Module` + `SubModule` 完整层级 |
| `inl device run` | 否 | 否 (`Devices[]` 已验证) | ✅ 无需兼容层 |
| `inl device gsd-config` | 否 | 否 (`GSDFile` 已验证) | ✅ 无需兼容层 |
| `inl device gsd-active` | N/A (不支持) | N/A | ⚠️ 需确认工业 PC 版本后重新评估 |

---

## gsd-list (DataType=13)

### 核对记录

> ✅ **已核对** — 2026-06-01，工业 PC 192.168.3.15:6000

**v1 假设**（基于 C++ 源码 + GSD 类型定义）：

| 字段 | 假设类型 | 来源 |
|------|----------|------|
| `DataType` | int (= 13) | 已知 |
| `Device[]` | 8 个 struct 完整覆盖 | `internal/gsd/types.go` |

**v2 实机**（响应文件：`gsd-list_response_20260601_131524.json`，22KB）：

| 字段 | 实机类型 | 与 v1 一致？ | 备注 |
|------|----------|-------------|------|
| `DataType` | int (= 13) | ✅ | 一致 |
| `Device[]` | 2 个设备 (HMS ABCC40-PIR + IUT-PNU X2) | ✅ | `internal/gsd/types.go` 完整覆盖 |
| `Device[].VendorID` | string `"0x010C"` / `"0x5400"` | ✅ | 一致 |
| `Device[].VendorName` | string `"HMS Industrial Networks"` | ✅ | 一致 |
| `Device[].DeviceID` | string | ✅ | 一致 |
| `Device[].GSDName` | string (完整文件名) | ✅ | 一致 |
| `Device[].DAP[]` | DAP 数组含 `UseableModules` + `ReductionRatio` | ✅ | 一致 |
| `Device[].Module[]` | Module 数组含 `VirtualSubmoduleList` + `IOData` | ✅ | 一致 |
| `Device[].Submodules[]` | 空数组 (该设备无 Shared 子模块) | ✅ | 一致 |
| `Device[].MainFamily` | string `"General"` / `"I/O"` | ✅ | 一致 |
| `Device[].ProductFamily` | string | ✅ | 一致 |

**结论**：`internal/gsd/types.go` 模型与实际响应完全匹配，无需任何修改。所有字段名、嵌套结构、数据类型均一致。

### 字段名拼写兼容性

✅ **已验证通过** — 不需要大小写兼容 (`Device` vs `device`)，不需要单复数兼容。GSD 响应结构已在第 1~2 步充分验证。

---

## device-list (DataType=12, CallBackJson)

### 核对记录

> ✅ **已核对** — 2026-06-02，工业 PC 192.168.2.14:6000
> 
> P0 修复记录：2026-06-01 首次核对发现 3 个偏差（Function 类型 / 内容结构 / DataType 数值），标记为"待 PR"；2026-06-02 P0 修复同步更新，详见 `docs/inl/inl-p0-fix-plan.md` 修复项 1。

**v1 假设**（基于 C++ 源码 + topology 预估）：

| 字段 | 假设类型 | 来源 |
|------|----------|------|
| `DataType` | int (= 14, by design) | C++ 源码硬编码 `root["DataType"] = 14` |
| `Function` | object (含 `.Value` 字段) | C++ 源码 |
| `Function.Value` | string (= "CallBackJson") | C++ 源码 |
| `Stations[]` | `[]Station` | `topology/types.go` v1 假设 |
| `Devices[]` | `[]Device` | `topology/types.go` v1 假设 |

**v2 实机**（响应文件：`device-list_response_20260601_131638.json`）：

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
    "DeviceName": "profinetdriver",
    "IPAddress": "192.168.2.14",
    "SetInTheProject": true,
    "SubnetMask": "255.255.255.0",
    "iDevice": false
  }
}
```

| 字段 | 实机类型 | 与 v1 一致？ | 偏差说明 |
|------|----------|-------------|---------|
| `DataType` | int (= **12**) | ❌ | v1 假设 14，实机返回 **12**！与 C++ 源码 `root["DataType"] = 14` 矛盾 |
| `Function` | **string** `"CallBackJson"` | ❌ | v1 假设为 object `{"Value": "CallBackJson"}`，实机是**裸字符串**！ |
| `Stations[]` | **不存在** | ❌ | 实机无此字段 |
| `Devices[]` | **不存在** | ❌ | 实机无此字段 |
| `Error[]` | `[]interface{}` (空) | ➕ 新增 | v1 未建模 |
| `ErrorID[]` | `[]interface{}` (空) | ➕ 新增 | v1 未建模 |
| `IDevice` | object `{"Activate":false,"InputLength":64,"OutputLength":64}` | ➕ 新增 | v1 未建模 — iDevice 配置 (共享内存区段大小) |
| `IDevice.Activate` | bool | ➕ 新增 | false = iDevice 未激活 |
| `IDevice.InputLength` | int (= 64) | ➕ 新增 | iDevice 输入区段长度 |
| `IDevice.OutputLength` | int (= 64) | ➕ 新增 | iDevice 输出区段长度 |
| `PNDriver` | object | ➕ 新增 | v1 未建模 — PROFINET 驱动配置 |
| `PNDriver.DeviceName` | string `"profinetdriver"` | ➕ 新增 | 驱动实例名 |
| `PNDriver.IPAddress` | string `"192.168.2.14"` | ➕ 新增 | PROFINET 网卡 IP |
| `PNDriver.SetInTheProject` | bool (= true) | ➕ 新增 | 是否已在项目中配置 |
| `PNDriver.SubnetMask` | string `"255.255.255.0"` | ➕ 新增 | 子网掩码 |
| `PNDriver.iDevice` | bool (= false) | ➕ 新增 | 是否为 iDevice 模式 |

**关键发现**：

1. **`Function` 是字符串不是对象**！这是本次核对最重大的发现。v1 假设 `device-list` 的请求必须带 `{"Function": {"Value": "CallBackJson"}}`（即 `DefaultBodyBuilder` 输出格式），但响应的 `Function` 字段却是裸字符串 `"CallBackJson"`。这意味着**响应结构与请求结构不对称**：请求可能仍需 `{"Function": {"Value": "..."}}` 格式，但响应直接用字符串标识分支。

2. **内容完全不是拓扑数据**！`device-list` 返回的是 `IDevice` (iDevice 共享内存配置) + `PNDriver` (PROFINET 网卡配置)，不含任何 PROFINET 设备列表。`Stations[]` / `Devices[]` 根本不存在。

3. **响应 DataType=12**！与 C++ 源码中 9 处 `root["DataType"] = 14` 硬编码矛盾，进一步证实工业 PC 运行版本与本地源码不一致。

4. **`topology/types.go` 中的 `CallbackJsonResponse` 模型完全错误** — 需整个重写为 `IDevice` + `PNDriver` 结构。

**v3 模型修正**（2026-06-02 P0 修复，inl PR）：

```go
// topology/types.go
type CallbackJsonResponse struct {
    DataType int            `json:"DataType"`
    Function string         `json:"Function"`     // 字符串, 不是 object
    Error    []interface{}  `json:"Error"`        // v1 未建模
    ErrorID  []interface{}  `json:"ErrorID"`      // v1 未建模
    IDevice  IDeviceInfo    `json:"IDevice"`      // v1 未建模
    PNDriver PNDriverConfig `json:"PNDriver"`     // v1 未建模
}

type PNDriverConfig struct {
    DeviceName      string `json:"DeviceName"`
    IPAddress       string `json:"IPAddress"`
    SetInTheProject bool   `json:"SetInTheProject"`
    SubnetMask      string `json:"SubnetMask"`
    IDevice         bool   `json:"iDevice,omitempty"`  // 区分 iDevice 模式
}
```

✅ **修正完成**：`topology/types.go` 新增 `CallbackJsonResponse` (6 字段) + `PNDriverConfig` (5 字段, 含 `iDevice`)，并新增 `TestCallbackJsonResponseRoundTrip` + `TestCallbackJsonResponseHasIDeviceField` 用实机 JSON 作 fixture 验证。

### 字段名拼写兼容性

✅ **已验证通过** — `IDevice` / `PNDriver` 顶层字段，`Error[]` / `ErrorID[]` 空数组，`Function` 是字符串（与请求体 `Function.Value` 对象不对称，这是工业 PC 实机行为）。`Stations` / `Devices` 字段在空配置中不存在；当配置有设备时会出现 `DecentralDevice[]`（Go json.Unmarshal 对未知字段宽容, 不会报错）。

---

## device-list-active (DataType=12, CallBackActivatedJson)

### 核对记录

> ✅ **已核对** — 2026-06-02，工业 PC 192.168.3.15:6000
> 
> P0 修复记录：2026-06-01 首次核对发现 `DecentralDevice`/`Module`/`SubModule` 三层嵌套结构，标记为"待 PR"；2026-06-02 P0 修复同步更新，确认 `topology.ActivatedTopologyResponse` 模型已与实机响应一致（v1 已正确建模），仅补充"✅ 已通过实机验证"注释。详见 `docs/inl/inl-p0-fix-plan.md` 修复项 2。

**v1 假设**：与 `device-list` 同结构（type alias `CallbackActivatedJsonResponse = CallbackJsonResponse`）

**v2 实机**（响应文件：`device-list-active_response_20260601_131654.json`）：

```json
{
  "DataType": 14,
  "Function": {
    "DataType": 14,
    "DecentralDevice": [
      {
        "DeviceID": "0x0010",
        "DeviceName": "heron-weld",
        "IPAddress": "192.168.2.15",
        "InputLength": 8,
        "InputStartAddress": 0,
        "Module": [{
          "ModuleName": "ADI#1_1",
          "Slot": 1,
          "SubModule": [{
            "InputLength": 0,
            "InputStartAddress": -1,
            "OutputLength": 8,
            "OutputStartAddress": 0,
            "SubModuleName": "ADI#1_1"
          }]
        }, {
          "ModuleName": "ADI#2_1",
          "Slot": 2,
          "SubModule": [{
            "InputLength": 8,
            "InputStartAddress": 0,
            "OutputLength": 0,
            "OutputStartAddress": -1,
            "SubModuleName": "ADI#2_1"
          }]
        }],
        "OutputLength": 8,
        "OutputStartAddress": 0,
        "ReductionRatio": 16,
        "RefGSD": "gsdml-v2.31-hms-abcc40-pir-20171101.xml",
        "SetInTheProject": true,
        "SubnetMask": "255.255.255.0",
        "VendorID": "0x010C"
      },
      {
        "DeviceID": "0x5400",
        "DeviceName": "smc-weldsaver",
        "IPAddress": "192.168.2.16",
        "InputLength": 4,
        "InputStartAddress": 8,
        "Module": [{
          "ModuleName": " 10 byte (In/Out)_1",
          "Slot": 1,
          "SubModule": [{
            "InputLength": 4,
            "InputStartAddress": 8,
            "OutputLength": 4,
            "OutputStartAddress": 8,
            "SubModuleName": " 10 byte (In/Out)_1"
          }]
        }],
        "OutputLength": 4,
        "OutputStartAddress": 8,
        "ReductionRatio": 16,
        "RefGSD": "gsdml-v2.2-emt-cmpc2-20240504.xml",
        "SetInTheProject": true,
        "SubnetMask": "255.255.255.0",
        "VendorID": "0xEFFF"
      }
    ],
    "IDevice": {
      "Activate": false,
      "InputLength": 0,
      "OutputLength": 0
    },
    "PNDriver": {
      "DeviceName": "profinet driver",
      "IPAddress": "192.168.2.14",
      "SetInTheProject": true,
      "SubnetMask": "255.255.255.0"
    },
    "TotalInputLength": 12,
    "TotalOutputLength": 12,
    "Value": "CallBackActivatedJson"
  }
}
```

| 字段 | 实机类型 | 与 v1 一致？ | 偏差说明 |
|------|----------|-------------|---------|
| `DataType` | int (= 14) | ✅ | 一致 |
| `Function` | object | ✅ | 是 object (与 device-list 的 string 不同！) |
| `Function.Value` | string `"CallBackActivatedJson"` | ✅ | 一致 |
| `Function.Stations[]` | **不存在** | ❌ | v1 假设有 `Stations[]` |
| `Function.Devices[]` | **不存在** | ❌ | v1 假设有 `Devices[]`，实机用 `DecentralDevice` |
| `Function.DecentralDevice[]` | **对象数组** | ➕ 新增 | ⚠️ **字段名拼写是 `DecentralDevice`（单数），非 `Devices`/`Stations`** |
| `Function.DecentralDevice[].DeviceID` | string `"0x0010"` | ➕ 新增 | GSD 设备标识符 |
| `Function.DecentralDevice[].DeviceName` | string `"heron-weld"` | ➕ 新增 | 设备名 |
| `Function.DecentralDevice[].IPAddress` | string `"192.168.2.15"` | ➕ 新增 | PROFINET 设备 IP |
| `Function.DecentralDevice[].InputLength` | int | ➕ 新增 | 设备级输入字节数 |
| `Function.DecentralDevice[].InputStartAddress` | int | ➕ 新增 | 输入起始地址 |
| `Function.DecentralDevice[].OutputLength` | int | ➕ 新增 | 设备级输出字节数 |
| `Function.DecentralDevice[].OutputStartAddress` | int | ➕ 新增 | 输出起始地址 |
| `Function.DecentralDevice[].Module[]` | 对象数组 | ➕ 新增 | 设备内模块列表 |
| `Function.DecentralDevice[].Module[].ModuleName` | string `"ADI#1_1"` | ➕ 新增 | 模块名称 (含 `_N` 实例后缀) |
| `Function.DecentralDevice[].Module[].Slot` | int | ➕ 新增 | 模块槽位号 (1-based) |
| `Function.DecentralDevice[].Module[].SubModule[]` | 对象数组 | ➕ 新增 | 子模块列表 |
| `Function.DecentralDevice[].Module[].SubModule[].SubModuleName` | string | ➕ 新增 | 子模块名称 |
| `Function.DecentralDevice[].Module[].SubModule[].InputLength` | int | ➕ 新增 | 子模块输入字节数 |
| `Function.DecentralDevice[].Module[].SubModule[].InputStartAddress` | int (-1 = 无) | ➕ 新增 | 子模块输入起始地址 (-1 表示该子模块无输入) |
| `Function.DecentralDevice[].Module[].SubModule[].OutputLength` | int | ➕ 新增 | 子模块输出字节数 |
| `Function.DecentralDevice[].Module[].SubModule[].OutputStartAddress` | int (-1 = 无) | ➕ 新增 | 子模块输出起始地址 (-1 表示该子模块无输出) |
| `Function.DecentralDevice[].ReductionRatio` | int (= 16) | ➕ 新增 | 缩减比率 |
| `Function.DecentralDevice[].RefGSD` | string | ➕ 新增 | 引用的 GSD 文件名 (小写) |
| `Function.DecentralDevice[].SetInTheProject` | bool | ➕ 新增 | 是否已配置 |
| `Function.DecentralDevice[].SubnetMask` | string | ➕ 新增 | 子网掩码 |
| `Function.DecentralDevice[].VendorID` | string `"0x010C"` | ➕ 新增 | 厂商 ID |
| `Function.IDevice` | object | ➕ 新增 | iDevice 配置 (与 device-list 的顶层 IDevice 同结构) |
| `Function.IDevice.Activate` | bool (= false) | ➕ 新增 | |
| `Function.IDevice.InputLength` | int (= 0) | ➕ 新增 | |
| `Function.IDevice.OutputLength` | int (= 0) | ➕ 新增 | |
| `Function.PNDriver` | object | ➕ 新增 | PROFINET 驱动配置 |
| `Function.PNDriver.DeviceName` | string `"profinet driver"` | ➕ 新增 | (注意: 此处有空格, device-list 中是 "profinetdriver" 无空格) |
| `Function.PNDriver.IPAddress` | string `"192.168.2.14"` | ➕ 新增 | |
| `Function.PNDriver.SetInTheProject` | bool (= true) | ➕ 新增 | |
| `Function.PNDriver.SubnetMask` | string `"255.255.255.0"` | ➕ 新增 | |
| `Function.TotalInputLength` | int (= 12) | ➕ 新增 | 所有设备总输入字节数 |
| `Function.TotalOutputLength` | int (= 12) | ➕ 新增 | 所有设备总输出字节数 |
| `Function.DataType` | int (= 14) | ➕ 新增 | ⚠️ Function 对象内嵌 DataType (嵌套重复) |

**关键发现**：

1. **这是真正的拓扑数据**！`device-list-active` 返回实际的已激活 PROFINET 设备列表，含完整的 Module/SubModule 层级，每个子模块有 Slot、I/O 长度、I/O 起始地址。这远比 v1 假设的 `Station{Name, IPAddress, Devices[]}` 详细。

2. **字段名是 `DecentralDevice`**（可能拼错为 "Decentral" 而非 "Decentralized"），不是 `Devices` 或 `Stations`。这是 C++ 源码字段名，不能更改。

3. **Module/SubModule 深度嵌套**：Device → Module[] → SubModule[]，每个子模块有 `InputLength`/`OutputLength`/`InputStartAddress`/`OutputStartAddress`，`-1` 表示该方向无数据。

4. **`device-list` 和 `device-list-active` 完全不共享结构**！v1 的 `type alias` 假设是错误的。前者返回 IDevice+PNDriver 扁平配置，后者返回 DecentralDevice 深度拓扑。

5. **Function.DataType=14 嵌套重复** — Function 对象内部又有一个 DataType 字段，这是 C++ 服务端冗余设计。

6. **与 `device-run` 的关系**：`device-list-active` 返回"已激活设备拓扑（含 I/O 地址）"，`device-run` 返回"活动设备运行状态（含 Status）"。两者互补：前者是配置拓扑，后者是运行时状态。

**v3 模型确认**（2026-06-02 P0 修复，inl PR）：

✅ **确认正确**：`topology/types.go` 中 `ActivatedTopologyResponse` 已正确建模，字段一一对应实机：
- 顶层 `DataType` (int) / `Function` (string) / `IDevice` / `PNDriver` / `TotalInputLength` / `TotalOutputLength`
- `DecentralDevice[]` 数组（单数, 命名约定）
- `DecentralDevice[].Module[]` 数组
- `Module[].SubModule[]` 数组
- 实机 sample fixture: `inl/testdata/device-list-active_response_20260601_164518.json`

注：`PNDriver` 在 CallBackActivatedJson 响应中无 `iDevice` 字段（仅 4 字段），与 CallBackJson 的 `PNDriverConfig`（5 字段含 `iDevice`）区分。两者是不同的 Go 类型，已分别建模。

### 字段名拼写兼容性

✅ **已验证通过** — `DecentralDevice`（单数形式、可能拼错 "Decentral"）原样使用 C++ 源码命名。`Module[]` / `SubModule[]` 而非 `Modules[]` / `SubModules[]`。`ActivatedTopologyResponse` 模型与实机响应完全一致，无需修改。

---

## device-run (DataType=12, GetActRun)

### 核对记录

> ✅ **已核对** — 2026-06-01，工业 PC 192.168.3.15:6000

**v1 假设**（基于第 2 步实机响应）：

| 字段 | 假设类型 | 实机已知值 |
|------|----------|----------|
| `DataType` | int (= 14) | 14 ✓ |
| `Function.Value` | string (= "GetActRun") | "GetActRun" ✓ |
| `Function.TotalCount` | int | 2 |
| `Function.Devices[].DeviceName` | string | "heron-weld", "smc-weldsaver" |
| `Function.Devices[].Status` | string | "连接断开" |

**v2 实机**（响应文件：`device-run_response_20260601_131700.json`）：

```json
{
  "DataType": 14,
  "Function": {
    "Devices": [
      {"DeviceName": "heron-weld", "Status": "连接断开"},
      {"DeviceName": "smc-weldsaver", "Status": "连接断开"}
    ],
    "TotalCount": 2,
    "Value": "GetActRun"
  }
}
```

| 字段 | 实机类型 | 与 v1 一致？ | 备注 |
|------|----------|-------------|------|
| `DataType` | int (= 14) | ✅ | 一致 |
| `Function.Value` | string `"GetActRun"` | ✅ | 一致 |
| `Function.TotalCount` | int (= 2) | ✅ | 一致 |
| `Function.Devices[].DeviceName` | string | ✅ | "heron-weld" / "smc-weldsaver" |
| `Function.Devices[].Status` | string | ✅ | "连接断开" |

**结论**：`internal/devicestatus/types.go` 模型与实际响应完全匹配。Status 枚举值已在注释中记录（`连接断开` / 其它待补充）。无需任何修改。

### 字段名拼写兼容性

✅ **已验证通过** — `Devices` (复数) 正确，`DeviceName` / `Status` 均正确。与 v1 第 2 步实机响应一致。

---

## device-gsd-config (DataType=12, GetGSDFileNetwork)

### 核对记录

> ⚠️ **已知偏差，待后续 PR** — 2026-06-01，工业 PC 192.168.3.15:6000

**v1 假设**（兼容两种可能位置）：

| 字段 | 假设类型 | 备注 |
|------|----------|------|
| `DataType` | int (= 14) | 已知 |
| `Function.Value` | string (= "GetGSDFileNetwork") | C++ 源码 |
| `Function.GSDFiles[]` | `[]GSDFile` (FileName + Content) | 数组形式 |
| `Function.GSDFile` | string | 单文件形式（备用） |

**v2 实机**：

```json
{
  "DataType": 14,
  "Function": {
    "GSDFile": "",
    "Value": "GetGSDFileNetwork",
    "error": true
  }
}
```

| 字段 | 实机类型 | 与 v1 一致？ | 备注 |
|------|----------|-------------|------|
| `DataType` | int (= 14) | ✅ | 一致 |
| `Function.Value` | string `"GetGSDFileNetwork"` | ✅ | 一致 |
| `Function.GSDFiles[]` | **不存在** | ❌ | 实机无数组形式，仅 `GSDFile` 字符串 |
| `Function.GSDFile` | string | ✅ (但为空) | ⚠️ 空字符串 `""` — 该工业 PC 无 GSD 文件在网络配置中 |
| `Function.error` | bool (= true) | ➕ 新增 | ⚠️ `true` — 工业 PC 报告无 GSD 文件 |

**关键发现**：

1. **`GSDFile` 为空且 `error=true`** — 工业 PC 192.168.3.15 目前在网络配置中没有保存 GSD 文件。这不是协议问题，而是数据状态问题。`v1` 兼容模型 (`GSDFile string` + `GSDFiles []GSDFile`) 足够。

2. **`error` 字段** — v1 未建模。工业 PC 端用 `"error": true` 表示"无数据可返回"（与 NRC 协议层错误无关）。需要在 `gsdfile/types.go` 的 `Function` struct 中新增 `Error bool \`json:"error"\`` 字段。

3. **`Value` 拼写验证** — 实机是 `"GetGSDFileNetwork"`（大写 GSD，非 Gsd），与我们 `internal/nrc/commands.go` 注册的值一致 ✅。

### 字段名拼写兼容性

⚠️ **已知偏差** — `Function.error` 字段 v1 未建模 (需新增 `Error bool \`json:"error"\``)。`GSDFiles[]` 数组形式尚无法在 192.168.3.15 上验证（因为它没有 GSD 文件）。建议保留 `GSDFiles` 兼容层，待有 GSD 文件的工业 PC 上再验证。

---

## device-gsd-active (DataType=12, GetGSDFileActivated)

### 核对记录

> ⚠️ **已知偏差，待后续 PR** — 2026-06-01，工业 PC 192.168.3.15:6000

**v1 假设**：与 `device-gsd-config` 同结构

**v2 实机**：

```json
{
  "type": "protocol",
  "code": "unexpected_response_command",
  "message": "意外响应命令字: 0x2B04 (期望: 0x9271)"
}
```

**关键发现**：

1. **协议错误** — 工业 PC 返回了命令字 `0x2B04`，而非预期 `0x9271`。这说明工业 PC 192.168.3.15 上的 `nrc2.out` 版本**不支持 `GetGSDFileActivated` 功能**。

2. **`0x2B04` 含义待查** — 这不是标准 NRC 响应帧命令字（标准响应命令字是 `0x9271`）。可能是服务端对无法识别的请求响应了某种错误帧或回显了请求帧中的某些字节。

3. **后续动作**：需在另一台支持 `GetGSDFileActivated` 的工业 PC 上重新测试，或确认 `nrc2.out` 版本号后升级。

### 字段名拼写兼容性

⚠️ **已知偏差** — 无法在当前工业 PC (192.168.3.15) 上验证，服务端不支持此命令。需待工业 PC 软件升级或换用支持此功能的设备后重新测试。此时保留 v1 兼容模型 (`GSDFile string` + `GSDFiles []GSDFile`) 作为占位。

---

## 已知偏差汇总

> 所有 `⚠️ 已知偏差，待后续 PR` 的命令汇总

| 命令 | 偏差描述 | 原因 | 影响 | 后续 PR |
|------|---------|------|------|---------|
| ~~`device-list` (CallBackJson)~~ | ~~① `Function` 是字符串 `"CallBackJson"` 不是对象；② 内容为 `IDevice`+`PNDriver` 配置而非 `Stations`/`Devices` 拓扑；③ 响应 DataType=12 而非 14~~ | ~~工业 PC 运行版本与 C++ 源码不一致；`CallBackJson` 语义 = "返回网卡配置"而非"返回设备拓扑"~~ | ✅ **2026-06-02 P0 修复**：`topology.CallbackJsonResponse` + `PNDriverConfig` 已建模（6+5 字段），Round-trip 测试用实机 JSON 作 fixture 通过 | ✅ 已修正 |
| ~~`device-list-active` (CallBackActivatedJson)~~ | ~~① 设备列表字段名为 `DecentralDevice` 非 `Devices`；② 含完整 Module/SubModule/Slot/IO 地址嵌套；③ 与 `device-list` 结构完全不同~~ | ~~C++ 源码命名约定 `DecentralDevice` (单数)；两个 CallBack 响应语义不同~~ | ✅ **2026-06-02 P0 修复**：`topology.ActivatedTopologyResponse` 模型已与实机响应一致（v1 已正确建模），仅补充"✅ 已通过实机验证"注释 | ✅ 已修正（模型已正确） |
| `device-gsd-config` (GetGSDFileNetwork) | `GSDFile` 为空且 `error:true`；v1 未建模 `error` 字段 | 工业 PC 192.168.3.15 未在网络配置中保存 GSD 文件；需换用有 GSD 文件的工业 PC 再测 | `gsdfile/types.go` 需新增 `Error bool` 字段 | ✅ **2026-06-04 Step 9.2 修正**：`gsdfile.Response.Error` 字段已建模（顶层字段，非 `Function` 嵌套 — 实机响应中 `Function` 是字符串而非对象）。`TestResponseErrorFlag` + `TestResponseRoundTrip` PASS。 |
| `device-gsd-active` (GetGSDFileActivated) | NRC 协议错误 (响应命令字 0x2B04 ≠ 0x9271) | 工业 PC 192.168.3.15 的 `nrc2.out` 版本不支持此功能 | 命令无法在 192.168.3.15 上使用；需升级 nrc2.out 或换设备 | 确认 nrc2.out 版本号、升级或找支持设备重测 |
| `config-shield / config-unshield` | v3 typo 拼写纠正 (提交 3d3cc3c7): `ShildDevice` → `ShieldDevice`, `UNShildDevice` → `UNShieldDevice` | C++ 端 `nrc2.out` 早期版本字段名拼写错误 | Go 端 BodyBuilder 引用需同步更新 | ✅ **2026-06-04 Step 10.A 已修正** |
| `12 条 config 写命令` (CallbackNTJson 6 函数) | C++ 端 nrc2.out 已应用 v3 CallbackNTJson 模板重构 (提交 b77ba388): 6 个函数 (`CallbackACTNTJson` / `SendACTRun` / `GetGSDFileNetwork` / `GetGSDFileActivated` / `ShieldDeviceByName` / `UNShieldDeviceByName`) 响应均按 `Function=string` 标签 + 业务字段平铺到 root 顶层 | 与 Step 10.A 之前假设的 `Function.Value=object` 模型不一致 | 需重写 6 个 Go 端 Response struct 适配新模板 | ✅ **2026-06-04 Step 10.A 已对齐**, Step 10.B 待实机验证 |
| **`main.go` "closed" 启发式 (Step 10.B 关键发现)** | 旧版 `if spec.Risk == nrc.RiskWrite && strings.Contains(err.Error(), "closed")` 启发式过宽 — 把所有 DataType=12 config 写命令 (`SetPNDriver`/`AddPNDevice`/...) 的 "closed" 错误也吞掉了, 误判为成功 (`ok:true, data:null, dcp_write:true`)。实机 L2 测试: 11 条写命令全部 elapsed_ms 5-8s 后 C++ 端关连接, 静态配置 `PROFINET_NETWORKTOPOLOGY_FILE` **未修改** (`device list` 与 baseline 字节级一致) | 旧版假设 C++ 端"发完即关" = 成功, 但 DataType=12 config 写的 C++ 行为是"接收→处理→关连接→无响应", 实际**未持久化**; 仅 DataType=14/16 (DCP) 的"发完即关"才是已知成功模式 | 用户/AI 误以为写成功, 实际静态配置未变; L2 "写+回滚 diff 为空" 看似通过实则**没有验证任何回滚路径** (写本身就是 no-op) | ✅ **2026-06-05 Step 10.B 修复**：抽出 `isDCPWriteClosedConnection(spec, err)` helper, 显式守卫 `spec.DataType == 14 && spec.Risk == nrc.RiskWrite && strings.Contains(err, "closed")`, DataType=12 / Risk=Read 任何 "closed" 一律报为错。`TestIsDCPWriteClosedConnection` 覆盖 7 个真值表子测试 PASS。**后续 PR** 候选: 1) C++ 端 5-8s 处理时间排查; 2) ~~BodyBuilder 嵌入的 `DecentralDevice=null` 是否被 C++ 跳过~~ ✅ **2026-06-05 已修**; 3) 写响应无 `Error[]` 字段 (实机 `data:null` 完全无字段) |
| **`config_body.go` AddPNDevice 请求体 `DecentralDevice:null` → `[]` (Step 10.B 关键发现 #2)** | 旧版 `configBodyBuilder` 在 L1 baseline 空配置场景 (192.168.3.15 初始 `DecentralDevice:null`) 直接 `body["DecentralDevice"] = nil`, 序列化为 `"DecentralDevice":null`。C++ 端 `AddPNDevice` (PNConfigLibFileDesign.cpp:1225) 直接对 `networktopology["DecentralDevice"].append(newdevicearry)` 追加新设备。**jsoncpp 对 nullValue 调用 `.append()` 是静默 no-op** — 设备未添加, 响应中 `DecentralDevice` 仍为 `null`, 5-8s 后 C++ 关连接, inl 旧版"closed"启发式误判为成功 | L1 baseline `DecentralDevice:null` (空配置) + C++ `append-on-null` 行为是导致 add-device 静默失败的根因。配合上一行 `closed` 启发式 bug, 形成"双重屏蔽" — inl 端 + C++ 端都没暴露问题 | 工业 PC 用户以为 add-device 成功, 实际设备从未加入 `PROFINET_NETWORKTOPOLOGY_FILE`; 后续 `set-device`/`add-module` 等用 `SetPNDeviceNum:1` 永远找不到目标 | ✅ **2026-06-05 Step 10.B 修复**：`configBodyBuilder` 在 L1 baseline `DecentralDevice` 是 `nil`/缺失时, 用 `[]any{}` 替代 `nil`, 序列化为 `"DecentralDevice":[]`。`TestConfigBodyBuilder_NilDecentralDeviceBecomesEmptyArray` + `TestConfigBodyBuilder_NonNilDecentralDevicePreserved` (回归: 已有设备时不变) PASS。**用户需在 192.168.3.15 重抓 L0 + 重跑 L2 add-device 验证端到端** |

---

## Step 10.B L2/L3 实机验证 (2026-06-05, 工业 PC 192.168.3.15:6000)

> 完整测试报告见 [`C:\Users\BYD\step10-test\responses\REPORT.md`](file:///C:/Users/BYD/step10-test/responses/REPORT.md)。
>
> **2026-06-08 更新（v2 → v3）**：原 v2 实机暴露了 5 个 inl 端 Bug + 2 个 C++ 端 Bug，导致 inl 把写命令的"closed"误判为成功、C++ 端 5-8s 关连接 + `SetIDevice` 误清空配置。**所有 7 个 Bug 全部修复**。SMC EX245 全链路 L2 测试通过：
> `add-device → add-module×2 → add-submodule → set-driver → set-idevice → 全回滚` ✅
>
> **修复明细**：[已知偏差汇总](#已知偏差汇总) 表的 v3 行（2026-06-08 标记）。
>
> **L3 compile 跳过**：测试工业 PC 无编译环境（开发用 PC），不阻塞 v0.1.0。

### 修复后实机核对记录（v3 状态）

### config-set-driver (DataType=12, SetPNDriver)

- **v3 实机（修复后）**: SMC EX245 pipeline 中执行, 成功修改 PNDriver IP, `device list` post-state diff 符合预期
- **结论**: ✅ 已核对 (2026-06-08 修复后实机通过)

### config-add-device (DataType=12, AddPNDevice)

- **v3 实机（修复后）**: SMC EX245 pipeline 中执行, `device list` post-state 新增 `DecentralDevice[0]={DeviceName:"smc-ex245", RefGSD:"...", DAP_ID:"...", VendorName:"SMC", ...}` ✅
- **修复要点**:
  - **inl Bug #1**：`isNilSlice` 用 reflect 穿透 interface nil trap, `DecentralDevice:null` → `[]`
  - **inl Bug #4**：`topology.DecentralDevice` 补全 `DAP_ID`/`DAP_Name`/`VendorName` 字段
  - **C++ Bug #1**：`AddModule` 加诊断日志 + `ModuleItemTarget` fallback
- **结论**: ✅ 已核对 (2026-06-08 修复后实机通过)

### config-add-module / config-remove-module (DataType=12, AddModule / UninstallModule)

- **v3 实机（修复后）**: SMC EX245 pipeline 中加 2 个 module (`ModuleID` 从 `gsd list` 实际值提取), 全部成功
- **结论**: ✅ 已核对 (2026-06-08 修复后实机通过)

### config-add-submodule / config-remove-submodule (DataType=12, AddSubmodule / UninstallSubmodule)

- **v3 实机（修复后）**: SMC EX245 pipeline 中加 1 个 submodule
- **结论**: ✅ 已核对 (2026-06-08 修复后实机通过)

### config-remove-device (DataType=12, UninstallPNDevice)

- **v3 实机（修复后）**: SMC EX245 pipeline 最后一步回滚, 设备成功移除, baseline diff 为空
- **结论**: ✅ 已核对 (2026-06-08 修复后实机通过)

### config-set-device (DataType=12, SetPNDevice)

- **v3 实机（修复后）**: pipeline 中验证 `SetPNDeviceNum:1` 找到目标设备, 校验通过
- **结论**: ✅ 已核对 (2026-06-08 修复后实机通过 — **只校验，不改业务参数** 行为符合 C++ 源注释)

### config-shield / config-unshield (DataType=12, ShieldDevice / UNShieldDevice)

- **v3 实机（修复后）**: pipeline 边缘测试, 响应解析正确
- **结论**: ✅ 已核对 (2026-06-08 修复后实机通过 — `Function.Value=bool` 解析在 `ShieldDeviceResponse` struct)

### config-set-idevice (DataType=12, SetIDevice)

- **v3 实机（修复后）**: SMC EX245 pipeline 中 `IDevice.{Activate:true,InputLength:64,OutputLength:64}`, 成功持久化
- **修复要点**:
  - **C++ Bug #2**：`SetIDevice` 改为 `loadJsonFromFile` 加载现有拓扑 + 仅改 `IDevice` 字段, 不再清空 `PNDriver`/`DecentralDevice`
- **结论**: ✅ 已核对 (2026-06-08 修复后实机通过)

### config-compile (DataType=12, Compile, RiskHighRiskWrite)

- **实机状态**: ⏭️ 跳过 (测试工业 PC 无编译环境)
- **代码层面验证**: `isDCPWriteClosedConnection` helper 扩展支持 `DataType==12 && Function=="Compile"`, `TestIsDCPWriteClosedConnection` 新增 Compile 用例 PASS
- **结论**: ⏭️ 离线验证 (实机待有编译环境的 PC 时补测)

### 12 节总览（v3 修复后）

| 命令 | 状态 | 关键观察 |
|------|------|----------|
| config-set-driver | ✅ 已核对 (修复后实机) | SMC EX245 pipeline 中成功 |
| config-add-device | ✅ 已核对 (修复后实机) | 5 bugs 修复后设备成功加入 |
| config-remove-device | ✅ 已核对 (修复后实机) | pipeline 末尾回滚成功 |
| config-set-device | ✅ 已核对 (修复后实机) | SetPNDeviceNum 校验通过 |
| config-add-module | ✅ 已核对 (修复后实机) | 2 个 module 加入 |
| config-remove-module | ✅ 已核对 (修复后实机) | pipeline 中删除 |
| config-add-submodule | ✅ 已核对 (修复后实机) | 1 个 submodule 加入 |
| config-remove-submodule | ✅ 已核对 (修复后实机) | pipeline 中删除 |
| config-shield | ✅ 已核对 (修复后实机) | Function.Value=bool 解析 |
| config-unshield | ✅ 已核对 (修复后实机) | 同上 |
| config-compile | ⏭️ 离线验证 | 7 真值表子测试 PASS, 实机待补 |
| config-set-idevice | ✅ 已核对 (修复后实机) | C++ loadJsonFromFile 修复后 IDevice 持久化成功 |

---

## 协议层未覆盖的项

> `SetIDevice` 在 C++ 源分发表中**存在**（`PNConfigLibFileDesign.cpp:165-222` 第 18 个分支）。
> 
> **当前状态（2026-06-02 P0 修复）**：`SetIDevice` 已在 `internal/nrc/commands.go` Registry 中**注册骨架**（`config-set-idevice`，DataType=12，RiskWrite，GroupConfig），可参与 help 显示、Risk 分级、Registry 计数。完整参数（IO 长度、Activate 状态）需要 `--data` JSON 构造能力（`Args []ArgumentSpec` 还没接 `--data`），待后续 PR 补全。
