package topology

import (
	"encoding/json"
	"testing"
)

func TestScanResponse_RoundTrip(t *testing.T) {
	raw := `{
		"DataType": 14,
		"Devices": [
			{
				"Mac": "00:11:22:33:44:55",
				"DeviceVendorValue": "OBARA Corporation",
				"DeviceName": "heron-weld",
				"VendorID": "0x038A",
				"DeviceID": "0x0030",
				"DeviceRole": "PN设备",
				"IPAddress": "192.168.2.10",
				"SubNetMask": "255.255.255.0",
				"GateWay": "192.168.2.1"
			}
		]
	}`
	var resp ScanResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if resp.DataType != 14 {
		t.Errorf("DataType = %d, want 14", resp.DataType)
	}
	if len(resp.Devices) != 1 {
		t.Fatalf("Devices len = %d, want 1", len(resp.Devices))
	}
	dev := resp.Devices[0]
	if dev.Mac != "00:11:22:33:44:55" {
		t.Errorf("Devices[0].Mac = %q", dev.Mac)
	}
	if dev.DeviceName != "heron-weld" {
		t.Errorf("Devices[0].DeviceName = %q", dev.DeviceName)
	}
	if dev.VendorID != "0x038A" {
		t.Errorf("Devices[0].VendorID = %q", dev.VendorID)
	}

	out, err := json.Marshal(&resp)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var again ScanResponse
	if err := json.Unmarshal(out, &again); err != nil {
		t.Fatalf("Re-Unmarshal failed: %v", err)
	}
	if len(again.Devices) != 1 {
		t.Errorf("round-trip Devices len = %d, want 1", len(again.Devices))
	}
	if again.Devices[0].IPAddress != "192.168.2.10" {
		t.Errorf("round-trip IPAddress = %q", again.Devices[0].IPAddress)
	}
}

func TestResponseRoundTrip(t *testing.T) {
	raw := `{"DataType":12}`
	var resp Response
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if resp.DataType != 12 {
		t.Errorf("DataType = %d, want 12", resp.DataType)
	}
}

func TestResponseEmptyStations(t *testing.T) {
	raw := `{"DataType":12}`
	var resp Response
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if resp.DataType != 12 {
		t.Errorf("DataType = %d, want 12", resp.DataType)
	}
}

func TestActivatedTopologyResponseRoundTrip(t *testing.T) {
	raw := `{
		"DataType": 14,
		"Function": "CallBackActivatedJson",
		"DecentralDevice": [{
			"DeviceID": "0x0001",
			"DeviceName": "heron-weld",
			"IPAddress": "192.168.3.10",
			"InputLength": 64,
			"InputStartAddress": 0,
			"OutputLength": 64,
			"OutputStartAddress": 512,
			"ReductionRatio": 1.0,
			"RefGSD": "GSDML-V2.35-HMS-ABC-20231105.xml",
			"SetInTheProject": true,
			"SubnetMask": "255.255.255.0",
			"VendorID": "0x0101",
			"Module": [{
				"ModuleName": "DAP-1",
				"Slot": 1,
				"SubModule": [{
					"InputLength": 32,
					"InputStartAddress": 0,
					"OutputLength": 0,
					"OutputStartAddress": 0,
					"SubModuleName": "Input 32 byte"
				}]
			}]
		}],
		"IDevice": {"Activate": false, "InputLength": 0, "OutputLength": 0},
		"PNDriver": {"DeviceName": "profinet driver", "IPAddress": "192.168.3.15", "SetInTheProject": true, "SubnetMask": "255.255.255.0"},
		"TotalInputLength": 64,
		"TotalOutputLength": 64
	}`
	var resp ActivatedTopologyResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if resp.DataType != 14 {
		t.Errorf("DataType = %d, want 14", resp.DataType)
	}
	if resp.Function != "CallBackActivatedJson" {
		t.Errorf("Function = %q, want CallBackActivatedJson", resp.Function)
	}
	if resp.TotalInputLength != 64 {
		t.Errorf("TotalInputLength = %d, want 64", resp.TotalInputLength)
	}
	if resp.TotalOutputLength != 64 {
		t.Errorf("TotalOutputLength = %d, want 64", resp.TotalOutputLength)
	}
	if len(resp.DecentralDevice) != 1 {
		t.Fatalf("DecentralDevice len = %d, want 1", len(resp.DecentralDevice))
	}
	dd := resp.DecentralDevice[0]
	if dd.DeviceName != "heron-weld" {
		t.Errorf("DecentralDevice[0].DeviceName = %q, want heron-weld", dd.DeviceName)
	}
	if dd.DeviceID != "0x0001" {
		t.Errorf("DecentralDevice[0].DeviceID = %q, want 0x0001", dd.DeviceID)
	}
	if dd.IPAddress != "192.168.3.10" {
		t.Errorf("DecentralDevice[0].IPAddress = %q, want 192.168.3.10", dd.IPAddress)
	}
	if len(dd.Module) != 1 {
		t.Fatalf("Module len = %d, want 1", len(dd.Module))
	}
	mod := dd.Module[0]
	if mod.ModuleName != "DAP-1" {
		t.Errorf("Module[0].ModuleName = %q, want DAP-1", mod.ModuleName)
	}
	if mod.Slot != 1 {
		t.Errorf("Module[0].Slot = %d, want 1", mod.Slot)
	}
	if len(mod.SubModule) != 1 {
		t.Fatalf("SubModule len = %d, want 1", len(mod.SubModule))
	}
	sub := mod.SubModule[0]
	if sub.SubmoduleName != "Input 32 byte" {
		t.Errorf("SubModule[0].SubmoduleName = %q, want Input 32 byte", sub.SubmoduleName)
	}
	if sub.InputLength != 32 {
		t.Errorf("SubModule[0].InputLength = %d, want 32", sub.InputLength)
	}

	if resp.IDevice.Activate != false {
		t.Errorf("IDevice.Activate = %v, want false", resp.IDevice.Activate)
	}

	if resp.PNDriver.IPAddress != "192.168.3.15" {
		t.Errorf("PNDriver.IPAddress = %q, want 192.168.3.15", resp.PNDriver.IPAddress)
	}

	out, err := json.Marshal(&resp)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var resp2 ActivatedTopologyResponse
	if err := json.Unmarshal(out, &resp2); err != nil {
		t.Fatalf("Re-Unmarshal failed: %v", err)
	}
	if resp2.Function != "CallBackActivatedJson" {
		t.Errorf("round-trip Function = %q, want CallBackActivatedJson", resp2.Function)
	}
	if len(resp2.DecentralDevice) != 1 {
		t.Errorf("round-trip DecentralDevice len = %d, want 1", len(resp2.DecentralDevice))
	}
}

// TestCallbackJsonResponseRoundTrip 验证 CallbackJsonResponse 可反序列化实机响应。
//
// Fixture 来源: inl/testdata/device-list_response_20260601_164426.json
// (2026-06-01 工业 PC 192.168.2.14 实机响应)。
func TestCallbackJsonResponseRoundTrip(t *testing.T) {
	raw := `{"DataType":12,"Error":[],"ErrorID":[],"Function":"CallBackJson","IDevice":{"Activate":false,"InputLength":64,"OutputLength":64},"PNDriver":{"DeviceName":"profinetdriver","IPAddress":"192.168.2.14","SetInTheProject":true,"SubnetMask":"255.255.255.0","iDevice":false}}`
	var resp CallbackJsonResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if resp.Function != "CallBackJson" {
		t.Errorf("Function = %q, want CallBackJson", resp.Function)
	}
	if resp.PNDriver.IPAddress != "192.168.2.14" {
		t.Errorf("PNDriver.IPAddress = %q, want 192.168.2.14", resp.PNDriver.IPAddress)
	}
	if resp.PNDriver.DeviceName != "profinetdriver" {
		t.Errorf("PNDriver.DeviceName = %q, want profinetdriver", resp.PNDriver.DeviceName)
	}
	if resp.PNDriver.IDevice != false {
		t.Errorf("PNDriver.iDevice = %v, want false", resp.PNDriver.IDevice)
	}
	if resp.IDevice.InputLength != 64 {
		t.Errorf("IDevice.InputLength = %d, want 64", resp.IDevice.InputLength)
	}
	if resp.IDevice.OutputLength != 64 {
		t.Errorf("IDevice.OutputLength = %d, want 64", resp.IDevice.OutputLength)
	}
	if len(resp.Error) != 0 {
		t.Errorf("Error 应为空数组, got %d", len(resp.Error))
	}
	if len(resp.ErrorID) != 0 {
		t.Errorf("ErrorID 应为空数组, got %d", len(resp.ErrorID))
	}

	out, err := json.Marshal(&resp)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var again CallbackJsonResponse
	if err := json.Unmarshal(out, &again); err != nil {
		t.Fatalf("Re-Unmarshal failed: %v", err)
	}
	if again.PNDriver.IPAddress != "192.168.2.14" {
		t.Errorf("round-trip PNDriver.IPAddress = %q", again.PNDriver.IPAddress)
	}
	if again.Function != "CallBackJson" {
		t.Errorf("round-trip Function = %q", again.Function)
	}
}

// TestCallbackJsonResponseHasIDeviceField 验证 PNDriverConfig.iDevice=true 可正确反序列化。
func TestCallbackJsonResponseHasIDeviceField(t *testing.T) {
	raw := `{"DataType":12,"Error":[],"ErrorID":[],"Function":"CallBackJson","IDevice":{"Activate":false,"InputLength":64,"OutputLength":64},"PNDriver":{"DeviceName":"pndriver","IPAddress":"192.168.2.14","SetInTheProject":true,"SubnetMask":"255.255.255.0","iDevice":true}}`
	var resp CallbackJsonResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if !resp.PNDriver.IDevice {
		t.Error("iDevice should be true")
	}
}
