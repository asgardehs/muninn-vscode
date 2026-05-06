package transport

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log"
)

// Mux multiplexes the LSP and RPC channels over a single framed stdio stream.
// Run reads incoming frames and routes them to per-channel queues. Consumers
// read from LSP() and RPC() and write replies through Writer().
//
// On EOF or context cancellation, both channels are closed so consumers see
// a clean shutdown signal.
type Mux struct {
	reader io.Reader
	writer *FrameWriter
	logger *log.Logger

	lspCh chan []byte
	rpcCh chan []byte
}

// NewMux constructs a Mux over the given input/output streams.
func NewMux(r io.Reader, w io.Writer, logger *log.Logger) *Mux {
	return &Mux{
		reader: r,
		writer: NewFrameWriter(w),
		logger: logger,
		lspCh:  make(chan []byte, 64),
		rpcCh:  make(chan []byte, 64),
	}
}

// LSP returns the channel of LSP-frame bodies.
func (m *Mux) LSP() <-chan []byte { return m.lspCh }

// RPC returns the channel of RPC-frame bodies.
func (m *Mux) RPC() <-chan []byte { return m.rpcCh }

// Writer returns the shared serialized frame writer.
func (m *Mux) Writer() *FrameWriter { return m.writer }

// Run consumes frames from the reader until EOF or ctx is cancelled. Both the
// LSP and RPC channels are closed on exit so blocked consumers wake up.
func (m *Mux) Run(ctx context.Context) error {
	defer close(m.lspCh)
	defer close(m.rpcCh)

	br := bufio.NewReader(m.reader)
	for {
		if ctx.Err() != nil {
			return nil
		}
		f, err := ReadFrame(br)
		if errors.Is(err, io.EOF) {
			m.logger.Printf("mux: stdin EOF")
			return nil
		}
		if err != nil {
			return err
		}

		var ch chan []byte
		switch f.Channel {
		case ChannelLSP:
			ch = m.lspCh
		case ChannelRPC:
			ch = m.rpcCh
		default:
			m.logger.Printf("mux: unknown channel %q, dropping %d bytes", f.Channel, len(f.Body))
			continue
		}

		select {
		case ch <- f.Body:
		case <-ctx.Done():
			return nil
		}
	}
}
