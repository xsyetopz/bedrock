// Claude Code writes its plugin API types only beside a loaded plugin, so this module declares the part it uses, and `claude plugin validate` checks the calls against the real API.
export interface ProcessResult {
	readonly exitCode: number;
	readonly stdout: string;
	readonly stderr: string;
}

export interface Engine {
	readonly plugin: { readonly name: string; readonly root: string };
	readonly command: {
		readonly register: (command: {
			readonly name: string;
			readonly description: string;
		}) => Promise<unknown>;
	};
	readonly fs: { readonly exists: (path: string) => Promise<boolean> };
	readonly process: {
		readonly run: (
			argv: readonly string[],
			init: { readonly timeoutMs: number }
		) => Promise<ProcessResult>;
	};
	readonly prompt: {
		readonly read: () => Promise<{ readonly text: string }>;
		readonly fill: (input: {
			readonly text: string;
		}) => Promise<{ readonly isFilled: boolean }>;
	};
	readonly session: { readonly cwd: () => Promise<string> };
}

export interface CommandRun {
	readonly command: string;
	readonly args: string;
	readonly origin: { readonly kind: string };
}

export type Next<E> = (e: E) => Promise<unknown>;
export type Hook<E> = ($: Engine, e: E, next: Next<E>) => unknown;
export interface BashCall {
	readonly command: string;
}

export interface On {
	(event: "session.start", hook: Hook<unknown>): unknown;
	(
		event: "command.run",
		matcher: { readonly command: "adopt" | "bedrock" },
		hook: Hook<CommandRun>
	): unknown;
	(
		event: "tool.call",
		matcher: { readonly tool: "Bash" },
		hook: Hook<BashCall>
	): unknown;
}
