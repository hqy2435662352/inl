package gsdfile

import (
	"encoding/json"
	"testing"
)

func TestResponseRoundTrip(t *testing.T) {
	raw := `{
		"DataType": 14,
		"Function": "GetGSDFileNetwork",
		"GSDFile": "<xml>...</xml>",
		"error": false
	}`
	var resp Response
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if resp.DataType != 14 {
		t.Errorf("DataType = %d, want 14", resp.DataType)
	}
	if resp.Function != "GetGSDFileNetwork" {
		t.Errorf("Function = %q, want GetGSDFileNetwork", resp.Function)
	}
	if resp.GSDFile != "<xml>...</xml>" {
		t.Errorf("GSDFile = %q, want <xml>...</xml>", resp.GSDFile)
	}
	if resp.Error != false {
		t.Errorf("Error = %v, want false", resp.Error)
	}

	out, err := json.Marshal(&resp)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var resp2 Response
	if err := json.Unmarshal(out, &resp2); err != nil {
		t.Fatalf("Re-Unmarshal failed: %v", err)
	}
	if resp2.Function != "GetGSDFileNetwork" {
		t.Errorf("round-trip Function = %q, want GetGSDFileNetwork", resp2.Function)
	}
}

func TestResponseErrorFlag(t *testing.T) {
	raw := `{
		"DataType": 14,
		"Function": "GetGSDFileActivated",
		"GSDFile": "",
		"error": true
	}`
	var resp Response
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if resp.Function != "GetGSDFileActivated" {
		t.Errorf("Function = %q, want GetGSDFileActivated", resp.Function)
	}
	if resp.GSDFile != "" {
		t.Errorf("GSDFile = %q, want empty", resp.GSDFile)
	}
	if resp.Error != true {
		t.Errorf("Error = %v, want true", resp.Error)
	}

	out, err := json.Marshal(&resp)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if len(out) == 0 {
		t.Error("序列化结果不应为空")
	}
}
