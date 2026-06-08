package output

import (
	"encoding/csv"
	"fmt"
	"io"
)

// FormatCSV 将 columns + rows 渲染为 RFC 4180 CSV 格式写入 w。
//
// 规则:
//   - 行内逗号 / 引号 / 换行由 encoding/csv 自动转义
//   - 输出末尾带换行符
//   - columns 与每行 row 长度不一致时返回 error
//
// 用法: 与 ExtractTableRows 配合, 将对象数组渲染为 CSV 供 jq / awk / Excel 处理。
func FormatCSV(w io.Writer, columns []string, rows [][]string) error {
	if len(columns) == 0 {
		return nil
	}
	cw := csv.NewWriter(w)
	if err := cw.Write(columns); err != nil {
		return fmt.Errorf("CSV 写入 header 失败: %w", err)
	}
	for i, row := range rows {
		if len(row) != len(columns) {
			return fmt.Errorf("FormatCSV: 第 %d 行列宽 %d 与列数 %d 不一致", i+1, len(row), len(columns))
		}
		if err := cw.Write(row); err != nil {
			return fmt.Errorf("CSV 写入第 %d 行失败: %w", i+1, err)
		}
	}
	cw.Flush()
	return cw.Error()
}
