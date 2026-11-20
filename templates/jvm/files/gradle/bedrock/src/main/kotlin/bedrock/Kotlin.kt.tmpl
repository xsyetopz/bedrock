package bedrock

import org.jetbrains.kotlin.gradle.dsl.JvmTarget
import org.jetbrains.kotlin.gradle.dsl.KotlinCommonCompilerOptions
import org.jetbrains.kotlin.gradle.dsl.KotlinJvmCompilerOptions
import org.jetbrains.kotlin.gradle.plugin.mpp.KotlinNativeTarget

/** Every warning, the extra checkers included, is an error; progressive mode fixes old quirks. */
internal fun KotlinCommonCompilerOptions.strict() {
  allWarningsAsErrors.set(true)
  extraWarnings.set(true)
  progressiveMode.set(true)
}

/** Java 8 bytecode against the Java 8 API, whichever JDK compiles it. */
internal fun KotlinJvmCompilerOptions.java8() {
  jvmTarget.set(JvmTarget.JVM_1_8)
  freeCompilerArgs.add("-Xjdk-release=1.8")
}

/** Links the target's binaries for [minimum], since Kotlin/Native's macOS default sits below it. */
internal fun KotlinNativeTarget.macosMinimum(minimum: String) {
  binaries.configureEach {
    freeCompilerArgs += "-Xoverride-konan-properties=osVersionMin.${konanTarget.name}=$minimum"
  }
}
