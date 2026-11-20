# Managed by bedrock. Put project recipes in project.just.

set shell := ["bash", "-euo", "pipefail", "-c"]
set working-directory := ".."

# Check for the JDK that starts Gradle, then fetch Gradle and the plugins. The first compile
# fetches the GraalVM Community JDK the build compiles with. uv runs the pinned lizard
# that bedrock guard bounds file and function size with.
setup: (gradle "--quiet" "help")
    if ! command -v uvx >/dev/null && ! command -v pipx >/dev/null; then brew install uv; fi

# Format the Java with google-java-format and the Kotlin with ktfmt's kotlinlang style.
format: hold (gradle "spotlessApply")

# Fail if the Java or Kotlin is not formatted.
format-check: hold (gradle "spotlessCheck")

# Compile the main code and tests, where every warning is an error.
typecheck: hold (gradle "bedrockCompile")

# Run Error Prone, NullAway, and Android Lint on top of the compilers' warnings.
lint: hold (gradle "bedrockLint")

# Apply the formatters' fixes.
lint-fix: hold format

# Run the tests.
test: hold (gradle "bedrockTest")

# Report foreign build tools, suppressions without a reason, and drift from bedrock.
guard:
    {{ quote(replace(just_executable(), '\', '/')) }} bedrock guard jvm

# Compile each GraalVM native-image application ahead of time and run its tests natively.
native: (gradle "nativeCompile" "nativeTest")

# Convert Java files to Kotlin with IntelliJ IDEA's J2K, then format. The first run downloads IDEA; BEDROCK_IDEA names an installed one.
[positional-arguments]
j2k +paths:
    {{ quote(replace(just_executable(), '\', '/')) }} bedrock jvm j2k "$@"

# Run every JVM check CI runs, in one Gradle run.
verify: guard (gradle "spotlessCheck" "bedrockCompile" "bedrockLint" "bedrockTest")

# The JVM checks the pre-commit hook runs.
pre-commit: guard (gradle "spotlessCheck" "bedrockCompile")

# The JVM checks the pre-push hook runs: verify without the tests, which CI runs.
pre-push: guard (gradle "spotlessCheck" "bedrockCompile" "bedrockLint")

# Run Gradle with a JDK that works. macOS ships a /usr/bin/java that only says no JDK is
# installed, and Homebrew's JDKs are not linked where macOS looks, so this finds one, exports
# JAVA_HOME for Gradle, and stops with the install step when there is none. A build that
# applies bedrock.android, or bedrock.kmp or bedrock.cmp for android, also needs the Android SDK.
[positional-arguments]
[private]
gradle +tasks:
    #!/usr/bin/env bash
    set -euo pipefail
    find_jdk() {
        if [[ -n "${JAVA_HOME:-}" && -x "$JAVA_HOME/bin/java" ]]; then return 0; fi
        unset JAVA_HOME
        if java -version >/dev/null 2>&1; then return 0; fi
        local home candidate
        if home="$(/usr/libexec/java_home 2>/dev/null)"; then export JAVA_HOME="$home"; return 0; fi
        for candidate in \
            /opt/homebrew/opt/openjdk@25 /opt/homebrew/opt/openjdk@21 /opt/homebrew/opt/openjdk@17 /opt/homebrew/opt/openjdk \
            /usr/local/opt/openjdk@25 /usr/local/opt/openjdk@21 /usr/local/opt/openjdk@17 /usr/local/opt/openjdk \
            /opt/homebrew/opt/openjdk@* /usr/local/opt/openjdk@* /usr/lib/jvm/*; do
            for home in "$candidate/libexec/openjdk.jdk/Contents/Home" "$candidate"; do
                if [[ -x "$home/bin/java" ]]; then export JAVA_HOME="$home"; return 0; fi
            done
        done
        return 1
    }
    if ! find_jdk; then
        echo "Gradle needs a JDK 17 or newer to start, and none runs here. Install one, such as brew install openjdk@25 or your system's openjdk package, then rerun the recipe; it finds Homebrew's and /usr/lib/jvm's JDKs itself, or set JAVA_HOME." >&2
        exit 1
    fi
    uses_android=false
    while IFS= read -r -d '' script; do
        if grep -qE 'id\("bedrock\.android"\)' "$script"; then uses_android=true; break; fi
        # kmp and cmp build for Android unless the module's bedrock.platforms leaves it out.
        if grep -qE 'id\("bedrock\.(kmp|cmp)"\)' "$script"; then
            props="$(dirname "$script")/gradle.properties"
            platforms="$(sed -n 's/^bedrock\.platforms=//p' "$props" 2>/dev/null || true)"
            if [[ -z "$platforms" || ",$platforms," == *",android,"* ]]; then uses_android=true; break; fi
        fi
    done < <(git ls-files --cached --others --exclude-standard -z -- '*build.gradle.kts' 2>/dev/null)
    if [[ "$uses_android" == true ]]; then
        sdk="${ANDROID_HOME:-${ANDROID_SDK_ROOT:-}}"
        if [[ -z "$sdk" && -f local.properties ]]; then
            sdk="$(sed -n 's/^sdk\.dir=//p' local.properties | head -n 1)"
        fi
        if [[ -z "$sdk" || ! -d "$sdk" ]]; then
            level="$(sed -n 's/^android-compile-sdk = "\(.*\)"/\1/p' gradle/bedrock.versions.toml)"
            echo "The Android SDK is missing: neither ANDROID_HOME nor sdk.dir in local.properties names a directory." >&2
            echo "Install it, for example brew install --cask android-commandlinetools, then run" >&2
            echo "  sdkmanager --sdk_root=\"\$HOME/Library/Android/sdk\" --licenses" >&2
            echo "  sdkmanager --sdk_root=\"\$HOME/Library/Android/sdk\" \"platforms;android-$level\"" >&2
            echo "and set ANDROID_HOME to that directory or put sdk.dir=<directory> in local.properties." >&2
            exit 1
        fi
    fi
    # Gradle's workers default to one per core; see BEDROCK_JOBS in the justfile.
    exec ./gradlew --console=plain --max-workers="${BEDROCK_JVM_JOBS:-${BEDROCK_JOBS:-4}}" "$@"

# Takes one of the machine's check slots for this run; see `hold-slot` in the justfile.
[private]
hold:
    @{{ quote(replace(just_executable(), '\', '/')) }} hold-slot "$PPID"
