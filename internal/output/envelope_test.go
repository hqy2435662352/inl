package output

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestWriteSuccess_Basic(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteSuccess(&buf, []byte(`{"DataType":13}`), nil); err != nil {
		t.Fatal(err)
	}

	var env Envelope
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if !env.OK {
		t.Error("OK should be true")
	}
	if env.Identity != "inl" {
		t.Errorf("Identity = %q", env.Identity)
	}

	// Data 是 raw JSON, 验证可解析且字段值正确 (encoder SetIndent 会重排空白,
	// 但数字精度和字段顺序保持不变 — 这正是使用 json.RawMessage 的设计目的)
	var data struct {
		DataType int `json:"DataType"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatalf("Data 不可解析: %v (raw=%s)", err, env.Data)
	}
	if data.DataType != 13 {
		t.Errorf("Data.DataType = %d, want 13", data.DataType)
	}
	if env.Notice != nil {
		t.Error("Notice should be nil")
	}
}

func TestWriteSuccess_WithNotice(t *testing.T) {
	var buf bytes.Buffer
	notice := map[string]interface{}{
		"command":    "gsd-list",
		"elapsed_ms": 234,
	}
	WriteSuccess(&buf, []byte(`{}`), notice)

	var env Envelope
	json.Unmarshal(buf.Bytes(), &env)
	if env.Notice["command"] != "gsd-list" {
		t.Error("Notice.command mismatch")
	}
	if env.Notice["elapsed_ms"] != float64(234) {
		t.Error("Notice.elapsed_ms mismatch")
	}
}

func TestEnvelope_RoundTrip(t *testing.T) {
	data := []byte(`{"DataType":14,"Devices":[{"Mac":"00:11:22:33:44:55"}]}`)
	var buf bytes.Buffer
	WriteSuccess(&buf, data, nil)

	var env Envelope
	json.Unmarshal(buf.Bytes(), &env)
	if !env.OK {
		t.Error("OK should be true")
	}

	// 数据值精确还原 (无精度丢失, 无字段重排)
	var got struct {
		DataType int `json:"DataType"`
		Devices  []struct {
			Mac string `json:"Mac"`
		} `json:"Devices"`
	}
	if err := json.Unmarshal(env.Data, &got); err != nil {
		t.Fatalf("Data 不可解析: %v (raw=%s)", err, env.Data)
	}
	if got.DataType != 14 {
		t.Errorf("DataType = %d, want 14", got.DataType)
	}
	if len(got.Devices) != 1 {
		t.Fatalf("len(Devices) = %d, want 1", len(got.Devices))
	}
	if got.Devices[0].Mac != "00:11:22:33:44:55" {
		t.Errorf("Devices[0].Mac = %q, want 00:11:22:33:44:55", got.Devices[0].Mac)
	}
}
