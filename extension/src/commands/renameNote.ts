import * as path from "path";
import * as vscode from "vscode";
import { RpcClient } from "../sidecar/client";
import { SidecarState } from "../sidecar/state";

interface RenameResponse {
  oldName: string;
  newName: string;
  renamedFile: string;
  editedFiles: number;
}

/**
 * Rename the active note. Computes the current hierarchy name from the active
 * file, prompts for the new name, then calls vault/renameNote which updates
 * filename and all backlinks atomically. v0.2.0: command-palette only.
 */
export async function runRenameNote(
  client: RpcClient,
  state: SidecarState,
  log: (msg: string) => void
): Promise<void> {
  const editor = vscode.window.activeTextEditor;
  if (!editor || !editor.document.fileName.endsWith(".md")) {
    vscode.window.showWarningMessage("Muninn: open a markdown note to rename.");
    return;
  }
  const vaultPath = state.vaultPath();
  if (!vaultPath) {
    vscode.window.showErrorMessage("Muninn: vault path unknown — sidecar not ready.");
    return;
  }

  const rel = path.relative(vaultPath, editor.document.fileName);
  if (rel.startsWith("..")) {
    vscode.window.showWarningMessage("Muninn: active file is not inside the vault.");
    return;
  }
  const oldName = rel.replace(/\.md$/, "");

  const newName = await vscode.window.showInputBox({
    prompt: `Rename ${oldName} to`,
    value: oldName,
    valueSelection: [oldName.lastIndexOf(".") + 1, oldName.length],
    validateInput: (v) => {
      const trimmed = v.trim();
      if (!trimmed) return "Name is required.";
      if (trimmed === oldName) return "New name is the same as the old name.";
      if (!/^[A-Za-z0-9._-]+$/.test(trimmed)) return "Allowed: letters, digits, dot, hyphen, underscore.";
      if (trimmed.startsWith(".") || trimmed.endsWith(".")) return "Name cannot start or end with '.'.";
      if (trimmed.includes("..")) return "Name cannot contain consecutive dots.";
      return null;
    },
  });
  if (!newName) return;

  try {
    const res = await client.request<RenameResponse>("vault/renameNote", {
      oldName,
      newName: newName.trim(),
    });
    log(`renamed ${res.oldName} -> ${res.newName} (${res.editedFiles} backlink(s) updated)`);

    // Reopen the renamed file so the user's editor follows the rename.
    const newAbs = path.join(vaultPath, res.renamedFile);
    const doc = await vscode.workspace.openTextDocument(vscode.Uri.file(newAbs));
    await vscode.window.showTextDocument(doc);
    vscode.window.showInformationMessage(
      `Muninn: renamed to ${res.newName} (${res.editedFiles} backlink${res.editedFiles === 1 ? "" : "s"} updated).`
    );
  } catch (err) {
    vscode.window.showErrorMessage(`Muninn rename failed: ${err}`);
  }
}
