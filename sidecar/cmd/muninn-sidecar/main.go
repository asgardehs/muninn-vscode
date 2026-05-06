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
	"github.com/asgardehs/muninn-sidecar/internal/schema"
	"github.com/asgardehs/muninn-sidecar/internal/transport"
	"github.com/asgardehs/muninn-sidecar/internal/vault"
	"github.com/asgardehs/muninn-sidecar/internal/wikilink"
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
	lspServer.BuildLinkIndex()

	schemas, err := schema.Load(vaultPath)
	if err != nil {
		logger.Printf("schema load failed (continuing without schemas): %v", err)
	} else {
		lspServer.SetSchemas(schemas)
		logger.Printf("loaded %d schemas", schemas.Len())
	}

	dispatcher := rpc.NewDispatcher(logger)
	dispatcher.Register("rpc/ping", rpc.HandlePing(version))
	registerVaultHandlers(dispatcher, lspServer)
	registerSchemaHandlers(dispatcher, lspServer)

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

	// Filesystem watcher: emit vault/changed notifications and keep the
	// wikilink index in sync with external edits.
	watcherDone := make(chan struct{})
	go func() {
		defer close(watcherDone)
		runWatcher(ctx, lspServer, mux.Writer(), logger)
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
	<-watcherDone
	logger.Printf("clean shutdown")
}

// runWatcher consumes vault.Change events from the filesystem watcher and
// dispatches based on whether the change was a note edit or a schema edit.
//   - Note changes: update the wikilink index, refresh diagnostics, emit a
//     vault/changed notification.
//   - Schema changes: reload the registry, refresh diagnostics, emit a
//     schema/changed notification.
func runWatcher(ctx context.Context, lspServer *lsp.Server, fw *transport.FrameWriter, logger *log.Logger) {
	v := lspServer.Vault()
	idx := lspServer.LinkIndex()

	changes, err := v.Watch(ctx, logger)
	if err != nil {
		logger.Printf("watcher: failed to start: %v", err)
		return
	}

	for change := range changes {
		switch change.Source {
		case vault.SourceNote:
			handleNoteChange(ctx, change, v, idx, lspServer, fw, logger)
		case vault.SourceSchema:
			handleSchemaChange(ctx, change, lspServer, fw, logger)
		}
	}
}

func handleNoteChange(ctx context.Context, change vault.Change, v *vault.Vault, idx *wikilink.Index, lspServer *lsp.Server, fw *transport.FrameWriter, logger *log.Logger) {
	switch change.Kind {
	case vault.ChangeDelete:
		idx.Remove(change.RelPath)
	case vault.ChangeCreate, vault.ChangeModify:
		content, err := v.ReadNote(change.RelPath)
		if err != nil {
			logger.Printf("watcher: read %q: %v", change.RelPath, err)
			return
		}
		idx.Update(change.RelPath, wikilink.Extract(content))
	}
	lspServer.RefreshOpenDiagnostics(ctx)
	if err := sendNotification(fw, "vault/changed", map[string]any{
		"paths": []string{change.RelPath},
		"kind":  string(change.Kind),
	}); err != nil {
		logger.Printf("watcher: failed to emit vault/changed: %v", err)
	}
}

func handleSchemaChange(ctx context.Context, change vault.Change, lspServer *lsp.Server, fw *transport.FrameWriter, logger *log.Logger) {
	registry, err := schema.Load(lspServer.Vault().Root())
	if err != nil {
		logger.Printf("watcher: schema reload failed: %v", err)
		return
	}
	lspServer.SetSchemas(registry)
	lspServer.RefreshOpenDiagnostics(ctx)
	logger.Printf("watcher: reloaded %d schemas after %s %s", registry.Len(), change.Kind, change.RelPath)
	if err := sendNotification(fw, "schema/changed", map[string]any{
		"paths":       []string{change.RelPath},
		"kind":        string(change.Kind),
		"schemaCount": registry.Len(),
	}); err != nil {
		logger.Printf("watcher: failed to emit schema/changed: %v", err)
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
