/**
 * A Gradle test source set's directory: `src/test`, `src/testFixtures`, an
 * Android variant's `src/testDebug` or `src/androidTest`, a Kotlin
 * Multiplatform target's `src/commonTest` or `src/jvmTest`, and a JVM test
 * suite's `src/integrationTest`. Gradle compiles them only for tests.
 */
const TEST_SOURCE_SET = /(?:^|\/)src\/(?:test|[a-z]\w*Test)(?:[A-Z]\w*)?\//u;

/** The files of `files` in a Gradle test source set. */
export const jvmTestFiles = (
	_root: string,
	files: readonly string[]
): Promise<Set<string>> =>
	Promise.resolve(new Set(files.filter((file) => TEST_SOURCE_SET.test(file))));
