package devicestatus

import (
	"encoding/json"
	"testing"
)

func TestResponseRoundTrip(t *testing.T) {
	raw := `{
		"DataType": 14,
		"Function": "GetActRun",
		"TotalCount": 2,
		"Devices": [
			{"DeviceName": "heron-weld",    "Status": "连接断开"},
			{"DeviceName": "smc-weldsaver", "Status": "运行中"}
		]
	}`
	var resp Response
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if resp.DataType != 14 {
		t.Errorf("DataType = %d, want 14", resp.DataType)
	}
	if resp.Function != "GetActRun" {
		t.Errorf("Function = %q, want GetActRun", resp.Function)
	}
	if resp.TotalCount != 2 {
		t.Errorf("TotalCount = %d, want 2", resp.TotalCount)
	}
	if len(resp.Devices) != 2 {
		t.Fatalf("Devices len = %d, want 2", len(resp.Devices))
	}
	if resp.Devices[0].DeviceName != "heron-weld" {
		t.Errorf("Devices[0].DeviceName = %q, want heron-weld", resp.Devices[0].DeviceName)
	}
	if resp.Devices[0].Status != "连接断开" {
		t.Errorf("Devices[0].Status = %q, want 连接断开", resp.Devices[0].Status)
	}
	if resp.Devices[1].DeviceName != "smc-weldsaver" {
		t.Errorf("Devices[1].DeviceName = %q, want smc-weldsaver", resp.Devices[1].DeviceName)
	}
	if resp.Devices[1].Status != "运行中" {
		t.Errorf("Devices[1].Status = %q, want 运行中", resp.Devices[1].Status)
	}

	out, err := json.Marshal(&resp)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var resp2 Response
	if err := json.Unmarshal(out, &resp2); err != nil {
		t.Fatalf("Re-Unmarshal failed: %v", err)
	}
	if resp2.Function != "GetActRun" {
		t.Errorf("round-trip Function = %q, want GetActRun", resp2.Function)
	}
	if len(resp2.Devices) != 2 {
		t.Errorf("round-trip Devices len = %d, want 2", len(resp2.Devices))
	}
}

func TestResponseEmptyDevices(t *testing.T) {
	raw := `{"DataType":14,"Function":"GetActRun","TotalCount":0,"Devices":[]}`
	var resp Response
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if resp.Function != "GetActRun" {
		t.Errorf("Function = %q, want GetActRun", resp.Function)
	}
	if resp.TotalCount != 0 {
		t.Errorf("TotalCount = %d, want 0", resp.TotalCount)
	}
	if resp.Devices == nil {
		t.Error("Devices 不应为 nil, 至少应为空数组")
	}
	if len(resp.Devices) != 0 {
		t.Errorf("Devices len = %d, want 0", len(resp.Devices))
	}
}
