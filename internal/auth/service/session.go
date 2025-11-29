package service

import (
	"sync"

	"github.com/google/uuid"
)

// ============================================================
// Session Manager
// ============================================================

type SessionManager struct {
	mu     sync.Mutex
	tokens map[string]string // token -> userID
}

func NewSessionManager() *SessionManager {
	return &SessionManager{
		tokens: make(map[string]string),
	}
}

func (m *SessionManager) Issue(userID string) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	token := uuid.NewString()
	m.tokens[token] = userID
	return token
}

func (m *SessionManager) Resolve(token string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	userID, ok := m.tokens[token]
	return userID, ok
}
