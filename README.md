# Muninn

A hierarchical notes VS Code extension. Dot-path note names, fuzzy lookup, schema-driven creation, wikilinks with backlinks.

Heavy lifting in a Go sidecar; thin TypeScript glue for VS Code.

> **Status:** pre-release. v0.1 in development.

## Architecture

- `sidecar/` — Go binary. Vault I/O, wikilink index, hierarchy tree, schema engine, LSP server, custom JSON-RPC server. One binary, two protocols on a single multiplexed stdio pipe.
- `extension/` — VS Code extension (TypeScript). Spawns the sidecar, hosts UI commands and tree views, speaks both LSP (via `vscode-languageclient`) and the custom JSON-RPC channel.
- `shared/` — example schema packs (`generic`, `ehs`) and note templates shipped with the extension.

## Development

Requirements: Go 1.26+, Node 22+, VS Code 1.95+.

```sh
# Build the sidecar
cd sidecar && go build -o dist/muninn-sidecar ./cmd/muninn-sidecar

# Compile the extension
cd extension && npm install && npm run compile

# Open the repo in VS Code, press F5 to launch the dev host.
```

## License

GPL-3.0. See [LICENSE](LICENSE).
