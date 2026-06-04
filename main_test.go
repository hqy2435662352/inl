package main

import (
	"bytes"
	"encoding/json"
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
		nrc.GroupInterface, nrc.GroupGsd, nrc.GroupDevice, nrc.GroupConfig, nrc.GroupTopology, nrc.GroupSchema,
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
	root.SetArgs([]string{"--target", "1.2.3.4", "config", "add-device", "--dry-run"})

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
	if env.Notice.CommandCount != 23 {
		t.Errorf("Notice.command_count = %d, want 23 (不含自身)", env.Notice.CommandCount)
	}
	if env.Notice.GroupCount != 6 {
		// 6 个 group: gsd/device/config/interface/topology/schema (groups 包含 schema 组, 但 commands 不含)
		t.Errorf("Notice.group_count = %d, want 6", env.Notice.GroupCount)
	}
	if len(env.Data.Commands) != 23 {
		t.Errorf("len(commands) = %d, want 23 (不含 schema-list 自身)", len(env.Data.Commands))
	}
	for _, c := range env.Data.Commands {
		if c["name"] == "schema-list" {
			t.Errorf("commands 不应含 schema-list 自身, got: %+v", c)
		}
	}
	if len(env.Data.Groups) != 5 {
		// groups 不含 schema 组 (因为 schema 组内没有 commands)
		t.Errorf("len(groups) = %d, want 5 (gsd/device/config/interface/topology)", len(env.Data.Groups))
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
	if len(commands) != 23 {
		t.Errorf("len(commands) = %d, want 23 (不含 schema-list 自身)", len(commands))
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
	if len(groups) != 5 {
		t.Errorf("len(groups) = %d, want 5 (gsd/device/config/interface/topology)", len(groups))
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

// 抑制 unused import 警告
var _ = json.Marshal
