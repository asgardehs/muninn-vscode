// Package rpc implements the JSON-RPC 2.0 server used on the custom RPC
// channel of the sidecar transport. The LSP channel is handled separately
// by go.lsp.dev.
package rpc

import "encoding/json"

const Version = "2.0"

type Request struct {
	Version string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	Version string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

type Notification struct {
	Version string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (e *Error) Error() string { return e.Message }

const (
	CodeParse          = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternal       = -32603
	CodeVault          = -32000
	CodeSchema         = -32001
	CodeNotFound       = -32002
)

// IsNotification reports whether r has no id and therefore expects no response.
func (r *Request) IsNotification() bool {
	return len(r.ID) == 0
}
