package bedrock.jvm

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class GradleTest {
  @Test
  fun convertsAGroovyBuild() {
    val root =
      tree(
        mapOf(
          "settings.gradle" to
            "rootProject.name = 'shop'\ninclude 'core', ':web:api'\ninclude(':droid')\n",
          "build.gradle" to "group = 'com.acme'\nsubprojects { apply plugin: 'java' }\n",
          "core/build.gradle" to
            """
            plugins { id 'java-library' }
            java { sourceCompatibility = JavaVersion.VERSION_17 }
            dependencies {
              api 'com.google.guava:guava:33.0'
              compile group: 'org.slf4j', name: 'slf4j-api', version: '2.0.17'
              testImplementation 'junit:junit:4.13'
              implementation platform('io.netty:netty-bom:4.2.0')
              // A comment.
              implementation libs.own
              kapt 'x:y:1'
              implementation fileTree(dir: 'libs')
            }
            """
              .trimIndent(),
          "web/api/build.gradle.kts" to
            """
            plugins {
              kotlin("jvm")
              application
            }
            application { mainClass.set("com.acme.ApiKt") }
            dependencies {
              implementation(project(":core"))
              implementation(kotlin("stdlib"))
            }
            """
              .trimIndent(),
          "droid/build.gradle" to "plugins { id 'com.android.application' }\n",
        ),
      )
    assertTrue(isGradle(root))
    val migration = gradle(root)
    assertEquals("com.acme", migration.group)
    assertEquals(
      listOf("settings.gradle", "build.gradle", "core/build.gradle"),
      removed(migration.steps),
    )
    assertEquals(listOf("core", "web/api"), migration.modules.map { it.directory })
    val steps = render(root, migration)
    assertEquals(
      "plugins { id(\"bedrock.java\") }\n\ndependencies {\n  api(libs.guava)\n  api(libs.slf4j.api)\n  implementation(platform(libs.netty.bom))\n  implementation(libs.own)\n}\n",
      written(steps, "core/build.gradle.kts"),
    )
    assertEquals(
      "plugins {\n  id(\"bedrock.kotlin\")\n  id(\"bedrock.application\")\n}\n\napplication { mainClass = \"com.acme.ApiKt\" }\n\ndependencies {\n  implementation(project(\":core\"))\n}\n",
      written(steps, "web/api/build.gradle.kts"),
    )
    val notes = migration.notes.joinToString("\n")
    for (part in
      listOf(
        "allprojects or subprojects",
        "Java 17",
        "kapt 'x:y:1'",
        "fileTree",
        "droid/build.gradle: an Android",
      )) {
      assertTrue(part in notes, notes)
    }
  }

  @Test
  fun keepsModulesOnBedrockAlready() {
    val root =
      tree(
        mapOf(
          "settings.gradle.kts" to "include(\":a\", \":b\")\n",
          "a/build.gradle.kts" to "plugins { id(\"bedrock.java\") }\n",
          "b/build.gradle.kts" to "plugins { java }\n",
        ),
      )
    val migration = gradle(root)
    assertEquals("include(\":a\", \":b\")\n", includes(migration))
    assertEquals(
      listOf("b/build.gradle.kts"),
      render(root, migration).filterIsInstance<Step.Write>().map { it.path },
    )
    assertFalse(
      isGradle(tree(mapOf("settings.gradle.kts" to "plugins { id(\"bedrock.settings\") }\n"))),
    )
  }
}
