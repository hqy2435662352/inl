package gsd

import "github.com/your-org/inl/internal/dcpdevice"

// MatchResponse 是 DataType=16 响应的结构 (DCP 发现 + GSD 匹配结果)。
//
// C++ 端语义: 通过 DCP 协议在指定网卡上广播 identify 请求, 收集所有 PROFINET 设备的响应,
// 并与本地 GSD 库 (./communication/Profinet/*.xml) 中的 VendorID+DeviceID 比对,
// 只输出在 GSD 库中能匹配到的设备 (FilterGSDCompatibleDevices)。
// 对应命令: `inl gsd match --interface <port>`。
//
// 结构与 topology.ScanResponse 相同 (DataType + Devices []DCPDevice), 但只包含匹配项。
// 若 nrc2.out 版本不支持 DataType=16, 该命令会回退到 `topology scan` + AI 客户端按 VendorID/DeviceID 手动匹配。
type MatchResponse struct {
	DataType int                   `json:"DataType"`
	Devices  []dcpdevice.DCPDevice `json:"Devices"`
}

// Response 是 DataType=13（GSD文件列表回调）的顶层响应结构。
// Command=0x9271 的 Data 字段 JSON 反序列化目标。
type Response struct {
	// DataType 回调类型标识，固定为 13（GSD 文件列表）。
	DataType int `json:"DataType"`
	// Device 工业 PC 的 ./communication/Profinet/ 目录下所有 GSDML 文件解析出的设备列表。
	// 若目录下无 GSDML 文件，则为空数组（非 null）。
	Device []Device `json:"Device"`
}

// Device 表示一个 GSDML 文件解析后的设备信息。
// 每个 Device 对应一个 .xml 文件，包含设备元数据、DAP 变体、可用模块和子模块。
type Device struct {
	// VendorID PROFINET 厂商 ID，十六进制字符串，如 "0x002A"（Siemens）。
	VendorID string `json:"VendorID"`
	// VendorName 厂商名称，人类可读，如 "SIEMENS"、"OBARA"。
	VendorName string `json:"VendorName"`
	// DeviceID PROFINET 设备型号 ID，十六进制字符串，如 "0x0030"。
	DeviceID string `json:"DeviceID"`
	// GSDName GSDML 文件名，如 "GSDML-V2.31-OBARA-SIV31-40-20190707.xml"。
	GSDName string `json:"GSDName"`
	// MainFamily 设备大类。
	// 已知值: "General"（通用）, "I/O"（IO 模块）, "Sensors"（传感器）, "PLCs"（PLC）, "Valves"（阀岛）。
	MainFamily string `json:"MainFamily"`
	// ProductFamily 设备产品族描述，人类可读，如 "Welding Controller"、"CPU SR40"。
	ProductFamily string `json:"ProductFamily"`
	// DAP Device Access Point 列表，同一设备的不同硬件变体（如铜口/光口）。
	// 大多数设备只有 1 个 DAP，OBARA SIV31 有 2 个（DAP 和 DAP-FO）。
	DAP []DAP `json:"DAP"`
	// Module 设备支持的模块列表。
	// 普通设备在此列出所有可插入模块（如 HMS 的 ADI#1/ADI#2）。
	// PLC 类设备（Siemens CPU SR40）为空数组，其 IO 数据在 DAP.VirtualSubmoduleList 中。
	Module []Module `json:"Module"`
	// Submodules 设备级共享子模块列表。
	// 绝大多数设备为空数组。仅 SMC EX245 阀岛非空：包含 Shared 子模块的"输出镜像"——将输出数据复制回输入区以便 PLC 读取确认。
	// 此处的 IOData 无 Length 字段（输出镜像不需要长度信息）。
	Submodules []Submodule `json:"Submodules"`
}

// DAP Device Access Point，设备的一个硬件接口变体的描述。
// 一个设备可以有多个 DAP（如铜口版和光口版），配置时选择其中一个。
type DAP struct {
	// DAP_ID DAP 标识符，如 "DAP"、"DAP-FO"、"DAP 1"。
	DAPID string `json:"DAP_ID"`
	// DAP_Name DAP 人类可读名称，如 "SIV31 Std(2-Port)"、"CPU SR40"。
	DAPName string `json:"DAP_Name"`
	// DNS_CompatibleName PROFINET DCP 协议中的设备名，如 "plc200smart"、"ABCC40-PIR"。
	DNSCompatibleName string `json:"DNS_CompatibleName"`
	// FixedInSlots DAP 自身占用的槽位号。
	// "0" = 标准 DAP（大多数设备）即DAP本身不存在有信号定义的虚拟子模块；"1" = PLC 类设备（Siemens CPU SR40 自身占 Slot 1）。
	FixedInSlots string `json:"FixedInSlots"`
	// ModuleIdentNumber PROFINET 模块标识号，十六进制字符串，如 "0x80010000"。
	ModuleIdentNumber string `json:"ModuleIdentNumber"`
	// ReductionRatio 支持的 PROFINET 发送时钟缩减比列表。
	// 整数数组，常见值 [1,2,4,8,16,32,64,128,256,512]，部分传感器类设备从 16 起步。
	ReductionRatio []int `json:"ReductionRatio"`
	// UseableModules 此 DAP 支持的可插入模块列表，描述每个槽位可以插入什么模块。
	// PLC 类设备（Siemens）为空数组（不支持外扩模块）。
	// 此字段有两种互斥的子结构模式，见 DAPUseableModule。
	UseableModules []DAPUseableModule `json:"UseableModules"`
	// VirtualSubmoduleList 仅部分特殊设备 如PLC 类设备（Siemens CPU SR40）非空。
	// 普通设备的 IO 数据在 Module.VirtualSubmoduleList 内，PLC 的直接在 DAP 层。
	// 此处的 IOData 无 Length 字段。
	VirtualSubmoduleList []Submodule `json:"VirtualSubmoduleList,omitempty"`
}

// DAPUseableModule DAP 的 UseableModules 数组元素，描述一个槽位上可插入的模块。
//
// 此结构有两种互斥的表示模式，由 GSDML 源文件中 ModuleList 的写法决定：
//
// 模式 A — 固定单槽（FixedInSlots）: HMS, iutek, 部分 OBARA
//
//	JSON 含 FixedInSlots + ModuleIDTarget，不含 AllowedInSlots*。
//	表示该模块只能插入 FixedInSlots 指定的唯一槽位。
//
// 模式 B — 允许槽位范围（AllowedInSlots）: Proteus, TMGTE, SMC, 部分 OBARA
//
//	JSON 含 AllowedInSlots + AllowedInSlotsStart/EndNumber + ModuleIDTarget，
//	可选含 UsedInSlots。AllowedInSlots 是一个人类可读的范围字符串（如 "1..2"），
//	Start/EndNumber 是数值形式。UsedInSlots 为建议的默认槽位，TMGTE 设备无此字段。
type DAPUseableModule struct {
	// ModuleIDTarget 目标模块的 ModuleID 引用，指向 Device.Module 中某个 Module.ModuleID。
	ModuleIDTarget string `json:"ModuleIDTarget"`
	// --- 模式 A 字段 ---
	// FixedInSlots 模块固定插入的槽位号，如 "0"、"1"、"2"。
	// 仅模式 A 出现。
	FixedInSlots string `json:"FixedInSlots,omitempty"`
	// --- 模式 B 字段 ---
	// AllowedInSlots 模块允许插入的槽位范围，人类可读字符串，如 "1..2"、"3..10"、"1..64"。
	// 仅模式 B 出现。
	AllowedInSlots string `json:"AllowedInSlots,omitempty"`
	// AllowedInSlotsStartNumber 槽位范围起始值（数值字符串），如 "3"。
	AllowedInSlotsStartNumber string `json:"AllowedInSlotsStartNumber,omitempty"`
	// AllowedInSlotsEndNumber 槽位范围结束值（数值字符串），如 "10"。
	AllowedInSlotsEndNumber string `json:"AllowedInSlotsEndNumber,omitempty"`
	// UsedInSlots 建议的默认槽位，如 "2"。TMGTE 设备无此字段（可选）。
	UsedInSlots string `json:"UsedInSlots,omitempty"`
}

// Module 设备的一个可用模块，描述可插入到 DAP 槽位中的 IO 模块。
// 每个模块包含一个或多个虚拟子模块，每个子模块包含 IO 数据点列表。
type Module struct {
	// ModuleID 模块唯一标识，被 DAPUseableModule.ModuleIDTarget 引用。
	// 如 "ID_MODULE_ADI1"、"OUT_MODULE"、"ID_MOD_VALVE32_OUTPUT"。
	ModuleID string `json:"ModuleID"`
	// ModuleName 模块人类可读名称，如 "64 Digital Input"、"32 valves"。
	ModuleName string `json:"ModuleName"`
	// UseableSubmodules 模块下可用的子模块槽位分配列表。
	// 大部分设备为 null（空 JSON 数组，Go 侧为 nil slice），表示无子模块可选。
	// 仅 SMC EX245 阀岛的 Shared 模块非空，描述子模块在子槽位中的分配。
	UseableSubmodules []UseableSubmodule `json:"UseableSubmodules"`
	// VirtualSubmoduleList 模块内的虚拟子模块列表。
	// 每个子模块包含实际的 IO 数据点定义。此处的 IOData 含 Length 字段。
	VirtualSubmoduleList []Submodule `json:"VirtualSubmoduleList"`
}

// UseableSubmodule 子模块槽位分配，描述一个子模块可以插入到哪些子槽位。
// 仅当 Module.UseableSubmodules 非空时出现（目前仅 SMC EX245）。
type UseableSubmodule struct {
	// AllowedInSubslotsStartNumber 允许插入的起始子槽位号，如 "2"。
	AllowedInSubslotsStartNumber string `json:"AllowedInSubslotsStartNumber"`
	// AllowedInSubslotsEndNumber 允许插入的结束子槽位号，如 "4"。
	AllowedInSubslotsEndNumber string `json:"AllowedInSubslotsEndNumber"`
	// SubmoduleItemTarget 目标子模块的 SubmoduleID 引用，如 "ID_SUBMOD_VALVE32_OUTPUT_SHARED"。
	SubmoduleItemTarget string `json:"SubmoduleItemTarget"`
	// UsedInSubslots 默认使用的子槽位号，如 "2"。
	UsedInSubslots string `json:"UsedInSubslots"`
}

// Submodule 虚拟子模块，是 IO 数据点的容器。
// 出现于三个位置，含义不同：
//   - Module.VirtualSubmoduleList    → 模块内的 IO 数据（含 Length）
//   - DAP.VirtualSubmoduleList       → PLC 类设备的 DAP 自带 IO 数据（无 Length）
//   - Device.Submodules              → Shared 子模块的输出镜像（无 Length）
type Submodule struct {
	// SubmoduleID 子模块唯一标识，如 "ID_SUBMOD_ADI1_GROUP1"、"VSM_2_1000"。
	SubmoduleID string `json:"SubmoduleID"`
	// SubmoduleName 子模块人类可读名称，如 "64 Digital Input"、"传送区01"。
	SubmoduleName string `json:"SubmoduleName"`
	// IOData IO 数据点列表，每个元素描述一个 PROFINET 信号。
	IOData []IOData `json:"IOData"`
}

// IOData 单个 PROFINET IO 数据点/信号的定义。
//
// Length 字段的存在性遵循"三态"规则：
//   - Module.VirtualSubmoduleList 内 → Length 必存在
//   - DAP.VirtualSubmoduleList 内    → Length 不存在（Go 零值 = 0）
//   - Device.Submodules 内           → Length 不存在（Go 零值 = 0）
type IOData struct {
	// DataName 信号名称，如 "DI Status1"、"In 0..7"、"传送区01"。
	// 部分设备同一模块内多个信号同名（如 SMC EX245 的多个 "Output 1 byte"），
	// 此时需依赖数组索引区分。
	DataName string `json:"DataName"`
	// DataType PROFINET 数据类型。
	// 已知值: "Unsigned8", "Unsigned16", "Unsigned32", "Integer8", "OctetString"。
	DataType string `json:"DataType"`
	// InOrOut 信号方向。
	// "Input"  = 从设备到控制器的数据（PLC 读取传感器状态）
	// "Output" = 从控制器到设备的数据（PLC 写入执行器指令）
	InOrOut string `json:"InOrOut"`
	// Length 数据长度（字节）。
	// 仅 Module.VirtualSubmoduleList 内的 IOData 有此字段。
	// DAP 层和 Device.Submodules 层的 IOData 无此字段（JSON 中不存在，Go 零值为 0）。
	Length int `json:"Length,omitempty"`
	// UseAsBits 是否按位解析（位视图）。
	// true  = 每个 bit 为一个独立信号（如 "In 0..7" 表示 8 个数字输入位）
	// false = 整个字节/字作为一个数值信号（如 "Inlet Flow Rate" 作为 16 位无符号整数）
	UseAsBits bool `json:"UseAsBits"`
}
