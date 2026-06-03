package nrc

import (
	"encoding/binary"
	"net"
	"testing"
)

func TestBuildFrame(t *testing.T) {
	frame := BuildFrame(0x2001, `{"robot":1,"status":0}`)

	if frame[0] != 0x4E || frame[1] != 0x66 {
		t.Fatalf("sync byte mismatch: got [%02X %02X], want [4E 66]", frame[0], frame[1])
	}

	dataLen := binary.BigEndian.Uint16(frame[2:4])
	expectedLen := uint16(len(`{"robot":1,"status":0}`))
	if dataLen != expectedLen {
		t.Fatalf("length mismatch: got %04X, want %04X", dataLen, expectedLen)
	}

	crc := binary.BigEndian.Uint32(frame[len(frame)-4:])
	if crc != 0x53DDEB72 {
		t.Fatalf("CRC mismatch: got %08X, want 53DDEB72", crc)
	}
}

func TestRoundTrip(t *testing.T) {
	payload := `{"DataType":13}`
	frame := BuildFrame(0x9275, payload)

	r, w := net.Pipe()
	go func() {
		w.Write(frame)
		w.Close()
	}()

	cmd, data, err := ReadFrame(r)
	if err != nil {
		t.Fatal(err)
	}
	if cmd != 0x9275 {
		t.Errorf("command mismatch: got %04X, want 9275", cmd)
	}
	if string(data) != payload {
		t.Errorf("data mismatch: got %s, want %s", string(data), payload)
	}
}

func TestCRCFromPDF(t *testing.T) {
	payload := `{"robot":1,"status":0}` + "\n"
	frame := BuildFrame(0x2001, payload)

	crc := binary.BigEndian.Uint32(frame[len(frame)-4:])
	if crc != 0x6B926DFF {
		t.Errorf("CRC mismatch: got %08X, want 6B926DFF", crc)
	}
}
