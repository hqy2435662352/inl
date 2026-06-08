---
title: inl 第 8 步开发计划 — raw send + --format table
tags: [inl, development, plan, raw, table, output]
created: 2026-06-04
status: draft
---

# inl — 第 8 步：raw send + --format table

## 目标

1. **`raw send`** — 透传任意 JSON 帧到工业 PC，覆盖 24 条 Registry 外的边缘场景，补齐 PRD 命令树最后一块拼图
2. **`--format table`** — 表格输出，让 FAE 现场能肉眼读懂 `topology scan` / `gsd list` 等读命令的结果

---

## 一、`raw send` — 透传原始 JSON 帧

### 命令行形态

```bash
# 透传任意 DataType + Function JSON
$ inl raw send --data '{"DataType":14,"Function":1,"Portname":"enp4s0"}'

# 等价于
$ inl topology scan --interface enp4s0
```

### 设计

| 维度 | 设计 |
|------|------|
| Group | `raw`（新 Group） |
| 风险 | `write`（无法预判 payload 内容，默认要求 `--yes`） |
| 参数 | `--data` string（必填，完整 JSON payload） |
| NRC | 正常走 TCP:6000 连接 → 发送 → 接收 → Envelope 输出 |
| BodyBuilder | 无——用户/AI 提供的 JSON 直接作为帧 payload |

### Registry 条目

```go
{
    Name:        "raw-send",
    Code:        0x9275,
    DataType:    0,            // 哨兵: 透传, 不走 DefaultBodyBuilder
    Direction:   DirectionRequest,
    Description: "透传任意 JSON 帧到工业 PC (兜底覆盖非标准 NRC 命令)",
    Risk:        RiskWrite,    // 保守策略: 默认 write, 要求 --yes
    Group:       GroupRaw,
    Args:        []ArgumentSpec{{Name: "data", Description: "完整 JSON payload", Required: true}},
    BodyBuilder: rawSendBody,  // 直接返回用户提供的 --data 字符串
},
```

### `--data` 参数处理

```go
func rawSendBody(spec CommandSpec, args map[string]string) (string, error) {
    data := args["data"]
    if data == "" {
        return "", fmt.Errorf("--data 不能为空")
    }
    // 基础校验: 确保是合法 JSON
    if !json.Valid([]byte(data)) {
        return "", fmt.Errorf("--data 不是合法 JSON")
    }
    return data, nil
}
```

### main.go 新增

```go
case nrc.GroupRaw:
    use = "raw"
    short = "透传原始 JSON 帧"
    long = "直接发送任意 JSON payload 到工业 PC (兜底)。"

// --- 注册 --data flag ---
case "raw-send":
    subCmd.Flags().String("data", "", "完整 JSON payload (必填)")
    subCmd.MarkFlagRequired("data")
```

### 安全考虑

| 规则 | 说明 |
|------|------|
| 默认 `write` | AI 必须加 `--yes` 才能发送，防止未确认就执行高危操作 |
| 不校验 DataType/Function | 透传——这是兜底命令，Registry 已覆盖所有已知命令 |
| `--dry-run` 不支持 | DryRunFrame 基于 `spec.DataType+Function` 构建，raw send 没有这些元数据 |

---

## 二、`--format table` — 表格输出

### 命令行形态

```bash
# AI 消费 (默认)
$ inl topology scan --interface pnio1 --target 192.168.3.15
{"ok":true,"data":{"DataType":14,"Devices":[{"Mac":"00:11:...","DeviceName":"heron-weld",...}]}}

# FAE 现场 (--format table)
$ inl topology scan --interface pnio1 --target 192.168.3.15 --format table
🔌 连接 192.168.3.15:6000 ...
  ✅ 已连接
📤 发送 topology-scan (DataType=14)

MAC                DeviceName    IPAddress       VendorID  DeviceRole
00:11:22:33:44:55  heron-weld   192.168.2.10    0x038A    PN设备
aa:bb:cc:dd:ee:ff  smc-valve   192.168.2.20    0x0083    PN设备
11:22:33:44:55:66  scalance     192.168.2.30    0x002A    PN控制器
```

### 设计决策

| 决策 | 原因 |
|------|------|
| table 输出到 **stdout**（不走 Envelope） | table 是人类看的，不是 AI 消费的。Envelope + table 混在一起会破坏 pipe 链 |
| 自动探测响应中的数组字段 | 不同 DataType 的字段名不同（`Device` / `Devices` / `DecentralDevice` / `Function.Devices`），需要统一探测算法 |
| 字段截断 + 对齐 | 长字符串（如 GSDName "GSDML-V2.35-Siemens-002A-S7-1500-20170801.xml"）截断到 40 字符 + `...` |
| stderr 不变 | 进度/警告仍走 stderr，与 JSON 输出一致 |

### 探测算法：`ExtractTableRows`

```go
// ExtractTableRows 从响应 JSON 中提取表格行。
// 算法: 递归搜索第一个非空数组，提取其元素的公共字段作为列，返回列名 + 行数据。
//
// 示例:
//   DataType=13 → Device[]             → 列: VendorID, VendorName, DeviceID, ...
//   DataType=14 → Devices[]            → 列: Mac, DeviceName, IPAddress, ...
//   DataType=12 → DecentralDevice[]    → 列: DeviceName, IPAddress, ...
//   DataType=17 → Function.Devices[]   → 列: DeviceName, Status
func ExtractTableRows(data []byte) (columns []string, rows [][]string, err error)
```

**探测逻辑**：

```
1. json.Unmarshal → map[string]interface{}
2. 递归搜索第一个非空数组:
   for each key, value in map:
       if value is []interface{} and len > 0 && first element is map:
           return extractColumns(value)
       if value is map:
           recurse into value
3. extractColumns:
   - 取第一个元素的所有 string/int/float/bool 字段作为列名
   - 对所有元素逐字段 fmt.Sprintf → string
   - 返回 columns + rows
4. 如果找不到任何数组 → 返回 nil (该命令不适合 table 格式)
```

### 渲染算法：`FormatTable`

```go
// FormatTable 将 columns + rows 渲染为对齐的固定宽度表格。
// 每列宽度 = max(列名长度, 该列最长值长度)，最小 8，最大 40。
func FormatTable(w io.Writer, columns []string, rows [][]string) error
```

**渲染示例**：

```
MAC                DeviceName    IPAddress       VendorID  DeviceRole
00:11:22:33:44:55  heron-weld   192.168.2.10    0x038A    PN设备
aa:bb:cc:dd:ee:ff  smc-valve   192.168.2.20    0x0083    PN设备
11:22:33:44:55:66  scalance     192.168.2.30    0x002A    PN控制器
```

### 单字段回退

如果响应中没有数组（如 `device list` 的 CallBackJson 只有 PNDriver + IDevice），直接 prettified JSON 输出 + stderr 警告 `⚠️ --format table 不适用此命令，回退 JSON`。

### main.go 接入

在 `runNrcCommand` 的 Envelope 输出前插入：

```go
if formatFlag == "table" {
    cols, rows, err := output.ExtractTableRows(data)
    if err != nil || cols == nil {
        fmt.Fprintln(cmd.ErrOrStderr(), "⚠️ --format table 不适用此命令, 回退 JSON")
        // 走原有 Envelope 输出
    } else {
        output.FormatTable(cmd.OutOrStdout(), cols, rows)
        return nil  // 不走 Envelope
    }
}
```

---

## 文件清单

```
inl/
├── internal/nrc/
│   └── commands.go            ← 改: GroupRaw + raw-send 条目
│   └── commands_test.go       ← 改: Registry 25 条检查 + raw-send 测试
├── internal/output/
│   ├── table.go               ← 新增: ExtractTableRows + FormatTable
│   └── table_test.go          ← 新增: 4 个测试 (Device[] / Devices[] / Function.Devices[] / 无数组回退)
├── main.go                    ← 改: raw-send 注册 + --format table 分支
└── AGENTS.md                  ← 改: Registry 25 条 + 新增 raw / --format table 说明
```

**新增文件**: 2（table.go + table_test.go）
**修改文件**: 4（commands.go + commands_test.go + main.go + AGENTS.md）
**新增依赖**: 0

---

## Step 1：Registry 新增 raw-send

### GroupRaw

```go
const GroupRaw CommandGroup = "raw"
```

### Registry 条目

```go
{
    Name:        "raw-send",
    Code:        0x9275,
    DataType:    0,
    Direction:   DirectionRequest,
    Description: "透传任意 JSON 帧到工业 PC",
    Risk:        RiskWrite,
    Group:       GroupRaw,
    Args:        []ArgumentSpec{{Name: "data", Description: "完整 JSON payload", Required: true}},
    BodyBuilder: rawSendBody,
},
```

> DataType=0 哨兵（纯客户端）已由 Step 7 的 `init()` 唯一性检查处理——`Function=="" && DataType==0` 跳过冲突检查。

Registry: 25 条 (gsd=2, device=7, config=12, interface=1, topology=1, schema=1, raw=1)

---

## Step 2：raw-send BodyBuilder + main.go 注册

### rawSendBody

```go
func rawSendBody(spec CommandSpec, args map[string]string) (string, error) {
    data := args["data"]
    if data == "" {
        return "", fmt.Errorf("--data 不能为空")
    }
    if !json.Valid([]byte(data)) {
        return "", fmt.Errorf("--data 不是合法 JSON")
    }
    return data, nil
}
```

### main.go buildGroupCmd

```go
case nrc.GroupRaw:
    use = "raw"
    short = "透传原始 JSON 帧"
    long = "直接发送任意 JSON payload 到工业 PC (兜底, 覆盖非标准命令)。"
```

### main.go buildSubCmd — raw-send 特殊 flag

```go
if spec.Name == "raw-send" {
    subCmd.Flags().String("data", "", "完整 JSON payload (必填)")
    subCmd.MarkFlagRequired("data")
}
```

---

## Step 3：ExtractTableRows + FormatTable

### `internal/output/table.go`

```go
package output

// ExtractTableRows 从响应 JSON 中递归搜索第一个非空对象数组，
// 提取公共字段作为列名，返回 columns + rows。
// 找不到数组时返回 nil columns (由调用方回退 JSON 输出)。
func ExtractTableRows(data []byte) (columns []string, rows [][]string, err error)

// FormatTable 将对齐的固定宽度表格渲染到 w。
// columnWidth: max(header, max(value)), min=8, max=40。
// rows 中 nil/empty 值输出 "-"。
func FormatTable(w io.Writer, columns []string, rows [][]string) error
```

### 测试用例 (table_test.go)

```go
// DataType=13 → Device[] 数组
func TestExtractTableRows_DeviceArray(t *testing.T) { ... }

// DataType=14 → Devices[] 数组
func TestExtractTableRows_DevicesArray(t *testing.T) { ... }

// DataType=17/GetActRun → Function.Devices[] 嵌套数组
func TestExtractTableRows_NestedDevices(t *testing.T) { ... }

// CallBackJson → 无数组
func TestExtractTableRows_NoArray(t *testing.T) { ... }

// FormatTable 渲染验证
func TestFormatTable(t *testing.T) { ... }
```

---

## Step 4：main.go `--format table` 接入

在 `runNrcCommand` 的 Envelope 输出前（当前 L250 附近）插入：

```go
if formatFlag == "table" {
    cols, rows, err := output.ExtractTableRows(data)
    if err != nil || cols == nil {
        fmt.Fprintln(cmd.ErrOrStderr(), "⚠️ --format table 不适用此命令, 回退 JSON")
        output.WriteSuccess(cmd.OutOrStdout(), data, notice)
        return nil
    }
    output.FormatTable(cmd.OutOrStdout(), cols, rows)
    return nil
}
```

---

## 完整验收清单

### 离线验收

- [ ] Registry 25 条 (raw=1)
- [ ] `inl raw send --help` 显示 `--data` 必填
- [ ] `rawSendBody` 拒绝非 JSON `--data`
- [ ] `rawSendBody` 透传有效 JSON
- [ ] `ExtractTableRows` 4 种场景全通过
- [ ] `FormatTable` 列宽对齐正确，长字段截断
- [ ] `inl gsd list --format table` 离线测试（mock 数据）
- [ ] `go test ./...` 全部 PASS
- [ ] `go vet ./...` 零警告

### 实机验证

- [ ] `inl raw send --data '{"DataType":13}' --yes` → 返回 GSD 列表 (等价于 `gsd list`)
- [ ] `inl topology scan --interface pnio1 --target 192.168.3.15 --format table` → 设备表格
- [ ] `inl gsd list --target 192.168.3.15 --format table` → GSD 表格
- [ ] `inl device run --target 192.168.3.15 --format table` → 焊机状态表格
- [ ] `inl device list --target 192.168.3.15 --format table` → ⚠️ 回退 JSON (无数组)

---

## 执行节奏

| Step | 内容 | 预计耗时 |
|------|------|:---:|
| 1 | Registry raw-send + GroupRaw + rawSendBody | 15 分钟 |
| 2 | main.go raw-send 注册 | 10 分钟 |
| 3 | table.go + table_test.go | 30 分钟 |
| 4 | main.go --format table 分支 | 10 分钟 |
| 5 | 实机验证 | 10 分钟 |

**总共约 1 小时 15 分钟**。

---

## 相关文档

- [inl-prd.md](inl-prd.md#L102-L155) — PRD 命令树 (raw 组)
- [inl-architecture.md](inl-architecture.md#L253-L260) — 输出系统设计 (§5)
- [cli-module-client-output](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/cli/docs/cli-module-client-output.md#L504-L537) — lark-cli table 输出参考
