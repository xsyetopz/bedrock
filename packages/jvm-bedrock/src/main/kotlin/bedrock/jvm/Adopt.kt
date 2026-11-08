package bedrock.jvm

import bedrock.core.Guard
import java.io.File
import java.nio.file.Files
import java.nio.file.LinkOption

/** Files whose job a managed file took over: hook managers and Make. */
private val REPLACED =
  listOf(
    ".husky",
    ".lefthook.yml",
    ".pre-commit-config.yaml",
    ".simple-git-hooks.json",
    "Makefile",
    "lefthook.yaml",
    "lefthook.yml",
  )

private val BASE =
  listOf(
    "justfile",
    ".githooks/commit-msg",
    ".githooks/pre-commit",
    ".markdownlint.jsonc",
    ".github/workflows/base.yml",
  )

private val ROOT_NAME = Regex("""rootProject\.name\s*=\s*['"]([^'"]+)['"]""")

/** Build tools adopt finds but cannot read. */
private val UNREAD =
  mapOf(
    "build.sc" to "Mill",
    "build.mill" to "Mill",
    "MODULE.bazel" to "Bazel",
    "WORKSPACE" to "Bazel",
  )

private fun blocks(file: String): List<Block> =
  listOf(
    Block("bedrock", Bundle.text("base/blocks/$file")),
    Block("jvm-bedrock", Bundle.text("blocks/$file")),
  )

private fun read(root: File, path: String): String? =
  File(root, path).takeIf(File::isFile)?.readText()

private fun managed(directory: String, path: String): Step.Write {
  val bytes = Bundle.bytes("$directory/$path")
  return Step.Write(path, bytes, executable = path.startsWith(".githooks/") || path == "gradlew")
}

/** The migration for the build tool in [root], or null when it has none or uses bedrock already. */
internal fun migration(root: File): Migration? {
  val foreign = foreignSources(root)
  require(foreign.isEmpty()) {
    "${foreign.joinToString()}: bedrock compiles Java and Kotlin, not Scala or Groovy; port those sources first"
  }
  val migration =
    when {
      File(root, "pom.xml").isFile -> maven(root)
      File(root, "build.sbt").isFile -> sbt(root)
      File(root, "build.xml").isFile || File(root, "ivy.xml").isFile -> ant(root)
      isGradle(root) -> gradle(root)
      else -> null
    }
  val unread = UNREAD.filterKeys { File(root, it).isFile }
  if (unread.isEmpty()) return migration
  val noted = migration ?: Migration(unread.values.first())
  unread.forEach { (file, tool) ->
    noted.notes.add(
      "$file: adopt cannot read $tool builds; write each module's build.gradle.kts by hand",
    )
  }
  return noted
}

/**
 * Every change adopt makes in [root], including ones already in place, and what it could not carry
 * over.
 */
internal fun plan(root: File, group: String?): Pair<List<Step>, Set<String>> {
  val migration = migration(root)
  val steps = ArrayList<Step>()
  BASE.forEach { steps.add(managed("base", it)) }
  if (!File(root, "package.json").exists()) steps.add(managed("base", ".markdownlint-cli2.jsonc"))
  val jvm = Bundle.under("jvm")
  jvm.forEach { steps.add(managed("jvm", it)) }
  steps.add(
    Step.Write(Guard.MANIFEST, Guard.manifest(jvm.associateWith { Bundle.bytes("jvm/$it") })),
  )
  REPLACED.filter { Files.exists(File(root, it).toPath(), LinkOption.NOFOLLOW_LINKS) }
    .forEach { steps.add(Step.Remove(it)) }
  steps.addAll(shared(root))

  // A Gradle build's own settings would apply plugins before bedrock's, so they start over.
  val old = read(root, "settings.gradle.kts") ?: read(root, "settings.gradle")
  val name = old?.let { ROOT_NAME.find(it)?.groupValues?.get(1) } ?: root.canonicalFile.name
  var settings =
    upsert(
      if (migration?.tool == "Gradle") null else read(root, "settings.gradle.kts"),
      "rootProject.name = \"$name\"\n",
      Syntax.SLASH,
      listOf(Block("jvm-bedrock", Bundle.text("blocks/settings.gradle.kts"))),
      first = true,
    )
  migration
    ?.let { includes(it) }
    ?.takeIf { it.isNotEmpty() && !settings.contains(it) }
    ?.let { settings += it }
  steps.add(Step.Write("settings.gradle.kts", settings))

  var properties =
    upsert(
      read(root, "gradle.properties"),
      "",
      Syntax.HASH,
      listOf(Block("jvm-bedrock", Bundle.text("blocks/gradle.properties"))),
    )
  val notes = LinkedHashSet<String>()
  if (!Regex("""(?m)^group\s*=""").containsMatchIn(properties)) {
    val chosen = group ?: migration?.group
    if (chosen != null) properties += "group=$chosen\n"
    else
      notes.add(
        "gradle.properties: set group= to the package the code lives in; NullAway checks it and Android derives namespaces from it",
      )
  }
  steps.add(Step.Write("gradle.properties", properties))

  if (migration != null) {
    steps.addAll(render(root, migration))
    notes.addAll(migration.notes)
  }
  return steps to notes
}

/** AGENTS.md, its links, and the files every bedrock writes a block into. */
private fun shared(root: File): List<Step> {
  val steps = ArrayList<Step>()
  val claude = File(root, "CLAUDE.md")
  val agents =
    read(root, "AGENTS.md")
      ?: claude.takeIf { !Files.isSymbolicLink(it.toPath()) }?.let { read(root, "CLAUDE.md") }
  steps.add(
    Step.Write("AGENTS.md", upsert(agents, "# Agents\n", Syntax.MARKDOWN, blocks("AGENTS.md"))),
  )
  listOf("CLAUDE.md", "GEMINI.md").forEach { steps.add(Step.Link(it, "AGENTS.md")) }
  for ((path, header, file) in
    listOf(
      Triple(".gitignore", "", "gitignore"),
      Triple(".gitattributes", "", "gitattributes"),
      Triple(".editorconfig", "root = true\n", "editorconfig"),
    )) {
    steps.add(Step.Write(path, upsert(read(root, path), header, Syntax.HASH, blocks(file))))
  }
  val dependabot = read(root, ".github/dependabot.yml")?.takeIf { it.contains("# >>> bedrock") }
  steps.add(
    Step.Write(
      ".github/dependabot.yml",
      upsert(dependabot, "version: 2\nupdates:\n", Syntax.HASH, blocks("dependabot.yml")),
    ),
  )
  return steps
}
