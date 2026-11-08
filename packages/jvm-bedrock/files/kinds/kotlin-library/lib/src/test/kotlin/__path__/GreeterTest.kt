package __package__

import kotlin.test.assertEquals
import org.junit.jupiter.api.Test

class GreeterTest {
  @Test
  fun greets() {
    assertEquals("Hello, Ada", greet("Ada"))
  }
}
