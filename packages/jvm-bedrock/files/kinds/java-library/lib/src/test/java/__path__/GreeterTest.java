package __package__;

import static org.junit.jupiter.api.Assertions.assertEquals;

import org.junit.jupiter.api.Test;

final class GreeterTest {
  @Test
  void greets() {
    assertEquals("Hello, Ada", Greeter.greet("Ada"));
  }
}
