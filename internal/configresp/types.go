// Package configresp 提供 config 写命令 (DataType=12) 响应的领域模型。
//
// 设计依据: docs/inl-config-field-reference.md (Step 10.A, 2026-06-04 v3)。
//
// 涵盖 11 条 config 写命令的响应结构, 关键约束来自
// [inl-config-field-reference.md §0.3 错误模型不统一] 与
// [inl-config-field-reference.md §0.5 ShieldDevice 响应特殊]:
//
//   - 大多数 config 写命令 (SetPNDriver/AddPNDevice/UninstallPNDevice/AddModule/AddSubmodule/
//     UninstallModule/UninstallSubmodule) 用 `Error: []string` +
//     `ErrorID: []int` (数组模型)。
//   - SetIDevice 用 `Error: string` + `ErrorID: int` (单值模型)。
//   - ShieldDevice/UNShieldDevice 用 CallbackNTJson 模板 (v3): 自建 root 对象
//     (DataType: 14), `Function` 是 string 标签 (e.g. "ShieldDevice"),
//     `DeviceName` 和 `Result` (bool) 都在顶层; 没有 Error[]/ErrorID[]。
//
// 2026-06-04 v3 重大更新 (基于 b77ba388 提交的统一 P 网配置 JSON 响应结构):
//   - **CallbackNTJson 模板** (参考 CallbackNTJson L280-291):
//     Function 是 string 标签, 数据字段平铺到 root 顶层。
//   - **ShieldDevice 响应重构**: 原 `Function: {Value: bool, DeviceName: "..."}`
//     改为 `Function: "ShieldDevice"` (string) + `Result: bool` (顶层) +
//     `DeviceName: "..."` (顶层)。
//   - **常量拼写纠正** (提交 3d3cc3c7): "ShieldDevice" / "UNShieldDevice"
//     (之前 v2 误用 "ShildDevice" / "UNShildDevice" typo)。
//
// 2026-06-04 v2 修正 (基于新版 C++ 源反推):
//   - **Function 字段保留在响应中**: 旧版 field-reference 声称 "Function 在发送前
//     被 .clear()", 但新版 C++ 源 (SetPNDriver L511/SetPNDevice L1842/SetIDevice L3302
//     等) 实际上在 `writer.write(networktopology)` 之后才调用
//     `networktopology["Function"].clear()`, 即响应**包含** Function 字段。
//     WriteResponse 不解析 Function, 但解析者应知道响应 body 里会有 Function。
//
// WriteResponse 用 json.RawMessage 存 Error/ErrorID 以同时容忍两种模型;
// ShieldDeviceResponse 单独建模 CallbackNTJson 模板响应 (Function=string, Result/root)。
package configresp

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// WriteResponse 是大多数 config 写命令的响应模型 (10 条, 不含 ShieldDevice/UNShieldDevice)。
//
// 错误模型 (Step 10.A field-reference §0.3):
//   - Error 可能是 string 或 []string
//   - ErrorID 可能是 int 或 []int
//
// 解决方案: Error/ErrorID 用 json.RawMessage 存储原始字节, 通过 HasErrors() 统一判定。
type WriteResponse struct {
	DataType          int             `json:"DataType"`
	PNDriver          json.RawMessage `json:"PNDriver,omitempty"`
	DecentralDevice   json.RawMessage `json:"DecentralDevice,omitempty"`
	IDevice           json.RawMessage `json:"IDevice,omitempty"`
	TotalInputLength  int             `json:"TotalInputLength,omitempty"`
	TotalOutputLength int             `json:"TotalOutputLength,omitempty"`
	// 错误模型 F3: Error 可能是 string 或 []string
	Error json.RawMessage `json:"Error,omitempty"`
	// 错误模型 F3: ErrorID 可能是 int 或 []int
	ErrorID json.RawMessage `json:"ErrorID,omitempty"`
}

// HasErrors 报告响应是否包含业务错误。
//
// 容忍以下所有情形 (基于 inl-config-field-reference.md §0.3):
//   - Error: null              → false
//   - Error: []                → false (空数组)
//   - Error: ["err1"]          → true  (单元素数组)
//   - Error: ["err1","err2"]   → true  (多元素数组)
//   - Error: "some error"      → true  (单值, SetIDevice 模型)
//
// ErrorID 同理 (int vs []int), 但只要 Error 非空, 一律视为有错误。
func (r *WriteResponse) HasErrors() bool {
	return hasErrorPayload(r.Error)
}

// ErrorMessages 把 Error 字段解析为 []string。
//
// 容忍 string / []string / 缺失 / null 四种情形:
//   - 缺失或 null → 返回 nil
//   - 空数组 [] → 返回 nil
//   - string → 返回 [string]
//   - []string → 原样返回
//   - []interface{} (json 默认反序列化) → 逐元素 toString
func (r *WriteResponse) ErrorMessages() []string {
	if len(r.Error) == 0 || string(r.Error) == "null" {
		return nil
	}
	// 数组情形
	if r.Error[0] == '[' {
		var arr []json.RawMessage
		if err := json.Unmarshal(r.Error, &arr); err == nil {
			if len(arr) == 0 {
				return nil
			}
			out := make([]string, 0, len(arr))
			for _, e := range arr {
				out = append(out, unquoteString(e))
			}
			return out
		}
	}
	// 单值情形
	return []string{unquoteString(r.Error)}
}

// ErrorIDs 把 ErrorID 字段解析为 []int。
//
// 容忍 int / []int / 缺失 / null 四种情形。
func (r *WriteResponse) ErrorIDs() []int {
	if len(r.ErrorID) == 0 || string(r.ErrorID) == "null" {
		return nil
	}
	// 数组情形
	if r.ErrorID[0] == '[' {
		var arr []int
		if err := json.Unmarshal(r.ErrorID, &arr); err == nil {
			return arr
		}
	}
	// 单值情形
	var single int
	if err := json.Unmarshal(r.ErrorID, &single); err == nil {
		return []int{single}
	}
	return nil
}

// hasErrorPayload 判定 json.RawMessage 是否承载了真实的错误内容。
//
// 规则 (与 ErrorMessages 同步):
//   - 缺失 / 空 / "null" / "[]" → false
//   - 其他 → true
func hasErrorPayload(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return false
	}
	s := string(trimmed)
	if s == "null" {
		return false
	}
	if s == "[]" || s == "{}" {
		return false
	}
	// "0" 或 0 也视为无错误 (C++ 端 ErrorID=0 常表示 OK)
	if s == "0" {
		return false
	}
	return true
}

// unquoteString 尝试把 json.RawMessage 解码为字符串 (去除外层引号)。
// 失败时返回原始字节的 string 形式。
func unquoteString(raw json.RawMessage) string {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return string(raw)
}

// ShieldDeviceResponse 是 ShieldDevice/UNShieldDevice 的特殊响应 (CallbackNTJson 模板)。
//
// 2026-06-04 v3 重大重构 (提交 b77ba388):
//   - C++ 端 (PNConfigLibFileDesign.cpp L312-344) 现在用 CallbackNTJson 模板:
//     Function 是 string 标签, DeviceName 和 Result (bool) 都在顶层。
//   - 原 `Function: {Value: bool, DeviceName: "..."}` 改为
//     `Function: "ShieldDevice"` (string) + `Result: bool` (顶层) +
//     `DeviceName: "..."` (顶层)。
//
// 实机响应样本 (v3):
//
//	{
//	  "DataType": 14,
//	  "Function": "ShieldDevice",
//	  "DeviceName": "welder-01",
//	  "Result": true
//	}
//
// 注意: 请求体仍用旧模板 (Function 为 object, 内含 Value + DeviceName),
// 因为 C++ 端 NetWorkTopologyFunction 分发器仍读 `networktopology["Function"]["Value"]`
// 和 `networktopology["Function"]["DeviceName"]`。请求体重构预计在后续提交。
type ShieldDeviceResponse struct {
	DataType   int    `json:"DataType"`
	Function   string `json:"Function"`   // string 标签, e.g. "ShieldDevice" / "UNShieldDevice"
	DeviceName string `json:"DeviceName"` // 顶层, echo 设备名
	Result     bool   `json:"Result"`     // 顶层, true=成功, false=失败
}

// IsSuccess 返回 ShieldDevice 操作是否成功 (Result==true)。
func (r *ShieldDeviceResponse) IsSuccess() bool {
	return r.Result
}

// String 返回响应的可读形式, 便于 AI Agent 报告。
func (r *ShieldDeviceResponse) String() string {
	if r.IsSuccess() {
		return fmt.Sprintf("ShieldDevice 成功: Function=%q DeviceName=%q", r.Function, r.DeviceName)
	}
	return fmt.Sprintf("ShieldDevice 失败: Function=%q DeviceName=%q", r.Function, r.DeviceName)
}
