// Managed by bedrock: Compose Multiplatform UI shared by the platforms in the bedrock.platforms
// property of the module's gradle.properties, android and ios by default. The iOS app links the
// Shared framework; macos, linux, and windows run Compose on the JVM, which Compose Multiplatform
// supports for desktop in place of native targets, and wasm runs it in the browser.
import bedrock.library
import bedrock.platforms
import org.jetbrains.kotlin.gradle.ExperimentalWasmDsl

// Compose goes before bedrock.kmp, which then leaves the native desktop targets out.
plugins {
  id("org.jetbrains.kotlin.multiplatform")
  id("org.jetbrains.compose")
  id("org.jetbrains.kotlin.plugin.compose")
  id("bedrock.kmp")
}

val selected = platforms(setOf("android", "ios"))

kotlin {
  if ("ios" in selected) {
    listOf(iosArm64(), iosSimulatorArm64()).forEach {
      it.binaries.framework {
        baseName = "Shared"
        isStatic = true
      }
    }
  }
  // bedrock.kmp runs the wasm tests on Node.js, where Compose's skiko cannot load its wasm module,
  // so the tests compile but do not run; the browser bundle needs no browser to build.
  @OptIn(ExperimentalWasmDsl::class)
  if ("wasm" in selected) {
    wasmJs {
      browser { testTask { enabled = false } }
      binaries.executable()
    }
  }
  tasks.matching { it.name == "wasmJsNodeTest" }.configureEach { enabled = false }
  sourceSets.commonMain.dependencies {
    implementation(library("compose-runtime"))
    implementation(library("compose-foundation"))
    implementation(library("compose-ui"))
  }
  if (selected.any { it in setOf("macos", "linux", "windows") }) {
    sourceSets.getByName("jvmMain").dependencies { implementation(compose.desktop.currentOs) }
  }
}
