package __package__;

import org.jspecify.annotations.Nullable;

/** Prints a greeting. */
final class Main {
  private Main() {}

  static String greet(@Nullable String name) {
    return "Hello, " + (name == null ? "everyone" : name);
  }

  public static void main(String[] args) {
    System.out.println(greet(args.length > 0 ? args[0] : null));
  }
}
