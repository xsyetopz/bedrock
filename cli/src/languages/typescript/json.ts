import { readText } from "../../core/fs.ts";

/** A parsed JSON object. */
export type Json = Record<string, unknown>;

/** Whether `value` is an array. */
export const isArray = (value: unknown): value is readonly unknown[] =>
	Array.isArray(value);

/** Whether `value` is a JSON object. */
export const isRecord = (value: unknown): value is Json =>
	typeof value === "object" && value !== null && !Array.isArray(value);

/** `value` when it is an object, else an empty one. */
export const asRecord = (value: unknown): Json =>
	isRecord(value) ? value : {};

/** The string items of `value`, or none when it is not an array. */
export const asStrings = (value: unknown): string[] =>
	isArray(value)
		? value.filter((item): item is string => typeof item === "string")
		: [];

/** The JSONC file's object, or undefined when the file is missing. */
export const readJsonc = async (path: string): Promise<Json | undefined> => {
	const text = await readText(path);
	return text === undefined ? undefined : asRecord(Bun.JSONC.parse(text));
};

/** JSON in the repo's tab-indented style, ending in a newline. */
export const toJson = (value: unknown): string =>
	`${JSON.stringify(value, undefined, "\t")}\n`;
