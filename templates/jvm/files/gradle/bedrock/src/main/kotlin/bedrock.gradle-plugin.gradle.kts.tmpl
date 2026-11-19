// Managed by bedrock: a Gradle plugin, with the Gradle API and TestKit. Apply it after
// bedrock.java or bedrock.kotlin. Gradle's API is Java 17 bytecode, so unlike the other
// bedrock plugins this one cannot target Java 8.
import org.jetbrains.kotlin.gradle.dsl.JvmTarget
import org.jetbrains.kotlin.gradle.tasks.KotlinJvmCompile

plugins { `java-gradle-plugin` }

val pluginJava = 17

// These run after the Java 8 settings of bedrock.java and bedrock.kotlin, which the module applies
// first, so they win.
tasks.withType<JavaCompile>().configureEach { options.release = pluginJava }

pluginManager.withPlugin("org.jetbrains.kotlin.jvm") {
  tasks.withType<KotlinJvmCompile>().configureEach {
    compilerOptions {
      jvmTarget = JvmTarget.fromTarget(pluginJava.toString())
      freeCompilerArgs.set(freeCompilerArgs.get().filterNot { it.startsWith("-Xjdk-release") })
    }
  }
}

dependencies { testImplementation(gradleTestKit()) }
