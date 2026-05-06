// Package transport implements LSP-style Content-Length framing extended with
// an optional Channel header. Frames without a Channel header are treated as
// LSP, which keeps the sidecar compatible with vscode-languageclient as a
// drop-in client on the LSP channel.
package transport

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
)

const (
	ChannelLSP = "lsp"
	ChannelRPC = "rpc"
)

// Frame is one framed message read from or written to the transport.
type Frame struct {
	Channel string
	Body    []byte
}

// ReadFrame reads a single Content-Length framed message from br. The Channel
// header is optional; absence defaults to ChannelLSP.
func ReadFrame(br *bufio.Reader) (*Frame, error) {
	contentLength := -1
	channel := ChannelLSP

	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			return nil, fmt.Errorf("malformed header line: %q", line)
		}
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "content-length":
			n, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return nil, fmt.Errorf("invalid Content-Length %q: %w", value, err)
			}
			contentLength = n
		case "channel":
			channel = strings.ToLower(strings.TrimSpace(value))
		}
	}

	if contentLength < 0 {
		return nil, errors.New("missing Content-Length header")
	}

	body := make([]byte, contentLength)
	if _, err := io.ReadFull(br, body); err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	return &Frame{Channel: channel, Body: body}, nil
}

// FrameWriter serializes Write calls so concurrent goroutines can share one
// underlying io.Writer (e.g., LSP responses + RPC responses interleaving).
type FrameWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func NewFrameWriter(w io.Writer) *FrameWriter {
	return &FrameWriter{w: w}
}

// Write emits a single frame. The Channel header is omitted when channel is
// empty or ChannelLSP, so vscode-languageclient parses LSP frames unchanged.
func (fw *FrameWriter) Write(f *Frame) error {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "Content-Length: %d\r\n", len(f.Body))
	if f.Channel != "" && f.Channel != ChannelLSP {
		fmt.Fprintf(&buf, "Channel: %s\r\n", f.Channel)
	}
	buf.WriteString("\r\n")
	buf.Write(f.Body)

	_, err := fw.w.Write(buf.Bytes())
	return err
}
