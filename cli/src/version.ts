import manifest from "../../package.json" with { type: "json" };

/** The bedrock release this build is, from the root `package.json`. */
export const VERSION: string = manifest.version;

/** Where the release binaries are published. */
export const REPOSITORY = "https://github.com/xsyetopz/bedrock";
