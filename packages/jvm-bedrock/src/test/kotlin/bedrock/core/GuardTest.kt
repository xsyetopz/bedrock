package bedrock.core

import bedrock.jvm.tree
import kotlin.test.Test
import kotlin.test.assertEquals

class GuardTest {
  private val junit4 = "import org.junit.Test;\n"

  @Test
  fun reportsDriftForeignBuildsAndWeakRules() {
    val root =
      tree(
        mapOf(
          "gradle/bedrock.pro" to "-dontshrink\n",
          "gradle/kept.txt" to "kept\n",
          Guard.MANIFEST to "",
          "pom.xml" to "<project/>",
          ".mvn/wrapper/maven-wrapper.properties" to "",
          "old/build.gradle" to "",
          "lib/config.gradle" to "",
          "app/build.gradle.kts" to "plugins { java }\n",
          "app/proguard-rules.pro" to "-keep class ** { *; }\n-keep class a.B\n",
          "included/build.gradle.kts" to "plugins { java }\n",
          "included/settings.gradle.kts" to "",
        ),
      )
    root
      .resolve(Guard.MANIFEST)
      .writeText(
        Guard.manifest(
          mapOf("gradle/kept.txt" to "changed\n".toByteArray(), "gradle/gone.txt" to ByteArray(0)),
        ),
      )
    val files =
      root.walkTopDown().filter { it.isFile }.map { it.relativeTo(root).path }.sorted().toList()
    assertEquals(
      listOf(
        ".mvn/wrapper/maven-wrapper.properties: Maven wrapper; ./gradlew replaces it",
        "app/build.gradle.kts: applies no bedrock.* plugin, so it skips the toolchain, Java 8 bytecode, and lints",
        "app/proguard-rules.pro:1: -keep class ** { *; } undoes the shrinking and optimizing; keep only the classes reflection needs",
        "gradle/gone.txt: missing; run jvm-bedrock adopt to restore it",
        "gradle/kept.txt: differs from what jvm-bedrock adopt writes; move the change into project files, then adopt again",
        "lib/config.gradle: Groovy build script; write it as lib/config.gradle.kts",
        "old/build.gradle: Gradle's Groovy DSL build file; the Gradle Kotlin DSL build replaces it, so jvm-bedrock adopt migrates it",
        "pom.xml: Maven build file; the Gradle Kotlin DSL build replaces it, so jvm-bedrock adopt migrates it",
      ),
      Guard.check(root, files).sorted(),
    )
  }

  @Test
  fun asksForReasonsAndJUnit6OutsideAndroid() {
    val root =
      tree(
        mapOf(
          "gradle/bedrock/managed.sha256" to "",
          "lib/build.gradle.kts" to "plugins { id(\"bedrock.java\") }\n",
          "lib/src/test/java/a/ATest.java" to
            junit4 + "@SuppressWarnings(\"x\")\n// Why.\n@SuppressWarnings(\"y\")\n",
          "droid/build.gradle.kts" to "plugins { id(\"bedrock.android\") }\n",
          "droid/src/test/kotlin/BTest.kt" to junit4,
          "shared/src/androidUnitTest/kotlin/CTest.kt" to junit4,
        ),
      )
    val files = root.walkTopDown().filter { it.isFile }.map { it.relativeTo(root).path }.toList()
    assertEquals(
      setOf(
        "lib/src/test/java/a/ATest.java:1: JUnit 4 or TestNG; write the test with JUnit 6, org.junit.jupiter.api",
        "lib/src/test/java/a/ATest.java:2: say why in a // comment on the suppression or the line above it",
      ),
      Guard.check(root, files).toSet(),
    )
  }

  @Test
  fun asksForAdoptWithoutAManifest() {
    assertEquals(
      listOf("${Guard.MANIFEST}: missing; run jvm-bedrock adopt"),
      Guard.check(tree(), emptyList()),
    )
  }
}
