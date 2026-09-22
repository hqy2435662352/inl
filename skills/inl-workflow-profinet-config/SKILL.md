---
name: inl-workflow-profinet-config
version: 1.0.0
description: "PROFINET 端到端配网 8 阶段编排: 环境评估 → 网络发现 → 方案规划 → 配置写入 → 编译激活 → 验证交付。AI 接到'配网/添加设备/修改配置/屏蔽设备'任务时强制加载。"
metadata:
  requires:
    bins: ["inl"]
    skills: ["inl-shared"]
---

# inl PROFINET 端到端配网工作流

**CRITICAL — 开始前 MUST 先用 Read 工具读取 [`inl-shared/SKILL.md`](../inl-shared/SKILL.md)**，其中包含 `--target` / `--yes` / `--dry-run` / Risk 等级 / 结构化错误等共享约定。

---

## 适用场景

AI Agent 收到下列任务时,**强制**触发本 Skill:

- "配网" / "给产线配网" / "配置 PROFINET 网络"
- "添加一台焊机到配置"
- "修改主站 IP 为 xxx"
- "把设备 X 屏蔽掉"
- "新来了 3 个从站, 帮我配网"
- "编译并激活配置"

## 不适用场景

下列场景**禁止**走本 Skill,直接用读命令:

- "看一下现在配了哪些设备" → `device list` / `device list-active` (读)
- "现在焊机在跑没" → `device run` (读)
- "扫描一下有哪些在线设备" → `topology scan` (读)

---

## 8 阶段全景

```
Phase 0 ──→ Phase 1 ──→ Phase 2 ──→ Phase 3 ──→ Phase 4 ──→ Phase 5 ──→ Phase 6 ──→ Phase 7 ──→ Phase 8
Schema发现  环境评估    网络发现    方案规划    写入确认    配置写入    编译激活    DCP 分配    验证交付
(Client)   (Read)     (Read)     (AI)       (DryRun)   (Write)    (HiRisk)   (DCP)      (Read)
```

> **例外**: `config shield` 和 `config unshield` 虽然是 config 命令组，但**运行时生效**（不走 compile 链路），写入后直接生效无需编译激活。

---

## Phase 0：Schema 发现（强制）

**目标**: 获取所有命令的精确字段元数据和全局标志，避免凭记忆拼凑参数。

**这是强制步骤，不可跳过。**

```bash
inl schema list
inl --help
```

`schema list` 返回 `commands[].args[].fields[]` 提供：
- 字段名（`name`）、字段类型（`type`）、是否必填（`required`）、描述与示例

`--help` 展示全局标志（`--target`、`--output`、`--retry`、`--format`）和写命令专属标志（`--yes`、`--dry-run`、`--stdin`、`--data-file`）。

> 两者互补：schema list = 数据模型，--help = 调用接口。组合使用后即可构造合法命令，无需探索项目文件。

> **构造 `--data` JSON 时，必须严格对照 `fields[]`。如 `config-add-device` 的 fields 只有 `RefGSD` + `DAP_ID`，则不应传入 `DeviceName` / `IPAddress`。**

---

## Phase 1：环境评估

**目标**: 获取工业 PC 当前 PROFINET 配置的完整快照。

### 执行命令

> **🚫 禁止并行执行**：Phase 1 的 4 条命令必须串行，每条等前一条完成后再执行下一条。并行连接会导致 nrc2.out 拒绝连接。

> 所有输出大的命令使用 `--output $env:TEMP\...` 写入文件，避免终端截断和 `Out-File` 安全检查。命令执行后用 Read 工具直接读取 JSON 文件。

```bash
# 1.1 GSD 驱动列表（输出 >20KB，必须 --output）
inl --target <IP> gsd list --output $env:TEMP\gsd.json

# 1.2 配置中拓扑 (CallBackJson — 设计师视角)
inl --target <IP> device list --output $env:TEMP\device_list.json

# 1.3 激活中拓扑 (CallBackActivatedJson — 运行时视角)
inl --target <IP> device list-active --output $env:TEMP\device_active.json

# 1.4 活动设备状态 (焊机运行状态)
inl --target <IP> device run --output $env:TEMP\device_run.json
```

### Step 1.5: GSD 语义富化

对 `gsd list` 返回的每个设备驱动,生成 `semantic_profile`:

1. 提取 VendorName / ProductFamily / DAP_Name / GSDName / Module 名称
2. 从 IOData 名称推断物理功能 (如 "Inlet Flow Rate" → 流量传感器)
3. 联网搜索 VendorName + ProductFamily 关键词
4. 生成中文/英文语义标签 + 标注置信度 (`high` / `medium` / `low` / `unknown`)

> ⚠️ 语义富化是补充信息,不是权威结论。用户明确意图始终优先于 AI 推断。`confidence != high` 的设备不参与自动语义匹配。

### 反向评估用户初始意图

Phase 1 完成后,AI **必须**回到用户的初始输入,用富化后的设备知识重新解读:

```
用户: "我要配小原焊机的网, 再加一台 SMC 阀岛"
  ↓ Phase 1 完成后
富化 GSD:  OBARA (confidence:high) → SIV31-40 中频焊控
          SMC  (confidence:high) → EX245-SPN 阀岛
  ↓ 反向匹配
结果: "小原焊机" → OBARA SIV31-40 ✅
      "SMC 阀岛" → SMC EX245-SPN  ✅
```

### Phase 1 输出: `CurrentState`

```json
{
  "gsd_templates": {
    "drivers": [
      {"VendorID": "0x038A", "VendorName": "OBARA",
       "semantic_profile": {"confidence": "high", "device_type_cn": "中频逆变焊控",
        "brand_aliases": ["小原"], "match_keywords": ["焊机","小原焊机"]}}
    ]
  },
  "device_instances": { "PNDriver": {...}, "IDevice": {...}, "DecentralDevice": [...] },
  "active_topology":   { "PNDriver": {...}, "DecentralDevice": [...] },
  "running_devices":   [{"DeviceName": "heron-weld", "Status": "运行中"}],
  "resolved_intent": {
    "original_user_input": "我要配小原焊机的网",
    "resolved_devices": [
      {"gsd_name": "GSDML-V2.31-OBARA-SIV31-40-20190707.xml",
       "matched_by": "P2_semantic", "confidence": "high"}
    ]
  }
}
```

---

## Phase 2：网络发现

**目标**: DCP 发现在线设备,获取物理世界真相。部分设备离线是**正常**状态——盲配是现场常见工作方式。

> **🔴 `device list-active` 不是在线设备列表！**
> `device list-active` 返回的是工业 PC 端已激活的配置拓扑，**不是**实际在线的物理设备。
> 只有 `topology scan` (DCP Layer 2 广播) 能确认哪些设备真正在线。
> 如果 `topology scan` 失败，**不得**用 `device list-active` 推断在线设备，必须标记为"盲配"。

> **🚫 禁止并行执行**：Phase 2 的命令同样必须串行。

### Step 2.0: 端口选择

DCP 发现需要指定物理端口。先获取工业 PC 可用端口:

```bash
inl --target <IP> interface list
```

自动选择规则:
- 存在 `enp4s0` → 默认选中 (PROFINET 常用端口名)
- 仅一个非 loopback 端口 → 默认选中
- 多个端口 → 列出所有端口, 提示用户选择

### 执行命令

```bash
# 2.1 DCP 发现在线设备
inl --target <IP> topology scan --interface <port> --output $env:TEMP\scan.json

# 2.2 GSD 匹配
inl --target <IP> gsd match --interface <port> --output $env:TEMP\gsd_match.json
```

### 每设备粒度判断

```
DiscoveredDevices:
  heron-weld     (online_status: online)  → Phase 8 做 DCP 级比对
  smc-valve-01   (online_status: offline) → Phase 8 仅做配置文件级比对
```

| 属性 | 在线设备 | 离线设备 (盲配) |
|------|---------|----------------|
| Phase 3-7 | 正常操作 | 正常操作 |
| Phase 8 | 🟢 DCP 级比对 | 🟡 配置文件级比对 |

---

## Phase 3：方案规划

**目标**: 对比"当前状态"与"用户期望", 生成 `ChangePlan`。

### Step 3.1: 意图确认

在生成 ChangePlan 前,**必须**评估用户意图是否明确。模糊时禁止猜测,逐条澄清 (每次最多 2 个问题):

| 维度 | 明确 | 模糊 |
|------|------|------|
| 目标设备 | "小原焊机" → 命中 OBARA | "焊机" (多个焊机 GSD) |
| 操作类型 | "添加" / "删除" / "改 IP" | "配一下" / "搞一下" |
| 目标参数 | "IP 改成 192.168.2.20" | "改下 IP" (无具体值) |
| 设备名称 | "叫 heron-weld-2" | 未指定 |
| 作用范围 | "只配这台" | "全部配好" |

### Step 3.1.5: set-device 参数确认

当 ChangePlan 包含 `config-set-device` 时，由于全部 6 字段必传，**必须**逐字段与用户确认（不修改的字段传 device list 原值，用户未指定的字段用默认值）：

| 字段 | 默认值 | 说明 |
|------|--------|------|
| `DeviceName` | 原值 | 用户指定新名称则用新的，否则沿用 `device list` 中的值 |
| `IPAddress` | 原值 | 用户指定新 IP 则用新的，否则沿用 `device list` 中的值 |
| `SubnetMask` | `255.255.255.0` | 用户未指定时默认此值 |
| `ReductionRatio` | `16` | 设备信号更新周期（ms），用户未指定时默认 16ms |
| `SetInTheProject` | `true` | 是否在项目中配置 IP 地址。true 时 IPAddress/SubnetMask 生效；false 时改用设备实际 IP 通信 |

AI 必须向用户展示完整参数表并要求确认：
```
即将设置设备 [ex245] 的参数：
  DeviceName:      ex245           (原值不变)
  IPAddress:       192.168.2.17    (原值不变)
  SubnetMask:      255.255.255.0   (默认)
  ReductionRatio:  16              (默认 16ms)
  SetInTheProject: true            (默认)
  确认？(yes/修改某项/取消)
```

### Step 3.2: Multi-DAP 选择（强制）

当 Phase 1 `gsd list` 中目标设备的 `DAP[]` 数组长度 > 1 时，**必须**让用户选择使用哪个 DAP 接口。

```
检测到多 DAP:
  GSD: GSDML-V2.34-SMC-EX245-SPN-20181102.xml (SMC EX245)
  DAP 列表:
    [1] DAP 1 — EX245-SPN FX  (DAP_ID="DAP 1")
    [2] DAP 2 — EX245-SPN Cu  (DAP_ID="DAP 2")

AI 必须提问: "SMC EX245 有两个接口版本，请选择：
    [1] EX245-SPN FX (DAP 1)
    [2] EX245-SPN Cu (DAP 2)"
```

> `DAP_ID` 是 `config add-device --data` 的必填字段。多 DAP 时不可自动选第一个，必须由用户确认。

### Step 3.3: 模块选择（无预装模块设备）

当目标 DAP 的全部 `UseableModules` **只有** `AllowedInSlots`（没有 `FixedInSlots` 或 `UsedInSlots`）时，说明该设备**没有任何预装模块**，需要由用户决定要安装哪些模块。

```
检测到无预装模块设备:
  Device: TMGTE SUNKE SK335x (DAP: DAP 335X)
  UseableModules (全部仅 AllowedInSlots):
    [1] IN_MODULE  — AllowedInSlots: 1..64
    [2] OUT_MODULE — AllowedInSlots: 1..64

AI 必须提问: "TMGTE SUNKE 没有预装模块。以下是可用模块：
    [1] IN_MODULE (输入模块, 可安装到 slot 1-64)
    [2] OUT_MODULE (输出模块, 可安装到 slot 1-64)
    请选择要添加的模块（可多选，如 '1,2' 或 'all'）"
```

选中的模块追加到 `ChangePlan.changes[]` 中作为 `config add-module` 条目，在 Phase 5-6 中按顺序执行（先 add-device 再 add-module）。

> **判断公式**：DAP 无预装模块 ⇔ `UseableModules[].FixedInSlots` 全部为空且 `UseableModules[].UsedInSlots` 全部为空。
> 如果至少一个模块有 `FixedInSlots` 或 `UsedInSlots`，则该 DAP 有预装模块，无需 Step 3.3。

### 设备匹配优先级

| P1 | P2 | P3 |
|:--:|:--:|:--:|
| GSD 文件名字符串匹配 🟢 | 高置信度语义匹配 🟡 | VendorName 显式匹配 🟡 |

- P1 (GSD 文件名) 是最可靠的——现场工程师直接给 `.xml` 文件名
- `confidence != high` 的设备只参与 P1/P3,**不参与 P2 自动语义匹配**

### 冲突检测 (10 条验证规则)

| # | 规则 | 触发条件 | 修复动作 |
|:--:|------|---------|---------|
| 1 | IP 格式 | 非合法 IPv4 | 提示修正 |
| 2 | IP 冲突 | IP 在验证域内已存在 | 自动递增或提示修正 |
| 3 | 主站 IP 保留 | 分配了 192.168.2.14 | 自动跳到下一可用 IP |
| 4 | Name 长度 | > 63 字符 | 截断或提示修正 |
| 5 | Name 格式 | 含非法字符 | 提示修正 |
| 6 | Name 重复 | 名称在验证域内已存在 | 提示修正 |
| 7 | 子网掩码匹配 | 从站掩码 ≠ 主站掩码 | 自动修正为主站掩码 |
| 8 | 同子网检查 | 从站 IP 不在主站子网内 | 提示修正 |
| 9 | MAC 格式 | 非合法 MAC | 提示修正 |
| 10 | GSD 缺失 | 目标设备无匹配 GSD 驱动 | 请求上传 GSDML |

### IP 自动分配

当用户未指定 IP 时:
- 起始: `192.168.2.1`, 逐设备递增
- ⚠️ 跳过 `192.168.2.14` (主站默认 IP 保留)
- 子网掩码: 继承 PNDriver.SubnetMask (默认 `255.255.255.0`)
- 设备名称: 基于设备类型自动生成 (如 `obara-siv31-01`)

### Phase 3 输出: `ChangePlan`

```json
{
  "summary": "添加 1 个 OBARA SIV31-40 焊机 (DAP=SIV31 Std), IP 192.168.2.20, 名称 heron-weld-2",
  "changes": [
    {
      "command": "config-add-device",
      "risk": "write",
      "payload": {"RefGSD": "GSDML-V2.31-OBARA-SIV31-40-20190707.xml", "DAP_ID": "DAP"},
      "requires_backup": false,
      "online_status": "offline",
      "rollback": "config-remove-device --data '{\"SetPNDeviceNum\":1}'"
    },
    {
      "command": "config-add-module",
      "risk": "write",
      "payload": {"SetPNDeviceNum": 1, "ModuleID": "ID_MOD_DX1"},
      "requires_backup": false,
      "online_status": "offline",
      "depends_on": "config-add-device"
    }
  ],
  "conflicts_detected": [],
  "requires_compile": true
}
```

---

## Phase 4-7：配置写入与编译

按 `ChangePlan.changes` 的顺序逐一执行。**注意依赖顺序**：`config add-device` 必须先于 `config add-module`（设备必须先存在才能添加模块）。

```
Phase 4 写入确认 ──→ inl --target <IP> config <subcommand> --data '<json>' --dry-run
Phase 5 配置写入 ──→ inl --target <IP> config <subcommand> --data '<json>' --yes
Phase 6 编译激活 ──→ inl --target <IP> config compile --yes（如需编译）
Phase 7 DCP 分配 ──→ 参见 inl-workflow-profinet-dcp（仅在线设备）
```

> `<subcommand>` 取自 `ChangePlan.changes[].command`（如 `add-device`、`set-device`、`remove-device`），`<json>` 取自 `changes[].payload`。

### Windows/PowerShell 注意事项

在 PowerShell 下构造 `--data` JSON 时，**必须**使用 `--stdin` 管道直传，禁止文件 I/O：

```powershell
# ✅ 正确：管道直传（零文件 I/O，不触发 Agent 安全检查）
'{"RefGSD":"GSDML-...-SMC-EX245-...xml","DAP_ID":"0x00000010"}' |
    inl --target 192.168.3.15 config add-device --stdin --yes

# ❌ 错误：写文件（Out-File 触发 Agent 安全检查）
# ❌ 错误：直接在命令行写 JSON（PowerShell 剥离引号）
```

详见 [inl-shared §1.5](../inl-shared/SKILL.md#15-windowspowershell-json-传参指南)。

---

## Phase 8：验证交付

**目标**: 验证编译后的激活中拓扑与预期配置一致。

### 执行命令

```bash
inl --target <IP> device list-active --output $env:TEMP\verified_active.json    # 激活中拓扑
inl --target <IP> device list --output $env:TEMP\verified_config.json           # 配置中拓扑 (对比基准)
```

### 对比校验项

> `device list` 与 `device list-active` 顶层结构一致 (IDevice + PNDriver + DecentralDevice[]), 直接逐字段对比。
>
> ⚠️ 如果配网过程中执行了 DCP 写命令（`device setup-name/ip`），Phase 8 后需额外执行 `topology scan` 来验证 DCP 写操作效果（DCP 命令无 JSON 响应，`topology scan` 是唯一确认手段）。

| 校验项 | 配置源 | 激活源 | 通过条件 |
|--------|--------|--------|---------|
| DecentralDevice 数量 | `DecentralDevice[]` 长度 | `DecentralDevice[]` 长度 | 一致 |
| PNDriver IP | `PNDriver.IPAddress` | `PNDriver.IPAddress` | 完全一致 |
| 设备名称 | `DecentralDevice[].DeviceName` | `DecentralDevice[].DeviceName` | 完全一致 |
| 设备 IP | `DecentralDevice[].IPAddress` | `DecentralDevice[].IPAddress` | 完全一致 |
| IDevice | `IDevice.InputLength` | `IDevice.InputLength` | 完全一致 |

### Phase 8 输出: 验证报告

```json
{
  "verified": true,
  "diff": {"added": [], "removed": [], "changed": []},
  "new_active_topology": { /* device list-active 快照 */ },
  "backup_path": "./backups/topology_20260603_143000.json"
}
```

**⚠️ 在线 vs 离线验证**:
- 在线设备: 🟢 DCP 级——对比在线 MAC/Name/IP + 配置文件
- 离线设备: 🟡 配置文件级——仅对比 `device list` vs `device list-active` (设备未到场时这是正常的)

---

## Session Context

AI Agent 在整个工作流中维护 `session_context`:

```json
{
  "current_state": "ASSESSING",
  "state_history": ["INIT"],
  "target_ip": "192.168.3.15",
  "resolved_intent": { "original_user_input": "...", "resolved_devices": [...] },
  "current_topology": { /* Phase 1 快照 */ },
  "discovered_devices": { /* Phase 2 快照 */ },
  "change_plan": { /* Phase 3 输出 */ },
  "verification_report": null
}
```

## 状态机

```
INIT → SCHEMA → ASSESSING → DISCOVERING → CLARIFYING → PLANNING → REVIEWING → WRITING
  → COMPILING → DCP_ASSIGNING → VERIFYING → COMPLETE
                               → FAILED
```

| 状态 | 操作 | 退出条件 |
|------|------|---------|
| `SCHEMA` | `inl schema list` → 获取所有命令 fields 元数据 | schema list 成功 |
| `ASSESSING` | gsd list + device list* + device run + 语义富化 + 反向评估 | 全部完成 |
| `DISCOVERING` | interface list + topology scan + gsd match | 完成或超时 (部分离线正常) |
| `CLARIFYING` | 逐条确认模糊意图 (每次 ≤2 个问题) | 全部澄清 |
| `PLANNING` | 冲突检测 + 变更规划 + 生成 ChangePlan | ChangePlan 生成 |
| `REVIEWING` | 展示 diff, 等待用户批准 | 用户 approve/reject |
| `WRITING` | 逐条执行 config * --dry-run → config * --yes | 全部写入 |
| `DCP_ASSIGNING` | `device setup-name/ip` 推送在线设备参数（无 JSON 响应, 写后需 `topology scan` 验证, 详见 inl-workflow-profinet-dcp） | 完成或跳过 |
| `COMPILING` | config compile --yes | compile 返回 |
| `VERIFYING` | device list-active ↔ device list 比对 | 验证通过/失败 |
| `COMPLETE` | 配网成功 | 终端 |
| `FAILED` | 不可恢复 | 终端 |

---

## 用户交互协议

| 交互点 | 触发条件 | AI 行为 |
|--------|---------|--------|
| 请求 `--target` | 工作流启动时未提供 | "请提供工业 PC IP" |
| 请求 GSDML | 设备无匹配 GSD 驱动 | "请提供 GSDML 文件路径" |
| 端口选择 | 多个网络端口 | 列出端口, 提示选择 |
| 意图确认 | 用户输入模糊 | 逐条提问 (每次 ≤2 个问题) |
| 展示变更方案 | Phase 3 完成 | 展示 ChangePlan + diff |
| 高危确认 | Phase 6 compile 前 | "即将编译并激活新配置。确定?" |

---

## 🚫 AI 禁止行为

- ❌ 跳过 GSD 语义富化
- ❌ 意图模糊时自行猜测
- ❌ 将 `192.168.2.14` 分配给从站
- ❌ 盲配完成后报告"设备已验证通过"——必须标注 "⚠️ 设备未实际验证"
- ❌ 用 `device run` 的 Status 判断是否应该配置
- ❌ **臆想设备/模块/子模块 ID** — `config add-device` 的 `DAP_ID`、`config add-module` 的 `ModuleID`、`config add-submodule` 的 `SubmoduleID` **必须**从 `gsd list` 返回的实际值中提取，**严禁**自编（如 `"0x0001"`）
- ❌ **多 DAP 设备自动选第一个** — `DAP[].length > 1` 时必须让用户选择，不可自行决定
- ❌ **无预装模块设备跳过模块选择** — `UseableModules[]` 全为 `AllowedInSlots` 时必须让用户确认要安装哪些模块

---

## 相关 Skill

- [inl-shared](../inl-shared/SKILL.md) — 共享规则 (必读)
- [inl-workflow-profinet-dcp](../inl-workflow-profinet-dcp/SKILL.md) — DCP 写操作独立工作流 (Phase 7)
