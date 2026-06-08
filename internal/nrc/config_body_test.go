package nrc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ====================== 通用辅助测试 ======================

func TestConfigBodyBuilder_EmptyData(t *testing.T) {
	_, err := configBodyBuilder(CommandSpec{Function: "AddPNDevice"}, map[string]string{"data": ""})
	if err == nil {
		t.Error("期望空 --data 错误, got nil")
	}
}

func TestConfigBodyBuilder_InvalidJSON(t *testing.T) {
	_, err := configBodyBuilder(CommandSpec{Function: "AddPNDevice"}, map[string]string{"data": "{not json}"})
	if err == nil {
		t.Error("期望非 JSON --data 错误, got nil")
	}
}

func TestConfigBodyBuilder_MissingDataArg(t *testing.T) {
	_, err := configBodyBuilder(CommandSpec{Function: "AddPNDevice"}, map[string]string{})
	if err == nil {
		t.Error("期望缺少 --data 参数错误, got nil")
	}
}

// TestConfigBodyBuilder_NilDecentralDeviceBecomesEmptyArray (Step 10.B 关键修复, P0 inl bug)
//
// L2 实机发现: 当 L1 baseline DecentralDevice 是 nil (空配置场景, 192.168.3.15
// 初始状态), inl 旧版直接 body["DecentralDevice"]=nil 序列化为 "DecentralDevice":null。
// C++ 端 AddPNDevice (PNConfigLibFileDesign.cpp:1225) 对 nullValue 调用 .append()
// 是 jsoncpp 静默 no-op — 设备未真正添加, 5-8s 后 C++ 关连接, inl 旧版 "closed"
// 启发式误判为成功。
//
// 修复后: inl 应将 nil DecentralDevice 转为空数组 [], 让 C++ 能正确 append 新设备。
func TestConfigBodyBuilder_NilDecentralDeviceBecomesEmptyArray(t *testing.T) {
	// 注入 mock fetchTopologyForConfig, 返回 DecentralDevice=nil (L1 空配置基线)
	originalFetch := fetchTopologyForConfig
	SetFetchTopologyForConfig(func(target string) (map[string]any, error) {
		return map[string]any{
			"PNDriver": map[string]any{
				"DeviceName": "profinetdriver",
				"IPAddress":  "192.168.2.14",
				"SubnetMask": "255.255.255.0",
			},
			"IDevice": map[string]any{
				"Activate":     false,
				"InputLength":  0,
				"OutputLength": 0,
			},
			"DecentralDevice": nil, // ← 关键: nil 而非 []
		}, nil
	})
	defer SetFetchTopologyForConfig(originalFetch)

	spec := CommandSpec{DataType: 12, Function: "AddPNDevice"}
	got, err := configBodyBuilder(spec, map[string]string{
		"data":   `{"RefGSD":"GSDML-V2.4-HERON-12345678","DAP_ID":1}`,
		"target": "192.168.3.15",
	})
	if err != nil {
		t.Fatalf("configBodyBuilder err: %v", err)
	}

	// 验证 DecentralDevice 是空数组 [] 而非 null
	if strings.Contains(got, `"DecentralDevice":null`) {
		t.Errorf("body 应含 \"DecentralDevice\":[] (空数组), got null:\n%s", got)
	}
	if !strings.Contains(got, `"DecentralDevice":[]`) {
		t.Errorf("body 应含 \"DecentralDevice\":[] (空数组, 让 C++ 能 append), got:\n%s", got)
	}

	// 验证 PNDriver/IDevice 仍正确嵌入
	if !strings.Contains(got, `"PNDriver":{`) {
		t.Errorf("body 应含 PNDriver 对象, got:\n%s", got)
	}
	if !strings.Contains(got, `"IDevice":{`) {
		t.Errorf("body 应含 IDevice 对象, got:\n%s", got)
	}

	// 用 JSON 反序列化二次验证 DecentralDevice 是数组类型
	var body map[string]any
	if err := json.Unmarshal([]byte(got), &body); err != nil {
		t.Fatalf("body JSON 解析失败: %v\nraw: %s", err, got)
	}
	dd, ok := body["DecentralDevice"]
	if !ok {
		t.Fatal("DecentralDevice 字段缺失")
	}
	if dd == nil {
		t.Errorf("DecentralDevice 不应为 null, got nil")
	}
	if _, isArray := dd.([]any); !isArray {
		t.Errorf("DecentralDevice 应为数组 []any, got %T (%v)", dd, dd)
	}
}

// TestConfigBodyBuilder_NonNilDecentralDevicePreserved (回归: 已有设备时不变)
func TestConfigBodyBuilder_NonNilDecentralDevicePreserved(t *testing.T) {
	originalFetch := fetchTopologyForConfig
	SetFetchTopologyForConfig(func(target string) (map[string]any, error) {
		return map[string]any{
			"PNDriver": map[string]any{"DeviceName": "profinetdriver", "IPAddress": "192.168.2.14", "SubnetMask": "255.255.255.0"},
			"IDevice":  map[string]any{"Activate": false, "InputLength": 0, "OutputLength": 0},
			"DecentralDevice": []any{
				map[string]any{"DeviceName": "heron-weld", "IPAddress": "192.168.2.15"},
			},
		}, nil
	})
	defer SetFetchTopologyForConfig(originalFetch)

	spec := CommandSpec{DataType: 12, Function: "AddPNDevice"}
	got, err := configBodyBuilder(spec, map[string]string{
		"data":   `{"RefGSD":"GSDML-V2.4-HERON-12345678","DAP_ID":1}`,
		"target": "192.168.3.15",
	})
	if err != nil {
		t.Fatalf("configBodyBuilder err: %v", err)
	}

	// 验证 DecentralDevice 包含已有设备, 不被替换为 []
	if !strings.Contains(got, `"DeviceName":"heron-weld"`) {
		t.Errorf("body 应保留已有设备 heron-weld, got:\n%s", got)
	}
	if strings.Contains(got, `"DecentralDevice":[]`) {
		t.Errorf("body 不应被替换为空数组, got:\n%s", got)
	}
}

// TestConfigAddDeviceBody_RoundTrip_L0Fixture (Step 10.B L0 dry-run 实机 fixture)
//
// 加载 C:\Users\BYD\step10-test\dryrun\add-device.txt 抓取的 DryRunFrame
// (2026-06-05 实机抓取, **修复前** 的 inl 输出, 记录历史行为)。
//
// 关键观察 (历史): L0 抓取的 payload **没有** DecentralDevice/PNDriver/IDevice 字段
// (total_bytes=108 偏小), 说明当时 auto-fetch 静默失败或返回空 topology。修复后
// 需用户重抓 L0 验证新 payload 含 `"DecentralDevice":[]` 等字段。
//
// 本测试只验证 fixture 是合法的 DryRunFrame + 关键字段一致, 不严格比较 payload 内容。
func TestConfigAddDeviceBody_RoundTrip_L0Fixture(t *testing.T) {
	fixturePath := filepath.Join("..", "..", "testdata", "config_add_device_dryrun.json")
	raw, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Skipf("L0 fixture 缺失 (跳过): %v (path=%s)", err, fixturePath)
	}

	var frame map[string]any
	if err := json.Unmarshal(raw, &frame); err != nil {
		t.Fatalf("DryRunFrame 解析失败: %v", err)
	}

	// 验证 DryRunFrame 顶层字段
	if frame["sync_byte"] != "0x4E66" {
		t.Errorf("sync_byte = %v, want 0x4E66", frame["sync_byte"])
	}
	if frame["command"] != "0x9275" {
		t.Errorf("command = %v, want 0x9275", frame["command"])
	}
	if frame["datatype"] != float64(12) {
		t.Errorf("datatype = %v, want 12", frame["datatype"])
	}
	if frame["function"] != "AddPNDevice" {
		t.Errorf("function = %v, want AddPNDevice", frame["function"])
	}
	if frame["risk"] != "write" {
		t.Errorf("risk = %v, want write", frame["risk"])
	}

	// 验证 payload 是合法 JSON 且 Function.Value="AddPNDevice"
	payload, ok := frame["payload"].(string)
	if !ok {
		t.Fatalf("payload 字段类型错, got: %T", frame["payload"])
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(payload), &body); err != nil {
		t.Fatalf("payload JSON 解析失败: %v\nraw: %s", err, payload)
	}
	fn, ok := body["Function"].(map[string]any)
	if !ok {
		t.Fatalf("Function 字段缺失或类型错, got: %T", body["Function"])
	}
	if fn["Value"] != "AddPNDevice" {
		t.Errorf("Function.Value = %v, want AddPNDevice", fn["Value"])
	}
	if fn["RefGSD"] == nil || fn["RefGSD"] == "" {
		t.Error("Function.RefGSD 不应为空")
	}
	if fn["DAP_ID"] == nil {
		t.Error("Function.DAP_ID 不应为空")
	}
}

// ====================== 模式 B: SetPNDriver ======================

// TestConfigSetDriverBody_Valid (F1: 字段在顶层 PNDriver)
func TestConfigSetDriverBody_Valid(t *testing.T) {
	spec := CommandSpec{DataType: 12, Function: "SetPNDriver"}
	got, err := configSetDriverBody(spec, map[string]string{
		"data": `{"DeviceName":"profinetdriver","IPAddress":"192.168.3.15","SubnetMask":"255.255.255.0","SetInTheProject":true}`,
	})
	if err != nil {
		t.Fatalf("configSetDriverBody err: %v", err)
	}
	// 验证: Function.Value="SetPNDriver" + 顶层 PNDriver
	if !strings.Contains(got, `"Function":{"Value":"SetPNDriver"}`) {
		t.Errorf("body 应含 Function.Value=SetPNDriver, got: %s", got)
	}
	if !strings.Contains(got, `"PNDriver":{`) {
		t.Errorf("body 应含顶层 PNDriver 对象, got: %s", got)
	}
	if !strings.Contains(got, `"IPAddress":"192.168.3.15"`) {
		t.Errorf("body 应含 IPAddress=192.168.3.15, got: %s", got)
	}
	// 验证: PNDriver 不应嵌套在 Function 下
	if strings.Contains(got, `"Function":{"Value":"SetPNDriver","PNDriver"`) {
		t.Errorf("PNDriver 不应在 Function 下 (模式 B 误判), got: %s", got)
	}
}

// TestConfigSetDriverBody_MissingRequired (F1: 必填字段校验)
func TestConfigSetDriverBody_MissingRequired(t *testing.T) {
	spec := CommandSpec{DataType: 12, Function: "SetPNDriver"}
	cases := []struct {
		name string
		data string
		want string
	}{
		{"缺 DeviceName", `{"IPAddress":"192.168.3.15","SubnetMask":"255.255.255.0","SetInTheProject":true}`, "DeviceName"},
		{"缺 IPAddress", `{"DeviceName":"profinetdriver","SubnetMask":"255.255.255.0","SetInTheProject":true}`, "IPAddress"},
		{"缺 SubnetMask", `{"DeviceName":"profinetdriver","IPAddress":"192.168.3.15","SetInTheProject":true}`, "SubnetMask"},
		{"缺 SetInTheProject", `{"DeviceName":"profinetdriver","IPAddress":"192.168.3.15","SubnetMask":"255.255.255.0"}`, "SetInTheProject"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := configSetDriverBody(spec, map[string]string{"data": c.data})
			if err == nil {
				t.Fatalf("期望错误, got nil")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("err = %q, 应含 %q", err.Error(), c.want)
			}
		})
	}
}

// TestConfigSetDriverBody_Empty (F1: --data 必填)
func TestConfigSetDriverBody_Empty(t *testing.T) {
	spec := CommandSpec{DataType: 12, Function: "SetPNDriver"}
	_, err := configSetDriverBody(spec, map[string]string{"data": ""})
	if err == nil {
		t.Error("期望空 --data 错误, got nil")
	}
}

// TestConfigSetDriverBody_InvalidJSON (F1: JSON 合法性)
func TestConfigSetDriverBody_InvalidJSON(t *testing.T) {
	spec := CommandSpec{DataType: 12, Function: "SetPNDriver"}
	_, err := configSetDriverBody(spec, map[string]string{"data": "{not valid"})
	if err == nil {
		t.Error("期望非 JSON --data 错误, got nil")
	}
}

// ====================== 模式 A: AddPNDevice ======================

func TestConfigAddDeviceBody_Valid(t *testing.T) {
	spec := CommandSpec{DataType: 12, Function: "AddPNDevice"}
	got, err := configAddDeviceBody(spec, map[string]string{
		"data":     `{"RefGSD":"gsdml-v2.31-hms-abcc40-pir-20171101.xml","DAP_ID":"0x0001"}`,
		"no-fetch": "true",
	})
	if err != nil {
		t.Fatalf("configAddDeviceBody err: %v", err)
	}
	// 用 JSON 反序列化验证 (避免依赖 json.Marshal 字段顺序)
	var body map[string]any
	if err := json.Unmarshal([]byte(got), &body); err != nil {
		t.Fatalf("body JSON 解析失败: %v\nraw: %s", err, got)
	}
	fn, ok := body["Function"].(map[string]any)
	if !ok {
		t.Fatalf("Function 字段缺失或类型错误, got: %s", got)
	}
	if fn["Value"] != "AddPNDevice" {
		t.Errorf("Function.Value = %v, want AddPNDevice", fn["Value"])
	}
	if fn["RefGSD"] != "gsdml-v2.31-hms-abcc40-pir-20171101.xml" {
		t.Errorf("Function.RefGSD = %v, want gsdml-v2.31-hms-abcc40-pir-20171101.xml", fn["RefGSD"])
	}
	if fn["DAP_ID"] != "0x0001" {
		t.Errorf("Function.DAP_ID = %v, want 0x0001", fn["DAP_ID"])
	}
}

func TestConfigAddDeviceBody_MissingRequired(t *testing.T) {
	spec := CommandSpec{DataType: 12, Function: "AddPNDevice"}
	cases := []struct {
		name string
		data string
	}{
		{"缺 RefGSD", `{"DAP_ID":"0x0001"}`},
		{"缺 DAP_ID", `{"RefGSD":"gsdml.xml"}`},
		{"全空", `{}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := configAddDeviceBody(spec, map[string]string{"data": c.data})
			if err == nil {
				t.Errorf("期望错误, got nil")
			}
		})
	}
}

// ====================== 模式 A: UninstallPNDevice (1-based 索引) ======================

// TestConfigRemoveDeviceBody_OneBasedIndex (F4)
func TestConfigRemoveDeviceBody_OneBasedIndex(t *testing.T) {
	spec := CommandSpec{DataType: 12, Function: "UninstallPNDevice"}
	// SetPNDeviceNum=0 → 期望 1-based 校验失败
	_, err := configRemoveDeviceBody(spec, map[string]string{
		"data": `{"SetPNDeviceNum":0}`,
	})
	if err == nil {
		t.Fatal("SetPNDeviceNum=0 应触发 1-based 校验错误, got nil")
	}
	if !strings.Contains(err.Error(), "1-based") {
		t.Errorf("err = %q, 应含 1-based", err.Error())
	}

	// SetPNDeviceNum=-1 → 同样失败
	_, err = configRemoveDeviceBody(spec, map[string]string{
		"data": `{"SetPNDeviceNum":-1}`,
	})
	if err == nil {
		t.Error("SetPNDeviceNum=-1 应触发校验错误, got nil")
	}
}

func TestConfigRemoveDeviceBody_Valid(t *testing.T) {
	spec := CommandSpec{DataType: 12, Function: "UninstallPNDevice"}
	got, err := configRemoveDeviceBody(spec, map[string]string{
		"data":     `{"SetPNDeviceNum":2}`,
		"no-fetch": "true",
	})
	if err != nil {
		t.Fatalf("configRemoveDeviceBody err: %v", err)
	}
	// JSON 反序列化验证 (避免依赖字段顺序)
	var body map[string]any
	if err := json.Unmarshal([]byte(got), &body); err != nil {
		t.Fatalf("body JSON 解析失败: %v\nraw: %s", err, got)
	}
	fn, ok := body["Function"].(map[string]any)
	if !ok {
		t.Fatalf("Function 字段缺失或类型错误, got: %s", got)
	}
	if fn["Value"] != "UninstallPNDevice" {
		t.Errorf("Function.Value = %v, want UninstallPNDevice", fn["Value"])
	}
	// SetPNDeviceNum 在 json.Unmarshal 后是 float64
	if n, ok := fn["SetPNDeviceNum"].(float64); !ok || n != 2 {
		t.Errorf("Function.SetPNDeviceNum = %v, want 2", fn["SetPNDeviceNum"])
	}
}

// ====================== 模式 A: SetPNDevice (只校验) ======================

func TestConfigSetDeviceBody_Valid(t *testing.T) {
	spec := CommandSpec{DataType: 12, Function: "SetPNDevice"}
	got, err := configSetDeviceBody(spec, map[string]string{
		"data":     `{"SetPNDeviceNum":1}`,
		"no-fetch": "true",
	})
	if err != nil {
		t.Fatalf("configSetDeviceBody err: %v", err)
	}
	if !strings.Contains(got, `"SetPNDevice"`) {
		t.Errorf("body 应含 SetPNDevice, got: %s", got)
	}
}

func TestConfigSetDeviceBody_ZeroIndex(t *testing.T) {
	spec := CommandSpec{DataType: 12, Function: "SetPNDevice"}
	_, err := configSetDeviceBody(spec, map[string]string{"data": `{"SetPNDeviceNum":0}`})
	if err == nil {
		t.Error("SetPNDeviceNum=0 应触发 1-based 校验错误, got nil")
	}
}

// ====================== 模式 A: AddModule ======================

func TestConfigAddModuleBody_Valid(t *testing.T) {
	spec := CommandSpec{DataType: 12, Function: "AddModule"}
	got, err := configAddModuleBody(spec, map[string]string{
		"data":     `{"SetPNDeviceNum":1,"ModuleID":"0x00000001"}`,
		"no-fetch": "true",
	})
	if err != nil {
		t.Fatalf("configAddModuleBody err: %v", err)
	}
	if !strings.Contains(got, `"AddModule"`) {
		t.Errorf("body 应含 AddModule, got: %s", got)
	}
	if !strings.Contains(got, `"ModuleID":"0x00000001"`) {
		t.Errorf("body 应含 ModuleID, got: %s", got)
	}
}

func TestConfigAddModuleBody_MissingRequired(t *testing.T) {
	spec := CommandSpec{DataType: 12, Function: "AddModule"}
	_, err := configAddModuleBody(spec, map[string]string{"data": `{"SetPNDeviceNum":1}`})
	if err == nil {
		t.Error("缺 ModuleID 应触发校验错误, got nil")
	}
	_, err = configAddModuleBody(spec, map[string]string{"data": `{"ModuleID":"0x1"}`})
	if err == nil {
		t.Error("缺 SetPNDeviceNum 应触发校验错误, got nil")
	}
}

// ====================== 模式 A: UninstallModule ======================

func TestConfigRemoveModuleBody_Valid(t *testing.T) {
	spec := CommandSpec{DataType: 12, Function: "UninstallModule"}
	got, err := configRemoveModuleBody(spec, map[string]string{
		"data":     `{"SetPNDeviceNum":1,"SetModuleSlot":3}`,
		"no-fetch": "true",
	})
	if err != nil {
		t.Fatalf("configRemoveModuleBody err: %v", err)
	}
	if !strings.Contains(got, `"UninstallModule"`) {
		t.Errorf("body 应含 UninstallModule, got: %s", got)
	}
	if !strings.Contains(got, `"SetModuleSlot":3`) {
		t.Errorf("body 应含 SetModuleSlot:3, got: %s", got)
	}
}

func TestConfigRemoveModuleBody_ZeroSlot(t *testing.T) {
	spec := CommandSpec{DataType: 12, Function: "UninstallModule"}
	_, err := configRemoveModuleBody(spec, map[string]string{
		"data": `{"SetPNDeviceNum":1,"SetModuleSlot":0}`,
	})
	if err == nil {
		t.Error("SetModuleSlot=0 应触发 1-based 校验错误, got nil")
	}
}

// ====================== 模式 A: AddSubmodule ======================

func TestConfigAddSubmoduleBody_Valid(t *testing.T) {
	spec := CommandSpec{DataType: 12, Function: "AddSubmodule"}
	got, err := configAddSubmoduleBody(spec, map[string]string{
		"data":     `{"SetPNDeviceNum":1,"ModuleID":"0x00000001","SubmoduleID":"0x00000001"}`,
		"no-fetch": "true",
	})
	if err != nil {
		t.Fatalf("configAddSubmoduleBody err: %v", err)
	}
	if !strings.Contains(got, `"AddSubmodule"`) {
		t.Errorf("body 应含 AddSubmodule, got: %s", got)
	}
	if !strings.Contains(got, `"SubmoduleID":"0x00000001"`) {
		t.Errorf("body 应含 SubmoduleID, got: %s", got)
	}
}

func TestConfigAddSubmoduleBody_MissingRequired(t *testing.T) {
	spec := CommandSpec{DataType: 12, Function: "AddSubmodule"}
	_, err := configAddSubmoduleBody(spec, map[string]string{
		"data": `{"SetPNDeviceNum":1,"ModuleID":"0x1"}`,
	})
	if err == nil {
		t.Error("缺 SubmoduleID 应触发校验错误, got nil")
	}
}

// ====================== 模式 A: UninstallSubmodule ======================

func TestConfigRemoveSubmoduleBody_Valid(t *testing.T) {
	spec := CommandSpec{DataType: 12, Function: "UninstallSubmodule"}
	got, err := configRemoveSubmoduleBody(spec, map[string]string{
		"data":     `{"SetPNDeviceNum":1,"SetModuleSlot":1,"SetSubmoduleSlot":1}`,
		"no-fetch": "true",
	})
	if err != nil {
		t.Fatalf("configRemoveSubmoduleBody err: %v", err)
	}
	if !strings.Contains(got, `"UninstallSubmodule"`) {
		t.Errorf("body 应含 UninstallSubmodule, got: %s", got)
	}
}

func TestConfigRemoveSubmoduleBody_AllZero(t *testing.T) {
	spec := CommandSpec{DataType: 12, Function: "UninstallSubmodule"}
	_, err := configRemoveSubmoduleBody(spec, map[string]string{
		"data": `{"SetPNDeviceNum":0,"SetModuleSlot":0,"SetSubmoduleSlot":0}`,
	})
	if err == nil {
		t.Error("全 0 索引应触发校验错误, got nil")
	}
}

// ====================== 模式 A: ShieldDevice (响应特殊, v3 正确拼写) ======================

// 2026-06-04 v3: C++ 源常量拼写已纠正回 "ShieldDevice" (typo 修复提交 3d3cc3c7),
// Registry + BodyBuilder 统一用正确拼写。
//
// 注意: 请求体仍用旧模板 (Function 为 object, 内含 Value + DeviceName),
// 因为 C++ 端 NetWorkTopologyFunction 分发器仍读 `networktopology["Function"]["Value"]`
// (L158, 165 等) 和 `networktopology["Function"]["DeviceName"]` (L316, 333)。
// 响应体已用新模板 (Function 为 string 标签, DeviceName/Result 在顶层),
// 见 ShieldDeviceByName/UNShieldDeviceByName 提交 b77ba388。
// 来源: io-controller/src/ioc/profinet_constants.h:82-83
//       io-controller/src/pnconfiglib/PNConfigLibFileDesign.cpp L158, 165, 316, 333

func TestConfigShieldBody_Valid(t *testing.T) {
	spec := CommandSpec{DataType: 12, Function: "ShieldDevice"}
	got, err := configShieldBody(spec, map[string]string{
		"data":     `{"DeviceName":"welder-01"}`,
		"no-fetch": "true",
	})
	if err != nil {
		t.Fatalf("configShieldBody err: %v", err)
	}
	// v3 验证: 请求体含 Function.Value="ShieldDevice" (分派器读)
	// JSON 反序列化验证 (避免依赖 json.Marshal 字段顺序)
	var body map[string]any
	if err := json.Unmarshal([]byte(got), &body); err != nil {
		t.Fatalf("body JSON 解析失败: %v\nraw: %s", err, got)
	}
	fn, ok := body["Function"].(map[string]any)
	if !ok {
		t.Fatalf("Function 字段缺失或类型错误, got: %s", got)
	}
	if fn["Value"] != "ShieldDevice" {
		t.Errorf("Function.Value = %v, want ShieldDevice (v3 正确拼写)", fn["Value"])
	}
	// v3 验证: DeviceName 仍在 Function 下 (C++ 分派器 L316 仍读 Function.DeviceName)
	if fn["DeviceName"] != "welder-01" {
		t.Errorf("Function.DeviceName = %v, want welder-01 (C++ 端仍按旧模板读)", fn["DeviceName"])
	}
}

func TestConfigShieldBody_MissingDeviceName(t *testing.T) {
	spec := CommandSpec{DataType: 12, Function: "ShieldDevice"}
	_, err := configShieldBody(spec, map[string]string{"data": `{}`})
	if err == nil {
		t.Error("缺 DeviceName 应触发校验错误, got nil")
	}
}

// ====================== 模式 A: UNShieldDevice (响应特殊, v3 正确拼写) ======================

func TestConfigUnshieldBody_Valid(t *testing.T) {
	spec := CommandSpec{DataType: 12, Function: "UNShieldDevice"}
	got, err := configUnshieldBody(spec, map[string]string{
		"data":     `{"DeviceName":"welder-01"}`,
		"no-fetch": "true",
	})
	if err != nil {
		t.Fatalf("configUnshieldBody err: %v", err)
	}
	// JSON 反序列化验证 (避免依赖 json.Marshal 字段顺序)
	var body map[string]any
	if err := json.Unmarshal([]byte(got), &body); err != nil {
		t.Fatalf("body JSON 解析失败: %v\nraw: %s", err, got)
	}
	fn, ok := body["Function"].(map[string]any)
	if !ok {
		t.Fatalf("Function 字段缺失或类型错误, got: %s", got)
	}
	if fn["Value"] != "UNShieldDevice" {
		t.Errorf("Function.Value = %v, want UNShieldDevice (v3 正确拼写)", fn["Value"])
	}
	if fn["DeviceName"] != "welder-01" {
		t.Errorf("Function.DeviceName = %v, want welder-01", fn["DeviceName"])
	}
}

func TestConfigUnshieldBody_MissingDeviceName(t *testing.T) {
	spec := CommandSpec{DataType: 12, Function: "UNShieldDevice"}
	_, err := configUnshieldBody(spec, map[string]string{"data": `{}`})
	if err == nil {
		t.Error("缺 DeviceName 应触发校验错误, got nil")
	}
}

// ====================== 模式 A: SetIDevice (Step 9.1 已有, 范围校验) ======================

func TestConfigSetIDeviceBody_RangeCheck(t *testing.T) {
	spec := CommandSpec{DataType: 12, Function: "SetIDevice"}
	// InputLength > 2048 → error
	_, err := configSetIDeviceBody(spec, map[string]string{
		"data": `{"Activate":true,"InputLength":4096,"OutputLength":64}`,
	})
	if err == nil {
		t.Error("InputLength=4096 应触发范围校验错误, got nil")
	}
	// OutputLength < 0 → error
	_, err = configSetIDeviceBody(spec, map[string]string{
		"data": `{"Activate":true,"InputLength":64,"OutputLength":-1}`,
	})
	if err == nil {
		t.Error("OutputLength=-1 应触发范围校验错误, got nil")
	}
}

func TestConfigSetIDeviceBody_PlaceFieldAtIDevice(t *testing.T) {
	spec := CommandSpec{DataType: 12, Function: "SetIDevice"}
	got, err := configSetIDeviceBody(spec, map[string]string{
		"data": `{"Activate":true,"InputLength":64,"OutputLength":64}`,
	})
	if err != nil {
		t.Fatalf("configSetIDeviceBody err: %v", err)
	}
	// 验证: SetIDevice 字段在顶层 IDevice (不是 Function 下)
	if !strings.Contains(got, `"IDevice":{`) {
		t.Errorf("body 应含顶层 IDevice 对象, got: %s", got)
	}
	if !strings.Contains(got, `"Function":{"Value":"SetIDevice"`) {
		t.Errorf("body 应含 Function.Value=SetIDevice, got: %s", got)
	}
	// 验证: Activate/InputLength/OutputLength 不在 Function 下
	if strings.Contains(got, `"Function":{"Value":"SetIDevice","Activate"`) {
		t.Errorf("Activate 不应在 Function 下, got: %s", got)
	}
}

// ====================== Routing 验证 (模式 A vs 模式 B) ======================

// TestConfigBodyBuilder_Routing_ModeA (F1: 业务字段放 Function 下)
func TestConfigBodyBuilder_Routing_ModeA(t *testing.T) {
	spec := CommandSpec{DataType: 12, Function: "AddPNDevice"}
	got, err := configBodyBuilder(spec, map[string]string{
		"data":     `{"RefGSD":"a.xml","DAP_ID":"0x1"}`,
		"no-fetch": "true",
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	// 验证: RefGSD/DAP_ID 在 Function 嵌套对象内
	var body map[string]any
	if err := json.Unmarshal([]byte(got), &body); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	fn, ok := body["Function"].(map[string]any)
	if !ok {
		t.Fatal("Function 字段缺失或不是对象")
	}
	if fn["Value"] != "AddPNDevice" {
		t.Errorf("Function.Value = %v, want AddPNDevice", fn["Value"])
	}
	if fn["RefGSD"] != "a.xml" {
		t.Errorf("Function.RefGSD = %v, want a.xml", fn["RefGSD"])
	}
	if fn["DAP_ID"] != "0x1" {
		t.Errorf("Function.DAP_ID = %v, want 0x1", fn["DAP_ID"])
	}
	// 验证: 顶层没有 PNDriver / 业务字段平铺
	if _, exists := body["PNDriver"]; exists {
		t.Error("模式 A 不应在顶层有 PNDriver")
	}
	if _, exists := body["RefGSD"]; exists {
		t.Error("RefGSD 不应平铺到顶层")
	}
}

// TestConfigBodyBuilder_Routing_ModeB (F1: 业务字段放顶层 PNDriver)
func TestConfigBodyBuilder_Routing_ModeB(t *testing.T) {
	spec := CommandSpec{DataType: 12, Function: "SetPNDriver"}
	got, err := configBodyBuilder(spec, map[string]string{
		"data":     `{"DeviceName":"p","IPAddress":"1.2.3.4","SubnetMask":"255.255.255.0","SetInTheProject":true}`,
		"no-fetch": "true",
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(got), &body); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	// 顶层 PNDriver 对象
	driver, ok := body["PNDriver"].(map[string]any)
	if !ok {
		t.Fatal("顶层 PNDriver 字段缺失或不是对象")
	}
	if driver["DeviceName"] != "p" {
		t.Errorf("PNDriver.DeviceName = %v, want p", driver["DeviceName"])
	}
	// Function 仅含 Value, 不含业务字段
	fn, ok := body["Function"].(map[string]any)
	if !ok {
		t.Fatal("Function 字段缺失或不是对象")
	}
	if fn["Value"] != "SetPNDriver" {
		t.Errorf("Function.Value = %v, want SetPNDriver", fn["Value"])
	}
	if _, exists := fn["DeviceName"]; exists {
		t.Error("模式 B 业务字段不应在 Function 下")
	}
	if _, exists := fn["IPAddress"]; exists {
		t.Error("模式 B 业务字段不应在 Function 下")
	}
}

// ====================== Auto-Fetch 验证 (F2) ======================

// TestConfigBodyBuilder_AutoFetch_NoFetch (F2: --no-fetch=true → 不调用 fetch)
func TestConfigBodyBuilder_AutoFetch_NoFetch(t *testing.T) {
	called := false
	prev := fetchTopologyForConfig
	SetFetchTopologyForConfig(func(target string) (map[string]any, error) {
		called = true
		return nil, nil
	})
	defer SetFetchTopologyForConfig(prev)

	spec := CommandSpec{DataType: 12, Function: "AddPNDevice"}
	got, err := configBodyBuilder(spec, map[string]string{
		"data":     `{"RefGSD":"a.xml","DAP_ID":"0x1"}`,
		"no-fetch": "true",
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if called {
		t.Error("--no-fetch=true 时不应调用 fetch, 但被调用了")
	}
	if strings.Contains(got, "DecentralDevice") {
		t.Errorf("--no-fetch=true 时 body 不应含 DecentralDevice, got: %s", got)
	}
}

// TestConfigBodyBuilder_AutoFetch_FetchFailed (F2: fetch 失败不阻塞)
func TestConfigBodyBuilder_AutoFetch_FetchFailed(t *testing.T) {
	prev := fetchTopologyForConfig
	SetFetchTopologyForConfig(func(target string) (map[string]any, error) {
		return nil, errMockFetch
	})
	defer SetFetchTopologyForConfig(prev)

	spec := CommandSpec{DataType: 12, Function: "AddPNDevice"}
	got, err := configBodyBuilder(spec, map[string]string{
		"data": `{"RefGSD":"a.xml","DAP_ID":"0x1"}`,
		// 注意: 没有 --no-fetch, 默认应 fetch
	})
	if err != nil {
		t.Fatalf("fetch 失败时应仍能构造 body, got err: %v", err)
	}
	// 验证: body 仍有效
	if !strings.Contains(got, `"RefGSD":"a.xml"`) {
		t.Errorf("body 应含 RefGSD, got: %s", got)
	}
	// 验证: body 不含 DecentralDevice (因为 fetch 失败)
	if strings.Contains(got, "DecentralDevice") {
		t.Errorf("fetch 失败时 body 不应含 DecentralDevice, got: %s", got)
	}
}

// errMockFetch 是 fetch 失败的 mock 错误。
var errMockFetch = errMock("mock fetch failed")

type errMock string

func (e errMock) Error() string { return string(e) }

// TestConfigBodyBuilder_AutoFetch_FetchSuccess (F2: mock fetch 返回 topology, body 应含 DecentralDevice)
func TestConfigBodyBuilder_AutoFetch_FetchSuccess(t *testing.T) {
	prev := fetchTopologyForConfig
	mockTopology := map[string]any{
		"DecentralDevice": []any{
			map[string]any{"DeviceName": "existing-device", "DeviceID": "0x0001"},
		},
		"PNDriver": map[string]any{
			"DeviceName": "profinetdriver",
			"IPAddress":  "192.168.3.15",
		},
		"IDevice": map[string]any{
			"Activate":     true,
			"InputLength":  64,
			"OutputLength": 64,
		},
	}
	SetFetchTopologyForConfig(func(target string) (map[string]any, error) {
		if target != "192.168.3.15" {
			t.Errorf("fetch target = %q, want 192.168.3.15", target)
		}
		return mockTopology, nil
	})
	defer SetFetchTopologyForConfig(prev)

	spec := CommandSpec{DataType: 12, Function: "AddPNDevice"}
	got, err := configBodyBuilder(spec, map[string]string{
		"data":   `{"RefGSD":"a.xml","DAP_ID":"0x1"}`,
		"target": "192.168.3.15",
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	// 验证: body 含 DecentralDevice / PNDriver / IDevice
	if !strings.Contains(got, `"DecentralDevice"`) {
		t.Errorf("body 应含 DecentralDevice, got: %s", got)
	}
	if !strings.Contains(got, `"existing-device"`) {
		t.Errorf("body 应含 existing-device, got: %s", got)
	}
	if !strings.Contains(got, `"PNDriver"`) {
		t.Errorf("body 应含 PNDriver, got: %s", got)
	}
	if !strings.Contains(got, `"IDevice"`) {
		t.Errorf("body 应含 IDevice, got: %s", got)
	}
}

// ====================== Compile 命令 (Step 9.1, 无 --data) ======================

func TestConfigCompileBody_NoArgsNeeded(t *testing.T) {
	spec, _ := LookupByName("config-compile")
	got, err := RequestBody(spec, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	// Compile 用 DefaultBodyBuilder, 输出 {"DataType":12,"Function":{"Value":"Compile"}}
	want := `{"DataType":12,"Function":{"Value":"Compile"}}`
	if got != want {
		t.Errorf("RequestBody(config-compile) = %q, want %q", got, want)
	}
}

// ====================== 验证 Registry 中的所有 11 条 config 写命令都已参数化 ======================

// TestRegistryConfigWriteCommandsHaveDataArg 验证 11 条 config 写命令的 Args 都含 data Required。
//
// 涵盖:
//   - set-driver, add-device, remove-device, set-device,
//     add-module, remove-module, add-submodule, remove-submodule,
//     shield, unshield, set-idevice (11 条)
//
// 不涵盖: config-compile (无 --data, 走 DefaultBodyBuilder)
func TestRegistryConfigWriteCommandsHaveDataArg(t *testing.T) {
	configWriteNames := []string{
		"config-set-driver", "config-add-device", "config-remove-device", "config-set-device",
		"config-add-module", "config-remove-module", "config-add-submodule", "config-remove-submodule",
		"config-shield", "config-unshield", "config-set-idevice",
	}
	for _, name := range configWriteNames {
		spec, ok := LookupByName(name)
		if !ok {
			t.Errorf("Registry 缺命令: %s", name)
			continue
		}
		if len(spec.Args) != 1 || spec.Args[0].Name != "data" {
			t.Errorf("%s.Args = %+v, want [{data Required}]", name, spec.Args)
		} else if !spec.Args[0].Required {
			t.Errorf("%s.Args[0].Required = false, want true", name)
		}
		if spec.BodyBuilder == nil {
			t.Errorf("%s.BodyBuilder = nil, want non-nil", name)
		}
		// BodyBuilder 是 func 类型, 只能与 nil 比较, 不能与具体函数值比较。
		// 这里通过名字 + Function 字段间接验证 (set-idevice 用 configSetIDeviceBody,
		// 其他 10 条用 configBodyBuilder 派生的具体函数)。
	}
}

// TestRegistryConfigCompileHasNoDataArg 验证 config-compile 不需要 --data。
func TestRegistryConfigCompileHasNoDataArg(t *testing.T) {
	spec, ok := LookupByName("config-compile")
	if !ok {
		t.Fatal("找不到 config-compile")
	}
	if len(spec.Args) != 0 {
		t.Errorf("config-compile.Args = %+v, want nil/empty", spec.Args)
	}
	// BodyBuilder 是 func 类型, 只能与 nil 比较。
	if spec.BodyBuilder == nil {
		t.Error("config-compile.BodyBuilder 不应为 nil (应回退 DefaultBodyBuilder)")
	}
}

// TestIsNilSlice 验证 Go interface nil 陷阱下 isNilSlice 的正确性。
//
// 场景: nil []DecentralDevice 存入 map[string]any 后, interface{} 记住了
// 具体类型, v == nil 恒假。isNilSlice 用 reflect 穿透判断。
func TestIsNilSlice(t *testing.T) {
	// 1. 纯 nil any
	if !isNilSlice(nil) {
		t.Error("isNilSlice(nil) should be true")
	}

	// 2. nil []int — Go interface nil 陷阱
	var s []int = nil
	var iv any = s
	if isNilSlice(iv) != true {
		t.Error("isNilSlice(nil []int) should be true")
	}

	// 3. 空但非 nil 的 slice
	empty := []int{}
	iv = empty
	if isNilSlice(iv) {
		t.Error("isNilSlice([]int{}) should be false (non-nil empty slice)")
	}

	// 4. 非切片类型
	iv = 42
	if isNilSlice(iv) {
		t.Error("isNilSlice(42) should be false")
	}

	// 5. string 类型
	iv = "hello"
	if isNilSlice(iv) {
		t.Error(`isNilSlice("hello") should be false`)
	}
}
