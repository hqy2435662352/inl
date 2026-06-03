// Package dcpdevice 提供 DCP (Discovery and Configuration Protocol) 发现响应的领域模型。
//
// 对应 C++ 端 PerformOnlineAccess 的 DataType=14 + Function=1 (topology scan) 与
// DataType=16 (GSD match) 响应中 Devices[] 数组的元素结构。
//
// 每个字段对应一个 DCP Block (Option, Suboption):
//   - (2, 1) DeviceVendorValue  - 设备厂商名称字符串
//   - (2, 2) DeviceName         - DCP 设备名称 (经过 TransformDeviceNameBack 反转义)
//   - (2, 3) VendorID / DeviceID - PI 分配的 16-bit ID
//   - (2, 4) DeviceRole         - 设备角色枚举
//   - (1, 2) IPAddress / SubNetMask / GateWay - 设备网络参数
//
// 包命名 "dcpdevice" 而非 "device" 是为了避免与未来可能的通用 device 包冲突,
// 也区别于现有的 devicestatus 包 (后者专用于 GetActRun 活动设备焊机状态)。
package dcpdevice

// DCPDevice 是 DCP 发现返回的单台设备信息。
//
// 对应 C++ 端 processResponseFilteredFrames 解析出的 9 字段结构。
// 在 inl 命令层, 该结构被 topology.ScanResponse.Devices 和 gsd.MatchResponse.Devices 引用。
type DCPDevice struct {
	// Mac 设备 MAC 地址, 格式 XX:XX:XX:XX:XX:XX。
	// 来源: DCP identify 响应的 StationInformation (Block (255, 255))。
	Mac string `json:"Mac"`

	// DeviceVendorValue 设备厂商名称字符串, 人类可读如 "OBARA Corporation"。
	// 来源: DCP Block (2, 1) DeviceVendorValue。
	DeviceVendorValue string `json:"DeviceVendorValue"`

	// DeviceName DCP 设备名称, 来自 PROFINET DCP 的 NameOfStation。
	// C++ 端经过 TransformDeviceNameBack 反转义 (恢复 . → - 等特殊字符)。
	// 来源: DCP Block (2, 2) DeviceName。
	DeviceName string `json:"DeviceName"`

	// VendorID PROFINET 厂商 ID, 十六进制字符串如 "0x038A"。
	// 来源: DCP Block (2, 3) VendorID。
	VendorID string `json:"VendorID"`

	// DeviceID PROFINET 设备型号 ID, 十六进制字符串如 "0x0030"。
	// 来源: DCP Block (2, 3) DeviceID。
	DeviceID string `json:"DeviceID"`

	// DeviceRole 设备角色, 由 C++ 端 Block (2, 4) 整数值映射为中文枚举字符串。
	// 已知值:
	//   "PN设备"     - 普通 PROFINET 从站设备 (Block (2,4) 整数 = 1)
	//   "PN控制器"   - PROFINET 控制器 (整数 = 2)
	//   "PN多设备"   - 多设备 (整数 = 4)
	//   "PN监视器"   - 监视器 (整数 = 8)
	DeviceRole string `json:"DeviceRole"`

	// IPAddress 设备 IP 地址, IPv4 字符串如 "192.168.2.10"。
	// 来源: DCP Block (1, 2) IPAddress。
	IPAddress string `json:"IPAddress"`

	// SubNetMask 设备子网掩码, IPv4 字符串如 "255.255.255.0"。
	// 来源: DCP Block (1, 2) SubNetMask。
	SubNetMask string `json:"SubNetMask"`

	// GateWay 设备默认网关, IPv4 字符串如 "192.168.2.1"。
	// 来源: DCP Block (1, 2) GateWay。
	GateWay string `json:"GateWay"`
}
