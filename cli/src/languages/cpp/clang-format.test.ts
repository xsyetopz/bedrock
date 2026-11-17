import { expect, test } from "bun:test";

import { asRecord, asString, asStrings, type Json } from "../swift/json.ts";

interface Entry {
	parent: string | undefined;
	key: string;
	value: string;
}

const read = (path: string): Promise<string> =>
	Bun.file(new URL(path, import.meta.url)).text();

const unquote = (value: string): string =>
	value.length >= 2 && value.startsWith('"') && value.endsWith('"')
		? value.slice(1, -1)
		: value;

// The keys and scalar values of a flat `.clang-format`, one level of nesting at most.
const entries = (text: string): Entry[] => {
	let parent: string | undefined;
	return text
		.split("\n")
		.filter((line) => line.includes(":") && !line.startsWith("#"))
		.map((line) => {
			const colon = line.indexOf(":");
			const nested = line.startsWith(" ");
			const key = line.slice(0, colon).trim();
			const value = unquote(line.slice(colon + 1).trim());
			if (!nested) {
				parent = value === "" ? key : undefined;
			}
			return { parent: nested ? parent : undefined, key, value };
		});
};

const problem = (properties: Json, entry: Entry): string[] => {
	const { key, value } = entry;
	if (!(key in properties)) {
		return [`${key} is not in the schema`];
	}
	const property = asRecord(properties[key]);
	const allowed = asStrings(property["enum"]);
	if (allowed.length > 0) {
		return allowed.includes(value)
			? []
			: [`${key}: ${value} is not in ${allowed.join(", ")}`];
	}
	const type = asString(property["type"]);
	if (type === "boolean") {
		return ["true", "false"].includes(value)
			? []
			: [`${key}: ${value} is not a boolean`];
	}
	if (type === "integer") {
		return Number.isInteger(Number(value))
			? []
			: [`${key}: ${value} is not an integer`];
	}
	return [];
};

test("the .clang-format template matches the clang-format 21 schema", async () => {
	const parsed: unknown = JSON.parse(
		await read("./fixtures/clang-format-21.x.json"),
	);
	const top = asRecord(asRecord(parsed)["properties"]);
	const template = await read(
		"../../../../templates/cpp/files/.clang-format.tmpl",
	);

	const problems = entries(template)
		.filter((entry) => entry.value !== "")
		.flatMap((entry) =>
			problem(
				entry.parent === undefined
					? top
					: asRecord(asRecord(top[entry.parent])["properties"]),
				entry,
			),
		);

	expect(problems).toEqual([]);
});
