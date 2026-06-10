---
title: inl MVP 最小试验单元开发计划
tags: [inl, mvp, development, plan]
created: 2026-05-27
aliases: [inl-mvp, inl-试验单元]
---

# inl — 最小试验单元开发计划

## 目标

```bash
$ go run main.go --target 192.168.1.100
🔌 连接 192.168.1.100:6000 ... OK
📤 发送 DataType=13 (GSD 文件列表请求)
📥 收到响应:
  - Vendor: OBARA (0x038A)
    Device: SIV31-40 (0x0030)
    GSD: GSDML-V2.31-OBARA-SIV31-40-20190707.xml
    Modules: 12, DAP: 1
  - Vendor: Siemens (0x002A)
    Device: ET200SP (0x0050)
    ...
✅ 共发现 5 个设备驱动
```

**两个核心能力**：
1. 与工业 PC 建立 TCP 通信，正常收发 NRC Socket 帧
2. 实现 `DataType=13`（GSD 文件列表回调）的发送与响应解析

## 背景上下文（自包含，无需外部资料）

### 系统拓扑

```
┌── 上位机：调试 PC ──────────────┐      ┌── 下位机：工业 PC ────────────┐
│  inl CLI (Go, 本项目)           │ TCP  │  nrc2.out (io-controller)       │
│     ↓                           │:6000 │     ↓                           │
│  NRC 帧 → TCP →                │ ────►│  switchsendmapvarvalue(json)    │
│                                │      │     ↓ DataType=13                │
│  响应 ← TCP ←                  │ ◄────│  CallBackGsdFileList()           │
│                                │      │     → 扫描 ./communication/      │
│  解析 JSON → 打印              │      │       Profinet/*.xml             │
└─────────────────────────────────┘      │     → 解析 GSDML                │
                                         │     → 返回 JSON 设备列表         │
                                         └─────────────────────────────────┘
```

### NRC Socket 协议帧格式

```
┌──────────┬──────────┬──────────┬───────────────┬──────────┐
│ SyncByte │  Length  │ Command  │  Data (JSON)  │   CRC32  │
│  2 Byte  │  2 Byte  │  2 Byte  │   Length−2字节  │  4 Byte  │
│  0x4E66  │  BigEnd  │  BigEnd  │    UTF-8       │  BigEnd  │
└──────────┴──────────┴──────────┴───────────────┴──────────┘
```

| 字段 | 长度 | 字节序 | 说明 |
|------|------|--------|------|
| SyncByte | 2 | Big Endian | 固定值 `0x4E66` |
| Length | 2 | Big Endian | `Command(2) + Data(N)` 的总长度，不含 SyncByte 和 CRC。范围 0~2000 |
| Command | 2 | Big Endian | 协议号。发送用 `0x9275`，响应用 `0x9271` |
| Data | N | — | JSON 字符串（UTF-8，可选末尾 `\n`） |
| CRC32 | 4 | Big Endian | IEEE 802.3 CRC32，计算范围：Length 字段之后的所有字节（即 Command + Data） |

### CRC32 具体说明

- 多项式：IEEE 802.3 标准（Go 的 `crc32.IEEE`）
- 计算范围：`[Length字段之后的所有字节]`，即 `Command(2B) + Data(N字节)`，**不含** SyncByte 和 Length
- 验证用已知正确帧（来自纳博特 PDF Page 4）：

```
无换行版本：
4E66 0016 2001 7B22726F626F74223A312C22737461747573223A307D 53DD EB72

有换行版本：
4E66 0017 2001 7B22726F626F74223A312C22737461747573223A307D0A 6B92 6DFF
```

对应的 JSON：`{"robot":1,"status":0}`（有换行时末尾加 `\n`）

**注意**：换行符 `0x0A` 可选。本 MVP 发送和接收均不加换行。

### TCP 连接参数

| 参数 | 值 |
|------|-----|
| 传输层 | TCP |
| 端口 | 6000 |
| 连接超时 | 5 秒 |
| 读写超时 | 10 秒 |
| 心跳 | 本 MVP 不需要 |

### 本 MVP 使用的协议号

| 方向 | 协议号 | 说明 |
|------|--------|------|
| 发送 | `0x9275` | 自定义多功能接口（`NRC_SetSocketCustomProtocalCB` 注册） |
| 接收 | `0x9271` | 通用查询响应 |

### 发送的 JSON

```json
{"DataType":13}
```

### 期望收到的响应 JSON 结构

```json
{
  "DataType": 13,
  "Device": [
    {
      "VendorID": "0x038A",
      "VendorName": "OBARA",
      "DeviceID": "0x0030",
      "MainFamily": "General",
      "ProductFamily": "Welding Controller",
      "GSDName": "GSDML-V2.31-OBARA-SIV31-40-20190707.xml",
      "DAP": [ ... ],
      "Module": [ ... ],
      "UseableModules": [ ... ],
      "ReductionRatio": ["8ms", "16ms", "32ms"]
    }
  ]
}
```

---

## 文件清单（共 4 个文件）

```
inl/
├── go.mod                     # 仅依赖标准库，无第三方依赖
├── main.go                    # 入口：连接 → 发送 DataType=13 → 解析 → 打印
└── internal/
    └── nrc/
        ├── frame.go           # 帧编解码 + CRC32
        └── client.go          # TCP 连接 + 收发
```

## Step 1：初始化 Go 模块

```bash
mkdir inl && cd inl
go mod init github.com/your-org/inl
```

**验收标准**：
- [ ] `go.mod` 文件生成，module 路径正确
- [ ] `go build` 不报错

---

## Step 2：`internal/nrc/frame.go` — 帧编解码

### 文件位置

`inl/internal/nrc/frame.go`

### 公开函数签名

```go
package nrc

const FrameSync = 0x4E66

// BuildFrame 构建 NRC 帧
func BuildFrame(command uint16, payload string) []byte

// ReadFrame 从 TCP 连接中读取一帧
func ReadFrame(conn net.Conn) (command uint16, data []byte, err error)
```

### BuildFrame 实现规格

1. 计算长度：`dataLen = 2 (command自身) + len(payload)`
2. 按 Big Endian 写入 SyncByte → Length → Command → Payload
3. 计算 CRC32（范围：Command + Payload，不含 SyncByte 和 Length）
4. 按 Big Endian 追加 CRC32

伪代码：

```go
func BuildFrame(command uint16, payload string) []byte {
    buf := new(bytes.Buffer)

    // SyncByte
    binary.Write(buf, binary.BigEndian, uint16(0x4E66))

    // Length = Command(2B) + len(payload)
    dataLen := 2 + len(payload)
    binary.Write(buf, binary.BigEndian, uint16(dataLen))

    // Command
    binary.Write(buf, binary.BigEndian, command)

    // Payload
    buf.WriteString(payload)

    // 拿到 Command+Payload 部分计算 CRC
    cmdPayload := buf.Bytes()[6:] // 跳过 SyncByte(2) + Length(2)
    crc := crc32.ChecksumIEEE(cmdPayload)

    // 追加 CRC32
    binary.Write(buf, binary.BigEndian, crc)

    return buf.Bytes()
}
```

### ReadFrame 实现规格

1. 用 `io.ReadFull` 读 2 字节 SyncByte → 必须等于 `0x4E66`
2. 用 `io.ReadFull` 读 2 字节 Length → 转换 Big Endian → uint16
3. 用 `io.ReadFull` 读 `Length` 字节（= Command + Data）
4. 用 `io.ReadFull` 读 4 字节 CRC32
5. 计算 CRC32 校验：对步骤 3 读到的 `Length` 字节做 `crc32.ChecksumIEEE`，必须等于步骤 4 读到的值
6. 步骤 3 的前 2 字节 = Command（Big Endian → uint16），剩余 = Data

伪代码：

```go
func ReadFrame(conn net.Conn) (uint16, []byte, error) {
    // 读 SyncByte
    var sync uint16
    io.ReadFull(conn, binary.BigEndian, &sync)
    if sync != 0x4E66 {
        return 0, nil, fmt.Errorf("invalid sync byte: 0x%04X", sync)
    }

    // 读 Length
    var dataLen uint16
    io.ReadFull(conn, binary.BigEndian, &dataLen)

    // 读 Command + Data
    buf := make([]byte, dataLen)
    io.ReadFull(conn, buf)

    // 读 CRC32
    var crc uint32
    io.ReadFull(conn, binary.BigEndian, &crc)

    // 验证 CRC
    if crc32.ChecksumIEEE(buf) != crc {
        return 0, nil, fmt.Errorf("CRC mismatch")
    }

    // 拆 Command + Data
    cmd := binary.BigEndian.Uint16(buf[0:2])
    data := buf[2:]

    return cmd, data, nil
}
```

### 单元测试：`internal/nrc/frame_test.go`

```go
package nrc

import (
    "net"
    "testing"
)

func TestBuildFrame(t *testing.T) {
    // 用 PDF 已知正确帧验证
    frame := BuildFrame(0x2001, `{"robot":1,"status":0}`)

    // frame[0:2] == [0x4E, 0x66]
    if frame[0] != 0x4E || frame[1] != 0x66 {
        t.Errorf("sync byte mismatch: got [%02X %02X]", frame[0], frame[1])
    }

    // Length 应等于 2 + len(payload)
    expectedLen := uint16(2 + len(`{"robot":1,"status":0}`))
    actualLen := binary.BigEndian.Uint16(frame[2:4])
    if actualLen != expectedLen {
        t.Errorf("length mismatch: expected %d, got %d", expectedLen, actualLen)
    }

    // CRC32 必须匹配已知正确帧
    // 已知: 无换行版本 CRC = 0x53DD_EB72
    expectedCRC := uint32(0x53DDEB72)
    crcPos := 6 + int(actualLen) // Sync(2) + Len(2) + data
    actualCRC := binary.BigEndian.Uint32(frame[crcPos : crcPos+4])
    if actualCRC != expectedCRC {
        t.Errorf("CRC mismatch: expected 0x%08X, got 0x%08X", expectedCRC, actualCRC)
    }
}

func TestRoundTrip(t *testing.T) {
    payload := `{"DataType":13}`
    frame := BuildFrame(0x9275, payload)

    // 用 pipe 模拟 TCP 连接
    r, w := net.Pipe()
    go func() {
        w.Write(frame)
        w.Close()
    }()

    cmd, data, err := ReadFrame(r)
    if err != nil {
        t.Fatalf("ReadFrame error: %v", err)
    }
    if cmd != 0x9275 {
        t.Errorf("command mismatch: expected 0x9275, got 0x%04X", cmd)
    }
    if string(data) != payload {
        t.Errorf("payload mismatch: expected %q, got %q", payload, string(data))
    }
}

func TestCRCFromPDF(t *testing.T) {
    // PDF 已知正确帧（有换行版本）
    // 4E66 0017 2001 {"robot":1,"status":0}\n 6B926DFF
    payload := `{"robot":1,"status":0}` + "\n"
    frame := BuildFrame(0x2001, payload)

    // 取最后 4 字节 CRC
    crc := binary.BigEndian.Uint32(frame[len(frame)-4:])
    expected := uint32(0x6B926DFF)

    if crc != expected {
        t.Errorf("CRC (with newline) mismatch: expected 0x%08X, got 0x%08X", expected, crc)
    }
}
```

### Step 2 验收标准

- [ ] `go test ./internal/nrc/` 全部通过
- [ ] `TestBuildFrame` 与 PDF 已知正确帧的 CRC32 一致（无换行版本 `0x53DDEB72`）
- [ ] `TestCRCFromPDF` 与 PDF 已知正确帧的 CRC32 一致（有换行版本 `0x6B926DFF`）
- [ ] `TestRoundTrip` BuildFrame → ReadFrame 能完整还原

---

## Step 3：`internal/nrc/client.go` — TCP 客户端

### 文件位置

`inl/internal/nrc/client.go`

### 公开函数签名

```go
package nrc

type Client struct {
    conn net.Conn
    addr string
}

// NewClient 创建客户端，addr 格式 "192.168.1.100:6000"
func NewClient(addr string) *Client

// Connect 建立 TCP 连接（超时 5s）
func (c *Client) Connect() error

// Close 关闭连接
func (c *Client) Close() error

// SendReceive 发送一帧并阻塞等待一帧响应（读写超时 10s）
func (c *Client) SendReceive(sendCmd uint16, payload string) (respCmd uint16, respData []byte, err error)
```

### SendReceive 实现规格

1. 调用 `BuildFrame(sendCmd, payload)` 构建帧
2. `conn.SetWriteDeadline(time.Now().Add(10 * time.Second))`
3. `conn.Write(frame)` → 检查写入字节数是否正确
4. `conn.SetReadDeadline(time.Now().Add(10 * time.Second))`
5. 调用 `ReadFrame(conn)` 等待响应

错误处理：
- 连接时 TCP 被拒 → `net.DialTimeout` 返回 error，包装为 "工业PC不可达"
- 写入失败 → 包装为 "发送失败"
- 读取超时 → 包装为 "响应超时（GSD 扫描可能需要时间）"
- 帧校验失败 → 包装为 "帧校验失败"

### Step 3 验收标准

- [ ] `go build ./...` 无编译错误
- [ ] `go vet ./...` 无警告

---

## Step 4：`main.go` — 入口

### 文件位置

`inl/main.go`

### 完整实现

```go
package main

import (
    "encoding/json"
    "flag"
    "fmt"
    "log"
    "os"

    "github.com/your-org/inl/internal/nrc"
)

type GSDDevice struct {
    VendorID      string `json:"VendorID"`
    VendorName    string `json:"VendorName"`
    DeviceID      string `json:"DeviceID"`
    GSDName       string `json:"GSDName"`
    MainFamily    string `json:"MainFamily"`
    ProductFamily string `json:"ProductFamily"`
}

type GSDResponse struct {
    DataType int         `json:"DataType"`
    Device   []GSDDevice `json:"Device"`
}

func main() {
    target := flag.String("target", "", "工业PC IP地址 (必填)")
    flag.Parse()

    if *target == "" {
        fmt.Fprintln(os.Stderr, "用法: go run main.go --target <IP地址>")
        os.Exit(1)
    }

    addr := *target + ":6000"
    fmt.Printf("🔌 连接 %s ...", addr)

    client := nrc.NewClient(addr)
    if err := client.Connect(); err != nil {
        log.Fatalf("❌ 连接失败: %v\n  请确认: 1) 工业PC已开机 2) IP地址正确 3) nrc2.out 已启动", err)
    }
    defer client.Close()
    fmt.Println(" OK")

    fmt.Println("📤 发送 DataType=13 (GSD 文件列表请求)")
    cmd, data, err := client.SendReceive(0x9275, `{"DataType":13}`)
    if err != nil {
        log.Fatalf("❌ 通信失败: %v", err)
    }

    if cmd != 0x9271 {
        log.Fatalf("❌ 意外响应命令字: 0x%04X (期望: 0x9271)", cmd)
    }

    var resp GSDResponse
    if err := json.Unmarshal(data, &resp); err != nil {
        log.Fatalf("❌ JSON 解析失败: %v\n  原始数据: %s", err, string(data))
    }

    fmt.Printf("\n📥 收到响应:\n")
    if len(resp.Device) == 0 {
        fmt.Println("  (无设备 — GSD 文件目录为空)")
        return
    }

    for _, d := range resp.Device {
        fmt.Printf("  - %s (%s) → %s\n", d.VendorName, d.VendorID, d.ProductFamily)
        fmt.Printf("    DeviceID: %s | GSD: %s\n", d.DeviceID, d.GSDName)
    }
    fmt.Printf("\n✅ 共发现 %d 个设备驱动\n", len(resp.Device))
}
```

### Step 4 验收标准

- [ ] `go build -o inl.exe` 编译成功，生成 `inl.exe`
- [ ] `go build ./...` 所有包编译通过

---

## Step 5：实机验收

### 环境准备

1. 确保工业 PC 已开机，`nrc2.out` 正在运行
2. 确保工业 PC 的 `./communication/Profinet/` 目录下有至少一个 `GSDML-*.xml` 文件
3. 确保调试 PC 与工业 PC 在同一局域网（能 ping 通）
4. 确认工业 PC 的 IP 地址（如 `192.168.1.100`）

### 运行命令

```bash
go run main.go --target 192.168.1.100
```

### 验收标准

- [ ] 能成功连接 `192.168.1.100:6000`
- [ ] 发送 `{"DataType":13}` 后收到 `0x9271` 响应
- [ ] 响应 JSON 中 `DataType` 字段为 `13`
- [ ] 响应 JSON 中 `Device` 数组非空（如果 GSD 文件存在）
- [ ] 每个设备对象包含 `VendorName`, `VendorID`, `DeviceID`, `GSDName` 字段
- [ ] 如果 `./communication/Profinet/` 下无 GSD 文件，`Device` 为空数组但不报错

### 故障排查

| 现象 | 可能原因 | 排查步骤 |
|------|---------|---------|
| `❌ 连接失败` | IP 不对、未开机、端口不对 | `ping <IP>` 确认可达；确认 nrc2.out 正在运行 |
| 连接成功但超时 | nrc2.out 未启动或协议号未注册 | 确认 `bindProtocalCB()` 已调用 |
| CRC 校验失败 | 字节序或 CRC 计算范围错误 | 用 Step 2 的单元测试验证 |
| 收到空 Device 列表 | GSD 文件目录为空 | 检查工业 PC 上 `./communication/Profinet/` 目录 |

---

## 完整验收清单（汇总）

### 离线验收（不需要工业 PC）

- [ ] Step 1: `go mod init` 成功，`go.mod` 内容正确
- [ ] Step 2: `go test ./internal/nrc/` 全部 PASS
- [ ] Step 2: PDF 无换行版本 CRC = `0x53DDEB72`，有换行版本 CRC = `0x6B926DFF`
- [ ] Step 2: BuildFrame → ReadFrame RoundTrip 完整还原
- [ ] Step 3: `go build ./...` 无编译错误
- [ ] Step 3: `go vet ./...` 无警告
- [ ] Step 4: `go build -o inl.exe` 成功

### 实机验收（需要工业 PC）

- [ ] Step 5: TCP 连接 `工业PC:6000` 成功
- [ ] Step 5: 发送 `0x9275 {"DataType":13}` 收到 `0x9271` 响应
- [ ] Step 5: 响应 JSON 解析成功，`Device` 列表正确显示

---

## 执行节奏

| Step | 内容 | 预计耗时 |
|------|------|---------|
| Step 1 | `go mod init` | 5 分钟 |
| Step 2 | `frame.go` + `frame_test.go` | 30 分钟 |
| Step 3 | `client.go` | 20 分钟 |
| Step 4 | `main.go` | 10 分钟 |
| Step 5 | 实机测试 | 5 分钟 |

**总共约 1 小时写完，到车间 5 分钟验证。**

---

## 相关文档

- [[inl-architecture]] — 架构设计文档
- [[inl-prd]] — 产品需求文档
- [[cli-architecture-overview]] — lark-cli 架构（参考源）
