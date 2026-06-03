// Package output 提供结构化错误输出, 借鉴 lark-cli output.Errorf 模式。
//
// AI Agent 可通过 type / code / message / hint 字段以结构化方式解析错误,
// 便于自动决策与提示。
package output

import (
	"encoding/json"
	"fmt"
	"io"
)

// Error 是 inl 统一的结构化错误类型。
//   - Type  : "validation" / "permission" / "protocol" ...
//   - Code  : "yes_required" / "sync_mismatch" / "crc_mismatch" ...
//   - Message: 人类可读的错误描述
//   - Hint  : 修复建议
//   - Detail: 任意附加上下文 (key-value)
type Error struct {
	Type    string         `json:"type"`
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Hint    string         `json:"hint,omitempty"`
	Detail  map[string]any `json:"detail,omitempty"`
}

// Error 让 Error 实现 error 接口, 返回 JSON 序列化字符串。
// 序列化失败时回退到基础字段拼接, 避免再次 panic。
func (e *Error) Error() string {
	buf, err := json.Marshal(e)
	if err != nil {
		return e.Type + ":" + e.Code + ": " + e.Message
	}
	return string(buf)
}

// WriteError 将 Error 缩进 JSON 后写入 w, 并补一个换行。
func WriteError(w io.Writer, err *Error) {
	if err == nil {
		return
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(err)
}

// YesRequired 是 write 操作的 --yes 强制错误。
// AI 看到此错误应自动追加 --yes 重试。
//
// 借鉴 lark-cli: 任何非 read 命令都强制 --yes (cli/cmd/service/service.go:182-185)
func YesRequired(action string) error {
	return &Error{
		Type:    "validation",
		Code:    "yes_required",
		Message: fmt.Sprintf("拒绝执行: %s 是 write 操作, 需加 --yes 标志确认", action),
		Hint:    fmt.Sprintf("查看风险: inl %s --yes --help", action),
		Detail: map[string]any{
			"action":            action,
			"risk_level":        "write",
			"roll_back_command": fmt.Sprintf("inl --target <IP> %s --yes --data '{...反操作...}'", action),
		},
	}
}

// ConfirmationRequired 是 high-risk-write 操作的人工确认错误。
// AI 看到此错误应**暂停**, 不自动追加 --yes, 必须人工决策。
//
// 借鉴 lark-cli: cli/internal/cmdutil/confirm.go:29-41 RequireConfirmation
func ConfirmationRequired(action string) error {
	return &Error{
		Type:    "validation",
		Code:    "confirmation_required",
		Message: fmt.Sprintf("⏸ %s 是高危操作, 需人工确认 (AI 不可自动追加 --yes)", action),
		Hint:    "请用户明确回复 yes/no 后再执行",
		Detail: map[string]any{
			"action":            action,
			"risk_level":        "high-risk-write",
			"ai_auto_yes":       false,
			"roll_back_command": "备份恢复: scp <IP>:./communication/Profinet/networktopology.json ./tmp/ && scp ./tmp/networktopology.json <IP>:./communication/Profinet/ && inl --target <IP> config compile --yes",
		},
	}
}
