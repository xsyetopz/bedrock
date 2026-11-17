/**
 * Runs `run` on each item, one after another, for work that must not
 * overlap, such as installs that write the same lockfile or print to the
 * same terminal.
 */
export const inOrder = async <T>(
	items: readonly T[],
	run: (item: T) => Promise<void>,
): Promise<void> => {
	const [head, ...tail] = items;
	if (head === undefined) {
		return;
	}
	await run(head);
	await inOrder(tail, run);
};
