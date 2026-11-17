import { fileURLToPath } from "node:url";

import { Glob } from "bun";

/** Every template file ends in this, so no tool reads a template as live config. */
export const SUFFIX = ".tmpl";

/** `@@name@@` placeholders a template fills from {@link Variables}. */
const PLACEHOLDER = /@@([a-z][a-zA-Z]*)@@/gu;

/**
 * `__name__` placeholders in a template's path, such as
 * `Sources/__typeName__/`. Paths take underscores rather than `@@`, since
 * globs and shells read them literally.
 */
const PATH_PLACEHOLDER = /__([a-z][a-zA-Z]*)__/gu;

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

/** Fills each match of `pattern`; a name without a value is an error in bedrock itself. */
const fill = (
	pattern: RegExp,
	text: string,
	variables: Variables,
	from: string,
): string =>
	text.replaceAll(pattern, (placeholder, name: string) => {
		const value = variables[name];
		if (value === undefined) {
			throw new Error(`${from}: no value for ${placeholder}`);
		}
		return value;
	});

/** Fills each `@@name@@` placeholder of a template's text. */
export const render = (
	text: string,
	variables: Variables,
	from: string,
): string => fill(PLACEHOLDER, text, variables, from);

/** A template's target path with each `__name__` placeholder filled. */
export const renderPath = (path: string, variables: Variables): string =>
	fill(PATH_PLACEHOLDER, path, variables, path);

/** One template as a repo receives it: the filled path, and text or the bytes of a binary file. */
export interface Rendered {
	readonly path: string;
	readonly content: string | Uint8Array;
}

/**
 * The template's content: text with its placeholders filled, or the bytes
 * unchanged when it holds a NUL byte, as a jar or an image does (git tells
 * binary from text the same way).
 */
const contentOf = async (
	file: Bun.BunFile,
	variables: Variables,
	from: string,
): Promise<string | Uint8Array> => {
	const bytes = await file.bytes();
	return bytes.includes(0)
		? bytes
		: render(new TextDecoder().decode(bytes), variables, from);
};

/** A read-only view of one directory of templates, such as `rust/files`. */
export interface TemplateDir {
	/** Target paths (without {@link SUFFIX}) of every template under the directory, sorted. */
	readonly paths: () => Promise<string[]>;
	/** The rendered template for a target path, or undefined when there is none. */
	readonly read: (
		path: string,
		variables?: Variables,
	) => Promise<string | undefined>;
	/** Every template, with its path and text filled from `variables` and binary files as bytes, sorted by template path. */
	readonly files: (variables?: Variables) => Promise<Rendered[]>;
}

/** The templates under `templates/<dir>`. */
export const templateDir = (
	dir: string,
	root = templatesRoot(),
): TemplateDir => {
	const base = `${root}/${dir}`;
	const paths = async (): Promise<string[]> => {
		const found = await Array.fromAsync(
			new Glob(`**/*${SUFFIX}`).scan({
				cwd: base,
				dot: true,
				onlyFiles: true,
			}),
		).catch(() => []);
		return found.map((path) => path.slice(0, -SUFFIX.length)).toSorted();
	};
	return {
		paths,
		read: async (path, variables = {}) => {
			const file = Bun.file(`${base}/${path}${SUFFIX}`);
			return (await file.exists())
				? render(await file.text(), variables, `${dir}/${path}`)
				: undefined;
		},
		files: async (variables = {}) => {
			const found = await paths();
			return Promise.all(
				found.map(async (path) => ({
					path: renderPath(path, variables),
					content: await contentOf(
						Bun.file(`${base}/${path}${SUFFIX}`),
						variables,
						`${dir}/${path}`,
					),
				})),
			);
		},
	};
};
