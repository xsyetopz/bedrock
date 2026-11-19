// Managed by bedrock: the Android 5 and up target of a Kotlin Multiplatform library, applied by
// bedrock.kmp when its platforms include android.
import bedrock.java8
import bedrock.version

plugins {
  id("org.jetbrains.kotlin.multiplatform")
  id("com.android.kotlin.multiplatform.library")
  id("com.android.lint")
}

// Lint runs on the toolchain the code compiles with: the Android and JVM targets each have a lint,
// and on the Gradle daemon's JVM each fails whenever that is older.
fun JavaToolchainSpec.lintToolchain() {
  languageVersion = JavaLanguageVersion.of(version("jdk"))
  vendor = JvmVendorSpec.GRAAL_VM
}

kotlin {
  android {
    namespace = "${project.group}.${project.name}".replace('-', '_')
    compileSdk = version("android-compile-sdk").toInt()
    minSdk = version("android-min-sdk").toInt()
    compilerOptions { java8() }
    withHostTest {}
    lint { toolchain { lintToolchain() } }
  }
}

lint {
  toolchain { lintToolchain() }
  warningsAsErrors = true
  checkDependencies = true
  // New releases would fail a build that did not change; Dependabot bumps the catalog instead.
  // The AndroidX releases that still run on Android 5 ship lint checks this lint cannot load.
  disable +=
    setOf(
      "GradleDependency",
      "NewerVersionAvailable",
      "AndroidGradlePluginVersion",
      "ObsoleteLintCustomCheck",
    )
}
