// Package mcp provides a Go implementation of the Model Context Protocol (MCP).
// MCP is a standardized protocol for exposing data and functionality to LLM applications.
//
// This package follows Go idiomatic patterns similar to popular frameworks like Gin:
//   - MCPServer for creating servers with method chaining
//   - MCPClient for connecting to MCP servers
//   - RouteGroup for organizing related resources/tools
//   - Middleware support for cross-cutting concerns
//   - Rich error handling with proper error wrapping
package mcp

import (
	"github.com/manojparvathaneni/go-mcp-sdk/mcp/auth"
	"github.com/manojparvathaneni/go-mcp-sdk/mcp/client"
	"github.com/manojparvathaneni/go-mcp-sdk/mcp/connection"
	"github.com/manojparvathaneni/go-mcp-sdk/mcp/elicitation"
	"github.com/manojparvathaneni/go-mcp-sdk/mcp/errors"
	"github.com/manojparvathaneni/go-mcp-sdk/mcp/events"
	"github.com/manojparvathaneni/go-mcp-sdk/mcp/middleware"
	"github.com/manojparvathaneni/go-mcp-sdk/mcp/security"
	"github.com/manojparvathaneni/go-mcp-sdk/mcp/server"
	"github.com/manojparvathaneni/go-mcp-sdk/mcp/tools"
	"github.com/manojparvathaneni/go-mcp-sdk/mcp/transport"
	"github.com/manojparvathaneni/go-mcp-sdk/mcp/types"
)

// Re-export key types and functions for easier access

// Client types
type Client = client.Client
type ClientOptions = client.ClientOptions

// Server types (low-level)
type Server = server.Server
type ServerOptions = server.ServerOptions
type ServerResourceHandler = server.ResourceHandler
type ServerToolHandler = server.ToolHandler
type ServerPromptHandler = server.PromptHandler

// High-level server builder types are defined in server_builder.go
// MCPServerOptions, MCPServer, RouteGroup, ResourceFunc, ToolFunc, PromptFunc

// Transport types
type StdioTransport = transport.StdioTransport
type SSETransport = transport.SSETransport

// Protocol types
type (
	JSONRPCMessage              = types.JSONRPCMessage
	JSONRPCError                = types.JSONRPCError
	Request                     = types.Request
	Notification                = types.Notification
	Response                    = types.Response
	McpError                    = types.McpError
	Implementation              = types.Implementation
	ClientCapabilities          = types.ClientCapabilities
	ServerCapabilities          = types.ServerCapabilities
	InitializeRequest           = types.InitializeRequest
	InitializeResult            = types.InitializeResult
	InitializedNotification     = types.InitializedNotification
	Resource                    = types.Resource
	ResourceTemplate            = types.ResourceTemplate
	ListResourcesRequest        = types.ListResourcesRequest
	ListResourcesResult         = types.ListResourcesResult
	ReadResourceRequest         = types.ReadResourceRequest
	ReadResourceResult          = types.ReadResourceResult
	ResourceContentsText        = types.ResourceContentsText
	ResourceContentsBlob        = types.ResourceContentsBlob
	Tool                        = types.Tool
	ListToolsRequest            = types.ListToolsRequest
	ListToolsResult             = types.ListToolsResult
	CallToolRequest             = types.CallToolRequest
	CallToolResult              = types.CallToolResult
	Prompt                      = types.Prompt
	PromptArgument              = types.PromptArgument
	ListPromptsRequest          = types.ListPromptsRequest
	ListPromptsResult           = types.ListPromptsResult
	GetPromptRequest            = types.GetPromptRequest
	GetPromptResult             = types.GetPromptResult
	PromptMessage               = types.PromptMessage
	// TextContent and ImageContent are defined as helper functions in helpers.go
	// types.TextContent and types.ImageContent are the struct types
	AudioContent                = types.AudioContent
	ResourceContents            = types.ResourceContents
	Role                        = types.Role
	LogLevel                    = types.LogLevel
	SetLogLevelRequest          = types.SetLogLevelRequest
	LoggingMessageNotification  = types.LoggingMessageNotification
	CompleteRequest             = types.CompleteRequest
	CompleteResult              = types.CompleteResult
	
	// Elicitation types
	ElicitRequest               = types.ElicitRequest
	ElicitResult                = types.ElicitResult
	ElicitSchema                = types.ElicitSchema
	ElicitOption                = types.ElicitOption
	ElicitProperty              = types.ElicitProperty
	ElicitValidation            = types.ElicitValidation
	ElicitationCapability       = types.ElicitationCapability
	ElicitNotification          = types.ElicitNotification
	ElicitResponseNotification  = types.ElicitResponseNotification
	ElicitCancelNotification    = types.ElicitCancelNotification
	ValidationError             = types.ValidationError
	ElicitTimeoutError          = types.ElicitTimeoutError
	ElicitCancelledError        = types.ElicitCancelledError
)

// Constants
const (
	RoleAssistant = types.RoleAssistant
	RoleUser      = types.RoleUser
	
	LogLevelDebug   = types.LogLevelDebug
	LogLevelInfo    = types.LogLevelInfo
	LogLevelWarning = types.LogLevelWarning
	LogLevelError   = types.LogLevelError
	
	ParseError     = types.ParseError
	InvalidRequest = types.InvalidRequest
	MethodNotFound = types.MethodNotFound
	InvalidParams  = types.InvalidParams
	InternalError  = types.InternalError
	
	// Elicitation type constants
	ElicitTypeConfirmation = types.ElicitTypeConfirmation
	ElicitTypeInput        = types.ElicitTypeInput
	ElicitTypeChoice       = types.ElicitTypeChoice
	ElicitTypeMultiChoice  = types.ElicitTypeMultiChoice
	ElicitTypeForm         = types.ElicitTypeForm
	ElicitTypeFile         = types.ElicitTypeFile
	ElicitTypePassword     = types.ElicitTypePassword
	ElicitTypeNumber       = types.ElicitTypeNumber
	ElicitTypeDate         = types.ElicitTypeDate
)

// Constructor functions
var (
	NewClient                    = client.NewClient
	NewCoreServer                = server.NewServer // Lower-level server API
	NewStdioTransport            = transport.NewStdioTransport
	NewStdioTransportWithStreams = transport.NewStdioTransportWithStreams
	NewSSETransport              = transport.NewSSETransport
	NewJSONRPCError              = types.NewJSONRPCError
)

// NewMCPServer creates a new high-level MCP server
// This is the primary way to create MCP servers in a Go-idiomatic way
// Defined in server_builder.go

// NewMCPClient creates a new MCP client with the given options
var NewMCPClient = client.NewClient

// Helper functions for creating content

// NewAudioContent creates an audio content item
func NewAudioContent(data string, mimeType string) types.AudioContent {
	return types.AudioContent{
		Type:     "audio",
		Data:     data,
		MimeType: mimeType,
	}
}

// Elicitation helper functions

// NewConfirmationElicit creates a yes/no confirmation elicitation
var NewConfirmationElicit = types.NewConfirmationElicit

// NewInputElicit creates a text input elicitation
var NewInputElicit = types.NewInputElicit

// NewChoiceElicit creates a single choice elicitation
var NewChoiceElicit = types.NewChoiceElicit

// NewFormElicit creates a structured form elicitation
var NewFormElicit = types.NewFormElicit

// Errors package exports
type (
	ErrorCode      = errors.ErrorCode
	ProtocolError  = errors.ProtocolError
)

const (
	ErrorCodeInvalidRequest    = errors.ErrorCodeInvalidRequest
	ErrorCodeResourceNotFound  = errors.ErrorCodeResourceNotFound
	ErrorCodeToolNotFound      = errors.ErrorCodeToolNotFound
	ErrorCodePromptNotFound    = errors.ErrorCodePromptNotFound
	ErrorCodeInvalidArguments  = errors.ErrorCodeInvalidArguments
	ErrorCodeInternalError     = errors.ErrorCodeInternalError
	ErrorCodeUnauthorized      = errors.ErrorCodeUnauthorized
	ErrorCodeRateLimited       = errors.ErrorCodeRateLimited
)

var (
	ErrNotInitialized   = errors.ErrNotInitialized
	ErrTransportNotSet  = errors.ErrTransportNotSet
	ErrResourceNotFound = errors.ErrResourceNotFound
	ErrToolNotFound     = errors.ErrToolNotFound
	ErrPromptNotFound   = errors.ErrPromptNotFound
	ErrInvalidArguments = errors.ErrInvalidArguments
	ErrTimeout          = errors.ErrTimeout
	ErrClosed           = errors.ErrClosed
	
	NewProtocolError         = errors.NewProtocolError
	NewProtocolErrorWithData = errors.NewProtocolErrorWithData
	IsProtocolError          = errors.IsProtocolError
	GetProtocolError         = errors.GetProtocolError
	IsErrorCode              = errors.IsErrorCode
)

// Security package exports
type (
	SamplingSecurity       = security.SamplingSecurity
	SamplingSecurityConfig = security.SamplingSecurityConfig
	SecureSamplingClient   = security.SecureSamplingClient
	ApprovalHandler        = security.ApprovalHandler
	ContentValidator       = security.ContentValidator
	AuditLogger            = security.AuditLogger
	ApprovalRequest        = security.ApprovalRequest
	ApprovalResponse       = security.ApprovalResponse
	ApprovalResult         = security.ApprovalResult
)

const (
	ApprovalDenied   = security.ApprovalDenied
	ApprovalGranted  = security.ApprovalGranted
	ApprovalModified = security.ApprovalModified
	ApprovalTimeout  = security.ApprovalTimeout
)

var (
	NewSamplingSecurity     = security.NewSamplingSecurity
	NewSecureSamplingClient = security.NewSecureSamplingClient
)

// Tools package exports
type (
	StructuredToolFunc   = tools.StructuredToolFunc
	JSONSchemaValidator  = tools.JSONSchemaValidator
)

var (
	StructuredToolResult     = tools.StructuredToolResult
	StructuredToolError      = tools.StructuredToolError
	StructuredToolWrapper    = tools.StructuredToolWrapper
	ValidateStructuredOutput = tools.ValidateStructuredOutput
	NewJSONSchemaValidator   = tools.NewJSONSchemaValidator
	StringOutputSchema       = tools.StringOutputSchema
	ObjectOutputSchema       = tools.ObjectOutputSchema
	ArrayOutputSchema        = tools.ArrayOutputSchema
	NumberOutputSchema       = tools.NumberOutputSchema
)

// Authentication and authorization package exports
type (
	User                    = auth.User
	TokenValidator          = auth.TokenValidator
	PermissionChecker       = auth.PermissionChecker
	SessionManager          = auth.SessionManager
	Session                 = auth.Session
	AuthError               = auth.AuthError
	JWTConfig               = auth.JWTConfig
	JWTValidator            = auth.JWTValidator
	OAuthConfig             = auth.OAuthConfig
	OAuthIntrospector       = auth.OAuthIntrospector
	InMemorySessionManager  = auth.InMemorySessionManager
	SessionManagerConfig    = auth.SessionManagerConfig
	BasicPermissionChecker  = auth.BasicPermissionChecker
	ToolPermission          = auth.ToolPermission
	AuthenticationConfig    = auth.AuthenticationConfig
	AuthorizationConfig     = auth.AuthorizationConfig
	MethodPermission        = auth.MethodPermission
	SessionStats            = auth.SessionStats
)

const (
	// Authentication error codes
	ErrCodeInvalidToken     = auth.ErrCodeInvalidToken
	ErrCodeExpiredToken     = auth.ErrCodeExpiredToken
	ErrCodeInsufficientScope = auth.ErrCodeInsufficientScope
	ErrCodeAccessDenied     = auth.ErrCodeAccessDenied
	ErrCodeInvalidAudience  = auth.ErrCodeInvalidAudience
	ErrCodeInvalidIssuer    = auth.ErrCodeInvalidIssuer
	ErrCodeMalformedToken   = auth.ErrCodeMalformedToken
	ErrCodeTokenNotFound    = auth.ErrCodeTokenNotFound
	ErrCodeSessionExpired   = auth.ErrCodeSessionExpired
	ErrCodePermissionDenied = auth.ErrCodePermissionDenied
	
	// Context keys
	UserContextKey        = auth.UserContextKey
	SessionContextKey     = auth.SessionContextKey
	TokenContextKey       = auth.TokenContextKey
	PermissionsContextKey = auth.PermissionsContextKey
)

var (
	// Authentication and authorization functions
	NewJWTValidator             = auth.NewJWTValidator
	NewOAuthIntrospector        = auth.NewOAuthIntrospector
	NewInMemorySessionManager   = auth.NewInMemorySessionManager
	NewBasicPermissionChecker   = auth.NewBasicPermissionChecker
	NewMCPPermissionChecker     = auth.NewMCPPermissionChecker
	AuthenticationMiddleware    = auth.AuthenticationMiddleware
	AuthorizationMiddleware     = auth.AuthorizationMiddleware
	SessionMiddleware          = auth.SessionMiddleware
	
	// Context helpers
	GetUserFromContext    = auth.GetUserFromContext
	GetSessionFromContext = auth.GetSessionFromContext
	GetTokenFromContext   = auth.GetTokenFromContext
	WithUser              = auth.WithUser
	WithSession           = auth.WithSession
	WithToken             = auth.WithToken
	
	// Permission helpers
	HasMCPAccess       = auth.HasMCPAccess
	CanExecuteTools    = auth.CanExecuteTools
	CanAccessResources = auth.CanAccessResources
	CanAccessPrompts   = auth.CanAccessPrompts
	
	// JWT utilities
	CreateJWT = auth.CreateJWT
	NewAuthError = auth.NewAuthError
)

// Events package exports
type (
	EventEmitter = events.EventEmitter
	Event        = events.Event
	EventType    = events.EventType
	EventHook    = events.EventHook
)

const (
	EventResourceListed      = events.EventResourceListed
	EventResourceRead        = events.EventResourceRead
	EventToolListed          = events.EventToolListed
	EventToolCalled          = events.EventToolCalled
	EventPromptListed        = events.EventPromptListed
	EventPromptExecuted      = events.EventPromptExecuted
	EventClientConnected     = events.EventClientConnected
	EventClientDisconnected  = events.EventClientDisconnected
	EventRequestReceived     = events.EventRequestReceived
	EventResponseSent        = events.EventResponseSent
	EventError               = events.EventError
)

var (
	NewEventEmitter    = events.NewEventEmitter
	ResourceEventData  = events.ResourceEventData
	ToolEventData      = events.ToolEventData
	PromptEventData    = events.PromptEventData
	RequestEventData   = events.RequestEventData
	ResponseEventData  = events.ResponseEventData
	ErrorEventData     = events.ErrorEventData
)

// Middleware package exports
type (
	Middleware             = middleware.Middleware
	MiddlewareHandler      = middleware.MiddlewareHandler
	MiddlewareHandlerFunc  = middleware.MiddlewareHandlerFunc
	RateLimiter            = middleware.RateLimiter
)

var (
	Chain               = middleware.Chain
	LoggingMiddleware   = middleware.LoggingMiddleware
	TimeoutMiddleware   = middleware.TimeoutMiddleware
	RecoveryMiddleware  = middleware.RecoveryMiddleware
	RateLimitMiddleware = middleware.RateLimitMiddleware
	NewRateLimiter      = middleware.NewRateLimiter
)

// Connection package exports
type (
	ConnectionManager  = connection.ConnectionManager
	ConnectionPool     = connection.ConnectionPool
	ConnectionState    = connection.ConnectionState
	ConnectionOptions  = connection.ConnectionOptions
)

const (
	ConnectionStateDisconnected = connection.ConnectionStateDisconnected
	ConnectionStateConnecting   = connection.ConnectionStateConnecting
	ConnectionStateConnected    = connection.ConnectionStateConnected
	ConnectionStateReconnecting = connection.ConnectionStateReconnecting
)

var (
	NewConnectionManager = connection.NewConnectionManager
	NewConnectionPool    = connection.NewConnectionPool
	DefaultConnectionOptions = connection.DefaultConnectionOptions
)

// Elicitation package exports
type (
	ElicitationClient     = elicitation.ElicitationClient
	ElicitationUIHandler  = elicitation.ElicitationUIHandler
)

var (
	NewElicitationClient = elicitation.NewElicitationClient
)