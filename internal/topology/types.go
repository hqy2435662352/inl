// Package topology 提供 DataType 响应的领域模型。
//
// 涵盖的响应结构:
//   - DataType=12, Function="CallBackJson"           → CallbackJsonResponse (配置中拓扑视角, 含 IDevice + PNDriver 扁平配置)
//   - DataType=12, Function="CallBackActivatedJson"  → ActivatedTopologyResponse (运行时激活视角, 含 DecentralDevice 列表)
//   - DataType=14, Function=1                        → ScanResponse (DCP 发现, 含 Devices []dcpdevice.DCPDevice)
//
// C++ 端已统一为新结构 (2026-06-01):
//   - Function 为顶层字符串, 不再是嵌套对象
//   - DataType 为顶层整数
//   - 所有业务字段平铺在顶层
//
// 反馈循环：实机响应与领域模型反向核对记录见
// `docs/protocol/field-verification.md`（2026-06-02 P0 修复同步更新）。
package topology

import (
	"github.com/your-org/inl/internal/dcpdevice"
)

// Response 是 DataType=12 响应的顶层结构 (历史占位, 不再使用)。
//
// 历史背景：v1 假设 CallBackJson 响应含 `Stations[]` 数组, 实机响应(2026-06-01)证实
// 该假设错误 — 实机无 `Stations` 字段, 而是 `IDevice`+`PNDriver` 扁平配置。
// 本类型保留仅为历史参考, 解析 CallBackJson 响应请使用 CallbackJsonResponse。
type Response struct {
	DataType int `json:"DataType"`
}

// CallbackJsonResponse 是 Function="CallBackJson" 响应 (配置中拓扑视角)。
//
// ✅ 已通过实机验证 (2026-06-02, 工业 PC 192.168.2.14)。
// C++ 端含义: 回调当前网络配置文件中的设备配置信息。
// 顶层结构: DataType + Function (字符串) + Error[] + ErrorID[] + IDevice + PNDriver
// 对应命令: `inl device list`。
//
// 实机样本: `inl/testdata/device-list_response_20260601_164426.json`。
//
// 注意：当配置中存在 PROFINET 设备时, 顶层会出现 `DecentralDevice[]` 数组;
// 本结构暂未建模该字段(实机空配置时不存在), Go json.Unmarshal 对未知字段宽容不会报错。
type CallbackJsonResponse struct {
	DataType int            `json:"DataType"`
	Function string         `json:"Function"`
	Error    []interface{}  `json:"Error"`
	ErrorID  []interface{}  `json:"ErrorID"`
	IDevice  IDeviceInfo    `json:"IDevice"`
	PNDriver PNDriverConfig `json:"PNDriver"`
}

// PNDriverConfig 是 CallBackJson 响应中的 PNDriver 信息。
//
// 与 ActivatedTopologyResponse 中 PNDriverInfo 不同：多了 iDevice 字段
// (标识此 PNDriver 是否作为 iDevice 运行)。
type PNDriverConfig struct {
	DeviceName      string `json:"DeviceName"`
	IPAddress       string `json:"IPAddress"`
	SetInTheProject bool   `json:"SetInTheProject"`
	SubnetMask      string `json:"SubnetMask"`
	IDevice         bool   `json:"iDevice,omitempty"`
}

// ActivatedTopologyResponse 是 Function="CallBackActivatedJson" 响应 (运行时激活视角)。
//
// ✅ 已通过实机验证 (2026-06-02, 工业 PC 192.168.3.15)。
// C++ 端含义: 回调当前运行时已激活的 PROFINET 设备列表, 含完整 Module/SubModule 层级。
// 对应命令: `inl device list-active`。
//
// 实机样本: `inl/testdata/device-list-active_response_20260601_164518.json`。
//
// 与 CallbackJsonResponse 的关键差异:
//   - Function 是字符串(一致), 但 DecentralDevice[] 数组取代了 IDevice+PNDriver 扁平配置
//   - PNDriver 不含 iDevice 字段 (此处为 PNDriverInfo 而非 PNDriverConfig)
//   - 顶层有 TotalInputLength / TotalOutputLength 聚合统计字段
type ActivatedTopologyResponse struct {
	DataType          int               `json:"DataType"`
	Function          string            `json:"Function"`
	DecentralDevice   []DecentralDevice `json:"DecentralDevice"`
	IDevice           IDeviceInfo       `json:"IDevice"`
	PNDriver          PNDriverInfo      `json:"PNDriver"`
	TotalInputLength  int               `json:"TotalInputLength"`
	TotalOutputLength int               `json:"TotalOutputLength"`
}

// DecentralDevice 描述一个分布式的 PROFINET 从站设备。
type DecentralDevice struct {
	DeviceID           string   `json:"DeviceID"`
	DeviceName         string   `json:"DeviceName"`
	IPAddress          string   `json:"IPAddress"`
	InputLength        int      `json:"InputLength"`
	InputStartAddress  int      `json:"InputStartAddress"`
	OutputLength       int      `json:"OutputLength"`
	OutputStartAddress int      `json:"OutputStartAddress"`
	ReductionRatio     float64  `json:"ReductionRatio"`
	RefGSD             string   `json:"RefGSD"`
	SetInTheProject    bool     `json:"SetInTheProject"`
	SubnetMask         string   `json:"SubnetMask"`
	VendorID           string   `json:"VendorID"`
	Module             []Module `json:"Module"`
}

// Module 描述设备中的一个可插拔模块。
type Module struct {
	ModuleName string      `json:"ModuleName"`
	Slot       int         `json:"Slot"`
	SubModule  []SubModule `json:"SubModule"`
}

// SubModule 描述模块中的一个子模块。
type SubModule struct {
	InputLength        int    `json:"InputLength"`
	InputStartAddress  int    `json:"InputStartAddress"`
	OutputLength       int    `json:"OutputLength"`
	OutputStartAddress int    `json:"OutputStartAddress"`
	SubModuleName      string `json:"SubModuleName"`
}

// PNDriverInfo 描述 PROFINET 控制器 (PN Driver) 的网络参数。
type PNDriverInfo struct {
	DeviceName      string `json:"DeviceName"`
	IPAddress       string `json:"IPAddress"`
	SetInTheProject bool   `json:"SetInTheProject"`
	SubnetMask      string `json:"SubnetMask"`
}

// IDeviceInfo 描述内部设备 (IDevice) 的 IO 参数。
type IDeviceInfo struct {
	Activate     bool `json:"Activate"`
	InputLength  int  `json:"InputLength"`
	OutputLength int  `json:"OutputLength"`
}

// ScanResponse 是 DataType=14 + Function=1 响应的结构 (DCP 网络发现结果)。
//
// C++ 端语义: 通过 DCP 协议在指定网卡上广播 identify 请求, 收集所有 PROFINET 设备的响应,
// 经 processResponseFilteredFrames 解析后输出到 Devices[] 数组。
// 对应命令: `inl topology scan --interface <port>`。
//
// 示例 JSON:
//
//	{
//	  "DataType": 14,
//	  "Devices": [
//	    {
//	      "Mac": "00:11:22:33:44:55",
//	      "DeviceVendorValue": "OBARA Corporation",
//	      "DeviceName": "heron-weld",
//	      "VendorID": "0x038A",
//	      "DeviceID": "0x0030",
//	      "DeviceRole": "PN设备",
//	      "IPAddress": "192.168.2.10",
//	      "SubNetMask": "255.255.255.0",
//	      "GateWay": "192.168.2.1"
//	    }
//	  ]
//	}
type ScanResponse struct {
	DataType int                   `json:"DataType"`
	Devices  []dcpdevice.DCPDevice `json:"Devices"`
}
