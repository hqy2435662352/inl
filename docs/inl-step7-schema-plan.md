---
title: inl 第 7 步开发计划 — schema list 命令
tags: [inl, development, plan, schema, ai-discovery]
created: 2026-06-03
status: draft
---

# inl — 第 7 步：schema list 命令

## 目标

实现 `inl schema list` — 纯客户端命令，遍历 `internal/nrc/commands.go` 的 Registry 输出所有 23 条命令的元数据，让 AI Agent 在不需要查文档的情况下自发现 inl 的能力。

## 设计

### 为什么不需要连工业 PC

`schema list` 读的是 inl **二进制自身**编译进去的 Registry，不是工业 PC 的状态。跟 `inl gsd list`（读工业 PC 的 GSD 驱动库）是完全不同的数据源：

```
inl gsd list   → inl ──TCP :6000──→ nrc2.out ──→ CallBackGsdFileList() ──→ 返回 JSON
inl schema list → inl ─────────────→ 直接读 Registry ──────────────────→ 返回 JSON
```

### 命令行形态

```bash
$ inl schema list

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
        "args": []
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
      "gsd":       {"count": 2,  "risk": "read"},
      "device":    {"count": 7,  "risk": "mixed"},
      "config":    {"count": 12, "risk": "mixed"},
      "interface": {"count": 1,  "risk": "read"},
      "topology":  {"count": 1,  "risk": "read"}
    }
  },
  "_notice": {
    "command": "schema-list",
    "command_count": 23,
    "group_count": 5
  }
}
```

### `--target` 不需要

因为不连工业 PC，`--target` 不是必需的。但当前 `runNrcCommand` 中硬检查了 `targetFlag == ""` 返回 `target_required`。需要在 schema-list 的 case 中跳过这个检查。

---

## 文件清单

```
inl/
├── internal/nrc/
│   └── commands.go          ← 改: Registry 新增 schema-list 条目
├── main.go                  ← 改: runNrcCommand 中 schema-list 短路处理
├── skills/inl-shared/
│   └── SKILL.md             ← 改: 命令列表新增 schema 组
└── AGENTS.md                ← 改: Registry 表 + Source Layout
```

**新增文件**: 0
**修改文件**: 4
**新增依赖**: 0

---

## Step 1：Registry 新增 `schema-list`

### 文件位置

`inl/internal/nrc/commands.go`

### 新增 Group 常量

```go
const (
    GroupGsd       CommandGroup = "gsd"
    GroupDevice    CommandGroup = "device"
    GroupConfig    CommandGroup = "config"
    GroupInterface CommandGroup = "interface"
    GroupTopology  CommandGroup = "topology"
    GroupSchema    CommandGroup = "schema"      // ← 新增
)
```

### 新增 Registry 条目

```go
{
    Name:        "schema-list",
    Code:        0,                // 纯客户端命令, 不发送 NRC 帧
    DataType:    0,                // 无对应 NRC DataType
    Direction:   DirectionRequest,
    Description: "列出所有可用命令的元数据 (供 AI Agent 自发现)",
    Risk:        RiskRead,
    Response:    nil,
    Function:    "",               // 无 NRC 分发
    Group:       GroupSchema,
    Args:        nil,
    BodyBuilder: nil,              // 不发帧, 无需 BodyBuilder
},
```

> `Code: 0` / `DataType: 0` 是哨兵值，表示此命令不发送 NRC 帧。在 `runNrcCommand` 中检查 `spec.DataType == 0` 时短路处理。

### Registry 分布

24 条：gsd=2, device=7, config=12, interface=1, topology=1, schema=1

### `init()` 唯一性检查调整

当前 `init()` 中 `DataType=0` 会触发 "DataType 重复" panic。需要改为：`Function == "" && DataType == 0` 时跳过 DataType 唯一性检查（允许多个纯客户端命令共存）。

---

## Step 2：`main.go` — `runNrcCommand` 短路处理

### buildGroupCmd 新增

```go
case nrc.GroupSchema:
    use = "schema"
    short = "AI Agent 能力发现"
    long = "列出所有可用命令的元数据 (纯客户端, 不连接工业 PC)。"
    pureGroup = true
```

### main() 遍历新增

```go
for _, g := range []nrc.CommandGroup{
    nrc.GroupGsd, nrc.GroupDevice, nrc.GroupConfig,
    nrc.GroupInterface, nrc.GroupTopology, nrc.GroupSchema,   // ← 新增
} {
```

### runNrcCommand 短路

在进入 TCP 连接逻辑之前插入：

```go
func runNrcCommand(name string) func(*cobra.Command, []string) error {
    return func(cmd *cobra.Command, args []string) error {
        spec, ok := nrc.LookupByName(name)
        if !ok {
            return &output.Error{Code: "unknown_command", ...}
        }

        // === 纯客户端命令 —— 短路, 不连接工业 PC ===
        if spec.Group == nrc.GroupSchema {
            data := buildSchemaJSON()
            notice := map[string]interface{}{
                "command":       spec.Name,
                "command_count": len(nrc.Registry) - 1, // 减去自身
                "group_count":   len(groupsFromRegistry(nrc.Registry)),
            }
            return output.WriteSuccess(cmd.OutOrStdout(), data, notice)
        }

        // === 以下为 NRC 命令的标准流程 ===
        if targetFlag == "" {
            return &output.Error{Code: "target_required", ...}
        }
        // ... 连接 / 发送 / 接收 / Envelope 输出 ...
    }
}
```

### buildSchemaJSON 函数

```go
func buildSchemaJSON() []byte {
    type cmdEntry struct {
        Name        string                `json:"name"`
        Group       string                `json:"group"`
        Use         string                `json:"use"`
        Description string                `json:"description"`
        Risk        string                `json:"risk"`
        DataType    int                   `json:"data_type"`
        Function    string                `json:"function"`
        Args        []nrc.ArgumentSpec    `json:"args"`
    }

    commands := make([]cmdEntry, 0, len(nrc.Registry)-1) // 不含 schema-list 自身
    groupCounts := make(map[string]int)
    groupRisks := make(map[string]string)

    for _, spec := range nrc.Registry {
        if spec.Name == "schema-list" {
            continue // 不报告自身
        }
        use := spec.Name[len(spec.Group)+1:]
        commands = append(commands, cmdEntry{
            Name: spec.Name, Group: string(spec.Group),
            Use: use, Description: spec.Description,
            Risk: string(spec.Risk), DataType: spec.DataType,
            Function: spec.Function, Args: spec.Args,
        })
        groupCounts[string(spec.Group)]++
    }

    // 推断每个 group 的风险等级
    for _, spec := range nrc.Registry {
        g := string(spec.Group)
        if spec.Risk == nrc.RiskHighRiskWrite {
            groupRisks[g] = "mixed (high-risk-write)"
        } else if spec.Risk == nrc.RiskWrite && groupRisks[g] == "" {
            groupRisks[g] = "mixed (write)"
        }
    }
    for g, risk := range groupRisks {
        if risk == "" {
            groupRisks[g] = "read"
        }
    }

    data := map[string]interface{}{
        "commands": commands,
        "groups":   buildGroupSummary(groupCounts, groupRisks),
    }
    out, _ := json.MarshalIndent(data, "", "  ")
    return out
}
```

---

## Step 3：测试

### Registry 测试更新

```go
func TestRegistryHas24Entries(t *testing.T) {
    if len(Registry) != 24 {
        t.Errorf("Registry = %d, want 24", len(Registry))
    }
}

func TestSchemaListRegistered(t *testing.T) {
    spec, ok := LookupByName("schema-list")
    if !ok { t.Fatal("schema-list not found") }
    if spec.DataType != 0 { t.Errorf(...) }
    if spec.Group != GroupSchema { t.Errorf(...) }
}
```

### schema 输出格式测试

```go
func TestBuildSchemaJSON(t *testing.T) {
    data := buildSchemaJSON()
    var parsed map[string]interface{}
    if err := json.Unmarshal(data, &parsed); err != nil {
        t.Fatal(err)
    }
    commands := parsed["commands"].([]interface{})
    if len(commands) != 23 {
        t.Errorf("commands = %d, want 23 (不含 schema-list 自身)", len(commands))
    }
    groups := parsed["groups"].(map[string]interface{})
    if len(groups) != 5 {
        t.Errorf("groups = %d, want 5", len(groups))
    }
}
```

---

## 完整验收清单

### 离线验收

- [ ] Registry 24 条（gsd=2, device=7, config=12, interface=1, topology=1, schema=1）
- [ ] `init()` 唯一性检查通过（DataType=0 用 Group+Name 组合键）
- [ ] `go test ./...` 全部 9 包 PASS（含 `TestSchemaListRegistered` + `TestBuildSchemaJSON`）
- [ ] `go vet ./...` 零警告
- [ ] `inl schema list` 不带 `--target` 正常输出 23 条命令元数据
- [ ] schema JSON 含 `commands[]`（23 条）+ `groups{}`（5 个 group）+ Envelope 包装
- [ ] 每条命令含 name/group/use/description/risk/data_type/function/args 8 个字段
- [ ] `config-compile` 的 risk 为 `high-risk-write`
- [ ] `topology-scan` 的 args 含 `interface` 必填参数

### 实机验证

不需要（纯客户端命令）。

---

## 执行节奏

| Step | 内容 | 预计耗时 |
|------|------|:---:|
| 1 | Registry 新增 schema-list + GroupSchema | 10 分钟 |
| 2 | main.go 短路处理 + buildSchemaJSON | 20 分钟 |
| 3 | 测试（Registry + schema 输出） | 10 分钟 |
| 4 | SKILL.md + AGENTS.md 同步 | 5 分钟 |

**总共约 45 分钟**。

---

## 相关文档

- [inl-prd.md](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/docs/inl/inl-prd.md#L133-L134) — PRD 命令树 schema list
- [inl-architecture.md](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/docs/inl/inl-architecture.md#L72-L73) — 架构 schema 包设计
- [cli-architecture-overview](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/cli/docs/cli-architecture-overview.md#L547-L555) — lark-cli schema 命令参考
