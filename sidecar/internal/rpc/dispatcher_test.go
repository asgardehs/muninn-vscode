package rpc

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"strings"
	"testing"
)

func newDispatcher() *Dispatcher {
	return NewDispatcher(log.New(io.Discard, "", 0))
}

func TestDispatchSuccess(t *testing.T) {
	d := newDispatcher()
	d.Register("rpc/ping", HandlePing("test"))

	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"rpc/ping"}`)
	resp, err := d.Dispatch(context.Background(), body)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	var got Response
	if err := json.Unmarshal(resp, &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got.Error != nil {
		t.Fatalf("unexpected error: %v", got.Error)
	}
	if !strings.Contains(string(got.Result), `"pong":true`) {
		t.Errorf("result missing pong: %s", got.Result)
	}
	if string(got.ID) != "1" {
		t.Errorf("id: got %s, want 1", got.ID)
	}
}

func TestDispatchMethodNotFound(t *testing.T) {
	d := newDispatcher()
	body := []byte(`{"jsonrpc":"2.0","id":2,"method":"does/not/exist"}`)
	resp, err := d.Dispatch(context.Background(), body)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	var got Response
	if err := json.Unmarshal(resp, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Error == nil || got.Error.Code != CodeMethodNotFound {
		t.Errorf("expected method-not-found error, got %+v", got.Error)
	}
}

func TestDispatchNotificationReturnsNil(t *testing.T) {
	d := newDispatcher()
	d.Register("noop", func(_ context.Context, _ json.RawMessage) (any, *Error) {
		return nil, nil
	})
	body := []byte(`{"jsonrpc":"2.0","method":"noop"}`)
	resp, err := d.Dispatch(context.Background(), body)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if resp != nil {
		t.Errorf("expected nil response for notification, got: %s", resp)
	}
}

func TestDispatchParseError(t *testing.T) {
	d := newDispatcher()
	resp, err := d.Dispatch(context.Background(), []byte(`{not json`))
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	var got Response
	if err := json.Unmarshal(resp, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Error == nil || got.Error.Code != CodeParse {
		t.Errorf("expected parse error, got %+v", got.Error)
	}
}

func TestDispatchWrongVersion(t *testing.T) {
	d := newDispatcher()
	body := []byte(`{"jsonrpc":"1.0","id":3,"method":"rpc/ping"}`)
	resp, err := d.Dispatch(context.Background(), body)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	var got Response
	if err := json.Unmarshal(resp, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Error == nil || got.Error.Code != CodeInvalidRequest {
		t.Errorf("expected invalid-request error, got %+v", got.Error)
	}
}
