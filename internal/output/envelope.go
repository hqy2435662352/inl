package output

import (
	"encoding/json"
	"io"
)

// Envelope 是 inl 所有命令 stdout 输出的统一 JSON 信封。
//
// 借鉴 lark-cli internal/output/envelope.go:7-14 的设计:
//   - stdout = Envelope (AI 数据消费)
//   - stderr = 进度 / 警告 / 结构错误
//
// JSON 结构:
//
//	{
//	  "ok": true,
//	  "identity": "inl",
//	  "data": { ... 命令的原始响应 JSON ... },
//	  "_notice": { "command": "gsd-list", "elapsed_ms": 123 }
//	}
type Envelope struct {
	OK       bool                   `json:"ok"`
	Identity string                 `json:"identity,omitempty"`
	Data     json.RawMessage        `json:"data,omitempty"`
	Error    *Error                 `json:"error,omitempty"`
	Notice   map[string]interface{} `json:"_notice,omitempty"`
}

// WriteSuccess 将原始响应 data 包装在 Envelope 中写入 w。
// notice 是可选诊断信息 (命令名/耗时等), 为 nil 时省略 _notice 字段。
func WriteSuccess(w io.Writer, data []byte, notice map[string]interface{}) error {
	env := Envelope{
		OK:       true,
		Identity: "inl",
		Data:     data,
		Notice:   notice,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(env)
}
