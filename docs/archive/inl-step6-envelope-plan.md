---
title: inl 第 6 步开发计划 — 统一 JSON Envelope
tags: [inl, development, plan, envelope, output]
created: 2026-06-03
status: draft
---

# inl — 第 6 步：统一 JSON Envelope

## 目标

将 inl 所有命令的 stdout 输出从"裸 JSON"升级为统一的 `{ok, data, _notice}` 信封格式，使 AI Agent 能以同一套解析逻辑消费所有命令的输出。

## 背景

[inl-prd.md](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/docs/inl/inl-prd.md#L54-L66) 设计原则第一条定义：

```json
{ "ok": true, "data": { ... }, "_notice": { ... } }
```

当前 23 条命令的 stdout 输出是裸 JSON——工业 PC 通过 NRC 返回什么，inl 就 prettify 后直出什么。这导致 AI 需要针对每个 DataType（12/13/14/16）写不同的解析逻辑。

## 现状分析

### main.go 中的输出点（3 处）

| 位置 | 场景 | 当前行为 |
|------|------|---------|
| `runNrcCommand` L278-284 | 正常命令响应 | `json.Indent` → `os.Stdout.Write`（裸 JSON） |
| `runNrcCommand` L247-249 | DCP 写操作无响应 | 仅 stderr ✅ 提示，无 stdout 输出 |
| `runNrcCommand` L154-157 | `--dry-run` 模式 | `PrintDryRunFrame` 输出 DryRunFrame JSON（已有结构） |

### 问题

```bash
# 当前: AI 需要理解 {DataType:14, Devices:[{Mac:...}]} 的结构
$ inl topology scan --target 192.168.3.15 --interface pnio1
{"DataType":14,"Devices":[{"Mac":"aa:bb:cc:dd:ee:ff",...}]}

# 期望: AI 可以统一通过 ok + data 路径消费
$ inl topology scan --target 192.168.3.15 --interface pnio1
{
  "ok": true,
  "data": {"DataType":14,"Devices":[{"Mac":"aa:bb:cc:dd:ee:ff",...}]},
  "_notice": {"command":"topology-scan","device_count":3,"elapsed_ms":234}
}
```

---

## 设计

### Envelope 结构

```go
// Envelope 是 inl 所有命令 stdout 输出的统一 JSON 信封。
// 借鉴 lark-cli internal/output/envelope.go 的设计。
//
// JSON 结构:
//
//	{
//	  "ok": true,
//	  "identity": "inl",
//	  "data": { ... 工业 PC 原始响应 JSON ... },
//	  "_notice": { "command": "gsd-list", "elapsed_ms": 123, ... }
//	}
type Envelope struct {
    OK       bool                   `json:"ok"`
    Identity string                 `json:"identity,omitempty"`
    Data     json.RawMessage        `json:"data,omitempty"`
    Error    *Error                 `json:"error,omitempty"`
    Notice   map[string]interface{} `json:"_notice,omitempty"`
}
```

### 关键设计决策

| 决策 | 原因 |
|------|------|
| `Data` 用 `json.RawMessage` | 保持原样透传，不做二次序列化——避免数字精度丢失、字段重排 |
| `Error` 复用现有 `*output.Error` | 已定义且已实现 `Error()` 接口，AI 解析逻辑无需改变 |
| `dry-run` 不包 Envelope | `DryRunFrame` 本身已是结构化数据，供 AI 校验 payload 用 |
| 错误 **不走** stdout Envelope | 沿用现有 `writeError` → stderr 逻辑，与 lark-cli 约定一致 |

### WriteSuccess 函数

```go
// WriteSuccess 将 data 包装在 Envelope 中，写入 w（通常为 stdout）。
// notice 是可选的诊断信息（命令名、耗时、设备数量等）。
func WriteSuccess(w io.Writer, data []byte, notice map[string]interface{}) error {
    env := Envelope{
        OK:       true,
        Identity: "inl",
        Data:     data,
        Notice:   notice,
    }
    enc := json.NewEncoder(w)
    enc.SetIndent("", "  ")
    return enc.Encode(env)
}
```

---

## 文件清单

```
inl/
├── internal/output/
│   ├── envelope.go        ← 新增: Envelope + WriteSuccess
│   └── envelope_test.go   ← 新增: 3 个 round-trip 测试
├── main.go                ← 改: 3 处输出点接入 Envelope
├── skills/inl-shared/
│   └── SKILL.md           ← 改: stdout 格式说明更新
└── AGENTS.md              ← 改: 输出约定章节更新
```

**新增文件**: 2（envelope.go + envelope_test.go）
**修改文件**: 3（main.go + SKILL.md + AGENTS.md）
**新增依赖**: 0

---

## Step 1：`internal/output/envelope.go`

### 文件

```go
package output

import (
    "encoding/json"
    "io"
)

// Envelope 是 inl 所有命令 stdout 输出的统一 JSON 信封。
//
// 借鉴 lark-cli internal/output/envelope.go:7-14 的设计:
//   - stdout = Envelope (AI 数据消费)
//   - stderr = 进度 / 警告 / 结构错误
//
// JSON 结构:
//   {
//     "ok": true,
//     "identity": "inl",
//     "data": { ... 命令的原始响应 JSON ... },
//     "_notice": { "command": "gsd-list", "elapsed_ms": 123 }
//   }
type Envelope struct {
    OK       bool                   `json:"ok"`
    Identity string                 `json:"identity,omitempty"`
    Data     json.RawMessage        `json:"data,omitempty"`
    Error    *Error                 `json:"error,omitempty"`
    Notice   map[string]interface{} `json:"_notice,omitempty"`
}

// WriteSuccess 将原始响应 data 包装在 Envelope 中写入 w。
// notice 是可选诊断信息 (命令名/耗时等), 为 nil 时省略 _notice 字段。
func WriteSuccess(w io.Writer, data []byte, notice map[string]interface{}) error {
    env := Envelope{
        OK:       true,
        Identity: "inl",
        Data:     data,
        Notice:   notice,
    }
    enc := json.NewEncoder(w)
    enc.SetIndent("", "  ")
    return enc.Encode(env)
}
```

---

## Step 2：`internal/output/envelope_test.go`

```go
func TestWriteSuccess_Basic(t *testing.T) {
    var buf bytes.Buffer
    if err := WriteSuccess(&buf, []byte(`{"DataType":13}`), nil); err != nil {
        t.Fatal(err)
    }

    var env Envelope
    if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
        t.Fatal(err)
    }
    if !env.OK { t.Error("OK should be true") }
    if env.Identity != "inl" { t.Errorf("Identity = %q", env.Identity) }
    if string(env.Data) != `{"DataType":13}` { t.Error("Data mismatch") }
    if env.Notice != nil { t.Error("Notice should be nil") }
}

func TestWriteSuccess_WithNotice(t *testing.T) {
    var buf bytes.Buffer
    notice := map[string]interface{}{
        "command": "gsd-list",
        "elapsed_ms": 234,
    }
    WriteSuccess(&buf, []byte(`{}`), notice)

    var env Envelope
    json.Unmarshal(buf.Bytes(), &env)
    if env.Notice["command"] != "gsd-list" { t.Error("Notice.command mismatch") }
    if env.Notice["elapsed_ms"] != float64(234) { t.Error("Notice.elapsed_ms mismatch") }
}

func TestEnvelope_RoundTrip(t *testing.T) {
    data := []byte(`{"DataType":14,"Devices":[{"Mac":"00:11:22:33:44:55"}]}`)
    var buf bytes.Buffer
    WriteSuccess(&buf, data, nil)

    var env Envelope
    json.Unmarshal(buf.Bytes(), &env)
    if !env.OK { t.Error("OK should be true") }
    if !bytes.Equal(env.Data, data) { t.Errorf("Data round-trip mismatch:\n  got:  %s\n  want: %s", env.Data, data) }
}
```

---

## Step 3：`main.go` 接入

### 3.1 正常命令响应（L278-284）

```go
// 旧:
var pretty bytes.Buffer
if err := json.Indent(&pretty, data, "", "  "); err != nil {
    os.Stdout.Write(data)
    return nil
}
pretty.WriteByte('\n')
os.Stdout.Write(pretty.Bytes())

// 新:
notice := map[string]interface{}{
    "command": spec.Name,
    "data_type": spec.DataType,
    "elapsed_ms": elapsed.Milliseconds(),
}
if err := output.WriteSuccess(cmd.OutOrStdout(), data, notice); err != nil {
    return fmt.Errorf("信封构造失败: %w", err)
}
```

### 3.2 DCP 写操作无响应（L247-249）

```go
// 旧:
fmt.Fprintln(os.Stderr, "✅ DCP 操作已发送 (无 JSON 响应, 请用 topology scan 验证)")

// 新:
notice := map[string]interface{}{
    "command": spec.Name,
    "dcp_write": true,
    "verify_with": "topology-scan",
}
output.WriteSuccess(cmd.OutOrStdout(), []byte("null"), notice)
fmt.Fprintln(cmd.ErrOrStderr(), "✅ DCP 操作已发送 (无 JSON 响应, 请用 topology scan 验证)")
```

### 3.3 dry-run（不变）

DryRunFrame 已有专用输出函数，不包 Envelope。

---

## Step 4：`skills/inl-shared/SKILL.md` 更新

将输出约定章节从：

```markdown
| **stdout** | 数据 / 命令结果（prettified JSON） | `{"DataType":13, "Device":[...]}` |
```

改为：

```markdown
| **stdout** | `{ok, data, _notice}` Envelope 格式 | `{"ok":true,"data":{"DataType":13,...},"_notice":{"command":"gsd-list"}}` |
```

新增 Envelope 字段说明：

```markdown
### stdout Envelope

所有命令的 stdout 输出包裹在统一 JSON 信封中:

| 字段 | 类型 | 说明 |
|------|------|------|
| `ok` | bool | `true` = 成功 |
| `identity` | string | 固定为 `"inl"` |
| `data` | object | 命令的原始响应 JSON（工业 PC 返回内容） |
| `_notice` | object | 诊断信息：`command`, `data_type`, `elapsed_ms` |

AI 判断成功: `if response.ok { process(response.data) }`
```

---

## Step 5：`AGENTS.md` 更新

在 "输出约定（stdout/stderr 分流）" 章节增加 Envelope 说明，替换原有的"prettified JSON"描述。

---

## 完整验收清单

### 离线验收

- [ ] `internal/output/envelope.go` 存在，含 `Envelope` struct + `WriteSuccess` 函数
- [ ] `internal/output/envelope_test.go` 含 3 个测试：基础、带 notice、round-trip
- [ ] `go test ./internal/output/` 全部 PASS
- [ ] `go test ./...` 全部 9 包 PASS
- [ ] `go vet ./...` 零警告
- [ ] `inl gsd list --target 192.168.3.15` stdout 输出 Envelope 格式（无网络时构建测试）
- [ ] Envelope JSON 可被 `json.Unmarshal` 还原，`data` 字段内容与原始响应一致

### 实机验证

- [ ] `inl gsd list --target 192.168.3.15` 输出 Envelope 包裹的 JSON
- [ ] `inl topology scan --target 192.168.3.15 --interface pnio1` 输出 Envelope
- [ ] `inl device setup-name ... --yes` DCP 写操作 `data: null` + `_notice.dcp_write: true`
- [ ] 现有 `> out.json 2> err.log` 分流验证仍正常

---

## 执行节奏

| Step | 内容 | 预计耗时 |
|------|------|:---:|
| 1 | `envelope.go` + `envelope_test.go` | 15 分钟 |
| 2 | `main.go` 3 处接入 + 耗时统计 | 15 分钟 |
| 3 | SKILL.md + AGENTS.md 更新 | 10 分钟 |
| 4 | 实机验证 | 5 分钟 |

**总共约 45 分钟**。

---

## 相关文档

- [inl-prd.md](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/docs/inl/inl-prd.md#L54-L66) — 产品设计原则（Agent-Native 输出）
- [inl-architecture.md](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/docs/inl/inl-architecture.md#L256-L276) — 输出系统设计
- [cli-module-client-output](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/cli/docs/cli-module-client-output.md#L504-L537) — lark-cli Envelope 参考源
- [inl/skills/inl-shared/SKILL.md](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/inl/skills/inl-shared/SKILL.md) — 输出约定章节
