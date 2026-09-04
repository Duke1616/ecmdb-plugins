package term

import (
	"fmt"
	"sync"
)

// SessionPool 提供并发安全的终端会话缓存池。
type SessionPool struct {
	sessions map[int64]Session
	mu       sync.RWMutex
}

// NewSessionPool 实例化一个会话池。
func NewSessionPool() *SessionPool {
	return &SessionPool{
		sessions: make(map[int64]Session),
	}
}

// GetSession 从池中读取会话（并发读安全）。
func (p *SessionPool) GetSession(id int64) (Session, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	session, exists := p.sessions[id]
	if !exists {
		return nil, fmt.Errorf("session %d not found", id)
	}

	return session, nil
}

// SetSession 向池中存放或更新会话（写安全）。
func (p *SessionPool) SetSession(id int64, session Session) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.sessions[id] = session
}

// DeleteSession 从池中注销并清除某个会话。
func (p *SessionPool) DeleteSession(id int64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.sessions, id)
}

// CloseSession 从池中移除并主动关闭会话（安全释放底层连接）。
func (p *SessionPool) CloseSession(id int64) error {
	p.mu.Lock()
	session, exists := p.sessions[id]
	if exists {
		delete(p.sessions, id)
	}
	p.mu.Unlock()

	if exists && session != nil {
		return session.Close()
	}
	return nil
}

// CloseAll 清理并关闭池中所有的活跃会话。
func (p *SessionPool) CloseAll() {
	p.mu.Lock()
	remaining := make([]Session, 0, len(p.sessions))
	for id, s := range p.sessions {
		remaining = append(remaining, s)
		delete(p.sessions, id)
	}
	p.mu.Unlock()

	for _, s := range remaining {
		if s != nil {
			_ = s.Close()
		}
	}
}

// Count 返回当前会话池中存活的会话总数
func (p *SessionPool) Count() int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return len(p.sessions)
}

// ListIDs 返回当前会话池中所有会话的 ID 列表（用于诊断与监控）
func (p *SessionPool) ListIDs() []int64 {
	p.mu.RLock()
	defer p.mu.RUnlock()

	ids := make([]int64, 0, len(p.sessions))
	for id := range p.sessions {
		ids = append(ids, id)
	}
	return ids
}
