package web

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/Duke1616/ecmdb/pkg/term"
)

type runtimeSession struct {
	token    string
	finderID int64
	session  term.Session
}

type runtimeSessionStore struct {
	mu       sync.RWMutex
	sessions map[string]runtimeSession
	seq      atomic.Int64
}

func newRuntimeSessionStore() *runtimeSessionStore {
	return &runtimeSessionStore{
		sessions: make(map[string]runtimeSession),
	}
}

func (s *runtimeSessionStore) Put(session term.Session) (runtimeSession, error) {
	for i := 0; i < 3; i++ {
		token, err := newSessionToken()
		if err != nil {
			return runtimeSession{}, err
		}

		s.mu.Lock()
		if _, exists := s.sessions[token]; exists {
			s.mu.Unlock()
			continue
		}

		runtime := runtimeSession{
			token:    token,
			finderID: s.seq.Add(1),
			session:  session,
		}
		s.sessions[token] = runtime
		s.mu.Unlock()
		return runtime, nil
	}

	return runtimeSession{}, fmt.Errorf("failed to allocate session token")
}

func (s *runtimeSessionStore) Get(token string) (runtimeSession, error) {
	if token == "" {
		return runtimeSession{}, fmt.Errorf("session token is required")
	}

	s.mu.RLock()
	runtime, ok := s.sessions[token]
	s.mu.RUnlock()
	if !ok {
		return runtimeSession{}, fmt.Errorf("session %s not found", token)
	}
	return runtime, nil
}

func (s *runtimeSessionStore) Close(token string) {
	s.mu.Lock()
	runtime, ok := s.sessions[token]
	if ok {
		delete(s.sessions, token)
	}
	s.mu.Unlock()

	if ok && runtime.session != nil {
		_ = runtime.session.Close()
	}
}

func newSessionToken() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return "ssh_" + hex.EncodeToString(raw[:]), nil
}
