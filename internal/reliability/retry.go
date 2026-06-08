// Package reliability — see doc.go for overview.
package reliability

import (
	"context"
	"math/rand"
	"strings"
	"time"
)

// Policy 定义重试策略。零值不可用, 必须显式设置 MaxRetries/BaseBackoff/MaxBackoff。
//
// 字段说明:
//   - MaxRetries  最大重试次数 (0 = 不重试, 1 = 重试 1 次, 共 2 次尝试)
//   - BaseBackoff 首次退避基数 (e.g. 300ms)
//   - MaxBackoff  退避上限 (e.g. 3s), 超过此值不再翻倍
//   - Jitter      抖动函数; nil = 退避的 0-50% 随机抖动
//   - ShouldRetry 触发条件; nil = 任意非 nil error 都重试
type Policy struct {
	MaxRetries  int
	BaseBackoff time.Duration
	MaxBackoff  time.Duration
	Jitter      func() time.Duration
	ShouldRetry func(err error) bool
}

// Default 返回 inl 默认策略: 3 次重试, 300ms 起始退避, 3s 上限, 仅在网络错误时重试。
func Default() *Policy {
	return &Policy{
		MaxRetries:  3,
		BaseBackoff: 300 * time.Millisecond,
		MaxBackoff:  3 * time.Second,
		ShouldRetry: ShouldRetryNetwork,
	}
}

// Do 执行 fn, 按策略重试。返回最后一次的 error (若全部失败)。
//
// 行为契约:
//   - fn 第一次成功: 立即返回 nil
//   - fn 失败且 ShouldRetry 返回 false: 立即返回该 error
//   - fn 失败且 ShouldRetry 返回 true: 退避后重试, 直到 MaxRetries 用尽
//   - ctx 取消: 立即返回 ctx.Err()
func (p *Policy) Do(ctx context.Context, fn func() error) error {
	if p == nil {
		// nil policy = 不重试, 直接执行 1 次
		return fn()
	}

	var lastErr error
	backoff := p.BaseBackoff
	for attempt := 0; attempt <= p.MaxRetries; attempt++ {
		if err := fn(); err == nil {
			return nil
		} else {
			lastErr = err
		}

		// 已达最大重试次数 → 退出
		if attempt >= p.MaxRetries {
			break
		}

		// ShouldRetry 拒绝 → 立即返回
		if p.ShouldRetry != nil && !p.ShouldRetry(lastErr) {
			break
		}

		// 退避 (含 jitter)
		sleep := backoff
		if p.Jitter != nil {
			sleep += p.Jitter()
		} else {
			sleep += time.Duration(rand.Int63n(int64(backoff / 2)))
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(sleep):
		}

		// 指数退避 (翻倍)
		backoff *= 2
		if backoff > p.MaxBackoff {
			backoff = p.MaxBackoff
		}
	}
	return lastErr
}

// ShouldRetryNetwork 与 inl 现有 shouldRetryDCP 行为一致, 用于 nrc/client 迁移。
//
// 触发重试的关键词 (兼容中英文, 工业 PC 错误信息双语混合):
//   - "timeout" / "超时"        — 读超时, 控制器忙 / DCP 扫描慢 / 网络丢包
//   - "closed" / "forcibly closed" — 控制器在 DCP 帧发完后主动关闭连接
//   - "EOF"                    — 远程半关
//
// 不重试的场景 (调用方应区分):
//   - JSON 解析错误 — 协议层问题, 重试也无效
//   - 业务错误 (unexpected_response_command) — 响应 command 不对
//   - 写帧失败 — 重连后再写即可
func ShouldRetryNetwork(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "超时") ||
		strings.Contains(msg, "closed") ||
		strings.Contains(msg, "forcibly closed") ||
		strings.Contains(msg, "EOF")
}
