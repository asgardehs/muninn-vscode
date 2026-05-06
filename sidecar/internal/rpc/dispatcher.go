package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
)

// Handler processes a single request and returns either a marshallable result
// or a JSON-RPC error. For notifications, the result is ignored.
type Handler func(ctx context.Context, params json.RawMessage) (any, *Error)

// Dispatcher is a JSON-RPC method registry.
type Dispatcher struct {
	logger   *log.Logger
	mu       sync.RWMutex
	handlers map[string]Handler
}

func NewDispatcher(logger *log.Logger) *Dispatcher {
	return &Dispatcher{
		logger:   logger,
		handlers: make(map[string]Handler),
	}
}

func (d *Dispatcher) Register(method string, h Handler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.handlers[method] = h
}

// Dispatch parses a frame body, invokes the handler, and returns the response
// bytes (or nil for notifications).
func (d *Dispatcher) Dispatch(ctx context.Context, body []byte) ([]byte, error) {
	var req Request
	if err := json.Unmarshal(body, &req); err != nil {
		return marshalError(nil, &Error{Code: CodeParse, Message: fmt.Sprintf("parse error: %v", err)})
	}
	if req.Version != Version {
		return marshalError(req.ID, &Error{Code: CodeInvalidRequest, Message: "jsonrpc version must be 2.0"})
	}

	d.mu.RLock()
	h, ok := d.handlers[req.Method]
	d.mu.RUnlock()
	if !ok {
		if req.IsNotification() {
			d.logger.Printf("rpc: dropped unknown notification %q", req.Method)
			return nil, nil
		}
		return marshalError(req.ID, &Error{Code: CodeMethodNotFound, Message: fmt.Sprintf("method not found: %s", req.Method)})
	}

	result, rpcErr := h(ctx, req.Params)
	if req.IsNotification() {
		if rpcErr != nil {
			d.logger.Printf("rpc: notification %q handler error: %v", req.Method, rpcErr)
		}
		return nil, nil
	}
	if rpcErr != nil {
		return marshalError(req.ID, rpcErr)
	}
	return marshalSuccess(req.ID, result)
}

func marshalSuccess(id json.RawMessage, result any) ([]byte, error) {
	resBytes, err := json.Marshal(result)
	if err != nil {
		return marshalError(id, &Error{Code: CodeInternal, Message: fmt.Sprintf("marshal result: %v", err)})
	}
	return json.Marshal(&Response{Version: Version, ID: id, Result: resBytes})
}

func marshalError(id json.RawMessage, e *Error) ([]byte, error) {
	if id == nil {
		id = json.RawMessage("null")
	}
	return json.Marshal(&Response{Version: Version, ID: id, Error: e})
}
