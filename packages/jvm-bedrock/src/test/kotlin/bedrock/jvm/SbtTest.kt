package bedrock.jvm

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class SbtTest {
  @Test
  fun splitsTheBuildIntoItsProjects() {
    val root =
      tree(
        mapOf(
          "build.sbt" to
            """
            organization := "com.acme"
            libraryDependencies += "org.slf4j" % "slf4j-api" % "2.0.17"
            javacOptions ++= Seq("--release", "21")

            lazy val core = (project in file("core"))
              .settings(libraryDependencies ++= Seq(
                "com.google.guava" % "guava" % "33.0",
                "org.typelevel" %% "cats-core" % "2.13.0",
                "org.junit.jupiter" % "junit-jupiter" % "5.13" % Test,
              ))

            lazy val app = project.in(file("app"))
              .dependsOn(core % "compile->compile")
              .settings(Compile / mainClass := Some("com.acme.Main"))
            """
              .trimIndent(),
          "project/build.properties" to "sbt.version=1.11.0\n",
        ),
      )
    val migration = sbt(root)
    assertEquals("com.acme", migration.group)
    assertEquals(listOf("build.sbt", "project"), removed(migration.steps))
    val steps = render(root, migration)
    assertEquals(
      "plugins { id(\"bedrock.java\") }\n\ndependencies {\n  api(libs.slf4j.api)\n  api(libs.guava)\n}\n",
      written(steps, "core/build.gradle.kts"),
    )
    val app = written(steps, "app/build.gradle.kts")
    assertTrue(
      "implementation(project(\":core\"))" in app &&
        "implementation(libs.slf4j.api)" in app &&
        "com.acme.Main" in app,
      app,
    )
    val notes = migration.notes.joinToString("\n")
    assertTrue("cats-core is a Scala library" in notes && "Java 21" in notes, notes)
  }

  @Test
  fun keepsABuildWithoutProjectsAtTheRoot() {
    val root =
      tree(mapOf("build.sbt" to "libraryDependencies += \"a\" % \"b\" % \"1\" % \"provided\"\n"))
    val migration = sbt(root)
    assertEquals(listOf(""), migration.modules.map { it.directory })
    assertTrue("compileOnly(libs.b)" in written(render(root, migration), "build.gradle.kts"))
  }
}
