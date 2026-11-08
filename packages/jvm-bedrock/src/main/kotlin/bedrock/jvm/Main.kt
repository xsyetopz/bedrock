package bedrock.jvm

import java.io.File
import kotlin.system.exitProcess

private const val USAGE =
  """usage:
  jvm-bedrock new <dir> [--kind <kind>] [--group <group>]
  jvm-bedrock adopt [<dir>] [--dry-run] [--yes] [--group <group>]

adopt migrates a Maven, sbt, Ant or Ivy, or Gradle build to the Gradle Kotlin DSL and the bedrock
plugins. --group sets the package the code lives in, which gradle.properties keeps.
The kinds are java-library, java-application, java-native, kotlin-library (the default),
kotlin-application, kotlin-native, kmp, cmp, and android."""

private val KINDS =
  listOf(
    "java-library",
    "java-application",
    "java-native",
    "kotlin-library",
    "kotlin-application",
    "kotlin-native",
    "kmp",
    "cmp",
    "android",
  )

internal class Options(
  val positional: List<String>,
  val kind: String?,
  val group: String?,
  val dryRun: Boolean,
  val yes: Boolean,
)

internal fun parse(args: List<String>): Options {
  val positional = ArrayList<String>()
  var kind: String? = null
  var group: String? = null
  var dryRun = false
  var yes = false
  val rest = args.iterator()
  while (rest.hasNext()) {
    val arg = rest.next()
    fun value(): String =
      if (rest.hasNext()) rest.next() else throw IllegalArgumentException("$arg needs a value")
    when {
      arg == "--kind" -> kind = value()
      arg == "--group" -> group = value()
      arg == "--dry-run" -> dryRun = true
      arg == "--yes" -> yes = true
      arg.startsWith("-") -> throw IllegalArgumentException("unknown option $arg")
      else -> positional.add(arg)
    }
  }
  group?.let {
    require(Regex("""[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)*""").matches(it)) {
      "$it is not a package name"
    }
  }
  return Options(positional, kind, group, dryRun, yes)
}

/** Runs the jvm-bedrock command line. */
fun main(args: Array<String>) {
  try {
    val options = parse(args.drop(1))
    when (args.firstOrNull()) {
      "adopt" -> adopt(File(options.positional.firstOrNull() ?: "."), options)
      "new" -> new(options)
      "help",
      "--help",
      "-h" -> println(USAGE)
      null -> throw IllegalArgumentException(USAGE)
      else -> throw IllegalArgumentException("unknown command ${args[0]}\n$USAGE")
    }
  } catch (error: IllegalArgumentException) {
    System.err.println(error.message)
    exitProcess(1)
  }
}

private fun adopt(root: File, options: Options) {
  val (steps, notes) = plan(root, options.group)
  val todo = pending(root, steps)
  todo.forEach { println(it.describe(root)) }
  if (options.dryRun) {
    notes.forEach(::println)
    return
  }
  if (todo.isNotEmpty() && !options.yes) {
    val console =
      System.console()
        ?: throw IllegalArgumentException("Nothing changed. Rerun with --yes to apply.")
    if (console.readLine("Apply these changes? [y/N] ")?.trim()?.lowercase() != "y") {
      throw IllegalArgumentException("Nothing changed.")
    }
  }
  todo.forEach { it.apply(root) }
  notes.forEach(::println)
  println(
    "Done. Run `just setup`, then `just verify`, and fix what it reports. Compare each *.bak file with its replacement and delete it; move what a removed Makefile or hook did into project.just.",
  )
}

/** A package name from a directory name: lowercase letters and digits, starting with a letter. */
internal fun packageName(name: String): String =
  name
    .lowercase()
    .filter { it in 'a'..'z' || it in '0'..'9' }
    .trimStart { it in '0'..'9' }
    .ifEmpty { "app" }

private fun new(options: Options) {
  val directory =
    options.positional.singleOrNull()
      ?: throw IllegalArgumentException("new takes one directory\n$USAGE")
  val kind = options.kind ?: "kotlin-library"
  require(kind in KINDS) { "unknown kind $kind\n$USAGE" }
  val root = File(directory)
  require(!File(root, "settings.gradle.kts").exists()) {
    "$directory has a Gradle build already: run jvm-bedrock adopt"
  }
  val name = root.absoluteFile.normalize().name
  val group = options.group ?: "com.example.${packageName(name)}"
  fun fill(text: String) =
    text
      .replace("__package__", group)
      .replace("__path__", group.replace('.', '/'))
      .replace("__name__", name)
  val files = Bundle.under("kinds/$kind")
  val steps = ArrayList<Step>()
  for (path in files) {
    steps.add(Step.Write(fill(path), fill(Bundle.text("kinds/$kind/$path"))))
  }
  val modules = files.map { it.substringBefore('/') }.distinct()
  val settings =
    modules.joinToString(", ", "rootProject.name = \"$name\"\n\ninclude(", ")\n") { "\":$it\"" }
  steps.add(
    Step.Write(
      "settings.gradle.kts",
      upsert(
        settings,
        "",
        Syntax.SLASH,
        listOf(Block("jvm-bedrock", Bundle.text("blocks/settings.gradle.kts"))),
        first = true,
      ),
    ),
  )
  steps.add(Step.Write("gradle.properties", "group=$group\n"))
  for (step in steps) {
    println(step.describe(root))
    step.apply(root)
  }
  if (!File(root, ".git").exists()) {
    check(
      ProcessBuilder("git", "init", "--quiet").directory(root).inheritIO().start().waitFor() == 0,
    ) {
      "git init failed"
    }
  }
  adopt(root, Options(emptyList(), null, group, dryRun = false, yes = true))
}
