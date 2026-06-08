package output

import (
	"bytes"
	"strings"
	"testing"
)

// === ExtractTableRows 测试 ===

// TestExtractTableRows_DeviceArray 模拟 DataType=13 gsd-list 响应, 顶层 Device[] 数组。
func TestExtractTableRows_DeviceArray(t *testing.T) {
	data := []byte(`{
		"DataType": 13,
		"Device": [
			{"VendorID": 909, "VendorName": "Siemens AG", "DeviceID": 1, "GSDName": "GSDML-V2.35-Siemens-002A"},
			{"VendorID": 131, "VendorName": "Beckhoff", "DeviceID": 2, "GSDName": "GSDML-V2.31-Beckhoff-0083"}
		]
	}`)

	cols, rows, err := ExtractTableRows(data)
	if err != nil {
		t.Fatalf("ExtractTableRows err: %v", err)
	}
	if len(cols) != 4 {
		t.Errorf("cols = %v, want 4 cols", cols)
	}
	if len(rows) != 2 {
		t.Errorf("rows len = %d, want 2", len(rows))
	}
	// 验证按 JSON 出现顺序保留列名
	wantOrder := []string{"VendorID", "VendorName", "DeviceID", "GSDName"}
	for i, c := range wantOrder {
		if cols[i] != c {
			t.Errorf("cols[%d] = %q, want %q (JSON 顺序)", i, cols[i], c)
		}
	}
	if rows[0][0] != "909" {
		t.Errorf("rows[0][0] = %q, want 909", rows[0][0])
	}
	if rows[0][1] != "Siemens AG" {
		t.Errorf("rows[0][1] = %q, want Siemens AG", rows[0][1])
	}
	if rows[1][2] != "2" {
		t.Errorf("rows[1][2] = %q, want 2", rows[1][2])
	}
}

// TestExtractTableRows_DevicesArray 模拟 DataType=14 topology-scan 响应, Devices[] 数组。
func TestExtractTableRows_DevicesArray(t *testing.T) {
	data := []byte(`{
		"DataType": 14,
		"Devices": [
			{"Mac": "00:11:22:33:44:55", "DeviceName": "heron-weld", "IPAddress": "192.168.2.10", "VendorID": "0x038A", "DeviceRole": "PN设备"},
			{"Mac": "aa:bb:cc:dd:ee:ff", "DeviceName": "smc-valve", "IPAddress": "192.168.2.20", "VendorID": "0x0083", "DeviceRole": "PN设备"}
		]
	}`)

	cols, rows, err := ExtractTableRows(data)
	if err != nil {
		t.Fatalf("ExtractTableRows err: %v", err)
	}
	if len(cols) != 5 {
		t.Errorf("cols = %v, want 5 cols", cols)
	}
	if len(rows) != 2 {
		t.Errorf("rows len = %d, want 2", len(rows))
	}
	// 按 JSON 出现顺序保留
	wantOrder := []string{"Mac", "DeviceName", "IPAddress", "VendorID", "DeviceRole"}
	for i, c := range wantOrder {
		if cols[i] != c {
			t.Errorf("cols[%d] = %q, want %q (JSON 顺序)", i, cols[i], c)
		}
	}
	if rows[0][0] != "00:11:22:33:44:55" {
		t.Errorf("rows[0][0] = %q, want MAC", rows[0][0])
	}
	if rows[1][3] != "0x0083" {
		t.Errorf("rows[1][3] = %q, want 0x0083", rows[1][3])
	}
}

// TestExtractTableRows_NestedDevices 模拟 DataType=12 GetActRun 响应, Function.Devices[] 嵌套数组。
func TestExtractTableRows_NestedDevices(t *testing.T) {
	data := []byte(`{
		"DataType": 12,
		"Function": {
			"Value": "GetActRun",
			"Devices": [
				{"DeviceName": "heron-weld-01", "Status": "运行", "Current": 234.5},
				{"DeviceName": "heron-weld-02", "Status": "待机", "Current": 0.0}
			]
		}
	}`)

	cols, rows, err := ExtractTableRows(data)
	if err != nil {
		t.Fatalf("ExtractTableRows err: %v", err)
	}
	if len(cols) != 3 {
		t.Errorf("cols = %v, want 3 cols", cols)
	}
	if len(rows) != 2 {
		t.Errorf("rows len = %d, want 2", len(rows))
	}
	// 嵌套数组中, Devices[] 里的列按 JSON 出现顺序
	wantOrder := []string{"DeviceName", "Status", "Current"}
	for i, c := range wantOrder {
		if cols[i] != c {
			t.Errorf("cols[%d] = %q, want %q (JSON 顺序)", i, cols[i], c)
		}
	}
	if rows[0][2] != "234.5" {
		t.Errorf("rows[0][2] = %q, want 234.5", rows[0][2])
	}
	if rows[1][1] != "待机" {
		t.Errorf("rows[1][1] = %q, want 待机", rows[1][1])
	}
}

// TestExtractTableRows_NoArray 模拟 device-list 响应, 无任何对象数组 (CallBackJson 只有 PNDriver + IDevice)。
// 期望: cols == nil, 调用方应回退 JSON 输出。
func TestExtractTableRows_NoArray(t *testing.T) {
	data := []byte(`{
		"DataType": 12,
		"Function": {"Value": "CallBackJson"},
		"PNDriver": {"Name": "PN-Driver-1", "Version": "1.0"},
		"IDevice": {"IDeviceName": "iDevice-1"}
	}`)

	cols, rows, err := ExtractTableRows(data)
	if err != nil {
		t.Fatalf("ExtractTableRows err: %v", err)
	}
	if cols != nil {
		t.Errorf("cols 应为 nil, got %v", cols)
	}
	if rows != nil {
		t.Errorf("rows 应为 nil, got %v", rows)
	}
}

// TestExtractTableRows_EmptyArray 模拟 Devices: [] 空数组。
// 期望: cols == nil (与无数组相同处理)。
func TestExtractTableRows_EmptyArray(t *testing.T) {
	data := []byte(`{"DataType": 14, "Devices": []}`)

	cols, rows, err := ExtractTableRows(data)
	if err != nil {
		t.Fatalf("ExtractTableRows err: %v", err)
	}
	if cols != nil {
		t.Errorf("cols 应为 nil (空数组), got %v", cols)
	}
	if rows != nil {
		t.Errorf("rows 应为 nil (空数组), got %v", rows)
	}
}

// TestExtractTableRows_InvalidJSON 测试非法 JSON 输入。
func TestExtractTableRows_InvalidJSON(t *testing.T) {
	data := []byte(`{not valid json`)

	_, _, err := ExtractTableRows(data)
	if err == nil {
		t.Error("期望 JSON 解析错误, got nil")
	}
}

// TestExtractTableRows_MissingField 测试数组元素的字段缺失 (部分元素少字段), 应填充 "-"。
func TestExtractTableRows_MissingField(t *testing.T) {
	data := []byte(`{
		"DataType": 14,
		"Devices": [
			{"Mac": "00:11:22:33:44:55", "DeviceName": "heron-weld", "IPAddress": "192.168.2.10"},
			{"Mac": "aa:bb:cc:dd:ee:ff", "IPAddress": "192.168.2.20"}
		]
	}`)

	cols, rows, err := ExtractTableRows(data)
	if err != nil {
		t.Fatalf("ExtractTableRows err: %v", err)
	}
	if len(cols) != 3 {
		t.Fatalf("cols = %v, want 3", cols)
	}
	if rows[1][1] != "-" {
		t.Errorf("rows[1][1] (缺失 DeviceName) = %q, want -", rows[1][1])
	}
}

// === FormatTable 测试 ===

// TestFormatTable 验证基本渲染: 列宽对齐, 长字符串截断, nil 显示为 -。
func TestFormatTable(t *testing.T) {
	cols := []string{"MAC", "Name", "IP"}
	rows := [][]string{
		{"00:11:22:33:44:55", "heron-weld", "192.168.2.10"},
		{"aa:bb:cc:dd:ee:ff", "", "192.168.2.20"},
		{"", "GSDML-V2.35-Siemens-002A-S7-1500-20170801.xml", "192.168.2.30"},
	}

	var buf bytes.Buffer
	if err := FormatTable(&buf, cols, rows); err != nil {
		t.Fatalf("FormatTable err: %v", err)
	}

	out := buf.String()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("lines = %d, want 4 (header + 3 rows)", len(lines))
	}

	// 表头应包含列名
	if !strings.Contains(lines[0], "MAC") {
		t.Errorf("header missing MAC: %q", lines[0])
	}
	if !strings.Contains(lines[0], "Name") {
		t.Errorf("header missing Name: %q", lines[0])
	}
	if !strings.Contains(lines[0], "IP") {
		t.Errorf("header missing IP: %q", lines[0])
	}

	// 第 2 行: 第二个 Name 为空 → "-"
	if !strings.Contains(lines[1], "-") {
		t.Errorf("row 1 missing - placeholder: %q", lines[1])
	}

	// 第 3 行: 长 GSDName 应被截断 (max=40 → 37 + "...")
	if !strings.Contains(lines[3], "...") {
		t.Errorf("row 2 long GSDName 未截断: %q", lines[3])
	}
}

// TestFormatTable_ColumnWidthAlignment 验证每行长度相同 (对齐)。
func TestFormatTable_ColumnWidthAlignment(t *testing.T) {
	cols := []string{"A", "BB", "CCC"}
	rows := [][]string{
		{"x", "yy", "zzz"},
		{"longer", "value", "data"},
	}

	var buf bytes.Buffer
	if err := FormatTable(&buf, cols, rows); err != nil {
		t.Fatalf("FormatTable err: %v", err)
	}

	out := strings.TrimRight(buf.String(), "\n")
	lines := strings.Split(out, "\n")
	if len(lines) != 3 {
		t.Fatalf("lines = %d, want 3", len(lines))
	}
	// 所有行应有相同长度 (左对齐 + pad 固定)
	if len(lines[0]) != len(lines[1]) || len(lines[1]) != len(lines[2]) {
		t.Errorf("行长度不一致: %d, %d, %d", len(lines[0]), len(lines[1]), len(lines[2]))
	}
}

// TestFormatTable_EmptyColumns 空列应该立即返回 nil。
func TestFormatTable_EmptyColumns(t *testing.T) {
	var buf bytes.Buffer
	if err := FormatTable(&buf, nil, nil); err != nil {
		t.Errorf("FormatTable(nil, nil) err: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("应无输出, got %q", buf.String())
	}
}

// TestFormatTable_RowWidthMismatch 行宽与列数不一致应返回 error。
func TestFormatTable_RowWidthMismatch(t *testing.T) {
	cols := []string{"A", "B"}
	rows := [][]string{
		{"x", "y"},
		{"z"}, // 缺一列
	}

	var buf bytes.Buffer
	if err := FormatTable(&buf, cols, rows); err == nil {
		t.Error("期望 row width mismatch error, got nil")
	}
}

// TestTruncate 验证截断逻辑。
func TestTruncate(t *testing.T) {
	tests := []struct {
		in   string
		max  int
		want string
	}{
		{"short", 10, "short"},
		{"this is a long string that should be truncated", 10, "this is..."},
		{"abc", 3, "abc"}, // max <= 3 时不截断 (避免 "..." 比原文还长)
		{"abcd", 3, "abcd"},
		{"hello", 5, "hello"},
		{"hello!", 5, "he..."},
	}
	for _, tt := range tests {
		got := truncate(tt.in, tt.max)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.in, tt.max, got, tt.want)
		}
	}
}
