package web

import (
	"sync/atomic"
	"time"

	"github.com/Duke1616/ecmdb/pkg/term"
)

type runtimeSessionStore struct {
	pool *term.SessionPool
	seq  atomic.Int64
}

func newRuntimeSessionStore() *runtimeSessionStore {
	store := &runtimeSessionStore{
		pool: term.NewSessionPool(),
	}
	store.seq.Store(time.Now().UnixNano())
	return store
}

func (s *runtimeSessionStore) Put(session term.Session) int64 {
	sessionID := s.seq.Add(1)
	s.pool.SetSession(sessionID, session)
	return sessionID
}

func (s *runtimeSessionStore) Get(sessionID int64) (term.Session, error) {
	return s.pool.GetSession(sessionID)
}

func (s *runtimeSessionStore) Close(sessionID int64) {
	if session, err := s.pool.GetSession(sessionID); err == nil && session != nil {
		_ = session.Close()
	}
	s.pool.DeleteSession(sessionID)
}
