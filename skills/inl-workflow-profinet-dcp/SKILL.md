---
name: inl-workflow-profinet-dcp
version: 1.0.0
description: "DCP 写操作独立 skill：device setup-name / setup-ip 简化 3 步流程（预检→确认→验证）。独立于完整配网流程，适合现场临时改在线设备的设备名/IP。"
metadata:
  requires:
    bins: ["inl"]
    skills: ["inl-shared"]
---

# inl DCP 写操作独立工作流

**CRITICAL — 开始前 MUST 先用 Read 工具读取 [`../inl-shared/SKILL.md`](../inl-shared/SKILL.md)**，其中包含 `--target` / `--yes` / Risk 等级 / 结构化错误等共享约定。

---

## 适用场景

AI Agent 收到下列**不涉及完整配网编排**的 DCP 写任务时，可独立触发本 Skill：

- "把焊机 X 的名字改成 Y" → `inl device setup-name`
- "把焊机 X 的 IP 改成 192.168.2.30" → `inl device setup-ip`

## 不适用场景

- ❌ 涉及 config 组命令（add-device / remove-device / set-driver 等）→ 走 `inl-workflow-profinet-config`
- ❌ 完整配网任务 → 走 `inl-workflow-profinet-config`
- ❌ 纯读操作（`device list` / `topology scan`）→ 直接用读命令

---

## 前置条件

1. ✅ **已读 [`../inl-shared/SKILL.md`](../inl-shared/SKILL.md)**
2. ✅ **已知 `--target` 工业 PC IP**
3. ✅ **已知目标设备的 MAC 地址**（从 `topology scan` 获取）
4. ✅ **已知 PROFINET 网络端口**（如 `enp4s0`）

---

## 执行流程

| # | 步骤 | 命令 | 说明 |
|:--:|------|------|------|
| 1 | 预检 | `inl --target <IP> topology scan --interface <port>` | 通过 DCP 扫描实际网络，确认目标设备 MAC 存在，新名称/IP 未被其他设备占用 |
| 2 | 执行 | `inl --target <IP> device setup-name --interface <port> --mac <MAC> --name <new_name> --yes` 或 `inl --target <IP> device setup-ip --interface <port> --mac <MAC> --ip <new_ip> --mask <subnet_mask> --yes` | 修改设备名称或 IP。`--yes` 由 AI 自动追加。 |
| 3 | 验证 | `sleep(5)` → `inl --target <IP> topology scan --interface <port>` | DCP 写操作**无 JSON 响应**，`topology scan` 是唯一确认手段。比对 scan 结果中目标设备的 DeviceName / IPAddress / SubnetMask 字段与预期一致。 |

### 注意事项

- **无 JSON 响应**：DCP 写操作执行后 Envelope 形如 `{"ok": true, "data": null, "_notice": {"dcp_write": true, ...}}`。`topology scan` 验证不可跳过。
- **MAC 定位设备**：DCP 写命令通过 MAC 地址定位设备，执行前必须从 `topology scan` 拿到目标设备的 MAC。
- **IP/Name 冲突检查**：执行前通过 `topology scan` 确认新名称/IP 未被其他在线设备占用。
- **改 IP 后失联**：改 IP 后需额外 `ping <新 IP>` 确认主站能 ping 通。
- **`topology scan` 为 table 输出**：`topology scan` 不支持 `--format json`，其输出为表格格式，解析时按列提取字段。

---

## 参考

- [`../inl-shared/SKILL.md`](../inl-shared/SKILL.md) — 共享规则（`--target` / `--yes` / Risk / 结构化错误码）
- [`../../inl/AGENTS.md`](../../inl/AGENTS.md) — inl 客户端权威开发文档
- [`../../inl/docs/inl-workflow-design.md`](../../inl/docs/inl-workflow-design.md) — 完整配网工作流设计（本 skill 为其中 Phase 7.3 的独立子集）
