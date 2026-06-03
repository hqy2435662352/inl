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
| `inl device list` | ⚠️ 已知偏差，待后续 PR | 3 | - |
| `inl device list-active` | ⚠️ 已知偏差，待后续 PR | 4 | - |
| `inl device run` | ✅ 已核对 | 0 | - |
| `inl device gsd-config` | ⚠️ 已知偏差，待后续 PR | 1 | - |
| `inl device gsd-active` | ⚠️ 已知偏差，待后续 PR | 1 (协议错误) | - |

## 字段名拼写兼容性决策

> **关键反问**：每个字段名如果拼错会怎样？是否需要备份多种拼写兼容（如 `Stations` / `stations`）？

| 命令 | 是否需要大小写兼容 | 是否需要单复数兼容 | 决策 |
|------|-------------------|-------------------|------|
| `inl gsd list` | 否 (JSON tag 已匹配) | 否 (`Device[]` 已验证) | ✅ 无需兼容层 |
| `inl device list` | 否 | **是** — 实机无 `Stations[]`/`Devices[]`，而是 `IDevice`+`PNDriver` 顶层字段 | ⚠️ 模型完全错误，需重写 |
| `inl device list-active` | 否 | **是** — 实机用 `DecentralDevice` 非 `Devices`/`Stations` | ⚠️ 需兼容 `DecentralDevice` |
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

> ⚠️ **已知偏差，待后续 PR** — 2026-06-01，工业 PC 192.168.3.15:6000

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

### 字段名拼写兼容性

⚠️ **已知偏差** — `Stations` / `Devices` 字段不存在于实机响应中。实机响应字段为 `IDevice` / `PNDriver`。`Function` 是字符串不是对象，与请求体的 `Function` 结构不对称。`topology/types.go` 中的 `CallbackJsonResponse` 需完全重写。

---

## device-list-active (DataType=12, CallBackActivatedJson)

### 核对记录

> ⚠️ **已知偏差，待后续 PR** — 2026-06-01，工业 PC 192.168.3.15:6000

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

### 字段名拼写兼容性

⚠️ **已知偏差** — `DecentralDevice`（单数形式、可能拼错 "Decentral"）必须原样使用。`Module[]` / `SubModule[]` 而非 `Modules[]` / `SubModules[]`。`topology/types.go` 中的 `CallbackActivatedJsonResponse` 模型需完全重写，不可用 type alias 复用 `CallbackJsonResponse`。

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
| `device-list` (CallBackJson) | ① `Function` 是字符串 `"CallBackJson"` 不是对象；② 内容为 `IDevice`+`PNDriver` 配置而非 `Stations`/`Devices` 拓扑；③ 响应 DataType=12 而非 14 | 工业 PC 运行版本与 C++ 源码不一致；`CallBackJson` 语义 = "返回网卡配置"而非"返回设备拓扑" | `topology/types.go` 中 `CallbackJsonResponse` 模型**完全错误**，需重写为 IDevice+PNDriver 结构 | 新增 `internal/idevice/types.go` 或重命名 `topology` 包 |
| `device-list-active` (CallBackActivatedJson) | ① 设备列表字段名为 `DecentralDevice` 非 `Devices`；② 含完整 Module/SubModule/Slot/IO 地址嵌套；③ 与 `device-list` 结构完全不同 | C++ 源码命名约定 `DecentralDevice` (单数)；两个 CallBack 响应语义不同 | `topology/types.go` 中 `CallbackActivatedJsonResponse` type alias 错误，需独立 struct | 重写 `CallbackActivatedJsonResponse`，含 DecentralDevice + Module + SubModule 完整层级 |
| `device-gsd-config` (GetGSDFileNetwork) | `GSDFile` 为空且 `error:true`；v1 未建模 `error` 字段 | 工业 PC 192.168.3.15 未在网络配置中保存 GSD 文件；需换用有 GSD 文件的工业 PC 再测 | `gsdfile/types.go` 需新增 `Error bool` 字段 | 新增 `Error bool \`json:"error"\`` 到 `gsdfile.Function` |
| `device-gsd-active` (GetGSDFileActivated) | NRC 协议错误 (响应命令字 0x2B04 ≠ 0x9271) | 工业 PC 192.168.3.15 的 `nrc2.out` 版本不支持此功能 | 命令无法在 192.168.3.15 上使用；需升级 nrc2.out 或换设备 | 确认 nrc2.out 版本号、升级或找支持设备重测 |
