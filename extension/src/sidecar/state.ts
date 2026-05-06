/**
 * Captures sidecar lifecycle state that the extension needs at runtime —
 * primarily the resolved vault path from the sidecar/ready notification, so
 * commands and the tree view can build absolute file URIs without re-asking.
 */
export class SidecarState {
  private _vaultPath: string | null = null;
  private _ready = false;
  private readyResolvers: Array<() => void> = [];

  setReady(payload: { vaultPath?: string }): void {
    if (typeof payload.vaultPath === "string") {
      this._vaultPath = payload.vaultPath;
    }
    this._ready = true;
    const resolvers = this.readyResolvers;
    this.readyResolvers = [];
    for (const r of resolvers) r();
  }

  vaultPath(): string | null {
    return this._vaultPath;
  }

  isReady(): boolean {
    return this._ready;
  }

  /** Returns a promise that resolves when sidecar/ready arrives. */
  whenReady(): Promise<void> {
    if (this._ready) return Promise.resolve();
    return new Promise((resolve) => this.readyResolvers.push(resolve));
  }
}
