// Managed by jvm-bedrock: Compose Multiplatform UI shared by Android 5 and up and iOS. The iOS
// app links the Shared framework.
import bedrock.library

plugins {
  id("bedrock.kmp")
  id("org.jetbrains.compose")
  id("org.jetbrains.kotlin.plugin.compose")
}

kotlin {
  listOf(iosArm64(), iosSimulatorArm64()).forEach {
    it.binaries.framework {
      baseName = "Shared"
      isStatic = true
    }
  }
  sourceSets.commonMain.dependencies {
    implementation(library("compose-runtime"))
    implementation(library("compose-foundation"))
    implementation(library("compose-ui"))
  }
}
