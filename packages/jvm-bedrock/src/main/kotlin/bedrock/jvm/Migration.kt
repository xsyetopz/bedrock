package bedrock.jvm

import java.io.File

/** A dependency in Gradle terms: a library, another module, a platform, or local jars. */
internal sealed class Dependency(val configuration: String) {
  class Library(configuration: String, val group: String, val name: String, val version: String?) :
    Dependency(configuration) {
    val module: String
      get() = "$group:$name"
  }

  class Platform(configuration: String, val library: Library) : Dependency(configuration)

  class Module(configuration: String, val path: String) : Dependency(configuration)

  class Jars(configuration: String, val paths: List<String>) : Dependency(configuration)

  /** Kotlin DSL notation kept as written, such as a catalog accessor. */
  class Notation(configuration: String, val text: String) : Dependency(configuration)
}

/** One Gradle project the old build described, at [directory] relative to the root ("" for it). */
internal class Module(val directory: String) {
  var kotlin: Boolean = false
  var mainClass: String? = null

  /** Whether its build script applies bedrock already, so render leaves it as it is. */
  var kept: Boolean = false
  val dependencies: MutableList<Dependency> = ArrayList()

  /** The Gradle path, such as `:core`, for a module below the root. */
  val gradlePath: String
    get() = ":" + directory.replace('/', ':')
}

/** What a build tool's files turn into, and what adopt could not carry over. */
internal class Migration(val tool: String) {
  var group: String? = null
  val modules: MutableList<Module> = ArrayList()
  val steps: MutableList<Step> = ArrayList()
  val notes: MutableSet<String> = LinkedHashSet()
}

/** Libraries the bedrock plugins already add. */
private val PROVIDED =
  Regex(
    """^(org\.jetbrains\.kotlin:kotlin-(stdlib|test)\S*|junit:junit|org\.testng:testng|org\.junit(\.\w+)*:\S+)$""",
  )

private val OLD_TESTS = setOf("junit:junit", "org.testng:testng")

/** The language levels newer than the Java 8 bytecode bedrock compiles to. */
internal fun javaLevelNote(file: String, level: String?, notes: MutableSet<String>) {
  val number = level?.removePrefix("1.")?.toIntOrNull() ?: return
  if (number > 8) {
    notes.add(
      "$file: compiled for Java $number; bedrock compiles main code to Java 8, so the compilers reject newer APIs and syntax",
    )
  }
}

/** Scala and Groovy sources, which the bedrock plugins do not compile. */
internal fun foreignSources(root: File): List<String> =
  root
    .walkTopDown()
    .onEnter { it.name !in setOf("build", "target", "node_modules") && !it.name.startsWith(".") }
    .filter {
      it.isDirectory &&
        (it.name == "scala" || it.name == "groovy") &&
        it.parentFile.parentFile?.name == "src"
    }
    .map { it.relativeTo(root).path }
    .toList()

/**
 * The steps that write [migration]'s build files, with each library in gradle/libs.versions.toml.
 */
internal fun render(root: File, migration: Migration): List<Step> {
  val aliases = LinkedHashMap<String, Dependency.Library>()
  fun alias(library: Dependency.Library): String {
    aliases.entries
      .firstOrNull { it.value.module == library.module && it.value.version == library.version }
      ?.let {
        return it.key
      }
    // Gradle cannot make accessors for both foo and foo-bar.
    fun taken(alias: String) =
      aliases.keys.any { it == alias || it.startsWith("$alias-") || alias.startsWith("$it-") }
    val base = aliasOf(library.name)
    val name =
      if (taken(base)) aliasOf(library.group.substringAfterLast('.') + "-" + library.name) else base
    var unique = name
    var count = 2
    while (taken(unique)) unique = "$name${count++}"
    aliases[unique] = library
    return unique
  }
  val steps = ArrayList(migration.steps)
  for (module in migration.modules.filterNot(Module::kept)) {
    val lines = ArrayList<String>()
    for (dependency in module.dependencies) {
      if (dependency is Dependency.Library && dependency.module in OLD_TESTS) {
        migration.notes.add(
          "${module.gradlePath}: tests used ${dependency.module}; bedrock runs JUnit 6, so port them to org.junit.jupiter.api, which just guard checks",
        )
      }
      if (dependency is Dependency.Library && PROVIDED.matches(dependency.module)) continue
      val notation =
        when (dependency) {
          is Dependency.Library -> "libs." + alias(dependency).replace('-', '.')
          is Dependency.Platform -> "platform(libs.${alias(dependency.library).replace('-', '.')})"
          is Dependency.Module -> "project(\"${dependency.path}\")"
          is Dependency.Jars -> dependency.paths.joinToString(", ", "files(", ")") { "\"$it\"" }
          is Dependency.Notation -> dependency.text
        }
      lines.add("  ${dependency.configuration}($notation)")
    }
    val plugins =
      listOfNotNull(
        if (module.kotlin) "bedrock.kotlin" else "bedrock.java",
        module.mainClass?.let { "bedrock.application" },
      )
    val script = StringBuilder()
    script.append(
      if (plugins.size == 1) "plugins { id(\"${plugins[0]}\") }\n"
      else plugins.joinToString("\n", "plugins {\n", "\n}\n") { "  id(\"$it\")" },
    )
    module.mainClass?.let { script.append("\napplication { mainClass = \"$it\" }\n") }
    if (lines.isNotEmpty())
      script.append(lines.distinct().joinToString("\n", "\ndependencies {\n", "\n}\n"))
    val path =
      if (module.directory.isEmpty()) "build.gradle.kts" else "${module.directory}/build.gradle.kts"
    steps.add(Step.Write(path, script.toString()))
  }
  if (aliases.isNotEmpty()) {
    val catalog = File(root, "gradle/libs.versions.toml").takeIf(File::isFile)?.readText()
    val entries =
      aliases.entries.joinToString("\n", postfix = "\n") { (name, library) ->
        "$name = { module = \"${library.module}\"" +
          (library.version?.let { ", version = \"$it\"" } ?: "") +
          " }"
      }
    steps.add(
      Step.Write(
        "gradle/libs.versions.toml",
        if (catalog == null) "[libraries]\n$entries"
        else catalog.trimEnd() + "\n\n# Added by jvm-bedrock adopt.\n$entries",
      ),
    )
  }
  return steps
}

/**
 * A catalog alias Gradle can make accessors for: lowercase segments joined by `-`, each starting
 * with a letter, so log4j-1.2-api becomes log4j12-api.
 */
internal fun aliasOf(name: String): String {
  val segments = ArrayList<String>()
  for (part in name.lowercase().split(Regex("[^a-z0-9]+")).filter(String::isNotEmpty)) {
    if (part[0].isDigit() && segments.isNotEmpty()) segments[segments.size - 1] += part
    else segments.add(part)
  }
  val joined = segments.joinToString("-").ifEmpty { "library" }
  return when {
    !joined[0].isLetter() -> "lib-$joined"
    segments.firstOrNull() in setOf("bundles", "versions", "plugins") -> "lib-$joined"
    else -> joined
  }
}

/** The settings lines that include each module below the root. */
internal fun includes(migration: Migration): String {
  val paths = migration.modules.filter { it.directory.isNotEmpty() }.map { it.gradlePath }
  return if (paths.isEmpty()) "" else paths.joinToString(", ", "include(", ")\n") { "\"$it\"" }
}
