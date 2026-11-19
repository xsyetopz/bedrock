import type { Context } from "../../core/language.ts";
import type { Variables } from "../../core/templates.ts";

/** The Bun release bedrock pins; a test keeps it equal to the repository's own `.bun-version`. */
export const BUN_VERSION = "1.4.5";

/** Placeholder values for the TypeScript templates. */
export const variables = (_context: Context): Promise<Variables> =>
	Promise.resolve({
		bunVersion: BUN_VERSION,
		packageManager: `bun@${BUN_VERSION}`,
	});
