package configresp

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestWriteResponse_HasErrors_StringErr (发现 F3 单值错误模型)
func TestWriteResponse_HasErrors_StringErr(t *testing.T) {
	raw := `{"DataType":12,"Error":"some error","ErrorID":20}`
	var r WriteResponse
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		t.Fatalf("Unmarshal err: %v", err)
	}
	if !r.HasErrors() {
		t.Error("HasErrors() = false, want true (Error is non-empty string)")
	}
	if got := r.ErrorMessages(); len(got) != 1 || got[0] != "some error" {
		t.Errorf("ErrorMessages() = %v, want [some error]", got)
	}
	if got := r.ErrorIDs(); len(got) != 1 || got[0] != 20 {
		t.Errorf("ErrorIDs() = %v, want [20]", got)
	}
}

// TestWriteResponse_HasErrors_StringArrErr (发现 F3 数组错误模型)
func TestWriteResponse_HasErrors_StringArrErr(t *testing.T) {
	raw := `{"DataType":12,"Error":["err1","err2"],"ErrorID":[1,2]}`
	var r WriteResponse
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		t.Fatalf("Unmarshal err: %v", err)
	}
	if !r.HasErrors() {
		t.Error("HasErrors() = false, want true")
	}
	got := r.ErrorMessages()
	if len(got) != 2 || got[0] != "err1" || got[1] != "err2" {
		t.Errorf("ErrorMessages() = %v, want [err1 err2]", got)
	}
	gotIDs := r.ErrorIDs()
	if len(gotIDs) != 2 || gotIDs[0] != 1 || gotIDs[1] != 2 {
		t.Errorf("ErrorIDs() = %v, want [1 2]", gotIDs)
	}
}

// TestWriteResponse_HasErrors_NilErr
func TestWriteResponse_HasErrors_NilErr(t *testing.T) {
	raw := `{"DataType":12,"Error":null,"ErrorID":null}`
	var r WriteResponse
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		t.Fatalf("Unmarshal err: %v", err)
	}
	if r.HasErrors() {
		t.Error("HasErrors() = true, want false (Error is null)")
	}
	if got := r.ErrorMessages(); got != nil {
		t.Errorf("ErrorMessages() = %v, want nil", got)
	}
	if got := r.ErrorIDs(); got != nil {
		t.Errorf("ErrorIDs() = %v, want nil", got)
	}
}

// TestWriteResponse_HasErrors_EmptyArr
func TestWriteResponse_HasErrors_EmptyArr(t *testing.T) {
	raw := `{"DataType":12,"Error":[],"ErrorID":[]}`
	var r WriteResponse
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		t.Fatalf("Unmarshal err: %v", err)
	}
	if r.HasErrors() {
		t.Error("HasErrors() = true, want false (Error is empty array)")
	}
	if got := r.ErrorMessages(); got != nil {
		t.Errorf("ErrorMessages() = %v, want nil", got)
	}
}

// TestWriteResponse_HasErrors_MissingField
func TestWriteResponse_HasErrors_MissingField(t *testing.T) {
	raw := `{"DataType":12}`
	var r WriteResponse
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		t.Fatalf("Unmarshal err: %v", err)
	}
	if r.HasErrors() {
		t.Error("HasErrors() = true, want false (Error 字段缺失)")
	}
}

// TestWriteResponse_ErrorID_MixedTypes
func TestWriteResponse_ErrorID_MixedTypes(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want []int
	}{
		{"int", `{"DataType":12,"ErrorID":42}`, []int{42}},
		{"[]int", `{"DataType":12,"ErrorID":[1,2,3]}`, []int{1, 2, 3}},
		{"null", `{"DataType":12,"ErrorID":null}`, nil},
		{"missing", `{"DataType":12}`, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var r WriteResponse
			if err := json.Unmarshal([]byte(c.raw), &r); err != nil {
				t.Fatalf("Unmarshal err: %v", err)
			}
			got := r.ErrorIDs()
			if len(got) != len(c.want) {
				t.Errorf("ErrorIDs() = %v, want %v", got, c.want)
				return
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Errorf("ErrorIDs()[%d] = %d, want %d", i, got[i], c.want[i])
				}
			}
		})
	}
}

// TestWriteResponse_RoundTrip_DecentralDevice (含完整 topology 字段)
func TestWriteResponse_RoundTrip_DecentralDevice(t *testing.T) {
	raw := `{
		"DataType": 12,
		"PNDriver": {
			"DeviceName": "profinetdriver",
			"IPAddress": "192.168.3.15",
			"SubnetMask": "255.255.255.0",
			"SetInTheProject": true
		},
		"DecentralDevice": [
			{
				"DeviceID": "0x0001",
				"DeviceName": "welder-01",
				"IPAddress": "192.168.2.20",
				"RefGSD": "gsdml-v2.31-hms-abcc40-pir-20171101.xml",
				"Module": []
			}
		],
		"IDevice": {
			"Activate": true,
			"InputLength": 64,
			"OutputLength": 64
		},
		"TotalInputLength": 64,
		"TotalOutputLength": 64,
		"Error": [],
		"ErrorID": []
	}`
	var r WriteResponse
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		t.Fatalf("Unmarshal err: %v", err)
	}
	if r.DataType != 12 {
		t.Errorf("DataType = %d, want 12", r.DataType)
	}
	if r.TotalInputLength != 64 || r.TotalOutputLength != 64 {
		t.Errorf("TotalInput/Output = %d/%d, want 64/64",
			r.TotalInputLength, r.TotalOutputLength)
	}
	if len(r.DecentralDevice) == 0 {
		t.Error("DecentralDevice 字段缺失")
	}
	if len(r.IDevice) == 0 {
		t.Error("IDevice 字段缺失")
	}
	if len(r.PNDriver) == 0 {
		t.Error("PNDriver 字段缺失")
	}
	if r.HasErrors() {
		t.Error("HasErrors() = true, want false (空 Error 数组)")
	}
	// 验证 DecentralDevice 内容正确解析
	var devices []struct {
		DeviceID   string `json:"DeviceID"`
		DeviceName string `json:"DeviceName"`
		IPAddress  string `json:"IPAddress"`
		RefGSD     string `json:"RefGSD"`
	}
	if err := json.Unmarshal(r.DecentralDevice, &devices); err != nil {
		t.Fatalf("DecentralDevice 解析失败: %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("DecentralDevice 长度 = %d, want 1", len(devices))
	}
	if devices[0].DeviceName != "welder-01" {
		t.Errorf("DeviceName = %q, want welder-01", devices[0].DeviceName)
	}
	if devices[0].IPAddress != "192.168.2.20" {
		t.Errorf("IPAddress = %q, want 192.168.2.20", devices[0].IPAddress)
	}
}

// TestWriteResponse_RoundTrip_WithoutOptionalFields (最小化响应: 无 Error 字段)
func TestWriteResponse_RoundTrip_WithoutOptionalFields(t *testing.T) {
	raw := `{"DataType":12}`
	var r WriteResponse
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		t.Fatalf("Unmarshal err: %v", err)
	}
	if r.HasErrors() {
		t.Error("HasErrors() 应为 false")
	}
}

// TestShieldDeviceResponse_Success (v3 CallbackNTJson 模板)
// 验证 ShieldDevice 成功响应的新结构 (提交 b77ba388):
//   - Function 是 string 标签 (e.g. "ShieldDevice")
//   - Result 是 bool, 在 root 顶层
//   - DeviceName 在 root 顶层
func TestShieldDeviceResponse_Success(t *testing.T) {
	raw := `{"DataType":14,"Function":"ShieldDevice","DeviceName":"welder-01","Result":true}`
	var r ShieldDeviceResponse
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		t.Fatalf("Unmarshal err: %v", err)
	}
	if r.DataType != 14 {
		t.Errorf("DataType = %d, want 14", r.DataType)
	}
	if r.Function != "ShieldDevice" {
		t.Errorf("Function = %q, want ShieldDevice", r.Function)
	}
	if r.DeviceName != "welder-01" {
		t.Errorf("DeviceName = %q, want welder-01", r.DeviceName)
	}
	if !r.IsSuccess() {
		t.Error("IsSuccess() = false, want true (Result=true)")
	}
	if !strings.Contains(r.String(), "成功") {
		t.Errorf("String() = %q, 应含 成功", r.String())
	}
	if !strings.Contains(r.String(), "welder-01") {
		t.Errorf("String() = %q, 应含 DeviceName", r.String())
	}
}

// TestShieldDeviceResponse_Failure (v3 失败响应)
func TestShieldDeviceResponse_Failure(t *testing.T) {
	raw := `{"DataType":14,"Function":"ShieldDevice","DeviceName":"nonexistent-device","Result":false}`
	var r ShieldDeviceResponse
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		t.Fatalf("Unmarshal err: %v", err)
	}
	if r.IsSuccess() {
		t.Error("IsSuccess() = true, want false (Result=false)")
	}
	if r.Function != "ShieldDevice" {
		t.Errorf("Function = %q, want ShieldDevice", r.Function)
	}
	if r.DeviceName != "nonexistent-device" {
		t.Errorf("DeviceName = %q, want nonexistent-device", r.DeviceName)
	}
	if !strings.Contains(r.String(), "失败") {
		t.Errorf("String() = %q, 应含 失败", r.String())
	}
}

// TestShieldDeviceResponse_UNShieldDevice (v3 UNShieldDevice 用同一模型, Function 字符串不同)
func TestShieldDeviceResponse_UNShieldDevice(t *testing.T) {
	raw := `{"DataType":14,"Function":"UNShieldDevice","DeviceName":"welder-01","Result":true}`
	var r ShieldDeviceResponse
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		t.Fatalf("Unmarshal err: %v", err)
	}
	if r.Function != "UNShieldDevice" {
		t.Errorf("Function = %q, want UNShieldDevice (v3 正确拼写)", r.Function)
	}
	if !r.IsSuccess() {
		t.Error("IsSuccess() = false, want true")
	}
}

// TestShieldDeviceResponse_TypeAssertion (Function 必须是 string, Result 必须是 bool)
//
// 模拟 C++ 端真的按新模板返回 — 如果 C++ 改回旧模板
// (Function 是 object 含 Value), 该测试会失败, 提醒维护者。
func TestShieldDeviceResponse_TypeAssertion(t *testing.T) {
	// 新模板: Function 是 string, Result 是 bool → 反序列化成功
	raw := `{"DataType":14,"Function":"ShieldDevice","DeviceName":"x","Result":true}`
	var r ShieldDeviceResponse
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		t.Fatalf("新模板反序列化失败: %v", err)
	}
	if !r.IsSuccess() {
		t.Error("IsSuccess 应为 true")
	}
	if r.Function != "ShieldDevice" {
		t.Errorf("Function 应为 ShieldDevice, got %q", r.Function)
	}

	// 旧模板: Function 是 object {Value: bool, DeviceName: "..."} → 反序列化失败
	// (新模型要求 Function 是 string)
	rawOld := `{"Function":{"Value":true,"DeviceName":"x"},"Result":true}`
	var r2 ShieldDeviceResponse
	if err := json.Unmarshal([]byte(rawOld), &r2); err == nil {
		t.Error("旧模板 (Function 是 object) 应无法反序列化为新模型, got nil")
	}
}

// TestWriteResponse_RoundTrip_RealMachineFixture (Step 10.B 实机 fixture)
//
// 加载 192.168.3.15 工业 PC 实机响应 (2026-06-05 L2 测试中 `config remove-device`
// 在 L1 baseline 设备不存在时的 C++ 真实响应 — 唯一获取到 C++ 响应的命令)。
//
// 关键观察:
//   - C++ 端真的会发 JSON 响应 (含 Error[20]), 之前 11 条 L2 命令"data:null"
//     是 inl "closed" 启发式误吞, 不是 C++ 端不发响应
//   - Function 是 object (含 SetPNDeviceNum + Value) 而非 string — 说明
//     `UninstallPNDevice` 还没用 v3 CallbackNTJson 模板
//   - Error 是 ["在进行设备卸载时索引不到需要卸载的设备。"] (中文, 单元素数组)
//   - ErrorID 是 [20] (单元素 int 数组)
//
// 该测试是 L2 阶段的"金标准" — 验证 WriteResponse 模型与 C++ 真实响应一致。
func TestWriteResponse_RoundTrip_RealMachineFixture(t *testing.T) {
	fixturePath := filepath.Join("..", "..", "testdata", "config_remove_device_post.json")
	raw, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("读取 fixture 失败: %v (path=%s)", err, fixturePath)
	}

	var r WriteResponse
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Fatalf("实机 fixture 反序列化失败: %v\nraw=%s", err, string(raw))
	}

	// DataType 必须 = 12
	if r.DataType != 12 {
		t.Errorf("DataType = %d, want 12", r.DataType)
	}

	// DecentralDevice 必须是 null (json.RawMessage 长度 > 0 但值是 "null")
	if len(r.DecentralDevice) == 0 {
		t.Error("DecentralDevice 字段缺失 (应存在且为 null)")
	}
	if string(bytes.TrimSpace(r.DecentralDevice)) != "null" {
		t.Errorf("DecentralDevice = %q, want null", string(r.DecentralDevice))
	}

	// HasErrors 必须返回 true (Error 是单元素非空数组)
	if !r.HasErrors() {
		t.Error("HasErrors() = false, want true (实机响应含业务错误)")
	}

	// ErrorMessages 必须返回中文错误 (容忍 []string / string 两种)
	msgs := r.ErrorMessages()
	if len(msgs) != 1 {
		t.Fatalf("ErrorMessages() 长度 = %d, want 1", len(msgs))
	}
	wantMsg := "在进行设备卸载时索引不到需要卸载的设备。"
	if msgs[0] != wantMsg {
		t.Errorf("ErrorMessages()[0] = %q, want %q", msgs[0], wantMsg)
	}

	// ErrorIDs 必须返回 [20]
	ids := r.ErrorIDs()
	if len(ids) != 1 || ids[0] != 20 {
		t.Errorf("ErrorIDs() = %v, want [20]", ids)
	}

	// 错误消息中应含 "设备" 和 "卸载" 关键字 (C++ 端中文字符编码验证)
	if !strings.Contains(msgs[0], "设备") || !strings.Contains(msgs[0], "卸载") {
		t.Errorf("ErrorMessages()[0] = %q, 应含 '设备' 和 '卸载'", msgs[0])
	}
}
