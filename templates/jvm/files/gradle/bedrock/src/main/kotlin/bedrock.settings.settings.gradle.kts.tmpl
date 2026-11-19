// Managed by bedrock: repositories, the toolchain resolver, the bedrock catalog, and the
// lifecycle tasks just runs in each project, and the repository task j2k.
import bedrock.ConvertJavaToKotlin

plugins { id("org.gradle.toolchains.foojay-resolver-convention") }

dependencyResolutionManagement {
  repositoriesMode = RepositoriesMode.FAIL_ON_PROJECT_REPOS
  repositories {
    mavenCentral()
    // Kotlin/Wasm downloads Node.js and Binaryen as ivy distributions, which
    // FAIL_ON_PROJECT_REPOS would reject when the plugin adds them to each project.
    ivy("https://nodejs.org/dist") {
      name = "Node.js distributions"
      patternLayout { artifact("v[revision]/[artifact](-v[revision]-[classifier]).[ext]") }
      metadataSources { artifact() }
      content { includeModule("org.nodejs", "node") }
    }
    ivy("https://github.com/WebAssembly/binaryen/releases/download") {
      name = "Binaryen distributions"
      patternLayout { artifact("version_[revision]/[artifact]-version_[revision]-[classifier].[ext]") }
      metadataSources { artifact() }
      content { includeModule("com.github.webassembly", "binaryen") }
    }
    google {
      content {
        includeGroupByRegex("""androidx\..*""")
        includeGroupByRegex("""com\.android(\..*)?""")
        includeGroupByRegex("""com\.google\.android\..*""")
        includeGroupByRegex("""com\.google\.testing\.platform(\..*)?""")
      }
    }
  }
  versionCatalogs { create("bedrock") { from(files("gradle/bedrock.versions.toml")) } }
}

val ideaHome = providers.environmentVariable("BEDROCK_IDEA")
val cacheHome = gradle.gradleUserHomeDir.resolve("caches/bedrock/idea")

// Picking tasks from their names leaves the rest unconfigured, which keeps configuration fast.
gradle.lifecycle.beforeProject {
  fun tasksNamed(pick: (String) -> Boolean) = provider { tasks.names.filter(pick) }
  val compile =
    tasks.register("bedrockCompile") {
      description = "Compiles main and test code for every target, with warnings as errors."
      group = "verification"
      dependsOn(tasksNamed { it.startsWith("compile") && !it.contains("AndroidTest") })
    }
  tasks.register("bedrockLint") {
    description = "Runs the compilers' Error Prone and NullAway checks, and Android Lint."
    group = "verification"
    dependsOn(compile, tasksNamed { it == "lint" })
  }
  tasks.register("bedrockTest") {
    description = "Runs the tests every target can run on this machine."
    group = "verification"
    // Android's test runs each build type's unit tests; the debug ones cover the same code.
    dependsOn(
      provider {
        tasks.names.filter { it.endsWith("DebugUnitTest") && it.startsWith("test") }.ifEmpty {
          tasks.names.filter { it == "test" || it == "allTests" }
        }
      }
    )
  }
}

gradle.rootProject {
  tasks.register<ConvertJavaToKotlin>("j2k") {
    description = "Converts Java files to Kotlin with IntelliJ IDEA's J2K: --file <path>."
    root = layout.projectDirectory
    cache = layout.dir(provider { cacheHome })
    idea = ideaHome
  }
}
