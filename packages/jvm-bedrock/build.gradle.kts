plugins {
  id("bedrock.kotlin")
  id("bedrock.application")
}

version = "0.1.1"

application { mainClass = "bedrock.jvm.MainKt" }

// The CLI and the Gradle conventions share this code, so it lives where adopt copies it from.
kotlin.sourceSets.main { kotlin.srcDir("files/jvm/gradle/bedrock/src/main/kotlin/bedrock/core") }

val bundle = layout.projectDirectory.dir("files")
val index =
  tasks.register("bundleIndex") {
    description = "Lists the files jvm-bedrock writes, since a jar cannot list its resources."
    val root = bundle.asFile
    val tree = fileTree(root) { exclude("**/build/**", "**/.gradle/**", "**/.kotlin/**") }
    val output = layout.buildDirectory.file("bundle/files.index")
    inputs.files(tree)
    outputs.file(output)
    doLast {
      output
        .get()
        .asFile
        .writeText(
          tree.files
            .map { it.relativeTo(root).invariantSeparatorsPath }
            .sorted()
            .joinToString("\n", postfix = "\n"),
        )
    }
  }

tasks.processResources {
  from(bundle) {
    into("files")
    exclude("**/build/**", "**/.gradle/**", "**/.kotlin/**")
  }
  from(index) { into("files") }
}
