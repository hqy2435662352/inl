// Package reliability 提供 inl 客户端的可靠性原语：可注入、可测试、可配置的重试策略。
//
// 动机 (Step 9 稳定性正式化):
//   - 重试逻辑原本耦合在 internal/nrc/client.go 的 SendReceiveFiltered 私有循环中,
//     既不可单测, 也无法通过 flag 配置。
//   - 抽包后, Connect 与 SendReceiveFiltered 改为调用 reliability.Policy.Do,
//     默认 1 次 (DCP) / 3 次 (Connect) 重试 + 指数退避 + 抖动。
//   - 触发条件统一由 ShouldRetryNetwork 提供 (timeout / closed / forcibly closed / EOF)。
//
// 用法:
//   - 默认策略:  err := reliability.Default().Do(ctx, fn)
//   - 自定义次数: policy := &reliability.Policy{MaxRetries: 5, BaseBackoff: 500*time.Millisecond, ...}
//   - 自定义触发: policy.ShouldRetry = func(err error) bool { ... }
//
// 测试覆盖 (retry_test.go):
//   - 首次成功 / 多次重试后成功 / 超过 MaxRetries / ShouldRetry 拒绝 / 指数退避耗时 / 触发条件关键词
package reliability
