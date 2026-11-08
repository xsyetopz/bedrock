# Java and Kotlin

How `jvm-bedrock` sets up Java and Kotlin, and why. The pins live in `gradle/bedrock.versions.toml`.

## Toolchain

Gradle 9.9 runs the build, and the foojay resolver downloads GraalVM Community 25, the free GraalVM, as the toolchain for every module. Main code compiles to Java 8 bytecode: javac with `--release 8`, which also rejects newer JDK APIs, and Kotlin 2.4 with JVM target 1.8. Tests compile and run on Java 25, so they can use its APIs. Android apps run their debug unit tests; the release ones test the same code.

Every compiler warning is an error. On top of the compilers:

- Error Prone runs on Java, and NullAway treats each package under `group` in `gradle.properties` as non-null, so set `group` to the package the code lives in. Mark nullable Java types with JSpecify's `@Nullable`.
- Kotlin library modules use explicit API mode, which needs a visibility and return type on each public declaration. Applications skip it, since no other code calls them.
- Android Lint checks Android and KMP modules, and rejects APIs above Android 5.
- Spotless formats Java with google-java-format and Kotlin with ktfmt, both in Google style.

## Kinds

`jvm-bedrock new <dir> --kind <kind>` starts one of these:

| Kind | Plugins | Output |
| --- | --- | --- |
| `java-library`, `kotlin-library` | `bedrock.java`, or `bedrock.kotlin` | A jar for other code |
| `java-application`, `kotlin-application` | adds `bedrock.application` | A jar, and a ProGuard-shrunk `build/libs/<name>-min.jar` |
| `java-native` | adds `bedrock.native-image` | Also a GraalVM native executable, from `just jvm::native` |
| `kotlin-native` | `bedrock.kotlin-native` | A Kotlin/Native executable for the host |
| `kmp` | `bedrock.kmp` | A Kotlin Multiplatform library for the JVM, Android, and iOS |
| `cmp` | `bedrock.cmp` and `bedrock.android` | Compose Multiplatform UI in a shared module for Android and iOS, and the Android app |
| `android` | `bedrock.android` | An Android app in Compose |

## Shrinking

`bedrock.application` runs ProGuard over the application and its runtime classpath, with five optimization passes, access widening, and repackaging, and keeps `build/proguard/mapping.txt` for stack traces. Android release builds use R8, Android's ProGuard-compatible shrinker, with `proguard-android-optimize.txt` and `gradle/bedrock-android.pro`. A module's own rules go in `proguard-rules.pro` beside its build. Keep only the classes reflection needs there; the guard rejects `-dontshrink`, `-dontoptimize`, `-dontobfuscate`, `-ignorewarnings`, and keep rules for every class.

## Android and iOS Floors

Android 5 (API 21) is the minimum SDK. Jetpack Compose 1.10 and activity 1.12 need API 23, so Compose stays on 1.9, Compose Multiplatform on 1.9.3, and activity-compose on 1.11.

iOS targets start at 15.0, Kotlin/Native's minimum. Kotlin 2.2 still builds for iOS 12; moving to it would mean an older Kotlin for every module.

## Java to Kotlin

`just jvm::j2k <path>…` converts Java files, or directories of them, with IntelliJ IDEA's J2K converter, then formats the result. It runs IntelliJ IDEA 2026.3 headless, and the free tier is enough. The first run downloads IDEA, about 1.5 GB, into `~/.gradle/caches/bedrock/idea`; set `BEDROCK_IDEA` to an installed IDEA 2026.3 to skip it. The IDE runs with a 2 GB heap.

J2K replaces each `.java` file and moves the result from `src/<set>/java` to `src/<set>/kotlin`, so the task stops when a file has uncommitted changes, and when the module doesn't apply `bedrock.kotlin` yet. Its output compiles in most cases but not all. Run `just verify`, then fix what it reports, the `!!` J2K adds, and nullable types the Java never meant.

## Caveats

- Kotlin/Native's compile cache holds objects without a DWARF compile directory, and Xcode 27's linker records those as entries dsymutil can't open. Each debug link of an iOS binary in a `kmp` or `cmp` module prints a warning for each cached object, about 300, and their debug info is missing from the dSYM; the build still passes. `bedrock.kotlin-native` turns the cache off for its debug binaries on macOS, which avoids the warnings at about 1.5 times the link time. Turning it off for iOS would take Compose links about four times as long.
