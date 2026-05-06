import * as path from "path";
import * as vscode from "vscode";
import { RpcClient } from "../sidecar/client";
import { SidecarState } from "../sidecar/state";

interface HierarchyNode {
  name: string;
  path?: string;
  isStub: boolean;
  children: number;
}

interface ListResponse {
  nodes: HierarchyNode[];
}

/**
 * Lazy-loaded tree view backed by the sidecar's vault/listRoots and
 * vault/listChildren RPC methods. Stubs (synthesized parents with no .md file)
 * are rendered italic and don't open on click.
 */
export class HierarchyTreeProvider implements vscode.TreeDataProvider<HierarchyNode> {
  private _onDidChangeTreeData = new vscode.EventEmitter<HierarchyNode | undefined | void>();
  readonly onDidChangeTreeData = this._onDidChangeTreeData.event;

  constructor(
    private client: RpcClient,
    private state: SidecarState
  ) {}

  refresh(): void {
    this._onDidChangeTreeData.fire();
  }

  getTreeItem(node: HierarchyNode): vscode.TreeItem {
    const collapsibleState =
      node.children > 0
        ? vscode.TreeItemCollapsibleState.Collapsed
        : vscode.TreeItemCollapsibleState.None;
    const item = new vscode.TreeItem(node.name, collapsibleState);
    item.id = node.name;
    item.contextValue = node.isStub ? "muninnStub" : "muninnNote";
    item.iconPath = node.isStub
      ? new vscode.ThemeIcon("symbol-namespace")
      : new vscode.ThemeIcon("file");
    if (node.isStub) {
      item.description = "(stub)";
    }

    if (!node.isStub && node.path) {
      const vaultPath = this.state.vaultPath();
      if (vaultPath) {
        const absPath = path.join(vaultPath, node.path);
        item.resourceUri = vscode.Uri.file(absPath);
        item.command = {
          command: "vscode.open",
          title: "Open Note",
          arguments: [vscode.Uri.file(absPath)],
        };
      }
    }
    return item;
  }

  async getChildren(node?: HierarchyNode): Promise<HierarchyNode[]> {
    try {
      if (!node) {
        const res = await this.client.request<ListResponse>("vault/listRoots");
        return res.nodes;
      }
      const res = await this.client.request<ListResponse>("vault/listChildren", {
        name: node.name,
      });
      return res.nodes;
    } catch {
      return [];
    }
  }
}
