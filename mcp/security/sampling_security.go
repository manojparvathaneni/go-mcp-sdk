package security

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/manojparvathaneni/go-mcp-sdk/mcp/types"
)

// ApprovalResult represents the result of a user approval request
type ApprovalResult int

const (
	ApprovalDenied ApprovalResult = iota
	ApprovalGranted
	ApprovalModified
	ApprovalTimeout
)

// ApprovalRequest contains the details of a sampling request that needs approval
type ApprovalRequest struct {
	RequestID        string                    `json:"requestId"`
	Messages         []types.SamplingMessage   `json:"messages"`
	ModelPreferences *types.ModelPreferences   `json:"modelPreferences,omitempty"`
	SystemPrompt     *string                  `json:"systemPrompt,omitempty"`
	Temperature      *float64                 `json:"temperature,omitempty"`
	MaxTokens        int                      `json:"maxTokens"`
	StopSequences    []string                 `json:"stopSequences,omitempty"`
	Metadata         map[string]interface{}   `json:"metadata,omitempty"`
	Timestamp        time.Time                `json:"timestamp"`
}

// ApprovalResponse contains the user's response to an approval request
type ApprovalResponse struct {
	Result           ApprovalResult            `json:"result"`
	ModifiedMessages []types.SamplingMessage   `json:"modifiedMessages,omitempty"`
	ModifiedPrompt   *string                  `json:"modifiedPrompt,omitempty"`
	Reason           string                   `json:"reason,omitempty"`
}

// ApprovalHandler is called when user approval is needed for a sampling request
type ApprovalHandler func(ctx context.Context, req ApprovalRequest) (ApprovalResponse, error)

// ContentValidator validates sampling request content for security
type ContentValidator func(ctx context.Context, req types.CreateMessageRequest) error

// AuditLogger logs sampling operations for audit trail
type AuditLogger interface {
	LogSamplingRequest(ctx context.Context, req types.CreateMessageRequest) error
	LogApprovalRequest(ctx context.Context, req ApprovalRequest, resp ApprovalResponse) error
	LogSamplingResponse(ctx context.Context, result types.CreateMessageResult) error
}

// RateLimiter controls the rate of sampling requests
type SamplingRateLimiter interface {
	AllowSampling(ctx context.Context, clientID string) (bool, error)
	GetRemainingQuota(ctx context.Context, clientID string) (int, error)
}

// SamplingSecurityConfig contains security configuration for sampling
type SamplingSecurityConfig struct {
	RequireUserApproval bool                 `json:"requireUserApproval"`
	ApprovalTimeout     time.Duration        `json:"approvalTimeout"`
	MaxTokensLimit      int                  `json:"maxTokensLimit"`
	BlockedPatterns     []string             `json:"blockedPatterns"`
	AllowedMimeTypes    []string             `json:"allowedMimeTypes"`
	EnableAuditLogging  bool                 `json:"enableAuditLogging"`
	EnableRateLimit     bool                 `json:"enableRateLimit"`
}

// SamplingSecurity provides security controls for sampling operations
type SamplingSecurity struct {
	config           SamplingSecurityConfig
	approvalHandler  ApprovalHandler
	contentValidator ContentValidator
	auditLogger      AuditLogger
	rateLimiter      SamplingRateLimiter
	mu               sync.RWMutex
}

// NewSamplingSecurity creates a new sampling security manager
func NewSamplingSecurity(config SamplingSecurityConfig) *SamplingSecurity {
	return &SamplingSecurity{
		config: config,
	}
}

// SetApprovalHandler sets the handler for user approval requests
func (s *SamplingSecurity) SetApprovalHandler(handler ApprovalHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.approvalHandler = handler
}

// SetContentValidator sets the content validator
func (s *SamplingSecurity) SetContentValidator(validator ContentValidator) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.contentValidator = validator
}

// SetAuditLogger sets the audit logger
func (s *SamplingSecurity) SetAuditLogger(logger AuditLogger) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.auditLogger = logger
}

// SetRateLimiter sets the rate limiter
func (s *SamplingSecurity) SetRateLimiter(limiter SamplingRateLimiter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rateLimiter = limiter
}

// ValidateAndApprove validates and potentially requests approval for a sampling request
func (s *SamplingSecurity) ValidateAndApprove(ctx context.Context, req types.CreateMessageRequest, clientID string) (*types.CreateMessageRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Rate limiting check
	if s.config.EnableRateLimit && s.rateLimiter != nil {
		allowed, err := s.rateLimiter.AllowSampling(ctx, clientID)
		if err != nil {
			return nil, fmt.Errorf("rate limit check failed: %w", err)
		}
		if !allowed {
			return nil, fmt.Errorf("rate limit exceeded for client %s", clientID)
		}
	}

	// Content validation
	if s.contentValidator != nil {
		if err := s.contentValidator(ctx, req); err != nil {
			return nil, fmt.Errorf("content validation failed: %w", err)
		}
	}

	// Built-in security checks
	if err := s.performSecurityChecks(req); err != nil {
		return nil, err
	}

	// Audit logging
	if s.config.EnableAuditLogging && s.auditLogger != nil {
		if err := s.auditLogger.LogSamplingRequest(ctx, req); err != nil {
			// Log error but don't fail the request
			fmt.Printf("Failed to log sampling request: %v\n", err)
		}
	}

	// User approval if required
	if s.config.RequireUserApproval && s.approvalHandler != nil {
		approvalReq := ApprovalRequest{
			RequestID:        generateRequestID(),
			Messages:         req.Messages,
			ModelPreferences: req.ModelPreferences,
			SystemPrompt:     req.SystemPrompt,
			Temperature:      req.Temperature,
			MaxTokens:        req.MaxTokens,
			StopSequences:    req.StopSequences,
			Metadata:         req.Metadata,
			Timestamp:        time.Now(),
		}

		approvalCtx := ctx
		if s.config.ApprovalTimeout > 0 {
			var cancel context.CancelFunc
			approvalCtx, cancel = context.WithTimeout(ctx, s.config.ApprovalTimeout)
			defer cancel()
		}

		resp, err := s.approvalHandler(approvalCtx, approvalReq)
		if err != nil {
			return nil, fmt.Errorf("approval request failed: %w", err)
		}

		// Log approval result
		if s.config.EnableAuditLogging && s.auditLogger != nil {
			if err := s.auditLogger.LogApprovalRequest(ctx, approvalReq, resp); err != nil {
				fmt.Printf("Failed to log approval request: %v\n", err)
			}
		}

		switch resp.Result {
		case ApprovalDenied:
			return nil, fmt.Errorf("user denied sampling request: %s", resp.Reason)
		case ApprovalTimeout:
			return nil, fmt.Errorf("user approval timeout")
		case ApprovalModified:
			// Apply modifications
			modifiedReq := req
			if resp.ModifiedMessages != nil {
				modifiedReq.Messages = resp.ModifiedMessages
			}
			if resp.ModifiedPrompt != nil {
				modifiedReq.SystemPrompt = resp.ModifiedPrompt
			}
			return &modifiedReq, nil
		case ApprovalGranted:
			return &req, nil
		default:
			return nil, fmt.Errorf("unknown approval result: %d", resp.Result)
		}
	}

	return &req, nil
}

// LogSamplingResponse logs the sampling response for audit
func (s *SamplingSecurity) LogSamplingResponse(ctx context.Context, result types.CreateMessageResult) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.config.EnableAuditLogging && s.auditLogger != nil {
		if err := s.auditLogger.LogSamplingResponse(ctx, result); err != nil {
			fmt.Printf("Failed to log sampling response: %v\n", err)
		}
	}
}

// ValidateToolCall validates and potentially requests approval for a tool call
func (s *SamplingSecurity) ValidateToolCall(ctx context.Context, req types.CallToolRequest, clientID string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Rate limiting check - apply to all tool calls
	if s.config.EnableRateLimit && s.rateLimiter != nil {
		allowed, err := s.rateLimiter.AllowSampling(ctx, clientID)
		if err != nil {
			return fmt.Errorf("rate limit check failed: %w", err)
		}
		if !allowed {
			return fmt.Errorf("rate limit exceeded for client %s", clientID)
		}
	}

	// Check if this tool call requires approval (based on tool name or arguments)
	requiresApproval := s.requiresApprovalForTool(req)
	if s.config.RequireUserApproval && s.approvalHandler != nil && requiresApproval {
		// Convert tool call to approval request format
		approvalReq := ApprovalRequest{
			RequestID: generateRequestID(),
			Messages: []types.SamplingMessage{
				{
					Role: types.RoleUser,
					Content: types.TextContent{
						Type: "text",
						Text: fmt.Sprintf("Tool call: %s with arguments", req.Name),
					},
				},
			},
			MaxTokens: 100, // Default for tool calls
			Timestamp: time.Now(),
		}

		approvalCtx := ctx
		if s.config.ApprovalTimeout > 0 {
			var cancel context.CancelFunc
			approvalCtx, cancel = context.WithTimeout(ctx, s.config.ApprovalTimeout)
			defer cancel()
		}

		resp, err := s.approvalHandler(approvalCtx, approvalReq)
		if err != nil {
			return fmt.Errorf("approval request failed: %w", err)
		}

		switch resp.Result {
		case ApprovalDenied:
			return fmt.Errorf("user denied tool call: %s", resp.Reason)
		case ApprovalTimeout:
			return fmt.Errorf("user approval timeout")
		case ApprovalGranted, ApprovalModified:
			return nil // Allow the tool call
		default:
			return fmt.Errorf("unknown approval result: %d", resp.Result)
		}
	}

	return nil
}

// requiresApprovalForTool determines if a tool call requires user approval
func (s *SamplingSecurity) requiresApprovalForTool(req types.CallToolRequest) bool {
	// For testing, we'll require approval for tools with "secure" in the name
	// or tools that have "delete" or other dangerous operations
	toolName := strings.ToLower(req.Name)
	return strings.Contains(toolName, "secure") || 
		   strings.Contains(toolName, "delete") || 
		   strings.Contains(toolName, "remove")
}

// performSecurityChecks performs built-in security validations
func (s *SamplingSecurity) performSecurityChecks(req types.CreateMessageRequest) error {
	// Check max tokens limit
	if s.config.MaxTokensLimit > 0 && req.MaxTokens > s.config.MaxTokensLimit {
		return fmt.Errorf("max tokens %d exceeds limit %d", req.MaxTokens, s.config.MaxTokensLimit)
	}

	// Check for blocked patterns in messages
	for _, pattern := range s.config.BlockedPatterns {
		for _, msg := range req.Messages {
			if s.containsBlockedPattern(msg, pattern) {
				return fmt.Errorf("message contains blocked pattern: %s", pattern)
			}
		}
	}

	// Validate MIME types for image/audio content
	for _, msg := range req.Messages {
		if err := s.validateContentMimeType(msg.Content); err != nil {
			return err
		}
	}

	return nil
}

// containsBlockedPattern checks if a message contains blocked content
func (s *SamplingSecurity) containsBlockedPattern(msg types.SamplingMessage, pattern string) bool {
	// Simple pattern matching - could be enhanced with regex
	switch content := msg.Content.(type) {
	case types.TextContent:
		return strings.Contains(strings.ToLower(content.Text), strings.ToLower(pattern))
	case string:
		return strings.Contains(strings.ToLower(content), strings.ToLower(pattern))
	default:
		return false
	}
}

// validateContentMimeType validates MIME types for media content
func (s *SamplingSecurity) validateContentMimeType(content interface{}) error {
	if len(s.config.AllowedMimeTypes) == 0 {
		return nil // No restrictions
	}

	var mimeType string
	switch c := content.(type) {
	case types.ImageContent:
		mimeType = c.MimeType
	case types.AudioContent:
		mimeType = c.MimeType
	default:
		return nil // Non-media content
	}

	for _, allowed := range s.config.AllowedMimeTypes {
		if mimeType == allowed {
			return nil
		}
	}

	return fmt.Errorf("MIME type %s not allowed", mimeType)
}

// generateRequestID generates a unique request ID
func generateRequestID() string {
	return fmt.Sprintf("req_%d", time.Now().UnixNano())
}

// DefaultSamplingSecurityConfig returns a secure default configuration
func DefaultSamplingSecurityConfig() SamplingSecurityConfig {
	return SamplingSecurityConfig{
		RequireUserApproval: true,
		ApprovalTimeout:     30 * time.Second,
		MaxTokensLimit:      4096,
		BlockedPatterns: []string{
			"password", "secret", "token", "key", "credential",
			"social security", "ssn", "credit card", "bank account",
		},
		AllowedMimeTypes: []string{
			"image/jpeg", "image/png", "image/gif", "image/webp",
			"audio/mpeg", "audio/wav", "audio/ogg", "audio/mp4",
		},
		EnableAuditLogging: true,
		EnableRateLimit:    true,
	}
}