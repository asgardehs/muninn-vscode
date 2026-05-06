import * as path from "path";
import * as vscode from "vscode";
import { RpcClient } from "../sidecar/client";
import { SidecarState } from "../sidecar/state";

interface ListPacksResponse {
  packs: string[];
}

interface ExportPackResponse {
  files: Record<string, string>;
}

/**
 * installSchemaPack lets the user copy a bundled schema pack (generic, ehs)
 * into <vault>/.muninn/schemas/. The bundled YAML lives inside the sidecar
 * binary; the extension fetches it via schema/exportPack and writes it to
 * disk on the user's behalf so they don't need to download anything.
 *
 * If a destination file already exists the user is asked once whether to
 * overwrite all conflicts, skip them, or cancel the install.
 */
export async function runInstallSchemaPack(
  client: RpcClient,
  state: SidecarState,
  log: (msg: string) => void
): Promise<void> {
  const vaultPath = state.vaultPath();
  if (!vaultPath) {
    vscode.window.showWarningMessage("Muninn: vault path unknown — sidecar not ready.");
    return;
  }

  let listResp: ListPacksResponse;
  try {
    listResp = await client.request<ListPacksResponse>("schema/listPacks");
  } catch (err) {
    vscode.window.showErrorMessage(`Muninn: failed to list packs: ${err}`);
    return;
  }

  if (listResp.packs.length === 0) {
    vscode.window.showInformationMessage("Muninn: no schema packs are bundled.");
    return;
  }

  const pickItems = listResp.packs.map<vscode.QuickPickItem>((p) => ({
    label: p,
    description: packDescription(p),
  }));
  const chosen = await vscode.window.showQuickPick(pickItems, {
    title: "Install a schema pack into this vault",
    placeHolder: "Select a pack to copy into .muninn/schemas/",
  });
  if (!chosen) return;

  let exportResp: ExportPackResponse;
  try {
    exportResp = await client.request<ExportPackResponse>("schema/exportPack", {
      pack: chosen.label,
    });
  } catch (err) {
    vscode.window.showErrorMessage(`Muninn: failed to export pack ${chosen.label}: ${err}`);
    return;
  }

  const files = Object.entries(exportResp.files);
  if (files.length === 0) {
    vscode.window.showInformationMessage(`Muninn: pack ${chosen.label} is empty.`);
    return;
  }

  const destDir = vscode.Uri.file(path.join(vaultPath, ".muninn", "schemas"));
  await vscode.workspace.fs.createDirectory(destDir);

  // Detect conflicts up-front so we can ask once.
  const conflicts: string[] = [];
  for (const [name] of files) {
    const target = vscode.Uri.joinPath(destDir, name);
    try {
      await vscode.workspace.fs.stat(target);
      conflicts.push(name);
    } catch {
      // Doesn't exist — no conflict.
    }
  }

  let overwrite = false;
  if (conflicts.length > 0) {
    const choice = await vscode.window.showWarningMessage(
      `Muninn: ${conflicts.length} schema file${conflicts.length === 1 ? "" : "s"} already exist in this vault (${conflicts.join(", ")}). Overwrite?`,
      { modal: true },
      "Overwrite all",
      "Skip conflicts"
    );
    if (choice === undefined) return;
    overwrite = choice === "Overwrite all";
  }

  let written = 0;
  let skipped = 0;
  for (const [name, body] of files) {
    const target = vscode.Uri.joinPath(destDir, name);
    if (conflicts.includes(name) && !overwrite) {
      skipped++;
      continue;
    }
    await vscode.workspace.fs.writeFile(target, Buffer.from(body, "utf8"));
    written++;
  }

  // Reload the sidecar's registry so the freshly-written schemas take effect
  // without requiring the user to restart anything.
  try {
    await client.request<{ schemaCount: number }>("schema/reload");
  } catch (err) {
    log(`schema/reload failed (sidecar may need restart): ${err}`);
  }

  log(`installed pack ${chosen.label}: ${written} written, ${skipped} skipped`);
  vscode.window.showInformationMessage(
    `Muninn: installed ${chosen.label} (${written} written${skipped > 0 ? `, ${skipped} skipped` : ""}).`
  );
}

function packDescription(pack: string): string {
  switch (pack) {
    case "generic":
      return "5 starter schemas: daily, meeting, reference, decision, til";
    case "ehs":
      return "5 EHS schemas: incident, JHA, inspection, training, audit";
    default:
      return "";
  }
}
