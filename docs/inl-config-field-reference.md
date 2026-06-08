---
title: inl Config 命令字段参照（基于 C++ 源码反推）
tags: [inl, config, field-reference, cpp-source, step10]
created: 2026-06-04
updated: 2026-06-04
status: reference
---

# inl Config 命令字段参照（基于 C++ 源码反推）

> **权威来源**：[PNConfigLibFileDesign.cpp](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/io-controller/src/pnconfiglib/PNConfigLibFileDesign.cpp)（**v2 新版 io-controller 分支**，5741 行，2026-06-04）
> **行号标注**：所有行号引用当前 io-controller 仓库的该文件
> **用途**：Step 10 参数化 + 实机测试的"唯一权威字段表"
>
> **2026-06-04 v1 修正**：写命令的"自动 fetch 当前 topology"应使用 `device-list`（读 **PROFINET_NETWORKTOPOLOGY_FILE**，即**配置**），不是 `device-list-active`（读 **PROFINET_ACTIVATED_NETWORKTOPOLOGY_FILE**，即**编译后生效**）。两者读的是不同文件，写命令修改的是前者。
>
> **2026-06-04 v2 修正**（基于用户更新后的 io-controller 分支重核）：
> 1. **`ShildDevice` / `UNShildDevice` typo 拼写**：常量 `PNCONFIGLIB_FUNCTION_SHIELD_DEVICE = "ShildDevice"` 与 `PNCONFIGLIB_FUNCTION_UNSHIELD_DEVICE = "UNShildDevice"`（缺 `'e'`），与旧版文档化的 `"ShieldDevice"` / `"UNShieldDevice"` **不一致**。inl 必须用新版拼写（与 C++ 分发表一致），否则 C++ 端会进入默认错误分支。来源：[profinet_constants.h:82-83](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/io-controller/src/ioc/profinet_constants.h)
> 2. **写命令响应包含 `Function` 字段**（不是 `.clear()` 后再发）：新版 C++ 源 (SetPNDriver L511, SetPNDevice L1842, SetIDevice L3302 等) 顺序是 `writer.write(networktopology) → NRC_SendSocketCustomProtocal(...) → networktopology["Function"].clear() → saveJsonToFile(...)`，即 `.clear()` 在**发送后**。响应 body 里会看到 `Function` 子对象（含原始 `Value` + 业务字段）。这一变化影响所有 8 条"主路径"写命令的响应解析（不影响 ShildDevice/UNShildDevice，它们自己 new root）。
> 3. **请求体必须含完整 `DecentralDevice` / `PNDriver` / `IDevice`**（不只是响应回传）：新版代码 SetPNDriver L412 显式 `if (networktopology["DecentralDevice"][i]["DeviceName"]...`、AddPNDevice L864 显式 `networktopology["PNDriver"]["SubnetMask"]`、UninstallPNDevice L2718 显式遍历 `networktopology["DecentralDevice"]`，C++ **不会**自己从 `PROFINET_NETWORKTOPOLOGY_FILE` 重新加载。这与旧版 field-reference 假设一致（auto-fetch 设计仍正确）。
>    - **2026-06-04 v2 假设 B 实机确认**：inl **必须**把当前 `DecentralDevice[]` 嵌入请求体才能让 nrc2.out 正确处理 `UninstallPNDevice` 等写命令。`fetchTopologyForConfig` 真实实现于 2026-06-04 完成（参见 [internal/nrc/config_body.go](../internal/nrc/config_body.go) `fetchTopologyForConfig` 注释 + 单测 `internal/nrc/fetch_topology_test.go`）。详见 §0.5 与 [inl-step10-config-write-validation-plan.md §2.0](./inl-step10-config-write-validation-plan.md) 假设 A/B 段落。
> 4. **`ShildDevice` / `UNShildDevice` 响应 DataType = 14**（不是 12）：处理器内自建 root，根级 `DataType` 硬编码 14。inl 不按 DataType 路由（按 command word 0x9271），但解析器应知道响应顶层 DataType=14。
> 5. **`GetActRun` 响应 DataType = 14**（不变）：`SendACTRun` 同样 root["DataType"] = 14。已与 inl `device run` 命令一致。
>
> **2026-06-04 v3 重大重构**（基于提交 3d3cc3c7 + b77ba388 — 统一 P 网配置 JSON 响应结构为 CallbackNTJson 模板）：
> 1. **常量拼写纠正**（提交 3d3cc3c7）：`PNCONFIGLIB_FUNCTION_SHIELD_DEVICE = "ShieldDevice"`、`PNCONFIGLIB_FUNCTION_UNSHIELD_DEVICE = "UNShieldDevice"`。v2 误用 typo 拼写, v3 改回正确拼写。inl 需同步回滚到正确拼写。
> 2. **CallbackNTJson 模板**（提交 b77ba388）：参考 `CallbackNTJson` (L280-291) 的结构 —— `root["Function"] = "CallBackJson"` (string 标签), `root["IDevice"]` / `root["PNDriver"]` / `root["DecentralDevice"]` 全部平铺到顶层。**Function 永远是 string 标签, 业务字段在 root 顶层**。
> 3. **6 个函数已重构到新模板**:
>    - `CallbackACTNTJson` (device list-active): `Function: "CallBackActivatedJson"` (string), 文件内容字段平铺到 root
>    - `SendACTRun` (device run): `Function: "GetActRun"`, `Devices[]` + `TotalCount` 在 root
>    - `GetGSDFileNetwork` / `GetGSDFileActivated` (device gsd-config/active): `Function: "..."` (string), `GSDFile` + `error` 在 root
>    - `ShieldDeviceByName` / `UNShieldDeviceByName` (config shield/unshield): `Function: "ShieldDevice"` (string), `DeviceName` + `Result` (bool) 在 root
> 4. **请求体 vs 响应体不对称** (重要):
>    - **请求体**: 仍用旧模板 (`Function` 是 object, 内含 `Value: "..."` 字符串 + 业务字段), 因为 C++ `NetWorkTopologyFunction` 分发器 (L158, 165 等) 仍读 `networktopology["Function"]["Value"]` 确定分派函数。
>    - **响应体**: ShieldDevice/UNShieldDevice 已用新模板 (Function=string, 业务字段在 root)。inl 解析响应时按新模板反序列化。
> 5. **其他写命令 (SetPNDriver/AddPNDevice/UninstallPNDevice/AddModule/AddSubmodule/UninstallModule/UninstallSubmodule/SetPNDevice/SetIDevice) 尚未重构**: 仍用旧模板 (请求 `Function` 是 object + 业务字段, 响应 `Function` 在 body 里但结构未变)。预计在后续提交中按 CallbackNTJson 模板重构。inl 暂时按"模式 A (Function 下)"处理这些命令的请求和响应。

---

## 0. 关键架构发现（影响所有 config 命令的设计）

### 0.1 字段路径有**三种**模式（v2 修正：补充 Mode C）

C++ 函数读取请求字段时有**三种截然不同的位置**：

| 模式 | 路径示例 | 适用命令 | C++ 源证据 |
|------|----------|----------|------------|
| **A: Function 下** | `networktopology["Function"]["DeviceName"]` | ShildDevice / UNShildDevice / AddPNDevice / AddModule / AddSubmodule / UninstallPNDevice / UninstallModule / UninstallSubmodule | L316/333/821/832/1570/2013/2360/2712/2776/2960 |
| **B: Function 外（顶层 `PNDriver`）** | `networktopology["PNDriver"]["DeviceName"]` | SetPNDriver | L393-395, 411 |
| **C: 顶层 `IDevice`** | `networktopology["IDevice"]["Activate"]` | **SetIDevice** | L3202, 3205, 3210-3211 |

> ⚠️ 字段路径必须严格按 C++ 源码访问，**不能**统一规整到 Function 下，否则 C++ 读不到。
> ⚠️ v2 重要发现：SetIDevice **不是**模式 A——它直接读 `networktopology["IDevice"]`，与 SetPNDriver 类似但子对象名不同。BodyBuilder 需为它单设分支。
> ⚠️ SetPNDriver 也读 `networktopology["PNDriver"]["SetInTheProject"]`（L411）以决定是否校验 IP 冲突；inl 必传该字段，否则默认 `true`。

### 0.2 写命令的"上下文依赖"问题

大多数 C++ 函数**不**从 `PROFINET_NETWORKTOPOLOGY_FILE` 重新加载当前状态，而是直接使用传入的 `networktopology` 中已有的 `DecentralDevice` / `PNDriver` / `IDevice` 数组。证据：

```cpp
// UninstallPNDevice L2706-2766
int UninstallPNDevice(Json::Value &networktopology) {
    ...
    if (networktopology["Function"]["SetPNDeviceNum"].asInt() <= networktopology["DecentralDevice"].size()) {
        int devicenum = networktopology["Function"]["SetPNDeviceNum"].asInt() - 1;
        ...
        for (int i = 0; i < networktopology["DecentralDevice"].size(); i++) {
            if (i != devicenum) {
                copynetworktopology["DecentralDevice"].append(networktopology["DecentralDevice"][i]);
            }
        }
        networktopology = copynetworktopology;  // 直接修改传入的对象
    }
}
```

**含义**：
- AI 调 `inl config remove-device --data '{"SetPNDeviceNum":1}'` **不够**——必须先 fetch 当前 `DecentralDevice` 数组
- inl 应该自动：执行写命令前**先 `device-list-active` 拉一次完整 topology**，把 `DecentralDevice` / `PNDriver` / `IDevice` 灌到请求体里
- 用户只需在 `--data` 里给**业务字段**（如 `Function.SetPNDeviceNum` / `Function.RefGSD`）

### 0.3 错误模型不统一

C++ 函数对错误的返回有两种模式：

| 模式 | 字段 | 适用命令 |
|------|------|----------|
| **Error: string[] + ErrorID: int[]** | `Error.append(...)` / `ErrorID.append(...)` | SetPNDriver / AddPNDevice / UninstallPNDevice / AddModule / AddSubmodule / UninstallModule / UninstallSubmodule |
| **Error: string + ErrorID: int** | `Error = errormessage;` / `ErrorID = 20;` | SetIDevice |

> 成功路径一致：先 `Error.clear()` + `ErrorID.clear()`，再回传 `networktopology`。
> 函数返回：`int`（0 = 成功，-1 = 失败），但 C++ 实际**总是把整个 `networktopology` 通过 NRC 帧回传**，不管成败。

### 0.4 索引是 1-based

`UninstallPNDevice` / `SetPNDevice` / `AddModule` / `AddSubmodule` 等用 `SetPNDeviceNum` / `SetModuleSlot` / `SetSubmoduleSlot`，**全部 1-based**（`int devicenum = ...asInt() - 1;`）。

**对 AI 工作的影响**：
- AI 看到 `DecentralDevice[0]` 时传 `SetPNDeviceNum=1`
- 多次删除时**索引会变**（删了第 1 个后，原来的第 2 个变成第 1 个），AI 必须**每次都重读 `device-list` 取最新配置**

### 0.5 响应是**整个 networktopology 对象**（v2 重大修正）

写命令的 C++ 响应是 `writer.write(networktopology)`（SetPNDriver L511, SetPNDevice L1842, SetIDevice L3302, UninstallPNDevice L2758 等），**完整的拓扑数据**。

**v2 修正**：v2 之前文档声称"Function 字段在发送前会被 `.clear()`"，**这是错的**。新版 C++ 源的实际顺序是：

```cpp
// 新版 (v2) SetPNDriver L509-516
networktopology["Error"].clear();
networktopology["ErrorID"].clear();
std::string tempStr = writer.write(networktopology);  // ① 序列化
NRC_SendSocketCustomProtocal(0x9271, tempStr);         // ② 发送 (Function 还在)
networktopology["Function"].clear();                  // ③ .clear() (发送后)
CommonFunction::Jsonfunction::BYD_saveJsonToFile(jsonbackup, PROFINET_NETWORKTOPOLOGY_FILE);  // ④ 存盘
```

**结论**：
- 响应 body 里**有** `Function` 子对象（含 `Value` + 业务字段）
- `.clear()` 是为了**写回磁盘**时清干净
- inl 解析响应时，如果业务关心 Function 字段，应能容忍
- WriteResponse 模型（`internal/configresp/types.go`）不显式建模 Function，依赖 Go json 反序列化的"未知字段宽容"——这一宽容默认即兼容

例外：`ShildDevice` / `UNShildDevice` 不走透传，在处理器内自建 root，Function.Value 是 bool。

---

### 0.6 写命令请求体的"上下文"必须由 inl 注入（v2 实机确认, 2026-06-04）

> 来源：[inl-step10-config-write-validation-plan.md §2.0 假设 A/B v2 实机确认](./inl-step10-config-write-validation-plan.md)

C++ 端 nrc2.out **不**预加载 `PROFINET_NETWORKTOPOLOGY_FILE` 再合并请求体——它**直接使用**请求体里 `DecentralDevice` / `PNDriver` / `IDevice` 三个顶层数组/对象（SetPNDriver L412、AddPNDevice L864、UninstallPNDevice L2718 等显式 `networktopology["DecentralDevice"][i]` 访问，**不**重读磁盘）。

**含义**：
- inl **必须**在发写命令前先 `device-list` 拉一次配置（不是 `device-list-active`——后者是激活后的运行时视角，文件不同）
- 把返回的 `DecentralDevice` / `PNDriver` / `IDevice` 三个字段**直接嵌入**请求体的对应顶层位置
- `inl/internal/nrc/config_body.go` 的 `configBodyBuilder` 已通过 `fetchTopologyForConfig` 完成此自动注入（默认开启，`--no-fetch=true` 可禁用，fetch 失败不阻塞）
- 真实 `fetchTopologyForConfig` 实现于 2026-06-04 落地：用 `nrc.NewClient(addr).Connect() → SendReceiveFiltered(0x9275, "{\"DataType\":12,\"Function\":{\"Value\":\"CallBackJson\"}}", 12, 0x9271)` 拉响应，解析到 `topology.CallbackJsonResponse`，提取三字段。详见 `internal/nrc/config_body.go` 注释 + 单测 `internal/nrc/fetch_topology_test.go`。
- 对应业务影响（Step 10.A F2 + v2 实机确认）：AI 调 `inl config remove-device --data '{"SetPNDeviceNum":1}'` **不需要**自己先拉 topology——inl 会自动注入；只有 AI 已持有更新拓扑时可用 `--no-fetch=true` 跳过。

例外：`SetIDevice` 走模式 C（业务字段在顶层 `IDevice`），不需要 `DecentralDevice`，因此 `configBodyBuilder` 跳过 fetch（避免无谓 RTT）。

---

## 1. 11 条 config 命令的字段表

### 1.1 config-set-driver

| 项目 | 值 |
|------|------|
| Function.Value | `SetPNDriver` |
| C++ 实现 | `SetPNDevice(Json::Value&)` L387-524 |
| **输入字段路径（模式 B：顶层）** | |
| `PNDriver.DeviceName` (string) | 必填 |
| `PNDriver.IPAddress` (string) | 必填 |
| `PNDriver.SubnetMask` (string) | 必填 |
| `PNDriver.SetInTheProject` (bool) | 必填 |
| **C++ 自动行为** | 当 `SetInTheProject=true` 时，遍历 `DecentralDevice[]` 检查与主站 IP 冲突 |
| **错误模式** | `Error: string[]` + `ErrorID: int[]` |
| **响应** | 整个 `networktopology`（Function 已 clear） |
| 成功标志 | `Error[]` + `ErrorID[]` 都为空 |

### 1.2 config-add-device

| 项目 | 值 |
|------|------|
| Function.Value | `AddPNDevice` |
| C++ 实现 | `AddPNDevice(Json::Value&)` L815+ |
| **输入字段路径（模式 A：Function 下）** | |
| `Function.RefGSD` (string) | **必填**，GSD 文件名（如 `gsdml-v2.31-hms-abcc40-pir-20171101.xml`） |
| `Function.DAP_ID` (string) | **必填**，DAP ID |
| **C++ 自动生成** | 设备名 `DAP.DNS_CompatibleName_<N>` / 空闲 IP（`FindFreeIPAddress`）/ SubnetMask（继承自主站）/ ReductionRatio / DAP 模块布局 |
| **错误模式** | `Error: string[]` + `ErrorID: int[]` |
| **响应** | 整个 `networktopology`，新增的设备在 `DecentralDevice` 末尾 |

### 1.3 config-remove-device

| 项目 | 值 |
|------|------|
| Function.Value | `UninstallPNDevice` |
| C++ 实现 | `UninstallPNDevice(Json::Value&)` L2706-2766 |
| **输入字段路径（模式 A：Function 下）** | |
| `Function.SetPNDeviceNum` (int) | **必填**，1-based 设备索引 |
| **C++ 行为** | 直接在传入的 `DecentralDevice` 数组中过滤掉 `SetPNDeviceNum-1` 索引的元素 |
| **错误模式** | `Error: string[]` + `ErrorID: int[]`（索引越界时 `PNCONFIGLIB_CODE_ERROR`） |
| **响应** | 整个 `networktopology`，目标设备已从 `DecentralDevice` 移除 |

### 1.4 config-set-device

| 项目 | 值 |
|------|------|
| Function.Value | `SetPNDevice` |
| C++ 实现 | `SetPNDevice(Json::Value&)` L1561+ |
| **输入字段路径（模式 A：Function 下）** | |
| `Function.SetPNDeviceNum` (int) | **必填**，1-based 设备索引 |
| **C++ 行为** | 校验目标设备的 IP/名称冲突 / ReductionRatio / SetInTheProject，**不直接修改业务参数**（该函数主要做 `onlycheck` 模式的编译前校验） |
| **错误模式** | `Error: string[]` + `ErrorID: int[]` |
| **注意** | 此函数是 `SetPNDevice`，但 `inl config set-device` 命令名容易误解——它主要做**校验**而非"修改设备参数"。修改设备 IP/Name 需要用其他途径（DCP 或直接改 networktopology.json） |
| **响应** | 整个 `networktopology` |

### 1.5 config-add-module

| 项目 | 值 |
|------|------|
| Function.Value | `AddModule` |
| C++ 实现 | `AddModule(Json::Value&)` L1999+ |
| **输入字段路径（模式 A：Function 下）** | |
| `Function.SetPNDeviceNum` (int) | **必填**，1-based 设备索引 |
| `Function.ModuleID` (string) | **必填**，模块 ID（从 GSD 数据库的 `UseableModules[].ModuleIDTarget` 取） |
| **C++ 自动** | 从 GSD 数据库查模块的 `FixedInSlots` / `UsedInSlots` / `AllowedInSlots`，自动选 slot |
| **错误模式** | `Error: string[]` + `ErrorID: int[]` |
| **响应** | 整个 `networktopology`，目标设备的 `Module[]` 新增一项 |

### 1.6 config-remove-module

| 项目 | 值 |
|------|------|
| Function.Value | `UninstallModule` |
| C++ 实现 | `UninstallModule(Json::Value&)` L2769-2950 |
| **输入字段路径（模式 A：Function 下）** | |
| `Function.SetPNDeviceNum` (int) | **必填**，1-based 设备索引 |
| `Function.SetModuleSlot` (int) | **必填**，1-based 模块 slot 号 |
| **C++ 行为** | 从 `DecentralDevice[devicenum].Module[]` 中过滤掉 `Slot=SetModuleSlot` 的模块；同时校验 `FixedInSlots` 不可删 |
| **错误模式** | `Error: string[]` + `ErrorID: int[]`（预装模块 `PNCONFIGLIB_SUBMODULE_FIXED_CANT_DEL`） |
| **响应** | 整个 `networktopology` |

### 1.7 config-add-submodule

| 项目 | 值 |
|------|------|
| Function.Value | `AddSubmodule` |
| C++ 实现 | `AddSubmodule(Json::Value&)` L2351+ |
| **输入字段路径（模式 A：Function 下）** | |
| `Function.SetPNDeviceNum` (int) | **必填**，1-based 设备索引 |
| `Function.ModuleID` (string) | **必填**，模块 ID |
| `Function.SubmoduleID` (string) | **必填**，子模块 ID |
| **C++ 自动** | 从 GSD 数据库查子模块的 Subslot / IOData |
| **错误模式** | `Error: string[]` + `ErrorID: int[]` |
| **响应** | 整个 `networktopology` |

### 1.8 config-remove-submodule

| 项目 | 值 |
|------|------|
| Function.Value | `UninstallSubmodule` |
| C++ 实现 | `UninstallSubmodule(Json::Value&)` L2952+ |
| **输入字段路径（模式 A：Function 下）** | |
| `Function.SetPNDeviceNum` (int) | **必填**，1-based 设备索引 |
| `Function.SetModuleSlot` (int) | **必填**，1-based 模块 slot 号 |
| `Function.SetSubmoduleSlot` (int) | **必填**，1-based 子模块 slot 号 |
| **错误模式** | `Error: string[]` + `ErrorID: int[]` |
| **响应** | 整个 `networktopology` |

### 1.9 config-shield (v3 CallbackNTJson 模板)

| 项目 | 值 |
|------|------|
| **Function.Value (v3 修正: 拼写纠正)** | `ShieldDevice`（v3 正确拼写, 提交 3d3cc3c7 修复） |
| C++ 实现 | `ShieldDeviceByName(Json::Value&)` L312-327 |
| **请求体（仍用旧模板, 因为 C++ 分发器读 `Function.Value`）** | |
| `Function.Value` (string) | "ShieldDevice" (C++ 分发器 L158/165 读此字段分派) |
| `Function.DeviceName` (string) | **必填**, 设备名 (C++ L316 读此字段) |
| **响应体（v3 已用新模板）** | |
| 响应 `DataType` | **14**（处理器自建 root, 硬编码 14） |
| 响应 `Function` | **string 标签**: `"ShieldDevice"` (不再是 object) |
| 响应 `DeviceName` (顶层) | echo 的设备名 |
| 响应 `Result` (顶层, bool) | **true=屏蔽成功, false=失败** — 原 `Function.Value` (bool) 改为顶层 `Result` |
| **错误模式** | ❌ 不使用 `Error[]` / `ErrorID[]`, 仅用顶层 `Result` bool |
| **模板来源** | [CallbackNTJson L280-291](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/io-controller/src/pnconfiglib/PNConfigLibFileDesign.cpp) (Function=string 标签, 业务字段在 root) |

### 1.10 config-unshield (v3 CallbackNTJson 模板)

| 项目 | 值 |
|------|------|
| **Function.Value (v3 修正: 拼写纠正)** | `UNShieldDevice`（v3 正确拼写, 提交 3d3cc3c7 修复） |
| C++ 实现 | `UNShieldDeviceByName(Json::Value&)` L329-344 |
| **请求体（仍用旧模板）** | |
| `Function.Value` (string) | "UNShieldDevice" (C++ 分发器读) |
| `Function.DeviceName` (string) | **必填**, 设备名 |
| **响应体（v3 已用新模板）** | |
| 响应 `DataType` | **14** |
| 响应 `Function` | **string 标签**: `"UNShieldDevice"` |
| 响应 `DeviceName` (顶层) | echo 的设备名 |
| 响应 `Result` (顶层, bool) | **true=取消屏蔽成功, false=失败** |
| **响应模式** | 与 ShieldDevice 相同（v3 CallbackNTJson 模板） |

### 1.11 config-set-idevice (Step 9.1 已参数化, v2 修正模式)

| 项目 | 值 |
|------|------|
| Function.Value | `SetIDevice` |
| C++ 实现 | `SetIDevice(Json::Value&)` L3198-3315 |
| **输入字段路径（v2 修正：模式 C，顶层 `IDevice`）** | |
| `IDevice.Activate` (bool) | 必填 |
| `IDevice.InputLength` (int) | 必填，**范围 [0, 2048]** |
| `IDevice.OutputLength` (int) | 必填，**范围 [0, 2048]** |
| **C++ 校验** | 字段类型必须是 int；长度超界返回 error（仅设 `networktopology["Error"] = errormessage` 单值） |
| **错误模式** | `Error: string` + `ErrorID: int`（**单值，不是数组**）——其他 9 条都是数组 |
| **响应** | 整个 `networktopology`（含 Function 子对象，v2 修正） |
| **模式归属** | 模式 C（顶层 `IDevice`），与 SetPNDriver 模式 B 形似但子对象名不同 |

---

## 2. 通用响应模型

无论哪条 config 写命令，响应都遵循这个结构：

```json
{
  "DataType": 12,
  "PNDriver": { ... },                  // 顶层 PNDriver 对象
  "DecentralDevice": [ ... ],           // 顶层设备数组
  "IDevice": { ... },                   // 顶层 IDevice 对象
  "TotalInputLength": N,
  "TotalOutputLength": N,
  "Error": [],                          // 写命令特有
  "ErrorID": [],                        // 写命令特有
  // Function 字段在发送前已被 .clear()
}
```

inl 客户端需要：
1. 解析 `Error` / `ErrorID`（容忍 string vs []string、int vs []int）
2. 解析完整 `DecentralDevice` / `PNDriver` / `IDevice`（用于 diff baseline）
3. 写命令的响应**包含** `Function` 子对象（v2 修正：见 §0.5）。解析者可忽略，但应知道其存在

---

## 3. 字段路径总结表

| 命令 | Function.Value (v2 拼写) | 业务字段路径 | 必填 | 备注 |
|------|----------------|--------------|:--:|------|
| set-driver | `SetPNDriver` | `PNDriver.{DeviceName,IPAddress,SubnetMask,SetInTheProject}` | 4 | **模式 B**（顶层 `PNDriver`） |
| add-device | `AddPNDevice` | `Function.{RefGSD,DAP_ID}` | 2 | 模式 A；其他字段 C++ 自动生成 |
| remove-device | `UninstallPNDevice` | `Function.SetPNDeviceNum` | 1 | 1-based 索引 |
| set-device | `SetPNDevice` | `Function.SetPNDeviceNum` | 1 | **只做校验**，不直接改业务参数 |
| add-module | `AddModule` | `Function.{SetPNDeviceNum,ModuleID}` | 2 | 1-based 索引 |
| remove-module | `UninstallModule` | `Function.{SetPNDeviceNum,SetModuleSlot}` | 2 | |
| add-submodule | `AddSubmodule` | `Function.{SetPNDeviceNum,ModuleID,SubmoduleID}` | 3 | |
| remove-submodule | `UninstallSubmodule` | `Function.{SetPNDeviceNum,SetModuleSlot,SetSubmoduleSlot}` | 3 | |
| shield | `ShieldDevice` (v3 拼写纠正) | `Function.DeviceName` | 1 | **请求**模式 A, **响应**v3 CallbackNTJson (Function=string, Result 在 root, DeviceName 在 root) + `DataType=14` |
| unshield | `UNShieldDevice` (v3 拼写纠正) | `Function.DeviceName` | 1 | **请求**模式 A, **响应**v3 CallbackNTJson (Function=string, Result 在 root, DeviceName 在 root) + `DataType=14` |
| set-idevice | `SetIDevice` | `IDevice.{Activate,InputLength,OutputLength}` | 3 | **模式 C**（顶层 `IDevice`，非 Function 下）；长度 0-2048；Error 是单值 |

---

## 4. Step 10 BodyBuilder 设计建议

### 4.1 BodyBuilder 流程（统一模式）

```go
// 通用流程 (伪代码)
func configBodyBuilder(spec, args) (string, error) {
    // 1. 解析 --data 为业务字段 (e.g. {"RefGSD":"...","DAP_ID":"..."})
    business := parseJSON(args["data"])

    // 2. 自动 fetch 当前 topology (默认行为, --no-fetch 可禁用)
    var topology map[string]any
    if !args["no-fetch"] {
        topology = fetchCurrentTopology(target)  // 调 device-list-active 拉一次
    }

    // 3. 组装请求体
    body := map[string]any{
        "DataType": 12,
        "Function": merge({"Value": spec.Function}, business.FunctionFields),
    }
    // 模式 B 命令: 把业务字段放到顶层
    if spec.Function == "SetPNDriver" {
        body["PNDriver"] = business.PNDriver
    }
    // 模式 A 命令: DecentralDevice/PNDriver/IDevice 来自 fetch 后的 topology
    body["DecentralDevice"] = topology.DecentralDevice
    body["PNDriver"] = topology.PNDriver
    body["IDevice"] = topology.IDevice

    return jsonEncode(body), nil
}
```

### 4.2 --data 字段命名约定

为了避免和 C++ 字段名混淆，建议 inl 的 `--data` JSON 字段命名直接对应 C++ 路径：

```bash
# 模式 A 命令的 --data 格式 (业务字段都在 Function 下)
inl config add-device --data '{
  "RefGSD": "gsdml-v2.31-hms-abcc40-pir-20171101.xml",
  "DAP_ID": "0x0001"
}'

inl config remove-module --data '{
  "SetPNDeviceNum": 1,
  "SetModuleSlot": 3
}'

# 模式 B 命令的 --data 格式 (业务字段在 PNDriver 下)
inl config set-driver --data '{
  "DeviceName": "profinetdriver",
  "IPAddress": "192.168.3.15",
  "SubnetMask": "255.255.255.0",
  "SetInTheProject": true
}'

# 模式 C 命令的 --data 格式 (业务字段在顶层 IDevice 下)
inl config set-idevice --data '{
  "Activate": true,
  "InputLength": 64,
  "OutputLength": 64
}'
```

inl BodyBuilder 收到 `--data` 后，**根据 spec.Function 值**决定路由：
- 大多数命令 → 平铺到 `Function.<key>` (模式 A)
- `SetPNDriver` → 顶层 `PNDriver.<key>` (模式 B)
- `SetIDevice` → 顶层 `IDevice.<key>` (模式 C)

### 4.3 fetch 当前 topology 的策略

**默认开启 fetch**（覆盖 99% 用例）：
- inl 自动先 `device-list-active`，拿到 `DecentralDevice` / `PNDriver` / `IDevice`
- 与 `--data` 合并
- 发写命令

**`--no-fetch` 高级模式**：
- 用户已自行 fetch 并把 topology 完整塞到 `--data`（含 `DecentralDevice` 数组）
- inl 不再自动 fetch

**`--no-fetch` 触发条件**：
- AI 已在内存里有最新 topology（避免重复 fetch）
- 网络条件差，fetch 失败时仍想用 inl 发命令

### 4.4 11 个 BodyBuilder 汇总

| BodyBuilder | spec.Function | --data 必填字段 | 是否 fetch | 模式 |
|---|---|---|:--:|---|
| configSetDriverBody | SetPNDriver | DeviceName/IPAddress/SubnetMask/SetInTheProject | ✅ | B（顶层 `PNDriver`） |
| configAddDeviceBody | AddPNDevice | RefGSD/DAP_ID | ✅ | A |
| configRemoveDeviceBody | UninstallPNDevice | SetPNDeviceNum | ✅ | A |
| configSetDeviceBody | SetPNDevice | SetPNDeviceNum | ✅ | A |
| configAddModuleBody | AddModule | SetPNDeviceNum/ModuleID | ✅ | A |
| configRemoveModuleBody | UninstallModule | SetPNDeviceNum/SetModuleSlot | ✅ | A |
| configAddSubmoduleBody | AddSubmodule | SetPNDeviceNum/ModuleID/SubmoduleID | ✅ | A |
| configRemoveSubmoduleBody | UninstallSubmodule | SetPNDeviceNum/SetModuleSlot/SetSubmoduleSlot | ✅ | A |
| configShieldBody | ShildDevice (v2 typo) | DeviceName | ✅ | A |
| configUnshieldBody | UNShildDevice (v2 typo) | DeviceName | ✅ | A |
| configSetIDeviceBody | SetIDevice | Activate/InputLength (0-2048)/OutputLength (0-2048) | ❌ | **C**（顶层 `IDevice`） |

---

## 5. 单元测试设计（基于 C++ 字段）

每个 BodyBuilder 至少 5 个测试：

1. **Valid**：完整 `--data` → 输出 JSON 含正确 Function.Value + 业务字段
2. **MissingRequired**：缺字段 → error
3. **InvalidJSON**：非 JSON → error
4. **Empty**：--data="" → error
5. **AutoFetchLogic** (如果开启 fetch)：mock device-list-active 返回 topology，验证请求体中含 DecentralDevice 数组

---

## 6. 实机测试用例设计（基于字段表）

每条命令 1 个 e2e 测试：

```bash
# 1. 抓 baseline
inl --target 192.168.3.15 device list --output pre.json  # 读配置 (PROFINET_NETWORKTOPOLOGY_FILE)

# 2. dry-run
inl --target 192.168.3.15 config remove-device \
  --data '{"SetPNDeviceNum":1}' --dry-run
# 验证: DryRunFrame 含 Function.Value=UninstallPNDevice + Function.SetPNDeviceNum=1 + DecentralDevice (来自 pre.json)

# 3. 执行 (假设 remove 的是测试设备)
inl --target 192.168.3.15 config remove-device \
  --data '{"SetPNDeviceNum":1}' --yes
# 验证: stderr 显示请求成功, 响应 JSON 中 DecentralDevice 数量 -1

# 4. 抓 post
inl --target 192.168.3.15 device list-active --output post.json

# 5. 立即反向回滚 (重新 add)
# (add-device 不需要原数据, 因为 C++ 自动生成)
inl --target 192.168.3.15 config add-device \
  --data '{"RefGSD":"<原设备的 GSDName>","DAP_ID":"<原 DAP_ID>"}' --yes

# 6. 抓 rollback 后
inl --target 192.168.3.15 device list --output rollback.json  # 回滚后再读配置 diff

# 7. diff
diff pre.json rollback.json   # 期望为空
```

> 关键：`remove-*` 命令后**必须重新 fetch**当前 topology 才能加回（因为索引已变）。

---

## 7. C++ 行号索引（方便后续查阅）

| C++ 函数 | 行号 | 用途 |
|---------|------|------|
| `NetWorkTopologyFunction` (分派器) | L147-229 | Function.Value → 函数分派 |
| `ShieldDeviceByName` | L312-327 | shield |
| `UNShieldDeviceByName` | L329-344 | unshield |
| `GetGSDFileNetwork` | L346-362 | device-gsd-config |
| `GetGSDFileActivated` | L364-380 | device-gsd-active |
| `SetPNDriver` | L387-524 | set-driver |
| `AddPNDevice` | L815-1560 | add-device |
| `SetPNDevice` | L1561-1998 | set-device (校验) |
| `AddModule` | L1999-2350 | add-module |
| `AddSubmodule` | L2351-2705 | add-submodule |
| `UninstallPNDevice` | L2706-2766 | remove-device |
| `UninstallModule` | L2769-2950 | remove-module |
| `UninstallSubmodule` | L2952-3196 | remove-submodule |
| `SetIDevice` | L3198-3315 | set-idevice |
| `CheckAllNetworkTopologyJsonBeforeCompile` | L3317-3369 | compile 的预校验 |
| `CompileNetworkConfiguration` (PNCL) | L3390+ | compile 的实际编译 |

---

## 相关文档

| 主题 | 文档 |
|---|---|
| Step 10 计划 | [inl-step10-config-write-validation-plan.md](inl-step10-config-write-validation-plan.md) |
| 字段表（本文件） | [inl-config-field-reference.md](inl-config-field-reference.md) |
| 实机偏差登记 | [protocol/field-verification.md](protocol/field-verification.md) |
| 协议入口（C++ 源） | [PNConfigLibFileDesign.cpp](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/io-controller/src/pnconfiglib/PNConfigLibFileDesign.cpp) |
