package lsp

import (
	"context"
	"encoding/json"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

func (s *Server) handleCodeAction(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.CodeActionParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	// Phase B: no quick-fix actions yet. The "Create note: X" action
	// requires the muninn/createNote command which lands in Phase C
	// (createFromHierarchy). Returning an empty list keeps the LSP code
	// action surface alive without exposing a broken command.
	_ = params
	return reply(ctx, []protocol.CodeAction{}, nil)
}

