package bedrock.jvm

/** The files jvm-bedrock writes, packed into its jar under files/ with an index of their paths. */
internal object Bundle {
  private val index: List<String> by lazy { text("files.index").lines().filter(String::isNotEmpty) }

  fun bytes(path: String): ByteArray =
    (Bundle::class.java.getResourceAsStream("/files/$path")
        ?: throw IllegalStateException("jvm-bedrock is missing files/$path"))
      .use { it.readBytes() }

  fun text(path: String): String = String(bytes(path), Charsets.UTF_8)

  /** The paths under [directory], relative to it. */
  fun under(directory: String): List<String> =
    index.filter { it.startsWith("$directory/") }.map { it.removePrefix("$directory/") }
}
