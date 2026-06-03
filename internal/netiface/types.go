// Package netiface 提供 DataType=14 + Function=4 响应的领域模型。
//
// 对应 C++ 端 GeneralFunction::getInterfaceInfo() 返回的 PortName/Mac/IP 三个对齐数组。
// 三个数组按索引对应, 表示工业 PC 上同一网络接口的三个属性。
// 调用方通常使用 Flatten() 将对齐数组展开为 []Port 便于遍历。
//
// 过滤条件 (C++ 端已完成): IP 非空且 MAC ≠ 00:00:00:00:00:00。
// 本包不做额外过滤, 直接透传对齐数组。
package netiface

// ListResponse 是 DataType=14 + Function=4 的响应结构。
//
// 三个 []string 字段按索引对应:
//   - PortName[0] / Mac[0] / IP[0] 表示第 0 个网络接口
//   - PortName[1] / Mac[1] / IP[1] 表示第 1 个网络接口
//
// 示例 JSON:
//
//	{
//	  "DataType": 14,
//	  "PortName": ["enp4s0", "eth0"],
//	  "Mac":      ["68:ed:a6:0b:c4:3b", "00:1b:21:ab:cd:ef"],
//	  "IP":       ["192.168.3.15", "10.0.0.100"]
//	}
type ListResponse struct {
	DataType int      `json:"DataType"`
	PortName []string `json:"PortName"`
	Mac      []string `json:"Mac"`
	IP       []string `json:"IP"`
}

// Port 描述一个网络接口, 是 ListResponse.Flatten() 输出的扁平化元素。
type Port struct {
	// Name 网卡接口名, Linux 上为 "enp4s0" / "eth0" 等。
	Name string `json:"name"`
	// Mac 网卡 MAC 地址, 格式 XX:XX:XX:XX:XX:XX。
	Mac string `json:"mac"`
	// IP 网卡 IP 地址, IPv4 字符串如 "192.168.3.15"。
	IP string `json:"ip"`
}

// Flatten 将三个对齐数组按索引展开为 []Port。
//
// 数组长度不一致时 (异常响应), 以 PortName 长度为准。
// 超出 PortName 长度的 Mac/IP 元素被截断丢弃。
func (r *ListResponse) Flatten() []Port {
	ports := make([]Port, len(r.PortName))
	for i := range r.PortName {
		ports[i] = Port{
			Name: r.PortName[i],
			Mac:  r.Mac[i],
			IP:   r.IP[i],
		}
	}
	return ports
}
