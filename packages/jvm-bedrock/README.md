# JVM Bedrock

Shared tooling for strict Java and Kotlin repos built with Gradle 9.9. Main code compiles to Java 8 bytecode with GraalVM Community 25, Android apps run on Android 5 (API 21) and newer, and Kotlin Multiplatform code also targets iOS 15. The root `settings.gradle.kts` builds it as `:jvm-bedrock`.

## Commands

| Command | Does |
| --- | --- |
| `jvm-bedrock new <dir> [--kind <kind>] [--group <group>]` | Start a project. The kinds are `java-library`, `java-application`, `java-native`, `kotlin-library` (the default), `kotlin-application`, `kotlin-native`, `kmp`, `cmp`, and `android`. `--group` defaults to `com.example.<dir>`. |
| `jvm-bedrock adopt [<dir>] [--dry-run] [--yes] [--group <group>]` | Set up an existing repo, migrating a Maven, sbt, Ant or Ivy, or Gradle build to the Gradle Kotlin DSL. |

Build it with `./gradlew :jvm-bedrock:installDist`, which puts the launcher at `packages/jvm-bedrock/build/install/jvm-bedrock/bin/jvm-bedrock`.

## What It Sets Up

- `gradle/bedrock/`, an included build of convention plugins. Each module's `build.gradle.kts` applies one of `bedrock.java`, `bedrock.kotlin`, `bedrock.application`, `bedrock.native-image`, `bedrock.kmp`, `bedrock.cmp`, `bedrock.android`, or `bedrock.kotlin-native`.
- `gradle/bedrock.versions.toml`, the pinned toolchain and build tools. The project's own dependencies go in `gradle/libs.versions.toml`.
- javac `--release 8` and Kotlin's JVM target 1.8, with warnings as errors, Error Prone, and NullAway checking the packages under `group`.
- google-java-format and ktfmt, both in Google style, through Spotless.
- ProGuard for applications (`build/libs/<name>-min.jar`), with the rules in `gradle/bedrock.pro`, and R8 for Android release builds, with `gradle/bedrock-android.pro`.
- GraalVM native-image for `bedrock.native-image` applications.
- JUnit 6 and `kotlin.test`; JUnit 4 only in Android tests.
- `just jvm::j2k <path>…`, which converts Java to Kotlin with IntelliJ IDEA's J2K, run headless.
- The `bedrockGuard` task, `.just/jvm.just`, `.github/workflows/jvm.yml`, and its blocks in the files every bedrock shares (`files/base`, a copy of `base/`).

[docs/jvm.md](../../docs/jvm.md) explains the choices, and [docs/adopting.md](../../docs/adopting.md#java-and-kotlin) lists what `adopt` changes.

## Requirements

Gradle starts on a JDK 17 or newer, from `JAVA_HOME` or `java` on `PATH`; it downloads GraalVM Community 25 for the build itself. Android, KMP, and CMP modules need the Android SDK, through `ANDROID_HOME` or `local.properties`, and iOS targets need Xcode.
