import { Disposable, Event, EventEmitter } from "vscode";
import {
  DataCallback,
  Message,
  MessageReader,
  MessageWriter,
  PartialMessageInfo,
} from "vscode-jsonrpc";
import { ChannelLSP, Frame } from "./framing";
import { SidecarProcess } from "./process";

/**
 * MessageReader for vscode-languageclient that pulls LSP frames out of the
 * shared SidecarProcess. Frames whose channel is not "lsp" are ignored —
 * the RpcClient handles those.
 */
export class SidecarMessageReader implements MessageReader {
  private callback: DataCallback | null = null;
  private errorEmitter = new EventEmitter<Error>();
  private closeEmitter = new EventEmitter<void>();
  private partialMessageEmitter = new EventEmitter<PartialMessageInfo>();
  private frameListener: (f: Frame) => void;
  private exitListener: () => void;

  constructor(private sidecar: SidecarProcess) {
    this.frameListener = (frame: Frame) => {
      if (frame.channel !== ChannelLSP || !this.callback) return;
      try {
        const msg = JSON.parse(frame.body.toString("utf8")) as Message;
        this.callback(msg);
      } catch (err) {
        this.errorEmitter.fire(err as Error);
      }
    };
    this.exitListener = () => this.closeEmitter.fire();
    sidecar.on("frame", this.frameListener);
    sidecar.on("exit", this.exitListener);
  }

  get onError(): Event<Error> {
    return this.errorEmitter.event;
  }
  get onClose(): Event<void> {
    return this.closeEmitter.event;
  }
  get onPartialMessage(): Event<PartialMessageInfo> {
    return this.partialMessageEmitter.event;
  }

  listen(callback: DataCallback): Disposable {
    this.callback = callback;
    return { dispose: () => { this.callback = null; } };
  }

  dispose(): void {
    this.sidecar.off("frame", this.frameListener);
    this.sidecar.off("exit", this.exitListener);
    this.errorEmitter.dispose();
    this.closeEmitter.dispose();
    this.partialMessageEmitter.dispose();
  }
}

/**
 * MessageWriter for vscode-languageclient that sends LSP frames through the
 * shared SidecarProcess.
 */
export class SidecarMessageWriter implements MessageWriter {
  private errorEmitter = new EventEmitter<[Error, Message | undefined, number | undefined]>();
  private closeEmitter = new EventEmitter<void>();
  private exitListener: () => void;

  constructor(private sidecar: SidecarProcess) {
    this.exitListener = () => this.closeEmitter.fire();
    sidecar.on("exit", this.exitListener);
  }

  get onError(): Event<[Error, Message | undefined, number | undefined]> {
    return this.errorEmitter.event;
  }
  get onClose(): Event<void> {
    return this.closeEmitter.event;
  }

  async write(msg: Message): Promise<void> {
    const body = Buffer.from(JSON.stringify(msg), "utf8");
    try {
      this.sidecar.send({ channel: ChannelLSP, body });
    } catch (err) {
      this.errorEmitter.fire([err as Error, msg, undefined]);
    }
  }

  end(): void {
    // No-op: stdio lifecycle is owned by SidecarProcess. Closing here would
    // tear down the RPC channel too.
  }

  dispose(): void {
    this.sidecar.off("exit", this.exitListener);
    this.errorEmitter.dispose();
    this.closeEmitter.dispose();
  }
}
