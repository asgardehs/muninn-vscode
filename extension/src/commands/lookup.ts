import * as path from "path";
import * as vscode from "vscode";
import { RpcClient } from "../sidecar/client";
import { SidecarState } from "../sidecar/state";

interface LookupMatch {
  name: string;
  path?: string;
  title?: string;
  exists: boolean;
  isStub: boolean;
  score: number;
}

interface LookupResponse {
  matches: LookupMatch[];
}

interface CreateResponse {
  name: string;
  path: string;
  absPath: string;
  created: boolean;
}

interface LookupOptions {
  /** Prefill the quick-pick value (used by lookupHere). */
  prefill?: string;
}

/**
 * Dendron-style lookup palette: as the user types a dot-path, fuzzy-match
 * against the hierarchy. Selecting an existing note opens it; selecting a
 * stub or a never-before-seen name calls vault/createFromHierarchy.
 *
 * The "Create" entry is synthesized from the typed value when it doesn't
 * exactly match an existing note name — this is what lets the palette double
 * as the create flow.
 */
export async function runLookup(
  client: RpcClient,
  state: SidecarState,
  log: (msg: string) => void,
  options: LookupOptions = {}
): Promise<void> {
  const qp = vscode.window.createQuickPick<vscode.QuickPickItem & { match?: LookupMatch }>();
  qp.placeholder = "Lookup or create note (dot.path.notation)";
  qp.value = options.prefill ?? "";
  qp.matchOnDescription = false;
  qp.matchOnDetail = false;
  // The sidecar already fuzzy-matches; keep VS Code's filter off so its order
  // doesn't fight ours.
  (qp as any).sortByLabel = false;

  let debounce: NodeJS.Timeout | null = null;
  let inFlight = 0;

  const refresh = async (query: string) => {
    inFlight++;
    const myCall = inFlight;
    qp.busy = true;
    try {
      const result = await client.request<LookupResponse>("vault/lookup", {
        query,
        limit: 50,
        includeStubs: true,
      });
      // Drop late responses if the user has typed more.
      if (myCall !== inFlight) return;

      const items = result.matches.map<vscode.QuickPickItem & { match?: LookupMatch }>((m) => ({
        label: m.name,
        description: m.exists
          ? m.title ?? ""
          : "(stub — will be created)",
        detail: m.exists ? m.path : undefined,
        alwaysShow: true,
        match: m,
      }));

      // If the typed value isn't one of the matches and isn't empty, prepend
      // an explicit "Create" entry so the user can always create what they
      // typed regardless of the fuzzy results.
      const trimmed = query.trim();
      const exact = result.matches.find((m) => m.name === trimmed);
      if (trimmed && !exact) {
        items.unshift({
          label: trimmed,
          description: "Create new note",
          alwaysShow: true,
          iconPath: new vscode.ThemeIcon("add"),
          match: { name: trimmed, exists: false, isStub: true, score: 0 },
        });
      }
      qp.items = items;
    } catch (err) {
      log(`lookup failed: ${err}`);
      qp.items = [];
    } finally {
      if (myCall === inFlight) qp.busy = false;
    }
  };

  qp.onDidChangeValue((value) => {
    if (debounce) clearTimeout(debounce);
    debounce = setTimeout(() => void refresh(value), 80);
  });

  qp.onDidAccept(async () => {
    const sel = qp.selectedItems[0];
    if (!sel || !sel.match) {
      qp.hide();
      return;
    }
    qp.hide();
    await openOrCreate(client, state, log, sel.match);
  });

  qp.onDidHide(() => qp.dispose());
  qp.show();
  await refresh(qp.value);
}

async function openOrCreate(
  client: RpcClient,
  state: SidecarState,
  log: (msg: string) => void,
  match: LookupMatch
): Promise<void> {
  if (match.exists && match.path) {
    const vaultPath = state.vaultPath();
    if (!vaultPath) {
      vscode.window.showErrorMessage("Muninn: vault path unknown — sidecar not ready.");
      return;
    }
    const absPath = path.join(vaultPath, match.path);
    await openInEditor(absPath);
    return;
  }

  // Create-and-open path.
  try {
    const created = await client.request<CreateResponse>("vault/createFromHierarchy", {
      name: match.name,
      openAfterCreate: true,
    });
    log(`created ${created.path}`);
    await openInEditor(created.absPath);
  } catch (err) {
    vscode.window.showErrorMessage(`Muninn: failed to create ${match.name}: ${err}`);
  }
}

async function openInEditor(absPath: string): Promise<void> {
  const doc = await vscode.workspace.openTextDocument(vscode.Uri.file(absPath));
  await vscode.window.showTextDocument(doc);
}

/**
 * lookupHere: like runLookup, but prefills the quick-pick with the parent
 * dot-path of the active note (e.g., editing "foo.bar.baz" prefills "foo.bar.").
 */
export function lookupHerePrefill(activeFileName: string | undefined): string {
  if (!activeFileName) return "";
  const base = path.basename(activeFileName, ".md");
  const lastDot = base.lastIndexOf(".");
  if (lastDot < 0) return "";
  return base.slice(0, lastDot + 1);
}
