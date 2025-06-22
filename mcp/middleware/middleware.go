package middleware

import (
	"context"
	"log"
	"time"

	"github.com/manojparvathaneni/go-mcp-sdk/mcp/errors"
	"github.com/manojparvathaneni/go-mcp-sdk/mcp/types"
)

// MiddlewareHandlerFunc is an adapter to allow the use of ordinary functions as handlers
type MiddlewareHandlerFunc func(ctx context.Context, req *types.JSONRPCMessage) (*types.JSONRPCMessage, error)

// HandleRequest calls f(ctx, req)
func (f MiddlewareHandlerFunc) HandleRequest(ctx context.Context, req *types.JSONRPCMessage) (*types.JSONRPCMessage, error) {
	return f(ctx, req)
}

// HandleNotification is a no-op for MiddlewareHandlerFunc
func (f MiddlewareHandlerFunc) HandleNotification(ctx context.Context, notification *types.JSONRPCMessage) error {
	return nil
}

// Middleware is a function that wraps a MiddlewareHandler
type Middleware func(MiddlewareHandler) MiddlewareHandler

// MiddlewareHandler is the interface that wraps the basic request handling methods
type MiddlewareHandler interface {
	HandleRequest(ctx context.Context, req *types.JSONRPCMessage) (*types.JSONRPCMessage, error)
	HandleNotification(ctx context.Context, notification *types.JSONRPCMessage) error
}

// Chain creates a single middleware from multiple middlewares
func Chain(middlewares ...Middleware) Middleware {
	return func(next MiddlewareHandler) MiddlewareHandler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}
		return next
	}
}

// LoggingMiddleware logs all requests and responses
func LoggingMiddleware(logger *log.Logger) Middleware {
	return func(next MiddlewareHandler) MiddlewareHandler {
		return &loggingMiddlewareHandler{
			next:   next,
			logger: logger,
		}
	}
}

type loggingMiddlewareHandler struct {
	next   MiddlewareHandler
	logger *log.Logger
}

func (h *loggingMiddlewareHandler) HandleRequest(ctx context.Context, req *types.JSONRPCMessage) (*types.JSONRPCMessage, error) {
	start := time.Now()
	
	h.logger.Printf("[REQUEST] Method: %s, ID: %v", req.Method, req.ID)
	
	resp, err := h.next.HandleRequest(ctx, req)
	
	duration := time.Since(start)
	if err != nil {
		h.logger.Printf("[ERROR] Method: %s, Duration: %v, Error: %v", req.Method, duration, err)
	} else if resp.Error != nil {
		h.logger.Printf("[RESPONSE] Method: %s, Duration: %v, Error: %d %s", 
			req.Method, duration, resp.Error.Code, resp.Error.Message)
	} else {
		h.logger.Printf("[RESPONSE] Method: %s, Duration: %v, Success", req.Method, duration)
	}
	
	return resp, err
}

func (h *loggingMiddlewareHandler) HandleNotification(ctx context.Context, notification *types.JSONRPCMessage) error {
	h.logger.Printf("[NOTIFICATION] Method: %s", notification.Method)
	return h.next.HandleNotification(ctx, notification)
}

// RecoveryMiddleware recovers from panics and returns an internal error
func RecoveryMiddleware(logger *log.Logger) Middleware {
	return func(next MiddlewareHandler) MiddlewareHandler {
		return &recoveryMiddlewareHandler{
			next:   next,
			logger: logger,
		}
	}
}

type recoveryMiddlewareHandler struct {
	next   MiddlewareHandler
	logger *log.Logger
}

func (h *recoveryMiddlewareHandler) HandleRequest(ctx context.Context, req *types.JSONRPCMessage) (resp *types.JSONRPCMessage, err error) {
	defer func() {
		if r := recover(); r != nil {
			if h.logger != nil {
				h.logger.Printf("[PANIC] Method: %s, Panic: %v", req.Method, r)
			}
			
			resp = &types.JSONRPCMessage{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &types.JSONRPCError{
					Code:    types.InternalError,
					Message: "Internal server error",
				},
			}
			err = nil
		}
	}()
	
	return h.next.HandleRequest(ctx, req)
}

func (h *recoveryMiddlewareHandler) HandleNotification(ctx context.Context, notification *types.JSONRPCMessage) error {
	defer func() {
		if r := recover(); r != nil {
			if h.logger != nil {
				h.logger.Printf("[PANIC] Notification: %s, Panic: %v", notification.Method, r)
			}
		}
	}()
	
	return h.next.HandleNotification(ctx, notification)
}

// TimeoutMiddleware adds a timeout to requests
func TimeoutMiddleware(timeout time.Duration) Middleware {
	return func(next MiddlewareHandler) MiddlewareHandler {
		return &timeoutMiddlewareHandler{
			next:    next,
			timeout: timeout,
		}
	}
}

type timeoutMiddlewareHandler struct {
	next    MiddlewareHandler
	timeout time.Duration
}

func (h *timeoutMiddlewareHandler) HandleRequest(ctx context.Context, req *types.JSONRPCMessage) (*types.JSONRPCMessage, error) {
	ctx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()
	
	type result struct {
		resp *types.JSONRPCMessage
		err  error
	}
	
	done := make(chan result, 1)
	go func() {
		resp, err := h.next.HandleRequest(ctx, req)
		done <- result{resp, err}
	}()
	
	select {
	case <-ctx.Done():
		return &types.JSONRPCMessage{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &types.JSONRPCError{
				Code:    types.InternalError,
				Message: "Request timeout",
			},
		}, nil
	case r := <-done:
		return r.resp, r.err
	}
}

func (h *timeoutMiddlewareHandler) HandleNotification(ctx context.Context, notification *types.JSONRPCMessage) error {
	ctx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()
	
	done := make(chan error, 1)
	go func() {
		done <- h.next.HandleNotification(ctx, notification)
	}()
	
	select {
	case <-ctx.Done():
		return errors.ErrTimeout
	case err := <-done:
		return err
	}
}

// RateLimitMiddleware implements rate limiting
type RateLimiter interface {
	Allow() bool
}

func RateLimitMiddleware(limiter RateLimiter) Middleware {
	return func(next MiddlewareHandler) MiddlewareHandler {
		return &rateLimitMiddlewareHandler{
			next:    next,
			limiter: limiter,
		}
	}
}

type rateLimitMiddlewareHandler struct {
	next    MiddlewareHandler
	limiter RateLimiter
}

func (h *rateLimitMiddlewareHandler) HandleRequest(ctx context.Context, req *types.JSONRPCMessage) (*types.JSONRPCMessage, error) {
	if !h.limiter.Allow() {
		return &types.JSONRPCMessage{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &types.JSONRPCError{
				Code:    types.InvalidRequest,
				Message: "Rate limit exceeded",
			},
		}, nil
	}
	
	return h.next.HandleRequest(ctx, req)
}

func (h *rateLimitMiddlewareHandler) HandleNotification(ctx context.Context, notification *types.JSONRPCMessage) error {
	if !h.limiter.Allow() {
		return errors.NewProtocolError(errors.ErrorCodeRateLimited, "Rate limit exceeded", nil)
	}
	
	return h.next.HandleNotification(ctx, notification)
}

// ValidatorFunc is a function that validates requests
type ValidatorFunc func(ctx context.Context, req *types.JSONRPCMessage) error

// ValidationMiddleware validates requests before processing
func ValidationMiddleware(validator ValidatorFunc) Middleware {
	return func(next MiddlewareHandler) MiddlewareHandler {
		return &validationMiddlewareHandler{
			next:      next,
			validator: validator,
		}
	}
}

type validationMiddlewareHandler struct {
	next      MiddlewareHandler
	validator ValidatorFunc
}

func (h *validationMiddlewareHandler) HandleRequest(ctx context.Context, req *types.JSONRPCMessage) (*types.JSONRPCMessage, error) {
	if err := h.validator(ctx, req); err != nil {
		return &types.JSONRPCMessage{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &types.JSONRPCError{
				Code:    types.InvalidRequest,
				Message: err.Error(),
			},
		}, nil
	}
	
	return h.next.HandleRequest(ctx, req)
}

func (h *validationMiddlewareHandler) HandleNotification(ctx context.Context, notification *types.JSONRPCMessage) error {
	if err := h.validator(ctx, notification); err != nil {
		return err
	}
	
	return h.next.HandleNotification(ctx, notification)
}