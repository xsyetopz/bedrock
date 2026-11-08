// Managed by jvm-bedrock: a Kotlin/Native executable for the machine that builds it. The
// sources go in src/nativeMain and src/nativeTest.
import bedrock.strict
import bedrock.version
import org.jetbrains.kotlin.gradle.plugin.mpp.DisableCacheInKotlinVersion
import org.jetbrains.kotlin.gradle.plugin.mpp.KotlinNativeCacheApi
import org.jetbrains.kotlin.gradle.plugin.mpp.NativeBuildType

plugins {
  id("org.jetbrains.kotlin.multiplatform")
  id("com.diffplug.spotless")
}

val os = System.getProperty("os.name")
val arm = System.getProperty("os.arch") in setOf("aarch64", "arm64")

kotlin {
  compilerOptions { strict() }
  val host =
    when {
      os.startsWith("Mac") && arm -> macosArm64()
      os.startsWith("Linux") -> if (arm) linuxArm64() else linuxX64()
      os.startsWith("Windows") -> mingwX64()
      else -> throw GradleException("Kotlin/Native has no supported target for $os on this CPU.")
    }
  host.binaries.executable { entryPoint = "main" }
  if (os.startsWith("Mac")) {
    // Xcode 27's linker turns the cached stdlib's debug info into entries dsymutil cannot open,
    // which drops it from the dSYM and prints a warning for each. Re-check on each Kotlin release.
    @OptIn(KotlinNativeCacheApi::class)
    host.binaries.matching { it.buildType == NativeBuildType.DEBUG }.configureEach {
      disableNativeCache(DisableCacheInKotlinVersion.`2_4_21`, "Xcode 27 dsymutil cannot read it")
    }
  }
  sourceSets.commonTest.dependencies { implementation(kotlin("test")) }
}

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
