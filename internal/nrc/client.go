package nrc

import (
	"encoding/json"
	"fmt"
	"net"
	"time"
)

type Client struct {
	addr string
	conn net.Conn
}

func NewClient(addr string) *Client {
	return &Client{addr: addr}
}

func (c *Client) Connect() error {
	var lastErr error
	for i := 0; i < 3; i++ {
		conn, err := net.DialTimeout("tcp", c.addr, 2*time.Second)
		if err == nil {
			c.conn = conn
			return nil
		}
		lastErr = err
		if i < 2 {
			time.Sleep(300 * time.Millisecond)
		}
	}
	return fmt.Errorf("dial %s 失败 (重试3次): %w", c.addr, lastErr)
}

func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// SendReceiveFiltered 发送请求并循环读取响应, 跳过异步推送,
// 直到收到匹配的响应 (command 匹配且非 GetActRun) 或超时。
//
// 过滤规则:
//   1. 响应 command ≠ expectedCmd → 异步推送, 跳过
//   2. 响应 JSON 含 "Function":"GetActRun" → 异步推送, 跳过
// 超时: DCP 命令 (DataType=14/16) 用 30s, 其他用 10s。
func (c *Client) SendReceiveFiltered(sendCmd uint16, payload string, dataType int, expectedCmd uint16) (respCmd uint16, respData []byte, err error) {
	frame := BuildFrame(sendCmd, payload)
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
		c.conn.SetReadDeadline(time.Now().Add(remaining))

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
