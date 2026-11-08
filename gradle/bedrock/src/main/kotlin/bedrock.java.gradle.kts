// Managed by jvm-bedrock: Java with google-java-format, Error Prone, and NullAway.
import bedrock.library
import bedrock.version
import net.ltgt.gradle.errorprone.CheckSeverity
import net.ltgt.gradle.errorprone.errorprone

plugins {
  id("bedrock.jvm")
  id("net.ltgt.errorprone")
}

dependencies {
  errorprone(library("errorprone"))
  errorprone(library("nullaway"))
  compileOnlyApi(library("jspecify"))
  testCompileOnly(library("jspecify"))
}

tasks.withType<JavaCompile>().configureEach {
  // A library documents its public and protected API; an application need not.
  val library = !project.plugins.hasPlugin("application")
  options.compilerArgs.add(
    if (library && name == "compileJava") "-Xdoclint:all/protected" else "-Xdoclint:all,-missing/protected"
  )
  options.errorprone {
    disableWarningsInGeneratedCode = true
    allDisabledChecksAsWarnings = false
    check("NullAway", CheckSeverity.ERROR)
    // @NullMarked targets ElementType.MODULE, which javac --release 8 warns about, so
    // NullAway treats the project's group as null-marked instead.
    val group = project.group.toString()
    require(group.isNotEmpty()) { "Set group in gradle.properties; NullAway checks that package." }
    option("NullAway:AnnotatedPackages", group)
    option("NullAway:JSpecifyMode", "true")
  }
}

spotless {
  java {
    target("src/*/java/**/*.java")
    googleJavaFormat(version("google-java-format")).reflowLongStrings().formatJavadoc(true)
  }
}
