// Package topology 提供 DataType 响应的领域模型。
//
// 涵盖的响应结构:
//   - DataType=12, Function=CallBackActivatedJson → ActivatedTopologyResponse (运行时激活视角, 含 DecentralDevice 列表)
//   - DataType=14, Function=1                    → ScanResponse (DCP 发现, 含 Devices []dcpdevice.DCPDevice)
//
// C++ 端已统一为新结构 (2026-06-01):
//   - Function 为顶层字符串, 不再是嵌套对象
//   - DataType 为顶层整数
//   - 所有业务字段平铺在顶层
package topology

import (
	"encoding/json"

	"github.com/your-org/inl/internal/dcpdevice"
)

// Response 是 DataType=12 响应的顶层结构 (已弃用, 保留向后兼容)。
//
// Deprecated: 请使用 ActivatedTopologyResponse 解析新结构响应。
type Response struct {
	DataType int               `json:"DataType"`
	Stations []json.RawMessage `json:"Stations,omitempty"`
}

// ActivatedTopologyResponse 是 Function="CallBackActivatedJson" 响应 (运行时激活视角)。
//
// C++ 端含义: 回调当前运行时已激活的 PROFINET 设备列表, 含完整 Module/SubModule 层级。
// 对应命令: `inl device-list-active`。
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
