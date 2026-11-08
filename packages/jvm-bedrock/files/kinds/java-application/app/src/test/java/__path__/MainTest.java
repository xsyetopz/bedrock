package __package__;

import static org.junit.jupiter.api.Assertions.assertEquals;

import org.junit.jupiter.api.Test;

final class MainTest {
  @Test
  void greets() {
    assertEquals("Hello, Ada", Main.greet("Ada"));
  }
}
