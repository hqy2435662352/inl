// Package gsdfile 提供 Function={GetGSDFileNetwork, GetGSDFileActivated} 响应的领域模型。
//
// C++ 端语义:
//   - GetGSDFileNetwork   : 回调当前网络配置中各设备的 GSD 文件内容 (设计师视角)
//   - GetGSDFileActivated : 回调当前运行时激活网络拓扑中各设备的 GSD 文件内容 (运行时视角)
//
// 对应命令:
//   - `inl device gsd-config` → GetGSDFileNetwork
//   - `inl device gsd-active` → GetGSDFileActivated
//
// C++ 端已统一为新结构 (2026-06-01):
//   - Function 为顶层字符串, 不再是嵌套对象
//   - DataType 为顶层整数
//   - 所有业务字段平铺在顶层
//
// 响应样本:
//
//	{
//	  "DataType": 14,
//	  "Function": "GetGSDFileNetwork",
//	  "GSDFile": "...",
//	  "error": true
//	}
package gsdfile

// Response 是 Function={GetGSDFileNetwork, GetGSDFileActivated} 响应的顶层结构。
type Response struct {
	DataType int    `json:"DataType"`
	Function string `json:"Function"`
	GSDFile  string `json:"GSDFile"`
	Error    bool   `json:"error"`
}

// GSDFile 描述一个 GSDML 文件的内容 (保留向后兼容, 当前未使用)。
//
// Deprecated: 新结构 Response 中 GSDFile 字段直接为字符串。
type GSDFile struct {
	FileName string `json:"FileName"`
	Content  string `json:"Content"`
}
