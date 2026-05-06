package lsp

import (
	"context"
	"encoding/json"
	"io"
	"sync"

	"go.lsp.dev/jsonrpc2"

	"github.com/asgardehs/muninn-sidecar/internal/transport"
)

// MuxStream implements jsonrpc2.Stream by reading from a channel of LSP-frame
// bodies (fed by transport.Mux) and writing responses through a shared
// transport.FrameWriter as LSP-channel frames. The FrameWriter omits the
// Channel: header for LSP frames so vscode-languageclient parses them as
// plain LSP messages.
type MuxStream struct {
	incoming <-chan []byte
	fw       *transport.FrameWriter

	closeOnce sync.Once
	closed    chan struct{}
}

// NewMuxStream constructs a MuxStream wired to the given Mux's LSP channel
// and FrameWriter.
func NewMuxStream(incoming <-chan []byte, fw *transport.FrameWriter) *MuxStream {
	return &MuxStream{
		incoming: incoming,
		fw:       fw,
		closed:   make(chan struct{}),
	}
}

// Read decodes the next LSP message from the multiplexed stream.
func (s *MuxStream) Read(ctx context.Context) (jsonrpc2.Message, int64, error) {
	select {
	case <-ctx.Done():
		return nil, 0, ctx.Err()
	case <-s.closed:
		return nil, 0, io.EOF
	case body, ok := <-s.incoming:
		if !ok {
			return nil, 0, io.EOF
		}
		msg, err := jsonrpc2.DecodeMessage(body)
		if err != nil {
			return nil, 0, err
		}
		return msg, int64(len(body)), nil
	}
}

// Write marshals and emits the message as an LSP-channel frame.
func (s *MuxStream) Write(_ context.Context, msg jsonrpc2.Message) (int64, error) {
	body, err := json.Marshal(msg)
	if err != nil {
		return 0, err
	}
	if err := s.fw.Write(&transport.Frame{Channel: transport.ChannelLSP, Body: body}); err != nil {
		return 0, err
	}
	return int64(len(body)), nil
}

// Close marks the stream as closed; subsequent Read calls return io.EOF.
// Idempotent.
func (s *MuxStream) Close() error {
	s.closeOnce.Do(func() { close(s.closed) })
	return nil
}
