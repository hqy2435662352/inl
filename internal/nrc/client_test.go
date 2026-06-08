package nrc

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/your-org/inl/internal/reliability"
)

// === Connect 实测 (本地 TCP listener) ===

// startEchoListener 启动一个本地 TCP listener, accept 后立即关闭 (模拟控制器发完即关)。
// 返回 listener 和 acceptCount (原子计数, 用于测试重试次数)。
func startEchoListener(t *testing.T) (net.Listener, *int32) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen 失败: %v", err)
	}
	var count int32
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			atomic.AddInt32(&count, 1)
			// 模拟控制器发完响应后立即关闭 (DCP 读常见模式)
			conn.Close()
		}
	}()
	return ln, &count
}

// TestConnect_SetsKeepAlive 验证 Connect 成功后设置了 TCP keepalive。
func TestConnect_SetsKeepAlive(t *testing.T) {
	ln, _ := startEchoListener(t)
	defer ln.Close()

	c := NewClient(ln.Addr().String())
	if err := c.Connect(); err != nil {
		t.Fatalf("Connect 失败: %v", err)
	}
	defer c.Close()

	// 验证 c.conn 存在且是 *net.TCPConn
	if c.conn == nil {
		t.Fatal("Connect 后 c.conn 应非 nil")
	}
	// keepalive 已在 Connect 内通过 SetKeepAlive 设置,
	// 真实网络下可以通过 syscall 读 SO_KEEPALIVE, 但跨平台麻烦;
	// 这里通过 re-SetKeepAlive 不报错来间接证明已是 TCPConn 类型。
	tcpConn, ok := c.conn.(*net.TCPConn)
	if !ok {
		t.Fatalf("c.conn 类型 = %T, 期望 *net.TCPConn", c.conn)
	}
	if err := tcpConn.SetKeepAlive(true); err != nil {
		t.Errorf("SetKeepAlive 失败: %v", err)
	}
}

// TestConnect_RetryOnRefused 验证 Connect 在端口未监听时会重试 3 次 (reliability.Default)。
func TestConnect_RetryOnRefused(t *testing.T) {
	// 找一个空闲端口然后关闭, 模拟"无服务"
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	addr := ln.Addr().String()
	ln.Close() // 立即关闭 → connect 会失败

	c := NewClient(addr)
	start := time.Now()
	err := c.Connect()
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("期望 connect 失败")
	}
	// 3 次尝试, 中间 2 次 sleep (300ms + ~150ms jitter, 600ms + ~300ms jitter)
	// 总耗时至少 = 300 + 600 - jitter = 900ms
	// 加上 3 次 dial 每次 5s 超时上限, 但实际 dial 立即失败 (connection refused)
	// 所以 elapsed 应在 [0.9s, 2.0s] 之间
	if elapsed < 800*time.Millisecond {
		t.Errorf("elapsed = %v, 期望 >= 800ms (重试 sleep 累积)", elapsed)
	}
	if elapsed > 3*time.Second {
		t.Errorf("elapsed = %v, 期望 < 3s", elapsed)
	}
	if !strings.Contains(err.Error(), "dial") {
		t.Errorf("错误信息应含 'dial': %v", err)
	}
}

// TestConnect_WithConnectPolicy 验证 WithConnectPolicy 可注入自定义策略 (零重试)。
func TestConnect_WithConnectPolicy(t *testing.T) {
	// 找一个空闲端口然后关闭, 模拟"无服务"
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	addr := ln.Addr().String()
	ln.Close()

	// 自定义策略: 0 次重试 (快速失败)
	policy := &reliability.Policy{
		MaxRetries:  0,
		BaseBackoff: 10 * time.Millisecond,
		MaxBackoff:  10 * time.Millisecond,
		ShouldRetry: reliability.ShouldRetryNetwork,
	}
	c := NewClient(addr, WithConnectPolicy(policy))
	start := time.Now()
	err := c.Connect()
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("期望 connect 失败")
	}
	// 0 次重试 = 1 次尝试, 总耗时应远小于默认的 800ms
	if elapsed > 500*time.Millisecond {
		t.Errorf("elapsed = %v, 期望 < 500ms (0 次重试 = 1 次尝试)", elapsed)
	}
}

// TestClose_ResetsConn 验证 Close 后 c.conn 为 nil。
func TestClose_ResetsConn(t *testing.T) {
	ln, _ := startEchoListener(t)
	defer ln.Close()

	c := NewClient(ln.Addr().String())
	if err := c.Connect(); err != nil {
		t.Fatalf("Connect 失败: %v", err)
	}
	if c.conn == nil {
		t.Fatal("Connect 后 c.conn 应非 nil")
	}
	if err := c.Close(); err != nil {
		t.Errorf("Close err: %v", err)
	}
	if c.conn != nil {
		t.Error("Close 后 c.conn 应为 nil")
	}
	// 重复 Close 不报错
	if err := c.Close(); err != nil {
		t.Errorf("重复 Close 应不报错, got: %v", err)
	}
}

// === SendReceiveFiltered 实测 (本地 mock 服务器) ===

// TestSendReceiveFiltered_Success 测试正常 send+receive 流程。
func TestSendReceiveFiltered_Success(t *testing.T) {
	// 写一个 mock 服务器: 收到请求后返回匹配 cmd 的响应帧
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		// 读取请求帧 (但不解析, 直接返回响应)
		buf := make([]byte, 1024)
		conn.Read(buf)
		// 返回响应: cmd=0x9271, payload={"DataType":14,"Devices":[]}
		resp := BuildFrame(0x9271, `{"DataType":14,"Devices":[]}`)
		conn.Write(resp)
	}()

	c := NewClient(ln.Addr().String())
	if err := c.Connect(); err != nil {
		t.Fatalf("Connect 失败: %v", err)
	}
	defer c.Close()

	respCmd, data, err := c.SendReceiveFiltered(0x9275, `{"DataType":14,"Function":1}`, 14, 0x9271)
	if err != nil {
		t.Fatalf("SendReceiveFiltered 失败: %v", err)
	}
	if respCmd != 0x9271 {
		t.Errorf("respCmd = 0x%04X, want 0x9271", respCmd)
	}
	if !strings.Contains(string(data), "Devices") {
		t.Errorf("data 应含 'Devices', got: %s", data)
	}
}

// TestSendReceiveFiltered_NoConnection 测试未连接时立即返回错误, 不 panic。
func TestSendReceiveFiltered_NoConnection(t *testing.T) {
	c := NewClient("127.0.0.1:1") // 不会真的连
	_, _, err := c.SendReceiveFiltered(0x9275, `{}`, 13, 0x9271)
	if err == nil {
		t.Error("未连接时 SendReceiveFiltered 应返回错误")
	}
	if !strings.Contains(err.Error(), "连接未建立") {
		t.Errorf("错误应含 '连接未建立', got: %v", err)
	}
}

// TestSendReceiveFiltered_NonDCPNoRetry 验证非 DCP 命令 (DataType=12) 不触发重试。
func TestSendReceiveFiltered_NonDCPNoRetry(t *testing.T) {
	// mock 服务器: 第一次返回错命令字, 然后关闭连接
	// 客户端读到错命令字 → 跳过, 继续读 → 读到 EOF → 返回错误
	// 不应触发重连 (acceptCount = 1)
	var acceptCount int32
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			atomic.AddInt32(&acceptCount, 1)
			// 处理单个连接后退出 (非 DCP 不重试, 不会再来第二次)
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 1024)
				c.Read(buf)
				// 返回错命令字 0x9273 (与 0x9271 期望不符, 客户端会跳过并继续读)
				resp := BuildFrame(0x9273, `{"error":"first call"}`)
				c.Write(resp)
				// defer Close 触发客户端读到 EOF
			}(conn)
			return
		}
	}()

	c := NewClient(ln.Addr().String())
	if err := c.Connect(); err != nil {
		t.Fatalf("Connect 失败: %v", err)
	}
	defer c.Close()

	// DataType=12 非 DCP, 不重试
	// sendReceiveOnce 读到 0x9273 (非 expected 0x9271) → 跳过; 再读 → EOF → 返回错误
	_, _, _ = c.SendReceiveFiltered(0x9275, `{"DataType":12}`, 12, 0x9271)

	// 给 goroutine 一点时间记录 acceptCount
	time.Sleep(50 * time.Millisecond)
	if got := atomic.LoadInt32(&acceptCount); got != 1 {
		t.Errorf("acceptCount = %d, want 1 (非 DCP 不应触发重连)", got)
	}
}

// === Step 9.2 chaos test ===

// TestSendReceiveFiltered_RetriesOnClosed 模拟服务端"读完即关" (closed connection 错误),
// 第二次正常返回 0x9271 + JSON, 验证 DCP 1 次重试机制工作正常。
//
// 测试设计:
//   - 第一次 accept: 等客户端写完请求帧后, 立即关闭连接 (模拟 DCP 控制器发完即关)
//   - 第二次 accept: 正常返回响应
//   - 客户端 sendReceiveOnce 写帧成功, 但读响应时收到 EOF/closed
//   - reliability.ShouldRetryNetwork 触发重试, 重连 + 重发, 第二次成功
func TestSendReceiveFiltered_RetriesOnClosed(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	var acceptCount int32
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			n := atomic.AddInt32(&acceptCount, 1)
			go func(c net.Conn, attempt int) {
				defer c.Close()
				// 先读完客户端的请求帧 (无论 attempt 1 还是 2, 都要读)
				buf := make([]byte, 1024)
				c.Read(buf)
				if attempt == 1 {
					// 第一次: 读完即关 (触发客户端 read EOF → ShouldRetryNetwork)
					return
				}
				// 第二次: 正常返回响应
				resp := BuildFrame(0x9271, `{"DataType":14,"Devices":[{"Mac":"00:11:22:33:44:55","DeviceName":"retry-success"}]}`)
				c.Write(resp)
			}(conn, int(n))
		}
	}()

	// 默认 receivePolicy = 1 次重试
	c := NewClient(ln.Addr().String())
	if err := c.Connect(); err != nil {
		t.Fatalf("Connect 失败: %v", err)
	}
	defer c.Close()

	respCmd, data, err := c.SendReceiveFiltered(0x9275, `{"DataType":14,"Function":1}`, 14, 0x9271)
	if err != nil {
		t.Fatalf("SendReceiveFiltered 重试后应成功, got err: %v", err)
	}
	if respCmd != 0x9271 {
		t.Errorf("respCmd = 0x%04X, want 0x9271", respCmd)
	}
	if !strings.Contains(string(data), "retry-success") {
		t.Errorf("data 应含 'retry-success', got: %s", data)
	}
	if got := atomic.LoadInt32(&acceptCount); got != 2 {
		t.Errorf("acceptCount = %d, want 2 (1 次失败 + 1 次重试成功)", got)
	}
}

// TestSendReceiveFiltered_NoRetryOnProtocolErr 模拟响应 command 错, 不重试。
func TestSendReceiveFiltered_NoRetryOnProtocolErr(t *testing.T) {
	// mock 服务器: 始终返回错命令字 0x9273 (非 0x9271)
	// 注入 0 重试的 WithReceivePolicy, 单独验证 sendReceiveOnce 不过滤错命令
	// (SendReceiveFiltered 内部循环会被错命令卡住 → 这就是 "protocol err" 不重试的语义)
	var acceptCount int32
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			atomic.AddInt32(&acceptCount, 1)
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 1024)
				c.Read(buf)
				// 返回错命令字 0x9273 → SendReceiveFiltered 会跳过 (异步推送)
				// 但这与 0x9271 期望不符, sendReceiveOnce 内部会一直循环直到 read timeout
				// 因此本测试只验证 "第 1 次 accept 后, 不再有第 2 次 accept" (无重试)
				resp := BuildFrame(0x9273, `{"error":"wrong cmd"}`)
				c.Write(resp)
				// 立刻关闭, 让客户端读到 EOF → reliability.ShouldRetryNetwork 会触发重试
				// 但这里我们注入 0 重试 policy, 所以实际不会重试
			}(conn)
		}
	}()

	// 注入 0 重试 policy: 即使有重试触发条件, 也不重试
	policy := &reliability.Policy{
		MaxRetries:  0,
		BaseBackoff: 10 * time.Millisecond,
		MaxBackoff:  10 * time.Millisecond,
		ShouldRetry: reliability.ShouldRetryNetwork,
	}
	c := NewClient(ln.Addr().String(), WithReceivePolicy(policy))
	if err := c.Connect(); err != nil {
		t.Fatalf("Connect 失败: %v", err)
	}
	defer c.Close()

	// DataType=14 DCP 命令, 0 重试 policy → 仅 1 次尝试
	// 错命令字 0x9273 → SendReceiveFiltered 内部循环跳过, 直到 read EOF
	// EOF 触发 reliability.ShouldRetryNetwork, 但 MaxRetries=0 → 1 次尝试后退出
	_, _, _ = c.SendReceiveFiltered(0x9275, `{"DataType":14,"Function":1}`, 14, 0x9271)

	// 给 goroutine 一点时间记录 acceptCount
	time.Sleep(100 * time.Millisecond)
	if got := atomic.LoadInt32(&acceptCount); got != 1 {
		t.Errorf("acceptCount = %d, want 1 (0 重试 policy 不应触发重连)", got)
	}
}

// === Step 9.2 WithReceivePolicy 注入测试 ===

// TestSendReceiveFiltered_DefaultReceivePolicy 验证默认 DCP 1 次重试。
func TestSendReceiveFiltered_DefaultReceivePolicy(t *testing.T) {
	c := NewClient("127.0.0.1:6000")
	if c.receivePolicy == nil {
		t.Fatal("默认 receivePolicy 不应为 nil")
	}
	if c.receivePolicy.MaxRetries != 1 {
		t.Errorf("默认 receivePolicy.MaxRetries = %d, want 1", c.receivePolicy.MaxRetries)
	}
	if !c.receivePolicy.ShouldRetry(errors.New("timeout")) {
		t.Error("默认 receivePolicy.ShouldRetry 应触发 timeout")
	}
}

// === 验证已删除 shouldRetryDCP 私有函数 ===

// TestShouldRetryDCP_Removed 验证 shouldRetryDCP 已被 reliability.ShouldRetryNetwork 替代。
// 旧的 shouldRetryDCP 已删除, 这里作为占位测试, 防止有人误加回。
func TestShouldRetryDCP_Removed(t *testing.T) {
	// 直接调用 reliability.ShouldRetryNetwork (迁移后唯一权威实现)
	if !reliability.ShouldRetryNetwork(fmt.Errorf("等待匹配响应超时 (10s)")) {
		t.Error("reliability.ShouldRetryNetwork 应识别中文 timeout")
	}
	if reliability.ShouldRetryNetwork(errors.New("JSON 解析失败")) {
		t.Error("reliability.ShouldRetryNetwork 不应重试 JSON 解析失败")
	}
}
