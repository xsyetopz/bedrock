// Managed by jvm-bedrock: an application that ProGuard shrinks, optimizes, and obfuscates
// into one jar, build/libs/<name>-min.jar. Apply it after bedrock.java or bedrock.kotlin.
import bedrock.ShrinkJar
import bedrock.library

plugins { application }

val proguardTool = configurations.dependencyScope("proguard")
val proguardToolClasspath =
  configurations.resolvable("proguardClasspath") { extendsFrom(proguardTool.get()) }

dependencies { proguardTool(library("proguard")) }

val shrinkJar =
  tasks.register<ShrinkJar>("shrinkJar") {
    description = "Shrinks, optimizes, and obfuscates the application into one jar."
    group = "build"
    jar = tasks.jar.flatMap { it.archiveFile }
    runtimeClasspath.from(configurations.runtimeClasspath)
    compileClasspath.from(configurations.compileClasspath)
    proguardClasspath.from(proguardToolClasspath)
    mainClass = application.mainClass
    rules.from(
      isolated.rootProject.projectDirectory.file("gradle/bedrock.pro"),
      layout.projectDirectory.file("proguard-rules.pro"),
    )
    launcher = javaToolchains.launcherFor(java.toolchain)
    output = layout.buildDirectory.file("libs/${project.name}-min.jar")
    mapping = layout.buildDirectory.file("proguard/mapping.txt")
  }

tasks.jar { manifest { attributes("Main-Class" to application.mainClass) } }

tasks.assemble { dependsOn(shrinkJar) }
