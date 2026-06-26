package agenthub

import (
	"context"
	"testing"

	"github.com/redis/go-redis/v9"
)

type mockConn struct {
	closed bool
}

func (m *mockConn) WriteJSON(v any) error { return nil }
func (m *mockConn) Close() error {
	m.closed = true
	return nil
}

func TestHubConnsPointerComparison(t *testing.T) {
	h := &Hub{
		conns: make(map[string]*entry),
	}

	ctx := context.Background()

	// 1. 注册连接 1 (新客户端建联)
	conn1 := &mockConn{}
	e1, err := h.Register(ctx, "server-1", conn1)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// 检查 conns 里是否存入了 e1
	if h.conns["server-1"] != e1 {
		t.Errorf("expected e1 to be registered in conns")
	}

	// 2. 模拟重连：注册连接 2 (新客户端再次建联，旧连接 1 尚未执行 Unregister，但会被 Register 触发 Close)
	conn2 := &mockConn{}
	e2, err := h.Register(ctx, "server-1", conn2)
	if err != nil {
		t.Fatalf("Register 2 failed: %v", err)
	}

	// 此时连接 1 应该被关闭了
	if !conn1.closed {
		t.Errorf("expected conn1 to be closed after conn2 registered")
	}

	// 此时 conns 中记录的应该是 e2
	if h.conns["server-1"] != e2 {
		t.Errorf("expected e2 to be in conns now")
	}

	// 3. 此时，旧连接 1 的 goroutine defer 被触发，调用 Unregister(ctx, "server-1", e1)
	h.Unregister(ctx, "server-1", e1)

	// 核心测试点：由于 e1 != e2，Unregister(e1) 不应该删除 conns 里的 e2
	if h.conns["server-1"] == nil {
		t.Errorf("expected e2 to remain in conns, but it was deleted by obsolete e1's Unregister")
	}
	if h.conns["server-1"] != e2 {
		t.Errorf("expected e2 to remain key connection")
	}

	// 4. 最后，连接 2 挂掉，调用 Unregister(ctx, "server-1", e2)
	h.Unregister(ctx, "server-1", e2)

	// 此时连接 2 对应的 e2 应该被删除了
	if h.conns["server-1"] != nil {
		t.Errorf("expected connection to be unregistered")
	}
}

func TestHubRegisterFailureRollback(t *testing.T) {
	// 创建一个连接指向无效端口的 redis client，让其 Set 必定失败
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:1",
	})
	defer rdb.Close()

	h := &Hub{
		rds:   rdb,
		conns: make(map[string]*entry),
	}

	ctx := context.Background()
	conn := &mockConn{}

	// 注册连接，因为 redis 连接不上，应该返回 error
	e, err := h.Register(ctx, "server-err", conn)
	if err == nil {
		t.Fatalf("expected Register to fail due to unreachable redis, but got success")
	}
	if e != nil {
		t.Fatalf("expected entry to be nil on failure, got %v", e)
	}

	// 验证 conns 里已被成功删除，没有 entry 残留
	h.mu.RLock()
	entryInMap, ok := h.conns["server-err"]
	h.mu.RUnlock()

	if ok || entryInMap != nil {
		t.Errorf("expected entry to be cleaned up/deleted from conns map on failure, but it was found: %v", entryInMap)
	}
}
