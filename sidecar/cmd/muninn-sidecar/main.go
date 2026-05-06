package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/asgardehs/muninn-sidecar/internal/env"
	"github.com/asgardehs/muninn-sidecar/internal/lsp"
	"github.com/asgardehs/muninn-sidecar/internal/rpc"
	"github.com/asgardehs/muninn-sidecar/internal/transport"
	"github.com/asgardehs/muninn-sidecar/internal/vault"
)

const version = "0.0.1"

func main() {
	workspace := flag.String("workspace", "", "vault root path (defaults to current directory)")
	useHeimdall := flag.Bool("use-heimdall", false, "consult Heimdall config for muninn.vault_path")
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

	vaultPath := env.Resolve(*workspace, *useHeimdall)
	logger.Printf("starting v%s, workspace=%s, vault=%s, log-level=%s, heimdall=%v",
		version, *workspace, vaultPath, *logLevel, *useHeimdall)

	v := vault.New(vaultPath)
	lspServer := lsp.New(v)

	dispatcher := rpc.NewDispatcher(logger)
	dispatcher.Register("rpc/ping", rpc.HandlePing(version))

	mux := transport.NewMux(os.Stdin, os.Stdout, logger)

	if err := sendNotification(mux.Writer(), "sidecar/ready", map[string]any{
		"version":      version,
		"capabilities": []string{"lsp", "lookup", "schema"},
		"vaultPath":    vaultPath,
		"workspacePath": *workspace,
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

	// LSP server runs on the multiplexed LSP channel.
	lspStream := lsp.NewMuxStream(mux.LSP(), mux.Writer())
	lspDone := make(chan error, 1)
	go func() {
		lspDone <- lspServer.ServeOn(ctx, lspStream)
	}()

	// RPC dispatcher consumes the RPC channel.
	rpcDone := make(chan struct{})
	go func() {
		defer close(rpcDone)
		for body := range mux.RPC() {
			body := body
			go func() {
				respBytes, err := dispatcher.Dispatch(ctx, body)
				if err != nil {
					logger.Printf("rpc dispatch internal error: %v", err)
					return
				}
				if respBytes == nil {
					return
				}
				if err := mux.Writer().Write(&transport.Frame{
					Channel: transport.ChannelRPC,
					Body:    respBytes,
				}); err != nil {
					logger.Printf("rpc write response: %v", err)
				}
			}()
		}
	}()

	// Run the multiplexer; returns on stdin EOF or ctx cancel.
	if err := mux.Run(ctx); err != nil {
		logger.Printf("mux exited with error: %v", err)
		os.Exit(1)
	}

	cancel()
	_ = lspStream.Close()
	<-rpcDone
	<-lspDone
	logger.Printf("clean shutdown")
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
