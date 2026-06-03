package nrc

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"net"
)

const FrameSync = 0x4E66

func BuildFrame(command uint16, payload string) []byte {
	var buf bytes.Buffer

	binary.Write(&buf, binary.BigEndian, uint16(FrameSync))

	dataLen := uint16(len(payload))
	binary.Write(&buf, binary.BigEndian, dataLen)

	binary.Write(&buf, binary.BigEndian, command)

	buf.WriteString(payload)

	cmdPayload := buf.Bytes()[2:]

	crc := crc32.ChecksumIEEE(cmdPayload)
	binary.Write(&buf, binary.BigEndian, crc)

	return buf.Bytes()
}

func ReadFrame(conn net.Conn) (command uint16, data []byte, err error) {
	syncBuf := make([]byte, 2)
	if _, err = io.ReadFull(conn, syncBuf); err != nil {
		return 0, nil, fmt.Errorf("read sync: %w", err)
	}
	if sync := binary.BigEndian.Uint16(syncBuf); sync != FrameSync {
		return 0, nil, fmt.Errorf("sync byte mismatch: got %04X, want %04X", sync, FrameSync)
	}

	lenBuf := make([]byte, 2)
	if _, err = io.ReadFull(conn, lenBuf); err != nil {
		return 0, nil, fmt.Errorf("read length: %w", err)
	}
	dataLen := binary.BigEndian.Uint16(lenBuf)

	cmdBuf := make([]byte, 2)
	if _, err = io.ReadFull(conn, cmdBuf); err != nil {
		return 0, nil, fmt.Errorf("read command: %w", err)
	}

	buf := make([]byte, dataLen)
	if _, err = io.ReadFull(conn, buf); err != nil {
		return 0, nil, fmt.Errorf("read data: %w", err)
	}

	crcBuf := make([]byte, 4)
	if _, err = io.ReadFull(conn, crcBuf); err != nil {
		return 0, nil, fmt.Errorf("read crc: %w", err)
	}
	expectedCRC := binary.BigEndian.Uint32(crcBuf)

	cmdPayload := append(append([]byte{}, lenBuf...), cmdBuf...)
	cmdPayload = append(cmdPayload, buf...)
	if crc32.ChecksumIEEE(cmdPayload) != expectedCRC {
		return 0, nil, fmt.Errorf("crc mismatch: got %08X, want %08X", crc32.ChecksumIEEE(cmdPayload), expectedCRC)
	}

	command = binary.BigEndian.Uint16(cmdBuf)
	data = buf

	return command, data, nil
}
