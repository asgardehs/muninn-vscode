package lsp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"

	"github.com/asgardehs/muninn-sidecar/internal/vault"
)

// captureConn records the params passed to Notify so tests can inspect the
// wire-level JSON. All other methods of jsonrpc2.Conn would panic if called;
// publishDiagnostics only exercises Notify.
type captureConn struct {
	jsonrpc2.Conn
	notifications []capturedNotification
}

type capturedNotification struct {
	method string
	params any
}

func (c *captureConn) Notify(_ context.Context, method string, params any) error {
	c.notifications = append(c.notifications, capturedNotification{method: method, params: params})
	return nil
}

// TestPublishDiagnosticsEmptyMarshalsAsArray pins the wire format when there
// are no violations. A nil slice marshals to JSON `null`, which crashes
// vscode-languageclient's diagnostic queue (it calls .length on the payload),
// silently drops the publish, and leaves stale diagnostics in the editor
// until the user reloads the window.
func TestPublishDiagnosticsEmptyMarshalsAsArray(t *testing.T) {
	s := New(vault.New(t.TempDir()))
	conn := &captureConn{}
	s.conn = conn

	// A note with no frontmatter and no wikilinks → no diagnostics.
	s.publishDiagnostics(context.Background(), protocol.DocumentURI("file:///note.md"), "just body text\n")

	if len(conn.notifications) != 1 {
		t.Fatalf("got %d notifications, want 1", len(conn.notifications))
	}
	n := conn.notifications[0]
	if n.method != "textDocument/publishDiagnostics" {
		t.Errorf("method = %q, want textDocument/publishDiagnostics", n.method)
	}
	raw, ok := n.params.(json.RawMessage)
	if !ok {
		t.Fatalf("params type = %T, want json.RawMessage", n.params)
	}
	wire := string(raw)
	if strings.Contains(wire, `"diagnostics":null`) {
		t.Errorf("wire payload contains diagnostics:null, vscode-languageclient will drop it: %s", wire)
	}
	if !strings.Contains(wire, `"diagnostics":[]`) {
		t.Errorf("wire payload missing diagnostics:[]: %s", wire)
	}
}
