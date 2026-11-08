package bedrock.jvm

import kotlin.test.Test
import kotlin.test.assertEquals

class MigrationTest {
  @Test
  fun makesAliasesGradleCanUse() {
    assertEquals("guava", aliasOf("guava"))
    assertEquals("log4j12-api", aliasOf("log4j-1.2-api"))
    assertEquals("lib-2d", aliasOf("2d"))
    assertEquals("lib-bundles-x", aliasOf("bundles-x"))
    assertEquals("jackson-databind", aliasOf("Jackson_Databind"))
    assertEquals("library", aliasOf("--"))
  }

  @Test
  fun writesBuildsAndTheCatalog() {
    val migration = Migration("test")
    val core = Module("core")
    core.dependencies.addAll(
      listOf(
        Dependency.Library("api", "a.b", "foo", "1"),
        Dependency.Library("api", "c.d", "foo-bar", "2"),
        Dependency.Library("implementation", "e.f", "foo", "3"),
        Dependency.Library("testImplementation", "junit", "junit", "4.13"),
        Dependency.Library("implementation", "org.jetbrains.kotlin", "kotlin-stdlib", "2"),
        Dependency.Platform(
          "implementation",
          Dependency.Library("implementation", "g", "bom", "4"),
        ),
        Dependency.Jars("implementation", listOf("lib/a.jar")),
        Dependency.Notation("implementation", "libs.own"),
      ),
    )
    val app = Module("apps/cli")
    app.kotlin = true
    app.mainClass = "a.MainKt"
    app.dependencies.add(Dependency.Module("implementation", core.gradlePath))
    app.dependencies.add(Dependency.Library("implementation", "a.b", "foo", "1"))
    migration.modules.addAll(listOf(core, app))
    val root =
      tree(
        mapOf(
          "gradle/libs.versions.toml" to
            "[libraries]\nown = { module = \"o:o\", version = \"1\" }\n",
        ),
      )
    val steps = render(root, migration)
    assertEquals(
      """
      plugins { id("bedrock.java") }

      dependencies {
        api(libs.foo)
        api(libs.d.foo.bar)
        implementation(libs.f.foo)
        implementation(platform(libs.bom))
        implementation(files("lib/a.jar"))
        implementation(libs.own)
      }
      """
        .trimIndent() + "\n",
      written(steps, "core/build.gradle.kts"),
    )
    assertEquals(
      """
      plugins {
        id("bedrock.kotlin")
        id("bedrock.application")
      }

      application { mainClass = "a.MainKt" }

      dependencies {
        implementation(project(":core"))
        implementation(libs.foo)
      }
      """
        .trimIndent() + "\n",
      written(steps, "apps/cli/build.gradle.kts"),
    )
    assertEquals(
      "[libraries]\nown = { module = \"o:o\", version = \"1\" }\n\n# Added by jvm-bedrock adopt.\n" +
        "foo = { module = \"a.b:foo\", version = \"1\" }\n" +
        "d-foo-bar = { module = \"c.d:foo-bar\", version = \"2\" }\n" +
        "f-foo = { module = \"e.f:foo\", version = \"3\" }\n" +
        "bom = { module = \"g:bom\", version = \"4\" }\n",
      written(steps, "gradle/libs.versions.toml"),
    )
    assertEquals(
      setOf(
        ":core: tests used junit:junit; bedrock runs JUnit 6, so port them to org.junit.jupiter.api, which just guard checks",
      ),
      migration.notes,
    )
    assertEquals("include(\":core\", \":apps:cli\")\n", includes(migration))
  }
}
