package bedrock.jvm

import java.io.File

/**
 * Converts build.xml and ivy.xml. Ant names its source folders in javac tasks, so the sources move
 * to Gradle's src/main and src/test; jars in lib/ stay as files() until they get coordinates.
 */
internal fun ant(root: File): Migration {
  val migration = Migration("Ant")
  val module = Module("")
  migration.modules.add(module)
  val build = File(root, "build.xml")
  if (build.isFile) {
    val project = xml(build)
    val properties = HashMap<String, String>()
    descendants(project, "property").forEach {
      if (it.hasAttribute("name") && it.hasAttribute("value"))
        properties[it.getAttribute("name")] = it.getAttribute("value")
    }
    fun resolve(value: String) =
      value.replace(Regex("""\$\{([^}]+)}""")) { properties[it.groupValues[1]] ?: it.value }
    module.mainClass =
      descendants(project, "attribute")
        .firstOrNull { it.getAttribute("name") == "Main-Class" }
        ?.let { resolve(it.getAttribute("value")) }
    val sources = LinkedHashSet<String>()
    descendants(project, "javac").forEach { javac ->
      if (javac.hasAttribute("srcdir"))
        sources.addAll(resolve(javac.getAttribute("srcdir")).split(':', ','))
      descendants(javac, "src").forEach {
        if (it.hasAttribute("path")) sources.add(resolve(it.getAttribute("path")))
      }
      listOf("release", "target", "source").firstOrNull(javac::hasAttribute)?.let {
        javaLevelNote("build.xml", resolve(javac.getAttribute(it)), migration.notes)
      }
    }
    for (source in sources.map { it.trim().trim('/') }.filter(String::isNotEmpty)) {
      val set = if (source.contains("test")) "test" else "main"
      if (source == "src/$set/java") continue
      val directory = File(root, source)
      val files = directory.walkTopDown().filter(File::isFile).toList()
      for (file in files) {
        val relative = file.relativeTo(directory).path
        val kind = if (file.extension == "java") "java" else "resources"
        migration.steps.add(Step.Move("$source/$relative", "src/$set/$kind/$relative"))
      }
    }
    migration.steps.add(Step.Remove("build.xml"))
  }
  val ivy = File(root, "ivy.xml")
  if (ivy.isFile) {
    val ivyModule = xml(ivy)
    descendants(ivyModule, "info")
      .firstOrNull()
      ?.getAttribute("organisation")
      ?.ifEmpty { null }
      ?.let { migration.group = it }
    for (dependency in descendants(ivyModule, "dependency")) {
      val conf = dependency.getAttribute("conf")
      val configuration =
        when {
          conf.contains("test") -> "testImplementation"
          conf.contains("provided") -> "compileOnly"
          conf.startsWith("runtime") -> "runtimeOnly"
          else -> "implementation"
        }
      module.dependencies.add(
        Dependency.Library(
          configuration,
          dependency.getAttribute("org"),
          dependency.getAttribute("name"),
          dependency.getAttribute("rev").ifEmpty { null },
        ),
      )
    }
    migration.steps.add(Step.Remove("ivy.xml"))
  }
  val jars =
    File(root, "lib")
      .listFiles { file -> file.extension == "jar" }
      .orEmpty()
      .map { "lib/${it.name}" }
      .sorted()
  if (jars.isNotEmpty()) {
    module.dependencies.add(Dependency.Jars("implementation", jars))
    migration.notes.add(
      "lib/: the build uses ${jars.size} jar(s) as files(); replace each with its Maven coordinates in gradle/libs.versions.toml",
    )
  }
  return migration
}
