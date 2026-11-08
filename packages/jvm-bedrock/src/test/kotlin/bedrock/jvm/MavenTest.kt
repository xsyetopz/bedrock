package bedrock.jvm

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.test.assertTrue

class MavenTest {
  private fun dependency(group: String, artifact: String, extra: String = "") =
    "<dependency><groupId>$group</groupId><artifactId>$artifact</artifactId>$extra</dependency>"

  private val parent =
    """
    <project>
      <parent><groupId>org.springframework.boot</groupId><artifactId>spring-boot-starter-parent</artifactId><version>3.5.0</version></parent>
      <groupId>com.acme</groupId><artifactId>root</artifactId><version>1.0</version><packaging>pom</packaging>
      <properties><guava.version>33.0</guava.version><maven.compiler.release>17</maven.compiler.release></properties>
      <modules><module>core</module><module>app</module></modules>
      <dependencyManagement><dependencies>
        ${dependency("com.google.guava", "guava", "<version>\${guava.version}</version>")}
        ${dependency("io.netty", "netty-bom", "<version>4.2.0</version><type>pom</type><scope>import</scope>")}
      </dependencies></dependencyManagement>
    </project>
    """

  @Test
  fun convertsAMultiModuleBuild() {
    val root =
      tree(
        mapOf(
          "pom.xml" to parent,
          "mvnw" to "",
          "core/pom.xml" to
            """
            <project><parent><groupId>com.acme</groupId><artifactId>root</artifactId><version>1.0</version></parent>
              <artifactId>core</artifactId>
              <dependencies>
                ${dependency("com.google.guava", "guava")}
                ${dependency("javax.servlet", "servlet-api", "<version>2.5</version><scope>provided</scope>")}
                ${dependency("org.postgresql", "postgresql", "<version>42</version><scope>runtime</scope>")}
                ${dependency("junit", "junit", "<version>4.13</version><scope>test</scope>")}
                ${dependency("x", "local", "<version>1</version><scope>system</scope><systemPath>\${project.basedir}/lib/x.jar</systemPath>")}
                ${dependency("y", "native", "<version>1</version><classifier>linux</classifier>")}
              </dependencies>
              <build><sourceDirectory>src</sourceDirectory></build>
            </project>
            """,
          "app/pom.xml" to
            """
            <project><parent><groupId>com.acme</groupId><artifactId>root</artifactId><version>1.0</version></parent>
              <artifactId>app</artifactId>
              <properties><start-class>com.acme.App</start-class></properties>
              <dependencies>${dependency("com.acme", "core", "<version>\${project.version}</version>")}</dependencies>
            </project>
            """,
          "app/src/main/kotlin/App.kt" to "",
        ),
      )
    val migration = maven(root)
    assertEquals("com.acme", migration.group)
    assertEquals(listOf("pom.xml", "core/pom.xml", "app/pom.xml", "mvnw"), removed(migration.steps))
    val steps = render(root, migration)
    val core = written(steps, "core/build.gradle.kts")
    assertTrue("api(platform(libs.spring.boot.starter.parent))" in core, core)
    assertTrue("api(platform(libs.netty.bom))" in core, core)
    assertTrue("api(libs.guava)" in core, core)
    assertTrue("compileOnly(libs.servlet.api)" in core, core)
    assertTrue("runtimeOnly(libs.postgresql)" in core, core)
    assertTrue(
      "implementation(files(\"\${'$'}{project.basedir}/lib/x.jar\"))" in core || "files(" in core,
      core,
    )
    val app = written(steps, "app/build.gradle.kts")
    assertTrue("id(\"bedrock.kotlin\")" in app && "mainClass = \"com.acme.App\"" in app, app)
    assertTrue("implementation(project(\":core\"))" in app, app)
    assertTrue(
      "guava = { module = \"com.google.guava:guava\", version = \"33.0\" }" in
        written(steps, "gradle/libs.versions.toml"),
    )
    val notes = migration.notes.joinToString("\n")
    for (part in
      listOf(
        "spring-boot-starter-parent:3.5.0 is outside",
        "system scope",
        "classifier",
        "Java 17",
        "sourceDirectory src",
        "junit:junit",
      )) {
      assertTrue(part in notes, notes)
    }
  }

  @Test
  fun refusesADoctype() {
    val root =
      tree(
        mapOf(
          "pom.xml" to
            "<!DOCTYPE p [<!ENTITY x SYSTEM \"file:///etc/passwd\">]><project>&x;</project>",
        ),
      )
    assertTrue(runCatching { maven(root) }.isFailure)
  }

  @Test
  fun keepsASingleModuleAtTheRoot() {
    val root =
      tree(
        mapOf(
          "pom.xml" to
            "<project><groupId>g</groupId><artifactId>a</artifactId><version>1</version></project>",
        ),
      )
    val migration = maven(root)
    assertEquals(listOf(""), migration.modules.map { it.directory })
    assertNull(migration.modules[0].mainClass)
    assertEquals(
      "plugins { id(\"bedrock.java\") }\n",
      written(render(root, migration), "build.gradle.kts"),
    )
  }
}
