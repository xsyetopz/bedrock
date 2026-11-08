package bedrock.jvm

import kotlin.test.Test
import kotlin.test.assertEquals

class BlocksTest {
  private val jvm = listOf(Block("jvm-bedrock", "j\n"))

  @Test
  fun startsAMissingFileFromItsHeader() {
    assertEquals(
      "root = true\n\n# >>> jvm-bedrock\nj\n# <<< jvm-bedrock\n",
      upsert(null, "root = true\n", Syntax.HASH, jvm),
    )
    assertEquals("# >>> jvm-bedrock\nj\n# <<< jvm-bedrock\n", upsert(null, "", Syntax.HASH, jvm))
  }

  @Test
  fun replacesItsBlockAndKeepsTheRest() {
    val old = "own\n\n<!-- jvm-bedrock -->\nold\n<!-- /jvm-bedrock -->\nafter\n"
    assertEquals(
      "own\n\n<!-- jvm-bedrock -->\nj\n<!-- /jvm-bedrock -->\nafter\n",
      upsert(old, "", Syntax.MARKDOWN, jvm),
    )
  }

  @Test
  fun keepsBedrockOrder() {
    val text =
      upsert("x", "", Syntax.SLASH, listOf(Block("swift-bedrock", "s\n"), Block("bedrock", "b\n")))
    assertEquals(
      "x\n\n// >>> bedrock\nb\n// <<< bedrock\n\n// >>> swift-bedrock\ns\n// <<< swift-bedrock\n",
      text,
    )
    assertEquals(
      "x\n\n// >>> bedrock\nb\n// <<< bedrock\n\n// >>> swift-bedrock\ns\n// <<< swift-bedrock\n\n// >>> jvm-bedrock\nj\n// <<< jvm-bedrock\n",
      upsert(text, "", Syntax.SLASH, jvm),
    )
    assertEquals(
      "// >>> rust-bedrock\nr\n// <<< rust-bedrock\n\n// >>> swift-bedrock\ns\n",
      upsert("// >>> swift-bedrock\ns\n", "", Syntax.SLASH, listOf(Block("rust-bedrock", "r\n"))),
    )
  }

  @Test
  fun putsAFirstBlockAtTheTop() {
    assertEquals(
      "// >>> jvm-bedrock\nj\n// <<< jvm-bedrock\n\nname\n",
      upsert("name\n", "", Syntax.SLASH, jvm, first = true),
    )
    assertEquals(
      "// >>> jvm-bedrock\nj\n// <<< jvm-bedrock\n",
      upsert("", "", Syntax.SLASH, jvm, first = true),
    )
  }
}
