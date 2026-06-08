---
title: inl 第 10 步开发计划 — Config 写命令参数化 + 实机系统测试
tags: [inl, development, plan, validation, e2e, field-test, config-write]
created: 2026-06-04
updated: 2026-06-08
status: ✅ COMPLETED (L0-L2 实机通过, L3 离线验证)
---

# inl — 第 10 步：Config 写命令参数化 + 实机系统测试

> **2026-06-04 重大更新**：基于 [PNConfigLibFileDesign.cpp L147-229 分派器 + 各分派函数实现](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/io-controller/src/pnconfiglib/PNConfigLibFileDesign.cpp) 的逐函数反推，重写了 §1.5 / §2.1 / §2.3 / §2.4 / §2.5 / §3.5 / §3.7。
> **字段参照**（权威）：[inl-config-field-reference.md](inl-config-field-reference.md) — 11 条命令的精确字段路径 + 5 个关键设计发现。
>
> **2026-06-04 v3 更新**（基于提交 3d3cc3c7 + b77ba388）：
> 1. **常量拼写纠正**：ShieldDevice/UNShieldDevice 改回正确拼写（之前 v2 误用 ShildDevice/UNShildDevice typo）
> 2. **CallbackNTJson 模板**（参考 [PNConfigLibFileDesign.cpp L280-291](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/io-controller/src/pnconfiglib/PNConfigLibFileDesign.cpp)）：6 个函数重构到新模板（Function=string 标签, 业务字段平铺到 root）
> 3. **请求体 vs 响应体不对称**（关键设计约束）：
>    - 请求体仍用旧模板（Function 是 object 含 Value + 业务字段）— C++ NetWorkTopologyFunction 分发器 L158/165 仍读 `Function.Value`
>    - 响应体用新模板（Function=string, 业务字段在 root）— ShieldDevice/UNShieldDevice 已是新结构
> 4. **inl 实施差异**：
>    - inl `ShieldDeviceResponse` 改为新结构（DataType/Function(string)/DeviceName/Result(bool)）
>    - inl `configShieldBody`/`configUnshieldBody` 仍按旧模板构造请求体（因为 C++ 分发器读旧模板）
>    - inl 4 个读命令响应模型（devicestatus/gsdfile/topology.ActivatedTopologyResponse）本就是新结构，无需改动

## 目标

**填补 Step 2-9 遗留的"虚完成"**：12 条 `config-*` 写命令**从未在工业 PC 上端到端验证过**，且**11 条缺乏参数化能力**（AI Agent 实际无法用）。本步先把所有 config 写命令补到"参数化 + 实机可验证"，再打 **v0.1.0** tag。

> 本步**禁止远程触发任何命令**（`git push` / `npm publish` / `goreleaser release`），所有操作由用户在工业 PC 现场 + 工作区手动执行。

---

## 背景：复盘 Step 2-9 的"虚完成"

### 关键发现（2026-06-04 复盘）

| # | 命令 | Args | 单元测试 | 实机测试 | 实际可用？ |
|:--:|------|:--:|:--:|:--:|:--:|
| 1 | config-set-driver | **nil** | BodyBuilder 字符串拼接 | ❌ | ❌ 无法指定新 IP |
| 2 | config-add-device | **nil** | 同上 | ❌ | ❌ 无法指定设备名/IP/GSD |
| 3 | config-remove-device | **nil** | 同上 | ❌ | ❌ 无法指定设备名 |
| 4 | config-set-device | **nil** | 同上 | ❌ | ❌ 无法指定目标设备/新参数 |
| 5 | config-add-module | **nil** | 同上 | ❌ | ❌ 无法指定设备/模块 |
| 6 | config-remove-module | **nil** | 同上 | ❌ | ❌ 同上 |
| 7 | config-add-submodule | **nil** | 同上 | ❌ | ❌ 同上 |
| 8 | config-remove-submodule | **nil** | 同上 | ❌ | ❌ 同上 |
| 9 | config-shield | **nil** | 同上 | ❌ | ❌ 无法指定设备 |
| 10 | config-unshield | **nil** | 同上 | ❌ | ❌ 同上 |
| 11 | **config-compile** | nil | 同上 | ❌ | ✅ (无参数, 可执行) |
| 12 | config-set-idevice | ✅ (Step 9.1) | ✅ + 4 测试 | ❌ | ✅ (Step 9.1 参数化) |

**事实链**：

1. Step 2-3 注册了 12 条 config 命令，但 `Args: nil` + `DefaultBodyBuilder` 意味着用户跑 `inl config set-driver --yes` 时，inl 实际发出去的请求体是 `{"DataType":12,"Function":{"Value":"SetPNDriver"}}`——**没有新 IP、新 Name、新 GSD**。C++ 端拿到这个请求，**根本不知道要改什么**。
2. 单元测试只验证了 BodyBuilder **字符串拼接逻辑正确**，从不验证"工业 PC 收到这个请求后会发生什么"。
3. AI Agent 拿到"把焊机 IP 改成 192.168.2.20"任务时，调 `inl config set-device --yes` **实际不会改任何东西**——这是 inl 当前最大的"虚完成"。
4. 仅有 `config-set-idevice`（Step 9.1）和 `config-compile`（无参数需要）真正可用。

### 为什么 Step 9 没发现

- Step 9 重点是稳定性、分发、体验补完，**未触及 11 条 config 命令的参数化缺口**。
- 单元测试全绿，盲区被"测试覆盖 95%+"的进度数据掩盖。
- 用户的 skills (write) 设计了 4 层安全工作流，但所有指令都假设"命令能传参"。

---

## 一、范围拆分

本步分两个**顺序子任务**，子任务 1 是子任务 2 的前置：

### 子任务 10.A：参数化（不需实机）

把 11 条 config 写命令（除 compile 外）补上 Args 规格 + 专用 BodyBuilder，让 inl 真正能传业务参数。

**风险**：纯代码变更，不连工业 PC，0 风险。

### 子任务 10.B：实机系统测试（需工业 PC）

在 192.168.3.15 上做端到端测试，每条命令：dry-run 抓帧 → 备份 → 执行 → 抓响应 → 状态 diff → 回滚。修正任何发现的 BodyBuilder / Response struct 偏差。

**风险**：会真实修改工业 PC 配置。必须**严格按回滚预案**执行。

---

## 二、子任务 10.A：参数化设计

### 2.0 5 个 C++ 源码揭示的关键设计发现（影响所有 BodyBuilder）

> 来源：[inl-config-field-reference.md](inl-config-field-reference.md) §0 — 基于 [PNConfigLibFileDesign.cpp](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/io-controller/src/pnconfiglib/PNConfigLibFileDesign.cpp) 11 个分派函数的逐行分析。

| # | 发现 | 设计影响 |
|:--:|------|----------|
| **F1** | **字段路径两种模式**：SetPNDriver 字段在**顶层** `networktopology["PNDriver"]`，其他 10 条在 `networktopology["Function"]` 下 | BodyBuilder 必须按 `spec.Function` 决定字段位置，**不能**统一规整到 Function 下 |
| **F2** | **写命令的上下文依赖**：C++ 函数（如 UninstallPNDevice L2706）不重读 PROFINET_NETWORKTOPOLOGY_FILE，直接使用传入的 `DecentralDevice[]` 数组操作 | inl 必须**先 `device-list` 拉配置**（**不是** `device-list-active`！），把 DecentralDevice/PNDriver/IDevice 灌到请求体 |
| **F3** | **错误模型不统一**：SetIDevice 用 `Error: string` / `ErrorID: int`（**单值**），其他 10 条用 `Error: string[]` / `ErrorID: int[]`（**数组**） | Response struct 需容忍两种（`json.RawMessage` 或 `any` 类型） |
| **F4** | **1-based 索引**：SetPNDeviceNum/SetModuleSlot/SetSubmoduleSlot 全部 1-based | AI 看到 DecentralDevice[0] 时传 SetPNDeviceNum=1；连续删除会改变索引 |
| **F5** | **ShieldDevice/UnshieldDevice 响应特殊**：`Function.Value` 被**复用**为 `true/false` 成功标志（覆盖 dispatch 用法），无 `Error[]` / `ErrorID[]` | ShieldDevice 响应解析需特殊分支 |

**配置 vs 生效的区分**（C++ 源 L189-191, 197-199）：

| 命令 | C++ 端读取的文件 | inl 命令 |
|------|------------------|----------|
| `CallBackJson` | **PROFINET_NETWORKTOPOLOGY_FILE**（**配置**） | `device list` |
| `CallBackActivatedJson` | **PROFINET_ACTIVATED_NETWORKTOPOLOGY_FILE**（**生效**） | `device list-active` |

> ⚠️ 写命令修改的是**配置**。Step 10 BodyBuilder 的自动 fetch 必须用 `device-list`，**不能**用 `device-list-active`。

**还需 10.B 实机验证的假设**（C++ 端 nrc2.out 服务端行为，未在 PNConfigLibFileDesign.cpp 中直接出现）：

- 假设 A：nrc2.out 收到写命令请求时，**先**从 PROFINET_NETWORKTOPOLOGY_FILE 加载配置，**再**用请求体覆盖 `Function.*` 字段，然后才调 NetWorkTopologyFunction。如是，inl **不需要**在请求体里塞 DecentralDevice[]，BodyBuilder 只需发 `{"DataType":12,"Function":{"Value":"...","...":...}}`。
- 假设 B：nrc2.out 透传请求体，**不**预加载配置。如是，inl **必须**先 fetch 并把 DecentralDevice[] 嵌入请求体。
- **10.B L0 dry-run 抓帧是验证此假设的关键步骤**。建议实现 BodyBuilder 时**同时支持两种模式**（默认自动 fetch，`--no-fetch` 标志让 AI 在已持有 topology 时跳过），10.B 实机验证哪种是真实的。

> **✅ v2 实机确认（2026-06-04）**：**假设 B 确认**。inl **必须**把当前 `DecentralDevice[]` 嵌入请求体，否则 nrc2.out 处理 `UninstallPNDevice` 等写命令时遍历 `networktopology["DecentralDevice"]` 会得到空数组/缺失字段，写操作失败或行为不正确。
>
> **实施状态**：`internal/nrc/config_body.go` 的 `fetchTopologyForConfig` 默认实现已替换为真实 NRC 客户端调用（2026-06-04 完成）：
> - `nrc.NewClient(addr).Connect()` 连接到 `target:6000`（自动追加端口）
> - 发送 `device-list` 请求：`{"DataType":12,"Function":{"Value":"CallBackJson"}}`
> - 解析响应到 `topology.CallbackJsonResponse`（v2 已建模 `DecentralDevice` 字段）
> - 提取 `PNDriver` / `IDevice` / `DecentralDevice` 三字段并转为 `map[string]any`
> - 供 `configBodyBuilder` 在步骤 4 自动嵌入请求体顶层
> - 失败不阻塞（与原设计一致）：错误由 `configBodyBuilder` 吞掉, 请求体照常发出
>
> **单测覆盖**：`internal/nrc/fetch_topology_test.go`（mock TCP server）+ `internal/nrc/fetch_topology_integration_test.go`（build tag `integration`, 需 `INL_INTEGRATION_TARGET` 环境变量, 默认不跑）
>
> **拓扑类型补全**：`topology.CallbackJsonResponse` 已新增 `DecentralDevice []DecentralDevice \`json:"DecentralDevice,omitempty"\`` 字段（与 `ActivatedTopologyResponse` 同类型）。`omitempty` 保证现有无该字段的响应（空配置场景）round-trip 不变。
>
> **影响范围**：
> - 10.A 测试 `TestCallbackJsonResponseRoundTrip` / `TestCallbackJsonResponseHasIDeviceField` 仍 PASS（omitempty 保护）
> - 写命令的 `configBodyBuilder` 现在默认会自动调真实 fetch（之前是 mock 注入的 fetch）
> - 离线测试：mock server 模式无外部依赖
> - 实机测试：build tag `integration` 走真实 nrc2.out

### 2.1 11 条命令的请求体字段

> **唯一权威字段表**：[inl-config-field-reference.md §1](inl-config-field-reference.md) — 由 C++ 源码逐函数反推。
>
> 不在本计划文档复制完整表格，避免双源不一致。本节只列**摘要**。

| 命令 | Function.Value (v2 拼写) | 业务字段路径 | 必填 | 路径模式 | 错误模式 | 特殊 |
|------|----------------|--------------|:--:|:--:|----------|------|
| config-set-driver | `SetPNDriver` | `PNDriver.{DeviceName,IPAddress,SubnetMask,SetInTheProject}` | 4 | **B 顶层 `PNDriver`** | Error[] | 模式 B |
| config-add-device | `AddPNDevice` | `Function.{RefGSD,DAP_ID}` | 2 | A | Error[] | 其他字段 C++ 自动生成 |
| config-remove-device | `UninstallPNDevice` | `Function.SetPNDeviceNum` | 1 | A | Error[] | 1-based 索引 |
| config-set-device | `SetPNDevice` | `Function.SetPNDeviceNum` | 1 | A | Error[] | **只做校验，不改业务参数**（C++ L1561） |
| config-add-module | `AddModule` | `Function.{SetPNDeviceNum,ModuleID}` | 2 | A | Error[] | 1-based |
| config-remove-module | `UninstallModule` | `Function.{SetPNDeviceNum,SetModuleSlot}` | 2 | A | Error[] | |
| config-add-submodule | `AddSubmodule` | `Function.{SetPNDeviceNum,ModuleID,SubmoduleID}` | 3 | A | Error[] | |
| config-remove-submodule | `UninstallSubmodule` | `Function.{SetPNDeviceNum,SetModuleSlot,SetSubmoduleSlot}` | 3 | A | Error[] | |
| config-shield | `ShieldDevice` (v3 拼写纠正) | `Function.DeviceName` | 1 | A | **无** | **请求**模式 A, **响应**v3 CallbackNTJson (Function=string, Result 在 root, DeviceName 在 root) + `DataType=14` |
| config-unshield | `UNShieldDevice` (v3 拼写纠正) | `Function.DeviceName` | 1 | A | **无** | **请求**模式 A, **响应**v3 CallbackNTJson (Function=string, Result 在 root, DeviceName 在 root) + `DataType=14` |
| config-set-idevice | `SetIDevice` | `IDevice.{Activate,InputLength,OutputLength}` | 3 | **C 顶层 `IDevice`** | **单值** | Error: string，ErrorID: int |
| config-compile | `Compile` | (无) | 0 | - | - | 无 `--data`，10.B 验证响应 |

> **参数化设计原则**：每条命令加 `--data <json>` flag（与 raw-send、set-idevice 一致），AI Agent 传业务字段 JSON object。BodyBuilder 接收 `--data` 后，按 spec.Function 和路径模式决定字段放置位置，并按 §2.0 F2 自动 fetch 当前配置合并。

### 2.2 Registry 改造模式（以 set-driver 为例）

```go
{
    Name:        "config-set-driver",
    Code:        0x9275,
    DataType:    12,
    Direction:   DirectionRequest,
    Description: "设置 PROFINET 控制器 (PN Driver) 参数 (DeviceName/IPAddress/SubnetMask/SetInTheProject)",
    Risk:        RiskWrite,
    Response:    nil,
    Function:    "SetPNDriver",
    Group:       GroupConfig,
    Args: []ArgumentSpec{
        {Name: "data", Description: "PNDriver JSON (含 DeviceName/IPAddress/SubnetMask/SetInTheProject)", Required: true},
    },
    BodyBuilder: configSetDriverBody,
},
```

### 2.3 BodyBuilder 通用模式（基于 §2.0 的 5 个发现设计）

```go
// configBodyBuilder 通用写命令 BodyBuilder。
// 适用于 11 条 config 写命令, 差异仅在:
//   - 字段路径模式 (mode A: Function 下 / mode B: 顶层 PNDriver / IDevice)
//   - 必填字段校验规则
//   - Function.Value 字符串
//
// 流程 (按 §2.0 F1-F4 设计):
//   1. 解析 --data 为业务字段 JSON
//   2. 校验必填字段 (避免 C++ 端静默失败)
//   3. 字段放置: mode A → Function.<key> ; mode B → 顶层 PNDriver / IDevice
//   4. 默认自动 fetch 当前配置 (--no-fetch 可禁用, 见 §2.0 假设 A/B)
//   5. 拼装最终 JSON body
func configBodyBuilder(spec CommandSpec, args map[string]string) (string, error) {
    // 步骤 1: 解析 --data
    data := args["data"]
    if data == "" {
        return "", fmt.Errorf("--data 不能为空")
    }
    if !json.Valid([]byte(data)) {
        return "", fmt.Errorf("--data 不是合法 JSON")
    }
    var business map[string]any
    if err := json.Unmarshal([]byte(data), &business); err != nil {
        return "", fmt.Errorf("--data 解析失败: %w", err)
    }

    // 步骤 2: 校验必填字段 (按 spec 定义的字段集)
    if err := validateRequiredFields(spec, business); err != nil {
        return "", err
    }

    // 步骤 3 + 4: 拼装请求体
    body := map[string]any{"DataType": 12}

    // 模式 B: 字段在顶层 PNDriver (SetPNDriver)
    if spec.Function == "SetPNDriver" {
        body["PNDriver"] = business
        body["Function"] = map[string]any{"Value": spec.Function}
        return jsonEncode(body)
    }

    // 模式 A: 字段在 Function 下 (其他 9 条: ShildDevice/UNShildDevice/AddPNDevice/AddModule/AddSubmodule/UninstallPNDevice/UninstallModule/UninstallSubmodule/SetPNDevice)
    fnObj := map[string]any{"Value": spec.Function}
    for k, v := range business {
        fnObj[k] = v
    }
    body["Function"] = fnObj

    // 模式 C: 字段在顶层 IDevice (SetIDevice)
    if spec.Function == "SetIDevice" {
        body["IDevice"] = business
    }

    // 步骤 4 (可选): 自动 fetch 当前配置 (假设 B 场景)
    if !args["no-fetch"] == "true" {  // 默认 fetch
        topology, err := fetchDeviceList(args["target"])  // 调 device-list 拉配置
        if err == nil {
            // 灌 DecentralDevice / PNDriver / IDevice
            for _, k := range []string{"DecentralDevice", "PNDriver", "IDevice"} {
                if v, ok := topology[k]; ok {
                    body[k] = v
                }
            }
        }
        // fetch 失败不阻塞, 让请求体也能发出 (nrc2.out 可能已预加载)
    }

    return jsonEncode(body)
}
```

**关键设计点**：

1. **`--data` 字段命名约定**：
   - 模式 A 命令的 `--data` 字段直接对应 C++ 路径（`RefGSD` / `DAP_ID` / `SetPNDeviceNum` 等）
   - 模式 B 命令（SetPNDriver）的 `--data` 字段对应 `PNDriver` 子对象字段（`DeviceName` / `IPAddress` / `SubnetMask` / `SetInTheProject`）
   - AI 不需要知道模式 A/B，BodyBuilder 按 `spec.Function` 自动路由

2. **1-based 索引校验**（§2.0 F4）：`validateRequiredFields` 对 `SetPNDeviceNum` / `SetModuleSlot` / `SetSubmoduleSlot` 字段额外校验 `>= 1`

3. **自动 fetch 设计**（§2.0 F2 + 假设 A/B）：
   - 默认开启 fetch（`--no-fetch` 标志可禁用）
   - fetch 失败**不阻塞**——nrc2.out 可能已预加载（假设 A），inl 也可能已有 topology
   - 10.B L0 dry-run 抓帧**验证**哪种是真实的

4. **ShieldDevice/UnshieldDevice 响应特殊**（§2.0 F5）：BodyBuilder 输出与模式 A 一致（`Function.Value` + `Function.DeviceName`），但响应解析（在 main.go / Response struct）需要识别 `Function.Value` 是 bool 而非 string

### 2.4 11 条 BodyBuilder 模式表

| BodyBuilder | spec.Function | --data 必填字段 | 模式 | 自动 fetch | 特殊处理 |
|---|---|---|:--:|:--:|---|
| `configSetDriverBody` | SetPNDriver | DeviceName/IPAddress/SubnetMask/SetInTheProject | **B 顶层** | ✅ | 字段置 `PNDriver` 顶层 |
| `configAddDeviceBody` | AddPNDevice | RefGSD/DAP_ID | A | ✅ | 其他字段 C++ 自动生成 |
| `configRemoveDeviceBody` | UninstallPNDevice | SetPNDeviceNum (≥1) | A | ✅ | 1-based 索引校验 |
| `configSetDeviceBody` | SetPNDevice | SetPNDeviceNum (≥1) | A | ✅ | **只校验，不改参数**（10.B 验证） |
| `configAddModuleBody` | AddModule | SetPNDeviceNum/ModuleID | A | ✅ | |
| `configRemoveModuleBody` | UninstallModule | SetPNDeviceNum/SetModuleSlot | A | ✅ | |
| `configAddSubmoduleBody` | AddSubmodule | SetPNDeviceNum/ModuleID/SubmoduleID | A | ✅ | |
| `configRemoveSubmoduleBody` | UninstallSubmodule | SetPNDeviceNum/SetModuleSlot/SetSubmoduleSlot | A | ✅ | |
| `configShieldBody` | ShieldDevice (v3 拼写纠正) | DeviceName | A | ✅ | **响应**v3 CallbackNTJson (Function=string, Result 在 root) + `DataType=14` |
| `configUnshieldBody` | UNShieldDevice (v3 拼写纠正) | DeviceName | A | ✅ | **响应**v3 CallbackNTJson (Function=string, Result 在 root) + `DataType=14` |
| `configSetIDeviceBody` | SetIDevice | Activate/InputLength (0-2048)/OutputLength (0-2048) | **C** (字段在 `IDevice` 顶层) | ❌ | 长度范围校验 |

新增 `internal/nrc/config_body.go`（避免 commands.go 过于膨胀），11 个函数 + 1 个通用辅助 `configBodyBuilder`。

**辅助函数**：
- `validateRequiredFields(spec, business)` — 按 spec 校验必填字段 + 范围
- `fetchDeviceList(target)` — 调 `device-list` 拉 PROFINET_NETWORKTOPOLOGY_FILE
- `jsonEncode(m)` — 统一 JSON 编码

### 2.4.1 写命令的 Response struct（错误模型 F3 + ShieldDevice 特殊 F5）

新增 `internal/configresp/types.go`：

```go
package configresp

import "encoding/json"

// WriteResponse 写命令的响应模型。
// 容忍 §2.0 F3 描述的两种错误模型 (数组 vs 单值), 用 json.RawMessage 存。
type WriteResponse struct {
    PNDriver        *topology.PNDriverConfig  `json:"PNDriver,omitempty"`
    DecentralDevice []topology.DecentralDevice `json:"DecentralDevice,omitempty"`
    IDevice         *topology.IDeviceInfo     `json:"IDevice,omitempty"`
    // 错误模型 F3: Error 可能是 string 或 string[]
    Error           json.RawMessage           `json:"Error,omitempty"`
    ErrorID         json.RawMessage           `json:"ErrorID,omitempty"`
    // 其他字段 (如 TotalInputLength / TotalOutputLength) 透传
    Extra           map[string]any            `json:"-"`
}

// HasErrors 检查响应是否包含错误 (容忍 string / []string / int / []int / nil)。
func (r *WriteResponse) HasErrors() bool {
    return len(r.Error) > 0 && string(r.Error) != "null" &&
           !(len(r.Error) == 2 && string(r.Error) == "[]")
}

// ShieldDeviceResponse ShieldDevice/UnshieldDevice 的特殊响应 (§2.0 F5)。
// Function.Value 是 bool (true=成功, false=失败), 覆盖了 dispatch 用法。
type ShieldDeviceResponse struct {
    Function struct {
        Value      bool   `json:"Value"`       // bool 而非 string!
        DeviceName string `json:"DeviceName"`
    } `json:"Function"`
}
```

> 10.B 实机测试时验证：
> - ShieldDevice 响应是否**完全覆盖**为 `Function.Value=bool`，还是既有 `Value=bool` 又有 `Error[]` 数组
> - 根据验证结果调整 ShieldDeviceResponse 模型

### 2.5 单元测试（基于 §2.0 5 个发现的设计）

`internal/nrc/config_body_test.go` + `internal/configresp/types_test.go`：

| # | 测试 | 对应发现 | 验收 |
|:--:|------|:--:|------|
| 1 | `TestConfigSetDriverBody_Valid` | F1 | 完整 JSON → 含 `SetPNDriver` + **顶层** `PNDriver` 子对象 |
| 2 | `TestConfigSetDriverBody_MissingRequiredField` | F1 | 缺 IPAddress → error |
| 3 | `TestConfigAddDeviceBody_Valid` | F1 | 完整 JSON → 含 `AddPNDevice` + `Function.{RefGSD,DAP_ID}` |
| 4 | `TestConfigRemoveDeviceBody_OneBasedIndex` | F4 | `SetPNDeviceNum=0` → error (`>= 1` 校验) |
| 5 | `TestConfigSetIDeviceBody_RangeCheck` | F1 | InputLength > 2048 → error |
| 6 | `TestConfigSetIDeviceBody_PlaceFieldAtIDevice` | F1 | 完整 JSON → `IDevice` 顶层对象 + `Function.Value="SetIDevice"` |
| 7-12 | (模式 A 6 条命令 × 4 测试 = 24) | F1 | valid/missing/invalid/empty |
| 13-15 | (模式 B SetPNDriver + Compile 边界) | F1 | |
| 16 | `TestConfigBodyBuilder_AutoFetch_NoFetch` | F2 | `--no-fetch=true` → 请求体不含 DecentralDevice |
| 17 | `TestConfigBodyBuilder_AutoFetch_FetchFailed` | F2 | fetch 失败 → 仍发请求体（不阻塞） |
| 18 | `TestConfigBodyBuilder_AutoFetch_FetchSuccess` | F2 | mock device-list 返回 topology → 请求体含 DecentralDevice |
| 19 | `TestConfigBodyBuilder_Routing_ModeA` | F1 | spec.Function="AddPNDevice" → 字段放 Function 下 |
| 20 | `TestConfigBodyBuilder_Routing_ModeB` | F1 | spec.Function="SetPNDriver" → 字段放顶层 PNDriver |
| 21 | `TestConfigShieldBody_Valid` | F5 | 输出 Function.DeviceName + Function.Value="ShieldDevice" |
| 22 | `TestConfigCompileBody_NoArgsNeeded` | - | `--data` 可选 |
| 23-30 | (8 个新组合: routing / fetch / 索引 / 范围 / 错误模型) | 各种 | |

**`internal/configresp/types_test.go`**（新增包）：

| # | 测试 | 对应发现 | 验收 |
|:--:|------|:--:|------|
| 31 | `TestWriteResponse_HasErrors_StringErr` | F3 | `Error: "some error"` → HasErrors=true |
| 32 | `TestWriteResponse_HasErrors_StringArrErr` | F3 | `Error: ["err1","err2"]` → HasErrors=true |
| 33 | `TestWriteResponse_HasErrors_NilErr` | F3 | `Error: null` → HasErrors=false |
| 34 | `TestWriteResponse_HasErrors_EmptyArr` | F3 | `Error: []` → HasErrors=false |
| 35 | `TestWriteResponse_ErrorID_MixedTypes` | F3 | ErrorID 容忍 int 和 []int |
| 36 | `TestWriteResponse_RoundTrip_DecentralDevice` | F3 | 完整 topology round-trip |
| 37 | `TestShieldDeviceResponse_ValueIsBool` | F5 | `Function.Value=true` 解析为 bool（不是 string） |
| 38 | `TestShieldDeviceResponse_FailureValue` | F5 | `Function.Value=false` → HasErrors=true |

合计 **~60+ 个新测试**（45 config body + 8 response struct + 7 routing/fetch）。

### 2.6 验收门槛（10.A）

- [ ] 11 条 config 命令的 Args 全部更新为 `{Name: "data"}` Required
- [ ] `internal/nrc/config_body.go` 11 BodyBuilder + 1 通用 `configBodyBuilder` 存在
- [ ] `internal/configresp/types.go` `WriteResponse` + `ShieldDeviceResponse` 存在
- [ ] `config_body_test.go` 30+ BodyBuilder 测试全 PASS（含 routing / fetch / 索引 / 范围）
- [ ] `configresp/types_test.go` 8+ Response 测试全 PASS（含 string vs []string 错误模型 + ShieldDevice bool）
- [ ] `inl config set-driver --help` 显示 `--data` 必填 + `--no-fetch` 可选
- [ ] `go test ./...` 10 包 → **11 包**（+configresp）
- [ ] `go vet ./...` 零警告
- [ ] `go build` 成功
- [ ] **无 nrc2.out 也可离线跑通**：所有测试用 mock，无工业 PC 也可全 PASS

---

## 三、子任务 10.B：实机系统测试

### 3.1 前置条件（用户必须在场）

| # | 前置条件 | 谁做 | 何时 |
|:--:|----------|:--:|:--:|
| 1 | 工业 PC 192.168.3.15 在线 | 用户 | 测试前 5 分钟 |
| 2 | `networktopology.json` 备份到 `~/backup/` | **用户（系统层）** | 测试前 |
| 3 | 工业 PC 系统层备份完整 (`/opt/profinet/` 全量) | **用户（系统层）** | 测试前 |
| 4 | 测试设备与产线**物理隔离**（不接 PLC） | **用户** | 测试全程 |
| 5 | C++ 源分发表源码可见（`nrc2.out` 路径或 git 仓库） | **用户** | dry-run 阶段 |
| 6 | 测试时间窗口：连续 **2-3 小时** 不被打扰 | **用户** | 测试全程 |
| 7 | 串口 / KVM 接入（万一 SSH 中断可恢复） | **用户** | 测试全程 |

> **关键**：任何一步没满足，**不要开始 10.B**。

### 3.2 测试分级（按风险递增）

| Level | 内容 | 风险 | 命令数 | 是否可回滚 |
|:--:|------|:--:|:--:|:--:|
| **L0** | `--dry-run` 抓 DryRunFrame + C++ 源码对照 | 0 | 12 | N/A |
| **L1** | 读命令状态 diff（无写） | 0 | (辅助) | N/A |
| **L2** | 写 + 立即反向回滚 | 中 | 10 (除 compile) | ✅ |
| **L3** | `config-compile` + 验证 | 高 | 1 | 需手动 |

### 3.3 L0：Dry-run 抓帧

```bash
# 1. 准备 inl 二进制
cd inl
go build -o inl.exe .

# 2. 创建测试目录
mkdir -p ~/step10-test/{dryrun,pre,post,responses,rollback}
cd ~/step10-test

# 3. 12 条 config 命令的 dry-run
for cmd in \
  "config set-driver" \
  "config add-device" \
  "config remove-device" \
  "config set-device" \
  "config add-module" \
  "config remove-module" \
  "config add-submodule" \
  "config remove-submodule" \
  "config shield" \
  "config unshield" \
  "config compile"; do
  echo "=== $cmd ==="
  ./inl.exe --target 192.168.3.15 $cmd --dry-run \
    2>&1 | tee dryrun/${cmd// /_}.txt
done
```

**核对清单**（L0 输出）：

- [ ] 12 条命令全部生成 DryRunFrame
- [ ] 帧的 sync_byte (0x4E66) / command (0x9275) / CRC32 与 PDF 规范一致
- [ ] 每个 Function.Value 与 C++ 源码分发表**字面一致**（注意拼写：`SetPNDriver` vs `SetPNdevice` / `UNShieldDevice` 大写拼写）
- [ ] 请求体 JSON 包含子对象（`PNDriver` / `IDevice` / `Device` / `Module` / `Submodule`），与 C++ 源码字段一致

**额外：验证 §2.0 假设 A/B（nrc2.out 是否预加载配置）**

```bash
# 测试 1: 带 --no-fetch, 不预加载 topology, 跑 remove-device --data '...'
# 期望 (假设 A): 仍然成功, 因为 nrc2.out 预加载
# 期望 (假设 B): 失败, 因为请求体无 DecentralDevice[]

./inl.exe --target 192.168.3.15 config remove-device --no-fetch \
  --data '{"SetPNDeviceNum":1}' --yes \
  --output responses/remove_no_fetch.json
# 检查响应中 Error[] 是否非空 → 假设 B 确认

# 测试 2: 不带 --no-fetch (默认), 跑同样的命令
./inl.exe --target 192.168.3.15 config remove-device \
  --data '{"SetPNDeviceNum":1}' --yes \
  --output responses/remove_auto_fetch.json
# 检查响应是否成功 → 假设 B 场景下应该成功 (因 fetch 了 topology)
```

**根据测试结果决定**：
- 如果**两种模式都成功**：BodyBuilder 简化（移除自动 fetch，节省一次 RTT）
- 如果**只有 auto-fetch 成功**：保留默认 fetch 行为
- 如果**只有 no-fetch 成功**：nrc2.out 预加载，可简化 BodyBuilder

> **L0 偏差处理**：在 field-verification.md 加新章节"Step 10 L0 dry-run"，登记发现的请求体格式偏差 + 假设 A/B 验证结果。

### 3.4 L1：读后状态对比

**目的**：建立"基线"，用于 L2 写后 diff。**同时抓取配置和生效两个状态**（两个不同文件）：

| 抓取命令 | 文件 | 用途 |
|---------|------|------|
| `device list` | `PROFINET_NETWORKTOPOLOGY_FILE` | L2 写后 diff 的基线（**写命令修改的就是这个文件**） |
| `device list-active` | `PROFINET_ACTIVATED_NETWORKTOPOLOGY_FILE` | L3 compile 后的生效状态验证基线 |

```bash
cd ~/step10-test

# 1. 抓当前网络配置全貌
./inl.exe --target 192.168.3.15 device list --output pre/device_list.json
./inl.exe --target 192.168.3.15 device list-active --output pre/device_list_active.json
./inl.exe --target 192.168.3.15 gsd list --output pre/gsd_list.json
./inl.exe --target 192.168.3.15 interface list --output pre/interface_list.json
./inl.exe --target 192.168.3.15 topology scan --interface enp4s0 --output pre/topology_scan.json

# 2. 保存为基线
cp pre/*.json baseline/
```

### 3.5 L2：写 + 立即回滚

> **10 条命令的通用流程**（以 `config set-driver` 为例）：

```bash
cd ~/step10-test

# 1. 准备测试 payload (改 IP 为测试值 192.168.99.1, 与生产 192.168.3.15 隔离)
TEST_DRIVER='{"DeviceName":"profinetdriver","IPAddress":"192.168.99.1","SubnetMask":"255.255.255.0","SetInTheProject":true}'

# 2. 执行写
./inl.exe --target 192.168.3.15 config set-driver --data "$TEST_DRIVER" --yes \
  --output responses/set_driver_post.json

# 3. 抓 post-state
./inl.exe --target 192.168.3.15 device list --output post/device_list_after_set_driver.json

# 4. 立即回滚 (用原值)
ORIG_DRIVER='{"DeviceName":"profinetdriver","IPAddress":"192.168.3.15","SubnetMask":"255.255.255.0","SetInTheProject":true}'
./inl.exe --target 192.168.3.15 config set-driver --data "$ORIG_DRIVER" --yes \
  --output responses/set_driver_rollback.json

# 5. 抓回滚后 state
./inl.exe --target 192.168.3.15 device list --output post/device_list_after_rollback.json

# 6. diff (回滚后 vs baseline)
diff baseline/device_list.json post/device_list_after_rollback.json
# 期望: 无差异
```

**10 条命令逐一执行**（顺序按风险递增）：

| 顺序 | 命令 | 风险 | 备注 |
|:--:|------|:--|------|
| 1 | `config shield` (测试值) | 低 | 不破坏数据, 仅改屏蔽标志 |
| 2 | `config unshield` (回滚) | 低 | |
| 3 | `config set-device` (测试 IP) | 中 | 改一个现有设备的 IP, 立即反向改回 |
| 4 | `config add-module` (空模块) | 中 | 加一个测试模块, 立即删 |
| 5 | `config remove-module` (反向回滚 #4) | 中 | |
| 6 | `config add-submodule` (空 submodule) | 中 | 同上模式 |
| 7 | `config remove-submodule` (反向回滚 #6) | 中 | |
| 8 | `config set-driver` (测试 IP) | 中 | 改 PNDriver IP, 立即反向 |
| 9 | `config add-device` (测试设备) | 高 | 加一个**不接 PLC 的虚拟设备**, 立即删 |
| 10 | `config remove-device` (反向回滚 #9) | 高 | |

**每条命令的验收**：

- [ ] 写响应 JSON 成功保存到 `responses/`
- [ ] post-state 与 pre-state 的 diff 符合预期（仅目标字段变化）
- [ ] 回滚后 state 与 baseline **完全一致**（`diff` 输出为空）
- [ ] 期间 `inl device run` / `topology scan` 等读命令无异常

**L2 偏差处理**：

- 写响应缺字段 / 多字段 → 补 `Response` struct（新建 `internal/configresp/`）
- 请求体 C++ 端拒收 → 改 BodyBuilder（回 10.A）
- 回滚后 state 仍有差异 → **立即停止后续测试**, 现场分析, 必要时手动从备份恢复 networktopology.json

### 3.6 L3：config-compile

```bash
cd ~/step10-test

# 1. 最终 backup
cp baseline/* pre/  # 保险

# 2. compile
./inl.exe --target 192.168.3.15 config compile --yes \
  --output responses/compile.json

# 3. 等待 5s (compile 会重启控制器)
sleep 5

# 4. 抓 post-state
./inl.exe --target 192.168.3.15 device list --output post/device_list_after_compile.json
./inl.exe --target 192.168.3.15 device list-active --output post/device_list_active_after_compile.json

# 5. diff (应与 baseline 一致 - 因为 L2 已回滚)
diff baseline/device_list.json post/device_list_after_compile.json
```

**L3 验收**：

- [ ] compile 响应成功
- [ ] 重启后能重连 (timeout 处理已在 Step 9 强化)
- [ ] post-state 与 baseline 一致

**L3 失败处理**（任一条件不满足）：

1. 现场 SSH 进工业 PC, 手动 `cp ~/backup/networktopology.json /opt/profinet/`
2. 重启 nrc2.out: `systemctl restart nrc2.out`
3. 在 field-verification.md 登记严重偏差
4. **本 Step 终止**, v0.1.0 tag 不打

### 3.7 偏差登记

`docs/protocol/field-verification.md` 新增章节 "Step 10 L2/L3 实机验证"：

```markdown
## config-set-driver (DataType=12, SetPNDriver)

### 核对记录

> ✅ **已核对** — 2026-06-XX, 工业 PC 192.168.3.15:6000

**v1 假设**（基于 Step 2-3 单元测试）:
| 字段 | 假设类型 | 来源 |
| ... | ... | ... |

**v2 实机**（响应文件: `set_driver_post.json`）:
| 字段 | 实机类型 | 偏差 |
| ... | ... | ... |
```

每条命令一节，含 pre/post JSON、diff、结论。

### 3.8 验收门槛（10.B）

- [ ] 12 条 config 命令在 field-verification.md 全部标 `✅ 已核对` 或 `⚠️ 已知偏差，已建模`
- [ ] 10 条 config 写命令（除 compile）有 testdata fixture（`config_set_driver_response_*.json` 等）
- [ ] compile 响应有 fixture
- [ ] 新增 `internal/configresp/` 子包（如果响应缺字段需建模）
- [ ] `internal/nrc/config_body_test.go` 45+ 测试全 PASS（已含 10.A 验收）
- [ ] 期间产线无任何告警 / 中断
- [ ] 回滚后 baseline 与 post-回滚 state 字节级一致（`diff` 为空）

---

## 四、文件清单（Step 10 全量）

```
inl/
├── internal/
│   ├── nrc/
│   │   ├── commands.go              ← 改: 11 条 config 命令加 Args
│   │   ├── config_body.go           ← 新增: 11 个 BodyBuilder
│   │   ├── config_body_test.go      ← 新增: 45+ 测试
│   │   ├── commands_test.go         ← 改: Registry 测试更新
│   │   └── (其他不动)
│   │
│   ├── configresp/                  ← 新增 (仅当 10.B 发现响应缺字段)
│   │   ├── types.go
│   │   └── types_test.go
│   │
│   └── (其他不动)
│
├── docs/
│   ├── inl-step10-config-write-validation-plan.md  ← 本文档
│   ├── protocol/field-verification.md  ← 改: +12 节 (L0 + L2 + L3)
│   ├── inl-development-status.md    ← 改: 偏差表清零, 95%+ → 98%+
│   └── (其他不动)
│
├── AGENTS.md                        ← 改: 标注 12 条 config 全部已实机验证
│
└── testdata/
    ├── config_set_driver_post_*.json        ← 新增 (实机响应)
    ├── config_add_device_post_*.json
    ├── (其他 9 条 + compile)
    └── (原有不动)
```

**新增文件**: 2 (config_body.go + config_body_test.go) + 1 (configresp/) + 11 testdata fixtures
**修改文件**: 4 (commands.go / commands_test.go / field-verification.md / AGENTS.md)

---

## Step 1（10.A）：参数化骨架

1. `internal/nrc/config_body.go` 写 11 个 BodyBuilder（骨架，校验函数可暂留 TODO）
2. `internal/nrc/commands.go` 11 条 config 命令加 `Args: []ArgumentSpec{{Name: "data", Required: true}}`
3. `internal/nrc/config_body_test.go` 11 条命令 × 4 测试 = 44 测试

## Step 2（10.A）：编译通过 + 单元测试全绿

```bash
go vet ./...
go test -count=1 ./...
go build -o inl.exe .
```

## Step 3（10.A）：人工 dry-run 验证

```bash
inl --target 192.168.3.15 config set-driver --help
# 期望: --data (必填)

inl --target 192.168.3.15 config set-driver --data '{}' --dry-run
# 期望: DryRunFrame 输出 + "字段缺失" 错误
```

## Step 4（10.B L0）：抓 12 条命令 DryRunFrame

按 §3.3 流程。

## Step 5（10.B L0）：与 C++ 源码对照

- 用户提供 nrc2.out 源码路径
- 逐条对比 Function.Value 字符串拼写
- 在 field-verification.md 登记任何偏差

## Step 6（10.B L1）：抓 baseline

按 §3.4 流程。

## Step 7（10.B L2）：10 条写 + 回滚

按 §3.5 顺序执行。每条命令执行后立即 diff baseline vs post-回滚。

## Step 8（10.B L3）：config-compile

按 §3.6 流程。

## Step 9：偏差修正 + 测试 fixture

- 修 BodyBuilder / Response struct（如有）
- 11+1 个 testdata fixtures 加入 `testdata/`
- 在 `config_body_test.go` 增 round-trip 测试（用实机响应）

## Step 10：文档归档

- `field-verification.md` 12 节完整
- `inl-development-status.md` 偏差表清零
- `AGENTS.md` 标注 12 条 config 已实机验证

## Step 11：v0.1.0 tag (用户显式)

按 [project_rules](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/.trae/rules/project_rules.md)：

```bash
# 由用户执行, AI 不自动:
git tag v0.1.0
git push origin v0.1.0
# 触发 .github/workflows/release.yml + npm-publish.yml
```

---

## 完整验收清单

### 10.A 离线验收

- [ ] `internal/nrc/config_body.go` 11 BodyBuilder + 1 通用辅助 存在
- [ ] `internal/nrc/config_body_test.go` 30+ 测试全 PASS（含 routing / fetch / 索引 / 范围）
- [ ] `internal/configresp/types.go` `WriteResponse` + `ShieldDeviceResponse` 存在
- [ ] `internal/configresp/types_test.go` 8+ 测试全 PASS（含 string vs []string 错误模型 + ShieldDevice bool）
- [ ] 11 条 config 命令 `Args` 全部含 `--data` Required + `--no-fetch` 可选
- [ ] `inl config set-driver --help` / `add-device` / `set-device` / `add-module` / `remove-module` / `add-submodule` / `remove-submodule` / `shield` / `unshield` / `set-idevice` 均显示 `--data` 必填
- [ ] `inl config compile --help` 不要求 `--data`
- [ ] `go vet ./...` 零警告
- [ ] `go test ./...` 11 包全 PASS（10 → 11，+configresp）
- [ ] `go build` 成功
- [ ] **离线可跑通**：所有测试用 mock，无工业 PC 也可全 PASS

### 10.B 实机验收

- [ ] 12 条 config 命令在 `field-verification.md` 全部 `✅ 已核对` 或 `⚠️ 已建模偏差`
- [ ] 11 条 config 写命令的 testdata fixture 存在
- [ ] 1 条 config-compile 的 testdata fixture 存在
- [ ] **§2.0 假设 A/B 验证完毕**：L0 跑两种模式（`--no-fetch` 和自动 fetch），决定最终 BodyBuilder 行为
- [ ] **L2 写后 diff 用 `device list`**（不是 `device-list-active`！），diff `baseline/device_list.json` vs `post/device_list.json` 为空
- [ ] **L3 compile 后 fetch 两个状态**：`device list`（配置应不变）+ `device list-active`（生效应反映新配置）
- [ ] L2 10 条命令的"回滚后 baseline vs post" diff 全部为空
- [ ] L3 compile 后 device-list 与 baseline diff 为空
- [ ] 期间产线无任何告警 / 中断
- [ ] C++ 源码对照表填入 L0 章节

### 10.B 偏差修正（如有）

- [ ] BodyBuilder / Response struct 修正完毕
- [ ] 修正后 `go test ./...` 仍全 PASS
- [ ] field-verification.md 偏差章节已更新

### 文档归档

- [ ] `docs/inl-step10-config-write-validation-plan.md` (本文档) 生成
- [ ] `docs/protocol/field-verification.md` 12 节新增（每条 config 一节）
- [ ] `docs/inl-development-status.md` 偏差表清零, 95% → **98%+**
- [ ] `AGENTS.md` Source Layout 标注 12 条 config 已实机验证

---

## 执行节奏

| Step | 内容 | 预计耗时 |
|------|------|:---:|
| 1 | 10.A 参数化 (45 测试 + 11 BodyBuilder) | 2-3 小时 |
| 2 | 10.A 编译 + 离线测试 | 30 分钟 |
| 3 | 10.B L0 dry-run 抓帧 | 30 分钟 |
| 4 | 10.B L0 与 C++ 源码对照 (需用户提供源码) | 1 小时 |
| 5 | 10.B L1 抓 baseline | 15 分钟 |
| 6 | 10.B L2 10 条写 + 回滚 (含 diff 验证) | 2-3 小时 |
| 7 | 10.B L3 compile | 30 分钟 |
| 8 | 偏差修正 + testdata fixture | 1-2 小时 |
| 9 | 文档归档 | 30 分钟 |

**总耗时**: 8-11 小时，分 2 个工作日。
**关键依赖**: 用户全程在场（10.B 不能无人值守）。

---

## 风险与降级

| 风险 | 概率 | 影响 | 降级策略 |
|---|:--:|---|---|
| C++ 源码不可访问 (nrc2.out 在工业 PC 闭源) | 中 | 无法对照 Function.Value 拼写 | 靠 L2 实机响应反推 + 工业 PC 现场咨询 |
| 10.B 写后无法回滚 | 低 | 产线配置破坏 | 现场手动 SCP 恢复 backup + 重启 nrc2.out |
| compile 失败 | 中 | 控制器重启失败 | 现场手动恢复 + 终止本 Step |
| 期间 SSH 中断 | 低 | 测试中断 | 串口 / KVM 兜底（前置条件 #7） |
| 工业 PC 192.168.3.15 被产线占用 | 中 | 无法测试 | 推迟 Step 10 至产线停机窗口 |
| v0.1.0 tag 在测试完成前被打 | 低 | 名不副实 | 严格按 project_rules 等待用户显式打 tag |
| **AI 误调某条 config 命令** | 中 | 改错设备 / 改错 IP | AI 本身不执行 10.B（用户手动跑 inl），AI 仅参与文档编写 |

### 不可降级项（一旦发生必须终止 Step 10）

- 10.B 写后回滚失败且手动恢复也失败
- compile 后控制器无法重启
- 工业 PC 出现未知的硬件 / 软件错误

---

## 不属于 Step 10

- **Step 11**: `network +shortcuts` 语义层（diagnose / auto-fix / batch-setup / generate）
- **Step 12**: IO Routing (DataType=17) — topology active / verify / compile
- **Step 13+**: 多车间 Profile / GSDML 离线字典 / `inl backup` / `inl restore` / 安全策略层

---

## 相关文档

| 主题 | 文档 |
|---|---|
| **字段参照（权威）** | [inl-config-field-reference.md](inl-config-field-reference.md) — 11 条命令的精确字段路径 + 5 个关键设计发现 + 7 个 C++ 行号索引 |
| Step 9 计划（已发布） | [inl-step9-distribution-plan.md](inl-step9-distribution-plan.md) |
| Step 9 状态 (status) | [inl-development-status.md](inl-development-status.md) |
| 实机偏差登记 | [protocol/field-verification.md](protocol/field-verification.md) |
| P0 修复 (含 set-idevice 骨架先例) | [inl-p0-fix-plan.md](inl-p0-fix-plan.md) |
| 写工作流 (skill) | [../skills/inl-workflow-profinet-write/SKILL.md](../skills/inl-workflow-profinet-write/SKILL.md) |
| **C++ 源（字段反推权威）** | [PNConfigLibFileDesign.cpp](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/io-controller/src/pnconfiglib/PNConfigLibFileDesign.cpp) — 11 个分派函数实现 |
| 项目规则 (不自动 push/release) | [project_rules.md](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/.trae/rules/project_rules.md) |
