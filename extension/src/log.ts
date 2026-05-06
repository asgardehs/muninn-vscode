import * as vscode from "vscode";

export class Logger {
  private channel: vscode.OutputChannel;

  constructor(name: string) {
    this.channel = vscode.window.createOutputChannel(name);
  }

  info(msg: string): void {
    this.channel.appendLine(`[info]  ${ts()} ${msg}`);
  }

  warn(msg: string): void {
    this.channel.appendLine(`[warn]  ${ts()} ${msg}`);
  }

  error(msg: string): void {
    this.channel.appendLine(`[error] ${ts()} ${msg}`);
  }

  show(): void {
    this.channel.show();
  }

  dispose(): void {
    this.channel.dispose();
  }
}

function ts(): string {
  return new Date().toISOString();
}
