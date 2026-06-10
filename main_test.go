package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/your-org/inl/internal/nrc"
	"github.com/your-org/inl/internal/output"
)

// buildRootCmdForTest 构造一个 rootCmd 用于测试(简化版)
func buildRootCmdForTest() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "inl",
		Short:         "工业 PC NRC Socket 协议 CLI 工具",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	rootCmd.PersistentFlags().StringVar(&targetFlag, "target", "", "工业 PC IP 地址")
	for _, g := range []nrc.CommandGroup{
		nrc.GroupInterface, nrc.GroupGsd, nrc.GroupDevice, nrc.GroupConfig, nrc.GroupTopology, nrc.GroupSchema, nrc.GroupRaw,
	} {
		rootCmd.AddCommand(buildGroupCmd(g))
	}
	installRiskHelpFunc(rootCmd)
	return rootCmd
}

func TestDryRunFlag(t *testing.T) {
	root := buildRootCmdForTest()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	// Step 10.A: config-add-device 需要 --data, 此处提供有效 payload + --no-fetch
	// (避免 BodyBuilder 自动 fetch 触发网络调用)
	root.SetArgs([]string{
		"--target", "1.2.3.4",
		"config", "add-device",
		"--data", `{"RefGSD":"gsdml.xml","DAP_ID":"0x0001"}`,
		"--no-fetch",
		"--dry-run",
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	outStr := stdout.String()
	if !strings.Contains(outStr, "0x4E66") {
		t.Errorf("stdout 应含 SyncByte 0x4E66, got: %s", outStr)
	}
	if !strings.Contains(outStr, "AddPNDevice") {
		t.Errorf("stdout 应含 AddPNDevice, got: %s", outStr)
	}
	if !strings.Contains(stderr.String(), "🛑 --dry-run 模式") {
		t.Errorf("stderr 应含 --dry-run 模式提示, got: %s", stderr.String())
	}
}

func TestWriteRequiresYes(t *testing.T) {
	savedTarget := targetFlag
	targetFlag = "1.2.3.4"
	defer func() { targetFlag = savedTarget }()

	spec, _ := nrc.LookupByName("config-add-device")

	err := output.YesRequired(spec.Name)
	if err == nil {
		t.Fatal("expected non-nil")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "yes_required") {
		t.Errorf("err 应含 yes_required, got: %s", errStr)
	}
	if !strings.Contains(errStr, "write") {
		t.Errorf("err 应含 risk_level write, got: %s", errStr)
	}
}

func TestCompileRequiresConfirmation(t *testing.T) {
	savedTarget := targetFlag
	targetFlag = "1.2.3.4"
	defer func() { targetFlag = savedTarget }()

	spec, _ := nrc.LookupByName("config-compile")

	err := output.ConfirmationRequired(spec.Name)
	if err == nil {
		t.Fatal("expected non-nil")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "confirmation_required") {
		t.Errorf("err 应含 confirmation_required, got: %s", errStr)
	}
	if !strings.Contains(errStr, "high-risk-write") {
		t.Errorf("err 应含 risk_level high-risk-write, got: %s", errStr)
	}
	if !strings.Contains(errStr, "ai_auto_yes") {
		t.Errorf("err 应含 ai_auto_yes 字段, got: %s", errStr)
	}
}

func TestReadCommandNoYesRequired(t *testing.T) {
	root := buildRootCmdForTest()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"gsd", "list", "--help"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	outStr := stdout.String()
	if strings.Contains(outStr, "--yes") {
		t.Errorf("读命令 gsd list 不应含 --yes flag, got: %s", outStr)
	}
	if strings.Contains(outStr, "--dry-run") {
		t.Errorf("读命令 gsd list 不应含 --dry-run flag, got: %s", outStr)
	}
}

// === Step 7: schema-list 集成测试 ===

// TestSchemaList_NoTargetRequired 验证 schema list 是纯客户端命令, 无需 --target。
func TestSchemaList_NoTargetRequired(t *testing.T) {
	savedTarget := targetFlag
	targetFlag = ""
	defer func() { targetFlag = savedTarget }()

	root := buildRootCmdForTest()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"schema", "list"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	outStr := stdout.String()
	if !strings.Contains(outStr, `"ok": true`) {
		t.Errorf("stdout 应含 \"ok\": true, got: %s", outStr)
	}
	if !strings.Contains(outStr, `"identity": "inl"`) {
		t.Errorf("stdout 应含 identity=inl, got: %s", outStr)
	}
	if !strings.Contains(outStr, `"commands"`) {
		t.Errorf("stdout 应含 commands 字段, got: %s", outStr)
	}
	if !strings.Contains(outStr, `"groups"`) {
		t.Errorf("stdout 应含 groups 字段, got: %s", outStr)
	}
	if strings.Contains(stderr.String(), "target_required") {
		t.Errorf("schema list 不应触发 target_required, stderr: %s", stderr.String())
	}
}

// TestSchemaList_DoesNotIncludeSelf 验证 schema list 输出不含 schema-list 自身。
func TestSchemaList_DoesNotIncludeSelf(t *testing.T) {
	savedTarget := targetFlag
	targetFlag = ""
	defer func() { targetFlag = savedTarget }()

	root := buildRootCmdForTest()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"schema", "list"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	// 解析 stdout JSON 信封
	var env struct {
		OK   bool `json:"ok"`
		Data struct {
			Commands []map[string]interface{} `json:"commands"`
			Groups   map[string]struct {
				Count int    `json:"count"`
				Risk  string `json:"risk"`
			} `json:"groups"`
		} `json:"data"`
		Notice struct {
			Command      string `json:"command"`
			CommandCount int    `json:"command_count"`
			GroupCount   int    `json:"group_count"`
		} `json:"_notice"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("解析 stdout 失败: %v\noutput: %s", err, stdout.String())
	}

	if env.Notice.Command != "schema-list" {
		t.Errorf("Notice.command = %q, want schema-list", env.Notice.Command)
	}
	if env.Notice.CommandCount != 24 {
		// Step 8: 24 = 23 (旧) + 1 (raw-send)
		t.Errorf("Notice.command_count = %d, want 24 (不含自身, 含 raw-send)", env.Notice.CommandCount)
	}
	if env.Notice.GroupCount != 7 {
		// 7 个 group: gsd/device/config/interface/topology/schema/raw (groups 包含 schema + raw, 但 commands 不含)
		t.Errorf("Notice.group_count = %d, want 7 (含 raw 组)", env.Notice.GroupCount)
	}
	if len(env.Data.Commands) != 24 {
		t.Errorf("len(commands) = %d, want 24 (不含 schema-list 自身, 含 raw-send)", len(env.Data.Commands))
	}
	for _, c := range env.Data.Commands {
		if c["name"] == "schema-list" {
			t.Errorf("commands 不应含 schema-list 自身, got: %+v", c)
		}
	}
	if len(env.Data.Groups) != 6 {
		// groups 不含 schema 组 (schema 组内只有 schema-list 自身), 但含 raw 组 (有 raw-send)
		t.Errorf("len(groups) = %d, want 6 (gsd/device/config/interface/topology/raw)", len(env.Data.Groups))
	}
}

// TestBuildSchemaJSON 验证 buildSchemaJSON 输出结构。
func TestBuildSchemaJSON(t *testing.T) {
	data := buildSchemaJSON()
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("JSON 解析失败: %v\nraw: %s", err, data)
	}

	commands, ok := parsed["commands"].([]interface{})
	if !ok {
		t.Fatalf("commands 字段类型错误: %T", parsed["commands"])
	}
	if len(commands) != 24 {
		// Step 8: 24 = 23 (旧) + 1 (raw-send)
		t.Errorf("len(commands) = %d, want 24 (不含 schema-list 自身, 含 raw-send)", len(commands))
	}

	// 验证每条 command 含 8 个字段
	required := []string{"name", "group", "use", "description", "risk", "data_type", "function", "args"}
	for i, c := range commands {
		m, ok := c.(map[string]interface{})
		if !ok {
			t.Errorf("commands[%d] 不是 object: %T", i, c)
			continue
		}
		for _, f := range required {
			if _, ok := m[f]; !ok {
				t.Errorf("commands[%d] 缺字段 %q", i, f)
			}
		}
	}

	// 验证 config-compile risk == high-risk-write
	foundCompile := false
	for _, c := range commands {
		m := c.(map[string]interface{})
		if m["name"] == "config-compile" {
			if m["risk"] != "high-risk-write" {
				t.Errorf("config-compile.risk = %v, want high-risk-write", m["risk"])
			}
			foundCompile = true
		}
	}
	if !foundCompile {
		t.Error("commands 中找不到 config-compile")
	}

	// 验证 topology-scan args 含 interface 必填参数
	for _, c := range commands {
		m := c.(map[string]interface{})
		if m["name"] == "topology-scan" {
			args, ok := m["args"].([]interface{})
			if !ok || len(args) != 1 {
				t.Errorf("topology-scan.args = %+v, want [{interface}]", m["args"])
			} else {
				arg := args[0].(map[string]interface{})
				if arg["name"] != "interface" {
					t.Errorf("topology-scan.args[0].name = %v, want interface", arg["name"])
				}
				if arg["required"] != true {
					t.Errorf("topology-scan.args[0].required = %v, want true", arg["required"])
				}
			}
		}
	}

	groups, ok := parsed["groups"].(map[string]interface{})
	if !ok {
		t.Fatalf("groups 字段类型错误: %T", parsed["groups"])
	}
	if len(groups) != 6 {
		// Step 8: 6 = 5 (旧) + 1 (raw 组有 raw-send)
		t.Errorf("len(groups) = %d, want 6 (gsd/device/config/interface/topology/raw)", len(groups))
	}

	// config 组的 risk 应是 high-risk-write (因含 config-compile)
	configGroup, ok := groups["config"].(map[string]interface{})
	if !ok {
		t.Error("groups.config 缺失")
	} else {
		if configGroup["count"] != float64(12) {
			t.Errorf("groups.config.count = %v, want 12", configGroup["count"])
		}
		if configGroup["risk"] != "high-risk-write" {
			t.Errorf("groups.config.risk = %v, want high-risk-write", configGroup["risk"])
		}
	}
}

// === Step 11: --data JSON 子字段元数据 (fields) 单元测试 ===

// TestSchemaList_Fields 验证 --data 命令的 args[0].fields 输出正确。
func TestSchemaList_Fields(t *testing.T) {
	data := buildSchemaJSON()
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("JSON 解析失败: %v", err)
	}
	commands, ok := parsed["commands"].([]interface{})
	if !ok {
		t.Fatalf("commands 字段类型错误: %T", parsed["commands"])
	}

	// 测试 config-add-device
	var addDevice map[string]interface{}
	for _, c := range commands {
		m := c.(map[string]interface{})
		if m["name"] == "config-add-device" {
			addDevice = m
			break
		}
	}
	if addDevice == nil {
		t.Fatal("找不到 config-add-device")
	}

	args := addDevice["args"].([]interface{})
	if len(args) != 1 {
		t.Fatalf("config-add-device args 长度 = %d, want 1", len(args))
	}
	dataArg := args[0].(map[string]interface{})
	fields, ok := dataArg["fields"].([]interface{})
	if !ok {
		t.Fatalf("config-add-device args[0].fields 缺失或类型错误: %T", dataArg["fields"])
	}
	if len(fields) < 2 {
		t.Fatalf("config-add-device fields 长度 = %d, 应至少含 RefGSD/DAP_ID", len(fields))
	}

	fieldNames := make(map[string]bool)
	for _, f := range fields {
		fm := f.(map[string]interface{})
		fieldNames[fm["name"].(string)] = true
	}
	if !fieldNames["RefGSD"] {
		t.Error("fields 应含 RefGSD")
	}
	if !fieldNames["DAP_ID"] {
		t.Error("fields 应含 DAP_ID")
	}

	// 测试 topology-scan（DCP 结构化 flags）不输出 fields
	for _, c := range commands {
		m := c.(map[string]interface{})
		if m["name"] == "topology-scan" {
			scanArgs, ok := m["args"].([]interface{})
			if !ok || len(scanArgs) == 0 {
				continue
			}
			firstArg := scanArgs[0].(map[string]interface{})
			if _, hasFields := firstArg["fields"]; hasFields {
				t.Error("topology-scan.args[0] 不应含 fields 字段")
			}
		}
	}

	// 测试 raw-send（透传）不输出 fields
	for _, c := range commands {
		m := c.(map[string]interface{})
		if m["name"] == "raw-send" {
			rawArgs, ok := m["args"].([]interface{})
			if !ok || len(rawArgs) == 0 {
				continue
			}
			firstArg := rawArgs[0].(map[string]interface{})
			if _, hasFields := firstArg["fields"]; hasFields {
				t.Error("raw-send.args[0] 不应含 fields 字段（透传自由格式）")
			}
		}
	}

	// 测试 config-set-idevice 必填字段都存在
	for _, c := range commands {
		m := c.(map[string]interface{})
		if m["name"] == "config-set-idevice" {
			ideviceArgs := m["args"].([]interface{})
			if len(ideviceArgs) != 1 {
				t.Fatalf("config-set-idevice args 长度 = %d, want 1", len(ideviceArgs))
			}
			ideviceFields := ideviceArgs[0].(map[string]interface{})["fields"].([]interface{})
			ideviceFieldNames := make(map[string]bool)
			for _, f := range ideviceFields {
				fm := f.(map[string]interface{})
				ideviceFieldNames[fm["name"].(string)] = true
			}
			for _, want := range []string{"Activate", "InputLength", "OutputLength"} {
				if !ideviceFieldNames[want] {
					t.Errorf("config-set-idevice fields 应含 %s", want)
				}
			}
		}
	}
}

// === Step 10.B: "closed" 启发式收紧单元测试 (2026-06-04, 2026-06-08 扩展) ===
//
// 历史: 旧版用 `Risk == nrc.RiskWrite && strings.Contains(err, "closed")` 过宽, 把
// 所有 DataType=12 config 写命令的 "closed" 错误也吞掉了, 误判为成功。
// 修复后: 抽出 `isDCPWriteClosedConnection(spec, err)` 守卫:
//   - DCP 写 (DataType=14 + Risk=Write) → 视为成功 (发完即关)
//   - Compile (DataType=12 + Function="Compile") → 视为成功 (编译重启协议栈)
//   - 其它 DataType=12 config 写 → 不误吞

// TestIsDCPWriteClosedConnection 覆盖 spec.DataType / spec.Risk / spec.Function /
// err.Error() 四因素的真值表。
func TestIsDCPWriteClosedConnection(t *testing.T) {
	closedErr := fmt.Errorf("read tcp 127.0.0.1:6000: use of closed network connection")
	otherErr := fmt.Errorf("read tcp 127.0.0.1:6000: i/o timeout")

	// 12 条 config 写命令的代表性 spec (DataType=12 + Risk=Write)
	configSpec, ok := nrc.LookupByName("config-set-driver")
	if !ok {
		t.Fatal("config-set-driver 必须在 Registry 中存在")
	}

	// DCP 写命令的代表性 spec (DataType=14 + Risk=Write)
	dcpWriteSpec, ok := nrc.LookupByName("device-setup-name")
	if !ok {
		t.Fatal("device-setup-name 必须在 Registry 中存在")
	}

	// Compile 命令的代表性 spec (DataType=12 + Function="Compile")
	compileSpec, ok := nrc.LookupByName("config-compile")
	if !ok {
		t.Fatal("config-compile 必须在 Registry 中存在")
	}

	cases := []struct {
		name string
		spec nrc.CommandSpec
		err  error
		want bool
	}{
		// === DataType=12 config 写 (非 Compile) + closed → false ===
		{
			name: "config-set-driver+closed → false (修复重点: 不再误吞)",
			spec: configSpec, err: closedErr, want: false,
		},
		{
			name: "config-set-driver+非closed → false",
			spec: configSpec, err: otherErr, want: false,
		},
		{
			name: "config-set-driver+nil → false",
			spec: configSpec, err: nil, want: false,
		},

		// === DCP 写 + closed → true (保留原行为) ===
		{
			name: "DCP-写+closed → true (DCP 发完即关)",
			spec: dcpWriteSpec, err: closedErr, want: true,
		},
		{
			name: "DCP-写+非closed → false",
			spec: dcpWriteSpec, err: otherErr, want: false,
		},

		// === Compile + closed → true (2026-06-08: 编译重启协议栈, 必然关连接) ===
		{
			name: "config-compile+closed → true (编译重启协议栈, 视为成功)",
			spec: compileSpec, err: closedErr, want: true,
		},
		{
			name: "config-compile+非closed → false",
			spec: compileSpec, err: otherErr, want: false,
		},
		{
			name: "config-compile+nil → false",
			spec: compileSpec, err: nil, want: false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := isDCPWriteClosedConnection(c.spec, c.err)
			if got != c.want {
				t.Errorf("isDCPWriteClosedConnection(%s, %q) = %v, want %v",
					c.spec.Name, c.err, got, c.want)
			}
		})
	}
}
