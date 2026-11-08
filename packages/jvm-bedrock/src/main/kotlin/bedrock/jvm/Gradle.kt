package bedrock.jvm

import java.io.File

private const val QUOTED = """['"]([^'"]+)['"]"""
private val INCLUDE = Regex("""^\s*include\s*\(?(.+?)\)?\s*$""", RegexOption.MULTILINE)
private val DEPENDENCY = Regex("""^\s*(\w+)\s*[ (]\s*(.+?)\s*$""")
private val COORDINATES = Regex("""^\(?$QUOTED\)?$""")
private val MAP =
  Regex(
    """group\s*[:=]\s*$QUOTED\s*,\s*name\s*[:=]\s*$QUOTED(?:\s*,\s*version\s*[:=]\s*$QUOTED)?""",
  )
private val PROJECT = Regex("""project\s*\(\s*(?:path\s*[:=]\s*)?$QUOTED\s*\)""")
private val PLATFORM = Regex("""^\(?(?:enforced)?[pP]latform\s*\(\s*$QUOTED\s*\)\)?$""")
private val CATALOG = Regex("""^\(?(libs(?:\.\w+)+)\)?$""")
private val MAIN_CLASS = Regex("""mainClass(?:Name)?(?:\s*=\s*|\.set\(\s*|\s+)$QUOTED""")
private val GROUP = Regex("""^group\s*=?\s*$QUOTED""", RegexOption.MULTILINE)
private val LEVEL =
  Regex(
    """(?:sourceCompatibility|targetCompatibility|jvmTarget)\s*=\s*['"]?(?:JavaVersion\.VERSION_)?([\d_.]+)|(?:jvmToolchain|JavaLanguageVersion\.of)\s*\(\s*(\d+)""",
  )

/** Gradle's own names for configurations that Gradle 7 removed, and the ones it keeps. */
private val CONFIGURATIONS =
  mapOf(
    "compile" to "api",
    "testCompile" to "testImplementation",
    "runtime" to "runtimeOnly",
    "testRuntime" to "testRuntimeOnly",
  ) +
    listOf(
        "api",
        "implementation",
        "compileOnly",
        "compileOnlyApi",
        "runtimeOnly",
        "annotationProcessor",
        "testImplementation",
        "testCompileOnly",
        "testRuntimeOnly",
        "testAnnotationProcessor",
      )
      .associateWith { it }

/** Whether [root] has a Gradle build that applies no bedrock plugin yet. */
internal fun isGradle(root: File): Boolean =
  listOf("settings.gradle", "build.gradle").any { File(root, it).isFile } ||
    listOf("settings.gradle.kts", "build.gradle.kts").any { name ->
      File(root, name).let { it.isFile && !it.readText().contains("bedrock.") }
    }

/**
 * Converts a Gradle build in either DSL onto the bedrock plugins. Build scripts are code, so this
 * reads the plugin, mainClass, and dependency lines each module's script has, and names the others
 * in the notes; the old scripts stay as .bak files.
 */
internal fun gradle(root: File): Migration {
  val migration = Migration("Gradle")
  val settings =
    listOf("settings.gradle.kts", "settings.gradle")
      .map { File(root, it) }
      .firstOrNull(File::isFile)
  val directories =
    settings
      ?.readText()
      ?.let { text ->
        INCLUDE.findAll(text)
          .flatMap { Regex(QUOTED).findAll(it.groupValues[1]) }
          .map { it.groupValues[1] }
      }
      .orEmpty()
      .map { it.trim(':').replace(':', '/') }
      .toList()
  if (settings?.name == "settings.gradle") migration.steps.add(Step.Remove("settings.gradle"))
  val withRoot =
    if (File(root, "src").isDirectory || directories.isEmpty()) listOf("") + directories
    else directories
  for (directory in (listOf("") + directories).distinct()) {
    val base = if (directory.isEmpty()) "" else "$directory/"
    val groovy = File(root, "${base}build.gradle")
    val kotlinScript = File(root, "${base}build.gradle.kts")
    val script = listOf(groovy, kotlinScript).firstOrNull(File::isFile) ?: continue
    val text = script.readText()
    val path = "$base${script.name}"
    if (directory.isEmpty()) GROUP.find(text)?.let { migration.group = it.groupValues[1] }
    if (directory !in withRoot) {
      // The root holds no sources, so what it sets for the others is all it does.
      migration.steps.add(Step.Remove(path))
      if (Regex("""\b(allprojects|subprojects)\b""").containsMatchIn(text)) {
        migration.notes.add(
          "$path: allprojects or subprojects settings; the bedrock plugins set what each module needs, so move the rest into the modules",
        )
      }
      continue
    }
    if (text.contains("id(\"bedrock.")) {
      migration.modules.add(Module(directory).also { it.kept = true })
      continue
    }
    if (text.contains("com.android.") || text.contains("multiplatform")) {
      migration.notes.add(
        "$path: an Android or multiplatform build; apply bedrock.android, bedrock.kmp, or bedrock.cmp to it by hand",
      )
      continue
    }
    if (script == groovy) migration.steps.add(Step.Remove(path))
    val module = Module(directory)
    module.kotlin =
      Regex("""kotlin\(\s*"jvm"\s*\)|org\.jetbrains\.kotlin\.jvm|kotlin-gradle-plugin""")
        .containsMatchIn(text) || File(root, "${base}src/main/kotlin").isDirectory
    module.mainClass = MAIN_CLASS.find(text)?.groupValues?.get(1)
    LEVEL.find(text)?.let {
      javaLevelNote(
        path,
        it.groupValues[1].ifEmpty { it.groupValues[2] }.replace('_', '.'),
        migration.notes,
      )
    }
    val block =
      Regex("""(?m)^dependencies\s*\{\s*\n(.*?)^}""", RegexOption.DOT_MATCHES_ALL)
        .find(text)
        ?.groupValues
        ?.get(1)
        .orEmpty()
    // The bedrock plugins add the Kotlin standard library and test library.
    for (line in
      block.lines().map(String::trim).filter {
        it.isNotEmpty() && !it.startsWith("//") && !it.contains("kotlin(")
      }) {
      val match = DEPENDENCY.find(line)
      val configuration = match?.let { CONFIGURATIONS[it.groupValues[1]] }
      if (match == null || configuration == null) {
        migration.notes.add("$path: kept out \"$line\"; add it to the new build by hand")
        continue
      }
      val argument = match.groupValues[2]
      val dependency =
        COORDINATES.find(argument)?.groupValues?.get(1)?.let { library(configuration, it) }
          ?: MAP.find(argument)?.destructured?.let { (group, name, version) ->
            Dependency.Library(configuration, group, name, version.ifEmpty { null })
          }
          ?: PROJECT.find(argument)?.let {
            Dependency.Module(configuration, ":" + it.groupValues[1].trim(':'))
          }
          ?: PLATFORM.find(argument)?.let { found ->
            library(configuration, found.groupValues[1])?.let {
              Dependency.Platform(configuration, it)
            }
          }
          ?: CATALOG.find(argument)?.let { Dependency.Notation(configuration, it.groupValues[1]) }
      if (dependency != null) module.dependencies.add(dependency)
      else migration.notes.add("$path: kept out \"$line\"; add it to the new build by hand")
    }
    migration.modules.add(module)
  }
  return migration
}

private fun library(configuration: String, coordinates: String): Dependency.Library? {
  val parts = coordinates.split(':')
  if (parts.size < 2) return null
  return Dependency.Library(configuration, parts[0], parts[1], parts.getOrNull(2))
}
