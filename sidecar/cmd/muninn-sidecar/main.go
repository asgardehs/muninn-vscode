package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/asgardehs/muninn-sidecar/internal/rpc"
	"github.com/asgardehs/muninn-sidecar/internal/transport"
)

const version = "0.0.1"

func main() {
	workspace := flag.String("workspace", "", "vault root path (defaults to current directory)")
	logLevel := flag.String("log-level", "info", "log level: error|warn|info|debug")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

	if *workspace == "" {
		wd, err := os.Getwd()
		if err != nil {
			log.Fatalf("muninn-sidecar: cannot determine workspace: %v", err)
		}
		*workspace = wd
	}

	logger := log.New(os.Stderr, "[muninn-sidecar] ", log.LstdFlags|log.Lmicroseconds)
	logger.Printf("starting v%s, workspace=%s, log-level=%s", version, *workspace, *logLevel)

	dispatcher := rpc.NewDispatcher(logger)
	dispatcher.Register("rpc/ping", rpc.HandlePing(version))

	writer := transport.NewFrameWriter(os.Stdout)

	if err := sendNotification(writer, "sidecar/ready", map[string]any{
		"version":      version,
		"capabilities": []string{"lookup", "schema"},
		"vaultPath":    *workspace,
	}); err != nil {
		logger.Printf("failed to send sidecar/ready: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case s := <-sigCh:
			logger.Printf("received %s, shutting down", s)
			cancel()
		case <-ctx.Done():
		}
	}()

	if err := serve(ctx, os.Stdin, writer, dispatcher, logger); err != nil {
		logger.Printf("serve exited with error: %v", err)
		os.Exit(1)
	}
	logger.Printf("clean shutdown")
}

func serve(ctx context.Context, r io.Reader, w *transport.FrameWriter, d *rpc.Dispatcher, logger *log.Logger) error {
	br := bufio.NewReader(r)
	for {
		if ctx.Err() != nil {
			return nil
		}
		f, err := transport.ReadFrame(br)
		if err == io.EOF {
			logger.Printf("stdin EOF, exiting")
			return nil
		}
		if err != nil {
			return fmt.Errorf("read frame: %w", err)
		}

		switch f.Channel {
		case transport.ChannelRPC:
			go handleRPC(ctx, f, w, d, logger)
		case transport.ChannelLSP:
			logger.Printf("lsp frame received but LSP server is not yet wired (Phase B); dropping %d bytes", len(f.Body))
		default:
			logger.Printf("unknown channel %q, dropping", f.Channel)
		}
	}
}

func handleRPC(ctx context.Context, f *transport.Frame, w *transport.FrameWriter, d *rpc.Dispatcher, logger *log.Logger) {
	respBytes, err := d.Dispatch(ctx, f.Body)
	if err != nil {
		logger.Printf("dispatch internal error: %v", err)
		return
	}
	if respBytes == nil {
		return
	}
	if err := w.Write(&transport.Frame{Channel: transport.ChannelRPC, Body: respBytes}); err != nil {
		logger.Printf("write response: %v", err)
	}
}

func sendNotification(w *transport.FrameWriter, method string, params any) error {
	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return err
	}
	body, err := json.Marshal(&rpc.Notification{
		Version: rpc.Version,
		Method:  method,
		Params:  paramsBytes,
	})
	if err != nil {
		return err
	}
	return w.Write(&transport.Frame{Channel: transport.ChannelRPC, Body: body})
}
