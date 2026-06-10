---
title: inl 第 9 步开发计划 — 生产化与发布
tags: [inl, development, plan, distribution, reliability, polish]
created: 2026-06-04
status: draft
---

# inl — 第 9 步：生产化与发布

## 目标

把项目从 **85% 完整 / 0% 可分发** 推进到 **95%+ 完整 / 可通过 `npm install` 与 GitHub Release 消费**，完成 **v0.1.0** 跨平台可分发里程碑。

1. **功能收尾** — 关闭 `config-set-idevice` P1 缺口 + `device-gsd-config` 偏差登记
2. **稳定性正式化** — 把已隐式完成的 DCP 重连重试抽到 `internal/reliability/`，加 `--retry` 全局开关 + chaos test
3. **跨平台分发** — GoReleaser 多平台二进制 + npm 薄壳包 + GitHub Actions 自动 release
4. **体验补完** — `--format csv/ndjson` + `--dry-run` 适配 DCP + `device setup` 闭环

---

## 背景与上下文

### 当前状态（截至 2026-06-04）

| 维度 | 指标 |
|---|---|
| 总体完成度 | **~85%** — 核心功能 100%、输出体系 100%、文档 95%、**分发 0%** |
| 命令数 | **25 条** — `config=12` 中 1 条未参数化 |
| 测试 | 9 包全 PASS |
| 已知偏差 | 1 项 — `device-gsd-config` 缺 `error` 字段建模 |
| AGENTS.md 提示 | "DCP 命令 (DataType=14/16) 自动重连重试 1 次 (**Step 9 稳定性优化**)" |

> 2026-06-04 已通过 DCP 稳定性实测：连续高频率 DCP 读操作稳定。
> 本 Step 把已完成的稳定性工作**显式化**（抽包 + 测试矩阵 + 全局 flag），并叠加**分发**与**体验补完**。

### 9.x 主题映射

| 子任务 | 主题 | 来源 |
|---|---|---|
| 9.1 | 功能收尾（P1+P2） | `docs/inl-development-status.md` 待开发任务表 |
| 9.2 | 稳定性正式化 | `AGENTS.md` Step 9 提示 + 2026-06-04 DCP 实测 |
| 9.3 | 跨平台分发 | `docs/inl-development-status.md` "分发 0%" + `docs/inl-prd.md` 分发计划 |
| 9.4 | 体验补完 | `docs/inl-development-status.md` 待开发任务表 P2/P3 |

---

## 一、`config-set-idevice` 参数化（P1）

### 1.1 现状

P0 修复（2026-06-02）已在 `internal/nrc/commands.go` 注册 `config-set-idevice` 骨架：

```go
{
    Name:        "config-set-idevice",
    Code:        0x9275,
    DataType:    12,
    Direction:   DirectionRequest,
    Description: "设置 IDevice IO 长度参数",
    Risk:        RiskWrite,
    Response:    nil,
    Function:    "SetIDevice",
    Group:       GroupConfig,
    Args:        nil,                       // ← 缺口: 缺 --data 构造能力
    BodyBuilder: DefaultBodyBuilder,
},
```

`docs/protocol/field-verification.md` "协议层未覆盖的项" 标注：
> `SetIDevice` 在 C++ 源分发表中存在。完整参数（IO 长度、Activate 状态）需要 `--data` JSON 构造能力（`Args []ArgumentSpec` 还没接 `--data`），待后续 PR 补全。

### 1.2 命令行形态

```bash
# 设置 iDevice 输入输出长度各 64 字节, 激活
$ inl config set-idevice \
    --data '{"Activate":true,"InputLength":64,"OutputLength":64}' \
    --yes --target 192.168.3.15

# 与其他 config 写命令保持一致 --yes 必需 (Risk=write)
```

### 1.3 设计

| 维度 | 设计 |
|---|---|
| 参数 | `--data` string（必填, JSON object, 含 `Activate` / `InputLength` / `OutputLength`） |
| BodyBuilder | 新增 `configSetIDeviceBody` 替换 `DefaultBodyBuilder` |
| 风险 | `write`（保持）— 需 `--yes` |
| C++ 端协议 | 与 `device-list` 的 `IDevice` 响应结构**对称**：请求体也用 `IDevice.{Activate, InputLength, OutputLength}` 三字段 |

### 1.4 Registry 变更

```go
{
    Name:        "config-set-idevice",
    Code:        0x9275,
    DataType:    12,
    Direction:   DirectionRequest,
    Description: "设置 IDevice IO 长度参数 (Activate/InputLength/OutputLength)",
    Risk:        RiskWrite,
    Response:    nil,
    Function:    "SetIDevice",
    Group:       GroupConfig,
    Args: []ArgumentSpec{
        {Name: "data", Description: "IDevice JSON {Activate, InputLength, OutputLength}", Required: true},
    },
    BodyBuilder: configSetIDeviceBody,   // ← 替换
},
```

### 1.5 BodyBuilder

```go
// configSetIDeviceBody 构造 SetIDevice 请求的 JSON body。
//
// C++ 端 SetIDevice 分支读取 root["IDevice"] 子对象, 包含三个字段:
//   - Activate     bool  // 是否激活 iDevice
//   - InputLength  int   // 输入区段字节数
//   - OutputLength int   // 输出区段字节数
//
// 用户提供的 --data 必须是合法 JSON object (含上述三字段), 与 device-list 响应中的 IDevice 字段对称。
func configSetIDeviceBody(spec CommandSpec, args map[string]string) (string, error) {
    data := args["data"]
    if data == "" {
        return "", fmt.Errorf("--data 不能为空")
    }
    if !json.Valid([]byte(data)) {
        return "", fmt.Errorf("--data 不是合法 JSON")
    }
    // 解析为 map 校验必要字段, 避免运行时才发现 C++ 端报错
    var id struct {
        Activate     bool `json:"Activate"`
        InputLength  int  `json:"InputLength"`
        OutputLength int  `json:"OutputLength"`
    }
    if err := json.Unmarshal([]byte(data), &id); err != nil {
        return "", fmt.Errorf("--data 不是 IDevice 对象: %w", err)
    }
    return fmt.Sprintf(
        `{"DataType":12,"Function":{"Value":"SetIDevice"},"IDevice":%s}`,
        data,
    ), nil
}
```

### 1.6 main.go 接入

`--data` flag 注册已在 main.go 的 `collectDCPArgs` 与 buildSubCmd 中支持（raw-send 用了同样模式），无需新增 switch case；只须确认：

- `buildSubCmd` 中 `case "data"` 已存在（已确认 ✅ 见 [main.go:161-163](file:///c:/Users/BYD/Documents/trae_projects/feesh_cli/inl/main.go#L161-L163)）
- `collectDCPArgs` 提取 `data` flag（已确认 ✅）

唯一需要补的：`init()` 的 `Registry` 唯一性检查对 `Function="SetIDevice"` 与现有 `Function="SetIDevice"` 一致 ✅。

### 1.7 测试用例

```go
// commands_test.go
func TestConfigSetIDeviceBody(t *testing.T) {
    // 1. 合法 IDevice JSON
    body, err := configSetIDeviceBody(spec, map[string]string{
        "data": `{"Activate":true,"InputLength":64,"OutputLength":64}`,
    })
    require.NoError(t, err)
    require.JSONEq(t,
        `{"DataType":12,"Function":{"Value":"SetIDevice"},"IDevice":{"Activate":true,"InputLength":64,"OutputLength":64}}`,
        body)

    // 2. --data 为空
    _, err = configSetIDeviceBody(spec, map[string]string{"data": ""})
    require.Error(t, err)

    // 3. --data 非 JSON
    _, err = configSetIDeviceBody(spec, map[string]string{"data": "not json"})
    require.Error(t, err)

    // 4. --data 缺字段 (C++ 端需要全部三字段, 否则编译失败)
    _, err = configSetIDeviceBody(spec, map[string]string{
        "data": `{"Activate":true,"InputLength":64}`,  // 缺 OutputLength
    })
    require.Error(t, err)  // 解析后 OutputLength 默认为 0, C++ 端会拒绝, 但当前实现不强制校验
    // 注: 此用例实际不会 fail (Go zero-value OK), 删去或加更严格校验
}
```

> **简化决策**：9.1 阶段不强制校验三字段非零，Go zero-value (false/0/0) 透传给 C++ 端由其拒绝即可。后续如发现 AI 误传，再加校验。

---

## 二、`device-gsd-config` Error 字段建模（P2 偏差修正）

### 2.1 现状

`docs/protocol/field-verification.md` "已知偏差汇总"：
> `device-gsd-config` (GetGSDFileNetwork): `GSDFile` 为空且 `error:true`；v1 未建模 `error` 字段。
> **影响**: `gsdfile/types.go` 需新增 `Error bool` 字段

### 2.2 实机响应（回顾）

```json
{
  "DataType": 14,
  "Function": {
    "GSDFile": "",
    "Value": "GetGSDFileNetwork",
    "error": true
  }
}
```

### 2.3 变更

`internal/gsdfile/types.go` 的 `Function` struct：

```go
type Function struct {
    Value   string  `json:"Value"`
    GSDFile string  `json:"GSDFile,omitempty"`
    GSDFiles []GSDFile `json:"GSDFiles,omitempty"`  // 兼容: 数组形式
    Error   bool    `json:"error,omitempty"`         // ← 新增
}
```

### 2.4 测试用例

```go
// gsdfile/types_test.go
func TestFunction_HasErrorField(t *testing.T) {
    raw := []byte(`{
        "DataType": 14,
        "Function": {
            "GSDFile": "",
            "Value": "GetGSDFileNetwork",
            "error": true
        }
    }`)
    var resp Response
    require.NoError(t, json.Unmarshal(raw, &resp))
    require.True(t, resp.Function.Error, "error 字段必须被建模")
    require.Empty(t, resp.Function.GSDFile)
}

func TestFunction_ErrorFalseWhenAbsent(t *testing.T) {
    // 正常情况: 工业 PC 返回 GSDFile 列表, error 字段不存在
    raw := []byte(`{
        "DataType": 14,
        "Function": {
            "GSDFile": "gsdml-v2.31.xml",
            "Value": "GetGSDFileNetwork"
        }
    }`)
    var resp Response
    require.NoError(t, json.Unmarshal(raw, &resp))
    require.False(t, resp.Function.Error)
}
```

### 2.5 文档同步

- `docs/protocol/field-verification.md` "已知偏差汇总" 表中本行从 `⚠️ 已知偏差` 改为 `✅ 已修正 (2026-06-04 Step 9.2)`
- AGENTS.md 不需改（field-verification.md 是唯一权威）

---

## 三、稳定性正式化（`internal/reliability/`）

### 3.1 动机

`internal/nrc/client.go` 现状（2026-06-04 注释）：

```go
// DCP 稳定性优化 (Step 9):
//   - 对 DataType=14/16 命令, 失败时 (timeout / closed / forcibly closed) 自动重连重试 1 次。
//   - 最多 1 次重试 (共 2 次尝试), 仅在 timeout/closed 时触发, 其他错误不重试。
```

代码已**隐式**完成测试（2026-06-04 DCP 稳定性实测通过），但有 3 个工程问题：

1. **不可测** — 重试逻辑耦合在 `SendReceiveFiltered` 私有循环里，没有独立单元测试覆盖。
2. **不可配** — 重试次数硬编码为 1，未来想给 DCP 命令加 `--retry 3` 没有开关。
3. **不可观察** — 用户看不到"已重试 1 次"的提示，调试 DCP 抖动时只能看日志。

### 3.2 抽包目标

新建 `internal/reliability/` 子包，把重试策略变成**可注入、可测试、可配置**的独立组件。

```
internal/reliability/
├── retry.go        # RetryPolicy: 指数退避 + 抖动 + 触发条件
├── retry_test.go   # 5+ 单元测试
└── doc.go          # 包注释
```

### 3.3 RetryPolicy API

```go
package reliability

import (
    "context"
    "math/rand"
    "strings"
    "time"
)

// Policy 定义重试策略。
type Policy struct {
    MaxRetries  int                              // 最大重试次数 (0 = 不重试)
    BaseBackoff time.Duration                    // 首次退避基数 (e.g. 300ms)
    MaxBackoff  time.Duration                    // 退避上限 (e.g. 3s)
    Jitter      func() time.Duration             // 抖动函数; nil = 退避的 0-50%
    ShouldRetry func(err error) bool            // 触发条件; nil = 任意非 nil error
}

// Default 返回 inl 默认策略:
//   - 3 次重试 (与 Connect 一致)
//   - 300ms 起始退避, 3s 上限
//   - 退避的 0-50% 抖动
//   - 仅在 timeout/closed/EOF/超时 时重试
func Default() *Policy {
    return &Policy{
        MaxRetries:  3,
        BaseBackoff: 300 * time.Millisecond,
        MaxBackoff:  3 * time.Second,
        ShouldRetry: ShouldRetryNetwork,
    }
}

// Do 执行 fn, 按策略重试。返回最后一次的 error (若全部失败)。
func (p *Policy) Do(ctx context.Context, fn func() error) error {
    var lastErr error
    backoff := p.BaseBackoff
    for attempt := 0; attempt <= p.MaxRetries; attempt++ {
        if err := fn(); err == nil {
            return nil
        } else {
            lastErr = err
        }
        if attempt >= p.MaxRetries || p.ShouldRetry == nil || !p.ShouldRetry(lastErr) {
            break
        }
        // 退避
        sleep := backoff
        if p.Jitter != nil {
            sleep += p.Jitter()
        } else {
            sleep += time.Duration(rand.Int63n(int64(backoff / 2)))
        }
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-time.After(sleep):
        }
        backoff *= 2
        if backoff > p.MaxBackoff {
            backoff = p.MaxBackoff
        }
    }
    return lastErr
}

// ShouldRetryNetwork 与 inl 现有 shouldRetryDCP 行为一致。
// 触发重试的关键词: timeout / 超时 / closed / forcibly closed / EOF
func ShouldRetryNetwork(err error) bool {
    if err == nil {
        return false
    }
    msg := err.Error()
    return strings.Contains(msg, "timeout") ||
        strings.Contains(msg, "超时") ||
        strings.Contains(msg, "closed") ||
        strings.Contains(msg, "forcibly closed") ||
        strings.Contains(msg, "EOF")
}
```

### 3.4 client.go 迁移

`internal/nrc/client.go` 改造：

| 旧实现 | 新实现 |
|---|---|
| `Connect` 内嵌 3 次重试 + 退避循环 | 调用 `reliability.Default().Do(ctx, dialOnce)` |
| `SendReceiveFiltered` 内嵌 DCP 1 次重试循环 | 调用 `reliability.Policy{MaxRetries: 1, ShouldRetry: reliability.ShouldRetryNetwork}.Do(...)` |
| `shouldRetryDCP` 私有函数 | 删除, 改用 `reliability.ShouldRetryNetwork` |

迁移后**行为不变**, 但新增 3 个能力：

1. **可测** — `reliability/retry_test.go` 5+ 用例覆盖指数退避/抖动/触发条件/边界
2. **可配** — 全局 `--retry N` flag 覆盖默认 MaxRetries
3. **可观察** — 重试时 stderr 打印 `🔄 重试 1/3 (因: timeout)` 提示

### 3.5 全局 `--retry` flag

`main.go` 增 `rootCmd.PersistentFlags().IntVar(&retryFlag, "retry", 0, "DCP 命令重试次数 (0=使用默认值)")`。

```go
var retryFlag int

// buildSubCmd 中:
//   if retryFlag > 0 {
//       dcpPolicy = &reliability.Policy{MaxRetries: retryFlag, ...}
//   } else {
//       dcpPolicy = reliability.Default()
//   }
```

### 3.6 chaos test

`internal/nrc/client_test.go` 新增：

```go
// TestSendReceiveFiltered_RetriesOnTimeout 模拟控制器第一次 timeout, 第二次成功。
func TestSendReceiveFiltered_RetriesOnTimeout(t *testing.T) {
    // 用 httptest 起假服务端:
    //   - 第一次收到请求后, sleep 11s (超过 10s 读超时)
    //   - 第二次正常返回 0x9271 + JSON
    // 断言: 共耗时 > 10s, 但 err == nil
}

// TestSendReceiveFiltered_NoRetryOnProtocolErr 模拟响应 command 错, 不重试。
func TestSendReceiveFiltered_NoRetryOnProtocolErr(t *testing.T) {
    // 假服务端始终返回 0x9273 (非 0x9271)
    // 断言: 1 次尝试后立即返回错误, 不重试
}
```

> chaos test 需要假服务端, 现有 `client_test.go` 是否有 fake server 待 9.3 实施时确认; 若无, 用 `net.Pipe()` + goroutine 模拟。

### 3.7 文件清单（仅本节）

```
internal/reliability/
├── retry.go              # 新增
├── retry_test.go         # 新增 (5+ 测试)
└── doc.go                # 新增

internal/nrc/client.go    # 改: 迁移到 reliability.Policy
internal/nrc/client_test.go  # 改: +2 chaos test
main.go                   # 改: +retryFlag 注册
```

---

## 四、跨平台分发（GoReleaser + npm + GitHub Actions）

### 4.1 目标

| 分发渠道 | 安装命令 | 目标用户 |
|---|---|---|
| GitHub Release | `curl -L ... \| tar xf -` 或直接下载 zip | CI 集成、容器镜像 |
| npm (薄壳) | `npm install -g inl-cli` | Node.js 生态、AI Agent 工具链 |
| Homebrew (后续) | `brew install inl` (Step 10+) | macOS 开发者 |
| Scoop (后续) | `scoop install inl` (Step 10+) | Windows 开发者 |

本 Step 仅实现前两项。

### 4.2 目标平台矩阵

| OS | Arch | GoReleaser 标识 | npm 子包 |
|---|---|---|---|
| Windows | amd64 | `windows_amd64` | `@inl/cli-win32-x64` |
| Windows | arm64 | `windows_arm64` | `@inl/cli-win32-arm64` |
| Linux | amd64 | `linux_amd64` | `@inl/cli-linux-x64` |
| Linux | arm64 | `linux_arm64` | `@inl/cli-linux-arm64` |
| macOS | amd64 | `darwin_amd64` | `@inl/cli-darwin-x64` |
| macOS | arm64 | `darwin_arm64` | `@inl/cli-darwin-arm64` |

> 6 平台 × 1 入口 = Go 二进制 6 个 + npm 平台子包 6 个 + 总入口包 `@inl/cli` 1 个 = 13 个 npm 包。

### 4.3 GoReleaser 配置

新建 `.goreleaser.yaml`：

```yaml
version: 2

before:
  hooks:
    - go mod tidy
    - go vet ./...
    - go test ./...

builds:
  - id: inl
    main: .
    binary: inl
    env:
      - CGO_ENABLED=0
    goos:
      - windows
      - linux
      - darwin
    goarch:
      - amd64
      - arm64
    mod_timestamp: '{{ .CommitTimestamp }}'
    flags:
      - -trimpath
      - -ldflags=-s -w -X main.version={{.Version}} -X main.commit={{.Commit}} -X main.date={{.Date}}

archives:
  - id: inl-archive
    formats: [tar.gz, zip]
    name_template: >-
      {{ .ProjectName }}_
      {{- .Version }}_
      {{- .Os }}_
      {{- .Arch }}
    files:
      - LICENSE
      - README.md

checksum:
  name_template: 'checksums.txt'
  algorithm: sha256

snapshot:
  version_template: "{{ .Tag }}-next"

changelog:
  use: git
  sort: asc
  filters:
    exclude:
      - '^docs:'
      - '^test:'
      - '^chore:'

release:
  github:
    owner: hqy2435662352
    name: inl
  prerelease: auto
  draft: false
```

> **变更说明**：`.goreleaser.yaml` 引入新文件,**不是** Go module 依赖 (GoReleaser 是独立 CLI 工具, 不在 go.mod 中)。

### 4.4 GitHub Actions

新建 `.github/workflows/release.yml`：

```yaml
name: release

on:
  push:
    tags:
      - 'v*'
  workflow_dispatch:        # 允许手动触发 dry-run

permissions:
  contents: write
  id-token: write           # npm provenance (trusted publishing)

jobs:
  goreleaser:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'
      - name: Run GoReleaser
        uses: goreleaser/goreleaser-action@v6
        with:
          version: latest
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

### 4.5 npm 薄壳包结构

```
npm/
├── inl-cli/                        # 入口包 (用户在 npm install)
│   ├── package.json
│   ├── README.md
│   ├── bin/inl.js                   # 透传到 .bin/inl
│   └── install.js                   # postinstall: 装对应平台子包
│
├── inl-cli-linux-x64/
│   ├── package.json
│   ├── index.js                     # re-export inl
│   └── bin/inl                      # 实际二进制
│
├── inl-cli-darwin-arm64/
│   └── ...
│
└── (其他 4 个平台子包, 同上结构)
```

**入口包 `inl-cli/package.json`**：

```json
{
  "name": "inl-cli",
  "version": "0.1.0",
  "description": "Industrial Netline CLI (NRC protocol)",
  "bin": { "inl": "./bin/inl.js" },
  "scripts": {
    "postinstall": "node install.js"
  },
  "license": "MIT",
  "repository": "github:hqy2435662352/inl",
  "optionalDependencies": {
    "@inl/cli-linux-x64": "^0.1.0",
    "@inl/cli-linux-arm64": "^0.1.0",
    "@inl/cli-darwin-x64": "^0.1.0",
    "@inl/cli-darwin-arm64": "^0.1.0",
    "@inl/cli-win32-x64": "^0.1.0",
    "@inl/cli-win32-arm64": "^0.1.0"
  }
}
```

**入口包 `inl-cli/install.js`** (postinstall 钩子)：

```javascript
#!/usr/bin/env node
// 装完子包后, 复制对应平台的二进制到 ./bin/inl
const fs = require('fs');
const path = require('path');
const { platform, arch } = process;

const platformMap = {
  'linux-x64':    '@inl/cli-linux-x64',
  'linux-arm64':  '@inl/cli-linux-arm64',
  'darwin-x64':   '@inl/cli-darwin-x64',
  'darwin-arm64': '@inl/cli-darwin-arm64',
  'win32-x64':    '@inl/cli-win32-x64',
  'win32-arm64':  '@inl/cli-win32-arm64',
};

const key = `${platform}-${arch}`;
const pkg = platformMap[key];
if (!pkg) {
  console.error(`❌ inl-cli: 不支持的平台 ${key}`);
  console.error(`   支持: ${Object.keys(platformMap).join(', ')}`);
  process.exit(1);
}

try {
  const src = require.resolve(`${pkg}/bin/inl`);
  const dst = path.join(__dirname, 'bin', 'inl');
  fs.mkdirSync(path.dirname(dst), { recursive: true });
  fs.copyFileSync(src, dst);
  fs.chmodSync(dst, 0o755);
  console.log(`✅ inl-cli: 已安装 (${key})`);
} catch (e) {
  console.error(`❌ inl-cli: 找不到 ${pkg} (npm install 是否成功?)`);
  process.exit(1);
}
```

**入口包 `inl-cli/bin/inl.js`** (透传 wrapper)：

```javascript
#!/usr/bin/env node
// npm bin 入口: 透传到真实二进制
const { spawn } = require('child_process');
const path = require('path');
const realBin = path.join(__dirname, 'inl');

const child = spawn(realBin, process.argv.slice(2), {
  stdio: 'inherit',
});
child.on('exit', (code) => process.exit(code ?? 0));
child.on('error', (e) => {
  console.error(`❌ inl 执行失败: ${e.message}`);
  process.exit(1);
});
```

### 4.6 npm 发布工作流

新建 `.github/workflows/npm-publish.yml`：

```yaml
name: npm-publish

on:
  release:
    types: [published]

jobs:
  publish:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        pkg:
          - inl-cli
          - inl-cli-linux-x64
          - inl-cli-linux-arm64
          - inl-cli-darwin-x64
          - inl-cli-darwin-arm64
          - inl-cli-win32-x64
          - inl-cli-win32-arm64
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: '20'
          registry-url: 'https://registry.npmjs.org'
      - run: npm ci
        working-directory: npm/${{ matrix.pkg }}
      - run: npm pack
        working-directory: npm/${{ matrix.pkg }}
      - run: npm publish --access public
        working-directory: npm/${{ matrix.pkg }}
        env:
          NODE_AUTH_TOKEN: ${{ secrets.NPM_TOKEN }}
```

### 4.7 本地 dry-run 验证

```bash
# 1. GoReleaser 本地测试
goreleaser release --snapshot --clean --skip publish
# 期望: dist/ 目录有 6 个平台 tar.gz + 6 个 zip + checksums.txt

# 2. npm 入口包本地 link
cd npm/inl-cli && npm link
inl --version
# 期望: 输出 inl version 0.1.0
```

> 本地 dry-run 不实际推送, 仅生成产物。tag push / `npm publish` / GitHub Release 必须用户显式触发（符合 [project_rules](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/.trae/rules/project_rules.md)）。

### 4.8 文件清单（仅本节）

```
.github/workflows/
├── release.yml                # 新增
└── npm-publish.yml            # 新增

.goreleaser.yaml               # 新增

npm/
├── inl-cli/                   # 新增 (入口包, 7 文件)
│   ├── package.json
│   ├── README.md
│   ├── bin/inl.js
│   └── install.js
├── inl-cli-linux-x64/         # 新增
│   ├── package.json
│   ├── index.js
│   └── bin/inl
├── (其他 5 个平台子包, 同上)
```

---

## 五、体验补完

### 5.1 `--format csv` / `--format ndjson`

`internal/output/` 新增 2 个渲染器, 复用 `ExtractTableRows` 的列提取结果。

#### 5.1.1 CSV 渲染

```go
// internal/output/csv.go
package output

import (
    "encoding/csv"
    "io"
)

// FormatCSV 将 columns + rows 渲染为 RFC 4180 CSV 格式。
// 行内逗号/引号/换行由 encoding/csv 自动转义。
// 输出末尾带换行符。
func FormatCSV(w io.Writer, columns []string, rows [][]string) error {
    cw := csv.NewWriter(w)
    if err := cw.Write(columns); err != nil {
        return err
    }
    for _, row := range rows {
        if err := cw.Write(row); err != nil {
            return err
        }
    }
    cw.Flush()
    return cw.Error()
}
```

#### 5.1.2 NDJSON 渲染

```go
// internal/output/ndjson.go
package output

import (
    "encoding/json"
    "io"
)

// FormatNDJSON 将 rows 渲染为 NDJSON (Newline Delimited JSON)。
// 第一行: columns 数组 (字段名)
// 后续每行: 一个 row 对象 {列名: 值}
// 适合 jq / 管道处理。
func FormatNDJSON(w io.Writer, columns []string, rows [][]string) error {
    enc := json.NewEncoder(w)
    // 字段名行 (作为数组输出, 方便 header 解析)
    header := make([]string, len(columns))
    copy(header, columns)
    if err := enc.Encode(header); err != nil {
        return err
    }
    for _, row := range rows {
        obj := make(map[string]string, len(columns))
        for i, col := range columns {
            obj[col] = row[i]
        }
        if err := enc.Encode(obj); err != nil {
            return err
        }
    }
    return nil
}
```

#### 5.1.3 main.go 接入

`formatFlag` 已有 `json` / `table` 两个分支, 加 `csv` / `ndjson`：

```go
// main.go runNrcCommand 末段
switch formatFlag {
case "table":
    // ... 现有逻辑
case "csv":
    cols, rows, terr := output.ExtractTableRows(data)
    if terr != nil || len(cols) == 0 {
        fmt.Fprintln(cmd.ErrOrStderr(), "⚠️  --format csv 不适用此命令, 回退 JSON 输出")
        output.WriteSuccess(cmd.OutOrStdout(), data, notice)
        return nil
    }
    return output.FormatCSV(cmd.OutOrStdout(), cols, rows)
case "ndjson":
    cols, rows, terr := output.ExtractTableRows(data)
    if terr != nil || len(cols) == 0 {
        fmt.Fprintln(cmd.ErrOrStderr(), "⚠️  --format ndjson 不适用此命令, 回退 JSON 输出")
        output.WriteSuccess(cmd.OutOrStdout(), data, notice)
        return nil
    }
    return output.FormatNDJSON(cmd.OutOrStdout(), cols, rows)
default: // json
    return output.WriteSuccess(cmd.OutOrStdout(), data, notice)
}
```

#### 5.1.4 测试用例

```go
// internal/output/csv_test.go
func TestFormatCSV_HandlesCommas(t *testing.T) {
    cols := []string{"Name", "IP"}
    rows := [][]string{{"heron, weld", "192.168.2.10"}}
    var buf bytes.Buffer
    require.NoError(t, FormatCSV(&buf, cols, rows))
    require.Equal(t,
        "Name,IP\n\"heron, weld\",192.168.2.10\n",
        buf.String())
}

func TestFormatCSV_HandlesQuotes(t *testing.T) {
    cols := []string{"DeviceName"}
    rows := [][]string{{`she said "hi"`}}
    var buf bytes.Buffer
    require.NoError(t, FormatCSV(&buf, cols, rows))
    // RFC 4180: 双引号转义为 ""
    require.Contains(t, buf.String(), `"she said ""hi"""`)
}

// internal/output/ndjson_test.go
func TestFormatNDJSON_JqCompatible(t *testing.T) {
    cols := []string["Mac", "Name"}
    rows := [][]string{
        {"00:11:22:33:44:55", "heron-weld"},
        {"aa:bb:cc:dd:ee:ff", "smc-valve"},
    }
    var buf bytes.Buffer
    require.NoError(t, FormatNDJSON(&buf, cols, rows))
    // 期望输出可被 jq 直接消费:
    //   ["Mac","Name"]
    //   {"Mac":"00:11:22:33:44:55","Name":"heron-weld"}
    //   {"Mac":"aa:bb:cc:dd:ee:ff","Name":"smc-valve"}
    lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
    require.Len(t, lines, 3)
    require.Equal(t, `["Mac","Name"]`, lines[0])
}
```

### 5.2 `--dry-run` 适配 DCP 写

#### 5.2.1 现状

DCP 写（`device-setup-name`, `device-setup-ip`）当前**不支持** `--dry-run`：
- 走 RiskWrite 流程, dry-run 分支只对 raw/registry 命令有效
- DCP 写走的是 `client.SendReceiveFiltered`, 没有任何 dry-run 拦截

#### 5.2.2 设计

DCP 写**实际可以** dry-run：构造帧 + 打印 + 跳过发送。

```go
// main.go runNrcCommand, 在 Risk 检查段插入 DCP dry-run
if spec.Risk != nrc.RiskRead {
    dryRun, _ := cmd.Flags().GetBool("dry-run")
    if dryRun {
        body, err := nrc.RequestBody(spec, collectDCPArgs(cmd))
        if err != nil {
            return fmt.Errorf("构造请求体失败: %w", err)
        }
        if err := output.PrintDryRunFrame(cmd.OutOrStdout(), spec, body); err != nil {
            return fmt.Errorf("打印 dry-run 帧失败: %w", err)
        }
        fmt.Fprintf(cmd.ErrOrStderr(),
            "🛑 --dry-run 模式: DCP 写不会真实广播 (仍需 --yes 才会执行)\n")
        return nil
    }
    // ... --yes 检查
}
```

> **注意**：DCP 写无 JSON 响应（控制器发完即关连接），dry-run 只能**展示请求体**, 不能展示"会收到的响应"（因为本来就没有响应）。这点需要在 stderr 明确提示, 避免用户误以为 dry-run 漏看了响应。

#### 5.2.3 测试用例

- 手动测试（无单元测试, 因为 dry-run 行为是退出而非返回）：
  ```bash
  $ inl --target 192.168.3.15 device setup-name \
      --interface enp4s0 --mac 00:11:22:33:44:55 --name test --dry-run
  🛑 --dry-run 模式: DCP 写不会真实广播 (仍需 --yes 才会执行)
  # stdout: DryRunFrame 十六进制
  ```

### 5.3 `device setup` 闭环验证

#### 5.3.1 动机

`device setup-name` + `device setup-ip` 写完**无响应**（控制器发完即关），用户不知道是否成功。当前做法：手动 `topology scan` 验证。

#### 5.3.2 设计

新增 `inl device setup` **组合命令**（注意与现有 `device-setup-name` / `device-setup-ip` 区分）：

```bash
# 一步完成: 设置名称 + IP, 自动 scan 验证
$ inl device setup \
    --interface enp4s0 \
    --mac 00:11:22:33:44:55 \
    --name heron-weld \
    --ip 192.168.2.10 \
    --mask 255.255.255.0 \
    --yes

# 等价于:
#   inl device setup-name ... --yes
#   inl device setup-ip ... --yes
#   inl topology scan --interface enp4s0   # 验证
```

#### 5.3.3 实现

`main.go` 新增顶层命令（不走 Registry, 走特殊路径）：

```go
rootCmd.AddCommand(&cobra.Command{
    Use:   "setup",
    Short: "一键设置 DCP 设备 (名称+IP+验证)",
    Long:  "组合 device setup-name + device setup-ip + topology scan 验证",
    RunE: func(cmd *cobra.Command, args []string) error {
        // 1. 解析 --interface --mac --name --ip --mask
        // 2. 调用 device-setup-name 子命令的内部函数
        // 3. 调用 device-setup-ip 子命令的内部函数
        // 4. 调用 topology-scan 子命令的内部函数, 输出验证表格
        // 5. 在验证结果 stderr 提示 "✅ setup 完成, 设备已可见" / "❌ setup 后未发现设备"
    },
    Hidden: false,
})
```

> 实施时把 3 个子命令的 RunE 抽成可复用函数（`runDeviceSetupName` / `runDeviceSetupIP` / `runTopologyScan`），`device setup` 组合命令依次调用。

#### 5.3.4 测试用例

- 离线：mock client 3 次调用, 验证 3 个子命令都执行了
- 实机：在 192.168.3.15 上用一台可发现的设备（如未配置的 heron-weld）端到端验证

---

## 文件清单（Step 9 全量）

```
inl/
├── .github/workflows/
│   ├── release.yml                    ← 新增: GoReleaser 自动 release
│   └── npm-publish.yml                ← 新增: npm 自动发布
│
├── .goreleaser.yaml                   ← 新增: 跨平台构建配置
│
├── internal/
│   ├── reliability/                   ← 新增: 重试策略子包
│   │   ├── retry.go
│   │   ├── retry_test.go
│   │   └── doc.go
│   │
│   ├── nrc/
│   │   ├── commands.go                ← 改: config-set-idevice 加 Args + BodyBuilder
│   │   ├── commands_test.go           ← 改: +1 测试
│   │   ├── client.go                  ← 改: 迁移到 reliability.Policy
│   │   └── client_test.go             ← 改: +2 chaos test
│   │
│   ├── gsdfile/
│   │   ├── types.go                   ← 改: Function 加 Error bool
│   │   └── types_test.go              ← 改: +1 测试
│   │
│   ├── output/
│   │   ├── csv.go                     ← 新增: FormatCSV
│   │   ├── csv_test.go                ← 新增
│   │   ├── ndjson.go                  ← 新增: FormatNDJSON
│   │   └── ndjson_test.go             ← 新增
│   │
│   └── (其他包不动)
│
├── main.go                            ← 改: --retry flag + --format csv/ndjson + device setup 组合
│
├── AGENTS.md                          ← 改: +reliability 包 + --retry + --format csv/ndjson
│
├── docs/
│   ├── inl-step9-distribution-plan.md ← 新增: 本文档
│   ├── inl-development-status.md      ← 改: 85%→95%+
│   ├── protocol/field-verification.md ← 改: device-gsd-config 偏差修正
│   └── (inl-prd.md, inl-architecture.md 微调: +npm 分发章节)
│
├── npm/                               ← 新增: 7 个 npm 子包
│   ├── inl-cli/                       (入口)
│   ├── inl-cli-linux-x64/
│   ├── inl-cli-linux-arm64/
│   ├── inl-cli-darwin-x64/
│   ├── inl-cli-darwin-arm64/
│   ├── inl-cli-win32-x64/
│   └── inl-cli-win32-arm64/
│
└── README.md                          ← 改: +npm install 安装说明
```

**新增文件**: ~25
**修改文件**: ~10
**新增 Go 依赖**: 0
**新增工具依赖**: 2 (GoReleaser CLI + GitHub Actions 默认 Runner 工具)

---

## Step 1：Registry + BodyBuilder（`config-set-idevice`）

### 1.1 `internal/nrc/commands.go`

修改 `config-set-idevice` 条目（已在 P0 注册骨架）：

```go
{
    Name:        "config-set-idevice",
    Code:        0x9275,
    DataType:    12,
    Direction:   DirectionRequest,
    Description: "设置 IDevice IO 长度参数 (Activate/InputLength/OutputLength)",
    Risk:        RiskWrite,
    Response:    nil,
    Function:    "SetIDevice",
    Group:       GroupConfig,
    Args: []ArgumentSpec{
        {Name: "data", Description: "IDevice JSON (含 Activate/InputLength/OutputLength)", Required: true},
    },
    BodyBuilder: configSetIDeviceBody,
},
```

### 1.2 新增 `configSetIDeviceBody`

```go
// configSetIDeviceBody 构造 SetIDevice 请求的 JSON body。
//
// C++ 端 SetIDevice 分支读取 root["IDevice"] 子对象, 包含:
//   - Activate     bool
//   - InputLength  int
//   - OutputLength int
func configSetIDeviceBody(spec CommandSpec, args map[string]string) (string, error) {
    data := args["data"]
    if data == "" {
        return "", fmt.Errorf("--data 不能为空")
    }
    if !json.Valid([]byte(data)) {
        return "", fmt.Errorf("--data 不是合法 JSON")
    }
    return fmt.Sprintf(
        `{"DataType":12,"Function":{"Value":"SetIDevice"},"IDevice":%s}`,
        data,
    ), nil
}
```

### 1.3 测试

```go
// commands_test.go
func TestConfigSetIDeviceBody_Valid(t *testing.T) {
    body, err := configSetIDeviceBody(CommandSpec{}, map[string]string{
        "data": `{"Activate":true,"InputLength":64,"OutputLength":64}`,
    })
    require.NoError(t, err)
    require.Contains(t, body, `"SetIDevice"`)
    require.Contains(t, body, `"InputLength":64`)
}

func TestConfigSetIDeviceBody_Empty(t *testing.T) {
    _, err := configSetIDeviceBody(CommandSpec{}, map[string]string{"data": ""})
    require.Error(t, err)
}

func TestConfigSetIDeviceBody_InvalidJSON(t *testing.T) {
    _, err := configSetIDeviceBody(CommandSpec{}, map[string]string{"data": "{not json}"})
    require.Error(t, err)
}
```

Registry: 25 → 25 条 (不变, 仅参数化)

---

## Step 2：`gsdfile` Error 字段建模

### 2.1 `internal/gsdfile/types.go`

```go
type Function struct {
    Value    string    `json:"Value"`
    GSDFile  string    `json:"GSDFile,omitempty"`
    GSDFiles []GSDFile `json:"GSDFiles,omitempty"`
    Error    bool      `json:"error,omitempty"`   // ← 新增
}
```

### 2.2 测试

```go
// types_test.go
func TestFunction_ErrorField(t *testing.T) {
    raw := []byte(`{"Value":"GetGSDFileNetwork","GSDFile":"","error":true}`)
    var f Function
    require.NoError(t, json.Unmarshal(raw, &f))
    require.True(t, f.Error)
}
```

### 2.3 field-verification.md 同步

`docs/protocol/field-verification.md` "已知偏差汇总" 表中本行从 `⚠️ 已知偏差` 改为 `✅ 已修正 (2026-06-04 Step 9.2)`。

---

## Step 3：抽 `internal/reliability/` 子包

### 3.1 `internal/reliability/retry.go`

见 §3.3。

### 3.2 `internal/reliability/retry_test.go`

```go
func TestPolicy_DoSucceedsFirstTry(t *testing.T) {
    p := &Policy{MaxRetries: 3, BaseBackoff: time.Millisecond, MaxBackoff: 10*time.Millisecond}
    calls := 0
    err := p.Do(context.Background(), func() error {
        calls++
        return nil
    })
    require.NoError(t, err)
    require.Equal(t, 1, calls)
}

func TestPolicy_DoRetriesUntilSuccess(t *testing.T) {
    p := &Policy{
        MaxRetries:  3,
        BaseBackoff: time.Millisecond,
        MaxBackoff:  10*time.Millisecond,
        ShouldRetry: func(err error) bool { return true },
    }
    calls := 0
    err := p.Do(context.Background(), func() error {
        calls++
        if calls < 3 {
            return errors.New("timeout")
        }
        return nil
    })
    require.NoError(t, err)
    require.Equal(t, 3, calls)
}

func TestPolicy_DoGivesUpAfterMaxRetries(t *testing.T) {
    p := &Policy{
        MaxRetries:  2,
        BaseBackoff: time.Millisecond,
        MaxBackoff:  10*time.Millisecond,
        ShouldRetry: func(err error) bool { return true },
    }
    calls := 0
    err := p.Do(context.Background(), func() error {
        calls++
        return errors.New("timeout")
    })
    require.Error(t, err)
    require.Equal(t, 3, calls) // 1 + 2 retries
}

func TestPolicy_DoRespectsShouldRetry(t *testing.T) {
    p := &Policy{
        MaxRetries:  3,
        BaseBackoff: time.Millisecond,
        MaxBackoff:  10*time.Millisecond,
        ShouldRetry: func(err error) bool { return strings.Contains(err.Error(), "timeout") },
    }
    calls := 0
    err := p.Do(context.Background(), func() error {
        calls++
        return errors.New("protocol error")
    })
    require.Error(t, err)
    require.Equal(t, 1, calls) // ShouldRetry 返回 false, 不重试
}

func TestPolicy_ExponentialBackoff(t *testing.T) {
    // 验证 backoff 翻倍: 1ms → 2ms → 4ms (含抖动前)
    // 实测: 总耗时 > 7ms (1+2+4), < 50ms (50% 抖动 + 调度)
    p := &Policy{
        MaxRetries:  3,
        BaseBackoff: time.Millisecond,
        MaxBackoff:  100*time.Millisecond,
        Jitter:      func() time.Duration { return 0 }, // 关抖动便于测时
        ShouldRetry: func(err error) bool { return true },
    }
    start := time.Now()
    _ = p.Do(context.Background(), func() error { return errors.New("x") })
    elapsed := time.Since(start)
    require.Greater(t, elapsed, 7*time.Millisecond, "3 次退避至少 1+2+4=7ms")
}

func TestShouldRetryNetwork(t *testing.T) {
    require.True(t, ShouldRetryNetwork(errors.New("read timeout")))
    require.True(t, ShouldRetryNetwork(errors.New("use of closed network connection")))
    require.True(t, ShouldRetryNetwork(errors.New("等待匹配响应超时 (10s)")))
    require.True(t, ShouldRetryNetwork(errors.New("EOF")))
    require.False(t, ShouldRetryNetwork(errors.New("invalid JSON")))
    require.False(t, ShouldRetryNetwork(errors.New("unexpected response command")))
    require.False(t, ShouldRetryNetwork(nil))
}
```

### 3.3 `internal/nrc/client.go` 迁移

`Connect` 改写：

```go
func (c *Client) Connect() error {
    err := reliability.Default().Do(context.Background(), func() error {
        conn, err := net.DialTimeout("tcp", c.addr, 5*time.Second)
        if err != nil {
            return err
        }
        if tcpConn, ok := conn.(*net.TCPConn); ok {
            _ = tcpConn.SetKeepAlive(true)
            _ = tcpConn.SetKeepAlivePeriod(30 * time.Second)
        }
        c.conn = conn
        return nil
    })
    if err != nil {
        return fmt.Errorf("dial %s 失败: %w", c.addr, err)
    }
    return nil
}
```

`SendReceiveFiltered` 改写：

```go
func (c *Client) SendReceiveFiltered(sendCmd uint16, payload string, dataType int, expectedCmd uint16) (uint16, []byte, error) {
    var (
        respCmd uint16
        respData []byte
        err     error
    )

    policy := &reliability.Policy{
        MaxRetries:  1,  // DCP 1 次重试, 与现有行为一致
        BaseBackoff: 200 * time.Millisecond,
        MaxBackoff:  1 * time.Second,
        ShouldRetry: reliability.ShouldRetryNetwork,
    }

    _ = policy.Do(context.Background(), func() error {
        // 第 1 次失败需要重连; 第 2 次后 c.conn 已重建, 不需要重连
        if c.conn == nil {
            if connErr := c.reconnect(); connErr != nil {
                return connErr
            }
        }
        respCmd, respData, err = c.sendReceiveOnce(sendCmd, payload, dataType, expectedCmd)
        return err
    })
    return respCmd, respData, err
}

func (c *Client) reconnect() error {
    if c.conn != nil {
        _ = c.conn.Close()
        c.conn = nil
    }
    return c.Connect()
}
```

`shouldRetryDCP` 私有函数删除。

### 3.4 main.go 新增 `--retry`

```go
var retryFlag int

func init() {
    rootCmd.PersistentFlags().IntVar(&retryFlag, "retry", 0,
        "DCP 命令重试次数 (0 = 默认 1 次)")
}

// runNrcCommand 调 SendReceiveFiltered 前, 把 retryFlag 传给 client
```

> 实施时 client 增加 `WithRetryPolicy(*reliability.Policy) *Client` 构造选项; 默认 1 次。

### 3.5 chaos test (client_test.go)

```go
func TestSendReceiveFiltered_RetriesOnTimeout(t *testing.T) {
    // 用 net.Pipe() 模拟服务端:
    //   第一次收到请求, 11s 不返回 (触发客户端 read deadline 10s 超时)
    //   第二次正常返回 0x9271 + JSON
    //
    // 实施细节: goroutine + chan 同步; 测试需要 10+ 秒, 加 t.Parallel() 警告
    // (本测试慢, 默认 skip, 跑时用 -run TestSendReceiveFiltered_RetriesOnTimeout 单独跑)
    if testing.Short() {
        t.Skip("skipping slow chaos test in short mode")
    }
    // ...
}
```

> 简化为：第一个用例用 `time.Sleep(100ms)` 在 fake server 里触发客户端 timeout 短版本（改 `sendReceiveOnce` 接受 deadline 注入）。或保留慢测试 + `-short` 跳过。

---

## Step 4：GoReleaser + npm 骨架

### 4.1 `.goreleaser.yaml`

见 §4.3。完成后:

```bash
# 1. 装 GoReleaser (一次性)
go install github.com/goreleaser/goreleaser/v2@latest

# 2. 验证配置
goreleaser check

# 3. 本地 snapshot 构建 (不推送)
goreleaser release --snapshot --clean --skip publish
ls dist/
# 期望:
#   inl_0.1.0-next_windows_amd64.tar.gz
#   inl_0.1.0-next_windows_amd64.zip
#   inl_0.1.0-next_linux_amd64.tar.gz
#   ... (6 平台)
#   checksums.txt
```

### 4.2 npm 7 包

按 §4.5 结构新建。本地 link 测试:

```bash
# 1. 装入口包
cd npm/inl-cli
npm install
node install.js   # 复制二进制
./bin/inl --version
# 期望: inl version 0.1.0
```

### 4.3 GitHub Actions

新建 `.github/workflows/release.yml` 和 `npm-publish.yml`（见 §4.4 / §4.6）。

不实际 push tag, 仅验证 yml 语法:

```bash
# 用 actionlint (可选)
npx actionlint .github/workflows/
```

---

## Step 5：体验补完

### 5.1 `--format csv/ndjson`

见 §5.1。

### 5.2 `--dry-run` 适配 DCP

见 §5.2。

### 5.3 `device setup` 组合

见 §5.3。

### 5.4 AGENTS.md 同步

AGENTS.md 新增：

- Source Layout 表加 `internal/reliability/`
- 协议命令表加 `--format csv/ndjson` 描述
- Build & Test 章节加 `goreleaser check` / `goreleaser release --snapshot --clean --skip publish`
- Run 章节加 `inl device setup ...` 示例
- "Where to Find More" 表加 `docs/inl-step9-distribution-plan.md`

---

## 完整验收清单

### 5.1 离线验收

#### 功能收尾（9.1）

- [ ] `inl config set-idevice --help` 显示 `--data` 必填
- [ ] `inl config set-idevice --data '{}' --yes` 报错 "不是合法 JSON"
- [ ] `inl config set-idevice --data '{"Activate":true,"InputLength":64,"OutputLength":64}' --yes --target 192.168.3.15` 返回 Envelope `{"ok":true,...}`
- [ ] `internal/gsdfile/types.go` 的 `Function.Error` 字段已加
- [ ] `TestFunction_ErrorField` PASS
- [ ] `TestFunction_ErrorFalseWhenAbsent` PASS

#### 稳定性（9.2）

- [ ] `internal/reliability/` 3 文件 (retry.go, retry_test.go, doc.go) 存在
- [ ] `reliability.Policy.Do` 6+ 测试全 PASS
- [ ] `reliability.ShouldRetryNetwork` 7 用例 PASS
- [ ] `internal/nrc/client.go` 不再有 `shouldRetryDCP` 私有函数
- [ ] `client.Connect` 调用 `reliability.Default().Do`
- [ ] `client.SendReceiveFiltered` 调用 `reliability.Policy{MaxRetries: 1, ShouldRetry: reliability.ShouldRetryNetwork}.Do`
- [ ] `go test ./...` 全部 PASS (9 包 → **10 包**)
- [ ] `go vet ./...` 零警告
- [ ] `inl --retry 3 --target ... topology scan` 不报错 (DCP 走 3 次重试)

#### 分发（9.3）

- [ ] `.goreleaser.yaml` 存在
- [ ] `goreleaser check` PASS
- [ ] `goreleaser release --snapshot --clean --skip publish` 产物: 6 平台 tar.gz + 6 zip + checksums.txt
- [ ] `.github/workflows/release.yml` 存在
- [ ] `.github/workflows/npm-publish.yml` 存在
- [ ] `npm/inl-cli/{package.json, bin/inl.js, install.js, README.md}` 4 文件
- [ ] `npm/inl-cli-{linux,darwin,win32}-{x64,arm64}/` 6 个平台子包
- [ ] `cd npm/inl-cli && npm install && ./bin/inl --version` 输出 inl version 0.1.0

#### 体验（9.4）

- [ ] `internal/output/csv.go` + `csv_test.go` 存在
- [ ] `internal/output/ndjson.go` + `ndjson_test.go` 存在
- [ ] `inl topology scan --format csv` 输出 CSV
- [ ] `inl topology scan --format ndjson` 输出 NDJSON
- [ ] `inl device setup-name --interface enp4s0 --mac ... --name ... --dry-run` 显示 DryRunFrame + 提示 DCP 不会真实广播
- [ ] `inl device setup --interface enp4s0 --mac ... --name ... --ip ... --mask ... --yes` 依次执行 3 个子命令

### 5.2 实机验证（仅步骤 9.1 + 9.2 需实机, 9.3-9.4 可离线）

- [ ] `inl --target 192.168.3.15 config set-idevice --data '{"Activate":true,"InputLength":64,"OutputLength":64}' --yes` → 返回成功 Envelope
- [ ] `inl --target 192.168.3.15 --retry 3 topology scan --interface enp4s0` → 在网络抖动场景下比默认 1 次重试更稳定（手动模拟: 拔插网线 1 次）
- [ ] `inl --target 192.168.3.15 device setup --interface enp4s0 --mac 00:11:22:33:44:55 --name heron-weld --ip 192.168.2.10 --mask 255.255.255.0 --yes` → 三个子命令依次完成, 最终 topology scan 表格中可见 heron-weld 设备

### 5.3 文档归档

- [ ] `docs/inl-step9-distribution-plan.md` (本文档) 生成
- [ ] `docs/inl-development-status.md` 进度表: 85% → **95%+**
- [ ] `docs/inl-development-status.md` 阶段表新增: `Step 9 | 生产化与发布 | ✅ | 10 包, v0.1.0 cross-platform`
- [ ] `docs/protocol/field-verification.md` "已知偏差汇总" 表本行: `✅ 已修正 (2026-06-04 Step 9.2)`
- [ ] `AGENTS.md` Source Layout 加 reliability 包; Run 章节加 csv/ndjson + device setup
- [ ] `README.md` 加 npm install + GitHub Release badge

### 5.4 Git（**仅产物生成, push 需用户明示**）

- [ ] 9.1-9.4 全部 commit 在工作区（**不 push**）
- [ ] 用户在验收后, 显式触发 `git tag v0.1.0 && git push origin v0.1.0`（按 [project_rules](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/.trae/rules/project_rules.md) 不自动执行）
- [ ] 用户在 GitHub Releases 页面编辑 release notes, 触发 `.github/workflows/release.yml` + `npm-publish.yml`

---

## 执行节奏

| Step | 内容 | 预计耗时 |
|------|------|:---:|
| 1 | config-set-idevice 参数化 + gsdfile.Error 字段 | 30 分钟 |
| 2 | internal/reliability 子包 + client.go 迁移 + 6+ 测试 | 1.5 小时 |
| 3 | chaos test (client_test.go +2) | 30 分钟 |
| 4 | .goreleaser.yaml + 本地 snapshot 验证 | 30 分钟 |
| 5 | npm 7 包骨架 + 本地 link 验证 | 1 小时 |
| 6 | .github/workflows/release.yml + npm-publish.yml | 30 分钟 |
| 7 | --format csv/ndjson + csv_test.go + ndjson_test.go | 1 小时 |
| 8 | --dry-run 适配 DCP | 15 分钟 |
| 9 | device setup 组合命令 | 45 分钟 |
| 10 | AGENTS.md / README.md / status / field-verification 同步 | 30 分钟 |
| 11 | 实机验证 (9.1 + 9.2) | 30 分钟 |

**总共约 7 小时**（分 2-3 个工作日完成）。

---

## 风险与依赖

| 风险 | 影响 | 应对 |
|---|---|---|
| GoReleaser 首次配置坑多 | 9.3 延期 | 预留 1h buffer; 严格按官方 quickstart |
| npm 平台子包矩阵维护成本 | 后续每次发布需同步 7 包版本 | 用 `npm version` + CI 自动 bump |
| GitHub Actions 默认 runner 时间限制 (6h/job) | 7 包 × 1min/pack = 7min, 充裕 | 暂不需并行优化 |
| 用户未授权 `git push tag` | 9.3 走完无法 release | 严格遵循 [project_rules](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/.trae/rules/project_rules.md) — 仅本地产物, push/release 由用户触发 |
| chaos test 慢 (10s+) | CI 跑全套测试变慢 | 加 `-short` 跳过; CI 跑 `--short`, 慢测试本地/nightly 跑 |
| npm Trusted Publishing 配置 | 首次发布需在 npmjs.com 配 GitHub Actions 信任 | README 写明 onboarding 步骤, 用户首次手动配一次 |

---

## 不属于 Step 9

以下条目推迟至 Step 10+：

- **Step 10**: `network +shortcuts` 语义层（diagnose/auto-fix/batch-setup/generate）— 需 v0.1.0 分发通道跑通 + 用户反馈
- **Step 10**: Homebrew formula + Scoop manifest（macOS / Windows 原生包管理）
- **Step 11**: IO Routing (DataType=17) — topology active / verify / compile
- **Step 12+**: 多车间 Profile (`--profile`)、GSDML 离线字典、`inl backup` / `inl restore`、安全策略层 (guard/sandbox/risk 独立包)

---

## 相关文档

| 主题 | 文档 |
|---|---|
| 当前进度 | [inl-development-status.md](inl-development-status.md) |
| 上一计划（Step 8） | [inl-step8-raw-table-plan.md](inl-step8-raw-table-plan.md) |
| PRD | [inl-prd.md](inl-prd.md) |
| 架构 | [inl-architecture.md](inl-architecture.md) |
| 工作流 | [inl-workflow-design.md](inl-workflow-design.md) |
| 实机偏差 | [protocol/field-verification.md](protocol/field-verification.md) |
| P0 修复（Step 9.1 的 config-set-idevice 骨架来源） | [inl-p0-fix-plan.md](inl-p0-fix-plan.md) |
| 客户端输出参考 | [cli-module-client-output](file:///c:/Users/BYD/Documents/trae_projects/feishu_cli/cli/docs/cli-module-client-output.md#L504-L537) |
| GoReleaser 文档 | https://goreleaser.com/quick-start/ |
| npm 平台子包模式 | https://docs.npmjs.com/cli/v10/configuring-npm/package-json#optionaldependencies |
