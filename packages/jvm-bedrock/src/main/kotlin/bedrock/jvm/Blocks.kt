package bedrock.jvm

/** One bedrock's part of a file that several bedrocks write to. */
internal class Block(val name: String, val content: String)

/** How a file marks its blocks. */
internal enum class Syntax(private val open: String, private val close: String) {
  MARKDOWN("<!-- %s -->", "<!-- /%s -->"),
  HASH("# >>> %s", "# <<< %s"),
  SLASH("// >>> %s", "// <<< %s");

  fun open(name: String): String = open.format(name)

  fun close(name: String): String = close.format(name)
}

/**
 * Every bedrock's block, in the order a file lists them, so a file comes out the same whichever
 * bedrock adopts first.
 */
private val ORDER =
  listOf(
    "bedrock",
    "typescript-bedrock",
    "rust-bedrock",
    "swift-bedrock",
    "swift-bedrock-cxx",
    "jvm-bedrock",
  )

private fun lineStart(text: String, marker: String): Int? {
  var index = text.indexOf(marker)
  while (index >= 0) {
    if (index == 0 || text[index - 1] == '\n') return index
    index = text.indexOf(marker, index + 1)
  }
  return null
}

/**
 * [current] with each block replaced or, when missing, added in bedrock order. A missing file
 * starts as [header]. With [first], a missing block goes at the top, as Gradle needs for
 * pluginManagement.
 */
internal fun upsert(
  current: String?,
  header: String,
  syntax: Syntax,
  blocks: List<Block>,
  first: Boolean = false,
): String {
  var text = current ?: header
  for (block in blocks) {
    val open = syntax.open(block.name)
    val close = syntax.close(block.name)
    val marked = "$open\n${block.content}$close\n"
    val start = text.indexOf(open)
    val end = if (start < 0) -1 else text.indexOf(close, start)
    val next =
      ORDER.dropWhile { it != block.name }
        .drop(1)
        .mapNotNull { lineStart(text, syntax.open(it)) }
        .minOrNull()
    text =
      when {
        end >= 0 -> {
          val after = end + close.length
          text.substring(0, start) +
            marked +
            text.substring(if (text.startsWith("\n", after)) after + 1 else after)
        }
        next != null -> text.substring(0, next) + marked + "\n" + text.substring(next)
        first -> if (text.isEmpty()) marked else "$marked\n$text"
        text.isEmpty() || text.endsWith("\n\n") -> text + marked
        text.endsWith("\n") -> "$text\n$marked"
        else -> "$text\n\n$marked"
      }
  }
  return text
}
