export const ChannelLSP = "lsp";
export const ChannelRPC = "rpc";

export interface Frame {
  channel: string;
  body: Buffer;
}

/**
 * Streaming parser for LSP-style framed messages with an optional Channel
 * header. Frames without a Channel header default to ChannelLSP, matching the
 * Go side's behavior — this lets vscode-languageclient interoperate with the
 * same transport.
 */
export class FrameParser {
  private buffer: Buffer = Buffer.alloc(0);

  push(chunk: Buffer): Frame[] {
    this.buffer = Buffer.concat([this.buffer, chunk]);
    const out: Frame[] = [];
    while (true) {
      const headerEnd = this.buffer.indexOf("\r\n\r\n");
      if (headerEnd === -1) break;

      const header = this.buffer.subarray(0, headerEnd).toString("utf8");
      let contentLength = -1;
      let channel = ChannelLSP;
      for (const line of header.split("\r\n")) {
        const idx = line.indexOf(":");
        if (idx === -1) continue;
        const key = line.slice(0, idx).trim().toLowerCase();
        const value = line.slice(idx + 1).trim();
        if (key === "content-length") {
          contentLength = parseInt(value, 10);
        } else if (key === "channel") {
          channel = value.toLowerCase();
        }
      }
      if (contentLength < 0) {
        throw new Error("missing Content-Length header");
      }

      const totalLen = headerEnd + 4 + contentLength;
      if (this.buffer.length < totalLen) break;

      const body = Buffer.from(this.buffer.subarray(headerEnd + 4, totalLen));
      out.push({ channel, body });
      this.buffer = this.buffer.subarray(totalLen);
    }
    return out;
  }
}

export function writeFrame(frame: Frame): Buffer {
  let header = `Content-Length: ${frame.body.length}\r\n`;
  if (frame.channel && frame.channel !== ChannelLSP) {
    header += `Channel: ${frame.channel}\r\n`;
  }
  header += "\r\n";
  return Buffer.concat([Buffer.from(header, "utf8"), frame.body]);
}
