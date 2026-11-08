package bedrock.jvm

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class AntTest {
  @Test
  fun movesSourcesAndReadsIvy() {
    val root =
      tree(
        mapOf(
          "build.xml" to
            """
            <project name="p" default="jar">
              <property name="src" value="source"/>
              <target name="compile">
                <javac srcdir="${'$'}{src}" release="11"/>
                <javac srcdir="tests"/>
              </target>
              <target name="jar"><jar><manifest><attribute name="Main-Class" value="a.Main"/></manifest></jar></target>
            </project>
            """
              .trimIndent(),
          "ivy.xml" to
            """
            <ivy-module version="2.0"><info organisation="com.acme" module="p"/>
              <dependencies>
                <dependency org="commons-io" name="commons-io" rev="2.20.0"/>
                <dependency org="org.junit.jupiter" name="junit-jupiter" rev="5.13" conf="test->default"/>
                <dependency org="javax.servlet" name="servlet-api" rev="2.5" conf="provided"/>
              </dependencies>
            </ivy-module>
            """
              .trimIndent(),
          "source/a/Main.java" to "",
          "source/a/messages.properties" to "",
          "tests/a/MainTest.java" to "",
          "lib/x.jar" to "",
        ),
      )
    val migration = ant(root)
    assertEquals("com.acme", migration.group)
    assertEquals(
      setOf(
        "source/a/Main.java" to "src/main/java/a/Main.java",
        "source/a/messages.properties" to "src/main/resources/a/messages.properties",
        "tests/a/MainTest.java" to "src/test/java/a/MainTest.java",
      ),
      migration.steps.filterIsInstance<Step.Move>().map { it.from to it.path }.toSet(),
    )
    assertEquals(listOf("build.xml", "ivy.xml"), removed(migration.steps))
    val build = written(render(root, migration), "build.gradle.kts")
    for (line in
      listOf(
        "mainClass = \"a.Main\"",
        "implementation(libs.commons.io)",
        "compileOnly(libs.servlet.api)",
        "implementation(files(\"lib/x.jar\"))",
      )) {
      assertTrue(line in build, build)
    }
    assertTrue("junit" !in build, build)
    val notes = migration.notes.joinToString("\n")
    assertTrue("Java 11" in notes && "1 jar(s)" in notes, notes)
  }
}
