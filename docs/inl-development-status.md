---
title: inl 开发现状报告
tags: [inl, status, development, report]
created: 2026-06-04
status: published
---

# inl — 开发现状报告

## 一、项目总体进度

| 维度 | 指标 | 说明 |
|------|:--:|------|
| 总体完成度 | **100%** (L0-L2) / **95%** (含 L3 离线验证) | 核心功能 100%、输出体系 100%、稳定性 100%、分发 100%、文档 100%；**Step 10.B 修复 5 个 inl Bug + 2 个 C++ Bug，SMC EX245 全链路 L2 实机通过**；L3 compile 离线验证通过 (实机待有编译环境的 PC) |
| 命令总数 | **25 条 + 1 组合** | gsd=2, device=7+setup, config=12 (含 set-idevice 参数化), interface=1, topology=1, schema=1, raw=1 |
| 测试覆盖 | **11 包全部 PASS** | go vet 零警告；新增 reliability 10 测试 + configresp 12 测试 + main 包 `TestIsDCPWriteClosedConnection` 7 真值表 + `TestIsNilSlice` 5 真值表 = 34 新测试 |
| Skill 体系 | **3/3** | inl-shared + inl-workflow-profinet-write + inl-workflow-profinet-config (后者增"禁止臆想 DAP_ID/ModuleID"规则) |
| 设计文档 | **19 份** | PRD / 架构 / 工作流 / 9 份 step plan / P0 修复 / field-verification (含 12 节 v3 修复后实机核对) / step10-config-write-validation-plan / config-field-reference / 字段参照 |
| 实机验证 | **L0-L2 全部通过** | 工业 PC 192.168.3.15 SMC EX245 pipeline (add-device → add-module×2 → add-submodule → set-driver → set-idevice → 全回滚) ✅. L3 compile 离线验证 |
| DCP 稳定性实测 | **已通过** | 2026-06-04 连续高频率 DCP 读操作稳定性测试通过 |
| 跨平台分发 | **已就绪（v0.1.0 待 L3 实机补测后触发）** | GoReleaser + GitHub Actions + npm 7 包就绪, v0.1.0 tag **阻塞在 L3 compile 实机补测**（用户决策 2026-06-08：不着急发布） |
| GitHub | **已推送** | https://github.com/hqy2435662352/inl |

### 分阶段完成情况

| 阶段 | 内容 | 状态 | 验收 |
|------|------|:--:|:---:|
| Step 1 | MVP — NRC 帧编解码 + TCP 客户端 | ✅ | CRC32 与 PDF 一致 |
| Step 2 | 命令骨架 — gsd list + topology + device | ✅ | 15 命令 Registry |
| Step 3 | 协议对齐 — Cobra 树 + DryRun + Risk | ✅ | 17 命令, `--yes` 机制 |
| Step 4 | AI Skills — inl-shared + write | ✅ | 2 Skills, AGENTS.md |
| P0 fix | 领域模型修正 + SetIDevice | ✅ | 23 命令, CallbackJsonResponse |
| Step 5 | DCP 命令 — interface/topology/gsd/device setup | ✅ | 22+1 命令, 实机验证 |
| Step 6 | JSON Envelope — `{ok, data, _notice}` | ✅ | 4/4 实机验证 |
| Step 7 | schema list — AI 自发现 | ✅ | 纯客户端命令 |
| Step 7.1 | AGENTS.md 精简 + 文档搬入仓库 | ✅ | 585→104 行 |
| Step 8 | raw send + `--format table` | ✅ | 24 命令, 实机验证 |
| Step 9 | 生产化与发布 | ✅ | 10 包, v0.1.0 cross-platform |
| Step 10.1 | 参数化 (10.A 子任务) | ✅ (2026-06-04 完成) | 11 BodyBuilder + configresp 子包 + 34 新测试 PASS, 离线可跑 |
| Step 10.2 | 实机验证 (10.B 子任务) | ✅ (2026-06-08 完成, L3 待补) | SMC EX245 L2 pipeline 全通过, 5 inl + 2 C++ Bug 全部修复, L3 compile 离线验证 |

---

## 二、各子模块详细进展

### 2.1 NRC 协议通信（internal/nrc/）

| 文件 | 功能 | 状态 |
|------|------|:--:|
| `frame.go` | 帧编解码 (SyncByte 0x4E66 + CRC32 IEEE 802.3) | ✅ |
| `client.go` | TCP 客户端 (5s 连接超时, 10s 读写超时, `SendReceiveFiltered`) | ✅ |
| `commands.go` | 25 条 Registry + 6 Group 常量 + 7 BodyBuilder | ✅ |
| `annotation.go` | 4 个 Cobra Annotations 常量 | ✅ |
| `commands_test.go` | Registry 唯一性 + BodyBuilder + Group 分布测试 (~35 条) | ✅ |
| `frame_test.go` | 帧编解码 + 已知 PDF CRC 验证 | ✅ |

### 2.2 输出系统（internal/output/）

| 文件 | 功能 | 状态 |
|------|------|:--:|
| `errors.go` | 结构化错误 (5 字段: type/code/message/hint/detail) | ✅ |
| `envelope.go` | 统一 stdout JSON 信封 `{ok, data, _notice}` | ✅ |
| `dryrun.go` | DryRunFrame 预览 + PrintDryRunFrame | ✅ |
| `table.go` | ExtractTableRows (数组自动探测) + FormatTable (对齐渲染) | ✅ |
| `csv.go` | FormatCSV (RFC 4180) — Step 9.4 | ✅ |
| `ndjson.go` | FormatNDJSON (header + 对象行) — Step 9.4 | ✅ |
| `*_test.go` | 所有功能有对应测试 | ✅ |

### 2.6 可靠性子包（internal/reliability/）— Step 9.2 新增

| 文件 | 功能 | 状态 |
|------|------|:--:|
| `retry.go` | `Policy{ MaxRetries, BaseBackoff, MaxBackoff, Jitter, ShouldRetry }` + `Default()` + `ShouldRetryNetwork` | ✅ |
| `doc.go` | 包注释 + 用法说明 | ✅ |
| `retry_test.go` | 10 测试 (Do / 退避 / 触发 / context / nil policy) | ✅ |

### 2.3 领域模型（internal/）

| 包 | 模型 | 涵盖 DataType | 状态 |
|----|------|:--:|:--:|
| `gsd/` | 8 structs + MatchResponse | 13, 16 | ✅ |
| `topology/` | CallbackJsonResponse, ActivatedTopologyResponse + ScanResponse | 12, 14 | ✅ |
| `devicestatus/` | Response/Function/Device | 12 (GetActRun) | ✅ |
| `gsdfile/` | Response/Function/GSDFile | 12 (GetGSDFile*) | ✅ |
| `dcpdevice/` | DCPDevice 9 字段 + DCP Block 映射 | 14 (DCP 发现) | ✅ |
| `netiface/` | ListResponse + Flatten() | 14 (Func=4) | ✅ |

### 2.4 CLI 入口（main.go）

| 功能 | 状态 |
|------|:--:|
| Cobra 命令树 (7 个 Group 自动遍历 Registry) | ✅ |
| Risk 检查 (read/write/high-risk-write + `--yes`) | ✅ |
| `--dry-run` (构建帧不发送) | ✅ |
| `--format table` (自动数组探测 → 表格输出) | ✅ |
| `--format json` (Envelope 包裹) | ✅ |
| schema 短路 (纯客户端, 不连工业 PC) | ✅ |
| `--target` 支持 `IP:PORT` 自适应 | ✅ |
| DCP 写操作无响应处理 | ✅ |

### 2.5 AI Skills（skills/）

| Skill | 行数 | 依赖 | 覆盖 |
|------|:--:|------|------|
| `inl-shared` | ~150 | bins:["inl"] | Risk/--yes/--dry-run/错误码/Envelope |
| `inl-workflow-profinet-write` | ~250 | shared | 4 层安全: 备份→预检→写入→编译 |
| `inl-workflow-profinet-config` | ~370 | shared+write | 8 阶段编排: 评估→发现→规划→验证 |

### 2.6 文档

| 文档 | 状态 |
|------|:--:|
| PRD | ✅ 已同步 Step 8 进度 |
| 架构设计 | ✅ 已同步 Step 8 进度 |
| 工作流设计 | ✅ 完整 (Phase 1-8, 状态机, 错误处理) |
| AGENTS.md | ✅ 精简为 104 行 (Source Layout + Conventions + Where to Find) |
| README.md | ✅ 命令总览 + 架构图 + 快速开始 |
| field-verification.md | ✅ 实机偏差登记 |
| 开发计划 (step1-step8) | ✅ 8 份全部归档 |
| P0 修复计划 | ✅ 已归档 |
| 本报告 | ✅ 生成中 |

---

## 三、25 条命令明细

| # | 命令 | 协议 | 风险 | 状态 |
|:--:|------|------|:--:|:--:|
| 1 | `gsd list` | DataType=13 | read | ✅ |
| 2 | `gsd match` | DataType=16 | read | ✅ |
| 3 | `device list` | DataType=12, CallBackJson | read | ✅ |
| 4 | `device list-active` | DataType=12, CallBackActivatedJson | read | ✅ |
| 5 | `device run` | DataType=12, GetActRun | read | ✅ |
| 6 | `device gsd-config` | DataType=12, GetGSDFileNetwork | read | ⚠️ |
| 7 | `device gsd-active` | DataType=12, GetGSDFileActivated | read | ⚠️ |
| 8 | `device setup-name` | DataType=14, Func=2 | write | ✅ |
| 9 | `device setup-ip` | DataType=14, Func=3 | write | ✅ |
| 10-21 | `config *` (12 条) | DataType=12 | write/高危 | ✅ |
| 22 | `topology scan` | DataType=14, Func=1 | read | ✅ |
| 23 | `interface list` | DataType=14, Func=4 | read | ✅ |
| 24 | `schema list` | 纯客户端 | read | ✅ |
| 25 | `raw send` | DataType=0 (透传) | write | ✅ |

> ⚠️ = nrc2.out 版本限制，非 inl 代码缺陷

---

## 四、待开发任务

| 优先级 | 任务 | 预估工作量 | 说明 |
|:---:|------|:---:|------|
| ~~P1~~ | ~~`config-set-idevice` 参数化~~ | ~~30min~~ | ✅ **2026-06-04 Step 9.1 完成**：`configSetIDeviceBody` + `--data` JSON 注入, 4 测试 PASS |
| ~~P2~~ | ~~`--format csv/ndjson`~~ | ~~1h~~ | ✅ **2026-06-04 Step 9.4 完成**：`output.FormatCSV` / `FormatNDJSON` + 8 测试 PASS, main.go 接入 |
| ~~P2~~ | ~~`device-gsd-config` Error 模型修正~~ | ~~15min~~ | ✅ **2026-06-04 Step 9.2 完成**：`gsdfile.Response.Error` 字段已存在, field-verification.md 标记为已修正 |
| ~~P2~~ | ~~npm + GoReleaser 分发~~ | ~~2h~~ | ✅ **2026-06-04 Step 9.3 完成**：`.goreleaser.yaml` + 2 GitHub Actions + 7 npm 包骨架就绪, 待用户触发 v0.1.0 tag |
| ~~P2~~ | ~~稳定性正式化 (reliability)~~ | ~~1.5h~~ | ✅ **2026-06-04 Step 9.2 完成**：`internal/reliability/` 抽包 + 10 测试 + client.go 迁移 + `--retry` flag |
| ~~P3~~ | ~~`--dry-run` for DCP write~~ | ~~30min~~ | ✅ **2026-06-04 Step 9.4 完成**：`--dry-run` 已在 Risk 检查处拦截所有写命令 (含 DCP), 实测 DCP 写也走 dry-run |
| ~~P3~~ | ~~`inl device setup` 闭环验证~~ | ~~30min~~ | ✅ **2026-06-04 Step 9.4 完成**：`device setup` 组合命令, 顺序执行 setup-name + setup-ip + topology scan |
| **P0** | **L3 compile 实机补测 (v0.1.0 gate)** | **1-2h 现场** | ⏭️ **待工业 PC 有编译环境时执行**。需在 nrc2.out 编译环境可用的 PC（不是 192.168.3.15 产线 PC）上跑 `inl config compile --yes`，验证 controller 重启 + inl 重连 + `device list-active` 反映新拓扑。补测通过 + 用户显式确认后，才能打 v0.1.0 tag（按 project_rules） |
| **P0** | **`config_body.go` AddPNDevice 请求体 `DecentralDevice:null` → `[]` (Step 10.B 实机发现)** | **0.3h** | ✅ **2026-06-08 已修 (inl 侧)**: `isNilSlice(v)` 用 reflect 穿透判断, `DecentralDevice:null` → `[]any{}`, `TestIsNilSlice` 5 真值表 PASS. 修复后 L2 add-device 实机通过 |
| **P0** | **`config_body.go` 步骤 4 fetch 覆盖 PNDriver (Step 10.B 实机发现)** | **0.2h** | ✅ **2026-06-08 已修 (inl 侧)**: `if _, already := body[k]; already { continue }` 防止自动 fetch 覆盖用户业务字段. 修复后 set-driver 成功修改 IP |
| **P0** | **`main.go` fetch 另建连接导致 C++ 关 socket (Step 10.B 实机发现)** | **0.5h** | ✅ **2026-06-08 已修 (inl 侧)**: `PreFetchTopology(client)` 复用同一连接 + `preFetchedTopology` 缓存, 写命令发得出去 |
| **P0** | **`topology/types.go` DecentralDevice 缺 DAP_ID 等字段 (Step 10.B 实机发现)** | **0.3h** | ✅ **2026-06-08 已修 (inl 侧)**: 补全 `DAP_ID`/`DAP_Name`/`VendorName`/`ModuleID`/`SubmoduleID` 等字段. C++ AddModule 用 DAP_ID 查 GSD 配置才能工作 |
| **C++** | **PNConfigLibFileDesign.cpp AddModule 诊断 + ModuleItemTarget fallback** | **0.3h** | ✅ **2026-06-08 已修 (C++ 侧)**: 加 `UseableModules size` 诊断日志 + `ModuleItemTarget` 字段名 fallback, 解决解析器字段名不一致 |
| **C++** | **PNConfigLibFileDesign.cpp SetIDevice loadJsonFromFile 修复** | **0.5h** | ✅ **2026-06-08 已修 (C++ 侧)**: `loadJsonFromFile` 加载现有拓扑 + 仅改 `IDevice` 字段, 不再清空 `PNDriver`/`DecentralDevice` |
| P3 | `network +shortcuts` 语义层 | 4-6h | diagnose/auto-fix/batch-setup/generate (Step 11) |

---

## 五、技术难点与解决方案

| 难点 | 已解决？ | 方案 |
|------|:---:|------|
| NRC 帧 CRC32 计算 | ✅ | PDF 已知帧 `0x53DDEB72` / `0x6B926DFF` 验证通过 |
| DataType=12/14 Function 类型冲突 | ✅ | DCP 命令用专用 BodyBuilder (整数 Function)，与 DefaultBodyBuilder (字符串对象) 隔离 |
| 工业 PC 异步推送污染响应 | ✅ | `SendReceiveFiltered` 按 command code + JSON DataType 双重过滤 |
| DCP 写操作无 JSON 响应 | ✅ | 连接关闭时判成功 + stderr 提示 "请用 topology scan 验证" |
| io-controller C++ 数组下标 bug | ✅ | 定位到 `pndcp.cpp:1375` `localdevice[i]` → `localdevice["Devices"][i]` |
| 不同 DataType 响应 JSON 字段名不一 | ✅ | `ExtractTableRows` 递归探测算法，兼容 Device[]/Devices[]/DecentralDevice[]/Function.Devices[] |
| `--yes` 与 high-risk-write 的 AI 自动决策边界 | ✅ | 三级 Risk，`yes_required` AI 可自动重试，`confirmation_required` AI 必须暂停 |
| Registry 混合 NRC 命令 + 纯客户端命令 | ✅ | DataType=0 哨兵值，`init()` 跳过冲突检查 |

---

## 六、风险评估

| 风险 | 概率 | 影响 | 应对 |
|------|:---:|------|------|
| 工业 PC nrc2.out 版本差异 | 中 | 部分命令无响应 (gsd-active) | AGENTS.md 偏差表登记 + 降级策略 |
| DCP 广播被交换机过滤 | 低 | topology scan 无结果 | stderr 提示检查网络设置 |
| `raw send` 被误用于高危操作 | 中 | 绕过 Registry 的安全分级 | 默认 RiskWrite，强制 `--yes` |
| 未来 nrc2.out 升级破坏协议兼容性 | 低 | 所有 DataType=12/14 命令失效 | field-verification.md 反馈循环可快速发现 |

---

## 七、后续开发计划

### 短期 (1-2 周)

> **Step 10 已完成**（2026-06-08）：所有短期任务清零。

| 任务 | 状态 |
|------|:--:|
| `config-set-idevice` 参数化 | ✅ Step 9.1 + 10.A 完成 |
| `device-gsd-config` Error 模型修正 | ✅ Step 9.2 完成 |
| npm + GoReleaser 分发 | ✅ Step 9.3 完成, 待 v0.1.0 tag |
| **Step 10 全部 12 条 config 实机验证** | ✅ 2026-06-08 SMC EX245 pipeline 通过 (L3 离线) |
| v0.1.0 tag + GitHub Release + npm publish | ⏭️ **待用户显式触发**（project_rules） |

### 中期 (1-2 月)

| 时间 | 任务 |
|------|------|
| 第 1-2 周 | v0.1.0 验收窗口（用户/产线用户试用 + 反馈收集） |
| 第 3-4 周 | L3 compile 在有编译环境的工业 PC 上补测 |
| 第 5-6 周 | `network +shortcuts` 语义层 (4 个 shortcut: diagnose/auto-fix/batch-setup/generate) |
| 第 7-8 周 | IO Routing (DataType=17) — topology active/verify/compile |

### 长期 (> 3 月)

- 多车间 Profile 管理 (`--profile`)
- GSDML JSON 离线字典 (客户端离线匹配)
- `inl backup` / `inl restore` 子命令
- 安全策略层 (guard/sandbox/risk 独立包)

---

## 八、资源状态

| 资源 | 状态 |
|------|------|
| 开发环境 | Windows + Go 1.24 + cobra v1.10.2 |
| 测试环境 | 工业 PC 192.168.3.15 (nrc2.out :6000) |
| 代码仓库 | https://github.com/hqy2435662352/inl (SSH) |
| 外部依赖 | cobra (仅一个)，所有 internal/ 包零外部依赖 |
| 测试数据 | ~50 个实机响应 JSON 样本 (testdata/) |

---

## 九、附录：文件清单

```
inl/                                   # ~80 个文件 (Step 9 后)
├── main.go, main_test.go              # CLI 入口 + 测试
├── AGENTS.md                          # AI 导览 (Step 9 更新)
├── README.md                          # 项目说明 (Step 9 加 npm install)
├── go.mod, go.sum                     # Go module (依赖仅 cobra)
├── .goreleaser.yaml                   # Step 9.3 跨平台构建配置
├── .gitignore                         # 排除响应文件 + 日志 + .exe
│
├── .github/workflows/                 # Step 9.3 GitHub Actions
│   ├── release.yml                    #   GoReleaser 自动 release
│   └── npm-publish.yml                #   npm 7 包发布
│
├── internal/                          # 11 个包
│   ├── nrc/        (8 文件)           # NRC 协议 + Registry + 客户端
│   ├── nrc/config_body.go + config_body_test.go  # Step 10.A 11 个 BodyBuilder + 35 测试
│   ├── output/    (10 文件)           # 错误 + Envelope + DryRun + Table + CSV + NDJSON
│   ├── reliability/(3 文件)           # Step 9.2 重试策略子包
│   ├── gsd/        (2 文件)           # DataType=13/16 模型
│   ├── topology/   (2 文件)           # DataType=12/14 拓扑模型
│   ├── devicestatus/(2 文件)          # GetActRun 模型
│   ├── gsdfile/    (2 文件)           # GSDML 文件模型
│   ├── dcpdevice/  (2 文件)           # DCP 发现设备模型
│   ├── netiface/   (2 文件)           # 端口列表模型
│   ├── configresp/                     # Step 10.A WriteResponse + ShieldDeviceResponse + 12 测试
│
├── npm/                               # Step 9.3 npm 薄壳包
│   ├── inl-cli/                       #   入口包
│   ├── inl-cli-linux-x64/             #   6 平台子包
│   ├── inl-cli-linux-arm64/
│   ├── inl-cli-darwin-x64/
│   ├── inl-cli-darwin-arm64/
│   ├── inl-cli-win32-x64/
│   └── inl-cli-win32-arm64/
│
├── skills/                            # 3 个 Skill
│   ├── inl-shared/
│   ├── inl-workflow-profinet-write/
│   └── inl-workflow-profinet-config/
│
├── docs/                              # 17 份文档
│   ├── inl-prd.md, inl-architecture.md, inl-workflow-design.md
│   ├── inl-step{1-9}-*.md, inl-p0-fix-plan.md
│   ├── inl-development-status.md      # 本报告
│   └── protocol/field-verification.md
│
└── testdata/                          # ~50 个实机响应样本
testdata/                              # 11+1 个 config 实机 fixture (10.B 后)
```
