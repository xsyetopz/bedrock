package bedrock.jvm

import java.nio.file.Files
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class StepTest {
  @Test
  fun keepsWhatItReplacesAndSkipsWhatMatches() {
    val root =
      tree(mapOf("a.txt" to "old", "same.txt" to "same", "gone" to "x", "from.java" to "j"))
    val steps =
      listOf(
        Step.Write("a.txt", "new"),
        Step.Write("same.txt", "same"),
        Step.Write("bin/run", "#!/bin/sh\n".toByteArray(), executable = true),
        Step.Remove("gone"),
        Step.Remove("never"),
        Step.Link("CLAUDE.md", "AGENTS.md"),
        Step.Move("from.java", "src/main/java/from.java"),
      )
    val todo = pending(root, steps)
    assertEquals(
      listOf("a.txt", "bin/run", "gone", "CLAUDE.md", "src/main/java/from.java"),
      todo.map { it.path },
    )
    assertEquals(
      listOf(
        "update  a.txt (kept as a.txt.bak)",
        "create  bin/run",
        "remove  gone (kept as gone.bak)",
        "link    CLAUDE.md -> AGENTS.md",
        "move    from.java -> src/main/java/from.java",
      ),
      todo.map { it.describe(root) },
    )
    todo.forEach { it.apply(root) }
    assertEquals(emptyList(), pending(root, steps))
    assertEquals("old", root.resolve("a.txt.bak").readText())
    assertEquals("x", root.resolve("gone.bak").readText())
    assertTrue(root.resolve("bin/run").canExecute())
    assertEquals("j", root.resolve("src/main/java/from.java").readText())
    assertTrue(Files.isSymbolicLink(root.resolve("CLAUDE.md").toPath()))
  }

  @Test
  fun keepsTheFirstBackup() {
    val root = tree(mapOf("a" to "first", "a.bak" to "original"))
    Step.Write("a", "second").apply(root)
    assertEquals("original", root.resolve("a.bak").readText())
    assertEquals("second", root.resolve("a").readText())
    Step.Link("a", "b").apply(root)
    Step.Link("a", "c").apply(root)
    assertEquals("c", Files.readSymbolicLink(root.resolve("a").toPath()).toString())
    assertFalse(Step.Write("a", "c").isDone(root))
  }
}
