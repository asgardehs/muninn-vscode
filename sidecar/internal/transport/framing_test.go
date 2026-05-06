package transport

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	cases := []struct {
		name    string
		channel string
		body    string
	}{
		{"lsp_default", ChannelLSP, `{"jsonrpc":"2.0","method":"initialize"}`},
		{"rpc_explicit", ChannelRPC, `{"jsonrpc":"2.0","method":"rpc/ping"}`},
		{"empty_body", ChannelRPC, `{}`},
		{"unicode", ChannelRPC, `{"text":"日本語 — em dash"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			fw := NewFrameWriter(&buf)
			if err := fw.Write(&Frame{Channel: tc.channel, Body: []byte(tc.body)}); err != nil {
				t.Fatalf("write: %v", err)
			}

			br := bufio.NewReader(&buf)
			got, err := ReadFrame(br)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			if got.Channel != tc.channel {
				t.Errorf("channel: got %q, want %q", got.Channel, tc.channel)
			}
			if string(got.Body) != tc.body {
				t.Errorf("body: got %q, want %q", got.Body, tc.body)
			}
		})
	}
}

func TestReadFrameAbsentChannelDefaultsToLSP(t *testing.T) {
	body := `{"jsonrpc":"2.0","id":1}`
	raw := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body)
	br := bufio.NewReader(strings.NewReader(raw))
	f, err := ReadFrame(br)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if f.Channel != ChannelLSP {
		t.Errorf("expected default channel %q, got %q", ChannelLSP, f.Channel)
	}
	if string(f.Body) != body {
		t.Errorf("body: got %q, want %q", f.Body, body)
	}
}

func TestWriteOmitsChannelHeaderForLSP(t *testing.T) {
	var buf bytes.Buffer
	fw := NewFrameWriter(&buf)
	if err := fw.Write(&Frame{Channel: ChannelLSP, Body: []byte(`{}`)}); err != nil {
		t.Fatalf("write: %v", err)
	}
	if strings.Contains(buf.String(), "Channel:") {
		t.Errorf("LSP frame should not include Channel header, got: %q", buf.String())
	}
}

func TestWriteEmitsChannelHeaderForRPC(t *testing.T) {
	var buf bytes.Buffer
	fw := NewFrameWriter(&buf)
	if err := fw.Write(&Frame{Channel: ChannelRPC, Body: []byte(`{}`)}); err != nil {
		t.Fatalf("write: %v", err)
	}
	if !strings.Contains(buf.String(), "Channel: rpc\r\n") {
		t.Errorf("RPC frame missing Channel header, got: %q", buf.String())
	}
}

func TestReadFrameMissingContentLength(t *testing.T) {
	raw := "Channel: rpc\r\n\r\n{}"
	br := bufio.NewReader(strings.NewReader(raw))
	if _, err := ReadFrame(br); err == nil {
		t.Error("expected error for missing Content-Length, got nil")
	}
}

func TestReadFrameMalformedHeader(t *testing.T) {
	raw := "Content-Length 5\r\n\r\nhello"
	br := bufio.NewReader(strings.NewReader(raw))
	if _, err := ReadFrame(br); err == nil {
		t.Error("expected error for malformed header, got nil")
	}
}

func TestRoundTripManyFrames(t *testing.T) {
	var buf bytes.Buffer
	fw := NewFrameWriter(&buf)
	const n = 1000
	for i := 0; i < n; i++ {
		body := []byte(fmt.Sprintf(`{"id":%d}`, i))
		if err := fw.Write(&Frame{Channel: ChannelRPC, Body: body}); err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
	}
	br := bufio.NewReader(&buf)
	for i := 0; i < n; i++ {
		f, err := ReadFrame(br)
		if err != nil {
			t.Fatalf("read %d: %v", i, err)
		}
		want := fmt.Sprintf(`{"id":%d}`, i)
		if string(f.Body) != want {
			t.Errorf("frame %d: got %q, want %q", i, f.Body, want)
		}
	}
}

func TestConcurrentWritesAreSerialized(t *testing.T) {
	var buf bytes.Buffer
	fw := NewFrameWriter(&buf)

	var wg sync.WaitGroup
	const writers = 8
	const perWriter = 100
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < perWriter; i++ {
				body := []byte(fmt.Sprintf(`{"w":%d,"i":%d}`, w, i))
				if err := fw.Write(&Frame{Channel: ChannelRPC, Body: body}); err != nil {
					t.Errorf("write w=%d i=%d: %v", w, i, err)
				}
			}
		}(w)
	}
	wg.Wait()

	br := bufio.NewReader(&buf)
	count := 0
	for {
		_, err := ReadFrame(br)
		if err != nil {
			break
		}
		count++
	}
	if count != writers*perWriter {
		t.Errorf("frame count: got %d, want %d", count, writers*perWriter)
	}
}
