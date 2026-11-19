// Managed by bedrock: a Kotlin Multiplatform library for the JVM and the platforms in the
// bedrock.platforms property of the module's gradle.properties, android and ios by default.
// Android is Android 5 and up; the JVM and Android get Java 8 bytecode.
@file:OptIn(ExperimentalWasmDsl::class)

import bedrock.java8
import bedrock.platforms
import bedrock.strict
import bedrock.version
import org.jetbrains.kotlin.gradle.ExperimentalWasmDsl
import org.jetbrains.kotlin.gradle.targets.wasm.nodejs.WasmNodeJsEnvSpec
import org.jetbrains.kotlin.gradle.targets.wasm.nodejs.WasmNodeJsPlugin
import org.jetbrains.kotlin.gradle.targets.wasm.binaryen.BinaryenEnvSpec
import org.jetbrains.kotlin.gradle.targets.wasm.binaryen.BinaryenPlugin

plugins {
  id("org.jetbrains.kotlin.multiplatform")
  id("com.diffplug.spotless")
}

val selected = platforms(setOf("android", "ios"))
val nativeDesktop = !pluginManager.hasPlugin("org.jetbrains.compose")

if ("android" in selected) apply(plugin = "bedrock.kmp-android")

if ("wasm" in selected || "wasi" in selected) {
  // The Kotlin plugin adds its Node.js and Binaryen download repositories to the project, which
  // the settings' FAIL_ON_PROJECT_REPOS rejects; bedrock.settings declares them instead. Kotlin/Wasm
  // applies both plugins to the project that has the wasm target.
  // The root and the project each get a Node.js setup task.
  for (target in setOf(rootProject, project)) {
    target.plugins.withType<WasmNodeJsPlugin> {
      target.the<WasmNodeJsEnvSpec>().downloadBaseUrl.set(null as String?)
    }
  }
  plugins.withType<BinaryenPlugin> { the<BinaryenEnvSpec>().downloadBaseUrl.set(null as String?) }
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
  if ("ios" in selected) {
    iosArm64()
    iosSimulatorArm64()
  }
  // Compose Multiplatform runs desktop on the JVM, not on native targets.
  if (nativeDesktop) {
    if ("macos" in selected) macosArm64()
    if ("linux" in selected) linuxX64()
    if ("windows" in selected) mingwX64()
  }
  // The wasm targets run their tests on Node.js, which Gradle downloads; no browser is needed.
  if ("wasm" in selected) wasmJs { nodejs() }
  if ("wasi" in selected) wasmWasi { nodejs() }
  sourceSets.commonTest.dependencies { implementation(kotlin("test")) }
}

tasks.withType<Test>().configureEach { useJUnitPlatform() }

spotless {
  kotlin {
    target("src/*/kotlin/**/*.kt")
    targetExclude(".claude/**", "openspec/**")
    ktfmt(version("ktfmt")).kotlinlangStyle()
  }
  kotlinGradle {
    target("*.gradle.kts")
    targetExclude(".claude/**", "openspec/**")
    ktfmt(version("ktfmt")).kotlinlangStyle()
  }
}
