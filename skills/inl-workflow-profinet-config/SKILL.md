---
name: inl-workflow-profinet-config
version: 1.0.0
description: "PROFINET 端到端配网 8 阶段编排: 环境评估 → 网络发现 → 方案规划 → 安全备份 → 预检 → 配置写入 → 编译激活 → 验证交付。AI 接到'配网/添加设备/修改配置/屏蔽设备'任务时强制加载。编排 Phase 1-3-8, 委托 Phase 4-7 给 inl-workflow-profinet-write。"
metadata:
  requires:
    bins: ["inl"]
    skills: ["inl-shared", "inl-workflow-profinet-write"]
---

# inl PROFINET 端到端配网工作流

**CRITICAL — 开始前 MUST 先用 Read 工具读取以下两个文件**:

1. [`inl-shared/SKILL.md`](../inl-shared/SKILL.md) — Risk 等级 / `--yes` / `--dry-run` / 错误码
2. [`inl-workflow-profinet-write/SKILL.md`](../inl-workflow-profinet-write/SKILL.md) — 写操作 4 层安全流程 (备份 → 预检 → 写入 → 编译激活)

本 Skill 是"编排器"——负责 Phase 1-3 (评估→发现→规划) + Phase 8 (验证交付)。Phase 4-7 (备份→预检→写入→编译) 委托给 `inl-workflow-profinet-write`。两者通过 `ChangePlan` 数据结构交接。

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
Phase 1 ──→ Phase 2 ──→ Phase 3 ──→ Phase 4 ──→ Phase 5 ──→ Phase 6 ──→ Phase 7 ──→ Phase 8
环境评估    网络发现    方案规划    安全备份    预检确认    配置写入    编译激活    验证交付
(Read)     (Read)     (AI)       (Manual)   (DryRun)   (Write)    (HiRisk)   (Read)
│                                                      │                      │
└────────── 本 Skill 负责 ──────────┘    └── write Skill ──┘    └─ 本 Skill ──┘
```

---

## Phase 1：环境评估

**目标**: 获取工业 PC 当前 PROFINET 配置的完整快照。

### 执行命令

```bash
# 1.1 GSD 驱动列表
inl --target <IP> gsd list

# 1.2 配置中拓扑 (CallBackJson — 设计师视角)
inl --target <IP> device list

# 1.3 激活中拓扑 (CallBackActivatedJson — 运行时视角)
inl --target <IP> device list-active

# 1.4 活动设备状态 (焊机运行状态)
inl --target <IP> device run
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
inl --target <IP> topology scan --interface <port>

# 2.2 GSD 匹配
inl --target <IP> gsd match --interface <port>
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

### Step 3.0: 意图确认

在生成 ChangePlan 前,**必须**评估用户意图是否明确。模糊时禁止猜测,逐条澄清 (每次最多 2 个问题):

| 维度 | 明确 | 模糊 |
|------|------|------|
| 目标设备 | "小原焊机" → 命中 OBARA | "焊机" (多个焊机 GSD) |
| 操作类型 | "添加" / "删除" / "改 IP" | "配一下" / "搞一下" |
| 目标参数 | "IP 改成 192.168.2.20" | "改下 IP" (无具体值) |
| 设备名称 | "叫 heron-weld-2" | 未指定 |
| 作用范围 | "只配这台" | "全部配好" |

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
  "summary": "添加 1 个 OBARA SIV31-40 焊机, IP 192.168.2.20, 名称 heron-weld-2",
  "changes": [
    {
      "command": "config-add-device",
      "risk": "write",
      "payload": {"DeviceName": "heron-weld-2", "IPAddress": "192.168.2.20"},
      "requires_backup": false,
      "online_status": "offline",
      "rollback": "config-remove-device --data '{\"DeviceName\":\"heron-weld-2\"}'"
    }
  ],
  "conflicts_detected": [],
  "requires_compile": true
}
```

---

## Phase 4-7：委托给 inl-workflow-profinet-write

将 `ChangePlan` 传递给 `inl-workflow-profinet-write` Skill 执行:

```
Phase 4 安全备份 ──→ SCP 备份 networktopology.json (remove-*/compile 强制备份)
Phase 5 预检确认 ──→ 逐条 config * --dry-run, 校验 DryRunFrame
Phase 6 配置写入 ──→ config * --yes 依次执行
Phase 7.3 DCP 分配 ──→ 对在线设备 device setup-name/ip 推送参数
Phase 7.4 编译激活 ──→ config compile --yes (高危, 需人工确认)
```

> write Skill 执行完成后,控制权回到本 Skill 的 Phase 8。

---

## Phase 8：验证交付

**目标**: 验证编译后的激活中拓扑与预期配置一致。

### 执行命令

```bash
inl --target <IP> device list-active    # 激活中拓扑
inl --target <IP> device list           # 配置中拓扑 (对比基准)
```

### 对比校验项

> `device list` 与 `device list-active` 顶层结构一致 (IDevice + PNDriver + DecentralDevice[]), 直接逐字段对比。

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
  "backup_path": "",
  "verification_report": null
}
```

## 状态机

```
INIT → ASSESSING → DISCOVERING → CLARIFYING → PLANNING → REVIEWING → BACKING_UP
  → PRECHECKING → WRITING → DCP_ASSIGNING → COMPILING → VERIFYING → COMPLETE
                                                                  → ROLLING_BACK → FAILED
```

| 状态 | 操作 | 退出条件 |
|------|------|---------|
| `ASSESSING` | gsd list + device list* + device run + 语义富化 + 反向评估 | 全部完成 |
| `DISCOVERING` | interface list + topology scan + gsd match | 完成或超时 (部分离线正常) |
| `CLARIFYING` | 逐条确认模糊意图 (每次 ≤2 个问题) | 全部澄清 |
| `PLANNING` | 冲突检测 + 变更规划 + 生成 ChangePlan | ChangePlan 生成 |
| `REVIEWING` | 展示 diff, 等待用户批准 | 用户 approve/reject |
| `BACKING_UP` | 提示 SCP 命令, 等待用户确认 | 备份确认 |
| `PRECHECKING` | 逐条 config * --dry-run (委托 write) | 全部通过 |
| `WRITING` | config * --yes (委托 write) | 全部写入 |
| `DCP_ASSIGNING` | device setup-name/ip 推送在线设备参数 | 完成或跳过 |
| `COMPILING` | config compile --yes (委托 write, 高危确认) | compile 返回 |
| `VERIFYING` | device list-active ↔ device list 比对 | 验证通过/失败 |
| `COMPLETE` | 配网成功 | 终端 |
| `ROLLING_BACK` | 反向命令或 SCP 恢复 | 回滚完成 |
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
| 请求备份 | Phase 4 进入 | 列出 SCP 命令 |
| 高危确认 | Phase 7 compile 前 | "即将编译并激活新配置。确定?" |

---

## 🚫 AI 禁止行为

- ❌ 跳过 GSD 语义富化
- ❌ 意图模糊时自行猜测
- ❌ 将 `192.168.2.14` 分配给从站
- ❌ 跳过 remove-*/compile 的强制备份
- ❌ 盲配完成后报告"设备已验证通过"——必须标注 "⚠️ 设备未实际验证"
- ❌ 用 `device run` 的 Status 判断是否应该配置
- ❌ **臆想设备/模块/子模块 ID** — `config add-device` 的 `DAP_ID`、`config add-module` 的 `ModuleID`、`config add-submodule` 的 `SubmoduleID` **必须**从 `gsd list` 返回的实际值中提取，**严禁**自编（如 `"0x0001"`）

---

## 参考

- [inl-shared/SKILL.md](../inl-shared/SKILL.md) — 共享规则 (必读)
- [inl-workflow-profinet-write/SKILL.md](../inl-workflow-profinet-write/SKILL.md) — 写操作安全流程 (委托 Phase 4-7)
- [inl-workflow-design.md](../../docs/inl-workflow-design.md) — 工作流设计文档 (完整规范)
