package bedrock.jvm

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertTrue

class AdoptTest {
  @Test
  fun aSecondPlanIsEmpty() {
    val root = tree(mapOf("CLAUDE.md" to "# Notes\n\nOwn rule.\n", "Makefile" to "all:\n"))
    val (steps, notes) = plan(root, null)
    assertTrue(notes.single().startsWith("gradle.properties: set group="))
    pending(root, steps).forEach { it.apply(root) }
    assertEquals(emptyList(), pending(root, plan(root, null).first))
    val agents = root.resolve("AGENTS.md").readText()
    assertTrue(
      agents.startsWith("# Notes\n\nOwn rule.\n") && "<!-- jvm-bedrock -->" in agents,
      agents,
    )
    assertTrue(
      root.resolve("gradlew").canExecute() && root.resolve(".githooks/pre-commit").canExecute(),
    )
    assertTrue(root.resolve("Makefile.bak").isFile)
    assertTrue(root.resolve(".markdownlint-cli2.jsonc").isFile)
    assertTrue(
      root
        .resolve("settings.gradle.kts")
        .readText()
        .startsWith("// >>> jvm-bedrock\npluginManagement {"),
    )
  }

  @Test
  fun migratesAMavenBuildAndTakesItsGroup() {
    val root =
      tree(
        mapOf(
          "pom.xml" to
            "<project><groupId>com.acme</groupId><artifactId>a</artifactId><version>1</version><modules><module>m</module></modules><packaging>pom</packaging></project>",
          "m/pom.xml" to
            "<project><parent><groupId>com.acme</groupId><artifactId>a</artifactId><version>1</version></parent><artifactId>m</artifactId></project>",
          "package.json" to "{}",
        ),
      )
    val (steps, _) = plan(root, null)
    steps.forEach { it.apply(root) }
    assertTrue(root.resolve("gradle.properties").readText().endsWith("group=com.acme\n"))
    assertTrue(
      root
        .resolve("settings.gradle.kts")
        .readText()
        .endsWith("rootProject.name = \"${root.name}\"\ninclude(\":m\")\n"),
    )
    assertEquals(
      "plugins { id(\"bedrock.java\") }\n",
      root.resolve("m/build.gradle.kts").readText(),
    )
    assertTrue(!root.resolve("pom.xml").exists() && root.resolve("pom.xml.bak").isFile)
    assertTrue(!root.resolve(".markdownlint-cli2.jsonc").exists())
    assertEquals(emptyList(), pending(root, plan(root, null).first))
  }

  @Test
  fun refusesScalaAndNotesBuildsItCannotRead() {
    assertFailsWith<IllegalArgumentException> {
      migration(tree(mapOf("src/main/scala/A.scala" to "")))
    }
    assertEquals(
      setOf(
        "MODULE.bazel: adopt cannot read Bazel builds; write each module's build.gradle.kts by hand",
      ),
      migration(tree(mapOf("MODULE.bazel" to "")))!!.notes,
    )
  }

  @Test
  fun parsesOptions() {
    val options = parse(listOf("x", "--kind", "kmp", "--group", "com.acme", "--dry-run"))
    assertEquals(listOf("x"), options.positional)
    assertEquals("kmp", options.kind)
    assertTrue(options.dryRun)
    assertFailsWith<IllegalArgumentException> { parse(listOf("--group")) }
    assertFailsWith<IllegalArgumentException> { parse(listOf("--group", "Com-Acme")) }
    assertFailsWith<IllegalArgumentException> { parse(listOf("--force")) }
    assertEquals("myapp2", packageName("2My-App2"))
    assertEquals("app", packageName("---"))
  }
}
