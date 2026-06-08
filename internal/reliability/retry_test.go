package reliability

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// === Policy.Do 行为 ===

func TestPolicy_DoSucceedsFirstTry(t *testing.T) {
	p := &Policy{MaxRetries: 3, BaseBackoff: time.Millisecond, MaxBackoff: 10 * time.Millisecond}
	calls := 0
	err := p.Do(context.Background(), func() error {
		calls++
		return nil
	})
	if err != nil {
		t.Errorf("期望 nil, got %v", err)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

func TestPolicy_DoRetriesUntilSuccess(t *testing.T) {
	p := &Policy{
		MaxRetries:  3,
		BaseBackoff: time.Millisecond,
		MaxBackoff:  10 * time.Millisecond,
		ShouldRetry: func(err error) bool { return true },
	}
	calls := 0
	err := p.Do(context.Background(), func() error {
		calls++
		if calls < 3 {
			return errors.New("timeout")
		}
		return nil
	})
	if err != nil {
		t.Errorf("期望 nil, got %v", err)
	}
	if calls != 3 {
		t.Errorf("calls = %d, want 3 (前 2 次失败, 第 3 次成功)", calls)
	}
}

func TestPolicy_DoGivesUpAfterMaxRetries(t *testing.T) {
	p := &Policy{
		MaxRetries:  2,
		BaseBackoff: time.Millisecond,
		MaxBackoff:  10 * time.Millisecond,
		ShouldRetry: func(err error) bool { return true },
	}
	calls := 0
	err := p.Do(context.Background(), func() error {
		calls++
		return errors.New("timeout")
	})
	if err == nil {
		t.Error("期望 error, got nil")
	}
	if calls != 3 {
		t.Errorf("calls = %d, want 3 (1 + 2 retries)", calls)
	}
}

func TestPolicy_DoRespectsShouldRetry(t *testing.T) {
	p := &Policy{
		MaxRetries:  3,
		BaseBackoff: time.Millisecond,
		MaxBackoff:  10 * time.Millisecond,
		ShouldRetry: func(err error) bool { return strings.Contains(err.Error(), "timeout") },
	}
	calls := 0
	err := p.Do(context.Background(), func() error {
		calls++
		return errors.New("protocol error")
	})
	if err == nil {
		t.Error("期望 error, got nil")
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1 (ShouldRetry 返回 false, 不重试)", calls)
	}
}

func TestPolicy_ExponentialBackoff(t *testing.T) {
	// 验证 backoff 翻倍: 1ms → 2ms → 4ms (含抖动前)
	// 实测: 总耗时 > 7ms (1+2+4), < 50ms (50% 抖动 + 调度)
	p := &Policy{
		MaxRetries:  3,
		BaseBackoff: time.Millisecond,
		MaxBackoff:  100 * time.Millisecond,
		Jitter:      func() time.Duration { return 0 }, // 关抖动便于测时
		ShouldRetry: func(err error) bool { return true },
	}
	start := time.Now()
	_ = p.Do(context.Background(), func() error { return errors.New("x") })
	elapsed := time.Since(start)
	if elapsed < 7*time.Millisecond {
		t.Errorf("elapsed = %v, 期望 >= 7ms (3 次退避至少 1+2+4=7ms)", elapsed)
	}
}

func TestPolicy_MaxBackoffCaps(t *testing.T) {
	// 验证 backoff 不会无限翻倍, 会被 MaxBackoff 截断
	// 设 BaseBackoff=10ms, MaxBackoff=15ms, 翻倍应停在 15ms
	p := &Policy{
		MaxRetries:  4,
		BaseBackoff: 10 * time.Millisecond,
		MaxBackoff:  15 * time.Millisecond,
		Jitter:      func() time.Duration { return 0 },
		ShouldRetry: func(err error) bool { return true },
	}
	start := time.Now()
	_ = p.Do(context.Background(), func() error { return errors.New("x") })
	elapsed := time.Since(start)
	// 4 次退避: 10 + 15 + 15 + 15 = 55ms (第 1 次是 10, 后续被截到 15)
	if elapsed < 50*time.Millisecond {
		t.Errorf("elapsed = %v, 期望 >= 50ms (4 次退避)", elapsed)
	}
	if elapsed > 200*time.Millisecond {
		t.Errorf("elapsed = %v, 期望 < 200ms (截断应生效)", elapsed)
	}
}

func TestPolicy_ContextCancel(t *testing.T) {
	p := &Policy{
		MaxRetries:  10,
		BaseBackoff: 100 * time.Millisecond,
		MaxBackoff:  1 * time.Second,
		ShouldRetry: func(err error) bool { return true },
	}
	ctx, cancel := context.WithCancel(context.Background())
	var calls int32
	go func() {
		// 50ms 后取消
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	err := p.Do(ctx, func() error {
		atomic.AddInt32(&calls, 1)
		return errors.New("timeout")
	})
	if err == nil {
		t.Error("期望 ctx.Err(), got nil")
	}
	if !strings.Contains(err.Error(), "context") {
		t.Errorf("错误应含 'context', got: %v", err)
	}
}

func TestPolicy_NilPolicy(t *testing.T) {
	// nil policy 应不 panic, 直接执行 1 次
	var p *Policy
	calls := 0
	err := p.Do(context.Background(), func() error {
		calls++
		return errors.New("boom")
	})
	if err == nil {
		t.Error("期望 error, got nil")
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1 (nil policy 不重试)", calls)
	}
}

func TestDefault_Policy(t *testing.T) {
	p := Default()
	if p.MaxRetries != 3 {
		t.Errorf("Default().MaxRetries = %d, want 3", p.MaxRetries)
	}
	if p.BaseBackoff != 300*time.Millisecond {
		t.Errorf("Default().BaseBackoff = %v, want 300ms", p.BaseBackoff)
	}
	if p.MaxBackoff != 3*time.Second {
		t.Errorf("Default().MaxBackoff = %v, want 3s", p.MaxBackoff)
	}
	if p.ShouldRetry == nil {
		t.Error("Default().ShouldRetry 不应为 nil")
	}
}

// === ShouldRetryNetwork 关键词覆盖 ===

func TestShouldRetryNetwork(t *testing.T) {
	tests := []struct {
		err       error
		wantRetry bool
	}{
		{errors.New("read timeout"), true},
		{errors.New("i/o timeout"), true},
		{errors.New("等待匹配响应超时 (10s)"), true},
		{errors.New("use of closed network connection"), true},
		{errors.New("wsarecv: An existing connection was forcibly closed by the remote host"), true},
		{errors.New("read tcp 127.0.0.1:1234->127.0.0.1:5678: read: connection closed by peer"), true},
		{errors.New("EOF"), true},
		{errors.New("unexpected EOF"), true},
		{errors.New("invalid JSON"), false},
		{errors.New("unexpected response command"), false},
		{errors.New("连接未建立"), false},
		{errors.New("write frame: bad file descriptor"), false},
		{nil, false},
	}
	for _, tc := range tests {
		got := ShouldRetryNetwork(tc.err)
		if got != tc.wantRetry {
			t.Errorf("ShouldRetryNetwork(%q) = %v, want %v", tc.err, got, tc.wantRetry)
		}
	}
}
