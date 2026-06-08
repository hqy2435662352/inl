package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestFormatNDJSON_HeadersOnly(t *testing.T) {
	var buf bytes.Buffer
	if err := FormatNDJSON(&buf, []string{"Mac", "Name"}, nil); err != nil {
		t.Fatalf("FormatNDJSON err: %v", err)
	}
	want := "[\"Mac\",\"Name\"]\n"
	if buf.String() != want {
		t.Errorf("buf = %q, want %q", buf.String(), want)
	}
}

func TestFormatNDJSON_JqCompatible(t *testing.T) {
	cols := []string{"Mac", "Name"}
	rows := [][]string{
		{"00:11:22:33:44:55", "heron-weld"},
		{"aa:bb:cc:dd:ee:ff", "smc-valve"},
	}
	var buf bytes.Buffer
	if err := FormatNDJSON(&buf, cols, rows); err != nil {
		t.Fatalf("FormatNDJSON err: %v", err)
	}

	// 期望输出可被 jq 直接消费:
	//   ["Mac","Name"]
	//   {"Mac":"00:11:22:33:44:55","Name":"heron-weld"}
	//   {"Mac":"aa:bb:cc:dd:ee:ff","Name":"smc-valve"}
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("lines = %d, want 3", len(lines))
	}
	if lines[0] != `["Mac","Name"]` {
		t.Errorf("line 0 = %q, want %q", lines[0], `["Mac","Name"]`)
	}

	// 验证每行是合法 JSON
	for i, line := range lines {
		var v interface{}
		if err := json.Unmarshal([]byte(line), &v); err != nil {
			t.Errorf("line %d 不是合法 JSON: %v (line: %q)", i, err, line)
		}
	}
}

func TestFormatNDJSON_RowWidthMismatch(t *testing.T) {
	cols := []string{"A", "B"}
	rows := [][]string{{"only-a"}}
	var buf bytes.Buffer
	err := FormatNDJSON(&buf, cols, rows)
	if err == nil {
		t.Error("期望行宽不一致错误, got nil")
	}
}

func TestFormatNDJSON_EmptyColumns(t *testing.T) {
	var buf bytes.Buffer
	if err := FormatNDJSON(&buf, nil, nil); err != nil {
		t.Errorf("空 columns 不应报错, got: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("空 columns 不应有输出, got: %q", buf.String())
	}
}
