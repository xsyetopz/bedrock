package bedrock.core

import java.io.File
import java.io.IOException
import java.net.HttpURLConnection
import java.net.ServerSocket
import java.net.URI
import java.security.MessageDigest
import java.util.concurrent.TimeUnit

/** An IntelliJ IDEA download, pinned by its checksum. */
internal class IdeaRelease(val url: String, val sha256: String)

/**
 * Java-to-Kotlin conversion through IntelliJ IDEA's own converter, J2K. The IDE runs headless as
 * an MCP server and converts each file in place, so the Kotlin keeps the Java's types.
 * Converter and server are in the open-source IntelliJ Community code, and the free tier of
 * IntelliJ IDEA runs them without a licence.
 */
internal object J2k {
  const val VERSION = "2026.3"
  private const val BASE = "https://download.jetbrains.com/idea/idea-$VERSION"

  private val RELEASES =
    mapOf(
      "mac-aarch64" to
        IdeaRelease(
          "$BASE-aarch64.dmg",
          "6de6512375736915faec6166026a7572dd03a03fa7e8c87bb51f8fef3d5cc5aa",
        ),
      "mac-x86_64" to
        IdeaRelease("$BASE.dmg", "41ae3398b87a09c3d2886fd0cd41ead4f0c05e868b99893218d8c4aacc408400"),
      "linux-aarch64" to
        IdeaRelease(
          "$BASE-aarch64.tar.gz",
          "19c8ff6e144185ae4961080984a65b11c492f2820e975007b4276fe0deeffe52",
        ),
      "linux-x86_64" to
        IdeaRelease(
          "$BASE.tar.gz",
          "6a53ff71f4a06add4c97edc4efe86c83119ff82e5b5e9f1db62fc95995bd6b25",
        ),
    )

  /** The release for this machine, or null where JetBrains ships none that this code unpacks. */
  fun release(os: String, arch: String): IdeaRelease? {
    val system =
      when {
        os.startsWith("Mac") -> "mac"
        os.startsWith("Linux") -> "linux"
        else -> return null
      }
    val cpu = if (arch == "aarch64" || arch == "arm64") "aarch64" else "x86_64"
    return RELEASES["$system-$cpu"]
  }

  /** The IDE's launcher inside an installation: a macOS app bundle or a Linux directory. */
  fun launcher(home: File): File =
    if (home.name.endsWith(".app")) home.resolve("Contents/MacOS/idea") else home.resolve("bin/idea")

  /**
   * The IDE in [cache], downloaded and unpacked on first use. [log] reports progress, since the
   * download is about 1.5 GB.
   */
  fun install(cache: File, log: (String) -> Unit): File {
    val os = System.getProperty("os.name")
    val release =
      release(os, System.getProperty("os.arch"))
        ?: throw IOException(
          "No IntelliJ IDEA download for $os here: set BEDROCK_IDEA to an installed IDEA $VERSION."
        )
    val mac = os.startsWith("Mac")
    val home = cache.resolve(if (mac) "IntelliJ IDEA.app" else "idea")
    if (launcher(home).canExecute()) return home
    cache.mkdirs()
    val archive = cache.resolve(release.url.substringAfterLast('/'))
    if (!archive.isFile || sha256(archive) != release.sha256) {
      log("Downloading IntelliJ IDEA $VERSION, about 1.5 GB, to $cache")
      download(release.url, archive)
      val actual = sha256(archive)
      if (actual != release.sha256) {
        archive.delete()
        throw IOException("${release.url} has checksum $actual, not ${release.sha256}")
      }
    }
    val staging = cache.resolve("staging")
    staging.deleteRecursively()
    staging.mkdirs()
    if (mac) {
      val mount = cache.resolve("mount")
      run(listOf("hdiutil", "attach", "-nobrowse", "-readonly", "-mountpoint", "$mount", "$archive"))
      try {
        run(listOf("ditto", "${mount.resolve("IntelliJ IDEA.app")}", "${staging.resolve(home.name)}"))
      } finally {
        run(listOf("hdiutil", "detach", "$mount"))
      }
    } else {
      staging.resolve(home.name).mkdirs()
      run(
        listOf(
          "tar",
          "-xzf",
          "$archive",
          "-C",
          "${staging.resolve(home.name)}",
          "--strip-components=1",
        )
      )
    }
    home.deleteRecursively()
    check(staging.resolve(home.name).renameTo(home)) { "Could not move the IDE to $home" }
    staging.delete()
    archive.delete()
    return home
  }

  private fun download(url: String, to: File) {
    val connection = URI(url).toURL().openConnection() as HttpURLConnection
    connection.instanceFollowRedirects = true
    connection.inputStream.use { input -> to.outputStream().use { input.copyTo(it, 1 shl 20) } }
  }

  fun sha256(file: File): String {
    val digest = MessageDigest.getInstance("SHA-256")
    file.inputStream().use { input ->
      val buffer = ByteArray(1 shl 20)
      while (true) {
        val read = input.read(buffer)
        if (read < 0) break
        digest.update(buffer, 0, read)
      }
    }
    return digest.digest().joinToString("") { String.format("%02x", it) }
  }

  private fun run(command: List<String>) {
    val process = ProcessBuilder(command).inheritIO().start()
    check(process.waitFor() == 0) { "${command.joinToString(" ")} failed" }
  }

  /**
   * The Java files grouped by the Gradle project that holds them: the nearest directory with a
   * build.gradle.kts. The converter takes one project's files per call.
   */
  fun modules(root: File, files: List<File>): Map<File, List<File>> =
    files.groupBy { file ->
      generateSequence(file.parentFile) { it.parentFile }
        .takeWhile { it.path.startsWith(root.path) }
        .firstOrNull { it.resolve("build.gradle.kts").isFile } ?: root
    }

  /** Whether a project's build applies Kotlin, which J2K needs to put its output anywhere. */
  fun hasKotlin(buildScript: String): Boolean =
    Regex("""id\("bedrock\.(kotlin|kmp|cmp|android)"\)|kotlin\(""").containsMatchIn(buildScript)

  /** Where a converted file belongs: src/<set>/kotlin, beside the src/<set>/java it came from. */
  fun kotlinPath(converted: String): String = converted.replace(Regex("""(^|/)src/(\w+)/java/"""), "$1src/$2/kotlin/")

  /** Runs the IDE headless on [root] and converts [files] there, in place. */
  fun convert(idea: File, root: File, state: File, files: List<File>, log: (String) -> Unit): Result {
    val port = ServerSocket(0).use { it.localPort }
    listOf("config", "system", "plugins", "log").forEach { state.resolve(it).mkdirs() }
    val options = state.resolve("idea.vmoptions")
    // A config of its own keeps the user's IDE settings and recent projects untouched.
    options.writeText(
      listOf(
          "-Xmx2g",
          "-Djava.awt.headless=true",
          "-Didea.config.path=${state.resolve("config")}",
          "-Didea.system.path=${state.resolve("system")}",
          "-Didea.plugins.path=${state.resolve("plugins")}",
          "-Didea.log.path=${state.resolve("log")}",
        )
        .joinToString("\n", postfix = "\n")
    )
    val output = state.resolve("server.log")
    val builder =
      ProcessBuilder(
          launcher(idea).path,
          "mcpServer",
          "--project=${root.path}",
          "--port=$port",
          "--invocation-mode=direct",
          "--allowed-tools=convert_java_to_kotlin",
        )
        .redirectErrorStream(true)
        .redirectOutput(output)
    builder.environment()["IDEA_VM_OPTIONS"] = options.path
    log("Starting IntelliJ IDEA headless; the first run imports the build and takes a minute")
    val process = builder.start()
    try {
      awaitServer(process, output, port)
      val client = Mcp("http://127.0.0.1:$port/stream")
      client.initialize()
      val converted = ArrayList<Pair<String, String>>()
      val errors = ArrayList<Pair<String, String>>()
      for ((module, group) in modules(root, files)) {
        log("Converting ${group.size} file(s) in ${module.relativeTo(root).path.ifEmpty { "." }}")
        val paths = group.map { it.relativeTo(root).path }
        val result =
          client.call(
            "convert_java_to_kotlin",
            mapOf("projectPath" to root.path, "file_paths" to paths),
          )
        list(result["converted"]).forEach {
          converted.add(text(it, "javaPath") to text(it, "kotlinPath"))
        }
        list(result["errors"]).forEach { errors.add(text(it, "javaPath") to text(it, "message")) }
      }
      return Result(converted, errors)
    } finally {
      process.destroy()
      if (!process.waitFor(20, TimeUnit.SECONDS)) process.destroyForcibly()
    }
  }

  private fun awaitServer(process: Process, output: File, port: Int) {
    val ready = "MCP Server started on port $port"
    val deadline = System.nanoTime() + TimeUnit.MINUTES.toNanos(10)
    while (System.nanoTime() < deadline) {
      if (output.isFile && output.readText().contains(ready)) return
      if (!process.isAlive) throw IOException("IntelliJ IDEA exited early; see $output")
      Thread.sleep(500)
    }
    throw IOException("IntelliJ IDEA did not start its MCP server in 10 minutes; see $output")
  }

  private fun list(value: Any?): List<Map<*, *>> =
    (value as? List<*>).orEmpty().filterIsInstance<Map<*, *>>()

  private fun text(map: Map<*, *>, key: String): String = map[key]?.toString().orEmpty()

  /** The files J2K wrote and the ones it could not convert, as project-relative paths. */
  class Result(val converted: List<Pair<String, String>>, val errors: List<Pair<String, String>>)
}

/** An MCP client over streamable HTTP, enough to call one tool. */
internal class Mcp(private val url: String) {
  private var session: String? = null
  private var id = 0

  fun initialize() {
    request(
      "initialize",
      mapOf(
        "protocolVersion" to "2025-06-18",
        "capabilities" to emptyMap<String, Any>(),
        "clientInfo" to mapOf("name" to "bedrock-j2k", "version" to J2k.VERSION),
      ),
    )
    post(mapOf("jsonrpc" to "2.0", "method" to "notifications/initialized"))
  }

  /** The tool's structured result. A tool error throws with the IDE's message. */
  fun call(tool: String, arguments: Map<String, Any?>): Map<*, *> {
    val result =
      request("tools/call", mapOf("name" to tool, "arguments" to arguments)) as? Map<*, *>
        ?: throw IOException("$tool returned no result")
    if (result["isError"] == true) throw IOException("$tool failed: ${Json.write(result["content"])}")
    return result["structuredContent"] as? Map<*, *>
      ?: throw IOException("$tool returned no structured content: ${Json.write(result)}")
  }

  private fun request(method: String, params: Map<String, Any?>): Any? {
    val reply =
      post(mapOf("jsonrpc" to "2.0", "id" to ++id, "method" to method, "params" to params))
        as? Map<*, *> ?: throw IOException("$method had no reply")
    reply["error"]?.let { throw IOException("$method failed: ${Json.write(it)}") }
    return reply["result"]
  }

  private fun post(body: Map<String, Any?>): Any? {
    val connection = URI(url).toURL().openConnection() as HttpURLConnection
    connection.requestMethod = "POST"
    connection.doOutput = true
    connection.readTimeout = TimeUnit.MINUTES.toMillis(30).toInt()
    connection.setRequestProperty("Content-Type", "application/json")
    connection.setRequestProperty("Accept", "application/json, text/event-stream")
    session?.let { connection.setRequestProperty("Mcp-Session-Id", it) }
    connection.outputStream.use { it.write(Json.write(body).toByteArray(Charsets.UTF_8)) }
    if (connection.responseCode !in 200..299) {
      throw IOException("MCP server answered ${connection.responseCode}")
    }
    connection.getHeaderField("Mcp-Session-Id")?.let { session = it }
    val text = connection.inputStream.use { it.readBytes().toString(Charsets.UTF_8) }
    return payload(text)?.let(Json::read)
  }

  companion object {
    /** The JSON in a reply, which comes as plain JSON or as a server-sent event's data line. */
    fun payload(text: String): String? {
      val event = text.lineSequence().firstOrNull { it.startsWith("data:") }
      return (event?.removePrefix("data:") ?: text).trim().ifEmpty { null }
    }
  }
}
