# Managed by jvm-bedrock. Put project recipes in project.just.

set shell := ["bash", "-euo", "pipefail", "-c"]
set working-directory := ".."

gradle := "./gradlew --console=plain"

# Check for the JDK that starts Gradle, then fetch Gradle and the plugins. The first compile
# fetches the GraalVM Community JDK the build compiles with. macOS ships a java that only says
# none is installed, so the check runs it.
setup:
    @[[ -n "${JAVA_HOME:-}" ]] || java -version >/dev/null 2>&1 || { echo "Gradle needs a JDK 17 or newer to start. Install one with the system's package manager, then set JAVA_HOME to it; on macOS, linking it into /Library/Java/JavaVirtualMachines, as brew info openjdk shows, also works for git apps." >&2; exit 1; }
    {{ gradle }} --quiet help >/dev/null

# Format the Java with google-java-format and the Kotlin with ktfmt's Google style.
format:
    {{ gradle }} spotlessApply

# Fail if the Java or Kotlin is not formatted.
format-check:
    {{ gradle }} spotlessCheck

# Compile the main code and tests, where every warning is an error.
typecheck:
    {{ gradle }} bedrockCompile

# Run Error Prone, NullAway, and Android Lint on top of the compilers' warnings.
lint:
    {{ gradle }} bedrockLint

# Apply the formatters' fixes.
lint-fix: format

# Run the tests.
test:
    {{ gradle }} bedrockTest

# Report drift from jvm-bedrock, foreign build tools, and suppressions without a reason.
guard:
    {{ gradle }} bedrockGuard

# Compile each GraalVM native-image application ahead of time and run its tests natively.
native:
    {{ gradle }} nativeCompile nativeTest

# Convert Java files, or directories of them, to Kotlin with IntelliJ IDEA's J2K, then
# format the result. The first run downloads IntelliJ IDEA, about 1.5 GB; BEDROCK_IDEA
# points it at an installed IDEA instead.
j2k +paths:
    {{ gradle }} j2k {{ prepend("--file=", paths) }}
    {{ gradle }} spotlessApply

# Run every JVM check CI runs, in one Gradle run.
verify:
    {{ gradle }} spotlessCheck bedrockCompile bedrockLint bedrockGuard bedrockTest

# The JVM checks the pre-commit hook runs.
pre-commit:
    {{ gradle }} spotlessCheck bedrockCompile bedrockGuard
