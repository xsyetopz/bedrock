// Managed by jvm-bedrock: an application that GraalVM Community native-image also compiles
// ahead of time into one executable, build/native/nativeCompile/<name>. nativeTest runs
// the tests in a native image.
import bedrock.LinkNativeImage

plugins {
  id("bedrock.application")
  id("org.graalvm.buildtools.native")
}

val linkNativeImage =
  tasks.register<LinkNativeImage>("linkNativeImage") {
    launcher = javaToolchains.launcherFor(java.toolchain)
  }

tasks.matching { it.name == "nativeCompile" || it.name == "nativeTestCompile" }.configureEach {
  dependsOn(linkNativeImage)
}

graalvmNative {
  toolchainDetection = false
  binaries.all { javaLauncher = javaToolchains.launcherFor(java.toolchain) }
  binaries.named("main") {
    // bedrock.jvm applies java-library, which the plugin takes to mean a shared library.
    sharedLibrary = false
    buildArgs.addAll("-O3", "--no-fallback")
  }
}
