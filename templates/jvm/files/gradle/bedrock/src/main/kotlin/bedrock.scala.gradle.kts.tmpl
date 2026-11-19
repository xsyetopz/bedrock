// Managed by bedrock: Scala 3 with scalafmt, and every warning, the unused-code ones included,
// as an error.
import bedrock.library
import bedrock.version

plugins {
  id("bedrock.jvm")
  scala
}

dependencies { implementation(library("scala3-library")) }

tasks.withType<ScalaCompile>().configureEach {
  scalaCompileOptions.additionalParameters =
    listOf("-Werror", "-Wunused:all", "-deprecation", "-feature", "-unchecked", "-encoding", "UTF-8")
}

spotless {
  scala {
    target("src/*/scala/**/*.scala")
    targetExclude(".claude/**", "openspec/**")
    scalafmt(version("scalafmt"))
      .configFile(isolated.rootProject.projectDirectory.file("gradle/bedrock.scalafmt.conf"))
  }
}
