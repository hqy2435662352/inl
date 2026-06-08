package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/your-org/inl/internal/nrc"
	"github.com/your-org/inl/internal/output"
)

var (
	targetFlag string
	formatFlag string
	outputFlag string
	retryFlag  int
)

// isDCPWriteClosedConnection 判断错误是否为写操作的已知 "控制器发完即关" 行为。
//
// 两种场景:
//  1. DCP 写 (DataType=14 + Risk=Write): 帧发完控制器主动关连接, 不响应 JSON
//  2. Compile (DataType=12 + Function="Compile"): 编译重启 PROFINET 协议栈, 必然关连接
//
// 其它 DataType=12 config 写 (SetPNDriver/AddPNDevice/...) 的 "closed" 意味着
// 请求未到达 / 连接异常, **不**应误判为成功。
func isDCPWriteClosedConnection(spec nrc.CommandSpec, err error) bool {
	if err == nil {
		return false
	}
	if spec.DataType == 14 && spec.Risk == nrc.RiskWrite && strings.Contains(err.Error(), "closed") {
		return true
	}
	if spec.DataType == 12 && spec.Function == "Compile" && strings.Contains(err.Error(), "closed") {
		return true
	}
	return false
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "inl",
		Short: "工业 PC NRC Socket 协议 CLI 工具",
		Long: `inl — Industrial Netline CLI

通过 TCP:6000 与运行 nrc2.out 的工业 PC 通信, 收发 NRC 帧,
实现 PROFINET GSD 设备列表、设备读取、配置写等功能。`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	rootCmd.PersistentFlags().StringVar(&targetFlag, "target", "",
		"工业 PC IP 地址 (必填)")
	rootCmd.PersistentFlags().StringVar(&formatFlag, "format", "json",
		"输出格式: json (默认) | table | csv | ndjson")
	rootCmd.PersistentFlags().StringVar(&outputFlag, "output", "",
		"原始响应保存路径 (默认: ./<name>_response_<时间戳>.json)")
	rootCmd.PersistentFlags().IntVar(&retryFlag, "retry", 0,
		"DCP 命令重试次数 (0=默认 1 次; 网络抖动场景可设 3-5)")

	for _, g := range []nrc.CommandGroup{
		nrc.GroupInterface, nrc.GroupGsd, nrc.GroupDevice, nrc.GroupConfig, nrc.GroupTopology, nrc.GroupSchema, nrc.GroupRaw,
	} {
		rootCmd.AddCommand(buildGroupCmd(g))
	}

	installRiskHelpFunc(rootCmd)

	if err := rootCmd.Execute(); err != nil {
		if oerr, ok := err.(*output.Error); ok {
			output.WriteError(os.Stderr, oerr)
		} else {
			fmt.Fprintln(os.Stderr, err.Error())
		}
		os.Exit(1)
	}
}

func buildGroupCmd(group nrc.CommandGroup) *cobra.Command {
	var use, short, long string
	var pureGroup bool

	switch group {
	case nrc.GroupGsd:
		use = "gsd"
		short = "GSD 设备驱动管理"
		long = "GSD 设备驱动相关命令 (只读)。"
		pureGroup = true
	case nrc.GroupDevice:
		use = "device"
		short = "PROFINET 设备信息"
		long = "PROFINET 设备信息读取 (只读, 无副作用)。"
		pureGroup = true
	case nrc.GroupConfig:
		use = "config"
		short = "PROFINET 配置写操作"
		long = "PROFINET 配置写操作 (有副作用, 写/高危写需 --yes 确认)。"
		pureGroup = false
	case nrc.GroupInterface:
		use = "interface"
		short = "工业 PC 网络端口"
		long = "工业 PC 网络端口管理 (只读)。"
		pureGroup = true
	case nrc.GroupTopology:
		use = "topology"
		short = "PROFINET 拓扑管理"
		long = "PROFINET 拓扑扫描与管理 (只读)。"
		pureGroup = true
	case nrc.GroupSchema:
		use = "schema"
		short = "AI Agent 能力发现"
		long = "列出所有可用命令的元数据 (纯客户端, 不连接工业 PC)。"
		pureGroup = true
	case nrc.GroupRaw:
		use = "raw"
		short = "透传原始 JSON 帧"
		long = "直接发送任意 JSON payload 到工业 PC (兜底, 覆盖非标准命令)。"
		pureGroup = false
	}

	annotations := map[string]string{}
	if pureGroup {
		annotations[nrc.AnnotationPureGroup] = "true"
	}

	groupCmd := &cobra.Command{
		Use:         use,
		Short:       short,
		Long:        long,
		Annotations: annotations,
	}

	for _, spec := range nrc.Registry {
		if spec.Group != group {
			continue
		}
		groupCmd.AddCommand(buildSubCmd(spec))
	}

	// 注入特殊组合命令 (Step 9.4 — 不走 Registry, 走特殊路径)
	if group == nrc.GroupDevice {
		groupCmd.AddCommand(buildDeviceSetupCmd())
	}

	return groupCmd
}

func buildSubCmd(spec nrc.CommandSpec) *cobra.Command {
	use := spec.Name[len(spec.Group)+1:]

	subCmd := &cobra.Command{
		Use:   use,
		Short: spec.Description,
		Annotations: map[string]string{
			nrc.AnnotationRisk:     string(spec.Risk),
			nrc.AnnotationDataType: strconv.Itoa(spec.DataType),
			nrc.AnnotationFunction: spec.Function,
		},
		Args: cobra.NoArgs,
		RunE: runNrcCommand(spec.Name),
	}

	if spec.Risk != nrc.RiskRead {
		subCmd.Flags().Bool("yes", false,
			"确认执行 "+string(spec.Risk)+" 操作 (必需)")
		subCmd.Flags().Bool("dry-run", false,
			"预览请求帧 (不连接工业 PC, 不发送任何数据)")
	}

	// Step 10.A: config 写命令支持 --no-fetch 标志, 关闭 BodyBuilder 的自动 fetch。
	// 仅对 Function 字段需要上下文依赖 (Function 在 DecentralDevice/PNDriver/IDevice 上操作)
	// 的命令有意义, 即 set-driver/add-device/remove-device/set-device/add-module/
	// remove-module/add-submodule/remove-submodule/shield/unshield (10 条)。
	// 默认开启 fetch (与 field-reference §0.2 + step10 plan §2.3 一致)。
	if spec.Group == nrc.GroupConfig && spec.Risk != nrc.RiskRead {
		hasData := false
		for _, a := range spec.Args {
			if a.Name == "data" {
				hasData = true
				break
			}
		}
		if hasData {
			subCmd.Flags().Bool("no-fetch", false,
				"禁用 BodyBuilder 自动 fetch 当前 topology (默认: 启用, 需 --target)")
		}
	}

	// DCP 命令的参数注册: 根据 spec.Args 注册对应 String flag 并标记必填。
	// 参数全集: interface / mac / name / ip / mask / data。
	for _, arg := range spec.Args {
		switch arg.Name {
		case "interface":
			subCmd.Flags().String("interface", "", "DCP 操作端口名 (如 enp4s0)")
		case "mac":
			subCmd.Flags().String("mac", "", "目标设备 MAC 地址 (格式 XX:XX:XX:XX:XX:XX)")
		case "name":
			subCmd.Flags().String("name", "", "新设备名称")
		case "ip":
			subCmd.Flags().String("ip", "", "新 IP 地址")
		case "mask":
			subCmd.Flags().String("mask", "", "新子网掩码")
		case "data":
			subCmd.Flags().String("data", "", "完整 JSON payload (config-* 与 raw-send 必填)")
		}
		if arg.Required {
			_ = subCmd.MarkFlagRequired(arg.Name)
		}
	}

	return subCmd
}

// buildNRCClient 根据 retryFlag 构造 nrc.Client。
//
// retryFlag == 0 (默认): 使用 nrc.NewClient 的默认策略 (DCP 1 次重试)
// retryFlag > 0: 用 nrc.NewReceivePolicy(retryFlag) 覆盖默认 receive policy
//
//	(Step 9.2 --retry flag)
//	- retryFlag=3 表示总共尝试 4 次 (1 次 + 3 次重试)
//	- 网络抖动场景下推荐 3-5 次
func buildNRCClient(addr string) *nrc.Client {
	if retryFlag > 0 {
		return nrc.NewClient(addr, nrc.WithReceivePolicy(nrc.NewReceivePolicy(retryFlag)))
	}
	return nrc.NewClient(addr)
}

// collectDCPArgs 从 cmd flags 中提取 DCP 命令的参数集合, 供 nrc.RequestBody 构造请求体。
// 仅包含已被注册的 flag (通过 spec.Args 注册), 未注册的 flag 跳过。
// 返回的 map 始终非 nil, 即使为空。
//
// raw-send 命令使用 "data" 参数 (非 DCP), 也由本函数一并提取 (其他 DCP 命令不会注册 data flag, 自动跳过)。
// config-* 写命令 (Step 10.A) 使用 "data" 参数; --no-fetch 标志映射为 args["no-fetch"]="true" 字符串。
// --target 持久标志映射为 args["target"], 供 BodyBuilder 自动 fetch 使用。
func collectDCPArgs(cmd *cobra.Command) map[string]string {
	args := make(map[string]string)
	// --target 持久标志 (在 rootCmd 上注册, 通过 PersistentFlags 也能从 subCmd 取到)
	args["target"] = targetFlag
	// bool 标志: --no-fetch
	if cmd.Flags().Lookup("no-fetch") != nil {
		if v, _ := cmd.Flags().GetBool("no-fetch"); v {
			args["no-fetch"] = "true"
		}
	}
	for _, name := range []string{"interface", "mac", "name", "ip", "mask", "data"} {
		if cmd.Flags().Lookup(name) == nil {
			continue
		}
		if v, _ := cmd.Flags().GetString(name); v != "" {
			args[name] = v
		}
	}
	return args
}

func runNrcCommand(name string) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		start := time.Now()
		spec, ok := nrc.LookupByName(name)
		if !ok {
			return &output.Error{
				Type:    "protocol",
				Code:    "unknown_command",
				Message: fmt.Sprintf("命令未注册: %q", name),
			}
		}

		// === 纯客户端命令 —— 短路, 不连接工业 PC ===
		// schema-list 等命令不发 NRC 帧, 不检查 --target, 不走 Risk 流程。
		if spec.Group == nrc.GroupSchema {
			data := buildSchemaJSON()
			notice := map[string]interface{}{
				"command":       spec.Name,
				"command_count": schemaCommandCount(),
				"group_count":   len(schemaGroupList()),
			}
			return output.WriteSuccess(cmd.OutOrStdout(), data, notice)
		}

		if targetFlag == "" {
			return &output.Error{
				Type:    "validation",
				Code:    "target_required",
				Message: "--target 不能为空",
				Hint:    "请用 --target 192.168.x.x 指定工业 PC IP",
			}
		}

		if spec.Risk != nrc.RiskRead {
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			if dryRun {
				body, err := nrc.RequestBody(spec, collectDCPArgs(cmd))
				if err != nil {
					return fmt.Errorf("构造请求体失败: %w", err)
				}
				if err := output.PrintDryRunFrame(cmd.OutOrStdout(), spec, body); err != nil {
					return fmt.Errorf("打印 dry-run 帧失败: %w", err)
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "🛑 --dry-run 模式: 已跳过连接和发送\n")
				return nil
			}

			yesFlag, _ := cmd.Flags().GetBool("yes")
			if !yesFlag {
				if spec.Risk == nrc.RiskHighRiskWrite {
					return output.ConfirmationRequired(spec.Name)
				}
				return output.YesRequired(spec.Name)
			}
			if spec.Risk == nrc.RiskHighRiskWrite {
				fmt.Fprintf(os.Stderr, "⚠️  高危操作: %s (high-risk-write)\n    已通过 --yes, 即将发送请求到 %s\n",
					spec.Name, targetFlag)
			}
		}

		addr := targetFlag
		if !strings.Contains(addr, ":") {
			addr += ":6000"
		}
		fmt.Fprintf(os.Stderr, "🔌 连接 %s ...\n", addr)

		client := buildNRCClient(addr)
		if err := client.Connect(); err != nil {
			return fmt.Errorf("连接失败: %w", err)
		}
		defer client.Close()
		// 每次命令执行后清除预取拓扑缓存 (避免跨命令泄漏)
		defer nrc.ClearPreFetchedTopology()
		fmt.Fprintln(os.Stderr, "  ✓ 已连接")

		// 预取 topology: 用当前连接而非另建, 避免双连接导致 C++ 端关闭原有连接。
		// configBodyBuilder 会优先使用此缓存, 不再自行 fetch。
		if spec.Group == nrc.GroupConfig && spec.Risk != nrc.RiskRead {
			nrc.PreFetchTopology(client)
		}

		body, err := nrc.RequestBody(spec, collectDCPArgs(cmd))
		if err != nil {
			return fmt.Errorf("构造请求体失败: %w", err)
		}
		fmt.Fprintf(os.Stderr, "📤 发送 %s (DataType=%d, Function=%q)\n",
			spec.Name, spec.DataType, spec.Function)

		respCmd, data, err := client.SendReceiveFiltered(spec.Code, body, spec.DataType, nrc.ExpectedResponseCode(spec))
		if err != nil {
			// ⚠️ 2026-06-04 v3 bug fix (Step 10.B 实机发现):
			//
			// 旧版 "RiskWrite + closed" 启发式过宽 — 把所有 DataType=12 config 写命令
			// (SetPNDriver/AddPNDevice/...) 也吞掉了 "closed" 错误, 误判为成功, 但
			// C++ 端 DataType=12 实际会发响应, "closed" 意味着请求未到达 / 连接异常,
			// 不能再视为成功。
			//
			// "closed" 启发式应**仅**用于 DCP 写操作 (DataType=14, Function=2/3) —
			// DCP 帧发完后控制器主动关连接, **不**响应 JSON。其它 DataType 关闭错误需
			// 显式报告, 让用户/AI 能定位问题。
			if isDCPWriteClosedConnection(spec, err) {
				notice := map[string]interface{}{
					"command":     spec.Name,
					"data_type":   spec.DataType,
					"dcp_write":   true,
					"elapsed_ms":  time.Since(start).Milliseconds(),
					"verify_with": "topology-scan",
				}
				if err := output.WriteSuccess(cmd.OutOrStdout(), []byte("null"), notice); err != nil {
					return fmt.Errorf("信封构造失败: %w", err)
				}
				fmt.Fprintln(cmd.ErrOrStderr(), "✅ DCP 操作已发送 (无 JSON 响应, 请用 topology scan 验证)")
				return nil
			}
			return fmt.Errorf("通信失败: %w", err)
		}

		expected := nrc.ExpectedResponseCode(spec)
		if respCmd != expected {
			return &output.Error{
				Type:    "protocol",
				Code:    "unexpected_response_command",
				Message: fmt.Sprintf("意外响应命令字: 0x%04X (期望: 0x%04X)", respCmd, expected),
			}
		}

		outPath := outputFlag
		if outPath == "" {
			outPath = fmt.Sprintf("%s_response_%s.json",
				spec.Name, time.Now().Format("20060102_150405"))
		}
		if !filepath.IsAbs(outPath) {
			wd, _ := os.Getwd()
			outPath = filepath.Join(wd, outPath)
		}
		if err := os.WriteFile(outPath, data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  保存原始响应失败: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "💾 原始响应已保存: %s\n", outPath)
		}

		notice := map[string]interface{}{
			"command":    spec.Name,
			"data_type":  spec.DataType,
			"elapsed_ms": time.Since(start).Milliseconds(),
		}

		// === --format table 分支 (Step 8): 表格输出到 stdout, 不走 Envelope ===
		// table 是人类看的, 不混入 Envelope, 避免破坏 AI pipe 链。
		// 无数组时 (如 device list 的 CallBackJson) 回退 Envelope, stderr 警告。
		if formatFlag == "table" {
			cols, rows, terr := output.ExtractTableRows(data)
			if terr != nil || len(cols) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "⚠️  --format table 不适用此命令, 回退 JSON 输出")
				if err := output.WriteSuccess(cmd.OutOrStdout(), data, notice); err != nil {
					return fmt.Errorf("信封构造失败: %w", err)
				}
				return nil
			}
			if err := output.FormatTable(cmd.OutOrStdout(), cols, rows); err != nil {
				return fmt.Errorf("表格渲染失败: %w", err)
			}
			return nil
		}

		// === --format csv / ndjson 分支 (Step 9.4): 结构化输出, 适合 Excel / jq 处理 ===
		// 无数组时回退 Envelope + stderr 警告, 与 table 一致语义。
		if formatFlag == "csv" || formatFlag == "ndjson" {
			cols, rows, terr := output.ExtractTableRows(data)
			if terr != nil || len(cols) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"⚠️  --format %s 不适用此命令, 回退 JSON 输出\n", formatFlag)
				if err := output.WriteSuccess(cmd.OutOrStdout(), data, notice); err != nil {
					return fmt.Errorf("信封构造失败: %w", err)
				}
				return nil
			}
			if formatFlag == "csv" {
				if err := output.FormatCSV(cmd.OutOrStdout(), cols, rows); err != nil {
					return fmt.Errorf("CSV 渲染失败: %w", err)
				}
			} else {
				if err := output.FormatNDJSON(cmd.OutOrStdout(), cols, rows); err != nil {
					return fmt.Errorf("NDJSON 渲染失败: %w", err)
				}
			}
			return nil
		}

		if err := output.WriteSuccess(cmd.OutOrStdout(), data, notice); err != nil {
			return fmt.Errorf("信封构造失败: %w", err)
		}
		return nil
	}
}

func installRiskHelpFunc(root *cobra.Command) {
	defaultHelp := root.HelpFunc()
	root.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		defaultHelp(cmd, args)
		if risk, ok := cmd.Annotations[nrc.AnnotationRisk]; ok {
			fmt.Fprintf(cmd.OutOrStdout(), "\nRisk: %s\n", risk)
		}
	})
}

// === schema-list 纯客户端命令辅助函数 ===

// cmdEntry 是 schema JSON 中单条命令的结构。
type cmdEntry struct {
	Name        string             `json:"name"`
	Group       string             `json:"group"`
	Use         string             `json:"use"`
	Description string             `json:"description"`
	Risk        string             `json:"risk"`
	DataType    int                `json:"data_type"`
	Function    string             `json:"function"`
	Args        []nrc.ArgumentSpec `json:"args"`
}

// groupEntry 是 schema JSON 中单个 group 的汇总。
type groupEntry struct {
	Count int    `json:"count"`
	Risk  string `json:"risk"`
}

// buildSchemaJSON 构造 schema-list 命令的响应 JSON。
// 遍历 nrc.Registry, 跳过自身 (schema-list), 输出 commands[] + groups{}。
func buildSchemaJSON() []byte {
	commands := make([]cmdEntry, 0, len(nrc.Registry)-1)
	groupCounts := make(map[string]int)
	groupHasRead := make(map[string]bool)
	groupHasWrite := make(map[string]bool)
	groupHasHighRisk := make(map[string]bool)

	for _, spec := range nrc.Registry {
		if spec.Group == nrc.GroupSchema {
			continue // 不报告 schema-list 自身
		}
		// use = spec.Name 去掉 "<group>-" 前缀 (如 "gsd-list" → "list")
		use := spec.Name
		if prefix := string(spec.Group) + "-"; strings.HasPrefix(spec.Name, prefix) {
			use = spec.Name[len(prefix):]
		}
		commands = append(commands, cmdEntry{
			Name:        spec.Name,
			Group:       string(spec.Group),
			Use:         use,
			Description: spec.Description,
			Risk:        string(spec.Risk),
			DataType:    spec.DataType,
			Function:    spec.Function,
			Args:        spec.Args,
		})
		g := string(spec.Group)
		groupCounts[g]++
		switch spec.Risk {
		case nrc.RiskRead:
			groupHasRead[g] = true
		case nrc.RiskWrite:
			groupHasWrite[g] = true
		case nrc.RiskHighRiskWrite:
			groupHasHighRisk[g] = true
		}
	}

	groups := make(map[string]groupEntry, len(groupCounts))
	for g, count := range groupCounts {
		groups[g] = groupEntry{Count: count, Risk: groupRiskLabel(groupHasRead[g], groupHasWrite[g], groupHasHighRisk[g])}
	}

	data := map[string]interface{}{
		"commands": commands,
		"groups":   groups,
	}
	out, _ := json.MarshalIndent(data, "", "  ")
	return out
}

// groupRiskLabel 根据 group 内的 risk 分布, 推断最合适的单标签。
//   - 包含 high-risk-write → "high-risk-write" (最高风险)
//   - 包含 write/read 混合 → "mixed"
//   - 仅 write → "write"
//   - 仅 read → "read"
func groupRiskLabel(hasRead, hasWrite, hasHighRisk bool) string {
	switch {
	case hasHighRisk:
		return "high-risk-write"
	case hasWrite && hasRead:
		return "mixed"
	case hasWrite:
		return "write"
	default:
		return "read"
	}
}

// schemaCommandCount 返回 schema 输出中的命令数 (不含 schema-list 自身)。
func schemaCommandCount() int {
	n := 0
	for _, s := range nrc.Registry {
		if s.Group != nrc.GroupSchema {
			n++
		}
	}
	return n
}

// === device setup 组合命令 (Step 9.4) ===
//
// 组合 device setup-name + device setup-ip + topology scan 三个子命令,
// 一次执行完成 DCP 设备的发现-命名-配 IP-验证闭环。
//
// 与单命令的关系:
//   - `inl device setup-name` 仅设名称
//   - `inl device setup-ip`   仅设 IP
//   - `inl device setup`      一次完成上述两步 + topology scan 验证
//
// 注意: 组合命令不走 Registry, 走特殊路径, 因为它执行多个 sub-command。

// buildDeviceSetupCmd 构造 `inl device setup` 子命令 (Step 9.4)。
func buildDeviceSetupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "一键设置 DCP 设备 (名称+IP+验证, 组合 device setup-name + device setup-ip + topology scan)",
		Long: `组合 device setup-name + device setup-ip + topology scan 三个子命令,
一次完成 DCP 设备的发现-命名-配 IP-验证闭环。

DCP 写操作无 JSON 响应, 控制器发完即关连接。本命令通过 topology scan
验证设备是否被正确发现, 取代手动 scan 步骤。

示例:
  inl --target 192.168.3.15 device setup \
      --interface enp4s0 \
      --mac 00:11:22:33:44:55 \
      --name heron-weld \
      --ip 192.168.2.10 \
      --mask 255.255.255.0 \
      --yes`,
		Annotations: map[string]string{
			nrc.AnnotationRisk: string(nrc.RiskWrite), // 顶层 Risk 是 write (含 setup-name/setup-ip)
		},
		Args: cobra.NoArgs,
		RunE: runDeviceSetup,
	}
	cmd.Flags().String("interface", "", "DCP 操作端口名 (如 enp4s0)")
	cmd.Flags().String("mac", "", "目标设备 MAC 地址 (格式 XX:XX:XX:XX:XX:XX)")
	cmd.Flags().String("name", "", "新设备名称 (必填)")
	cmd.Flags().String("ip", "", "新 IP 地址 (可选, 不传则只设名称)")
	cmd.Flags().String("mask", "", "新子网掩码 (与 --ip 配套)")
	cmd.Flags().Bool("yes", false, "确认执行 write 操作 (必需)")
	_ = cmd.MarkFlagRequired("interface")
	_ = cmd.MarkFlagRequired("mac")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

// runDeviceSetup 实际执行 device setup 组合命令 (Step 9.4)。
//
// 步骤:
//  1. 解析与验证 flags
//  2. 顺序执行:
//     a. device-setup-name (设名称)
//     b. device-setup-ip   (设 IP + mask, 若提供)
//     c. topology-scan     (验证设备已发现)
//  3. 报告最终状态
//
// 错误处理: 任一步骤失败立即返回, 后续步骤不执行。
// --yes 必需 (Risk=write)。
func runDeviceSetup(cmd *cobra.Command, _ []string) error {
	ifname, _ := cmd.Flags().GetString("interface")
	mac, _ := cmd.Flags().GetString("mac")
	name, _ := cmd.Flags().GetString("name")
	ip, _ := cmd.Flags().GetString("ip")
	mask, _ := cmd.Flags().GetString("mask")

	if ifname == "" || mac == "" || name == "" {
		return &output.Error{
			Type:    "validation",
			Code:    "missing_args",
			Message: "--interface, --mac, --name 都不能为空",
		}
	}
	if ip != "" && mask == "" {
		return &output.Error{
			Type:    "validation",
			Code:    "missing_mask",
			Message: "提供 --ip 时必须同时提供 --mask",
		}
	}
	if targetFlag == "" {
		return &output.Error{
			Type:    "validation",
			Code:    "target_required",
			Message: "--target 不能为空",
			Hint:    "请用 --target 192.168.x.x 指定工业 PC IP",
		}
	}

	yesFlag, _ := cmd.Flags().GetBool("yes")
	if !yesFlag {
		return output.YesRequired("device-setup")
	}

	// Step 1: device setup-name
	fmt.Fprintln(cmd.ErrOrStderr(), "📝 Step 1/3: 设置设备名称...")
	if err := executeSetupSubCommand(cmd, "device-setup-name", map[string]string{
		"interface": ifname,
		"mac":       mac,
		"name":      name,
	}); err != nil {
		return fmt.Errorf("setup-name 失败: %w", err)
	}

	// Step 2: device setup-ip (若提供 --ip)
	if ip != "" {
		fmt.Fprintln(cmd.ErrOrStderr(), "🌐 Step 2/3: 设置设备 IP...")
		if err := executeSetupSubCommand(cmd, "device-setup-ip", map[string]string{
			"interface": ifname,
			"mac":       mac,
			"ip":        ip,
			"mask":      mask,
		}); err != nil {
			return fmt.Errorf("setup-ip 失败: %w", err)
		}
	} else {
		fmt.Fprintln(cmd.ErrOrStderr(), "⏭️  Step 2/3: 跳过 IP 设置 (未提供 --ip)")
	}

	// Step 3: topology scan (验证)
	fmt.Fprintln(cmd.ErrOrStderr(), "🔍 Step 3/3: 验证设备可见 (topology scan)...")
	if err := executeSetupSubCommand(cmd, "topology-scan", map[string]string{
		"interface": ifname,
	}); err != nil {
		return fmt.Errorf("topology-scan 验证失败: %w", err)
	}

	fmt.Fprintln(cmd.ErrOrStderr(), "✅ setup 完成 (3/3 子命令执行成功)")
	return nil
}

// executeSetupSubCommand 执行单个 setup 子命令, 输出到 cmd.ErrOrStderr。
//
// 与 runNrcCommand 的区别:
//   - 不走 Cobra flag 解析 (直接传 args map)
//   - 不发 Envelope (只 stderr 报告成功/失败)
//   - DCP 写无响应处理: 视为成功
//   - topology scan 输出: 写到 stdout (供 AI 消费)
func executeSetupSubCommand(cmd *cobra.Command, specName string, args map[string]string) error {
	spec, ok := nrc.LookupByName(specName)
	if !ok {
		return fmt.Errorf("未注册命令: %s", specName)
	}

	addr := targetFlag
	if !strings.Contains(addr, ":") {
		addr += ":6000"
	}

	body, err := nrc.RequestBody(spec, args)
	if err != nil {
		return fmt.Errorf("构造请求体失败: %w", err)
	}

	client := buildNRCClient(addr)
	if err := client.Connect(); err != nil {
		return fmt.Errorf("连接失败: %w", err)
	}
	defer client.Close()

	respCmd, data, err := client.SendReceiveFiltered(spec.Code, body, spec.DataType, nrc.ExpectedResponseCode(spec))
	if err != nil {
		// ⚠️ 2026-06-04 v3 启发式收紧 (Step 10.B 维护性修复, 与 runNrcCommand 对齐):
		//
		// "closed" 启发式**仅**对 DCP 写操作 (DataType=14, Function=2/3) 视为成功 —
		// DCP 帧发完后控制器主动关连接, **不**响应 JSON。其它 DataType (如 DataType=12
		// config 写) 出现 "closed" 错误意味着请求未到达 / 连接异常, **不**应误判为成功。
		//
		// 当前 executeSetupSubCommand 只被 device-setup-name/ip (DataType=14, Risk=Write)
		// 和 topology-scan (DataType=14, Risk=Read) 调用, 显式加 DataType=14 守卫可
		// 防止未来扩展时把 DataType=12 Risk=Write 命令误吞。
		if isDCPWriteClosedConnection(spec, err) {
			fmt.Fprintf(cmd.ErrOrStderr(), "  ✓ %s 已发送 (无 JSON 响应, 控制器发完即关)\n", specName)
			return nil
		}
		return fmt.Errorf("通信失败: %w", err)
	}

	expected := nrc.ExpectedResponseCode(spec)
	if respCmd != expected {
		return fmt.Errorf("意外响应命令字: 0x%04X (期望: 0x%04X)", respCmd, expected)
	}

	// topology-scan 响应写到 stdout (AI 消费)
	// setup-name/setup-ip 已在前面 closed 分支处理
	if specName == "topology-scan" {
		cmd.OutOrStdout().Write(data)
		cmd.OutOrStdout().Write([]byte("\n"))
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "  ✓ %s 成功\n", specName)
	return nil
}

// schemaGroupList 返回 Registry 中去重并排序的 group 名称列表。
func schemaGroupList() []string {
	seen := make(map[string]bool)
	var groups []string
	for _, s := range nrc.Registry {
		if seen[string(s.Group)] {
			continue
		}
		seen[string(s.Group)] = true
		groups = append(groups, string(s.Group))
	}
	return groups
}
