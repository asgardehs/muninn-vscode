import * as path from "path";
import * as vscode from "vscode";
import { Logger } from "./log";
import { SidecarProcess } from "./sidecar/process";
import { RpcClient } from "./sidecar/client";

let sidecar: SidecarProcess | null = null;
let logger: Logger | null = null;

export async function activate(context: vscode.ExtensionContext): Promise<void> {
  logger = new Logger("Muninn");
  context.subscriptions.push({ dispose: () => logger?.dispose() });

  logger.info("activating extension");

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

  const client = new RpcClient(sidecar, logger);
  client.onNotification((method, params) => {
    logger?.info(`notification ${method}: ${JSON.stringify(params)}`);
  });

  sidecar.start();

  context.subscriptions.push(
    vscode.commands.registerCommand("muninn.ping", async () => {
      try {
        const result = await client.request("rpc/ping");
        vscode.window.showInformationMessage(`Muninn ping: ${JSON.stringify(result)}`);
      } catch (err) {
        vscode.window.showErrorMessage(`Muninn ping failed: ${err}`);
      }
    }),
    vscode.commands.registerCommand("muninn.showSidecarLogs", () => {
      logger?.show();
    })
  );

  setTimeout(async () => {
    try {
      const result = await client.request("rpc/ping");
      logger?.info(`startup ping ok: ${JSON.stringify(result)}`);
    } catch (err) {
      logger?.error(`startup ping failed: ${err}`);
    }
  }, 250);
}

export function deactivate(): void {
  logger?.info("deactivating extension");
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
