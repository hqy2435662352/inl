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
		nrc.GroupInterface, nrc.GroupGsd, nrc.GroupDevice, nrc.GroupConfig, nrc.GroupTopology,
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

// 抑制 unused import 警告
var _ = json.Marshal
