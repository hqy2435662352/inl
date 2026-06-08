package output

import (
	"bytes"
	"testing"
)

func TestFormatCSV_HeadersOnly(t *testing.T) {
	var buf bytes.Buffer
	if err := FormatCSV(&buf, []string{"A", "B"}, nil); err != nil {
		t.Fatalf("FormatCSV err: %v", err)
	}
	want := "A,B\n"
	if buf.String() != want {
		t.Errorf("buf = %q, want %q", buf.String(), want)
	}
}

func TestFormatCSV_HandlesCommas(t *testing.T) {
	cols := []string{"Name", "IP"}
	rows := [][]string{{"heron, weld", "192.168.2.10"}}
	var buf bytes.Buffer
	if err := FormatCSV(&buf, cols, rows); err != nil {
		t.Fatalf("FormatCSV err: %v", err)
	}
	// RFC 4180: 行内逗号 → 整字段加双引号包裹
	want := "Name,IP\n\"heron, weld\",192.168.2.10\n"
	if buf.String() != want {
		t.Errorf("buf = %q, want %q", buf.String(), want)
	}
}

func TestFormatCSV_HandlesQuotes(t *testing.T) {
	cols := []string{"DeviceName"}
	rows := [][]string{{`she said "hi"`}}
	var buf bytes.Buffer
	if err := FormatCSV(&buf, cols, rows); err != nil {
		t.Fatalf("FormatCSV err: %v", err)
	}
	// RFC 4180: 双引号转义为 ""
	want := "DeviceName\n\"she said \"\"hi\"\"\"\n"
	if buf.String() != want {
		t.Errorf("buf = %q, want %q", buf.String(), want)
	}
}

func TestFormatCSV_HandlesNewlines(t *testing.T) {
	cols := []string{"Note"}
	rows := [][]string{{"line1\nline2"}}
	var buf bytes.Buffer
	if err := FormatCSV(&buf, cols, rows); err != nil {
		t.Fatalf("FormatCSV err: %v", err)
	}
	// RFC 4180: 字段含换行 → 加双引号包裹, 内部换行保留
	want := "Note\n\"line1\nline2\"\n"
	if buf.String() != want {
		t.Errorf("buf = %q, want %q", buf.String(), want)
	}
}

func TestFormatCSV_MultipleRows(t *testing.T) {
	cols := []string{"Mac", "Name"}
	rows := [][]string{
		{"00:11:22:33:44:55", "heron-weld"},
		{"aa:bb:cc:dd:ee:ff", "smc-valve"},
	}
	var buf bytes.Buffer
	if err := FormatCSV(&buf, cols, rows); err != nil {
		t.Fatalf("FormatCSV err: %v", err)
	}
	want := "Mac,Name\n00:11:22:33:44:55,heron-weld\naa:bb:cc:dd:ee:ff,smc-valve\n"
	if buf.String() != want {
		t.Errorf("buf = %q, want %q", buf.String(), want)
	}
}

func TestFormatCSV_RowWidthMismatch(t *testing.T) {
	cols := []string{"A", "B"}
	rows := [][]string{{"only-a"}} // 缺 B
	var buf bytes.Buffer
	err := FormatCSV(&buf, cols, rows)
	if err == nil {
		t.Error("期望行宽不一致错误, got nil")
	}
}

func TestFormatCSV_EmptyColumns(t *testing.T) {
	var buf bytes.Buffer
	if err := FormatCSV(&buf, nil, nil); err != nil {
		t.Errorf("空 columns 不应报错, got: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("空 columns 不应有输出, got: %q", buf.String())
	}
}
