// Managed by jvm-bedrock: Kotlin with ktfmt's Google style, explicit API for libraries,
// and every warning, the extra checkers included, as an error.
import bedrock.java8
import bedrock.strict
import bedrock.version
import org.jetbrains.kotlin.gradle.dsl.ExplicitApiMode
import org.jetbrains.kotlin.gradle.dsl.JvmDefaultMode
import org.jetbrains.kotlin.gradle.dsl.JvmTarget
import org.jetbrains.kotlin.gradle.tasks.KotlinJvmCompile

plugins {
  id("bedrock.jvm")
  id("org.jetbrains.kotlin.jvm")
}

kotlin {
  explicitApi = ExplicitApiMode.Strict
  compilerOptions {
    strict()
    jvmDefault = JvmDefaultMode.NO_COMPATIBILITY
    freeCompilerArgs.add("-Xjsr305=strict")
  }
}

// An application has no API for other code to call.
pluginManager.withPlugin("application") { kotlin.explicitApi = ExplicitApiMode.Disabled }

// The main code runs on Java 8; the tests run on the toolchain.
tasks.withType<KotlinJvmCompile>().configureEach {
  if (name == "compileKotlin") compilerOptions.java8()
  else compilerOptions.jvmTarget = JvmTarget.fromTarget(version("jdk"))
}

dependencies { testImplementation(kotlin("test")) }

spotless {
  kotlin {
    target("src/*/kotlin/**/*.kt", "src/*/java/**/*.kt")
    ktfmt(version("ktfmt")).googleStyle()
  }
}
