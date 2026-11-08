package bedrock.jvm

import java.io.File

private val LIBRARY =
  Regex(""""([^"]+)"\s*(%%?)\s*"([^"]+)"\s*%\s*"([^"]+)"(?:\s*%\s*"?(\w+)"?)?""")
private val PROJECT =
  Regex(
    """lazy\s+val\s+(\w+)\s*=\s*\(?\s*project(?:\s+in\s*|\.in\s*)\(?\s*file\s*\(\s*"([^"]*)"\s*\)""",
  )
private val DEPENDS_ON = Regex("""dependsOn\s*\(([^)]*)\)""")
private val MAIN_CLASS = Regex("""mainClass\s*:=\s*Some\s*\(\s*"([^"]+)"\s*\)""")
private val ORGANIZATION = Regex("""organization\s*:=\s*"([^"]+)"""")
private val JAVAC_RELEASE =
  Regex("""javacOptions\s*\+\+?=\s*Seq\([^)]*"(?:--release|-target)"\s*,\s*"([^"]+)"""")

/**
 * Converts build.sbt. An sbt build is Scala code, so this reads the common forms: settings before
 * the first project apply to every project, and each `lazy val x = project.in(file(…))` owns the
 * text up to the next one.
 */
internal fun sbt(root: File): Migration {
  val migration = Migration("sbt")
  val text = File(root, "build.sbt").readText()
  migration.group = ORGANIZATION.find(text)?.groupValues?.get(1)
  val projects = PROJECT.findAll(text).toList()
  val common = text.substring(0, projects.firstOrNull()?.range?.first ?: text.length)
  val names = projects.associate { it.groupValues[1] to it.groupValues[2].trim('/', '.') }
  val sections =
    if (projects.isEmpty()) listOf("" to text)
    else
      projects.mapIndexed { index, match ->
        names.getValue(match.groupValues[1]) to
          text.substring(
            match.range.first,
            projects.getOrNull(index + 1)?.range?.first ?: text.length,
          )
      }
  for ((directory, section) in sections) {
    val module = Module(directory)
    val source = if (projects.isEmpty()) section else common + section
    module.kotlin = File(root, directory).resolve("src/main/kotlin").isDirectory
    module.mainClass =
      MAIN_CLASS.find(section)?.groupValues?.get(1) ?: MAIN_CLASS.find(common)?.groupValues?.get(1)
    val api = if (module.mainClass == null) "api" else "implementation"
    for (match in LIBRARY.findAll(source)) {
      val (group, cross, name, version, scope) = match.destructured
      if (cross == "%%") {
        migration.notes.add(
          "build.sbt: $group %% $name is a Scala library; bedrock builds Java and Kotlin, so find a Java one",
        )
        continue
      }
      val configuration =
        when (scope.lowercase()) {
          "test" -> "testImplementation"
          "provided" -> "compileOnly"
          "runtime" -> "runtimeOnly"
          else -> api
        }
      module.dependencies.add(Dependency.Library(configuration, group, name, version))
    }
    DEPENDS_ON.find(section)?.groupValues?.get(1)?.split(',')?.forEach { reference ->
      names[reference.trim().substringBefore(' ')]?.let {
        module.dependencies.add(Dependency.Module(api, Module(it).gradlePath))
      }
    }
    JAVAC_RELEASE.find(source)?.let {
      javaLevelNote("build.sbt", it.groupValues[1], migration.notes)
    }
    migration.modules.add(module)
  }
  migration.steps.add(Step.Remove("build.sbt"))
  if (File(root, "project").isDirectory) migration.steps.add(Step.Remove("project"))
  migration.notes.add(
    "build.sbt: sbt builds are code, so compare each new build.gradle.kts with it; project/ held sbt's plugins and build code",
  )
  return migration
}
