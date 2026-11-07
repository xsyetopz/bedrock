import { stat } from "node:fs/promises";

export type Json = Record<string, unknown>;

export const isArray = (value: unknown): value is readonly unknown[] =>
	Array.isArray(value);

export const isRecord = (value: unknown): value is Json =>
	typeof value === "object" && value !== null && !Array.isArray(value);

export const asRecord = (value: unknown): Json =>
	isRecord(value) ? value : {};

export const asStrings = (value: unknown): string[] =>
	isArray(value)
		? value.filter((item): item is string => typeof item === "string")
		: [];

export const exists = async (path: string): Promise<boolean> => {
	try {
		await stat(path);
		return true;
	} catch {
		return false;
	}
};

export const readJsonc = async (path: string): Promise<Json | undefined> => {
	const file = Bun.file(path);
	return (await file.exists())
		? asRecord(Bun.JSONC.parse(await file.text()))
		: undefined;
};

export const toJson = (value: unknown): string =>
	`${JSON.stringify(value, undefined, "\t")}\n`;
