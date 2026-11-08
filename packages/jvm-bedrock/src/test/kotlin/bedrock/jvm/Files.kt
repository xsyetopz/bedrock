package bedrock.jvm

import java.io.File
import java.nio.file.Files

/** A new empty directory with [files] written into it, by path relative to it. */
internal fun tree(files: Map<String, String> = emptyMap()): File {
  val root = Files.createTempDirectory("jvm-bedrock").toFile().canonicalFile
  root.deleteOnExit()
  for ((path, text) in files) {
    val file = root.resolve(path)
    file.parentFile.mkdirs()
    file.writeText(text)
  }
  return root
}

/** The text a Write step in [steps] puts at [path]. */
internal fun written(steps: List<Step>, path: String): String =
  String((steps.filterIsInstance<Step.Write>().single { it.path == path }).content, Charsets.UTF_8)

internal fun removed(steps: List<Step>): List<String> =
  steps.filterIsInstance<Step.Remove>().map { it.path }
