package bedrock

import java.io.File
import javax.inject.Inject
import org.gradle.api.DefaultTask
import org.gradle.api.file.ConfigurableFileCollection
import org.gradle.api.file.RegularFileProperty
import org.gradle.api.provider.Property
import org.gradle.api.tasks.CacheableTask
import org.gradle.api.tasks.Classpath
import org.gradle.api.tasks.Input
import org.gradle.api.tasks.InputFiles
import org.gradle.api.tasks.Nested
import org.gradle.api.tasks.OutputFile
import org.gradle.api.tasks.PathSensitive
import org.gradle.api.tasks.PathSensitivity
import org.gradle.api.tasks.TaskAction
import org.gradle.jvm.toolchain.JavaLauncher
import org.gradle.process.ExecOperations

/**
 * Shrinks, optimizes, and obfuscates an application and its runtime classpath into one jar with
 * ProGuard. The JDK's java.* modules are the library classes.
 */
@CacheableTask
public abstract class ShrinkJar @Inject constructor(private val exec: ExecOperations) :
  DefaultTask() {
  /** The application's own jar. */
  @get:Classpath public abstract val jar: RegularFileProperty

  /** The jars the application runs with. */
  @get:Classpath public abstract val runtimeClasspath: ConfigurableFileCollection

  /** The jars the application compiles against; those not on the runtime classpath are libraries. */
  @get:Classpath public abstract val compileClasspath: ConfigurableFileCollection

  /** ProGuard itself. */
  @get:Classpath public abstract val proguardClasspath: ConfigurableFileCollection

  /** The class whose main method starts the application. */
  @get:Input public abstract val mainClass: Property<String>

  /** The bedrock rules, then the project's own proguard-rules.pro when it exists. */
  @get:InputFiles
  @get:PathSensitive(PathSensitivity.NONE)
  public abstract val rules: ConfigurableFileCollection

  /** The JDK that runs ProGuard and supplies the library classes. */
  @get:Nested public abstract val launcher: Property<JavaLauncher>

  /** The shrunk jar. */
  @get:OutputFile public abstract val output: RegularFileProperty

  /** The obfuscation map, for reading stack traces. */
  @get:OutputFile public abstract val mapping: RegularFileProperty

  @TaskAction
  internal fun shrink() {
    val home = launcher.get().metadata.installationPath.asFile
    val jmods =
      home.resolve("jmods").listFiles { file -> file.name.matches(Regex("java\\..+\\.jmod")) }
        ?.sorted()
        .orEmpty()
    check(jmods.isNotEmpty()) { "$home has no java.*.jmod files for ProGuard to read." }
    // Metadata only Kotlin reflection and Maven read, and signatures that shrinking breaks.
    val skip =
      listOf(
          "!META-INF/versions/**",
          "!META-INF/maven/**",
          "!META-INF/*.SF",
          "!META-INF/*.DSA",
          "!META-INF/*.RSA",
          "!META-INF/*.kotlin_module",
          "!**.kotlin_builtins",
          "!module-info.class",
        )
        .joinToString(",")
    val runtime = runtimeClasspath.files.filter(File::exists)
    val config = temporaryDir.resolve("bedrock.pro")
    config.writeText(
      buildString {
        appendLine("-injars ${quote(jar.get().asFile)}($skip)")
        runtime.forEach { appendLine("-injars ${quote(it)}(!META-INF/MANIFEST.MF,$skip)") }
        (compileClasspath.files - runtime.toSet()).filter(File::isFile).forEach {
          appendLine("-libraryjars ${quote(it)}")
        }
        appendLine("-outjars ${quote(output.get().asFile)}")
        jmods.forEach { appendLine("-libraryjars ${quote(it)}(!**.jar;!module-info.class)") }
        appendLine("-printmapping ${quote(mapping.get().asFile)}")
        appendLine(
          "-keep class ${mainClass.get()} { public static void main(java.lang.String[]); }"
        )
        rules.files.filter(File::exists).forEach { appendLine("-include ${quote(it)}") }
      }
    )
    exec
      .javaexec {
        executable = launcher.get().executablePath.asFile.path
        classpath = proguardClasspath
        mainClass.set("proguard.ProGuard")
        args("@${config.path}")
      }
      .assertNormalExitValue()
  }

  private fun quote(file: File): String = "'${file.path}'"
}
