---
title: Step 11 — Schema Fields: --data JSON 结构化字段元数据
tags: [inl, step11, schema, agent-self-discovery, field-metadata]
created: 2026-06-09
status: plan
---

# Step 11 — Schema Fields: `--data` JSON 结构化字段元数据

## 动机

当前 `inl schema list` 输出的 `args` 数组中，`--data` 参数只有一段纯文本 `Description`（如 `"Function JSON (含 RefGSD/DAP_ID)"`）。AI Agent 需要从中自然语言解析 JSON 内部字段，不可靠且易出错。

本文档描述给 `ArgumentSpec` 增加 `Fields` 字段的方案，使 `--data` JSON 的内部字段结构能通过 `inl schema list` 以结构化 JSON 输出。

## 涉及文件

| 文件 | 改动类型 | 说明 |
|------|---------|------|
| `internal/nrc/commands.go` | 新增类型 + 修改 struct + 填充 Registry | 核心改动 |
| `main_test.go` | 新增 1 个测试用例 | 验证 Fields 输出 |
| `skills/inl-shared/SKILL.md` | 文档更新 | 说明 fields 字段的消费方式 |
| `skills/inl-workflow-profinet-config/SKILL.md` | 可选更新 | 说明 AI 可用的新结构化信息 |

**零改动文件**: `main.go`、`commands_test.go`、`config_body.go`、`config_body_test.go`、所有 `internal/output/`、所有 `internal/*/types.go`。

---

## Phase 0: 定义新类型

### 0.1 FieldSpec

在 `internal/nrc/commands.go` 中，在 `ArgumentSpec` 之前新增：

```go
// FieldSpec 描述 --data JSON 内部的一个子字段。
// 仅当 ArgumentSpec.Name == "data" 时使用；DCP 参数（interface/mac/name 等）不使用。
type FieldSpec struct {
	Name        string `json:"name"`                  // 字段名，如 "RefGSD"
	Type        string `json:"type"`                  // "string" | "int" | "bool" | "object"
	Description string `json:"description"`           // 含义说明
	Required    bool   `json:"required"`              // 在 --data JSON 内是否必填
	Example     string `json:"example,omitempty"`     // 示例值（可选）
}
```

### 0.2 ArgumentSpec 加 Fields

```go
type ArgumentSpec struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Required    bool        `json:"required"`
	Fields      []FieldSpec `json:"fields,omitempty"` // 新增
}
```

`omitempty` 确保没有 `--data` 的命令（DCP 命令、读命令）不输出 `fields` 字段。

### 0.3 影响范围分析

| 位置 | 用法 | 影响 |
|------|------|------|
| `commands.go` Registry | 12 处 `Args: []ArgumentSpec{{Name:"data"}}` | 每处加 `Fields` |
| `main.go` `cmdEntry` (L480) | `Args []nrc.ArgumentSpec` | 自动继承新字段 |
| `main.go` `buildSchemaJSON` (L491) | 遍历 Registry 赋值 | **零改动**，struct 赋值自动带 Fields |
| `main.go` `buildSubCmd` (L197) | 遍历 spec.Args 注册 flag | **零改动** |
| `main_test.go` L243 | 验证 8 字段校验 | 不影响 |
| `collectDCPArgs` (main.go L242) | 从 flags 取 --data 值 | **零改动** |

---

## Phase 1: 填充 12 条 `--data` 命令的 Fields

### 1.1 config-set-driver — 模式 B（PNDriver 顶层）

```go
Args: []ArgumentSpec{{
	Name: "data", Required: true,
	Description: "PNDriver 参数 JSON（业务字段在顶层 PNDriver 对象）",
	Fields: []FieldSpec{
		{Name: "DeviceName",      Type: "string", Required: true,  Description: "PROFINET 主站设备名"},
		{Name: "IPAddress",       Type: "string", Required: true,  Description: "主站 IP 地址"},
		{Name: "SubnetMask",      Type: "string", Required: true,  Description: "子网掩码"},
		{Name: "SetInTheProject", Type: "bool",   Required: false, Description: "是否写入项目配置（默认 true）"},
	},
}},
```

### 1.2 config-add-device — 模式 A（Function 下）

```go
Args: []ArgumentSpec{{
	Name: "data", Required: true,
	Description: "添加设备的业务字段 JSON（Function 对象下）",
	Fields: []FieldSpec{
		{Name: "RefGSD",     Type: "string", Required: true,  Description: "GSDML 文件名"},
		{Name: "DAP_ID",     Type: "string", Required: true,  Description: "设备接口 ID（16 进制）",
		 Example: "0x00000010"},
		{Name: "DeviceName", Type: "string", Required: false, Description: "设备名称（不传则 C++ 端自动生成）"},
		{Name: "IPAddress",  Type: "string", Required: false, Description: "设备 IP 地址（不传则自动分配）"},
	},
}},
```

### 1.3 config-remove-device

```go
Args: []ArgumentSpec{{
	Name: "data", Required: true,
	Description: "卸载设备的索引 JSON（Function 下）",
	Fields: []FieldSpec{
		{Name: "SetPNDeviceNum", Type: "int", Required: true,
		 Description: "目标设备索引号（1-based，不是 DeviceName）"},
	},
}},
```

### 1.4 config-set-device

```go
Args: []ArgumentSpec{{
	Name: "data", Required: true,
	Description: "待校验设备的索引 JSON（Function 下）",
	Fields: []FieldSpec{
		{Name: "SetPNDeviceNum", Type: "int", Required: true,
		 Description: "目标设备索引号（1-based）"},
	},
}},
```

### 1.5 config-add-module

```go
Args: []ArgumentSpec{{
	Name: "data", Required: true,
	Description: "添加模块的索引 JSON（Function 下）",
	Fields: []FieldSpec{
		{Name: "SetPNDeviceNum", Type: "int",    Required: true, Description: "目标设备索引（1-based）"},
		{Name: "ModuleID",       Type: "string", Required: true, Description: "模块 ID（16 进制）"},
	},
}},
```

### 1.6 config-remove-module

```go
Args: []ArgumentSpec{{
	Name: "data", Required: true,
	Description: "移除模块的索引 JSON（Function 下）",
	Fields: []FieldSpec{
		{Name: "SetPNDeviceNum", Type: "int", Required: true, Description: "目标设备索引（1-based）"},
		{Name: "SetModuleSlot",  Type: "int", Required: true, Description: "模块插槽号（1-based）"},
	},
}},
```

### 1.7 config-add-submodule

```go
Args: []ArgumentSpec{{
	Name: "data", Required: true,
	Description: "添加子模块的索引 JSON（Function 下）",
	Fields: []FieldSpec{
		{Name: "SetPNDeviceNum", Type: "int",    Required: true, Description: "目标设备索引（1-based）"},
		{Name: "ModuleID",       Type: "string", Required: true, Description: "父模块 ID（16 进制）"},
		{Name: "SubmoduleID",    Type: "string", Required: true, Description: "子模块 ID（16 进制）"},
	},
}},
```

### 1.8 config-remove-submodule

```go
Args: []ArgumentSpec{{
	Name: "data", Required: true,
	Description: "移除子模块的索引 JSON（Function 下）",
	Fields: []FieldSpec{
		{Name: "SetPNDeviceNum",   Type: "int", Required: true, Description: "目标设备索引（1-based）"},
		{Name: "SetModuleSlot",    Type: "int", Required: true, Description: "父模块插槽号（1-based）"},
		{Name: "SetSubmoduleSlot", Type: "int", Required: true, Description: "子模块插槽号（1-based）"},
	},
}},
```

### 1.9 config-shield

```go
Args: []ArgumentSpec{{
	Name: "data", Required: true,
	Description: "屏蔽设备的标识 JSON（Function 下）",
	Fields: []FieldSpec{
		{Name: "DeviceName", Type: "string", Required: true,
		 Description: "要屏蔽的设备名称（不是索引号）"},
	},
}},
```

### 1.10 config-unshield

```go
Args: []ArgumentSpec{{
	Name: "data", Required: true,
	Description: "取消屏蔽的设备标识 JSON（Function 下）",
	Fields: []FieldSpec{
		{Name: "DeviceName", Type: "string", Required: true,
		 Description: "要取消屏蔽的设备名称"},
	},
}},
```

### 1.11 config-set-idevice — 模式 C（IDevice 顶层）

```go
Args: []ArgumentSpec{{
	Name: "data", Required: true,
	Description: "IDevice IO 参数 JSON（业务字段在顶层 IDevice 对象）",
	Fields: []FieldSpec{
		{Name: "Activate",     Type: "bool", Required: true, Description: "是否激活 IDevice"},
		{Name: "InputLength",  Type: "int",  Required: true, Description: "输入数据长度（字节）"},
		{Name: "OutputLength", Type: "int",  Required: true, Description: "输出数据长度（字节）"},
	},
}},
```

### 1.12 raw-send — 透传，无固定结构

```go
Args: []ArgumentSpec{{
	Name: "data", Required: true,
	Description: "原始 NRC JSON payload（透传，无字段约束）",
	// Fields 保持 nil — raw-send payload 是自由格式
}},
```

### 1.13 不设 Fields 的命令

以下命令的 Args 保持不变（nil 或 DCP 结构化 flags，均不涉及 `--data` JSON）：

- 所有 gsd/* 读命令
- 所有 device/* 读命令（list / list-active / run / gsd-config / gsd-active）
- device/setup-name / device/setup-ip（使用结构化 flags: interface/mac/name/ip/mask）
- device/setup（组合命令，不走 Registry）
- 所有 config/* 中 Args: nil 的命令（config-compile 等）
- interface/list
- topology/scan
- schema/list

---

## Phase 2: 更新测试

### 2.1 `main_test.go` 新增测试用例

在 `TestBuildSchemaJSON` 之后新增：

```go
// TestSchemaList_Fields 验证 --data 命令的 args[0].fields 输出正确。
func TestSchemaList_Fields(t *testing.T) {
	data := buildSchemaJSON()
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("JSON 解析失败: %v", err)
	}
	commands := parsed["commands"].([]any)

	// 测试 config-add-device
	var addDevice map[string]any
	for _, c := range commands {
		m := c.(map[string]any)
		if m["name"] == "config-add-device" {
			addDevice = m
			break
		}
	}
	if addDevice == nil {
		t.Fatal("找不到 config-add-device")
	}

	args := addDevice["args"].([]any)
	if len(args) != 1 {
		t.Fatalf("args 长度 = %d, want 1", len(args))
	}
	dataArg := args[0].(map[string]any)
	fields := dataArg["fields"].([]any)
	if len(fields) < 2 {
		t.Fatalf("fields 长度 = %d, 应至少含 RefGSD/DAP_ID", len(fields))
	}

	fieldNames := make(map[string]bool)
	for _, f := range fields {
		fm := f.(map[string]any)
		fieldNames[fm["name"].(string)] = true
	}
	if !fieldNames["RefGSD"] {
		t.Error("fields 应含 RefGSD")
	}
	if !fieldNames["DAP_ID"] {
		t.Error("fields 应含 DAP_ID")
	}

	// 测试 topology-scan（DCP 结构化 flags）不输出 fields
	for _, c := range commands {
		m := c.(map[string]any)
		if m["name"] == "topology-scan" {
			scanArgs := m["args"].([]any)
			if len(scanArgs) > 0 {
				firstArg := scanArgs[0].(map[string]any)
				if _, hasFields := firstArg["fields"]; hasFields {
					t.Error("topology-scan.args[0] 不应含 fields 字段")
				}
			}
		}
	}
}
```

---

## Phase 3: 更新 Skill 文档

### 3.1 `skills/inl-shared/SKILL.md`

在 §8 `inl schema list` 的字段说明表中补充：

```diff
- commands[].args[]         | 必填 DCP 参数列表         | 决定是否需追加 --interface 等 flag
+ commands[].args[]         | 参数列表                  | 决定需追加哪些 flag
+ commands[].args[].fields  | --data JSON 的子字段列表   | --data 参数需包含哪些字段、类型、必填
```

并在 schema 输出示例中添加一个带 `fields` 的命令示例（如 `config-add-device`），说明 AI 如何消费 `fields` 数组构造 `--data` JSON。

### 3.2 `skills/inl-workflow-profinet-config/SKILL.md`（可选）

在 Phase 3（方案规划）加一句说明，指示 Agent 现在可通过 `schema list` 的 `fields` 字段直接获取要构造的 JSON 字段列表。

---

## Phase 4: 回归验证

| 验证项 | 方法 | 预期 |
|--------|------|------|
| `go build ./...` | 编译 | 零编译错误 |
| `go vet ./...` | 静态分析 | 零警告 |
| `go test ./...` | 全部单元测试 | 全部通过 |
| `inl schema list`（无 --target）| 运行 | 所有 `--data` 命令的 args[0] 输出 `fields` 数组 |
| `inl config-set-driver --help` | 运行 | `--data` flag 的 help 文本不变 |
| 无 `--data` 命令的 schema | 检查输出 | DCP 命令的 args 不含 `fields`，读命令 args = null |

---

## 输出效果

### 当前（Step 11 前）

```json
{
  "name": "config-add-device",
  "args": [
    {"name": "data", "description": "Function JSON (含 RefGSD/DAP_ID)", "required": true}
  ]
}
```

### Step 11 后

```json
{
  "name": "config-add-device",
  "args": [{
    "name": "data",
    "description": "添加设备的业务字段 JSON（Function 对象下）",
    "required": true,
    "fields": [
      {"name": "RefGSD",     "type": "string", "description": "GSDML 文件名",                   "required": true,  "example": "GSDML-V2.31-OBARA-SIV31-40-20190707.xml"},
      {"name": "DAP_ID",     "type": "string", "description": "设备接口 ID（16 进制）",          "required": true,  "example": "0x00000010"},
      {"name": "DeviceName", "type": "string", "description": "设备名称（不传则 C++ 端自动生成）","required": false},
      {"name": "IPAddress",  "type": "string", "description": "设备 IP 地址（不传则自动分配）",  "required": false}
    ]
  }]
}
```

### AI 侧消费方式

AI Agent 收到 `schema list` 后，对 `--data` 命令不再需要自然语言解析，直接遍历 `fields` 数组：

```
fields = schema.commands["config-add-device"].args[0].fields

required_fields = fields.filter(f => f.required)
# → [{name:"RefGSD", type:"string"}, {name:"DAP_ID", type:"string"}]

optional_fields = fields.filter(f => !f.required)
# → [{name:"DeviceName", type:"string"}, {name:"IPAddress", type:"string"}]

# 构造 --data JSON
--data '{"RefGSD":"<用户提供的GSDML文件名>","DAP_ID":"<用户提供的接口ID>"}'
```
