package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestError_ErrorString(t *testing.T) {
	e := &Error{
		Type:    "validation",
		Code:    "yes_required",
		Message: "拒绝执行: config-add-device 是 write 操作, 需加 --yes 标志确认",
		Hint:    "查看风险: inl config-add-device --yes --help",
	}
	got := e.Error()

	var parsed map[string]any
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("Error() 输出不是合法 JSON: %v (raw=%q)", err, got)
	}
	if parsed["type"] != "validation" {
		t.Errorf("type = %v, want validation", parsed["type"])
	}
	if parsed["code"] != "yes_required" {
		t.Errorf("code = %v, want yes_required", parsed["code"])
	}
	if parsed["message"] == nil {
		t.Error("message 字段缺失")
	}
	if parsed["hint"] == nil {
		t.Error("hint 字段缺失")
	}
}

func TestError_OmitsEmptyHintAndDetail(t *testing.T) {
	e := &Error{
		Type:    "protocol",
		Code:    "crc_mismatch",
		Message: "CRC 校验失败",
	}
	got := e.Error()
	if strings.Contains(got, "hint") {
		t.Errorf("Hint 为空时不应出现在 JSON 中, got: %s", got)
	}
	if strings.Contains(got, "detail") {
		t.Errorf("Detail 为空时不应出现在 JSON 中, got: %s", got)
	}
}

func TestWriteError_IndentedJSON(t *testing.T) {
	var buf bytes.Buffer
	err := &Error{
		Type:    "validation",
		Code:    "yes_required",
		Message: "需要 --yes 确认",
	}
	WriteError(&buf, err)
	out := buf.String()

	if !strings.Contains(out, "\"type\": \"validation\"") {
		t.Errorf("WriteError 输出未含缩进 type, got: %q", out)
	}
	if !strings.Contains(out, "\n") {
		t.Errorf("WriteError 输出应补换行, got: %q", out)
	}
}

func TestWriteError_NilSafe(t *testing.T) {
	var buf bytes.Buffer
	WriteError(&buf, nil)
	if buf.Len() != 0 {
		t.Errorf("WriteError(nil) 不应写任何内容, got: %q", buf.String())
	}
}

func TestYesRequired(t *testing.T) {
	err := YesRequired("config-add-device")
	if err == nil {
		t.Fatal("expected non-nil")
	}
	got := err.Error()
	if !strings.Contains(got, "yes_required") {
		t.Errorf("code 缺失 yes_required, got: %s", got)
	}
	if !strings.Contains(got, "write") {
		t.Errorf("risk_level 缺失 write, got: %s", got)
	}
	if !strings.Contains(got, "config-add-device") {
		t.Errorf("action 缺失, got: %s", got)
	}
}

func TestConfirmationRequired(t *testing.T) {
	err := ConfirmationRequired("config-compile")
	if err == nil {
		t.Fatal("expected non-nil")
	}
	got := err.Error()
	if !strings.Contains(got, "confirmation_required") {
		t.Errorf("code 缺失 confirmation_required, got: %s", got)
	}
	if !strings.Contains(got, "high-risk-write") {
		t.Errorf("risk_level 缺失 high-risk-write, got: %s", got)
	}
	if !strings.Contains(got, "ai_auto_yes") {
		t.Errorf("ai_auto_yes 字段缺失, got: %s", got)
	}
	if !strings.Contains(got, "AI 不可自动追加") {
		t.Errorf("Message 缺少 AI 不可自动追加 提示, got: %s", got)
	}
}
