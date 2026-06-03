package output

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"

	"github.com/your-org/inl/internal/nrc"
)

// DryRunFrame 描述一次 dry-run 预览的完整 NRC 帧。
// 借鉴 lark-cli cli/internal/cmdutil/dryrun.go:DryRunAPI 模式
// (因 NRC 协议无 HTTP,改为单帧结构)。
type DryRunFrame struct {
	Description string `json:"description"`
	SyncByte    string `json:"sync_byte"`
	Command     string `json:"command"`
	DataType    int    `json:"datatype"`
	Function    string `json:"function,omitempty"`
	Payload     string `json:"payload"`
	PayloadHex  string `json:"payload_hex"`
	CRC32       string `json:"crc32"`
	TotalBytes  int    `json:"total_bytes"`
	Risk        string `json:"risk"`
}

func PrintDryRunFrame(w io.Writer, spec nrc.CommandSpec, payload string) error {
	frame := nrc.BuildFrame(spec.Code, payload)

	if len(frame) < 10 {
		return fmt.Errorf("frame too short: %d bytes", len(frame))
	}

	syncByte := fmt.Sprintf("0x%04X", uint16(frame[0])<<8|uint16(frame[1]))

	dataLen := int(uint16(frame[2])<<8 | uint16(frame[3]))

	cmd := fmt.Sprintf("0x%04X", uint16(frame[4])<<8|uint16(frame[5]))

	payloadBytes := frame[6 : 6+dataLen-2]
	_ = payloadBytes

	crc := fmt.Sprintf("0x%08X",
		uint32(frame[len(frame)-4])<<24|uint32(frame[len(frame)-3])<<16|
			uint32(frame[len(frame)-2])<<8|uint32(frame[len(frame)-1]))

	out := DryRunFrame{
		Description: spec.Description,
		SyncByte:    syncByte,
		Command:     cmd,
		DataType:    spec.DataType,
		Function:    spec.Function,
		Payload:     payload,
		PayloadHex:  hex.EncodeToString([]byte(payload)),
		CRC32:       crc,
		TotalBytes:  len(frame),
		Risk:        string(spec.Risk),
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
