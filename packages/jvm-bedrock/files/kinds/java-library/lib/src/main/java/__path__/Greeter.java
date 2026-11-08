package __package__;

import org.jspecify.annotations.Nullable;

/** Greets people. */
public final class Greeter {
  private Greeter() {}

  /**
   * Returns a greeting.
   *
   * @param name who to greet, or null for everyone
   * @return the greeting
   */
  public static String greet(@Nullable String name) {
    return "Hello, " + (name == null ? "everyone" : name);
  }
}
