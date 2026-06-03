// Package nrc 提供 NRC Socket 协议客户端与命令元数据集中层。
//
// commands.go 是 inl 所有 NRC 命令的"唯一权威来源"。
// 新增命令只需在 Registry 中追加一行，main.go 的 Cobra 树按 Group 自动遍历。
package nrc

import "fmt"

// Direction 区分命令是请求侧还是响应侧（用于扩展性）。
type Direction int

const (
	DirectionRequest Direction = iota
	DirectionResponse
)

// RiskLevel 命令的副作用等级，供 AI 调度决策。
type RiskLevel string

const (
	RiskRead          RiskLevel = "read"
	RiskWrite         RiskLevel = "write"
	RiskHighRiskWrite RiskLevel = "high-risk-write"
)

// CommandGroup 将 Registry 中的命令按 Cobra 顶层分组归类。
// main.go 遍历所有 Group 自动构建 cobra.Command 节点。
type CommandGroup string

const (
	GroupGsd       CommandGroup = "gsd"
	GroupDevice    CommandGroup = "device"
	GroupConfig    CommandGroup = "config"
	GroupInterface CommandGroup = "interface"
	GroupTopology  CommandGroup = "topology"
)

// ArgumentSpec 描述命令的位置参数。
// MVP 阶段写命令留空数组。
type ArgumentSpec struct {
	Name        string
	Description string
	Required    bool
}

// ResponseParser 返回响应 JSON 反序列化的目标类型。
// 返回值必须是 *T（如 *gsd.Response），用于在 client 层做统一反序列化。
// 留作 func() any 是为了避免 import cycle（nrc 不应该 import gsd/topology）。
type ResponseParser func() any

// CommandSpec 描述一个 NRC 命令的完整元数据。
// Registry 是 inl 所有命令的"唯一权威来源"，新增命令只需追加一行。
type CommandSpec struct {
	Name        string         // CLI 命令名: "gsd-list" / "device-list" / "config-set-driver"
	Code        uint16         // 帧级 Command, 请求侧用 0x9275, 响应侧 0x9271
	DataType    int            // JSON body 内的 DataType 字段: 12 / 13 / 14 ...
	Direction   Direction      // Request / Response
	Description string         // 一行说明, 用于 inl <cmd> --help
	Risk        RiskLevel      // read / write / high-risk-write
	Response    ResponseParser // 响应类型工厂, 返回 *T (MVP 阶段留 nil)

	Function    string                                                         // C++ 端 Function.Value 字符串; DataType=12 必填, 其他留空
	Group       CommandGroup                                                   // Cobra 顶层分组: gsd / device / config
	Args        []ArgumentSpec                                                 // 位置参数规格; MVP 阶段写命令留空数组
	BodyBuilder func(spec CommandSpec, args map[string]string) (string, error) // 请求体工厂; 默认 DefaultBodyBuilder
}

// Registry 命令注册表。inl 启动时遍历它构建 cobra 节点。
//
// 18 条 DataType=12 + 4 条 DataType=14 + 1 条 DataType=16 = 23 条。
// 与 C++ 源码 NetWorkTopologyFunction 分发器一一对齐：
//   - 1 条 gsd (DataType=13, read)
//   - 5 条 device (DataType=12 + Function.Value 区分, read)
//   - 12 条 config (DataType=12 + Function.Value 区分, 11 write + 1 high-risk-write)
//   - 1 条 interface (DataType=14, read)
//   - 1 条 topology (DataType=14, read)
//   - 1 条 gsd-match (DataType=16, read)
//   - 2 条 device-setup (DataType=14, write)
var Registry = []CommandSpec{
	{
		Name:        "gsd-list",
		Code:        0x9275,
		DataType:    13,
		Direction:   DirectionRequest,
		Description: "列出工业 PC 上所有 GSDML 设备驱动",
		Risk:        RiskRead,
		Response:    nil,
		Function:    "",
		Group:       GroupGsd,
		Args:        nil,
		BodyBuilder: DefaultBodyBuilder,
	},
	{
		Name:        "device-list",
		Code:        0x9275,
		DataType:    12,
		Direction:   DirectionRequest,
		Description: "列出所有已配置 PROFINET 设备 (网络配置视角)",
		Risk:        RiskRead,
		Response:    nil,
		Function:    "CallBackJson",
		Group:       GroupDevice,
		Args:        nil,
		BodyBuilder: DefaultBodyBuilder,
	},
	{
		Name:        "device-list-active",
		Code:        0x9275,
		DataType:    12,
		Direction:   DirectionRequest,
		Description: "列出当前已激活的 PROFINET 设备 (运行时视角)",
		Risk:        RiskRead,
		Response:    nil,
		Function:    "CallBackActivatedJson",
		Group:       GroupDevice,
		Args:        nil,
		BodyBuilder: DefaultBodyBuilder,
	},
	{
		Name:        "device-run",
		Code:        0x9275,
		DataType:    12,
		Direction:   DirectionRequest,
		Description: "获取当前活动运行的设备 (焊机) 状态",
		Risk:        RiskRead,
		Response:    nil,
		Function:    "GetActRun",
		Group:       GroupDevice,
		Args:        nil,
		BodyBuilder: DefaultBodyBuilder,
	},
	{
		Name:        "device-gsd-config",
		Code:        0x9275,
		DataType:    12,
		Direction:   DirectionRequest,
		Description: "读取网络配置中各设备的 GSD 文件内容",
		Risk:        RiskRead,
		Response:    nil,
		Function:    "GetGSDFileNetwork",
		Group:       GroupDevice,
		Args:        nil,
		BodyBuilder: DefaultBodyBuilder,
	},
	{
		Name:        "device-gsd-active",
		Code:        0x9275,
		DataType:    12,
		Direction:   DirectionRequest,
		Description: "读取运行时激活网络拓扑中各设备的 GSD 文件内容",
		Risk:        RiskRead,
		Response:    nil,
		Function:    "GetGSDFileActivated",
		Group:       GroupDevice,
		Args:        nil,
		BodyBuilder: DefaultBodyBuilder,
	},
	{
		Name:        "config-set-driver",
		Code:        0x9275,
		DataType:    12,
		Direction:   DirectionRequest,
		Description: "设置 PROFINET 控制器 (PN Driver) 参数",
		Risk:        RiskWrite,
		Response:    nil,
		Function:    "SetPNDriver",
		Group:       GroupConfig,
		Args:        nil,
		BodyBuilder: DefaultBodyBuilder,
	},
	{
		Name:        "config-add-device",
		Code:        0x9275,
		DataType:    12,
		Direction:   DirectionRequest,
		Description: "添加一个 PROFINET 设备到配置",
		Risk:        RiskWrite,
		Response:    nil,
		Function:    "AddPNDevice",
		Group:       GroupConfig,
		Args:        nil,
		BodyBuilder: DefaultBodyBuilder,
	},
	{
		Name:        "config-remove-device",
		Code:        0x9275,
		DataType:    12,
		Direction:   DirectionRequest,
		Description: "从配置中卸载一个 PROFINET 设备",
		Risk:        RiskWrite,
		Response:    nil,
		Function:    "UninstallPNDevice",
		Group:       GroupConfig,
		Args:        nil,
		BodyBuilder: DefaultBodyBuilder,
	},
	{
		Name:        "config-set-device",
		Code:        0x9275,
		DataType:    12,
		Direction:   DirectionRequest,
		Description: "修改一个已存在 PROFINET 设备的参数",
		Risk:        RiskWrite,
		Response:    nil,
		Function:    "SetPNDevice",
		Group:       GroupConfig,
		Args:        nil,
		BodyBuilder: DefaultBodyBuilder,
	},
	{
		Name:        "config-add-module",
		Code:        0x9275,
		DataType:    12,
		Direction:   DirectionRequest,
		Description: "向设备添加一个 Module",
		Risk:        RiskWrite,
		Response:    nil,
		Function:    "AddModule",
		Group:       GroupConfig,
		Args:        nil,
		BodyBuilder: DefaultBodyBuilder,
	},
	{
		Name:        "config-remove-module",
		Code:        0x9275,
		DataType:    12,
		Direction:   DirectionRequest,
		Description: "从设备卸载一个 Module",
		Risk:        RiskWrite,
		Response:    nil,
		Function:    "UninstallModule",
		Group:       GroupConfig,
		Args:        nil,
		BodyBuilder: DefaultBodyBuilder,
	},
	{
		Name:        "config-add-submodule",
		Code:        0x9275,
		DataType:    12,
		Direction:   DirectionRequest,
		Description: "向 Module 添加一个 Submodule",
		Risk:        RiskWrite,
		Response:    nil,
		Function:    "AddSubmodule",
		Group:       GroupConfig,
		Args:        nil,
		BodyBuilder: DefaultBodyBuilder,
	},
	{
		Name:        "config-remove-submodule",
		Code:        0x9275,
		DataType:    12,
		Direction:   DirectionRequest,
		Description: "从 Module 卸载一个 Submodule",
		Risk:        RiskWrite,
		Response:    nil,
		Function:    "UninstallSubmodule",
		Group:       GroupConfig,
		Args:        nil,
		BodyBuilder: DefaultBodyBuilder,
	},
	{
		Name:        "config-shield",
		Code:        0x9275,
		DataType:    12,
		Direction:   DirectionRequest,
		Description: "屏蔽一个设备 (屏蔽后 PLC 不再访问该设备)",
		Risk:        RiskWrite,
		Response:    nil,
		Function:    "ShieldDevice",
		Group:       GroupConfig,
		Args:        nil,
		BodyBuilder: DefaultBodyBuilder,
	},
	{
		Name:        "config-unshield",
		Code:        0x9275,
		DataType:    12,
		Direction:   DirectionRequest,
		Description: "取消屏蔽一个设备",
		Risk:        RiskWrite,
		Response:    nil,
		Function:    "UNShieldDevice",
		Group:       GroupConfig,
		Args:        nil,
		BodyBuilder: DefaultBodyBuilder,
	},
	{
		Name:        "config-compile",
		Code:        0x9275,
		DataType:    12,
		Direction:   DirectionRequest,
		Description: "编译并应用当前 PROFINET 配置 (高危, 会重启控制器)",
		Risk:        RiskHighRiskWrite,
		Response:    nil,
		Function:    "Compile",
		Group:       GroupConfig,
		Args:        nil,
		BodyBuilder: DefaultBodyBuilder,
	},
	{
		Name:        "config-set-idevice",
		Code:        0x9275,
		DataType:    12,
		Direction:   DirectionRequest,
		Description: "设置 IDevice IO 长度参数",
		Risk:        RiskWrite,
		Response:    nil,
		Function:    "SetIDevice",
		Group:       GroupConfig,
		Args:        nil,
		BodyBuilder: DefaultBodyBuilder,
	},

	// === DataType=14: DCP 相关 (PerformOnlineAccess) ===
	// 共 4 条 (Function=1/2/3/4), 由 C++ 端 PerformOnlineAccess 分发。
	// Function 字段使用整数语义 ("1" / "2" / "3" / "4"), 与 DataType=12 的字符串 Function.Value 区分。
	// 全部使用专用 BodyBuilder, 输出整数 Function 格式 (如 {"DataType":14,"Function":1,...})。
	{
		Name:        "interface-list",
		Code:        0x9275,
		DataType:    14,
		Function:    "4",
		Direction:   DirectionRequest,
		Description: "列出工业 PC 所有可用的网络端口",
		Risk:        RiskRead,
		Group:       GroupInterface,
		Args:        nil,
		BodyBuilder: interfaceListBody,
	},
	{
		Name:        "topology-scan",
		Code:        0x9275,
		DataType:    14,
		Function:    "1",
		Direction:   DirectionRequest,
		Description: "DCP 发现网络中所有 PROFINET 设备",
		Risk:        RiskRead,
		Group:       GroupTopology,
		Args: []ArgumentSpec{
			{Name: "interface", Description: "DCP 扫描端口名 (如 enp4s0)", Required: true},
		},
		BodyBuilder: topologyScanBody,
	},
	{
		Name:        "gsd-match",
		Code:        0x9275,
		DataType:    16,
		Function:    "",
		Direction:   DirectionRequest,
		Description: "DCP 发现 + 匹配在线设备与 GSD 驱动库",
		Risk:        RiskRead,
		Group:       GroupGsd,
		Args: []ArgumentSpec{
			{Name: "interface", Description: "DCP 扫描端口名 (如 enp4s0)", Required: true},
		},
		BodyBuilder: gsdMatchBody,
	},
	{
		Name:        "device-setup-name",
		Code:        0x9275,
		DataType:    14,
		Function:    "2",
		Direction:   DirectionRequest,
		Description: "通过 DCP 设置设备名称 (副作用: 无 JSON 响应, 失败时通过 BYD_TriggerErrorReport 报告)",
		Risk:        RiskWrite,
		Group:       GroupDevice,
		Args: []ArgumentSpec{
			{Name: "interface", Description: "端口名", Required: true},
			{Name: "mac", Description: "目标设备 MAC 地址", Required: true},
			{Name: "name", Description: "新设备名称", Required: true},
		},
		BodyBuilder: deviceSetupNameBody,
	},
	{
		Name:        "device-setup-ip",
		Code:        0x9275,
		DataType:    14,
		Function:    "3",
		Direction:   DirectionRequest,
		Description: "通过 DCP 设置设备 IP 和子网掩码 (副作用: 无 JSON 响应, 失败时通过 BYD_TriggerErrorReport 报告)",
		Risk:        RiskWrite,
		Group:       GroupDevice,
		Args: []ArgumentSpec{
			{Name: "interface", Description: "端口名", Required: true},
			{Name: "mac", Description: "目标设备 MAC 地址", Required: true},
			{Name: "ip", Description: "新 IP 地址", Required: true},
			{Name: "mask", Description: "新子网掩码", Required: true},
		},
		BodyBuilder: deviceSetupIPBody,
	},
}

func init() {
	seenName := make(map[string]bool, len(Registry))
	seenDT := make(map[int]bool, len(Registry))
	seenCombo := make(map[string]bool, len(Registry))
	for _, s := range Registry {
		if seenName[s.Name] {
			panic("nrc.Registry: 重复的 Name: " + s.Name)
		}
		seenName[s.Name] = true

		if s.Function == "" {
			if seenDT[s.DataType] {
				panic(fmt.Sprintf("nrc.Registry: 重复的 DataType: %d", s.DataType))
			}
			seenDT[s.DataType] = true
		} else {
			combo := fmt.Sprintf("%d:%s", s.DataType, s.Function)
			if seenCombo[combo] {
				panic("nrc.Registry: 重复的 Function: " + combo)
			}
			seenCombo[combo] = true
		}
	}
}

// LookupByName 按 Name 查找命令。返回的命令一定是 DirectionRequest。
func LookupByName(name string) (CommandSpec, bool) {
	for _, s := range Registry {
		if s.Name == name {
			return s, true
		}
	}
	return CommandSpec{}, false
}

// LookupByDataType 按 DataType 查找命令。返回的命令一定是 DirectionRequest。
// 注意: DataType=12 在 Registry 中存在多条 (不同 Function.Value), 本函数只返回第一条。
func LookupByDataType(dt int) (CommandSpec, bool) {
	for _, s := range Registry {
		if s.DataType == dt {
			return s, true
		}
	}
	return CommandSpec{}, false
}

// ExpectedResponseCode 给定请求命令, 返回对应的响应 Command 字。
// 当前所有请求都对应响应 0x9271, 但显式写出便于未来扩展（如有命令对应 0x9273）。
func ExpectedResponseCode(req CommandSpec) uint16 {
	return 0x9271
}

// DefaultBodyBuilder 是 BodyBuilder 的默认实现。
//   - spec.Function != "" → {"DataType":N,"Function":{"Value":"<Function>"}}
//   - spec.Function == "" → {"DataType":N}
func DefaultBodyBuilder(spec CommandSpec, args map[string]string) (string, error) {
	if spec.Function != "" {
		return fmt.Sprintf(`{"DataType":%d,"Function":{"Value":"%s"}}`, spec.DataType, spec.Function), nil
	}
	return fmt.Sprintf(`{"DataType":%d}`, spec.DataType), nil
}

// RequestBody 给定命令, 返回要发送的 JSON 字符串。
// 若 spec.BodyBuilder 为 nil 则回退到 DefaultBodyBuilder; 否则调用 spec.BodyBuilder。
func RequestBody(spec CommandSpec, args map[string]string) (string, error) {
	if spec.BodyBuilder == nil {
		return DefaultBodyBuilder(spec, args)
	}
	return spec.BodyBuilder(spec, args)
}

// === DataType=14/16 专用 BodyBuilder ===
//
// 下面 5 个函数对应 C++ 端 PerformOnlineAccess 的 4 个 Function 整数分支 + GSD 匹配入口 (DataType=16)。
// Function 字段使用整数格式 (如 "Function":1), 与 DataType=12 的字符串 Function.Value (如 "Function":{"Value":"CallBackJson"}) 区分。
// C++ 端通过 root["Function"] == 1/2/3/4 做整数比较, 字符串格式会失配。
//
// 共用参数 (args map) :
//   - "interface" — DCP 操作端口名 (如 "enp4s0")
//   - "mac"       — 目标设备 MAC 地址 (格式 XX:XX:XX:XX:XX:XX)
//   - "name"      — 新设备名称
//   - "ip"        — 新 IP 地址
//   - "mask"      — 新子网掩码
func interfaceListBody(spec CommandSpec, args map[string]string) (string, error) {
	return fmt.Sprintf(`{"DataType":14,"Function":4}`), nil
}

func topologyScanBody(spec CommandSpec, args map[string]string) (string, error) {
	port := args["interface"]
	return fmt.Sprintf(`{"DataType":14,"Function":1,"Portname":"%s"}`, port), nil
}

func gsdMatchBody(spec CommandSpec, args map[string]string) (string, error) {
	port := args["interface"]
	return fmt.Sprintf(`{"DataType":16,"Portname":"%s"}`, port), nil
}

func deviceSetupNameBody(spec CommandSpec, args map[string]string) (string, error) {
	port := args["interface"]
	mac := args["mac"]
	name := args["name"]
	return fmt.Sprintf(
		`{"DataType":14,"Function":2,"Portname":"%s","TargetMAC":"%s","Newdevicename":"%s"}`,
		port, mac, name), nil
}

func deviceSetupIPBody(spec CommandSpec, args map[string]string) (string, error) {
	port := args["interface"]
	mac := args["mac"]
	ip := args["ip"]
	mask := args["mask"]
	return fmt.Sprintf(
		`{"DataType":14,"Function":3,"Portname":"%s","TargetMAC":"%s","Newipaddress":"%s","Newsubnetmask":"%s"}`,
		port, mac, ip, mask), nil
}
