/** A parsed JSON object. */
export type Json = Record<string, unknown>;

/** `value` when it is a JSON object, else an empty one. */
export const asRecord = (value: unknown): Json =>
	typeof value === "object" && value !== null && !Array.isArray(value)
		? Object.fromEntries(Object.entries(value))
		: {};

/** The items of `value`, or none when it is not an array. */
export const asArray = (value: unknown): readonly unknown[] =>
	Array.isArray(value) ? value : [];

/** The string items of `value`. */
export const asStrings = (value: unknown): string[] =>
	asArray(value).filter((item): item is string => typeof item === "string");

/** `value` when it is a string, else an empty one. */
export const asString = (value: unknown): string =>
	typeof value === "string" ? value : "";
