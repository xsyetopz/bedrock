package __package__.shared

import kotlin.test.Test
import kotlin.test.assertEquals

class GreetingTest {
  @Test
  fun greets() {
    assertEquals("Hello, Ada", greet("Ada"))
  }
}
