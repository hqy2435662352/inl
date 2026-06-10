// Package nrc 提供 NRC Socket 协议客户端与命令元数据集中层。
//
// commands.go 是 inl 所有 NRC 命令的"唯一权威来源"。
// 新增命令只需在 Registry 中追加一行，main.go 的 Cobra 树按 Group 自动遍历。
package nrc

import (
	"encoding/json"
	"fmt"
)

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
	GroupSchema    CommandGroup = "schema" // 纯客户端命令组: 不连接工业 PC, 仅读 inl 自身 Registry
	GroupRaw       CommandGroup = "raw"    // 透传原始 JSON 帧: 兜底覆盖非标准 NRC 命令
)

// ArgumentSpec 描述命令的位置参数。
// MVP 阶段写命令留空数组。
type ArgumentSpec struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Required    bool        `json:"required"`
	Fields      []FieldSpec `json:"fields,omitempty"` // Step 11: --data JSON 子字段元数据
}

// FieldSpec 描述 --data JSON 内部的一个子字段。
// 仅当 ArgumentSpec.Name == "data" 时使用；DCP 参数（interface/mac/name 等）不使用。
type FieldSpec struct {
	Name        string `json:"name"`              // 字段名，如 "RefGSD"
	Type        string `json:"type"`              // "string" | "int" | "bool" | "object"
	Description string `json:"description"`       // 含义说明
	Required    bool   `json:"required"`          // 在 --data JSON 内是否必填
	Example     string `json:"example,omitempty"` // 示例值（可选）
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
// 18 条 DataType=12 + 4 条 DataType=14 + 1 条 DataType=16 + 1 条纯客户端 = 24 条。
// 与 C++ 源码 NetWorkTopologyFunction 分发器一一对齐：
//   - 1 条 gsd (DataType=13, read)
//   - 5 条 device (DataType=12 + Function.Value 区分, read)
//   - 12 条 config (DataType=12 + Function.Value 区分, 11 write + 1 high-risk-write)
//   - 1 条 interface (DataType=14, read)
//   - 1 条 topology (DataType=14, read)
//   - 1 条 gsd-match (DataType=16, read)
//   - 2 条 device-setup (DataType=14, write)
//   - 1 条 schema (DataType=0, 纯客户端, 不发 NRC 帧)
var Registry = []CommandSpec{
	{
		Name:        "gsd-list",
		Code:        0x9275,
		DataType:    13,
		Direction:   DirectionRequest,
		Description: "列出工业 PC 上安装的设备GSDML文件数据",
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
		Description: "列出配置中/未编译的网络拓扑数据 (网络配置视角)",
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
		Description: "列出当前已激活/编译成功的网络拓扑数据 (运行时视角)",
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
		Description: "获取当前已激活的从站设备(不包括PN Driver和i-Device)的连接状态 (运行时视角)",
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
		Description: "设置配置态中 PROFINET 主站 (PN Driver即机器人本身) 的参数 (DeviceName/IPAddress/SubnetMask/SetInTheProject; 注意: config 只改配置态 networktopology.json, 不改实际设备参数)",
		Risk:        RiskWrite,
		Response:    nil,
		Function:    "SetPNDriver",
		Group:       GroupConfig,
		Args: []ArgumentSpec{{
			Name: "data", Required: true,
			Description: "PNDriver 参数 JSON（业务字段在顶层 PNDriver 对象）",
			Fields: []FieldSpec{
				{Name: "DeviceName", Type: "string", Required: true, Description: "PROFINET 主站设备名称"},
				{Name: "IPAddress", Type: "string", Required: true, Description: "主站 IP 地址"},
				{Name: "SubnetMask", Type: "string", Required: true, Description: "子网掩码"},
				{Name: "SetInTheProject", Type: "bool", Required: false, Description: "参数在项目中设置（默认 true，当false时其余3个参数传入不生效）"},
			},
		}},
		BodyBuilder: configSetDriverBody,
	},
	{
		Name:        "config-add-device",
		Code:        0x9275,
		DataType:    12,
		Direction:   DirectionRequest,
		Description: "添加一个 PROFINET 设备到配置中/未编译的网络拓扑中 (提供 RefGSD/DAP_ID, 其他字段 C++ 自动生成)",
		Risk:        RiskWrite,
		Response:    nil,
		Function:    "AddPNDevice",
		Group:       GroupConfig,
		Args: []ArgumentSpec{{
			Name: "data", Required: true,
			Description: "添加设备的业务字段 JSON（Function 对象下）",
			Fields: []FieldSpec{
				{Name: "RefGSD", Type: "string", Required: true, Description: "GSDML 文件名", Example: "GSDML-V2.4-HERON-12345678.xml"},
				{Name: "DAP_ID", Type: "string", Required: true, Description: "设备接口描述", Example: "DAP"},
			},
		}},
		BodyBuilder: configAddDeviceBody,
	},
	{
		Name:        "config-remove-device",
		Code:        0x9275,
		DataType:    12,
		Direction:   DirectionRequest,
		Description: "从配置中/未编译的网络拓扑中卸载一个 PROFINET 设备",
		Risk:        RiskWrite,
		Response:    nil,
		Function:    "UninstallPNDevice",
		Group:       GroupConfig,
		Args: []ArgumentSpec{{
			Name: "data", Required: true,
			Description: "卸载设备的索引 JSON（Function 下）",
			Fields: []FieldSpec{
				{Name: "SetPNDeviceNum", Type: "int", Required: true,
					Description: "目标设备索引号(1-based 索引，对应 device-list 中的DecentralDevice[] 的0-based索引 , 即传入1就是卸载DecentralDevice[0])"},
			},
		}},
		BodyBuilder: configRemoveDeviceBody,
	},
	{
		Name:        "config-set-device",
		Code:        0x9275,
		DataType:    12,
		Direction:   DirectionRequest,
		Description: "设置配置中/未编译的网络拓扑中一个分散设备（DecentralDevice）的网络参数",
		Risk:        RiskWrite,
		Response:    nil,
		Function:    "SetPNDevice",
		Group:       GroupConfig,
		Args: []ArgumentSpec{{
			Name: "data", Required: true,
			Description: "分散设备的网络参数",
			Fields: []FieldSpec{
				{Name: "SetPNDeviceNum", Type: "int", Required: true,
					Description: "目标设备索引号(1-based 索引，对应 device-list 中的DecentralDevice[] 的0-based索引 , 即传入1就是设置DecentralDevice[0])"},
				{Name: "DeviceName", Type: "string", Required: false,
					Description: "设备名称（可选, 不传则保留原值）"},
				{Name: "IPAddress", Type: "string", Required: false,
					Description: "设备 IP 地址（可选, 不传则保留原值）"},
				{Name: "SubnetMask", Type: "string", Required: false,
					Description: "子网掩码（可选, 不传则保留原值）"},
				{Name: "ReductionRatio", Type: "int", Required: false,
					Description: "设备更新周期（可选, 不传则保留原值）"},
				{Name: "SetInTheProject", Type: "bool", Required: false,
					Description: "参数在项目中设置（默认 true，当false时DeviceName/IPAddress/SubnetMask传入不生效）"},
			},
		}},
		BodyBuilder: configSetDeviceBody,
	},
	{
		Name:        "config-add-module",
		Code:        0x9275,
		DataType:    12,
		Direction:   DirectionRequest,
		Description: "向配置中/未编译的网络拓扑中一个分散设备添加一个 Module",
		Risk:        RiskWrite,
		Response:    nil,
		Function:    "AddModule",
		Group:       GroupConfig,
		Args: []ArgumentSpec{{
			Name: "data", Required: true,
			Description: "添加模块的索引 JSON（Function 下）",
			Fields: []FieldSpec{
				{Name: "SetPNDeviceNum", Type: "int", Required: true, Description: "目标设备索引号(1-based 索引，对应 device-list 中的DecentralDevice[] 的0-based索引 , 即传入1就是添加到DecentralDevice[0])"},
				{Name: "ModuleID", Type: "string", Required: true, Description: "模块 ID"},
			},
		}},
		BodyBuilder: configAddModuleBody,
	},
	{
		Name:        "config-remove-module",
		Code:        0x9275,
		DataType:    12,
		Direction:   DirectionRequest,
		Description: "从配置中/未编译的网络拓扑中一个分散设备卸载一个 Module",
		Risk:        RiskWrite,
		Response:    nil,
		Function:    "UninstallModule",
		Group:       GroupConfig,
		Args: []ArgumentSpec{{
			Name: "data", Required: true,
			Description: "移除模块的索引 JSON（Function 下）",
			Fields: []FieldSpec{
				{Name: "SetPNDeviceNum", Type: "int", Required: true, Description: "目标设备索引号(1-based 索引，对应 device-list 中的DecentralDevice[] 的0-based索引 , 即传入1就是移除DecentralDevice[0]的模块)"},
				{Name: "SetModuleSlot", Type: "int", Required: true, Description: "目标模块插槽号(1-based 索引，对应 device-list 中的Module的Slot参数, 即传入1就是移除DecentralDevice槽号为1的模块)"},
			},
		}},
		BodyBuilder: configRemoveModuleBody,
	},
	{
		Name:        "config-add-submodule",
		Code:        0x9275,
		DataType:    12,
		Direction:   DirectionRequest,
		Description: "向配置中/未编译的网络拓扑中一个分散设备的 Module 添加一个 Submodule",
		Risk:        RiskWrite,
		Response:    nil,
		Function:    "AddSubmodule",
		Group:       GroupConfig,
		Args: []ArgumentSpec{{
			Name: "data", Required: true,
			Description: "添加子模块的索引 JSON（Function 下）",
			Fields: []FieldSpec{
				{Name: "SetPNDeviceNum", Type: "int", Required: true, Description: "目标设备索引号(1-based 索引，对应 device-list 中的DecentralDevice[] 的0-based索引 , 即传入1就是添加到DecentralDevice[0])"},
				{Name: "ModuleID", Type: "string", Required: true, Description: "模块 ID"},
				{Name: "SubmoduleID", Type: "string", Required: true, Description: "子模块 ID"},
			},
		}},
		BodyBuilder: configAddSubmoduleBody,
	},
	{
		Name:        "config-remove-submodule",
		Code:        0x9275,
		DataType:    12,
		Direction:   DirectionRequest,
		Description: "从配置中/未编译的网络拓扑中一个分散设备的 Module 卸载一个 Submodule",
		Risk:        RiskWrite,
		Response:    nil,
		Function:    "UninstallSubmodule",
		Group:       GroupConfig,
		Args: []ArgumentSpec{{
			Name: "data", Required: true,
			Description: "移除子模块的索引 JSON（Function 下）",
			Fields: []FieldSpec{
				{Name: "SetPNDeviceNum", Type: "int", Required: true, Description: "目标设备索引号(1-based 索引，对应 device-list 中的DecentralDevice[] 的0-based索引 , 即传入1就是移除DecentralDevice[0]的子模块)"},
				{Name: "SetModuleSlot", Type: "int", Required: true, Description: "目标模块插槽号(1-based 索引，对应 device-list 中的Module的Slot参数, 即传入1就是移除DecentralDevice槽号为1的模块的子模块)"},
				{Name: "SetSubmoduleSlot", Type: "int", Required: true, Description: "子模块插槽号（1-based）对应 device-list 中的Module的Submodules的Subslot参数，即传入1就是移除DecentralDevice的模块的子槽号为1的子模块"},
			},
		}},
		BodyBuilder: configRemoveSubmoduleBody,
	},
	{
		Name:        "config-shield",
		Code:        0x9275,
		DataType:    12,
		Direction:   DirectionRequest,
		Description: "屏蔽一个设备 (运行时生效: 屏蔽后机器人系统不再报出该设备的通信错误; 不走 compile, 响应特殊: Function.Value 返回 bool)",
		Risk:        RiskWrite,
		Response:    nil,
		// 2026-06-04 v3: C++ 源常量拼写已纠正回 "ShieldDevice" (typo 修复提交 3d3cc3c7)
		// 来源: io-controller/src/ioc/profinet_constants.h:82
		Function: "ShieldDevice",
		Group:    GroupConfig,
		Args: []ArgumentSpec{{
			Name: "data", Required: true,
			Description: "屏蔽设备的标识 JSON（Function 下）",
			Fields: []FieldSpec{
				{Name: "DeviceName", Type: "string", Required: true,
					Description: "要屏蔽的设备名称（不是索引号）"},
			},
		}},
		BodyBuilder: configShieldBody,
	},
	{
		Name:        "config-unshield",
		Code:        0x9275,
		DataType:    12,
		Direction:   DirectionRequest,
		Description: "取消屏蔽一个设备 (运行时生效: 取消屏蔽后机器人系统会监控该设备的通信错误; 不走 compile, 响应特殊: Function.Value 返回 bool)",
		Risk:        RiskWrite,
		Response:    nil,
		// 2026-06-04 v3: C++ 源常量拼写已纠正回 "UNShieldDevice" (typo 修复提交 3d3cc3c7)
		// 来源: io-controller/src/ioc/profinet_constants.h:83
		Function: "UNShieldDevice",
		Group:    GroupConfig,
		Args: []ArgumentSpec{{
			Name: "data", Required: true,
			Description: "取消屏蔽的设备标识 JSON（Function 下）",
			Fields: []FieldSpec{
				{Name: "DeviceName", Type: "string", Required: true,
					Description: "要取消屏蔽的设备名称"},
			},
		}},
		BodyBuilder: configUnshieldBody,
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
		Description: "设置 IDevice 的参数 (Activate/InputLength/OutputLength)",
		Risk:        RiskWrite,
		Response:    nil,
		Function:    "SetIDevice",
		Group:       GroupConfig,
		Args: []ArgumentSpec{{
			Name: "data", Required: true,
			Description: "IDevice 参数 JSON（业务字段在顶层 IDevice 对象）",
			Fields: []FieldSpec{
				{Name: "Activate", Type: "bool", Required: true, Description: "是否激活 IDevice"},
				{Name: "InputLength", Type: "int", Required: true, Description: "输入数据长度（字节）"},
				{Name: "OutputLength", Type: "int", Required: true, Description: "输出数据长度（字节）"},
			},
		}},
		BodyBuilder: configSetIDeviceBody,
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
			{Name: "interface", Description: "DCP 扫描端口名 (默认enp4s0或pnio)", Required: true},
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
			{Name: "interface", Description: "DCP 扫描端口名 (默认enp4s0或pnio)", Required: true},
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

	// === Group=schema: 纯客户端命令 (不发 NRC 帧, 不连工业 PC) ===
	// DataType=0 / Function="" 是哨兵值, 在 runNrcCommand 中检查 spec.Group == GroupSchema 短路处理。
	// init() 已对 Function=="" && DataType==0 的条目跳过 DataType 唯一性检查。
	{
		Name:        "schema-list",
		Code:        0,
		DataType:    0,
		Direction:   DirectionRequest,
		Description: "列出所有可用命令的元数据 (供 AI Agent 自发现能力)",
		Risk:        RiskRead,
		Response:    nil,
		Function:    "",
		Group:       GroupSchema,
		Args:        nil,
		BodyBuilder: nil,
	},

	// === Group=raw: 透传原始 JSON 帧 (兜底覆盖非标准 NRC 命令) ===
	// DataType=0 是哨兵, init() 唯一性检查对 Function=="" && DataType==0 跳过。
	// Code=0x9275 是 NRC 请求命令字, 但 DataType 来自用户提供的 --data payload, 不会被 DefaultBodyBuilder 覆盖。
	// Risk=write: 无法预判 payload 副作用, 默认要求 --yes。
	{
		Name:        "raw-send",
		Code:        0x9275,
		DataType:    0,
		Direction:   DirectionRequest,
		Description: "透传任意 JSON 帧到工业 PC (兜底覆盖非标准 NRC 命令)",
		Risk:        RiskWrite,
		Response:    nil,
		Function:    "",
		Group:       GroupRaw,
		Args: []ArgumentSpec{{
			Name: "data", Required: true,
			Description: "原始 NRC JSON payload（透传，无字段约束）",
			// Fields 保持 nil — raw-send payload 是自由格式
		}},
		BodyBuilder: rawSendBody,
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

		// 纯客户端命令 (Group=schema, Function="" 且 DataType=0) 不发送 NRC 帧,
		// 跳过 DataType:Function 唯一性检查, 允许多个纯客户端命令共存。
		if s.Function == "" && s.DataType == 0 {
			continue
		}

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

// === raw-send 专用 BodyBuilder ===
//
// rawSendBody 透传用户提供的 --data 字符串作为 NRC 帧 payload, 不做任何字段包装。
// 校验: 必须非空 + 必须是合法 JSON (避免 typo 导致 C++ 端解析失败)。
func rawSendBody(spec CommandSpec, args map[string]string) (string, error) {
	data := args["data"]
	if data == "" {
		return "", fmt.Errorf("--data 不能为空")
	}
	if !json.Valid([]byte(data)) {
		return "", fmt.Errorf("--data 不是合法 JSON")
	}
	return data, nil
}

// === config-set-idevice 专用 BodyBuilder (Step 9.1, Step 10.A 增强) ===
//
// configSetIDeviceBody 构造 SetIDevice 请求的 JSON body。
//
// C++ 端 SetIDevice 分支读取 root["IDevice"] 子对象, 包含三个字段:
//   - Activate     bool  // 是否激活 iDevice
//   - InputLength  int   // 输入区段字节数 (范围 0-2048)
//   - OutputLength int   // 输出区段字节数 (范围 0-2048)
//
// 用户提供的 --data 必须是合法 JSON object (含上述三字段), 与 device-list 响应中的 IDevice 字段对称。
//
// Step 10.A 增强: 委托给通用 configBodyBuilder 复用其校验逻辑 (必填字段 + InputLength/OutputLength 范围),
// 同时复用其 SetIDevice 路由 (业务字段在顶层 IDevice, Function 仅含 Value)。
func configSetIDeviceBody(spec CommandSpec, args map[string]string) (string, error) {
	return configBodyBuilder(spec, args)
}
