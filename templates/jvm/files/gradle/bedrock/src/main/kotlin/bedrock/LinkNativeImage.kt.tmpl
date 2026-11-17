package bedrock

import java.nio.file.Files
import org.gradle.api.DefaultTask
import org.gradle.api.provider.Property
import org.gradle.api.tasks.Internal
import org.gradle.api.tasks.TaskAction
import org.gradle.jvm.toolchain.JavaLauncher

/**
 * Restores the native-image symlinks in a GraalVM that Gradle provisioned. Gradle unpacks a
 * toolchain archive without its symlinks, which leaves bin/native-image an empty file.
 */
public abstract class LinkNativeImage : DefaultTask() {
  /** The GraalVM to repair. */
  @get:Internal public abstract val launcher: Property<JavaLauncher>

  init {
    doNotTrackState("It repairs the toolchain in place.")
  }

  @TaskAction
  internal fun link() {
    val home = launcher.get().metadata.installationPath.asFile.toPath()
    val links =
      mapOf(
        "bin/native-image" to "../lib/svm/bin/native-image",
        "bin/native-image-configure" to "../lib/svm/bin/native-image-configure",
        "bin/native-image-utils" to "../lib/svm/bin/native-image-utils",
        "lib/svm/bin/native-image-configure" to "native-image-utils",
      )
    for ((name, target) in links) {
      val link = home.resolve(name)
      val broken = Files.isRegularFile(link, java.nio.file.LinkOption.NOFOLLOW_LINKS) && Files.size(link) == 0L
      if (broken && Files.exists(link.resolveSibling(target))) {
        Files.delete(link)
        Files.createSymbolicLink(link, link.fileSystem.getPath(target))
      }
    }
  }
}
