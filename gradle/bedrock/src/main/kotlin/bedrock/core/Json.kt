package bedrock.core

/**
 * The JSON the MCP calls need. Objects read as maps, arrays as lists, numbers as doubles; the
 * code stays on the Java 8 API so jvm-bedrock and the Gradle build share it.
 */
internal object Json {
  fun write(value: Any?): String = buildString { write(value, this) }

  private fun write(value: Any?, out: StringBuilder) {
    when (value) {
      null -> out.append("null")
      is String -> quote(value, out)
      is Boolean,
      is Int,
      is Long -> out.append(value.toString())
      is Number -> out.append(value.toDouble().toString())
      is Map<*, *> -> {
        out.append('{')
        value.entries.forEachIndexed { index, (key, item) ->
          if (index > 0) out.append(',')
          quote(key.toString(), out)
          out.append(':')
          write(item, out)
        }
        out.append('}')
      }
      is Iterable<*> -> {
        out.append('[')
        value.forEachIndexed { index, item ->
          if (index > 0) out.append(',')
          write(item, out)
        }
        out.append(']')
      }
      else -> throw IllegalArgumentException("JSON has no form for ${value::class.java.name}")
    }
  }

  private fun quote(text: String, out: StringBuilder) {
    out.append('"')
    for (char in text) {
      when (char) {
        '"' -> out.append("\\\"")
        '\\' -> out.append("\\\\")
        '\n' -> out.append("\\n")
        '\r' -> out.append("\\r")
        '\t' -> out.append("\\t")
        else ->
          if (char < ' ') out.append(String.format("\\u%04x", char.code)) else out.append(char)
      }
    }
    out.append('"')
  }

  fun read(text: String): Any? {
    val reader = Reader(text)
    val value = reader.value()
    reader.space()
    require(reader.done()) { "JSON has text after the value at ${reader.at}" }
    return value
  }

  private class Reader(private val text: String) {
    var at = 0

    fun done(): Boolean = at == text.length

    fun space() {
      while (at < text.length && text[at].isWhitespace()) at++
    }

    private fun expect(char: Char) {
      space()
      require(at < text.length && text[at] == char) { "JSON expected '$char' at $at" }
      at++
    }

    private fun peek(): Char {
      space()
      require(at < text.length) { "JSON ended early" }
      return text[at]
    }

    fun value(): Any? =
      when (peek()) {
        '{' -> obj()
        '[' -> array()
        '"' -> string()
        't' -> word("true", true)
        'f' -> word("false", false)
        'n' -> word("null", null)
        else -> number()
      }

    private fun word(word: String, value: Any?): Any? {
      require(text.startsWith(word, at)) { "JSON has an unknown word at $at" }
      at += word.length
      return value
    }

    private fun number(): Double {
      val start = at
      while (at < text.length && text[at] in "+-0123456789.eE") at++
      return text.substring(start, at).toDoubleOrNull()
        ?: throw IllegalArgumentException("JSON has no value at $start")
    }

    private fun obj(): Map<String, Any?> {
      expect('{')
      val map = LinkedHashMap<String, Any?>()
      if (peek() == '}') {
        at++
        return map
      }
      while (true) {
        val key = string()
        expect(':')
        map[key] = value()
        if (peek() == ',') at++ else break
      }
      expect('}')
      return map
    }

    private fun array(): List<Any?> {
      expect('[')
      val list = ArrayList<Any?>()
      if (peek() == ']') {
        at++
        return list
      }
      while (true) {
        list.add(value())
        if (peek() == ',') at++ else break
      }
      expect(']')
      return list
    }

    private fun string(): String {
      expect('"')
      val out = StringBuilder()
      while (true) {
        require(at < text.length) { "JSON has an unclosed string" }
        val char = text[at++]
        when (char) {
          '"' -> return out.toString()
          '\\' -> {
            val escape = text[at++]
            when (escape) {
              'n' -> out.append('\n')
              'r' -> out.append('\r')
              't' -> out.append('\t')
              'b' -> out.append('\b')
              'f' -> out.append('\u000c')
              'u' -> {
                out.append(text.substring(at, at + 4).toInt(16).toChar())
                at += 4
              }
              else -> out.append(escape)
            }
          }
          else -> out.append(char)
        }
      }
    }
  }
}
