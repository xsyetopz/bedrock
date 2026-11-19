/**
 * How many MSBuild nodes, test applications, or project evaluations run at
 * once: `BEDROCK_DOTNET_JOBS` when it is a positive integer, else 4, since
 * each takes much of the machine's memory.
 */
export const dotnetJobs = (
	jobs: string | undefined = process.env["BEDROCK_DOTNET_JOBS"]
): number => {
	const count = Number(jobs);
	return Number.isInteger(count) && count > 0 ? count : 4;
};

/**
 * MSBuild switches every build takes: at most {@link dotnetJobs} nodes, and
 * no node left alive afterwards, since an idle node holds its memory for
 * minutes.
 */
export const msbuildBounds = (): string[] => [
	`-maxCpuCount:${String(dotnetJobs())}`,
	"-nodeReuse:false"
];
