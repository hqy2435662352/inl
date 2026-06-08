package output

import (
	"encoding/json"
	"fmt"
	"io"
)

// FormatNDJSON 将 columns + rows 渲染为 NDJSON (Newline Delimited JSON) 写入 w。
//
// 输出格式:
//   - 第一行: columns 数组 (字段名) — 方便 header 解析
//   - 后续每行: 一个 row 对象 {列名: 值}
//
// 适合 jq / 管道处理:
//
//	inl topology scan --format ndjson | jq -c 'select(.Mac=="...")'
//
// 与 CSV 的区别: NDJSON 自带字段名, 适合 schema 演进; CSV 紧凑, 适合 Excel。
func FormatNDJSON(w io.Writer, columns []string, rows [][]string) error {
	if len(columns) == 0 {
		return nil
	}
	enc := json.NewEncoder(w)
	// 字段名行 (作为数组输出, 方便 header 解析)
	header := make([]string, len(columns))
	copy(header, columns)
	if err := enc.Encode(header); err != nil {
		return fmt.Errorf("NDJSON 写入 header 失败: %w", err)
	}
	for i, row := range rows {
		if len(row) != len(columns) {
			return fmt.Errorf("FormatNDJSON: 第 %d 行列宽 %d 与列数 %d 不一致", i+1, len(row), len(columns))
		}
		obj := make(map[string]string, len(columns))
		for j, col := range columns {
			obj[col] = row[j]
		}
		if err := enc.Encode(obj); err != nil {
			return fmt.Errorf("NDJSON 写入第 %d 行失败: %w", i+1, err)
		}
	}
	return nil
}
