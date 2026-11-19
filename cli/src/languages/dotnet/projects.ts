import { Glob } from "bun";

import { listFiles } from "../../core/fs.ts";

/** MSBuild project files of the .NET languages bedrock checks. */
export const PROJECT_GLOB = "**/*.{csproj,fsproj}";

/** Solution files: the XML `.slnx` of .NET 9 and later, and the older `.sln`. */
const SOLUTION_GLOB = "*.{slnx,sln}";

/** The C# and F# sources lizard and the formatters read. */
export const CSHARP_GLOB = "**/*.{cs,csx}";
export const FSHARP_GLOB = "**/*.{fs,fsi,fsx}";

/** The project files under `root` that git tracks or would track. */
export const projectFiles = (root: string): Promise<string[]> =>
	listFiles(root, new Glob(PROJECT_GLOB));

/**
 * What `dotnet build` and `dotnet test` run on: each solution at the root,
 * since a solution lists the projects that build together, or, without one,
 * each project file. A `.sln` beside the `.slnx` of the same name is the
 * same solution in the old format, so only the `.slnx` counts.
 */
export const buildEntries = async (root: string): Promise<string[]> => {
	const solutions = await listFiles(root, new Glob(SOLUTION_GLOB));
	const kept = solutions.filter(
		(path) => !(path.endsWith(".sln") && solutions.includes(`${path}x`))
	);
	if (kept.length > 0) {
		return kept.toSorted();
	}
	const projects = await projectFiles(root);
	return projects.toSorted();
};
