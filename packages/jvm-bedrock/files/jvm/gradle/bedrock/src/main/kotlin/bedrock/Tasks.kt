package bedrock

import bedrock.core.Guard
import bedrock.core.J2k
import java.io.File
import javax.inject.Inject
import org.gradle.api.DefaultTask
import org.gradle.api.GradleException
import org.gradle.api.file.DirectoryProperty
import org.gradle.api.provider.ListProperty
import org.gradle.api.provider.Property
import org.gradle.api.tasks.Input
import org.gradle.api.tasks.Internal
import org.gradle.api.tasks.Optional
import org.gradle.api.tasks.TaskAction
import org.gradle.api.tasks.options.Option
import org.gradle.process.ExecOperations

/** Reports drift from what jvm-bedrock adopt writes, foreign build tools, and bare suppressions. */
public abstract class BedrockGuard @Inject constructor(private val exec: ExecOperations) :
  DefaultTask() {
  /** The repository root. */
  @get:Internal public abstract val root: DirectoryProperty

  init {
    doNotTrackState("It reads every file git tracks.")
  }

  @TaskAction
  internal fun guard() {
    val root = root.get().asFile
    val problems = Guard.check(root, tracked(exec, root))
    if (problems.isNotEmpty()) {
      throw GradleException(problems.joinToString("\n", postfix = "\n${problems.size} problem(s)"))
    }
  }
}

/**
 * Converts Java files to Kotlin with IntelliJ IDEA's J2K, run headless, and moves each result
 * from src/<set>/java to src/<set>/kotlin. It replaces each .java file, so it stops when one has
 * changes git has not committed.
 */
public abstract class ConvertJavaToKotlin @Inject constructor(private val exec: ExecOperations) :
  DefaultTask() {
  /** The repository root, which the IDE opens as its project. */
  @get:Internal public abstract val root: DirectoryProperty

  /** Where the IDE download lives between runs. */
  @get:Internal public abstract val cache: DirectoryProperty

  /** The Java files or directories of them to convert. */
  @get:Input
  @get:Option(option = "file", description = "A Java file or a directory of them; repeat it for more.")
  public abstract val files: ListProperty<String>

  /** An installed IntelliJ IDEA to use instead of the download, from BEDROCK_IDEA. */
  @get:Input @get:Optional public abstract val idea: Property<String>

  init {
    doNotTrackState("It rewrites sources in place.")
  }

  @TaskAction
  internal fun convert() {
    // The IDE resolves files against its canonical project path, such as /private/tmp for /tmp.
    val root = root.get().asFile.canonicalFile
    val sources =
      files.get().flatMap { name ->
        val file = File(name).let { if (it.isAbsolute) it else root.resolve(it) }.canonicalFile
        if (file.isDirectory) file.walk().filter { it.extension == "java" }.toList()
        else listOf(file)
      }
    if (sources.isEmpty()) throw GradleException("Name the Java files to convert: --file <path>")
    sources.firstOrNull { !it.isFile || it.extension != "java" }?.let {
      throw GradleException("$it is not a Java file")
    }
    for (module in J2k.modules(root, sources).keys) {
      val script = module.resolve("build.gradle.kts")
      if (!script.isFile || !J2k.hasKotlin(script.readText())) {
        throw GradleException(
          "${script.relativeTo(root)} does not apply Kotlin: add id(\"bedrock.kotlin\") beside bedrock.java, then convert"
        )
      }
    }
    val dirty = changed(exec, root, sources.map { it.relativeTo(root).path })
    if (dirty.isNotEmpty()) {
      throw GradleException("Commit or stash these first, since J2K replaces them:\n${dirty.joinToString("\n")}")
    }
    val home =
      idea.orNull?.let(::File) ?: J2k.install(cache.get().asFile) { logger.lifecycle(it) }
    val state = root.resolve(".gradle/bedrock/idea")
    val result = J2k.convert(home, root, state, sources) { logger.lifecycle(it) }
    for ((java, kotlin) in result.converted) {
      val target = J2k.kotlinPath(kotlin)
      if (target != kotlin) {
        root.resolve(target).parentFile.mkdirs()
        check(root.resolve(kotlin).renameTo(root.resolve(target))) { "Could not move $kotlin to $target" }
      }
      logger.lifecycle("$java -> $target")
    }
    if (result.errors.isNotEmpty()) {
      throw GradleException(result.errors.joinToString("\n") { (path, message) -> "$path: $message" })
    }
    logger.lifecycle(
      "Converted ${result.converted.size} file(s). Run just format and just test, then fix what J2K left: code that does not compile, !!, and nullable types the Java never meant."
    )
  }
}

private fun git(exec: ExecOperations, root: File, vararg args: String): String {
  val out = java.io.ByteArrayOutputStream()
  exec
    .exec {
      workingDir = root
      commandLine(listOf("git") + args)
      standardOutput = out
    }
    .assertNormalExitValue()
  return out.toString(Charsets.UTF_8)
}

private fun tracked(exec: ExecOperations, root: File): List<String> =
  git(exec, root, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
    .split('\u0000')
    .filter { it.isNotEmpty() && root.resolve(it).exists() }

private fun changed(exec: ExecOperations, root: File, paths: List<String>): List<String> =
  git(exec, root, *(listOf("status", "--porcelain", "--") + paths).toTypedArray())
    .lines()
    .filter(String::isNotBlank)
