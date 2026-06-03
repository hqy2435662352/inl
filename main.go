package main

import (
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
)

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
		"输出格式: json (默认) | table")
	rootCmd.PersistentFlags().StringVar(&outputFlag, "output", "",
		"原始响应保存路径 (默认: ./<name>_response_<时间戳>.json)")

	for _, g := range []nrc.CommandGroup{
		nrc.GroupInterface, nrc.GroupGsd, nrc.GroupDevice, nrc.GroupConfig, nrc.GroupTopology,
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

	// DCP 命令的参数注册: 根据 spec.Args 注册对应 String flag 并标记必填。
	// 参数全集: interface / mac / name / ip / mask。
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
		}
		if arg.Required {
			_ = subCmd.MarkFlagRequired(arg.Name)
		}
	}

	return subCmd
}

// collectDCPArgs 从 cmd flags 中提取 DCP 命令的参数集合, 供 nrc.RequestBody 构造请求体。
// 仅包含已被注册的 flag (通过 spec.Args 注册), 未注册的 flag 跳过。
// 返回的 map 始终非 nil, 即使为空。
func collectDCPArgs(cmd *cobra.Command) map[string]string {
	args := make(map[string]string)
	for _, name := range []string{"interface", "mac", "name", "ip", "mask"} {
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
		if targetFlag == "" {
			return &output.Error{
				Type:    "validation",
				Code:    "target_required",
				Message: "--target 不能为空",
				Hint:    "请用 --target 192.168.x.x 指定工业 PC IP",
			}
		}
		spec, ok := nrc.LookupByName(name)
		if !ok {
			return &output.Error{
				Type:    "protocol",
				Code:    "unknown_command",
				Message: fmt.Sprintf("命令未注册: %q", name),
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

		client := nrc.NewClient(addr)
		if err := client.Connect(); err != nil {
			return fmt.Errorf("连接失败: %w", err)
		}
		defer client.Close()
		fmt.Fprintln(os.Stderr, "  ✓ 已连接")

		body, err := nrc.RequestBody(spec, collectDCPArgs(cmd))
		if err != nil {
			return fmt.Errorf("构造请求体失败: %w", err)
		}
		fmt.Fprintf(os.Stderr, "📤 发送 %s (DataType=%d, Function=%q)\n",
			spec.Name, spec.DataType, spec.Function)

		respCmd, data, err := client.SendReceiveFiltered(spec.Code, body, spec.DataType, nrc.ExpectedResponseCode(spec))
		if err != nil {
			// DCP 写操作 (DataType=14, Function=2/3) 无 JSON 响应,
			// 控制器发完 DCP 帧后直接关连接 → 视为成功, stdout 输出 Envelope (data: null)。
			if spec.Risk == nrc.RiskWrite && strings.Contains(err.Error(), "closed") {
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
