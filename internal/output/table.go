package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// ExtractTableRows 从响应 JSON 中递归搜索第一个非空对象数组，
// 提取公共字段作为列名，返回 columns + rows。
//
// 找不到任何对象数组时返回 nil columns（由调用方回退 JSON 输出）。
//
// 探测顺序 (深度优先):
//  1. 当前 map 的每个 value:
//     - []interface{} 且 len>0 且首元素是 map → 命中
//     - map[string]interface{} → 递归
//  2. 列名: 第一个元素的 string/int/float/bool 字段名 (按 JSON 出现顺序)
//  3. 行值: 后续元素同名字段 fmt.Sprintf 后的字符串, nil/缺省 → "-"
//
// 支持的场景:
//   - DataType=13 → Device[]                (列: VendorID, VendorName, DeviceID, ...)
//   - DataType=14 → Devices[]               (列: Mac, DeviceName, IPAddress, ...)
//   - DataType=12 → DecentralDevice[]       (列: DeviceName, IPAddress, ...)
//   - DataType=17 → Function.Devices[]      (嵌套数组, 递归命中)
func ExtractTableRows(data []byte) (columns []string, rows [][]string, err error) {
	// 解析为保留键顺序的树 (标准 json.Unmarshal 进入 map 会丢顺序)。
	root, err := parseJSONNode(data)
	if err != nil {
		return nil, nil, fmt.Errorf("JSON 解析失败: %w", err)
	}

	arr, ok := findFirstObjectArrayNode(root)
	if !ok {
		return nil, nil, nil
	}

	columns, rows = extractColumnsAndRowsFromNode(arr)
	if len(columns) == 0 {
		return nil, nil, nil
	}
	return columns, rows, nil
}

// findFirstObjectArrayNode 在 node 中深度优先搜索第一个非空对象数组。
// 命中条件: value 是 array 且 len>0 且首元素是 object。
func findFirstObjectArrayNode(node *jsonNode) (*jsonNode, bool) {
	if node == nil {
		return nil, false
	}
	if node.isArray {
		if len(node.arrayItems) > 0 && node.arrayItems[0] != nil && node.arrayItems[0].isObject {
			return node, true
		}
		// 数组首元素不是 object, 继续搜索每个元素 (例如混合数组中的 object)
		for _, item := range node.arrayItems {
			if arr, ok := findFirstObjectArrayNode(item); ok {
				return arr, true
			}
		}
		return nil, false
	}
	if node.isObject {
		// 按 JSON 出现顺序遍历 (objectOrder 保留顺序)
		for _, key := range node.objectOrder {
			child := node.objectMap[key]
			if arr, ok := findFirstObjectArrayNode(child); ok {
				return arr, true
			}
		}
	}
	return nil, false
}

// extractColumnsAndRowsFromNode 从对象数组节点中提取列名与行数据。
// 列名: 第一个元素的所有 string/int/float/bool 字段名 (按 JSON 出现顺序)。
// 行值: fmt.Sprintf("%v", value), nil 字段 → "-"。
// 缺失字段: 该行填 "-" 保持列对齐。
func extractColumnsAndRowsFromNode(arr *jsonNode) (columns []string, rows [][]string) {
	if !arr.isArray || len(arr.arrayItems) == 0 {
		return nil, nil
	}
	first := arr.arrayItems[0]
	if !first.isObject {
		return nil, nil
	}

	// 收集列名 (按 JSON 出现顺序, 仅 string/int/float/bool 标量)
	for _, key := range first.objectOrder {
		child := first.objectMap[key]
		if child == nil {
			continue
		}
		if child.isScalar {
			columns = append(columns, key)
		}
	}
	if len(columns) == 0 {
		return nil, nil
	}

	// 收集行数据
	for _, item := range arr.arrayItems {
		if !item.isObject {
			continue
		}
		row := make([]string, len(columns))
		for i, col := range columns {
			child, present := item.objectMap[col]
			if !present || child == nil || !child.isScalar || child.scalar == nil {
				row[i] = "-"
			} else {
				row[i] = fmt.Sprintf("%v", child.scalar)
			}
		}
		rows = append(rows, row)
	}

	return columns, rows
}

// === JSON 树: 保留对象键顺序 ===

// jsonNode 表示一个 JSON 值, 对象键按 JSON 出现顺序保留。
type jsonNode struct {
	isObject bool
	isArray  bool
	isScalar bool

	// 对象
	objectOrder []string
	objectMap   map[string]*jsonNode

	// 数组
	arrayItems []*jsonNode

	// 标量
	scalar interface{}
}

// parseJSONNode 从字节流解析为保留键顺序的 JSON 树。
func parseJSONNode(data []byte) (*jsonNode, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	// 不允许未知的尾随数据
	dec.UseNumber()
	node, err := parseValue(dec)
	if err != nil {
		return nil, err
	}
	return node, nil
}

// parseValue 解析一个 JSON 值, 由 dec.Token() 引导。
func parseValue(dec *json.Decoder) (*jsonNode, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			return parseObject(dec)
		case '[':
			return parseArray(dec)
		default:
			return nil, fmt.Errorf("意外的 delimiter: %v", t)
		}
	default:
		// 标量: string / float64 / Number / bool / nil
		return &jsonNode{isScalar: true, scalar: t}, nil
	}
}

func parseObject(dec *json.Decoder) (*jsonNode, error) {
	node := &jsonNode{
		isObject:  true,
		objectMap: make(map[string]*jsonNode),
	}
	for dec.More() {
		// 读取 key
		keyTok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := keyTok.(string)
		if !ok {
			return nil, fmt.Errorf("对象 key 必须是 string, got %T", keyTok)
		}
		// 读取 value
		val, err := parseValue(dec)
		if err != nil {
			return nil, err
		}
		node.objectOrder = append(node.objectOrder, key)
		node.objectMap[key] = val
	}
	// 消费右括号 '}'
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	return node, nil
}

func parseArray(dec *json.Decoder) (*jsonNode, error) {
	node := &jsonNode{isArray: true}
	for dec.More() {
		val, err := parseValue(dec)
		if err != nil {
			return nil, err
		}
		node.arrayItems = append(node.arrayItems, val)
	}
	// 消费右方括号 ']'
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	return node, nil
}

// === FormatTable ===

// FormatTable 将 columns + rows 渲染为对齐的固定宽度表格写入 w。
//
// 列宽算法: width = max(列名长度, 该列所有行值最大长度), 最小 8, 最大 40。
// 超过 maxWidth 的字符串截断为 maxWidth-3 字符 + "..." 后缀。
// nil/空字符串显示为 "-"。
//
// 错误: 列数与行宽不一致时返回 error。
func FormatTable(w io.Writer, columns []string, rows [][]string) error {
	if len(columns) == 0 {
		return nil
	}

	const (
		minWidth = 8
		maxWidth = 40
		pad      = 2 // 列间空格
	)

	widths := make([]int, len(columns))
	for i, col := range columns {
		widths[i] = len(col)
	}
	for _, row := range rows {
		if len(row) != len(columns) {
			return fmt.Errorf("FormatTable: 行宽 %d 与列数 %d 不一致", len(row), len(columns))
		}
		for i, cell := range row {
			clamped := truncate(cell, maxWidth)
			if len(clamped) > widths[i] {
				widths[i] = len(clamped)
			}
		}
	}
	for i := range widths {
		if widths[i] < minWidth {
			widths[i] = minWidth
		}
		if widths[i] > maxWidth {
			widths[i] = maxWidth
		}
	}

	// 表头
	writeRow(w, columns, widths, pad)

	// 数据行
	for _, row := range rows {
		clamped := make([]string, len(row))
		for i, cell := range row {
			if cell == "" {
				clamped[i] = "-"
			} else {
				clamped[i] = truncate(cell, maxWidth)
			}
		}
		writeRow(w, clamped, widths, pad)
	}

	return nil
}

// writeRow 写入一行, 左对齐, 列间 pad 空格。
func writeRow(w io.Writer, cells []string, widths []int, pad int) {
	for i, cell := range cells {
		if i > 0 {
			io.WriteString(w, strings.Repeat(" ", pad))
		}
		fmt.Fprintf(w, "%-*s", widths[i], cell)
	}
	io.WriteString(w, "\n")
}

// truncate 将 s 截断到 maxWidth 字符以内, 超出部分替换为 "..."。
// maxWidth <= 3 时直接返回原字符串 (避免 "..." 比原文还长)。
func truncate(s string, maxWidth int) string {
	if maxWidth <= 3 {
		return s
	}
	if len(s) <= maxWidth {
		return s
	}
	return s[:maxWidth-3] + "..."
}
