package transport

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"testing"
	"time"
)

func newTestMux(input string) (*Mux, *bytes.Buffer) {
	out := &bytes.Buffer{}
	return NewMux(io.NopCloser(bytes.NewReader([]byte(input))), out, log.New(io.Discard, "", 0)), out
}

// writeFrame is a small helper for building input bytes in tests.
func writeFrame(channel, body string) string {
	if channel == "" || channel == ChannelLSP {
		return fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body)
	}
	return fmt.Sprintf("Content-Length: %d\r\nChannel: %s\r\n\r\n%s", len(body), channel, body)
}

func TestMuxRoutesByChannel(t *testing.T) {
	input := writeFrame(ChannelLSP, `{"jsonrpc":"2.0","id":1,"method":"initialize"}`) +
		writeFrame(ChannelRPC, `{"jsonrpc":"2.0","id":2,"method":"rpc/ping"}`) +
		writeFrame(ChannelLSP, `{"jsonrpc":"2.0","id":3,"method":"shutdown"}`)

	mux, _ := newTestMux(input)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- mux.Run(ctx) }()

	gotLSP, gotRPC := drainBoth(t, mux)

	if err := <-done; err != nil {
		t.Errorf("Run returned error: %v", err)
	}
	if len(gotLSP) != 2 {
		t.Errorf("expected 2 LSP frames, got %d: %v", len(gotLSP), gotLSP)
	}
	if len(gotRPC) != 1 {
		t.Errorf("expected 1 RPC frame, got %d: %v", len(gotRPC), gotRPC)
	}
}

// drainBoth reads from mux.LSP() and mux.RPC() until both channels close,
// returning the message bodies it observed. Times out after 2 seconds.
func drainBoth(t *testing.T, mux *Mux) ([]string, []string) {
	t.Helper()
	var lsp, rpc []string
	lspCh := mux.LSP()
	rpcCh := mux.RPC()
	deadline := time.After(2 * time.Second)
	for lspCh != nil || rpcCh != nil {
		select {
		case body, ok := <-lspCh:
			if !ok {
				lspCh = nil
				continue
			}
			lsp = append(lsp, string(body))
		case body, ok := <-rpcCh:
			if !ok {
				rpcCh = nil
				continue
			}
			rpc = append(rpc, string(body))
		case <-deadline:
			t.Fatal("timed out draining mux channels")
		}
	}
	return lsp, rpc
}

func TestMuxClosesChannelsOnEOF(t *testing.T) {
	mux, _ := newTestMux(writeFrame(ChannelRPC, `{"jsonrpc":"2.0"}`))
	if err := mux.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	lsp, rpc := drainBoth(t, mux)
	if len(lsp) != 0 {
		t.Errorf("expected 0 LSP frames, got %v", lsp)
	}
	if len(rpc) != 1 || rpc[0] != `{"jsonrpc":"2.0"}` {
		t.Errorf("expected one buffered RPC frame, got %v", rpc)
	}
}

func TestMuxDropsUnknownChannel(t *testing.T) {
	input := writeFrame("unknown", `{"x":1}`) +
		writeFrame(ChannelRPC, `{"y":2}`)
	mux, _ := newTestMux(input)
	if err := mux.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	lsp, rpc := drainBoth(t, mux)
	if len(lsp) != 0 {
		t.Errorf("unknown-channel frame should be dropped, but LSP got: %v", lsp)
	}
	if len(rpc) != 1 || rpc[0] != `{"y":2}` {
		t.Errorf("expected exactly one RPC frame, got %v", rpc)
	}
}
