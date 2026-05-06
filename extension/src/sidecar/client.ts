import { ChannelRPC, Frame } from "./framing";
import { SidecarProcess } from "./process";
import { Logger } from "../log";

interface PendingRequest {
  resolve: (result: unknown) => void;
  reject: (err: Error) => void;
}

interface RpcRequest {
  jsonrpc: "2.0";
  id?: number;
  method: string;
  params?: unknown;
}

interface RpcResponse {
  jsonrpc: "2.0";
  id: number | string | null;
  result?: unknown;
  error?: { code: number; message: string; data?: unknown };
}

export type NotificationHandler = (method: string, params: unknown) => void;

/**
 * JSON-RPC 2.0 client for the custom RPC channel. LSP frames (no Channel
 * header) are ignored here — vscode-languageclient handles them in Phase B.
 */
export class RpcClient {
  private nextId = 1;
  private pending = new Map<number, PendingRequest>();
  private notifHandler: NotificationHandler | null = null;

  constructor(private process: SidecarProcess, private logger: Logger) {
    this.process.on("frame", (frame: Frame) => this.handleFrame(frame));
    this.process.on("exit", () => this.rejectAll(new Error("sidecar exited")));
  }

  onNotification(handler: NotificationHandler): void {
    this.notifHandler = handler;
  }

  request<T = unknown>(method: string, params?: unknown): Promise<T> {
    const id = this.nextId++;
    const req: RpcRequest = { jsonrpc: "2.0", id, method, params };
    const body = Buffer.from(JSON.stringify(req), "utf8");
    return new Promise<T>((resolve, reject) => {
      this.pending.set(id, {
        resolve: resolve as (r: unknown) => void,
        reject,
      });
      try {
        this.process.send({ channel: ChannelRPC, body });
      } catch (err) {
        this.pending.delete(id);
        reject(err as Error);
      }
    });
  }

  notify(method: string, params?: unknown): void {
    const req: RpcRequest = { jsonrpc: "2.0", method, params };
    const body = Buffer.from(JSON.stringify(req), "utf8");
    this.process.send({ channel: ChannelRPC, body });
  }

  private handleFrame(frame: Frame): void {
    if (frame.channel !== ChannelRPC) return;

    let msg: RpcResponse | RpcRequest;
    try {
      msg = JSON.parse(frame.body.toString("utf8"));
    } catch (err) {
      this.logger.error(`rpc: bad json: ${err}`);
      return;
    }

    const resp = msg as RpcResponse;
    if (
      typeof resp.id === "number" &&
      (resp.result !== undefined || resp.error !== undefined)
    ) {
      const p = this.pending.get(resp.id);
      if (!p) {
        this.logger.warn(`rpc: unknown response id ${resp.id}`);
        return;
      }
      this.pending.delete(resp.id);
      if (resp.error) {
        p.reject(new Error(`rpc error ${resp.error.code}: ${resp.error.message}`));
      } else {
        p.resolve(resp.result);
      }
      return;
    }

    const req = msg as RpcRequest;
    if (typeof req.method === "string" && req.id === undefined) {
      this.notifHandler?.(req.method, req.params);
      return;
    }

    this.logger.warn(`rpc: unrecognized message: ${frame.body.toString("utf8")}`);
  }

  private rejectAll(err: Error): void {
    for (const p of this.pending.values()) {
      p.reject(err);
    }
    this.pending.clear();
  }
}
