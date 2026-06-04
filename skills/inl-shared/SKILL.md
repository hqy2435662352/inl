---
name: inl-shared
version: 1.0.0
description: "inl CLI 共享规则入口。使用 --target / --yes / --dry-run 前或收到 yes_required / confirmation_required 错误时必读。所有 inl-* skill 的前置依赖。"
metadata:
  requires:
    bins: ["inl"]
---

# inl 共享规则入口

**CRITICAL — 开始前 MUST 先用 Read 工具读取本文件**。所有 inl-* workflow skill 都依赖本规则。

`inl` 是工业 PC 上 NRC Socket 协议的 CLI 调试工具,通过 TCP:6000 与运行 `nrc2.out` 的工业 PC 通信。场景工作流见 [`../inl-workflow-profinet-write/SKILL.md`](../inl-workflow-profinet-write/SKILL.md)。

> **AI Agent 起步推荐**: 接到 inl 相关任务时,**先跑 `inl schema list`**(纯客户端, 无需 `--target`)获取所有 23 条 NRC 命令的元数据(name / group / use / description / risk / data_type / function / args 8 字段),再决定调用哪个子命令。schema list 输出的字段、枚举值、约束与 inl 二进制自身完全一致,无需另查文档。

---

## 1. `--target` 工业 PC

`--target` 是所有命令的**前置必填**参数,指向工业 PC 的 IPv4 地址(inl 内部自动追加 `:6000`)。

```bash
inl --target 192.168.3.15 gsd list
inl --target 192.168.3.15 config add-device --yes
inl --target 192.168.3.15 config add-device --dry-run
```

缺 `--target` 时,stderr 输出 `code: "target_required"` → AI **向用户追问 IP**,**不要**自选默认 IP(车间可能有多个 PC)。

---

## 2. 3 级 Risk 等级

| RiskLevel | 含义 | `--yes` 必需? | AI 自动追加 `--yes`? | 高危提示 |
|-----------|------|--------------|----------------------|----------|
| `read` | 只读,无副作用 | ❌ | ❌ | 无 |
| `write` | 写配置,可能影响设备状态 | ✅ | ✅ | 无 |
| `high-risk-write` | 高危(重启/擦除/编译) | ✅ | ❌ **不可** | stderr 输出 `⚠️  高危操作` |

**关键差异**:`high-risk-write` 触发 `confirmation_required` 错误,AI **不可自动追加 --yes**,必须**人工决策**;lark-cli 的 `write` 因云端可逆可自动追加,inl 不可逆操作必须显式确认。

查询方式:`inl <cmd> --help` 末尾追加 `Risk: <level>` 行(由 `installRiskHelpFunc` 自动注入,见 [`internal/nrc/annotation.go`](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/inl/internal/nrc/annotation.go))。

---

## 3. `--yes` 标志

写命令(`write` / `high-risk-write`)**必须**带 `--yes`,否则 inl **拒绝执行**并在 stderr 输出 `code: "yes_required"`。

**AI 调度行为**:
- `risk_level == "write"` → **AI 可自动在 argv 末尾追加 `--yes` 重试**(无需问用户)
- `risk_level == "high-risk-write"` → **不要自动追加**,改走 `confirmation_required` 路径,向用户显式确认

错误示例:

```json
{
  "type": "validation",
  "code": "yes_required",
  "message": "拒绝执行: config-add-device 是 write 操作, 需加 --yes 标志确认",
  "hint": "查看风险: inl config-add-device --yes --help",
  "detail": {
    "action": "config-add-device",
    "risk_level": "write",
    "roll_back_command": "inl config-remove-device --yes (若添加后想撤销)"
  }
}
```

退出码 1,stdout 为空。

---

## 4. `--dry-run` 标志

所有非 read 命令自动获得 `--dry-run` 标志。`--dry-run` **不连接 TCP**、**不发送 NRC 帧**、**不修改任何状态**;在内存构造 NRC 帧并把 `DryRunFrame` JSON 打印到 **stdout**,stderr 输出 `🛑 --dry-run 模式: 已跳过连接和发送`。

**强制流程**(`--dry-run` 必须跑在 `--yes` 之前):
1. 构造 argv: `inl --target X config add-device --dry-run`
2. 调命令,解析 stdout DryRunFrame JSON
3. 校验 payload 字段值是否与用户意图一致
4. 校验通过 → 在 argv 末尾追加 `--yes` 重试
5. 校验不通过 → 终止,向用户报错"payload 与意图不符"

DryRunFrame JSON 示例:

```json
{
  "description": "config-add-device (write) — 模拟 AddPNDevice 请求帧",
  "sync_byte": "0x4E66",
  "command": "0x9275",
  "datatype": 12,
  "function": "AddPNDevice",
  "payload": "{\"DataType\":12,\"Function\":{\"Value\":\"AddPNDevice\"}}",
  "payload_hex": "7B224461",
  "crc32": "0x6B926DFF",
  "total_bytes": 36,
  "risk": "write"
}
```

完整 9 字段含义见 [`../../inl/internal/output/dryrun.go`](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/inl/internal/output/dryrun.go) 注释。

---

## 5. 输出约定

| 流 | 内容 |
|---|------|
| **stdout** | `{ok, data, _notice}` Envelope 格式 / DryRunFrame |
| **stderr** | 进度 emoji(`🔌` / `📤` / `📥` / `💾` / `⚠️ ` / `🛑`)/ 警告 / 结构化错误 |

```bash
inl --target 192.168.3.15 gsd list > out.json 2> progress.log
# out.json:     Envelope 包裹的 JSON
# progress.log: 进度 emoji 行
```

**AI 解析规则**:
- 从 **stdout** 拿 Envelope,先判 `ok` 字段再消费 `data` 字段
- 从 **stderr** 拿结构化错误(用 `code` 字段判别)

### stdout Envelope

所有命令的 stdout 输出包裹在统一 JSON 信封中:

```json
{
  "ok": true,
  "identity": "inl",
  "data": { "DataType": 13, "Device": [ ... ] },
  "_notice": {
    "command": "gsd-list",
    "data_type": 13,
    "elapsed_ms": 234
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `ok` | bool | `true` = 成功 |
| `identity` | string | 固定为 `"inl"` |
| `data` | object | 命令的原始响应 JSON(工业 PC 返回内容,无字段重排/精度丢失) |
| `_notice` | object | 诊断信息:`command` / `data_type` / `elapsed_ms`(DCP 写操作额外含 `dcp_write: true` / `verify_with`) |

**AI 判断成功**: `if response.ok { process(response.data) }`

**DCP 写操作特例**(`device setup-name` / `device setup-ip`): 工业 PC 端无 JSON 响应,Envelope 形如 `{"ok": true, "data": null, "_notice": {"dcp_write": true, "verify_with": "topology-scan", ...}}`,AI 应在收到 Envelope 后立即跑 `topology scan` 闭环验证。

---

## 6. 结构化错误

所有错误以 **JSON 形式** 写到 **stderr**,AI **必须解析后再决策**。错误 struct 字段:`type` / `code` / `message` / `hint`(可选) / `detail`(可选)。

| 错误码 | 触发条件 | AI 行为 |
|--------|---------|---------|
| `target_required` | `--target` 缺 / 非法 IP | 向用户追问 IP |
| `yes_required` | 写命令缺 `--yes` | `write` → 自动追加 `--yes`;`high-risk-write` → 改走 `confirmation_required` |
| `confirmation_required` | 高危写命令缺 `--yes` | **不可自动追加**,向用户显式确认 |
| `unknown_command` | 命令名拼错 / Registry 找不到 | 检查拼写,提示 `inl --help` |
| `unexpected_response_command` | 响应 Command ≠ `0x9271` | 报告"工业 PC 端协议不匹配",**不要**自动重试 |
| `protocol_error` | TCP 失败 / 帧解码失败 / CRC 校验失败 | 报告网络/协议问题,检查工业 PC 状态 |

---

## 7. 拼写(已纠正)

C++ 控制器端 `ShieldDevice` / `UNShieldDevice` 拼写已纠正;inl 客户端 [`internal/nrc/commands.go:263, 276`](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/inl/internal/nrc/commands.go#L263) 命令名用 `config-shield` / `config-unshield`,请求体 `Function.Value` 用正确拼写。

| 实体 | 拼写 |
|------|------|
| inl 命令名 | `config-shield` / `config-unshield` |
| inl 请求体 `Function.Value` | `"ShieldDevice"` / `"UNShieldDevice"` |

详细 24 命令表(23 NRC + 1 schema 纯客户端)见 [`../../inl/AGENTS.md`](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/inl/AGENTS.md)。

---

## 8. `inl schema list` — AI Agent 能力自发现

```bash
inl schema list
```

**这是 AI Agent 接到 inl 相关任务时的第一个动作**(无需 `--target`, 不连工业 PC, 不发 NRC 帧)。

**stdout 输出** (Envelope 包裹):

```json
{
  "ok": true,
  "identity": "inl",
  "data": {
    "commands": [
      {
        "name": "gsd-list",
        "group": "gsd",
        "use": "list",
        "description": "列出工业 PC 上所有 GSDML 设备驱动",
        "risk": "read",
        "data_type": 13,
        "function": "",
        "args": null
      },
      {
        "name": "device-setup-name",
        "group": "device",
        "use": "setup-name",
        "description": "通过 DCP 设置设备名称",
        "risk": "write",
        "data_type": 14,
        "function": "2",
        "args": [
          {"name": "interface", "description": "端口名", "required": true},
          {"name": "mac", "description": "目标 MAC", "required": true},
          {"name": "name", "description": "新设备名称", "required": true}
        ]
      }
      // ... 其余 21 条
    ],
    "groups": {
      "config":    {"count": 12, "risk": "high-risk-write"},
      "device":    {"count": 7,  "risk": "mixed"},
      "gsd":       {"count": 2,  "risk": "read"},
      "interface": {"count": 1,  "risk": "read"},
      "topology":  {"count": 1,  "risk": "read"}
    }
  },
  "_notice": {
    "command": "schema-list",
    "command_count": 23,
    "group_count": 6
  }
}
```

**字段含义**:

| 字段 | 含义 | AI 用法 |
|------|------|---------|
| `commands[].name` | 命令全名 (如 `gsd-list`) | 拼到 `inl <name>` argv |
| `commands[].group` | 所属 Cobra 顶层组 | 拼到 `inl <group> <use>` argv |
| `commands[].use` | 子命令 (去掉 `<group>-` 前缀) | 同上 |
| `commands[].risk` | `read` / `write` / `high-risk-write` | 决定是否追加 `--yes` |
| `commands[].data_type` | NRC DataType 字段 | 协议层校验 (无需 AI 关心) |
| `commands[].function` | `Function.Value` 字符串或整数 | 协议层校验 (无需 AI 关心) |
| `commands[].args[]` | 必填 DCP 参数列表 | 决定是否需追加 `--interface` 等 flag |
| `groups[].count` | 该 group 内命令数 | 概览 |
| `groups[].risk` | group 最高风险等级 | 整组放行决策 (pure group 跳过 --yes) |
| `_notice.command_count` | NRC 命令数(不含 schema 自身) | 与 docs 交叉验证 |
| `_notice.group_count` | group 总数(含 schema 自身) | 同上 |

**为什么不需要 `--target`**:schema-list 读的是 inl **二进制自身**编译进去的 Registry,不是工业 PC 的状态。`init()` 对 `DataType=0 && Function==""` 的纯客户端命令跳过 DataType 唯一性检查,`runNrcCommand` 在 `spec.Group == GroupSchema` 时短路,直接返回 JSON,不走 TCP 连接流程。

**和 `inl gsd list` 的本质区别**:

| 命令 | 数据源 | 是否连工业 PC | 何时用 |
|------|--------|---------------|--------|
| `inl gsd list` | 工业 PC 端的 GSD 驱动库 | ✅ (TCP:6000) | 想看 PC 上**实际**有哪些 GSD 驱动 |
| `inl schema list` | inl 自身编译进去的 Registry | ❌ | 想看 inl **支持**哪些命令及参数 |

---

## 9. 参考

- [`../inl-workflow-profinet-write/SKILL.md`](../inl-workflow-profinet-write/SKILL.md) — 写命令工作流(4 层安全原则 + 11 命令安全矩阵)
- [`../../inl/AGENTS.md`](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/inl/AGENTS.md) — inl 客户端权威开发文档
- [`../../inl/internal/nrc/commands.go`](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/inl/internal/nrc/commands.go) — Registry 24 条命令元数据(23 NRC + 1 schema 纯客户端)
- [`../../inl/internal/nrc/frame.go`](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/inl/internal/nrc/frame.go) — NRC 帧编解码
- [`../../inl/internal/output/errors.go`](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/inl/internal/output/errors.go) — 结构化错误工厂
