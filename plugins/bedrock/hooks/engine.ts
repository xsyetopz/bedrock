// Claude Code writes its plugin API types only beside a loaded plugin, so this module declares the part it uses, and `claude plugin validate` checks the calls against the real API.
export interface ProcessResult {
	readonly exitCode: number;
	readonly stdout: string;
	readonly stderr: string;
}

export interface Engine {
	readonly plugin: { readonly name: string; readonly root: string };
	readonly tool: {
		readonly register: (tool: {
			readonly name: string;
			readonly description: string;
			readonly inputSchema: Readonly<Record<string, unknown>>;
			readonly isDeferred: boolean;
		}) => Promise<unknown>;
	};
	readonly fs: { readonly exists: (path: string) => Promise<boolean> };
	readonly process: {
		readonly run: (
			argv: readonly string[],
			init: { readonly timeoutMs: number }
		) => Promise<ProcessResult>;
	};
	readonly session: { readonly cwd: () => Promise<string> };
}

export type Next<E> = (e: E) => Promise<unknown>;
export type Hook<E> = ($: Engine, e: E, next: Next<E>) => unknown;
export interface BashCall {
	readonly command: string;
}
/** A call of the plugin's `run` tool: the bedrock command line the model passed. */
export interface BedrockCall {
	readonly args: string;
}

export interface On {
	(event: "session.start", hook: Hook<unknown>): unknown;
	(
		event: "tool.run",
		matcher: { readonly tool: "mcp__bedrock__run" },
		hook: Hook<BedrockCall>
	): unknown;
	(
		event: "tool.call",
		matcher: { readonly tool: "Bash" },
		hook: Hook<BashCall>
	): unknown;
}
