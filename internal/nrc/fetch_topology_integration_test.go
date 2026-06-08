//go:build integration
// +build integration

// 真实工业 PC 集成测试 (build tag: integration)。
//
// 运行方式:  go test -tags=integration ./internal/nrc/...
// 默认不运行, 避免 CI 环境找不到工业 PC 而失败。
//
// 环境变量:
//   - INL_INTEGRATION_TARGET: 工业 PC IP, 如 192.168.3.15 (必须)
//
// 验证项:
//   - 默认 fetchTopologyForConfig 能连真实 nrc2.out:6000
//   - 解析 CallBackJson 响应成功
//   - 返回的 map 包含 PNDriver / IDevice / DecentralDevice 三字段
package nrc

import (
	"os"
	"testing"
)

func TestFetchTopologyForConfig_IntegrationPC(t *testing.T) {
	target := os.Getenv("INL_INTEGRATION_TARGET")
	if target == "" {
		t.Skip("INL_INTEGRATION_TARGET 未设置, 跳过工业 PC 集成测试")
	}

	result, err := fetchTopologyForConfig(target)
	if err != nil {
		t.Fatalf("fetchTopologyForConfig(%s) 失败: %v", target, err)
	}
	if result == nil {
		t.Fatal("result 不应为 nil")
	}

	// 验证三个 key 都存在
	for _, key := range []string{"PNDriver", "IDevice", "DecentralDevice"} {
		if _, ok := result[key]; !ok {
			t.Errorf("result 应含 key %q, got: %+v", key, result)
		}
	}

	// DecentralDevice 可能是 nil (空配置场景), 仅当非 nil 且为 slice 时打 log
	switch v := result["DecentralDevice"].(type) {
	case nil:
		t.Logf("从 %s 拉取到空 DecentralDevice (nil, 空配置场景)", target)
	case []any:
		t.Logf("从 %s 拉取到 %d 个 DecentralDevice", target, len(v))
	default:
		t.Logf("从 %s 拉取到 DecentralDevice 字段, type=%T", target, v)
	}
}
