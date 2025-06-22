package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// InMemorySessionManager provides an in-memory session store
// Following MCP guidelines: sessions are for application state, NOT authentication
type InMemorySessionManager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
	
	// Session configuration
	defaultTTL time.Duration
	maxSessions int
	
	// Cleanup ticker
	cleanupTicker *time.Ticker
	stopChan      chan struct{}
}

// NewInMemorySessionManager creates a new in-memory session manager
func NewInMemorySessionManager(config SessionManagerConfig) *InMemorySessionManager {
	if config.DefaultTTL == 0 {
		config.DefaultTTL = 24 * time.Hour // Default 24 hours
	}
	if config.MaxSessions == 0 {
		config.MaxSessions = 10000 // Default max sessions
	}
	if config.CleanupInterval == 0 {
		config.CleanupInterval = 1 * time.Hour // Default cleanup every hour
	}
	
	sm := &InMemorySessionManager{
		sessions:    make(map[string]*Session),
		defaultTTL:  config.DefaultTTL,
		maxSessions: config.MaxSessions,
		stopChan:    make(chan struct{}),
	}
	
	// Start cleanup routine
	sm.cleanupTicker = time.NewTicker(config.CleanupInterval)
	go sm.cleanupRoutine()
	
	return sm
}

// SessionManagerConfig configures the session manager
type SessionManagerConfig struct {
	DefaultTTL      time.Duration // Default session TTL
	MaxSessions     int           // Maximum number of concurrent sessions
	CleanupInterval time.Duration // How often to cleanup expired sessions
}

// CreateSession creates a new session for the authenticated user
func (sm *InMemorySessionManager) CreateSession(ctx context.Context, user *User) (*Session, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	// Check session limit
	if len(sm.sessions) >= sm.maxSessions {
		// Try to clean up expired sessions first
		sm.cleanupExpiredSessionsLocked()
		
		if len(sm.sessions) >= sm.maxSessions {
			return nil, NewAuthError("session_limit_exceeded", "Maximum number of sessions reached", "")
		}
	}
	
	// Generate secure session ID following MCP guidelines
	sessionID, err := sm.generateSecureSessionID(user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate session ID: %w", err)
	}
	
	now := time.Now()
	session := &Session{
		ID:        sessionID,
		UserID:    user.ID, // Bound to user information per MCP guidelines
		Data:      make(map[string]interface{}),
		CreatedAt: now,
		UpdatedAt: now,
		ExpiresAt: now.Add(sm.defaultTTL),
	}
	
	sm.sessions[sessionID] = session
	
	return session, nil
}

// GetSession retrieves a session by ID
func (sm *InMemorySessionManager) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	session, exists := sm.sessions[sessionID]
	if !exists {
		return nil, NewAuthError(ErrCodeTokenNotFound, "Session not found", sessionID)
	}
	
	if session.IsExpired() {
		return nil, NewAuthError(ErrCodeSessionExpired, "Session has expired", sessionID)
	}
	
	// Return a copy to prevent external modification
	sessionCopy := *session
	return &sessionCopy, nil
}

// UpdateSession updates session data
func (sm *InMemorySessionManager) UpdateSession(ctx context.Context, session *Session) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	existingSession, exists := sm.sessions[session.ID]
	if !exists {
		return NewAuthError(ErrCodeTokenNotFound, "Session not found", session.ID)
	}
	
	if existingSession.IsExpired() {
		delete(sm.sessions, session.ID)
		return NewAuthError(ErrCodeSessionExpired, "Session has expired", session.ID)
	}
	
	// Update the session
	session.UpdatedAt = time.Now()
	sm.sessions[session.ID] = session
	
	return nil
}

// DeleteSession removes a session
func (sm *InMemorySessionManager) DeleteSession(ctx context.Context, sessionID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	delete(sm.sessions, sessionID)
	return nil
}

// CleanupExpiredSessions removes expired sessions
func (sm *InMemorySessionManager) CleanupExpiredSessions(ctx context.Context) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	sm.cleanupExpiredSessionsLocked()
	return nil
}

// cleanupExpiredSessionsLocked removes expired sessions (caller must hold write lock)
func (sm *InMemorySessionManager) cleanupExpiredSessionsLocked() {
	now := time.Now()
	for sessionID, session := range sm.sessions {
		if now.After(session.ExpiresAt) {
			delete(sm.sessions, sessionID)
		}
	}
}

// generateSecureSessionID generates a secure, non-deterministic session ID
// Following MCP guidelines: use secure random number generators and bind to user info
func (sm *InMemorySessionManager) generateSecureSessionID(userID string) (string, error) {
	// Generate 32 random bytes
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	
	// Create session ID with format: <user_id>:<random_hex>
	// This ensures that even if an attacker guesses a session ID,
	// they cannot impersonate another user (per MCP guidelines)
	randomHex := hex.EncodeToString(randomBytes)
	sessionID := fmt.Sprintf("%s:%s", userID, randomHex)
	
	return sessionID, nil
}

// cleanupRoutine runs periodic cleanup of expired sessions
func (sm *InMemorySessionManager) cleanupRoutine() {
	for {
		select {
		case <-sm.cleanupTicker.C:
			sm.CleanupExpiredSessions(context.Background())
		case <-sm.stopChan:
			return
		}
	}
}

// Close stops the session manager and cleanup routine
func (sm *InMemorySessionManager) Close() error {
	if sm.cleanupTicker != nil {
		sm.cleanupTicker.Stop()
	}
	
	close(sm.stopChan)
	
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.sessions = nil
	
	return nil
}

// GetStats returns session statistics
func (sm *InMemorySessionManager) GetStats() SessionStats {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	stats := SessionStats{
		TotalSessions: len(sm.sessions),
	}
	
	now := time.Now()
	for _, session := range sm.sessions {
		if now.After(session.ExpiresAt) {
			stats.ExpiredSessions++
		} else {
			stats.ActiveSessions++
		}
	}
	
	return stats
}

// SessionStats contains session statistics
type SessionStats struct {
	TotalSessions   int `json:"total_sessions"`
	ActiveSessions  int `json:"active_sessions"`
	ExpiredSessions int `json:"expired_sessions"`
}

// ExtendSession extends the expiration time of a session
func (sm *InMemorySessionManager) ExtendSession(ctx context.Context, sessionID string, duration time.Duration) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	session, exists := sm.sessions[sessionID]
	if !exists {
		return NewAuthError(ErrCodeTokenNotFound, "Session not found", sessionID)
	}
	
	if session.IsExpired() {
		delete(sm.sessions, sessionID)
		return NewAuthError(ErrCodeSessionExpired, "Session has expired", sessionID)
	}
	
	session.ExpiresAt = time.Now().Add(duration)
	session.UpdatedAt = time.Now()
	
	return nil
}

// GetSessionsByUser returns all active sessions for a user
func (sm *InMemorySessionManager) GetSessionsByUser(ctx context.Context, userID string) ([]*Session, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	var userSessions []*Session
	for _, session := range sm.sessions {
		if session.UserID == userID && !session.IsExpired() {
			sessionCopy := *session
			userSessions = append(userSessions, &sessionCopy)
		}
	}
	
	return userSessions, nil
}

// DeleteSessionsByUser deletes all sessions for a specific user
func (sm *InMemorySessionManager) DeleteSessionsByUser(ctx context.Context, userID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	for sessionID, session := range sm.sessions {
		if session.UserID == userID {
			delete(sm.sessions, sessionID)
		}
	}
	
	return nil
}