package server

import (
	"context"
	"net/http"
)

// mockHTTPRequest is a custom implementation for testing that supports PathValue
type mockHTTPRequest struct {
	*http.Request
	pathValues map[string]string
}

// PathValue returns path parameters for testing
func (r *mockHTTPRequest) PathValue(key string) string {
	return r.pathValues[key]
}

// WithPathValue creates a mock request with path values
func WithPathValue(req *http.Request, values map[string]string) *mockHTTPRequest {
	return &mockHTTPRequest{
		Request:    req,
		pathValues: values,
	}
}

// pathValueContextKey is used for storing path values in context
type pathValueContextKey struct{}

// NewPathValueContext creates a context with path values for testing
func NewPathValueContext(ctx context.Context, values map[string]string) context.Context {
	return context.WithValue(ctx, pathValueContextKey{}, values)
}

// PathValueFromContext extracts path values from context for testing
func PathValueFromContext(ctx context.Context, key string) string {
	values, ok := ctx.Value(pathValueContextKey{}).(map[string]string)
	if !ok {
		return ""
	}
	return values[key]
}
