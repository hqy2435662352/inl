package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/your-org/inl/internal/nrc"
)

func TestPrintDryRunFrame_DataType13(t *testing.T) {
	spec, _ := nrc.LookupByName("gsd-list")
	var buf bytes.Buffer
	if err := PrintDryRunFrame(&buf, spec, `{"DataType":13}`); err != nil {
		t.Fatalf("PrintDryRunFrame failed: %v", err)
	}

	var got DryRunFrame
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if got.SyncByte != "0x4E66" {
		t.Errorf("SyncByte = %q, want 0x4E66", got.SyncByte)
	}
	if got.Command != "0x9275" {
		t.Errorf("Command = %q, want 0x9275", got.Command)
	}
	if got.DataType != 13 {
		t.Errorf("DataType = %d, want 13", got.DataType)
	}
	if got.Function != "" {
		t.Errorf("Function = %q, want empty (DataType=13)", got.Function)
	}
	if got.Risk != "read" {
		t.Errorf("Risk = %q, want read", got.Risk)
	}
}

func TestPrintDryRunFrame_DataType12_WithFunction(t *testing.T) {
	spec, _ := nrc.LookupByName("config-add-device")
	var buf bytes.Buffer
	if err := PrintDryRunFrame(&buf, spec, `{"DataType":12,"Function":{"Value":"AddPNDevice"}}`); err != nil {
		t.Fatalf("PrintDryRunFrame failed: %v", err)
	}

	var got DryRunFrame
	json.Unmarshal(buf.Bytes(), &got)

	if got.DataType != 12 {
		t.Errorf("DataType = %d, want 12", got.DataType)
	}
	if got.Function != "AddPNDevice" {
		t.Errorf("Function = %q, want AddPNDevice", got.Function)
	}
	if got.Risk != "write" {
		t.Errorf("Risk = %q, want write", got.Risk)
	}
	if !strings.Contains(got.Payload, "AddPNDevice") {
		t.Errorf("Payload 应含 AddPNDevice, got: %s", got.Payload)
	}
}

func TestPrintDryRunFrame_HighRisk(t *testing.T) {
	spec, _ := nrc.LookupByName("config-compile")
	var buf bytes.Buffer
	PrintDryRunFrame(&buf, spec, `{"DataType":12,"Function":{"Value":"Compile"}}`)

	var got DryRunFrame
	json.Unmarshal(buf.Bytes(), &got)

	if got.Risk != "high-risk-write" {
		t.Errorf("Risk = %q, want high-risk-write", got.Risk)
	}
	if got.Function != "Compile" {
		t.Errorf("Function = %q, want Compile", got.Function)
	}
}
