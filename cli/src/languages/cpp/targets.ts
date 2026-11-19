/** One build target, as a build system's own query reports it. */
export interface Target {
	readonly id: string;
	/** Sources it compiles, relative to the repo root. */
	readonly sources: readonly string[];
	/** Ids of the targets it links or depends on. */
	readonly deps: readonly string[];
	/** Whether a test the build system registers runs this target. */
	readonly test: boolean;
	/** Whether the build installs it, which makes it product code. */
	readonly installed: boolean;
}

/** The sources only test-only targets compile, and those some other target compiles. */
export interface Split {
	readonly test: ReadonlySet<string>;
	readonly other: ReadonlySet<string>;
}

/**
 * Splits the sources of `targets` by the build's own view of tests: a target a
 * test runs is test-only, and so is an uninstalled one that only test-only
 * targets depend on, such as a fixture library. A source compiled by any other
 * target is product code.
 */
export const splitSources = (targets: readonly Target[]): Split => {
	const dependents = new Map<string, string[]>();
	for (const target of targets) {
		for (const dep of target.deps) {
			dependents.set(dep, [...(dependents.get(dep) ?? []), target.id]);
		}
	}
	const byId = new Map(targets.map((target) => [target.id, target]));
	const memo = new Map<string, boolean>();
	const testOnly = (id: string, seen: ReadonlySet<string>): boolean => {
		const target = byId.get(id);
		if (target === undefined || seen.has(id)) {
			return false;
		}
		if (target.test) {
			return true;
		}
		const known = memo.get(id);
		if (known !== undefined) {
			return known;
		}
		const users = dependents.get(id) ?? [];
		const next = new Set([...seen, id]);
		const result =
			!target.installed &&
			users.length > 0 &&
			users.every((user) => testOnly(user, next));
		memo.set(id, result);
		return result;
	};
	const test = new Set<string>();
	const other = new Set<string>();
	for (const target of targets) {
		const into = testOnly(target.id, new Set()) ? test : other;
		for (const source of target.sources) {
			into.add(source);
		}
	}
	return {
		test: new Set([...test].filter((source) => !other.has(source))),
		other
	};
};
