// Package devicestatus 提供 Function="GetActRun" 响应的领域模型。
//
// C++ 端语义: "Get Active Run" — 获取当前活动运行的设备 (通常是焊机) 状态。
// 对应命令: `inl device run`。
//
// C++ 端已统一为新结构 (2026-06-01):
//   - Function 为顶层字符串 "GetActRun", 不再是嵌套对象
//   - DataType 为顶层整数
//   - Devices 和 TotalCount 平铺在顶层
//
// 实机响应样本:
//
//	{
//	  "DataType": 14,
//	  "Function": "GetActRun",
//	  "Devices": [
//	    {"DeviceName": "heron-weld",    "Status": "连接断开"},
//	    {"DeviceName": "smc-weldsaver", "Status": "连接断开"}
//	  ],
//	  "TotalCount": 2
//	}
package devicestatus

// Response 是 Function="GetActRun" 响应的顶层结构。
type Response struct {
	DataType   int      `json:"DataType"`
	Function   string   `json:"Function"`
	Devices    []Device `json:"Devices"`
	TotalCount int      `json:"TotalCount"`
}

// Device 描述一个活动运行设备 (焊机)。
type Device struct {
	DeviceName string `json:"DeviceName"`
	Status     string `json:"Status"`
}
