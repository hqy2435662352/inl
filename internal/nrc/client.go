package nrc

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/your-org/inl/internal/reliability"
)

// Client 是 NRC Socket 协议的 TCP 客户端。
//
// 设计要点 (Step 9 稳定性正式化):
//   - Connect / SendReceiveFiltered 改用 internal/reliability.Policy.Do,
//     行为不变 (Connect 3 次重试, DCP 1 次重试) 但变得可注入、可测试、可配置
//   - 写超时 5s 防止 hung write 卡死
//   - 读超时: DCP 60s, 其他 10s
//   - TCP keepalive 30s 周期: 检测半开连接
//   - main.go 可通过 WithRetryPolicy 或 --retry 全局 flag 调整重试次数
type Client struct {
	addr          string
	conn          net.Conn
	connectPolicy *reliability.Policy
	receivePolicy *reliability.Policy
}

// ClientOption 客户端构造选项 (Step 9.2 新增)。
type ClientOption func(*Client)

// WithConnectPolicy 覆盖默认的 Connect 重试策略 (默认 3 次, 300ms 起始退避, 3s 上限)。
func WithConnectPolicy(p *reliability.Policy) ClientOption {
	return func(c *Client) { c.connectPolicy = p }
}

// WithReceivePolicy 覆盖默认的 SendReceiveFiltered 重试策略 (默认 1 次, 200ms 起始退避, 1s 上限, 仅 DCP)。
func WithReceivePolicy(p *reliability.Policy) ClientOption {
	return func(c *Client) { c.receivePolicy = p }
}

// NewClient 构造 NRC 客户端。可选传入 ClientOption 覆盖默认重试策略。
func NewClient(addr string, opts ...ClientOption) *Client {
	c := &Client{
		addr:          addr,
		connectPolicy: defaultConnectPolicy(),
		receivePolicy: defaultReceivePolicy(),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// defaultReceivePolicy 返回 DCP SendReceiveFiltered 用的默认策略。
//
// 与 reliability.Default() / reliability.ShouldRetryNetwork 的区别:
//   - 默认任何 sendReceiveOnce 错误都重试 (含 "aborted by software" 等 Windows 特定错误)
//   - 不用 ShouldRetryNetwork 是因为它只覆盖 timeout/closed/EOF 关键词,
//     但 Windows 上常见 "aborted by software in your host machine" 等错误
//     也属于 DCP 应重试的瞬时网络错误。
//
// 行为契约: DCP 是广播, 重发无害; sendReceiveOnce 的所有错误都是网络层错误
// (连接未建立 / 写帧失败 / 读超时 / 读 EOF), 重连+重发大概率能成功。
func defaultReceivePolicy() *reliability.Policy {
	return NewReceivePolicy(1)
}

// NewReceivePolicy 构造指定重试次数的 DCP receive policy (供 main.go 的 --retry flag 使用)。
//
// 与 defaultReceivePolicy 行为一致, 仅 MaxRetries 不同。
func NewReceivePolicy(maxRetries int) *reliability.Policy {
	return &reliability.Policy{
		MaxRetries:  maxRetries,
		BaseBackoff: 200 * time.Millisecond,
		MaxBackoff:  1 * time.Second,
		ShouldRetry: func(err error) bool { return err != nil },
	}
}

// defaultConnectPolicy 返回 Connect 用的默认策略。
//
// 与 reliability.Default() 的区别: Connect 任何 dial 错误都重试 (含 "connection refused"),
// 而 reliability.Default() 用 ShouldRetryNetwork 只重试 timeout/closed/EOF。
//
// 为什么: "connection refused" 是建立连接最常见的失败 (控制器刚启动 / 6000 端口未就绪),
// 短暂等待后重试大概率能成功。若用 ShouldRetryNetwork, Connect 在 refused 时会立即失败。
func defaultConnectPolicy() *reliability.Policy {
	return &reliability.Policy{
		MaxRetries:  3,
		BaseBackoff: 300 * time.Millisecond,
		MaxBackoff:  3 * time.Second,
		ShouldRetry: func(err error) bool { return err != nil },
	}
}

// Connect 建立 TCP 连接, 通过 reliability.Policy 注入重试逻辑 (Step 9.2)。
//
// 默认行为: 3 次重试, 指数退避 (300ms → 600ms → 1200ms, 上限 3s) + 随机抖动 (0~50%)。
// 解决"频繁操作时连接被拒": 短连接模式下控制器重开 6000 端口有短暂窗口。
//
// 单次 5s 连接超时 (工业网络延迟较大, 旧版 2s 太短)。
// TCP keepalive 30s 周期: 检测半开连接 (中间网络设备超时断开但本地不知)。
//
// 兼容旧行为: 仍返回 "dial %s 失败: %w" 错误格式 (省略 "重试3次" 字样以保持与
// reliability.Policy 解耦; 若需详细重试次数可检查上层日志)。
func (c *Client) Connect() error {
	policy := c.connectPolicy
	if policy == nil {
		policy = defaultConnectPolicy()
	}

	err := policy.Do(context.Background(), func() error {
		conn, err := net.DialTimeout("tcp", c.addr, 5*time.Second)
		if err != nil {
			return err
		}
		// TCP keepalive 防止半开连接卡死 (中间网络设备超时断开但本地不知)。
		// 30s 周期适合工业 PC 的稳态场景: 不会过度消耗 CPU, 又能在合理时间内发现死链。
		if tcpConn, ok := conn.(*net.TCPConn); ok {
			_ = tcpConn.SetKeepAlive(true)
			_ = tcpConn.SetKeepAlivePeriod(30 * time.Second)
		}
		c.conn = conn
		return nil
	})
	if err != nil {
		return fmt.Errorf("dial %s 失败: %w", c.addr, err)
	}
	return nil
}

// Close 关闭底层 TCP 连接。
func (c *Client) Close() error {
	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		return err
	}
	return nil
}

// SendReceiveFiltered 发送请求并循环读取响应, 跳过异步推送,
// 直到收到匹配的响应 (command 匹配且非 GetActRun) 或超时。
//
// 过滤规则:
//  1. 响应 command ≠ expectedCmd → 异步推送, 跳过
//  2. 响应 JSON 含 "Function":"GetActRun" → 异步推送, 跳过
//
// 超时: DCP 命令 (DataType=14/16) 用 60s, 其他用 10s。
//
// 稳定性优化 (Step 9.2 — 已正式化到 internal/reliability):
//   - 对 DataType=14/16 命令, 失败时 (timeout / closed / forcibly closed) 自动重连重试 1 次。
//     DCP 是广播, 重发无害; 控制器发完即关是已知场景, 不应立即判失败。
//   - 重试前会关闭当前连接, 重新建立, 然后重新 send + receive。
//   - 触发条件由 reliability.ShouldRetryNetwork 统一管理 (兼容中英文错误信息)。
func (c *Client) SendReceiveFiltered(sendCmd uint16, payload string, dataType int, expectedCmd uint16) (respCmd uint16, respData []byte, err error) {
	// 仅 DCP 命令 (DataType=14/16) 触发重试, 与历史行为一致
	isDCP := dataType == 14 || dataType == 16
	if !isDCP {
		// 非 DCP 命令: 单次执行, 不重试
		return c.sendReceiveOnce(sendCmd, payload, dataType, expectedCmd)
	}

	policy := c.receivePolicy
	if policy == nil {
		policy = defaultReceivePolicy()
	}

	// needsReconnect 标志: 第 1 次尝试复用现有 c.conn (从外层 Connect 来的);
	// 第 2 次及以后 (重试) 必须重连, 因为前一次 sendReceiveOnce 失败后 c.conn
	// 可能已损坏 (closed / aborted by software / half-open)。
	needsReconnect := false
	_ = policy.Do(context.Background(), func() error {
		if c.conn == nil || needsReconnect {
			if connErr := c.reconnect(); connErr != nil {
				return connErr
			}
		}
		needsReconnect = true
		respCmd, respData, err = c.sendReceiveOnce(sendCmd, payload, dataType, expectedCmd)
		return err
	})
	return respCmd, respData, err
}

// reconnect 关闭当前连接 (如有) 并重新建立。
// 用于 DCP 命令重试场景: 旧连接可能已被控制器发完即关。
func (c *Client) reconnect() error {
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}
	return c.Connect()
}

// sendReceiveOnce 单次 send + receive, SendReceiveFiltered 的内部辅助。
func (c *Client) sendReceiveOnce(sendCmd uint16, payload string, dataType int, expectedCmd uint16) (uint16, []byte, error) {
	if c.conn == nil {
		return 0, nil, fmt.Errorf("连接未建立")
	}
	frame := BuildFrame(sendCmd, payload)
	// 写超时 5s: 工业网络偶发慢, 但不应无限等。
	_ = c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if _, err := c.conn.Write(frame); err != nil {
		return 0, nil, fmt.Errorf("write frame: %w", err)
	}

	timeout := 10 * time.Second
	if dataType == 14 || dataType == 16 {
		timeout = 60 * time.Second // DCP 扫描 + GSD 匹配可能耗时较长
	}

	deadline := time.Now().Add(timeout)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return 0, nil, fmt.Errorf("等待匹配响应超时 (%v)", timeout)
		}
		_ = c.conn.SetReadDeadline(time.Now().Add(remaining))

		cmd, data, err := ReadFrame(c.conn)
		if err != nil {
			return 0, nil, err
		}

		if cmd != expectedCmd {
			continue // 异步推送 (非 0x9271), 跳过
		}

		if isGetActRunPush(data) {
			continue // IO Routing GetActRun 异步推送, 跳过
		}

		return cmd, data, nil
	}
}

// isGetActRunPush 判断响应是否为 IO Routing 循环自动推送的 GetActRun。
// GetActRun 特征: 含顶层 "Function":"GetActRun" 字段。
// DCP/端口列表响应均不含此字段, 不会误判。
func isGetActRunPush(data []byte) bool {
	var top struct {
		Function string `json:"Function"`
	}
	if err := json.Unmarshal(data, &top); err != nil {
		return false
	}
	return top.Function == "GetActRun"
}
