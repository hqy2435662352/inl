package dcpdevice

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestDCPDevice_RoundTrip(t *testing.T) {
	raw := `{
		"Mac": "00:11:22:33:44:55",
		"DeviceVendorValue": "OBARA Corporation",
		"DeviceName": "heron-weld",
		"VendorID": "0x038A",
		"DeviceID": "0x0030",
		"DeviceRole": "PN设备",
		"IPAddress": "192.168.2.10",
		"SubNetMask": "255.255.255.0",
		"GateWay": "192.168.2.1"
	}`
	var dev DCPDevice
	if err := json.Unmarshal([]byte(raw), &dev); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if dev.Mac != "00:11:22:33:44:55" {
		t.Errorf("Mac = %q, want 00:11:22:33:44:55", dev.Mac)
	}
	if dev.DeviceVendorValue != "OBARA Corporation" {
		t.Errorf("DeviceVendorValue = %q", dev.DeviceVendorValue)
	}
	if dev.DeviceName != "heron-weld" {
		t.Errorf("DeviceName = %q", dev.DeviceName)
	}
	if dev.VendorID != "0x038A" {
		t.Errorf("VendorID = %q", dev.VendorID)
	}
	if dev.DeviceID != "0x0030" {
		t.Errorf("DeviceID = %q", dev.DeviceID)
	}
	if dev.DeviceRole != "PN设备" {
		t.Errorf("DeviceRole = %q", dev.DeviceRole)
	}
	if dev.IPAddress != "192.168.2.10" {
		t.Errorf("IPAddress = %q", dev.IPAddress)
	}
	if dev.SubNetMask != "255.255.255.0" {
		t.Errorf("SubNetMask = %q", dev.SubNetMask)
	}
	if dev.GateWay != "192.168.2.1" {
		t.Errorf("GateWay = %q", dev.GateWay)
	}

	out, err := json.Marshal(&dev)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var again DCPDevice
	if err := json.Unmarshal(out, &again); err != nil {
		t.Fatalf("Re-Unmarshal failed: %v", err)
	}
	if again.DeviceName != "heron-weld" {
		t.Errorf("round-trip DeviceName = %q, want heron-weld", again.DeviceName)
	}
	if again.IPAddress != "192.168.2.10" {
		t.Errorf("round-trip IPAddress = %q", again.IPAddress)
	}
}

func TestDCPDevice_NineFields(t *testing.T) {
	if got := reflect.TypeOf(DCPDevice{}).NumField(); got != 9 {
		t.Errorf("DCPDevice 字段数 = %d, want 9", got)
	}
}

func TestDCPDevice_PartialJSON(t *testing.T) {
	raw := `{"Mac":"aa:bb:cc:dd:ee:ff","DeviceName":"smc-valve-01"}`
	var dev DCPDevice
	if err := json.Unmarshal([]byte(raw), &dev); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if dev.Mac != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("Mac = %q", dev.Mac)
	}
	if dev.DeviceName != "smc-valve-01" {
		t.Errorf("DeviceName = %q", dev.DeviceName)
	}
	if dev.DeviceRole != "" {
		t.Errorf("DeviceRole 应为空, got %q", dev.DeviceRole)
	}
	if dev.VendorID != "" {
		t.Errorf("VendorID 应为空, got %q", dev.VendorID)
	}
}
