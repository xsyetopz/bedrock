// Managed by jvm-bedrock: the toolchain, Java 8 bytecode, JUnit, and Spotless, shared by
// bedrock.java and bedrock.kotlin.
import bedrock.library
import bedrock.version

plugins {
  `java-library`
  id("com.diffplug.spotless")
}

val jdk = version("jdk").toInt()
val bytecode = version("bytecode").toInt()

java {
  toolchain {
    languageVersion = JavaLanguageVersion.of(jdk)
    vendor = JvmVendorSpec.GRAAL_VM
    nativeImageCapable = true
  }
  withSourcesJar()
}

tasks.withType<JavaCompile>().configureEach {
  options.encoding = "UTF-8"
  options.release = if (name == "compileJava") bytecode else jdk
  // -options: newer JDKs call release 8 obsolete; -processing: annotations without a processor.
  options.compilerArgs.addAll(listOf("-Xlint:all,-options,-processing", "-Werror", "-parameters"))
}

dependencies {
  testImplementation(platform(library("junit-bom")))
  testImplementation(library("junit-jupiter"))
  testRuntimeOnly(library("junit-launcher"))
}

tasks.withType<Test>().configureEach {
  useJUnitPlatform()
  maxParallelForks = (Runtime.getRuntime().availableProcessors() / 2).coerceAtLeast(1)
}

spotless {
  kotlinGradle {
    target("*.gradle.kts")
    ktfmt(version("ktfmt")).googleStyle()
  }
}
