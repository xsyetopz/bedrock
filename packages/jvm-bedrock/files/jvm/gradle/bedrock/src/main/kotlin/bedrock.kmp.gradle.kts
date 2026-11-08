// Managed by jvm-bedrock: a Kotlin Multiplatform library for the JVM, Android 5 and up, and
// iOS, with Java 8 bytecode on the JVM and Android.
import bedrock.java8
import bedrock.strict
import bedrock.version

plugins {
  id("org.jetbrains.kotlin.multiplatform")
  id("com.android.kotlin.multiplatform.library")
  id("com.android.lint")
  id("com.diffplug.spotless")
}

kotlin {
  explicitApi()
  compilerOptions { strict() }
  jvmToolchain {
    languageVersion = JavaLanguageVersion.of(version("jdk"))
    vendor = JvmVendorSpec.GRAAL_VM
    nativeImageCapable = true
  }
  jvm {
    compilations.named("main") { compileTaskProvider.configure { compilerOptions { java8() } } }
  }
  android {
    namespace = "${project.group}.${project.name}".replace('-', '_')
    compileSdk = version("android-compile-sdk").toInt()
    minSdk = version("android-min-sdk").toInt()
    compilerOptions { java8() }
    withHostTest {}
  }
  iosArm64()
  iosSimulatorArm64()
  sourceSets.commonTest.dependencies { implementation(kotlin("test")) }
}

lint {
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

tasks.withType<Test>().configureEach { useJUnitPlatform() }

spotless {
  kotlin {
    target("src/*/kotlin/**/*.kt")
    ktfmt(version("ktfmt")).googleStyle()
  }
  kotlinGradle {
    target("*.gradle.kts")
    ktfmt(version("ktfmt")).googleStyle()
  }
}
