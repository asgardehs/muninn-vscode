package rpc

import (
	"context"
	"encoding/json"
	"time"
)

// HandlePing returns a handler that responds with pong + version + uptime.
func HandlePing(version string) Handler {
	startedAt := time.Now()
	return func(_ context.Context, _ json.RawMessage) (any, *Error) {
		return map[string]any{
			"pong":      true,
			"version":   version,
			"uptime_ms": time.Since(startedAt).Milliseconds(),
		}, nil
	}
}
