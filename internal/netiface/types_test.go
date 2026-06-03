package netiface

import (
	"encoding/json"
	"testing"
)

func TestListResponse_RoundTrip(t *testing.T) {
	raw := `{
		"DataType": 14,
		"PortName": ["enp4s0", "eth0"],
		"Mac":      ["68:ed:a6:0b:c4:3b", "00:1b:21:ab:cd:ef"],
		"IP":       ["192.168.3.15", "10.0.0.100"]
	}`
	var resp ListResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if resp.DataType != 14 {
		t.Errorf("DataType = %d, want 14", resp.DataType)
	}
	if len(resp.PortName) != 2 {
		t.Fatalf("PortName len = %d, want 2", len(resp.PortName))
	}
	if resp.PortName[0] != "enp4s0" {
		t.Errorf("PortName[0] = %q, want enp4s0", resp.PortName[0])
	}
	if resp.Mac[1] != "00:1b:21:ab:cd:ef" {
		t.Errorf("Mac[1] = %q, want 00:1b:21:ab:cd:ef", resp.Mac[1])
	}
	if resp.IP[0] != "192.168.3.15" {
		t.Errorf("IP[0] = %q, want 192.168.3.15", resp.IP[0])
	}

	out, err := json.Marshal(&resp)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var again ListResponse
	if err := json.Unmarshal(out, &again); err != nil {
		t.Fatalf("Re-Unmarshal failed: %v", err)
	}
	if again.PortName[0] != "enp4s0" {
		t.Errorf("round-trip PortName[0] = %q, want enp4s0", again.PortName[0])
	}
	if again.IP[1] != "10.0.0.100" {
		t.Errorf("round-trip IP[1] = %q, want 10.0.0.100", again.IP[1])
	}
}

func TestFlatten_PreservesOrder(t *testing.T) {
	raw := `{
		"DataType": 14,
		"PortName": ["enp4s0", "eth0", "wlan0"],
		"Mac":      ["68:ed:a6:0b:c4:3b", "00:1b:21:ab:cd:ef", "aa:bb:cc:dd:ee:ff"],
		"IP":       ["192.168.3.15", "10.0.0.100", "192.168.1.50"]
	}`
	var resp ListResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	ports := resp.Flatten()
	if len(ports) != 3 {
		t.Fatalf("Flatten len = %d, want 3", len(ports))
	}
	want := []Port{
		{Name: "enp4s0", Mac: "68:ed:a6:0b:c4:3b", IP: "192.168.3.15"},
		{Name: "eth0", Mac: "00:1b:21:ab:cd:ef", IP: "10.0.0.100"},
		{Name: "wlan0", Mac: "aa:bb:cc:dd:ee:ff", IP: "192.168.1.50"},
	}
	for i, p := range ports {
		if p.Name != want[i].Name {
			t.Errorf("ports[%d].Name = %q, want %q", i, p.Name, want[i].Name)
		}
		if p.Mac != want[i].Mac {
			t.Errorf("ports[%d].Mac = %q, want %q", i, p.Mac, want[i].Mac)
		}
		if p.IP != want[i].IP {
			t.Errorf("ports[%d].IP = %q, want %q", i, p.IP, want[i].IP)
		}
	}
}

func TestFlatten_EmptyArrays(t *testing.T) {
	resp := ListResponse{DataType: 14}
	ports := resp.Flatten()
	if len(ports) != 0 {
		t.Errorf("Flatten len = %d, want 0", len(ports))
	}
}
