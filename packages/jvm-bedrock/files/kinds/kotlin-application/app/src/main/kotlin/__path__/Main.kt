package __package__

fun greet(name: String): String = "Hello, $name"

fun main(args: Array<String>) {
  println(greet(args.firstOrNull() ?: "everyone"))
}
