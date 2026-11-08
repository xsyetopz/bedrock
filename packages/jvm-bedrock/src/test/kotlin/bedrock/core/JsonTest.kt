package bedrock.core

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith

class JsonTest {
  @Test
  fun writesAndReadsBackNestedValues() {
    val value =
      mapOf(
        "a" to listOf(1, true, null, "x\"\n\t\u0001"),
        "b" to emptyMap<String, Any>(),
        "c" to 2.5,
      )
    val text = Json.write(value)
    assertEquals("""{"a":[1,true,null,"x\"\n\t\u0001"],"b":{},"c":2.5}""", text)
    assertEquals(
      mapOf(
        "a" to listOf(1.0, true, null, "x\"\n\t\u0001"),
        "b" to emptyMap<String, Any>(),
        "c" to 2.5,
      ),
      Json.read(text),
    )
  }

  @Test
  fun readsEscapesAndWhitespace() {
    assertEquals(
      listOf("é/\b\u000c\r", -1.5e3, false),
      Json.read(""" [ "é\/\b\f\r" , -1.5e3 , false ] """),
    )
    assertEquals(emptyList<Any>(), Json.read("[]"))
  }

  @Test
  fun rejectsBrokenJson() {
    for (text in listOf("", "[1", "{\"a\" 1}", "nul", "\"open", "1 2", "-")) {
      assertFailsWith<IllegalArgumentException>(text) { Json.read(text) }
    }
    assertFailsWith<IllegalArgumentException> { Json.write(Any()) }
  }
}
