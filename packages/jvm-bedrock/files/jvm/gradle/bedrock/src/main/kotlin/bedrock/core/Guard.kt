package bedrock.core

import java.io.File
import java.security.MessageDigest

/** What the compilers and linters cannot see: drift, foreign build tools, and bare suppressions. */
internal object Guard {
  /** The checksum list adopt writes beside the conventions, one `<sha256>  <path>` per line. */
  const val MANIFEST = "gradle/bedrock/managed.sha256"

  private val FOREIGN =
    mapOf(
      "pom.xml" to "Maven",
      "mvnw" to "Maven",
      "mvnw.cmd" to "Maven",
      "build.xml" to "Ant",
      "ivy.xml" to "Ivy",
      "build.sbt" to "sbt",
      "build.gradle" to "Gradle's Groovy DSL",
      "settings.gradle" to "Gradle's Groovy DSL",
    )

  /** Rules that turn off what bedrock.pro is for, or keep the whole program. */
  private val WEAKENING =
    Regex("""^\s*(-dontshrink|-dontoptimize|-dontobfuscate|-ignorewarnings|-keep\S*\s+class\s+\*\*\s*\{?)""")

  private val SUPPRESSION = Regex("""@(file:)?Suppress(Warnings)?\(""")

  private val OLD_TESTS = Regex("""^import\s+(org\.junit\.(Test|Assert|Before|After|Rule|runner)|junit\.framework|org\.testng)\b""")

  fun sha256(bytes: ByteArray): String =
    MessageDigest.getInstance("SHA-256").digest(bytes).joinToString("") { String.format("%02x", it) }

  /** The manifest text for the managed files, sorted by path. */
  fun manifest(files: Map<String, ByteArray>): String =
    files.toSortedMap().entries.joinToString("") { (path, bytes) -> "${sha256(bytes)}  $path\n" }

  /**
   * The problems in [root], whose git-tracked files are [files]. Each starts with the path it is
   * about.
   */
  fun check(root: File, files: List<String>): List<String> {
    val problems = ArrayList<String>()
    val manifest = root.resolve(MANIFEST)
    if (manifest.isFile) {
      for (line in manifest.readLines().filter(String::isNotBlank)) {
        val sum = line.substringBefore("  ")
        val path = line.substringAfter("  ")
        val file = root.resolve(path)
        when {
          !file.isFile -> problems.add("$path: missing; run jvm-bedrock adopt to restore it")
          sha256(file.readBytes()) != sum ->
            problems.add(
              "$path: differs from what jvm-bedrock adopt writes; move the change into project files, then adopt again"
            )
        }
      }
    } else {
      problems.add("$MANIFEST: missing; run jvm-bedrock adopt")
    }
    // Android runs instrumented and unit tests on JUnit 4.
    val android =
      files
        .filter { it.substringAfterLast('/') == "build.gradle.kts" }
        .filter { root.resolve(it).let { file -> file.isFile && "id(\"bedrock.android\")" in file.readText() } }
        .map { it.removeSuffix("build.gradle.kts") }
    for (path in files) {
      val name = path.substringAfterLast('/')
      FOREIGN[name]?.let {
        problems.add("$path: $it build file; the Gradle Kotlin DSL build replaces it, so jvm-bedrock adopt migrates it")
      }
      if (path.startsWith(".mvn/")) problems.add("$path: Maven wrapper; ./gradlew replaces it")
      if (path.endsWith(".gradle")) {
        if (name !in FOREIGN) problems.add("$path: Groovy build script; write it as $path.kts")
        continue
      }
      val file = root.resolve(path)
      if (!file.isFile) continue
      when {
        // An included build, such as gradle/bedrock, has a settings file and cannot apply them.
        name == "build.gradle.kts" && !file.resolveSibling("settings.gradle.kts").isFile ->
          if (!file.readText().contains("id(\"bedrock.")) {
            problems.add("$path: applies no bedrock.* plugin, so it skips the toolchain, Java 8 bytecode, and lints")
          }
        name.endsWith(".pro") && !path.startsWith("gradle/") ->
          file.readLines().forEachIndexed { index, line ->
            if (WEAKENING.containsMatchIn(line)) {
              problems.add("$path:${index + 1}: ${line.trim()} undoes the shrinking and optimizing; keep only the classes reflection needs")
            }
          }
        name.endsWith(".java") || name.endsWith(".kt") -> {
          val junit4 = path.contains("/src/android") || android.any { path.startsWith(it) }
          source(path, file.readLines(), !junit4, problems)
        }
      }
    }
    return problems
  }

  private fun source(
    path: String,
    lines: List<String>,
    junit6: Boolean,
    problems: MutableList<String>,
  ) {
    lines.forEachIndexed { index, line ->
      if (SUPPRESSION.containsMatchIn(line) && !line.contains("//") &&
        lines.getOrNull(index - 1)?.trim()?.startsWith("//") != true
      ) {
        problems.add("$path:${index + 1}: say why in a // comment on the suppression or the line above it")
      }
      if (junit6 && OLD_TESTS.containsMatchIn(line)) {
        problems.add("$path:${index + 1}: JUnit 4 or TestNG; write the test with JUnit 6, org.junit.jupiter.api")
      }
    }
  }
}
