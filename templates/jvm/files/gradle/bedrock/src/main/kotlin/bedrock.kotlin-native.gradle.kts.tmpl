// Managed by bedrock: a Kotlin/Native executable for the machine that builds it, or for the
// platforms in the bedrock.platforms property of the module's gradle.properties. The
// sources go in src/nativeMain and src/nativeTest.
import bedrock.platforms
import bedrock.strict
import bedrock.version
import org.jetbrains.kotlin.gradle.plugin.mpp.DisableCacheInKotlinVersion
import org.jetbrains.kotlin.gradle.plugin.mpp.KotlinNativeCacheApi
import org.jetbrains.kotlin.gradle.plugin.mpp.NativeBuildType
import org.jetbrains.kotlin.konan.target.Family

plugins {
  id("org.jetbrains.kotlin.multiplatform")
  id("com.diffplug.spotless")
}

val os = System.getProperty("os.name")
val arm = System.getProperty("os.arch") in setOf("aarch64", "arm64")
val hostPlatform =
  when {
    os.startsWith("Mac") -> "macos"
    os.startsWith("Linux") -> "linux"
    os.startsWith("Windows") -> "windows"
    else -> throw GradleException("Kotlin/Native has no supported target for $os on this CPU.")
  }
// The machine that builds it, or the platforms in the bedrock.platforms property.
val selected = platforms(setOf(hostPlatform))

kotlin {
  compilerOptions { strict() }
  val executables =
    selected.map {
      when (it) {
        "macos" -> macosArm64()
        "linux" -> if (arm && it == hostPlatform) linuxArm64() else linuxX64()
        "windows" -> mingwX64()
        else -> throw GradleException("kotlin-native builds macos, linux, and windows, not $it.")
      }
    }
  executables.forEach { it.binaries.executable { entryPoint = "main" } }
  executables
    .filter { it.konanTarget.family == Family.OSX }
    .forEach { target ->
      // Xcode 27's linker turns the cached stdlib's debug info into entries dsymutil cannot open,
      // which drops it from the dSYM and prints a warning for each. Re-check on each Kotlin release.
      @OptIn(KotlinNativeCacheApi::class)
      target.binaries.matching { it.buildType == NativeBuildType.DEBUG }.configureEach {
        disableNativeCache(DisableCacheInKotlinVersion.`2_4_21`, "Xcode 27 dsymutil cannot read it")
      }
    }
  sourceSets.commonTest.dependencies { implementation(kotlin("test")) }
}

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
