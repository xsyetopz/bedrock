# Java and Kotlin

How bedrock sets up Java and Kotlin, and why. The pins live in `gradle/bedrock.versions.toml`.

## Toolchain

Gradle 9.9 runs the build, and the foojay resolver downloads GraalVM Community 25, the free GraalVM, as the toolchain for every module. Main code compiles to Java 8 bytecode: javac with `--release 8`, which also rejects newer JDK APIs, and Kotlin 2.4 with JVM target 1.8. Tests compile and run on Java 25, so they can use its APIs. Android apps run their debug unit tests; the release ones test the same code.

Gradle itself needs a JDK 17 or newer to start. Its daemon then runs on JDK 25, which `gradle/gradle-daemon-jvm.properties` asks for. Gradle matches that major version exactly, so a newer JDK such as 26 does not count: it uses an installed JDK 25 when it finds one, as on GitHub's Ubuntu runners, and downloads one otherwise. The daemon needs a modern JDK since Spotless runs google-java-format in the daemon and its releases need JDK 21 or newer. The `gradle` recipe in `.just/jvm.just` finds one, in this order: a valid `JAVA_HOME`, a `java` that runs, macOS's `java_home`, then Homebrew and `/usr/lib/jvm`. It exports `JAVA_HOME` for Gradle, and stops with an install hint when none runs; `bedrock adopt` and `bedrock jvm j2k` search the same way. A build that applies `bedrock.android` or `bedrock.cmp` also needs the Android SDK, through `ANDROID_HOME` or `sdk.dir` in `local.properties`.

Every compiler warning is an error. On top of the compilers:

- Error Prone runs on Java, and NullAway treats each package under `group` in `gradle.properties` as non-null, so set `group` to the package the code lives in. Mark nullable Java types with JSpecify's `@Nullable`.
- Kotlin library modules use explicit API mode, which needs a visibility and return type on each public declaration. Applications skip it, since no other code calls them.
- Android Lint checks Android and KMP modules, and rejects APIs above Android 5.
- Spotless formats Java with google-java-format and Kotlin with ktfmt's kotlinlang style, which indents by four spaces, as the Kotlin coding conventions do.

## Kinds

`bedrock init <dir> jvm:<kind>` starts one of these:

| Kind | Plugins | Output |
| --- | --- | --- |
| `java-library`, `kotlin-library` | `bedrock.java`, or `bedrock.kotlin` | A jar for other code |
| `java-application`, `kotlin-application` | adds `bedrock.application` | A jar, and a ProGuard-shrunk `build/libs/<name>-min.jar` |
| `java-native` | adds `bedrock.native-image` | Also a GraalVM native executable, from `just jvm::native` |
| `java-gradle-plugin`, `kotlin-gradle-plugin` | adds `bedrock.gradle-plugin` | A Gradle plugin with the Gradle API and TestKit, as Java 17 bytecode, the floor of Gradle's API |
| `scala-library`, `scala-application` | `bedrock.scala`, and `application` for the application | Scala 3 with scalafmt, every warning an error |
| `kotlin-native` | `bedrock.kotlin-native` | A Kotlin/Native executable for the host, or for `@linux`, `@macos`, and `@windows` |
| `kmp` | `bedrock.kmp` | A Kotlin Multiplatform library for the JVM and `@android`, `@ios`, `@linux`, `@macos`, `@wasi`, `@wasm`, or `@windows` |
| `cmp` | `bedrock.cmp` and `bedrock.android` | Compose Multiplatform UI in a shared module for the JVM and `@android`, `@ios`, `@linux`, `@macos`, `@wasm`, or `@windows`, and the Android app |
| `android` | `bedrock.android` | An Android app in Compose |

`@<platform>,...` after the kind, such as `bedrock init app jvm:kmp@android,ios,wasm`, writes the list to the `bedrock.platforms` property of the module's `gradle.properties`, which the plugins read; `kmp` and `cmp` build for `android` and `ios` without it. `cmp` runs Compose on the JVM for `linux`, `macos`, and `windows`, as Compose Multiplatform supports the desktop, and in the browser for `wasm`. Its wasm tests stay off, since Compose's skiko cannot load its wasm module on Node.js, where bedrock.kmp runs them. A build for `android` needs the Android SDK: the recipes stop and say so when neither `ANDROID_HOME` nor `sdk.dir` in `local.properties` names it.

A new project's `group` is `com.example.<directory name>` unless `gradle.properties` already names one or `--group <package>` sets it, such as `bedrock init app jvm:kotlin-library --group org.acme.app`. `.editorconfig` keeps the group as `bedrock_jvm_group`, so `bedrock update` uses it again, and the starter sources sit under its package path. A `--group` wins over the `group` in `gradle.properties`: adopt rewrites that line, notes the change, and leaves the code in its old package for you to move.

## Shrinking

`bedrock.application` runs ProGuard over the application and its runtime classpath, with five optimization passes, access widening, and repackaging, and keeps `build/proguard/mapping.txt` for stack traces. Android release builds use R8, Android's ProGuard-compatible shrinker, with `proguard-android-optimize.txt` and `gradle/bedrock-android.pro`. A module's own rules go in `proguard-rules.pro` beside its build. Keep only the classes reflection needs there; the guard rejects `-dontshrink`, `-dontoptimize`, `-dontobfuscate`, `-ignorewarnings`, and keep rules for every class.

## Android and iOS Floors

Android 5 (API 21) is the minimum SDK. Jetpack Compose 1.10 and activity 1.12 need API 23, so Compose stays on 1.9, Compose Multiplatform on 1.9.3, and activity-compose on 1.11.

iOS targets start at 15.0, Kotlin/Native's minimum and the oldest iOS that Xcode 26 and later, which the App Store requires, build for. macOS targets link for 13.0, the floor every bedrock language shares, above Kotlin/Native's 12.0 default.

## Java to Kotlin

`just jvm::j2k <path>…`, or `bedrock jvm j2k <path>...`, converts Java files, or directories of them, with IntelliJ IDEA's J2K converter, then formats the result. It runs IntelliJ IDEA 2026.3 headless, and the free tier is enough. The first run downloads IDEA, about 1.5 GB, into `~/.gradle/caches/bedrock/idea`; set `BEDROCK_IDEA` to an installed IDEA 2026.3 to skip it. The IDE runs with a 2 GB heap.

J2K replaces each `.java` file and moves the result from `src/<set>/java` to `src/<set>/kotlin`, so the task stops when a file has uncommitted changes, and when the module doesn't apply `bedrock.kotlin` yet. Its output compiles in most cases but not all. Run `just verify`, then fix what it reports, the `!!` J2K adds, and nullable types the Java never meant.

## Caveats

- Kotlin/Native's compile cache holds objects without a DWARF compile directory, and Xcode 27's linker records those as entries dsymutil can't open. Each debug link of an iOS binary in a `kmp` or `cmp` module prints a warning for each cached object, about 300, and their debug info is missing from the dSYM; the build still passes. `bedrock.kotlin-native` turns the cache off for its debug binaries on macOS, which avoids the warnings at about 1.5 times the link time. Turning it off for iOS would take Compose links about four times as long.
