import * as path from "path";
import * as vscode from "vscode";
import { LanguageClient, LanguageClientOptions } from "vscode-languageclient/node";
import { Logger } from "./log";
import { SidecarProcess } from "./sidecar/process";
import { RpcClient } from "./sidecar/client";
import { SidecarMessageReader, SidecarMessageWriter } from "./sidecar/lspBridge";
import { SidecarState } from "./sidecar/state";
import { HierarchyTreeProvider } from "./views/hierarchyTreeProvider";
import { lookupHerePrefill, runLookup } from "./commands/lookup";
import { runInstallSchemaPack } from "./commands/installSchemaPack";

let sidecar: SidecarProcess | null = null;
let logger: Logger | null = null;
let languageClient: LanguageClient | null = null;

export async function activate(context: vscode.ExtensionContext): Promise<void> {
  logger = new Logger("Muninn");
  context.subscriptions.push({ dispose: () => logger?.dispose() });

  logger.info("activating extension");

  // Register commands unconditionally so users always get a useful error
  // message instead of "command not found" when no workspace is open.
  // The sidecar (and therefore the client) is conditional on a workspace.
  let client: RpcClient | null = null;
  let state: SidecarState | null = null;
  let treeProvider: HierarchyTreeProvider | null = null;

  const requireSidecar = (action: string): boolean => {
    if (!client) {
      vscode.window.showWarningMessage(
        `Muninn: open a folder first — ${action} needs an active sidecar.`
      );
      return false;
    }
    return true;
  };

  context.subscriptions.push(
    vscode.commands.registerCommand("muninn.ping", async () => {
      if (!requireSidecar("ping") || !client) return;
      try {
        const result = await client.request("rpc/ping");
        vscode.window.showInformationMessage(`Muninn ping: ${JSON.stringify(result)}`);
      } catch (err) {
        vscode.window.showErrorMessage(`Muninn ping failed: ${err}`);
      }
    }),
    vscode.commands.registerCommand("muninn.showSidecarLogs", () => {
      logger?.show();
    }),
    vscode.commands.registerCommand("muninn.lookup", async () => {
      if (!requireSidecar("lookup") || !client || !state || !logger) return;
      await runLookup(client, state, (m) => logger?.info(m));
    }),
    vscode.commands.registerCommand("muninn.lookupHere", async () => {
      if (!requireSidecar("lookupHere") || !client || !state || !logger) return;
      const active = vscode.window.activeTextEditor?.document.fileName;
      const prefill = lookupHerePrefill(active);
      await runLookup(client, state, (m) => logger?.info(m), { prefill });
    }),
    vscode.commands.registerCommand("muninn.refreshVault", async () => {
      if (!requireSidecar("refreshVault") || !client) return;
      try {
        const res = await client.request<{ noteCount: number }>("vault/refresh");
        vscode.window.showInformationMessage(`Muninn: refreshed ${res.noteCount} notes.`);
        treeProvider?.refresh();
      } catch (err) {
        vscode.window.showErrorMessage(`Muninn refresh failed: ${err}`);
      }
    }),
    vscode.commands.registerCommand("muninn.installSchemaPack", async () => {
      if (!requireSidecar("installSchemaPack") || !client || !state || !logger) return;
      await runInstallSchemaPack(client, state, (m) => logger?.info(m));
    })
  );

  const workspaceFolder = vscode.workspace.workspaceFolders?.[0];
  if (!workspaceFolder) {
    logger.warn("no workspace folder open; sidecar will not be started");
    return;
  }

  const binaryPath = resolveSidecarPath(context, logger);
  logger.info(`sidecar binary: ${binaryPath}`);

  sidecar = new SidecarProcess({
    binaryPath,
    workspacePath: workspaceFolder.uri.fsPath,
    logger,
  });

  client = new RpcClient(sidecar, logger);
  state = new SidecarState();
  client.onNotification((method, params) => {
    logger?.info(`notification ${method}: ${JSON.stringify(params)}`);
    if (method === "sidecar/ready") {
      state?.setReady((params ?? {}) as { vaultPath?: string });
    } else if (method === "vault/changed") {
      treeProvider?.refresh();
    }
  });

  treeProvider = new HierarchyTreeProvider(client, state);
  context.subscriptions.push(
    vscode.window.registerTreeDataProvider("muninn.hierarchyView", treeProvider)
  );

  sidecar.start();

  const reader = new SidecarMessageReader(sidecar);
  const writer = new SidecarMessageWriter(sidecar);
  const clientOptions: LanguageClientOptions = {
    documentSelector: [{ scheme: "file", language: "markdown" }],
    outputChannel: vscode.window.createOutputChannel("Muninn LSP"),
    initializationOptions: {
      "diagnostics.unresolvedLinks": vscode.workspace
        .getConfiguration("muninn")
        .get<boolean>("diagnostics.unresolvedLinks", true),
    },
  };
  languageClient = new LanguageClient(
    "muninn-lsp",
    "Muninn LSP",
    () => Promise.resolve({ reader, writer }),
    clientOptions
  );
  context.subscriptions.push({ dispose: () => reader.dispose() });
  context.subscriptions.push({ dispose: () => writer.dispose() });
  await languageClient.start();
  logger.info("language client started");

  const startupClient = client;
  setTimeout(async () => {
    try {
      const result = await startupClient.request("rpc/ping");
      logger?.info(`startup ping ok: ${JSON.stringify(result)}`);
    } catch (err) {
      logger?.error(`startup ping failed: ${err}`);
    }
  }, 250);
}

export async function deactivate(): Promise<void> {
  logger?.info("deactivating extension");
  if (languageClient) {
    try {
      await languageClient.stop(2000);
    } catch (err) {
      logger?.warn(`language client stop: ${err}`);
    }
    languageClient = null;
  }
  sidecar?.stop();
  sidecar = null;
}

function resolveSidecarPath(context: vscode.ExtensionContext, log: Logger): string {
  const configPath = vscode.workspace
    .getConfiguration("muninn")
    .get<string>("sidecarPath");
  if (configPath && configPath.trim() !== "") {
    log.info(`using muninn.sidecarPath from settings: ${configPath}`);
    return configPath;
  }
  const envPath = process.env.MUNINN_SIDECAR_PATH;
  if (envPath && envPath.trim() !== "") {
    log.info(`using MUNINN_SIDECAR_PATH env: ${envPath}`);
    return envPath;
  }
  const exe = process.platform === "win32" ? "muninn-sidecar.exe" : "muninn-sidecar";
  return path.join(context.extensionPath, "bin", exe);
}
