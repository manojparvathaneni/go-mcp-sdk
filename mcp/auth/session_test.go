package auth

import (
	"context"
	"testing"
	"time"
)

func TestNewInMemorySessionManager(t *testing.T) {
	config := SessionManagerConfig{
		DefaultTTL:      2 * time.Hour,
		MaxSessions:     1000,
		CleanupInterval: 30 * time.Minute,
	}
	
	sm := NewInMemorySessionManager(config)
	defer sm.Close()
	
	if sm == nil {
		t.Fatal("Expected NewInMemorySessionManager to return non-nil manager")
	}
	
	if sm.defaultTTL != 2*time.Hour {
		t.Errorf("Expected default TTL 2h, got %v", sm.defaultTTL)
	}
	
	if sm.maxSessions != 1000 {
		t.Errorf("Expected max sessions 1000, got %d", sm.maxSessions)
	}
}

func TestNewInMemorySessionManager_Defaults(t *testing.T) {
	config := SessionManagerConfig{} // Use all defaults
	
	sm := NewInMemorySessionManager(config)
	defer sm.Close()
	
	if sm.defaultTTL != 24*time.Hour {
		t.Errorf("Expected default TTL 24h, got %v", sm.defaultTTL)
	}
	
	if sm.maxSessions != 10000 {
		t.Errorf("Expected default max sessions 10000, got %d", sm.maxSessions)
	}
}

func TestInMemorySessionManager_CreateSession(t *testing.T) {
	config := SessionManagerConfig{
		DefaultTTL:  1 * time.Hour,
		MaxSessions: 100,
	}
	
	sm := NewInMemorySessionManager(config)
	defer sm.Close()
	
	user := &User{
		ID:       "test-user",
		Username: "testuser",
	}
	
	session, err := sm.CreateSession(context.Background(), user)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	
	if session == nil {
		t.Fatal("Expected non-nil session")
	}
	
	if session.ID == "" {
		t.Error("Expected session to have an ID")
	}
	
	if session.UserID != user.ID {
		t.Errorf("Expected session user ID '%s', got '%s'", user.ID, session.UserID)
	}
	
	if session.Data == nil {
		t.Error("Expected session to have initialized data map")
	}
	
	if session.IsExpired() {
		t.Error("Expected newly created session not to be expired")
	}
	
	// Verify session ID format (should contain user ID per MCP guidelines)
	if len(session.ID) < len(user.ID) {
		t.Error("Expected session ID to contain user ID per MCP guidelines")
	}
}

func TestInMemorySessionManager_GetSession(t *testing.T) {
	config := SessionManagerConfig{DefaultTTL: 1 * time.Hour}
	sm := NewInMemorySessionManager(config)
	defer sm.Close()
	
	user := &User{ID: "test-user"}
	
	// Create a session
	originalSession, err := sm.CreateSession(context.Background(), user)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	
	// Retrieve the session
	retrievedSession, err := sm.GetSession(context.Background(), originalSession.ID)
	if err != nil {
		t.Fatalf("Failed to get session: %v", err)
	}
	
	if retrievedSession.ID != originalSession.ID {
		t.Errorf("Expected session ID '%s', got '%s'", originalSession.ID, retrievedSession.ID)
	}
	
	if retrievedSession.UserID != originalSession.UserID {
		t.Errorf("Expected user ID '%s', got '%s'", originalSession.UserID, retrievedSession.UserID)
	}
}

func TestInMemorySessionManager_GetSession_NotFound(t *testing.T) {
	config := SessionManagerConfig{DefaultTTL: 1 * time.Hour}
	sm := NewInMemorySessionManager(config)
	defer sm.Close()
	
	_, err := sm.GetSession(context.Background(), "nonexistent-session")
	if err == nil {
		t.Error("Expected error when getting nonexistent session")
	}
	
	authErr, ok := err.(*AuthError)
	if !ok {
		t.Errorf("Expected AuthError, got %T", err)
	} else if authErr.Code != ErrCodeTokenNotFound {
		t.Errorf("Expected error code %s, got %s", ErrCodeTokenNotFound, authErr.Code)
	}
}

func TestInMemorySessionManager_GetSession_Expired(t *testing.T) {
	config := SessionManagerConfig{DefaultTTL: 1 * time.Millisecond}
	sm := NewInMemorySessionManager(config)
	defer sm.Close()
	
	user := &User{ID: "test-user"}
	
	// Create a session with very short TTL
	session, err := sm.CreateSession(context.Background(), user)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	
	// Wait for expiration
	time.Sleep(2 * time.Millisecond)
	
	// Try to get expired session
	_, err = sm.GetSession(context.Background(), session.ID)
	if err == nil {
		t.Error("Expected error when getting expired session")
	}
	
	authErr, ok := err.(*AuthError)
	if !ok {
		t.Errorf("Expected AuthError, got %T", err)
	} else if authErr.Code != ErrCodeSessionExpired {
		t.Errorf("Expected error code %s, got %s", ErrCodeSessionExpired, authErr.Code)
	}
}

func TestInMemorySessionManager_UpdateSession(t *testing.T) {
	config := SessionManagerConfig{DefaultTTL: 1 * time.Hour}
	sm := NewInMemorySessionManager(config)
	defer sm.Close()
	
	user := &User{ID: "test-user"}
	
	// Create a session
	session, err := sm.CreateSession(context.Background(), user)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	
	// Update session data
	session.Data["key"] = "value"
	originalUpdateTime := session.UpdatedAt
	
	// Small delay to ensure UpdatedAt changes
	time.Sleep(1 * time.Millisecond)
	
	err = sm.UpdateSession(context.Background(), session)
	if err != nil {
		t.Fatalf("Failed to update session: %v", err)
	}
	
	// Retrieve updated session
	updatedSession, err := sm.GetSession(context.Background(), session.ID)
	if err != nil {
		t.Fatalf("Failed to get updated session: %v", err)
	}
	
	if updatedSession.Data["key"] != "value" {
		t.Error("Expected session data to be updated")
	}
	
	if !updatedSession.UpdatedAt.After(originalUpdateTime) {
		t.Error("Expected UpdatedAt to be updated")
	}
}

func TestInMemorySessionManager_DeleteSession(t *testing.T) {
	config := SessionManagerConfig{DefaultTTL: 1 * time.Hour}
	sm := NewInMemorySessionManager(config)
	defer sm.Close()
	
	user := &User{ID: "test-user"}
	
	// Create a session
	session, err := sm.CreateSession(context.Background(), user)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	
	// Verify session exists
	_, err = sm.GetSession(context.Background(), session.ID)
	if err != nil {
		t.Fatalf("Session should exist before deletion")
	}
	
	// Delete session
	err = sm.DeleteSession(context.Background(), session.ID)
	if err != nil {
		t.Fatalf("Failed to delete session: %v", err)
	}
	
	// Verify session is gone
	_, err = sm.GetSession(context.Background(), session.ID)
	if err == nil {
		t.Error("Expected session to be deleted")
	}
}

func TestInMemorySessionManager_CleanupExpiredSessions(t *testing.T) {
	config := SessionManagerConfig{DefaultTTL: 1 * time.Millisecond}
	sm := NewInMemorySessionManager(config)
	defer sm.Close()
	
	user := &User{ID: "test-user"}
	
	// Create a session that will expire quickly
	_, err := sm.CreateSession(context.Background(), user)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	
	// Wait for expiration
	time.Sleep(2 * time.Millisecond)
	
	// Manually trigger cleanup
	err = sm.CleanupExpiredSessions(context.Background())
	if err != nil {
		t.Fatalf("Failed to cleanup expired sessions: %v", err)
	}
	
	// Verify session was cleaned up
	stats := sm.GetStats()
	if stats.TotalSessions != 0 {
		t.Errorf("Expected 0 total sessions after cleanup, got %d", stats.TotalSessions)
	}
}

func TestInMemorySessionManager_ExtendSession(t *testing.T) {
	config := SessionManagerConfig{DefaultTTL: 1 * time.Hour}
	sm := NewInMemorySessionManager(config)
	defer sm.Close()
	
	user := &User{ID: "test-user"}
	
	// Create a session
	session, err := sm.CreateSession(context.Background(), user)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	
	originalExpiry := session.ExpiresAt
	
	// Extend session
	err = sm.ExtendSession(context.Background(), session.ID, 2*time.Hour)
	if err != nil {
		t.Fatalf("Failed to extend session: %v", err)
	}
	
	// Get updated session
	extendedSession, err := sm.GetSession(context.Background(), session.ID)
	if err != nil {
		t.Fatalf("Failed to get extended session: %v", err)
	}
	
	if !extendedSession.ExpiresAt.After(originalExpiry) {
		t.Error("Expected session expiry to be extended")
	}
}

func TestInMemorySessionManager_GetSessionsByUser(t *testing.T) {
	config := SessionManagerConfig{DefaultTTL: 1 * time.Hour}
	sm := NewInMemorySessionManager(config)
	defer sm.Close()
	
	user1 := &User{ID: "user1"}
	user2 := &User{ID: "user2"}
	
	// Create sessions for both users
	session1a, _ := sm.CreateSession(context.Background(), user1)
	session1b, _ := sm.CreateSession(context.Background(), user1)
	session2, _ := sm.CreateSession(context.Background(), user2)
	
	// Get sessions for user1
	user1Sessions, err := sm.GetSessionsByUser(context.Background(), user1.ID)
	if err != nil {
		t.Fatalf("Failed to get sessions by user: %v", err)
	}
	
	if len(user1Sessions) != 2 {
		t.Errorf("Expected 2 sessions for user1, got %d", len(user1Sessions))
	}
	
	// Verify session IDs
	sessionIDs := make(map[string]bool)
	for _, session := range user1Sessions {
		sessionIDs[session.ID] = true
	}
	
	if !sessionIDs[session1a.ID] || !sessionIDs[session1b.ID] {
		t.Error("Expected to get both sessions for user1")
	}
	
	// Get sessions for user2
	user2Sessions, err := sm.GetSessionsByUser(context.Background(), user2.ID)
	if err != nil {
		t.Fatalf("Failed to get sessions by user: %v", err)
	}
	
	if len(user2Sessions) != 1 {
		t.Errorf("Expected 1 session for user2, got %d", len(user2Sessions))
	}
	
	if user2Sessions[0].ID != session2.ID {
		t.Error("Expected to get correct session for user2")
	}
}

func TestInMemorySessionManager_DeleteSessionsByUser(t *testing.T) {
	config := SessionManagerConfig{DefaultTTL: 1 * time.Hour}
	sm := NewInMemorySessionManager(config)
	defer sm.Close()
	
	user1 := &User{ID: "user1"}
	user2 := &User{ID: "user2"}
	
	// Create sessions for both users
	sm.CreateSession(context.Background(), user1)
	sm.CreateSession(context.Background(), user1)
	session2, _ := sm.CreateSession(context.Background(), user2)
	
	// Delete all sessions for user1
	err := sm.DeleteSessionsByUser(context.Background(), user1.ID)
	if err != nil {
		t.Fatalf("Failed to delete sessions by user: %v", err)
	}
	
	// Verify user1 sessions are gone
	user1Sessions, _ := sm.GetSessionsByUser(context.Background(), user1.ID)
	if len(user1Sessions) != 0 {
		t.Errorf("Expected 0 sessions for user1 after deletion, got %d", len(user1Sessions))
	}
	
	// Verify user2 session still exists
	_, err = sm.GetSession(context.Background(), session2.ID)
	if err != nil {
		t.Error("Expected user2 session to still exist")
	}
}

func TestInMemorySessionManager_GetStats(t *testing.T) {
	config := SessionManagerConfig{DefaultTTL: 1 * time.Hour}
	sm := NewInMemorySessionManager(config)
	defer sm.Close()
	
	user := &User{ID: "test-user"}
	
	// Initially no sessions
	stats := sm.GetStats()
	if stats.TotalSessions != 0 || stats.ActiveSessions != 0 || stats.ExpiredSessions != 0 {
		t.Error("Expected empty stats initially")
	}
	
	// Create some sessions
	sm.CreateSession(context.Background(), user)
	sm.CreateSession(context.Background(), user)
	
	stats = sm.GetStats()
	if stats.TotalSessions != 2 {
		t.Errorf("Expected 2 total sessions, got %d", stats.TotalSessions)
	}
	
	if stats.ActiveSessions != 2 {
		t.Errorf("Expected 2 active sessions, got %d", stats.ActiveSessions)
	}
	
	if stats.ExpiredSessions != 0 {
		t.Errorf("Expected 0 expired sessions, got %d", stats.ExpiredSessions)
	}
}

func TestInMemorySessionManager_MaxSessions(t *testing.T) {
	config := SessionManagerConfig{
		DefaultTTL:  1 * time.Hour,
		MaxSessions: 2, // Very low limit for testing
	}
	sm := NewInMemorySessionManager(config)
	defer sm.Close()
	
	user := &User{ID: "test-user"}
	
	// Create sessions up to the limit
	session1, err := sm.CreateSession(context.Background(), user)
	if err != nil {
		t.Fatalf("Failed to create first session: %v", err)
	}
	
	session2, err := sm.CreateSession(context.Background(), user)
	if err != nil {
		t.Fatalf("Failed to create second session: %v", err)
	}
	
	// Third session should fail
	_, err = sm.CreateSession(context.Background(), user)
	if err == nil {
		t.Error("Expected session creation to fail when limit exceeded")
	}
	
	authErr, ok := err.(*AuthError)
	if !ok {
		t.Errorf("Expected AuthError, got %T", err)
	} else if authErr.Code != "session_limit_exceeded" {
		t.Errorf("Expected session_limit_exceeded error, got %s", authErr.Code)
	}
	
	// Delete one session and try again
	sm.DeleteSession(context.Background(), session1.ID)
	
	_, err = sm.CreateSession(context.Background(), user)
	if err != nil {
		t.Errorf("Expected session creation to succeed after deleting one: %v", err)
	}
	
	// Verify we still have 2 sessions
	stats := sm.GetStats()
	if stats.TotalSessions != 2 {
		t.Errorf("Expected 2 total sessions after replacement, got %d", stats.TotalSessions)
	}
	
	// Verify the original session2 still exists
	_, err = sm.GetSession(context.Background(), session2.ID)
	if err != nil {
		t.Error("Expected original session2 to still exist")
	}
}

func TestInMemorySessionManager_Close(t *testing.T) {
	config := SessionManagerConfig{DefaultTTL: 1 * time.Hour}
	sm := NewInMemorySessionManager(config)
	
	user := &User{ID: "test-user"}
	sm.CreateSession(context.Background(), user)
	
	// Close should not error
	err := sm.Close()
	if err != nil {
		t.Errorf("Close returned error: %v", err)
	}
	
	// Sessions should be cleared
	stats := sm.GetStats()
	if stats.TotalSessions != 0 {
		t.Error("Expected sessions to be cleared after close")
	}
}