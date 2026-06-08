package nrc

import (
	"strconv"
	"strings"
	"testing"
)

func TestRegistryHasExpectedEntries(t *testing.T) {
	if len(Registry) < 17 {
		t.Fatalf("Registry 应含 17 条, 实际 %d", len(Registry))
	}
}

func TestRegistryNameUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, s := range Registry {
		if seen[s.Name] {
			t.Errorf("Name 重复: %q", s.Name)
		}
		seen[s.Name] = true
	}
}

func TestRegistryDataTypeFunctionComboUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, s := range Registry {
		// 哨兵条目 (Function=="" && DataType==0) 是透传 / 纯客户端命令,
		// init() 已对它们跳过 DataType 唯一性检查 — schema-list (纯客户端)
		// 与 raw-send (透传) 共用 DataType=0 是设计使然, 不算重复。
		if s.Function == "" && s.DataType == 0 {
			continue
		}
		var key string
		if s.Function == "" {
			key = "dt:" + strconv.Itoa(s.DataType)
		} else {
			key = "dt:" + strconv.Itoa(s.DataType) + ":fn:" + s.Function
		}
		if seen[key] {
			t.Errorf("DataType/Function 组合重复: %q", key)
		}
		seen[key] = true
	}
}

func TestLookupByName(t *testing.T) {
	spec, ok := LookupByName("gsd-list")
	if !ok {
		t.Fatal("找不到 gsd-list")
	}
	if spec.DataType != 13 {
		t.Errorf("DataType = %d, want 13", spec.DataType)
	}
	if spec.Code != 0x9275 {
		t.Errorf("Code = 0x%04X, want 0x9275", spec.Code)
	}
	if spec.Risk != RiskRead {
		t.Errorf("Risk = %q, want %q", spec.Risk, RiskRead)
	}
	if spec.Direction != DirectionRequest {
		t.Errorf("Direction = %d, want %d", spec.Direction, DirectionRequest)
	}
	if spec.Group != GroupGsd {
		t.Errorf("Group = %q, want %q", spec.Group, GroupGsd)
	}
	if spec.Function != "" {
		t.Errorf("Function = %q, want empty", spec.Function)
	}
}

func TestLookupByDataType(t *testing.T) {
	spec, ok := LookupByDataType(12)
	if !ok {
		t.Fatal("找不到 DataType=12")
	}
	if spec.Group != GroupDevice && spec.Group != GroupConfig {
		t.Errorf("DataType=12 首条 Group = %q, 应为 device 或 config", spec.Group)
	}
	if spec.Code != 0x9275 {
		t.Errorf("Code = 0x%04X, want 0x9275", spec.Code)
	}
}

func TestLookupNotFound(t *testing.T) {
	if _, ok := LookupByName("nonexistent"); ok {
		t.Error("期望 not found")
	}
	if _, ok := LookupByDataType(999); ok {
		t.Error("期望 not found")
	}
}

func TestExpectedResponseCode(t *testing.T) {
	req, _ := LookupByName("gsd-list")
	if got := ExpectedResponseCode(req); got != 0x9271 {
		t.Errorf("ExpectedResponseCode = 0x%04X, want 0x9271", got)
	}

	req2, _ := LookupByName("device-run")
	if got := ExpectedResponseCode(req2); got != 0x9271 {
		t.Errorf("ExpectedResponseCode(device-run) = 0x%04X, want 0x9271", got)
	}
}

func TestRequestBody(t *testing.T) {
	spec, _ := LookupByName("device-run")
	body, err := RequestBody(spec, nil)
	if err != nil {
		t.Fatalf("RequestBody 返回 err: %v", err)
	}
	want := `{"DataType":12,"Function":{"Value":"GetActRun"}}`
	if body != want {
		t.Errorf("RequestBody(device-run) = %q, want %q", body, want)
	}

	spec2, _ := LookupByName("gsd-list")
	body2, err := RequestBody(spec2, nil)
	if err != nil {
		t.Fatalf("RequestBody 返回 err: %v", err)
	}
	want2 := `{"DataType":13}`
	if body2 != want2 {
		t.Errorf("RequestBody(gsd-list) = %q, want %q", body2, want2)
	}
}

func TestDefaultBodyBuilder_DataType12_WithFunction(t *testing.T) {
	spec, ok := LookupByName("device-list")
	if !ok {
		t.Fatal("找不到 device-list")
	}
	got, err := DefaultBodyBuilder(spec, nil)
	if err != nil {
		t.Fatalf("DefaultBodyBuilder 返回 err: %v", err)
	}
	want := `{"DataType":12,"Function":{"Value":"CallBackJson"}}`
	if got != want {
		t.Errorf("DefaultBodyBuilder(device-list) = %q, want %q", got, want)
	}
}

func TestDefaultBodyBuilder_DataType13_NoFunction(t *testing.T) {
	spec, ok := LookupByName("gsd-list")
	if !ok {
		t.Fatal("找不到 gsd-list")
	}
	got, err := DefaultBodyBuilder(spec, nil)
	if err != nil {
		t.Fatalf("DefaultBodyBuilder 返回 err: %v", err)
	}
	want := `{"DataType":13}`
	if got != want {
		t.Errorf("DefaultBodyBuilder(gsd-list) = %q, want %q", got, want)
	}
}

// TestDefaultBodyBuilder_ShieldTypo 验证 config-shield 的 DefaultBodyBuilder
// 输出 Function.Value = "ShieldDevice" (v3 正确拼写)。
//
// 历史:
//   - 旧版 C++ 源常量: "ShieldDevice" (正确)
//   - v2 误用: "ShildDevice" (typo, 缺 'e')
//   - v3 修正: 回到 "ShieldDevice" (typo 修复提交 3d3cc3c7)
//
// 来源: io-controller/src/ioc/profinet_constants.h:82
func TestDefaultBodyBuilder_ShieldTypo(t *testing.T) {
	spec, ok := LookupByName("config-shield")
	if !ok {
		t.Fatal("找不到 config-shield")
	}
	got, err := DefaultBodyBuilder(spec, nil)
	if err != nil {
		t.Fatalf("DefaultBodyBuilder 返回 err: %v", err)
	}
	want := `{"DataType":12,"Function":{"Value":"ShieldDevice"}}`
	if got != want {
		t.Errorf("DefaultBodyBuilder(config-shield) = %q, want %q (v3 正确拼写)", got, want)
	}
}

func TestRequestBody_DefaultBuilder(t *testing.T) {
	rawSpec := CommandSpec{
		Name:        "test-default",
		Code:        0x9275,
		DataType:    12,
		Direction:   DirectionRequest,
		Description: "test",
		Risk:        RiskRead,
		Response:    nil,
		Function:    "GetActRun",
		Group:       GroupDevice,
		Args:        nil,
		BodyBuilder: nil,
	}
	got, err := RequestBody(rawSpec, nil)
	if err != nil {
		t.Fatalf("RequestBody 返回 err: %v", err)
	}
	want := `{"DataType":12,"Function":{"Value":"GetActRun"}}`
	if got != want {
		t.Errorf("RequestBody(nil builder) = %q, want %q", got, want)
	}
}

func TestRegistryHas24Entries(t *testing.T) {
	if len(Registry) != 25 {
		t.Fatalf("Registry 长度 = %d, want 25 (Step 8 新增 raw-send)", len(Registry))
	}
}

func TestRegistryByGroup(t *testing.T) {
	counts := map[CommandGroup]int{}
	for _, s := range Registry {
		counts[s.Group]++
	}
	if counts[GroupGsd] != 2 {
		t.Errorf("GroupGsd = %d, want 2", counts[GroupGsd])
	}
	if counts[GroupDevice] != 7 {
		t.Errorf("GroupDevice = %d, want 7", counts[GroupDevice])
	}
	if counts[GroupConfig] != 12 {
		t.Errorf("GroupConfig = %d, want 12", counts[GroupConfig])
	}
	if counts[GroupInterface] != 1 {
		t.Errorf("GroupInterface = %d, want 1", counts[GroupInterface])
	}
	if counts[GroupTopology] != 1 {
		t.Errorf("GroupTopology = %d, want 1", counts[GroupTopology])
	}
	if counts[GroupSchema] != 1 {
		t.Errorf("GroupSchema = %d, want 1", counts[GroupSchema])
	}
	if counts[GroupRaw] != 1 {
		t.Errorf("GroupRaw = %d, want 1 (Step 8 新增)", counts[GroupRaw])
	}
}

func TestRiskDistribution(t *testing.T) {
	counts := map[RiskLevel]int{}
	for _, s := range Registry {
		counts[s.Risk]++
	}
	if counts[RiskRead] != 10 {
		t.Errorf("RiskRead = %d, want 10", counts[RiskRead])
	}
	if counts[RiskWrite] != 14 {
		t.Errorf("RiskWrite = %d, want 14 (Step 8 新增 raw-send)", counts[RiskWrite])
	}
	if counts[RiskHighRiskWrite] != 1 {
		t.Errorf("RiskHighRiskWrite = %d, want 1", counts[RiskHighRiskWrite])
	}
}

// TestShieldFunctionNameKeepsTypo 验证 config-shield / config-unshield 的
// Function.Value 与 C++ 分发表常量拼写完全一致。
//
// 历史:
//   - 旧版 C++ 源: "ShieldDevice" / "UNShieldDevice" (正确拼写)
//   - v2 误用: "ShildDevice" / "UNShildDevice" (typo, 缺 'e')
//   - v3 修正: 重新回到 "ShieldDevice" / "UNShieldDevice" (typo 修复提交 3d3cc3c7)
//
// 来源: io-controller/src/ioc/profinet_constants.h:82-83
func TestShieldFunctionNameKeepsTypo(t *testing.T) {
	spec, ok := LookupByName("config-shield")
	if !ok {
		t.Fatal("找不到 config-shield")
	}
	if spec.Function != "ShieldDevice" {
		t.Errorf("config-shield.Function = %q, want %q (v3 正确拼写)",
			spec.Function, "ShieldDevice")
	}

	spec2, ok := LookupByName("config-unshield")
	if !ok {
		t.Fatal("找不到 config-unshield")
	}
	if spec2.Function != "UNShieldDevice" {
		t.Errorf("config-unshield.Function = %q, want %q (v3 正确拼写)",
			spec2.Function, "UNShieldDevice")
	}
}

func TestUniqueDataTypeFunctionCombo(t *testing.T) {
	seen := make(map[string]string)
	for _, s := range Registry {
		// 哨兵条目 (Function=="" && DataType==0) 跳过 (见 TestRegistryDataTypeFunctionComboUnique 注释)
		if s.Function == "" && s.DataType == 0 {
			continue
		}
		var key string
		if s.Function == "" {
			key = "dt:" + strconv.Itoa(s.DataType)
		} else {
			key = "dt:" + strconv.Itoa(s.DataType) + ":fn:" + s.Function
		}
		if prev, exists := seen[key]; exists {
			t.Errorf("重复组合 %q: %q 与 %q", key, prev, s.Name)
		}
		seen[key] = s.Name
	}
}

func TestAllEntriesHaveBodyBuilder(t *testing.T) {
	for _, s := range Registry {
		// 纯客户端命令 (Group=schema) 不发送 NRC 帧, BodyBuilder 必须为 nil
		// raw-send (Group=raw, DataType=0) 是透传命令, 必须有 BodyBuilder (rawSendBody)
		if s.Group == GroupSchema {
			if s.BodyBuilder != nil {
				t.Errorf("%s.BodyBuilder 应为 nil (纯客户端命令)", s.Name)
			}
			continue
		}
		if s.BodyBuilder == nil {
			t.Errorf("%s.BodyBuilder 为 nil", s.Name)
		}
	}
}

// === DCP Step 5 新增测试 ===

func TestLookupInterfaceList(t *testing.T) {
	spec, ok := LookupByName("interface-list")
	if !ok {
		t.Fatal("找不到 interface-list")
	}
	if spec.DataType != 14 {
		t.Errorf("DataType = %d, want 14", spec.DataType)
	}
	if spec.Function != "4" {
		t.Errorf("Function = %q, want 4", spec.Function)
	}
	if spec.Group != GroupInterface {
		t.Errorf("Group = %q, want %q", spec.Group, GroupInterface)
	}
	if spec.Risk != RiskRead {
		t.Errorf("Risk = %q, want %q", spec.Risk, RiskRead)
	}
}

func TestLookupTopologyScan(t *testing.T) {
	spec, ok := LookupByName("topology-scan")
	if !ok {
		t.Fatal("找不到 topology-scan")
	}
	if spec.DataType != 14 {
		t.Errorf("DataType = %d, want 14", spec.DataType)
	}
	if spec.Function != "1" {
		t.Errorf("Function = %q, want 1", spec.Function)
	}
	if spec.Group != GroupTopology {
		t.Errorf("Group = %q, want %q", spec.Group, GroupTopology)
	}
	if len(spec.Args) != 1 || spec.Args[0].Name != "interface" {
		t.Errorf("Args 应为 [{interface}], got %+v", spec.Args)
	}
}

func TestLookupGSDMatch(t *testing.T) {
	spec, ok := LookupByName("gsd-match")
	if !ok {
		t.Fatal("找不到 gsd-match")
	}
	if spec.DataType != 16 {
		t.Errorf("DataType = %d, want 16", spec.DataType)
	}
	if spec.Function != "" {
		t.Errorf("Function 应为空, got %q", spec.Function)
	}
	if spec.Group != GroupGsd {
		t.Errorf("Group = %q, want %q", spec.Group, GroupGsd)
	}
}

func TestLookupDeviceSetupName(t *testing.T) {
	spec, ok := LookupByName("device-setup-name")
	if !ok {
		t.Fatal("找不到 device-setup-name")
	}
	if spec.DataType != 14 {
		t.Errorf("DataType = %d, want 14", spec.DataType)
	}
	if spec.Function != "2" {
		t.Errorf("Function = %q, want 2", spec.Function)
	}
	if spec.Group != GroupDevice {
		t.Errorf("Group = %q, want %q", spec.Group, GroupDevice)
	}
	if spec.Risk != RiskWrite {
		t.Errorf("Risk = %q, want %q", spec.Risk, RiskWrite)
	}
}

func TestLookupDeviceSetupIP(t *testing.T) {
	spec, ok := LookupByName("device-setup-ip")
	if !ok {
		t.Fatal("找不到 device-setup-ip")
	}
	if spec.DataType != 14 {
		t.Errorf("DataType = %d, want 14", spec.DataType)
	}
	if spec.Function != "3" {
		t.Errorf("Function = %q, want 3", spec.Function)
	}
	if spec.Group != GroupDevice {
		t.Errorf("Group = %q, want %q", spec.Group, GroupDevice)
	}
	if spec.Risk != RiskWrite {
		t.Errorf("Risk = %q, want %q", spec.Risk, RiskWrite)
	}
}

func TestBodyBuilder_InterfaceList(t *testing.T) {
	spec, _ := LookupByName("interface-list")
	got, err := RequestBody(spec, nil)
	if err != nil {
		t.Fatalf("RequestBody err: %v", err)
	}
	want := `{"DataType":14,"Function":4}`
	if got != want {
		t.Errorf("RequestBody(interface-list) = %q, want %q", got, want)
	}
}

func TestBodyBuilder_TopologyScan(t *testing.T) {
	spec, _ := LookupByName("topology-scan")
	got, err := RequestBody(spec, map[string]string{"interface": "enp4s0"})
	if err != nil {
		t.Fatalf("RequestBody err: %v", err)
	}
	want := `{"DataType":14,"Function":1,"Portname":"enp4s0"}`
	if got != want {
		t.Errorf("RequestBody(topology-scan) = %q, want %q", got, want)
	}
}

func TestBodyBuilder_GSDMatch(t *testing.T) {
	spec, _ := LookupByName("gsd-match")
	got, err := RequestBody(spec, map[string]string{"interface": "enp4s0"})
	if err != nil {
		t.Fatalf("RequestBody err: %v", err)
	}
	want := `{"DataType":16,"Portname":"enp4s0"}`
	if got != want {
		t.Errorf("RequestBody(gsd-match) = %q, want %q", got, want)
	}
}

func TestBodyBuilder_DeviceSetupName(t *testing.T) {
	spec, _ := LookupByName("device-setup-name")
	got, err := RequestBody(spec, map[string]string{
		"interface": "enp4s0",
		"mac":       "00:11:22:33:44:55",
		"name":      "welder-01",
	})
	if err != nil {
		t.Fatalf("RequestBody err: %v", err)
	}
	want := `{"DataType":14,"Function":2,"Portname":"enp4s0","TargetMAC":"00:11:22:33:44:55","Newdevicename":"welder-01"}`
	if got != want {
		t.Errorf("RequestBody(device-setup-name) = %q, want %q", got, want)
	}
}

func TestBodyBuilder_DeviceSetupIP(t *testing.T) {
	spec, _ := LookupByName("device-setup-ip")
	got, err := RequestBody(spec, map[string]string{
		"interface": "enp4s0",
		"mac":       "00:11:22:33:44:55",
		"ip":        "192.168.2.20",
		"mask":      "255.255.255.0",
	})
	if err != nil {
		t.Fatalf("RequestBody err: %v", err)
	}
	want := `{"DataType":14,"Function":3,"Portname":"enp4s0","TargetMAC":"00:11:22:33:44:55","Newipaddress":"192.168.2.20","Newsubnetmask":"255.255.255.0"}`
	if got != want {
		t.Errorf("RequestBody(device-setup-ip) = %q, want %q", got, want)
	}
}

// === P0 修复: SetIDevice 注册测试 (2026-06-02) ===

func TestLookupConfigSetIDevice(t *testing.T) {
	spec, ok := LookupByName("config-set-idevice")
	if !ok {
		t.Fatal("找不到 config-set-idevice")
	}
	if spec.DataType != 12 {
		t.Errorf("DataType = %d, want 12", spec.DataType)
	}
	if spec.Function != "SetIDevice" {
		t.Errorf("Function = %q, want SetIDevice", spec.Function)
	}
	if spec.Group != GroupConfig {
		t.Errorf("Group = %q, want %q", spec.Group, GroupConfig)
	}
	if spec.Risk != RiskWrite {
		t.Errorf("Risk = %q, want %q", spec.Risk, RiskWrite)
	}
}

func TestBodyBuilder_ConfigSetIDevice(t *testing.T) {
	spec, _ := LookupByName("config-set-idevice")
	got, err := RequestBody(spec, map[string]string{
		"data": `{"Activate":true,"InputLength":64,"OutputLength":64}`,
	})
	if err != nil {
		t.Fatalf("RequestBody err: %v", err)
	}
	want := `{"DataType":12,"Function":{"Value":"SetIDevice"},"IDevice":{"Activate":true,"InputLength":64,"OutputLength":64}}`
	if got != want {
		t.Errorf("RequestBody(config-set-idevice) = %q, want %q", got, want)
	}
}

// === Step 9.1 config-set-idevice 参数化测试 ===

// ideviceTestSpec 是 Step 9.1 测试用的 CommandSpec (仅含 spec.Function 即可,
// Step 10.A 之后 configSetIDeviceBody 委托给 configBodyBuilder, 依赖 spec.Function 路由)。
var ideviceTestSpec = CommandSpec{DataType: 12, Function: "SetIDevice"}

func TestConfigSetIDeviceBody_Valid(t *testing.T) {
	body, err := configSetIDeviceBody(ideviceTestSpec, map[string]string{
		"data": `{"Activate":true,"InputLength":64,"OutputLength":64}`,
	})
	if err != nil {
		t.Fatalf("configSetIDeviceBody err: %v", err)
	}
	if !strings.Contains(body, `"SetIDevice"`) {
		t.Errorf("body 应含 SetIDevice, got: %s", body)
	}
	if !strings.Contains(body, `"InputLength":64`) {
		t.Errorf("body 应含 InputLength:64, got: %s", body)
	}
	if !strings.Contains(body, `"OutputLength":64`) {
		t.Errorf("body 应含 OutputLength:64, got: %s", body)
	}
}

func TestConfigSetIDeviceBody_Empty(t *testing.T) {
	_, err := configSetIDeviceBody(ideviceTestSpec, map[string]string{"data": ""})
	if err == nil {
		t.Error("期望空 --data 错误, got nil")
	}
}

func TestConfigSetIDeviceBody_InvalidJSON(t *testing.T) {
	_, err := configSetIDeviceBody(ideviceTestSpec, map[string]string{"data": "{not json}"})
	if err == nil {
		t.Error("期望非 JSON --data 错误, got nil")
	}
}

func TestConfigSetIDeviceBody_MissingDataArg(t *testing.T) {
	_, err := configSetIDeviceBody(ideviceTestSpec, map[string]string{})
	if err == nil {
		t.Error("期望缺少 --data 参数错误, got nil")
	}
}

// === Step 7 新增测试: schema-list 纯客户端命令 ===

func TestSchemaListRegistered(t *testing.T) {
	spec, ok := LookupByName("schema-list")
	if !ok {
		t.Fatal("找不到 schema-list")
	}
	if spec.DataType != 0 {
		t.Errorf("DataType = %d, want 0 (纯客户端哨兵值)", spec.DataType)
	}
	if spec.Group != GroupSchema {
		t.Errorf("Group = %q, want %q", spec.Group, GroupSchema)
	}
	if spec.Risk != RiskRead {
		t.Errorf("Risk = %q, want %q", spec.Risk, RiskRead)
	}
	if spec.Function != "" {
		t.Errorf("Function = %q, want empty", spec.Function)
	}
	if spec.Code != 0 {
		t.Errorf("Code = 0x%04X, want 0x0000", spec.Code)
	}
}

func TestSchemaListBodyBuilderIsNil(t *testing.T) {
	spec, ok := LookupByName("schema-list")
	if !ok {
		t.Fatal("找不到 schema-list")
	}
	if spec.BodyBuilder != nil {
		t.Errorf("schema-list.BodyBuilder 应为 nil, got %T", spec.BodyBuilder)
	}
}

func TestRequestBody_SchemaListReturnsEmpty(t *testing.T) {
	spec, _ := LookupByName("schema-list")
	// 纯客户端命令不应被 RequestBody 调用, 但若误调用, 默认 Builder 应回退到合法 JSON。
	// 这里验证: 即便 BodyBuilder 为 nil, DefaultBodyBuilder 也能处理 (DataType=0 + Function="" → {"DataType":0})。
	got, err := RequestBody(spec, nil)
	if err != nil {
		t.Fatalf("RequestBody err: %v", err)
	}
	want := `{"DataType":0}`
	if got != want {
		t.Errorf("RequestBody(schema-list) = %q, want %q", got, want)
	}
}

// === Step 8 新增测试: raw-send 透传命令 ===

func TestRawSendRegistered(t *testing.T) {
	spec, ok := LookupByName("raw-send")
	if !ok {
		t.Fatal("找不到 raw-send")
	}
	if spec.DataType != 0 {
		t.Errorf("DataType = %d, want 0 (哨兵, 透传)", spec.DataType)
	}
	if spec.Code != 0x9275 {
		t.Errorf("Code = 0x%04X, want 0x9275", spec.Code)
	}
	if spec.Group != GroupRaw {
		t.Errorf("Group = %q, want %q", spec.Group, GroupRaw)
	}
	if spec.Risk != RiskWrite {
		t.Errorf("Risk = %q, want %q (透传命令, 保守为 write)", spec.Risk, RiskWrite)
	}
	if spec.Function != "" {
		t.Errorf("Function = %q, want empty (透传, 不预设)", spec.Function)
	}
	if len(spec.Args) != 1 || spec.Args[0].Name != "data" {
		t.Errorf("Args 应为 [{data}], got %+v", spec.Args)
	}
	if !spec.Args[0].Required {
		t.Errorf("Args[0].Required 应为 true")
	}
}

func TestRawSendBodyBuilder(t *testing.T) {
	spec, _ := LookupByName("raw-send")
	if spec.BodyBuilder == nil {
		t.Fatal("raw-send.BodyBuilder 不应为 nil")
	}

	// 透传有效 JSON
	got, err := spec.BodyBuilder(spec, map[string]string{
		"data": `{"DataType":14,"Function":1,"Portname":"enp4s0"}`,
	})
	if err != nil {
		t.Fatalf("BodyBuilder err: %v", err)
	}
	want := `{"DataType":14,"Function":1,"Portname":"enp4s0"}`
	if got != want {
		t.Errorf("BodyBuilder(valid) = %q, want %q", got, want)
	}
}

func TestRawSendBodyBuilder_EmptyData(t *testing.T) {
	spec, _ := LookupByName("raw-send")
	_, err := spec.BodyBuilder(spec, map[string]string{"data": ""})
	if err == nil {
		t.Error("期望空 --data 错误, got nil")
	}
}

func TestRawSendBodyBuilder_InvalidJSON(t *testing.T) {
	spec, _ := LookupByName("raw-send")
	_, err := spec.BodyBuilder(spec, map[string]string{"data": "{not valid json"})
	if err == nil {
		t.Error("期望非 JSON --data 错误, got nil")
	}
}

func TestRawSendBodyBuilder_MissingDataArg(t *testing.T) {
	spec, _ := LookupByName("raw-send")
	_, err := spec.BodyBuilder(spec, map[string]string{})
	if err == nil {
		t.Error("期望缺少 --data 参数错误, got nil")
	}
}
