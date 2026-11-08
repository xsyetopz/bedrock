package bedrock.core

import bedrock.jvm.tree
import java.io.File
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

class J2kTest {
  @Test
  fun picksTheDownloadForThisMachine() {
    assertTrue(J2k.release("Mac OS X", "aarch64")!!.url.endsWith("-aarch64.dmg"))
    assertTrue(J2k.release("Mac OS X", "x86_64")!!.url.endsWith("${J2k.VERSION}.dmg"))
    assertTrue(J2k.release("Linux", "arm64")!!.url.endsWith("-aarch64.tar.gz"))
    assertTrue(J2k.release("Linux", "amd64")!!.url.endsWith("${J2k.VERSION}.tar.gz"))
    assertNull(J2k.release("Windows 11", "amd64"))
    assertEquals(
      File("/a/IntelliJ IDEA.app/Contents/MacOS/idea"),
      J2k.launcher(File("/a/IntelliJ IDEA.app")),
    )
    assertEquals(File("/a/idea/bin/idea"), J2k.launcher(File("/a/idea")))
  }

  @Test
  fun groupsFilesByTheirGradleProject() {
    val root =
      tree(mapOf("app/build.gradle.kts" to "", "app/src/main/java/A.java" to "", "B.java" to ""))
    val a = root.resolve("app/src/main/java/A.java")
    val b = root.resolve("B.java")
    assertEquals(
      mapOf(root.resolve("app") to listOf(a), root to listOf(b)),
      J2k.modules(root, listOf(a, b)),
    )
  }

  @Test
  fun findsKotlinAndWhereItsSourcesGo() {
    assertTrue(J2k.hasKotlin("plugins { id(\"bedrock.kotlin\") }"))
    assertTrue(J2k.hasKotlin("plugins { kotlin(\"jvm\") }"))
    assertFalse(J2k.hasKotlin("plugins { id(\"bedrock.java\") }"))
    assertEquals("app/src/main/kotlin/a/A.kt", J2k.kotlinPath("app/src/main/java/a/A.kt"))
    assertEquals("src/test/kotlin/A.kt", J2k.kotlinPath("src/test/java/A.kt"))
    assertEquals("lib/A.kt", J2k.kotlinPath("lib/A.kt"))
  }

  @Test
  fun hashesFilesAndReadsServerEvents() {
    val file = tree(mapOf("a" to "abc")).resolve("a")
    assertEquals(
      "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
      J2k.sha256(file),
    )
    assertEquals("{\"a\":1}", Mcp.payload("event: message\ndata: {\"a\":1}\n\n"))
    assertEquals("{}", Mcp.payload(" {} "))
    assertNull(Mcp.payload(""))
  }
}
