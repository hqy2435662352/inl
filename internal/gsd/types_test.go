package gsd

import (
	"encoding/json"
	"testing"
)

func TestMatchResponse_RoundTrip(t *testing.T) {
	raw := `{
		"DataType": 16,
		"Devices": [
			{
				"Mac": "00:11:22:33:44:55",
				"DeviceVendorValue": "OBARA Corporation",
				"DeviceName": "heron-weld",
				"VendorID": "0x038A",
				"DeviceID": "0x0030",
				"DeviceRole": "PN设备"
			},
			{
				"Mac": "aa:bb:cc:dd:ee:ff",
				"DeviceVendorValue": "SMC Corporation",
				"DeviceName": "smc-valve-01",
				"VendorID": "0x0083",
				"DeviceID": "0x0011",
				"DeviceRole": "PN设备"
			}
		]
	}`
	var resp MatchResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if resp.DataType != 16 {
		t.Errorf("DataType = %d, want 16", resp.DataType)
	}
	if len(resp.Devices) != 2 {
		t.Fatalf("Devices len = %d, want 2", len(resp.Devices))
	}
	if resp.Devices[0].DeviceName != "heron-weld" {
		t.Errorf("Devices[0].DeviceName = %q", resp.Devices[0].DeviceName)
	}
	if resp.Devices[1].VendorID != "0x0083" {
		t.Errorf("Devices[1].VendorID = %q", resp.Devices[1].VendorID)
	}

	out, err := json.Marshal(&resp)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var again MatchResponse
	if err := json.Unmarshal(out, &again); err != nil {
		t.Fatalf("Re-Unmarshal failed: %v", err)
	}
	if len(again.Devices) != 2 {
		t.Errorf("round-trip Devices len = %d, want 2", len(again.Devices))
	}
	if again.Devices[1].DeviceID != "0x0011" {
		t.Errorf("round-trip Devices[1].DeviceID = %q", again.Devices[1].DeviceID)
	}
}

func TestUnmarshalResponse(t *testing.T) {
	raw := `{"DataType":13,"Device":[{"VendorID":"0x010C","VendorName":"HMS Industrial Networks","DeviceID":"0x0010","GSDName":"GSDML-V2.31-HMS-ABCC40-PIR-20171101.xml","MainFamily":"General","ProductFamily":"Anybus CompactCom 40 PIR","DAP":[{"DAP_ID":"DAP","DAP_Name":"DAP","DNS_CompatibleName":"ABCC40-PIR","FixedInSlots":"0","ModuleIdentNumber":"0x80010000","ReductionRatio":[1,2,4,8,16,32,64,128,256,512],"UseableModules":[{"FixedInSlots":"1","ModuleIDTarget":"ID_MODULE_ADI1"},{"FixedInSlots":"2","ModuleIDTarget":"ID_MODULE_ADI2"}]}],"Module":[{"ModuleID":"ID_MODULE_ADI1","ModuleName":"ADI#1","UseableSubmodules":null,"VirtualSubmoduleList":[{"IOData":[{"DataName":"DI Status1","DataType":"Unsigned32","InOrOut":"Output","Length":4,"UseAsBits":false}],"SubmoduleID":"ID_SUBMOD_ADI1_GROUP1","SubmoduleName":"ADI#1"}]}],"Submodules":[]}]}`

	var resp Response
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.DataType != 13 {
		t.Errorf("DataType = %d, want 13", resp.DataType)
	}
	if len(resp.Device) != 1 {
		t.Fatalf("len(Device) = %d, want 1", len(resp.Device))
	}

	d := resp.Device[0]
	if d.VendorID != "0x010C" {
		t.Errorf("VendorID = %s", d.VendorID)
	}
	if d.MainFamily != "General" {
		t.Errorf("MainFamily = %s", d.MainFamily)
	}
	if len(d.DAP) != 1 {
		t.Errorf("len(DAP) = %d, want 1", len(d.DAP))
	}
	if len(d.Module) != 1 {
		t.Errorf("len(Module) = %d, want 1", len(d.Module))
	}

	mod := d.Module[0]
	if len(mod.VirtualSubmoduleList) != 1 {
		t.Errorf("len(VirtualSubmoduleList) = %d, want 1", len(mod.VirtualSubmoduleList))
	}

	iod := mod.VirtualSubmoduleList[0].IOData[0]
	if iod.DataName != "DI Status1" {
		t.Errorf("DataName = %s", iod.DataName)
	}
	if iod.Length != 4 {
		t.Errorf("Length = %d, want 4", iod.Length)
	}

	reEncoded, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var again Response
	if err := json.Unmarshal(reEncoded, &again); err != nil {
		t.Fatalf("re-unmarshal: %v", err)
	}
	if again.Device[0].VendorName != d.VendorName {
		t.Error("round-trip failed")
	}
}

func TestUseableModulesDualMode(t *testing.T) {
	raw := `{"DAP":[{"UseableModules":[{"FixedInSlots":"1","ModuleIDTarget":"A"},{"AllowedInSlots":"3..10","AllowedInSlotsEndNumber":"10","AllowedInSlotsStartNumber":"3","ModuleIDTarget":"B","UsedInSlots":"4"}]}]}`

	var dev Device
	if err := json.Unmarshal([]byte(raw), &dev); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	ums := dev.DAP[0].UseableModules
	if len(ums) != 2 {
		t.Fatalf("len(UseableModules) = %d, want 2", len(ums))
	}

	if ums[0].FixedInSlots != "1" || ums[0].ModuleIDTarget != "A" {
		t.Error("mode A (FixedInSlots) mismatch")
	}
	if ums[1].AllowedInSlots != "3..10" || ums[1].ModuleIDTarget != "B" || ums[1].UsedInSlots != "4" {
		t.Error("mode B (AllowedInSlots) mismatch")
	}
}

func TestSiemensDAPVirtualSubmodule(t *testing.T) {
	raw := `{"Device":[{"DAP":[{"DAP_ID":"DAP1","FixedInSlots":"1","VirtualSubmoduleList":[{"IOData":[{"DataName":"传送区01","DataType":"OctetString","InOrOut":"Output"}],"SubmoduleID":"VSM_2_1000","SubmoduleName":"传送区01"}],"UseableModules":[],"ReductionRatio":[4,8],"ModuleIdentNumber":"0x80000401","DAP_Name":"CPU SR40","DNS_CompatibleName":"plc200smart"}],"Module":[],"Submodules":[],"VendorID":"0x002A","VendorName":"SIEMENS","DeviceID":"0x0119","GSDName":"test.xml","MainFamily":"PLCs","ProductFamily":"CPU SR40"}]}`

	var resp Response
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	d := resp.Device[0]
	dap := d.DAP[0]
	if len(dap.VirtualSubmoduleList) != 1 {
		t.Fatalf("DAP VirtualSubmoduleList = %d, want 1", len(dap.VirtualSubmoduleList))
	}
	if len(dap.UseableModules) != 0 {
		t.Errorf("UseableModules = %d, want 0", len(dap.UseableModules))
	}
	if len(d.Module) != 0 {
		t.Errorf("Module = %d, want 0", len(d.Module))
	}

	iod := dap.VirtualSubmoduleList[0].IOData[0]
	if iod.Length != 0 {
		t.Errorf("Length should be 0 (absent from JSON), got %d", iod.Length)
	}
}

func TestSMCSubmodulesAndUseableSubmodules(t *testing.T) {
	raw := `{"Device":[{"VendorID":"0x0083","VendorName":"SMC Corporation","DeviceID":"0x0011","GSDName":"test.xml","MainFamily":"Valves","ProductFamily":"SMC EX245","DAP":[],"Submodules":[{"IOData":[{"DataName":"Copied output 1 byte","DataType":"Unsigned8","InOrOut":"Input","UseAsBits":true}],"SubmoduleID":"ID_SUBMOD_VALVE32_OUTPUT_SHARED","SubmoduleName":"32 valves (Shared)"}],"Module":[{"ModuleID":"ID_MOD_VALVE32_OUTPUT_SHARED","ModuleName":"32 valves shared","UseableSubmodules":[{"AllowedInSubslotsEndNumber":"4","AllowedInSubslotsStartNumber":"2","SubmoduleItemTarget":"ID_SUBMOD_VALVE32_OUTPUT_SHARED","UsedInSubslots":"2"}],"VirtualSubmoduleList":[{"IOData":[{"DataName":"Output 1 byte","DataType":"Unsigned8","InOrOut":"Output","Length":1,"UseAsBits":true}],"SubmoduleID":"ID_V_SUBMOD_VALVE32_OUTPUT_SHARED","SubmoduleName":"32 valves"}]}]}]}`

	var resp Response
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	d := resp.Device[0]
	if len(d.Submodules) != 1 {
		t.Errorf("Device Submodules = %d, want 1", len(d.Submodules))
	}
	mod := d.Module[0]
	if len(mod.UseableSubmodules) != 1 {
		t.Errorf("UseableSubmodules = %d, want 1", len(mod.UseableSubmodules))
	}
	us := mod.UseableSubmodules[0]
	if us.SubmoduleItemTarget != "ID_SUBMOD_VALVE32_OUTPUT_SHARED" {
		t.Errorf("SubmoduleItemTarget = %s", us.SubmoduleItemTarget)
	}
}
