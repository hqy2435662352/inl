// Package nrc — Config 写命令的 BodyBuilder 集合 (Step 10.A)。
//
// 设计依据: docs/inl-config-field-reference.md (C++ 源码 PNConfigLibFileDesign.cpp 反推)。
// 涵盖 11 条 config 写命令的请求体构造, 加上 1 个通用 configBodyBuilder 辅助。
//
// ⚠️ 核心概念: 大多数 config 命令操作的是"配置态" (networktopology.json),
// 即机器人对外部网络拓扑的"猜想"蓝图, 与实际物理设备无关。
// 要让配置态生效于实际设备, 需要:
//  1. config compile (将配置态编译激活到运行态)
//  2. device setup-name/ip (通过 DCP 协议直接修改实际设备参数)
//
// **例外**: ShieldDevice / UNShieldDevice 虽然是 config 命令组 (DataType=12),
// 但效果作用于**运行时态**——直接告诉机器人控制器忽略/监控某设备的通信错误,
// 不走 compile 链路, 响应特殊 (Function.Value 返回 bool 而非字符串)。
//
// 关键设计点 (来自 field-reference §0):
//   - 字段路径模式 A: 大多数命令的业务字段在 networktopology["Function"] 下
//   - 字段路径模式 B: SetPNDriver 的字段在 networktopology["PNDriver"] 顶层
//   - 1-based 索引: SetPNDeviceNum/SetModuleSlot/SetSubmoduleSlot 全部 1-based
//   - ShieldDevice 响应特殊: Function.Value 是 bool, 不用 Error[]/ErrorID[]
//   - 错误模型不统一: SetIDevice 用单值 Error: string, 其他用 Error: []string
//
// 调用方约定:
//   - 用户通过 --data 传入业务字段 JSON object
//   - BodyBuilder 负责:
//     1. 解析 + 校验 --data (JSON 合法性 + 必填字段 + 索引范围)
//     2. 按 spec.Function 决定字段放置位置 (Function 下 / PNDriver 顶层 / IDevice 顶层)
//     3. 默认自动 fetch 当前 topology (--no-fetch=true 可禁用, 见 fetchTopologyForConfig)
//     4. 拼装 JSON body
package nrc

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/your-org/inl/internal/topology"
)

// ====================== 预取 Topology (避免双连接) ======================

// preFetchedTopology 由主调方在建立连接后通过 PreFetchTopology 注入。
// configBodyBuilder 优先使用此缓存, 避免自行 fetch 时创建第二连接导致 C++ 端关原有连接。
//
// 设计背景 (2026-06-08 Step 10.B L2 实机验证):
//
//	runNrcCommand 先建连接 A, 然后调 configBodyBuilder → fetchTopologyForConfig
//	另建连接 B 发 CallBackJson。C++ 端处理完 CallBackJson 后关 socket, 导致连接 A 也被断。
//	回到 runNrcCommand 用连接 A 发 SetPNDriver 时已断, 报 "forcibly closed"。
//	修复: runNrcCommand 用连接 A 预取 topology 并注入缓存, configBodyBuilder 不再另建连接。
var preFetchedTopology map[string]any

// PreFetchTopology 用已有连接预取 topology 供后续 BodyBuilder 使用。
// 仅在连接成功后调用。失败时不报错 (configBodyBuilder 有 fallback)。
func PreFetchTopology(client *Client) {
	body := `{"DataType":12,"Function":{"Value":"CallBackJson"}}`
	_, respData, err := client.SendReceiveFiltered(0x9275, body, 12, 0x9271)
	if err != nil {
		return
	}
	var resp topology.CallbackJsonResponse
	if err := json.Unmarshal(respData, &resp); err != nil {
		return
	}
	preFetchedTopology = map[string]any{
		"PNDriver":        resp.PNDriver,
		"IDevice":         resp.IDevice,
		"DecentralDevice": resp.DecentralDevice,
	}
}

// ClearPreFetchedTopology 清除预取缓存 (应在每次命令执行完毕后调用)。
func ClearPreFetchedTopology() {
	preFetchedTopology = nil
}

// ====================== 通用辅助: configBodyBuilder ======================

// configBodyBuilder 通用 config 写命令 BodyBuilder。
//
// 适用于 10 条 config 写命令 (除 compile / set-idevice 外), 差异由 spec.Function 决定:
//   - 模式 A (Function 下): 业务字段直接平铺到 Function.<key>
//   - 模式 B (顶层 PNDriver): 业务字段放到顶层 PNDriver 对象 (仅 SetPNDriver)
//
// **注意**: 所有 config 命令都只修改"配置态" (networktopology.json, 机器人对网络的猜想蓝图),
// 不直接操作实际设备。要让变更生效于实际设备, 需 config compile + DCP 推送。
//
// 流程:
//  1. 解析 --data 为业务字段 (e.g. {"RefGSD":"...","DAP_ID":"..."})
//  2. 校验必填字段 + 范围 (按 spec.Function 决定规则)
//  3. 字段放置: 模式 A → Function.<key>; 模式 B → 顶层 PNDriver
//  4. 默认自动 fetch 当前 topology 并灌入 DecentralDevice/PNDriver/IDevice
//     (--no-fetch=true 可禁用, fetch 失败不阻塞, 见 field-reference §0.2)
//  5. 拼装最终 JSON body
//
// 注意: 此函数供 Step 10.A 之前的几个 9.x/10.x 调试场景使用,
// 完整的字段校验逻辑在每个具体 BodyBuilder (configSetDriverBody 等) 中实现。
// 通用辅助主要处理 JSON 解析 + fetch + 通用布局, 让每个具体函数只关心差异。
func configBodyBuilder(spec CommandSpec, args map[string]string) (string, error) {
	// 步骤 1: 解析 --data
	data := args["data"]
	if data == "" {
		return "", fmt.Errorf("--data 不能为空")
	}
	if !json.Valid([]byte(data)) {
		return "", fmt.Errorf("--data 不是合法 JSON")
	}
	var business map[string]any
	if err := json.Unmarshal([]byte(data), &business); err != nil {
		return "", fmt.Errorf("--data 解析失败: %w", err)
	}

	// 步骤 2: 通用必填字段校验 (各具体函数可在此基础上叠加规则)
	if err := validateRequiredFields(spec.Function, business); err != nil {
		return "", err
	}

	// 步骤 3: 拼装请求体
	body := map[string]any{"DataType": spec.DataType}

	switch spec.Function {
	case "SetPNDriver":
		// 模式 B: 业务字段在顶层 PNDriver
		// C++ SetPNDriver L393-395 直接读 networktopology["PNDriver"][...]
		body["PNDriver"] = business
		body["Function"] = map[string]any{"Value": spec.Function}
	case "SetIDevice":
		// 模式 C: 业务字段在顶层 IDevice (v2 修正, 非模式 A)
		// C++ SetIDevice L3202, 3205, 3210-3211 读 networktopology["IDevice"][...],
		// 不读 Function.* 业务字段, Function 仅需 Value (用于分派)
		body["IDevice"] = business
		body["Function"] = map[string]any{"Value": spec.Function}
	default:
		// 模式 A: 业务字段平铺到 Function 下
		// 适用 9 条: ShildDevice/UNShildDevice/AddPNDevice/AddModule/AddSubmodule/
		// UninstallPNDevice/UninstallModule/UninstallSubmodule/SetPNDevice
		fnObj := map[string]any{"Value": spec.Function}
		for k, v := range business {
			fnObj[k] = v
		}
		body["Function"] = fnObj
	}

	// 步骤 4: 默认自动 fetch 当前 topology (--no-fetch=true 跳过, SetIDevice 不需 topology)
	//
	// ⚠️ 2026-06-08 fix: 步骤 3 已按模式 B/C 设置 body["PNDriver"] 或 body["IDevice"]
	// 为用户 --data 值。此处仅补缺 (DecentralDevice 上下文 + 模式 A 的 PNDriver/IDevice),
	// 不能覆盖用户已指定的字段, 否则 SetPNDriver 设 IP 192.168.2.200 会被 fetch
	// 回的 192.168.2.14 覆盖——用户实际请求从未生效。
	if args["no-fetch"] != "true" && spec.Function != "SetIDevice" {
		topology := preFetchedTopology // 优先用主调方预取缓存 (复用连接, 避免双连接)
		if topology == nil {
			// 回退: 自行 fetch (测试 / 兼容旧路径)
			if t, ferr := fetchTopologyForConfig(args["target"]); ferr == nil && t != nil {
				topology = t
			}
		}
		if topology != nil {
			for _, k := range []string{"DecentralDevice", "PNDriver", "IDevice"} {
				// 仅补缺: 不覆盖步骤 3 已设的用户 --data 字段
				if _, already := body[k]; already {
					continue
				}
				if v, ok := topology[k]; ok {
					// ⚠️ 2026-06-05 Step 10.B 关键修复 (P0 inl bug, L2 实机发现):
					//
					// C++ 端 AddPNDevice (PNConfigLibFileDesign.cpp:1225) 直接对请求体的
					// `networktopology["DecentralDevice"].append(newdevicearry)` 追加新设备。
					// jsoncpp 对 nullValue 的 .append() 是**静默 no-op** — 设备不添加,
					// 响应中 DecentralDevice 仍为 null, 5-8s 后 C++ 关连接, inl 端
					// 旧版"closed"启发式误判为成功, 静态配置**未修改**。
					//
					// 修复: 当 L1 baseline DecentralDevice 是 nil/缺失 (空配置场景) 时,
					// 用空数组 [] 替代 null, 让 C++ 能正确 append 新设备。
					// 其他键 (PNDriver/IDevice) 是对象, 不存在此问题。
					//
					// ⚠️ 2026-06-08 fix: 原 v == nil 不生效, 因为 nil []DecentralDevice
					// 存入 map[string]any 后, interface{} 记住了具体类型, v == nil 恒假。
					// 改用 reflect 判断底层 slice 是否 nil。
					if k == "DecentralDevice" && isNilSlice(v) {
						body[k] = []any{}
					} else {
						body[k] = v
					}
				}
			}
		}
		// fetch 失败不阻塞 — nrc2.out 可能已预加载, inl 也可能已有最新 topology
	}

	// 步骤 5: 序列化
	out, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("序列化请求体失败: %w", err)
	}
	return string(out), nil
}

// isNilSlice 判断 any 值是否为 nil slice (Go interface nil 陷阱专用)。
//
// nil slice 存入 interface{} 后，interface{} 记住了具体类型 (e.g. []DecentralDevice)，
// 此时 v == nil 恒假。本函数用 reflect 穿透 interface 判断底层 slice 是否为 nil。
func isNilSlice(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	return rv.Kind() == reflect.Slice && rv.IsNil()
}

// fetchTopologyForConfig 拉取当前 PROFINET 网络配置 (device-list)。
//
// 真实实现 (Step 10.B v2 假设 B 实机验证后, 2026-06-04 实施)：
//   - 通过 nrc.Client 连接到 target:6000
//   - 发送 device-list 命令 (Function.Value="CallBackJson")
//   - 解析响应 JSON 为 topology.CallbackJsonResponse
//   - 提取 PNDriver / IDevice / DecentralDevice 三个字段并转为 map[string]any
//   - 供 configBodyBuilder 灌入请求体（解决假设 B: nrc2.out 不预加载配置）
//
// 行为契约:
//   - target 为空 → 返回 error（"target 不能为空"）
//   - target 不含端口 → 自动追加 :6000（nrc2.out 默认端口）
//   - 连接失败 / 发送失败 / 响应解析失败 → 返回 error（让 configBodyBuilder 决定）
//     当前 configBodyBuilder 选择"fetch 失败不阻塞", 因此错误会被吞掉,
//     请求体照常发出（与 Step 10.A 设计一致）
//   - 必须 defer client.Close() 防止 fd 泄漏
//
// 测试/扩展: 单测可调用 SetFetchTopologyForConfig 注入 mock,
// 例如 config_body_test.go 中 "AutoFetch_FetchSuccess" 测试。
//
// 详见:
//   - [inl-config-field-reference.md §0.2 上下文依赖] 与 [§0.5 响应模型]
//   - [inl-step10-config-write-validation-plan.md §2.0 假设 A/B v2 实机确认]
var fetchTopologyForConfig = func(target string) (map[string]any, error) {
	if target == "" {
		return nil, fmt.Errorf("fetchTopologyForConfig: target 不能为空")
	}
	addr := strings.TrimSpace(target)
	if !strings.Contains(addr, ":") {
		addr = addr + ":6000"
	}

	client := NewClient(addr)
	if err := client.Connect(); err != nil {
		return nil, fmt.Errorf("fetchTopologyForConfig: connect %s: %w", addr, err)
	}
	defer client.Close()

	// 构造 device-list 请求: DataType=12 + Function.Value="CallBackJson"
	// 与 device-list 命令的 DefaultBodyBuilder 输出字面一致。
	body := `{"DataType":12,"Function":{"Value":"CallBackJson"}}`
	_, respData, err := client.SendReceiveFiltered(0x9275, body, 12, 0x9271)
	if err != nil {
		return nil, fmt.Errorf("fetchTopologyForConfig: send/receive: %w", err)
	}

	// 解析响应为 topology.CallbackJsonResponse（v2 已建模 DecentralDevice 字段）
	var resp topology.CallbackJsonResponse
	if err := json.Unmarshal(respData, &resp); err != nil {
		return nil, fmt.Errorf("fetchTopologyForConfig: unmarshal: %w", err)
	}

	// 转为 map[string]any (configBodyBuilder 期望的格式)
	// - PNDriver / IDevice 即使为空结构体也保留 key（configBodyBuilder 内部 ok 检查会通过）
	// - DecentralDevice 可能是 nil（空配置场景），保留 key 让调用方按需判断
	result := map[string]any{
		"PNDriver":        resp.PNDriver,
		"IDevice":         resp.IDevice,
		"DecentralDevice": resp.DecentralDevice,
	}
	return result, nil
}

// SetFetchTopologyForConfig 注入 fetch 真实实现 (供 main.go / 未来联调使用)。
//
// 测试用: 在 *_test.go 中替换为 mock 实现, 验证 auto-fetch 逻辑。
// 生产用: 在 main.go init() 中替换为真实 NRC 客户端调用。
func SetFetchTopologyForConfig(f func(target string) (map[string]any, error)) {
	fetchTopologyForConfig = f
}

// validateRequiredFields 按 spec.Function 校验必填字段 + 范围约束。
//
// 返回 nil 表示校验通过, 否则返回描述性错误。
// 各命令的必填字段定义见 [inl-config-field-reference.md §1]。
func validateRequiredFields(functionName string, business map[string]any) error {
	required, ok := configRequiredFields[functionName]
	if !ok {
		// 未在表中的 Function (如 Compile) 视为无必填要求。
		return nil
	}
	for _, key := range required {
		if _, present := business[key]; !present {
			return fmt.Errorf("缺少必填字段 %q (--data 中需包含该字段)", key)
		}
	}

	// 1-based 索引校验 (SetPNDeviceNum/SetModuleSlot/SetSubmoduleSlot 必须 >= 1)
	for _, key := range []string{"SetPNDeviceNum", "SetModuleSlot", "SetSubmoduleSlot"} {
		if v, present := business[key]; present {
			n, ok := toInt(v)
			if !ok {
				return fmt.Errorf("字段 %q 必须是整数, got %T (%v)", key, v, v)
			}
			if n < 1 {
				return fmt.Errorf("字段 %q 必须是 1-based 索引 (>= 1), got %d", key, n)
			}
		}
	}

	// IDevice 长度范围校验
	if functionName == "SetIDevice" {
		for _, key := range []string{"InputLength", "OutputLength"} {
			if v, present := business[key]; present {
				n, ok := toInt(v)
				if !ok {
					return fmt.Errorf("字段 %q 必须是整数, got %T (%v)", key, v, v)
				}
				if n < 0 || n > 2048 {
					return fmt.Errorf("字段 %q 必须在 [0, 2048] 范围内, got %d", key, n)
				}
			}
		}
	}

	return nil
}

// configRequiredFields 是 11 条 config 写命令的必填字段白名单。
//
// 来源: inl-config-field-reference.md §1 (基于 PNConfigLibFileDesign.cpp 反推)。
// 不含 config-compile (无 --data, 走 DefaultBodyBuilder)。
//
// 2026-06-04 v3: ShieldDevice/UNShieldDevice 常量拼写已纠正回 "ShieldDevice"/"UNShieldDevice"
// (typo 修复提交 3d3cc3c7, 之前 v2 误用 "ShildDevice"/"UNShildDevice")。
// 来源: io-controller/src/ioc/profinet_constants.h:82-83
var configRequiredFields = map[string][]string{
	"SetPNDriver":        {"DeviceName", "IPAddress", "SubnetMask", "SetInTheProject"},
	"AddPNDevice":        {"RefGSD", "DAP_ID"},
	"UninstallPNDevice":  {"SetPNDeviceNum"},
	"SetPNDevice":        {"SetPNDeviceNum"},
	"AddModule":          {"SetPNDeviceNum", "ModuleID"},
	"UninstallModule":    {"SetPNDeviceNum", "SetModuleSlot"},
	"AddSubmodule":       {"SetPNDeviceNum", "ModuleID", "SubmoduleID"},
	"UninstallSubmodule": {"SetPNDeviceNum", "SetModuleSlot", "SetSubmoduleSlot"},
	"ShieldDevice":       {"DeviceName"},
	"UNShieldDevice":     {"DeviceName"},
	"SetIDevice":         {"Activate", "InputLength", "OutputLength"},
}

// toInt 将 json.Unmarshal 后的 any 值转为 int (容忍 float64 / int / int64 / json.Number)。
func toInt(v any) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, true
	case int32:
		return int(x), true
	case int64:
		return int(x), true
	case float64:
		return int(x), true
	case float32:
		return int(x), true
	case json.Number:
		n, err := x.Int64()
		if err == nil {
			return int(n), true
		}
	}
	return 0, false
}

// ====================== 10 个具体 BodyBuilder ======================
//
// 模式分类:
//   - 模式 B (顶层 PNDriver): configSetDriverBody
//   - 模式 A (Function 下):   其余 9 条
//
// 所有 BodyBuilder 都复用 configBodyBuilder 通用逻辑, 仅在以下方面差异:
//   - spec.Function 路由
//   - 必填字段 + 范围校验 (由 validateRequiredFields 统一处理)
//   - ShieldDevice 响应特殊: 仍走模式 A 输出, 响应解析由 main.go 分支处理 (见 ShieldDeviceResponse)

// configSetDriverBody — SetPNDriver (模式 B, 业务字段在顶层 PNDriver)。
//
// C++ 端 SetPNDriver 读取 root["PNDriver"].{DeviceName,IPAddress,SubnetMask,SetInTheProject}。
// 与其他 10 条 config 命令不同, 字段不在 Function 下。
func configSetDriverBody(spec CommandSpec, args map[string]string) (string, error) {
	return configBodyBuilder(spec, args)
}

// configAddDeviceBody — AddPNDevice (模式 A, 业务字段在 Function 下)。
//
// C++ 端 AddPNDevice 读取 root["Function"].{RefGSD, DAP_ID}。
// 其他字段 (DeviceName/IPAddress/SubnetMask/ReductionRatio/DAP 模块布局) C++ 自动生成。
func configAddDeviceBody(spec CommandSpec, args map[string]string) (string, error) {
	return configBodyBuilder(spec, args)
}

// configRemoveDeviceBody — UninstallPNDevice (模式 A, 1-based 索引)。
//
// C++ 端 UninstallPNDevice (L2706) 使用 SetPNDeviceNum - 1 作为 0-based 索引, 过滤 DecentralDevice[]。
func configRemoveDeviceBody(spec CommandSpec, args map[string]string) (string, error) {
	return configBodyBuilder(spec, args)
}

// configSetDeviceBody — SetPNDevice (模式 A, 1-based 索引, 修改配置态网络拓扑参数)。
//
// C++ 端 SetPNDevice (L1561) 将用户传入的业务字段 (DeviceName/IPAddress/SubnetMask 等)
// 写入 networktopology.json 中 DecentralDevice[SetPNDeviceNum-1] 对应的配置项。
// 注意: config 命令只修改"配置态" (networktopology.json, 即机器人对网络的猜想蓝图),
// 不会通过 DCP 协议修改实际设备的名称/IP。要让实际设备生效, 需要:
//  1. config compile (将配置态编译激活到运行态)
//  2. device setup-name/ip (通过 DCP 直接修改实际设备参数)
func configSetDeviceBody(spec CommandSpec, args map[string]string) (string, error) {
	return configBodyBuilder(spec, args)
}

// configAddModuleBody — AddModule (模式 A, SetPNDeviceNum + ModuleID)。
//
// C++ 端 AddModule 从 GSD 数据库查模块的 FixedInSlots/UsedInSlots/AllowedInSlots, 自动选 slot。
func configAddModuleBody(spec CommandSpec, args map[string]string) (string, error) {
	return configBodyBuilder(spec, args)
}

// configRemoveModuleBody — UninstallModule (模式 A, SetPNDeviceNum + SetModuleSlot)。
//
// C++ 端 UninstallModule (L2769) 从 DecentralDevice[].Module[] 中过滤掉 Slot=SetModuleSlot 的模块。
// 校验 FixedInSlots 不可删 (返回 PNCONFIGLIB_SUBMODULE_FIXED_CANT_DEL)。
func configRemoveModuleBody(spec CommandSpec, args map[string]string) (string, error) {
	return configBodyBuilder(spec, args)
}

// configAddSubmoduleBody — AddSubmodule (模式 A, SetPNDeviceNum + ModuleID + SubmoduleID)。
//
// C++ 端 AddSubmodule 从 GSD 数据库查子模块的 Subslot/IOData。
func configAddSubmoduleBody(spec CommandSpec, args map[string]string) (string, error) {
	return configBodyBuilder(spec, args)
}

// configRemoveSubmoduleBody — UninstallSubmodule (模式 A, 3 个 1-based 索引)。
func configRemoveSubmoduleBody(spec CommandSpec, args map[string]string) (string, error) {
	return configBodyBuilder(spec, args)
}

// configShieldBody — ShieldDevice (模式 A, 响应 Function.Value 为 bool)。
//
// C++ 端 ShieldDeviceByName (L312) 调用 device_bind::ShieldPROFINETDeviceByName。
// 响应特殊: Function.Value 字段被复用为 bool (true=成功, false=失败),
// 没有 Error[]/ErrorID[]。响应解析由 internal/configresp.ShieldDeviceResponse 处理。
func configShieldBody(spec CommandSpec, args map[string]string) (string, error) {
	return configBodyBuilder(spec, args)
}

// configUnshieldBody — UNShieldDevice (模式 A, 响应 Function.Value 为 bool)。
//
// 与 ShieldDevice 响应模式一致。
func configUnshieldBody(spec CommandSpec, args map[string]string) (string, error) {
	return configBodyBuilder(spec, args)
}
