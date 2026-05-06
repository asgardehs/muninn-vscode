import { ChildProcess, spawn } from "child_process";
import { EventEmitter } from "events";
import { Frame, FrameParser, writeFrame } from "./framing";
import { Logger } from "../log";

export interface SidecarOptions {
  binaryPath: string;
  workspacePath: string;
  logger: Logger;
}

/**
 * Owns the lifecycle of the muninn-sidecar child process. Emits "frame" for
 * every parsed framed message read from stdout, "exit" when the process ends,
 * and "error" on spawn failure.
 */
export class SidecarProcess extends EventEmitter {
  private child: ChildProcess | null = null;
  private parser = new FrameParser();

  constructor(private opts: SidecarOptions) {
    super();
  }

  start(): void {
    if (this.child) {
      throw new Error("sidecar already running");
    }
    const child = spawn(
      this.opts.binaryPath,
      ["--workspace", this.opts.workspacePath],
      { stdio: ["pipe", "pipe", "pipe"] }
    );
    this.child = child;

    child.stdout!.on("data", (chunk: Buffer) => {
      try {
        for (const f of this.parser.push(chunk)) {
          this.emit("frame", f);
        }
      } catch (err) {
        this.opts.logger.error(`frame parse error: ${err}`);
      }
    });

    child.stderr!.on("data", (chunk: Buffer) => {
      const text = chunk.toString().trimEnd();
      if (text.length > 0) {
        this.opts.logger.info(`[sidecar] ${text}`);
      }
    });

    child.on("exit", (code, signal) => {
      this.opts.logger.warn(`sidecar exited code=${code} signal=${signal}`);
      this.child = null;
      this.emit("exit", { code, signal });
    });

    child.on("error", (err) => {
      this.opts.logger.error(`sidecar process error: ${err.message}`);
      this.emit("error", err);
    });
  }

  send(frame: Frame): void {
    if (!this.child || !this.child.stdin) {
      throw new Error("sidecar not running");
    }
    this.child.stdin.write(writeFrame(frame));
  }

  stop(): void {
    if (this.child) {
      this.child.kill("SIGTERM");
    }
  }
}
