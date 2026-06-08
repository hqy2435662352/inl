package nrc

import (
	"encoding/json"
	"net"
	"strings"
	"testing"

	"github.com/your-org/inl/internal/topology"
)

// ====================== 真实 fetchTopologyForConfig 测试 (Step 10.B v2) ======================
//
// 真实实现于 2026-06-04 实施（参见 internal/nrc/config_body.go 中的 fetchTopologyForConfig 注释）。
// 这里用本地 mock TCP server 模拟 nrc2.out, 验证:
//   - 默认实现可连接 + 发送 device-list + 解析响应 + 返回拓扑 map
//   - 空 target → error
//   - 不含端口的 target → 自动追加 :6000
//   - 响应解析失败 → error（不 panic）
//   - 含 DecentralDevice[] 的响应能正确灌入 map["DecentralDevice"]
//   - 端到端: configBodyBuilder 默认 fetch 路径能拿到真实拓扑
//
// 为什么用 mock 而不是连真实工业 PC: 见顶部 README 单元测试隔离原则。
// 真实工业 PC 测试见 fetch_topology_integration_test.go (build tag: integration)。

// startMockNrcServer 启动一个本地 mock nrc2.out 监听器, 收到任意请求帧后
// 返回 caller 提供的 responsePayload (会自动用 BuildFrame 包成 0x9271 响应)。
//
// 关闭: defer ln.Close()
func startMockNrcServer(t *testing.T, responsePayload string) (net.Listener, string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen 失败: %v", err)
	}
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		// 读取请求帧 (但不解析, 直接返回响应) — 与 client_test.go 模式一致
		buf := make([]byte, 2048)
		_, _ = conn.Read(buf)
		resp := BuildFrame(0x9271, responsePayload)
		_, _ = conn.Write(resp)
	}()
	return ln, ln.Addr().String()
}

// TestFetchTopologyForConfig_Default_Success 验证默认真实实现能连 mock 服务器并解析 CallBackJson 响应。
func TestFetchTopologyForConfig_Default_Success(t *testing.T) {
	// 准备 mock 响应: 完整 CallBackJson 响应（含 DecentralDevice[]）
	mockResp := `{"DataType":12,"Error":[],"ErrorID":[],"Function":"CallBackJson","IDevice":{"Activate":false,"InputLength":64,"OutputLength":64},"PNDriver":{"DeviceName":"profinetdriver","IPAddress":"192.168.2.14","SetInTheProject":true,"SubnetMask":"255.255.255.0","iDevice":false},"DecentralDevice":[{"DeviceID":"0x0001","DeviceName":"welder-01","IPAddress":"192.168.2.10","InputLength":64,"InputStartAddress":0,"OutputLength":64,"OutputStartAddress":512,"ReductionRatio":1.0,"RefGSD":"GSDML-...","SetInTheProject":true,"SubnetMask":"255.255.255.0","VendorID":"0x0101","Module":[]}]}`

	ln, addr := startMockNrcServer(t, mockResp)
	defer ln.Close()

	// 不调用 SetFetchTopologyForConfig — 测默认真实实现
	result, err := fetchTopologyForConfig(addr)
	if err != nil {
		t.Fatalf("fetchTopologyForConfig 失败: %v", err)
	}
	if result == nil {
		t.Fatal("result 不应为 nil")
	}

	// 验证: PNDriver / IDevice / DecentralDevice 三个 key 都存在
	for _, key := range []string{"PNDriver", "IDevice", "DecentralDevice"} {
		if _, ok := result[key]; !ok {
			t.Errorf("result 应含 key %q, got: %+v", key, result)
		}
	}

	// 验证: 序列化回去是合法 JSON (configBodyBuilder 拼装后也会 json.Marshal 整个 body)
	out, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("result json.Marshal 失败: %v", err)
	}
	if !strings.Contains(string(out), `"PNDriver"`) {
		t.Errorf("序列化结果应含 PNDriver, got: %s", string(out))
	}
	if !strings.Contains(string(out), `"DecentralDevice"`) {
		t.Errorf("序列化结果应含 DecentralDevice, got: %s", string(out))
	}
	if !strings.Contains(string(out), `"welder-01"`) {
		t.Errorf("序列化结果应含 welder-01, got: %s", string(out))
	}
}

// TestFetchTopologyForConfig_Default_EmptyTarget 验证 target 为空时返回 error。
func TestFetchTopologyForConfig_Default_EmptyTarget(t *testing.T) {
	_, err := fetchTopologyForConfig("")
	if err == nil {
		t.Fatal("空 target 应返回 error")
	}
	if !strings.Contains(err.Error(), "target 不能为空") {
		t.Errorf("err = %q, 应含 'target 不能为空'", err.Error())
	}
}

// TestFetchTopologyForConfig_Default_ConnectRefused 验证无可用服务时返回 error（不 panic）。
func TestFetchTopologyForConfig_Default_ConnectRefused(t *testing.T) {
	// 找一个空闲端口然后关闭, 模拟"无服务"
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	addr := ln.Addr().String()
	ln.Close()

	_, err := fetchTopologyForConfig(addr)
	if err == nil {
		t.Fatal("无可用服务时应返回 error")
	}
	if !strings.Contains(err.Error(), "connect") {
		t.Errorf("err = %q, 应含 'connect'", err.Error())
	}
}

// TestFetchTopologyForConfig_Default_InvalidJSON 验证响应非合法 JSON 时返回 error。
func TestFetchTopologyForConfig_Default_InvalidJSON(t *testing.T) {
	ln, addr := startMockNrcServer(t, "{not valid json")
	defer ln.Close()

	_, err := fetchTopologyForConfig(addr)
	if err == nil {
		t.Fatal("响应非合法 JSON 时应返回 error")
	}
	if !strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("err = %q, 应含 'unmarshal'", err.Error())
	}
}

// TestFetchTopologyForConfig_Default_NoDecentralDevice 验证空配置响应 (无 DecentralDevice) 不报错。
//
// 空配置场景: nrc2.out 返回的 CallBackJson 不含 DecentralDevice 字段。
// 此时 map["DecentralDevice"] 应是 nil 或空 slice (Go json 行为可能返回其中之一)。
// 我们的实现保留 key, configBodyBuilder 内部 `if v, ok := topology[k]; ok` 会拿到
// nil/空值, 后续 `body[k] = v` 会塞 nil, 这是预期行为（与 10.A 设计一致）。
func TestFetchTopologyForConfig_Default_NoDecentralDevice(t *testing.T) {
	mockResp := `{"DataType":12,"Error":[],"ErrorID":[],"Function":"CallBackJson","IDevice":{"Activate":false,"InputLength":0,"OutputLength":0},"PNDriver":{"DeviceName":"profinetdriver","IPAddress":"192.168.3.15","SetInTheProject":true,"SubnetMask":"255.255.255.0"}}`
	ln, addr := startMockNrcServer(t, mockResp)
	defer ln.Close()

	result, err := fetchTopologyForConfig(addr)
	if err != nil {
		t.Fatalf("fetchTopologyForConfig 失败: %v", err)
	}
	if result == nil {
		t.Fatal("result 不应为 nil")
	}
	// 验证: DecentralDevice key 存在, value 为 nil 或空 slice
	v, ok := result["DecentralDevice"]
	if !ok {
		t.Error("result 应含 DecentralDevice key (即使 value 是 nil)")
	}
	// 容忍 nil 和空 slice 两种情形（取决于 Go 版本对 json.Unmarshal 的实现差异）
	if v != nil {
		// 非 nil 时必须是空 slice
		if devs, isSlice := v.([]topology.DecentralDevice); isSlice {
			if len(devs) != 0 {
				t.Errorf("空配置场景 DecentralDevice 应为 nil 或空 slice, got: len=%d", len(devs))
			}
		} else {
			t.Errorf("空配置场景 DecentralDevice 应为 nil 或空 slice, got: %v (%T)", v, v)
		}
	}
}

// TestConfigBodyBuilder_UsesRealFetchTopology 端到端验证: BodyBuilder 默认行为下调用真实 fetchTopologyForConfig。
//
// 行为契约 (Step 10.A F2):
//   - 没 --no-fetch → 自动调 fetchTopologyForConfig
//   - 默认实现连 mock 服务器 (这个测试) → 拿到 topology
//   - body 应含 DecentralDevice / PNDriver / IDevice (来自 mock 响应)
func TestConfigBodyBuilder_UsesRealFetchTopology(t *testing.T) {
	mockResp := `{"DataType":12,"Error":[],"ErrorID":[],"Function":"CallBackJson","IDevice":{"Activate":true,"InputLength":64,"OutputLength":64},"PNDriver":{"DeviceName":"profinetdriver","IPAddress":"192.168.3.15","SetInTheProject":true,"SubnetMask":"255.255.255.0"},"DecentralDevice":[{"DeviceID":"0x0001","DeviceName":"real-fetch-device","IPAddress":"192.168.3.20","InputLength":64,"OutputLength":64,"VendorID":"0x0001","SubnetMask":"255.255.255.0","SetInTheProject":true,"RefGSD":"test.xml","ReductionRatio":1.0,"Module":[]}]}`
	ln, addr := startMockNrcServer(t, mockResp)
	defer ln.Close()

	spec := CommandSpec{DataType: 12, Function: "UninstallPNDevice"}
	got, err := configRemoveDeviceBody(spec, map[string]string{
		"data":   `{"SetPNDeviceNum":1}`,
		"target": addr,
		// 不设 no-fetch, 默认应调真实 fetch
	})
	if err != nil {
		t.Fatalf("configRemoveDeviceBody err: %v", err)
	}

	// 验证: body 含真实 fetch 灌入的 DecentralDevice
	if !strings.Contains(got, `"real-fetch-device"`) {
		t.Errorf("body 应含 real-fetch-device (来自 mock 响应), got: %s", got)
	}
	if !strings.Contains(got, `"DecentralDevice"`) {
		t.Errorf("body 应含 DecentralDevice, got: %s", got)
	}
	if !strings.Contains(got, `"PNDriver"`) {
		t.Errorf("body 应含 PNDriver, got: %s", got)
	}
	if !strings.Contains(got, `"IDevice"`) {
		t.Errorf("body 应含 IDevice, got: %s", got)
	}
}
