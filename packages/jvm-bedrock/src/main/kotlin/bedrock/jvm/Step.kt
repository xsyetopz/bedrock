package bedrock.jvm

import java.io.File
import java.nio.file.Files
import java.nio.file.LinkOption
import java.nio.file.Paths
import java.util.Arrays

/** One change adopt or new makes, which it can print instead of applying. */
internal sealed class Step(val path: String) {
  class Write(path: String, val content: ByteArray, val executable: Boolean = false) : Step(path) {
    constructor(path: String, text: String) : this(path, text.toByteArray(Charsets.UTF_8))
  }

  class Remove(path: String) : Step(path)

  class Link(path: String, val target: String) : Step(path)

  /** Renames a file whose content stays as it is, so it needs no backup. */
  class Move(val from: String, path: String) : Step(path)

  fun describe(root: File): String =
    when (this) {
      is Write ->
        if (exists(root.resolve(path))) "update  $path (kept as $path.bak)" else "create  $path"
      is Remove -> "remove  $path (kept as $path.bak)"
      is Link -> "link    $path -> $target"
      is Move -> "move    $from -> $path"
    }

  /** Whether [root] already matches this step. */
  fun isDone(root: File): Boolean {
    val file = root.resolve(path)
    return when (this) {
      is Write ->
        file.isFile &&
          !Files.isSymbolicLink(file.toPath()) &&
          Arrays.equals(file.readBytes(), content) &&
          (!executable || file.canExecute())
      is Remove -> !exists(file)
      is Link ->
        Files.isSymbolicLink(file.toPath()) &&
          Files.readSymbolicLink(file.toPath()).toString() == target
      is Move -> !exists(root.resolve(from))
    }
  }

  /**
   * Applies the step, keeping a file it replaces or removes as `<file>.bak`. An existing backup
   * stays, since it holds the file from before the first adopt.
   */
  fun apply(root: File) {
    val file = root.resolve(path)
    file.parentFile.mkdirs()
    if (this is Move) {
      check(root.resolve(from).renameTo(file)) { "Could not move $from to $path" }
      return
    }
    if (exists(file)) {
      val backup = root.resolve("$path.bak")
      when {
        !exists(backup) -> check(file.renameTo(backup)) { "Could not back $path up" }
        Files.isSymbolicLink(file.toPath()) -> Files.delete(file.toPath())
        else -> file.deleteRecursively()
      }
    }
    when (this) {
      is Write -> {
        file.writeBytes(content)
        if (executable) file.setExecutable(true, false)
      }
      is Link -> Files.createSymbolicLink(file.toPath(), Paths.get(target))
      is Remove,
      is Move -> {}
    }
  }

  private fun exists(file: File): Boolean = Files.exists(file.toPath(), LinkOption.NOFOLLOW_LINKS)
}

/** The steps [root] does not match yet. */
internal fun pending(root: File, steps: List<Step>): List<Step> = steps.filterNot {
  it.isDone(root)
}
