import { fileURLToPath } from "node:url";

import { Glob } from "bun";

/** Every template file ends in this, so no tool reads a template as live config. */
export const SUFFIX = ".tmpl";

/** `@@name@@` placeholders a template fills from {@link Variables}. */
const PLACEHOLDER = /@@([a-z][a-zA-Z]*)@@/gu;

/** Values for template placeholders, such as the package name. */
export type Variables = Readonly<Record<string, string>>;

/**
 * The templates directory: embedded under `$bunfs` in the compiled binary
 * (`bun build --compile --asset=templates`), else beside the source tree.
 */
export const templatesRoot = (): string =>
	Bun.embeddedFiles.length > 0
		? `${import.meta.dir}/templates`
		: fileURLToPath(new URL("../../../templates", import.meta.url));

/** Paths that git and the hooks need to run as programs. */
export const isExecutable = (path: string): boolean =>
	path.startsWith(".githooks/") || /(?:^|\/)gradlew$/u.test(path);

/** Fills each placeholder; a name without a value is an error in bedrock itself. */
export const render = (
	text: string,
	variables: Variables,
	from: string,
): string =>
	text.replaceAll(PLACEHOLDER, (_, name: string) => {
		const value = variables[name];
		if (value === undefined) {
			throw new Error(`${from}: no value for @@${name}@@`);
		}
		return value;
	});

/** A read-only view of one directory of templates, such as `rust/files`. */
export interface TemplateDir {
	/** Target paths (without {@link SUFFIX}) of every template under the directory, sorted. */
	readonly paths: () => Promise<string[]>;
	/** The rendered template for a target path, or undefined when there is none. */
	readonly read: (
		path: string,
		variables?: Variables,
	) => Promise<string | undefined>;
}

/** The templates under `templates/<dir>`. */
export const templateDir = (
	dir: string,
	root = templatesRoot(),
): TemplateDir => {
	const base = `${root}/${dir}`;
	return {
		paths: async () => {
			const found = await Array.fromAsync(
				new Glob(`**/*${SUFFIX}`).scan({
					cwd: base,
					dot: true,
					onlyFiles: true,
				}),
			).catch(() => []);
			return found.map((path) => path.slice(0, -SUFFIX.length)).toSorted();
		},
		read: async (path, variables = {}) => {
			const file = Bun.file(`${base}/${path}${SUFFIX}`);
			return (await file.exists())
				? render(await file.text(), variables, `${dir}/${path}`)
				: undefined;
		},
	};
};
