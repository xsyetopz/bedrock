package bedrock.jvm

import java.io.File
import javax.xml.XMLConstants
import javax.xml.parsers.DocumentBuilderFactory
import org.w3c.dom.Element

/** A parsed pom.xml: its own elements, and the properties and managed versions it inherits. */
private class Pom(val directory: String, val element: Element, val parent: Pom?) {
  val properties: Map<String, String> = run {
    val own = LinkedHashMap(parent?.properties.orEmpty())
    child(element, "properties")?.let { properties ->
      children(properties).forEach { own[it.tagName] = it.textContent.trim() }
    }
    own["project.groupId"] = groupId
    own["project.artifactId"] = artifactId
    own["project.version"] = version
    own
  }

  val groupId: String
    get() =
      text(element, "groupId")
        ?: parent?.groupId
        ?: text(child(element, "parent"), "groupId").orEmpty()

  val artifactId: String
    get() = text(element, "artifactId").orEmpty()

  val version: String
    get() =
      text(element, "version")
        ?: parent?.version
        ?: text(child(element, "parent"), "version").orEmpty()

  /** Versions from dependencyManagement here and in the parents, by group:artifact. */
  val managed: Map<String, String> = run {
    val own = LinkedHashMap(parent?.managed.orEmpty())
    dependencies(child(element, "dependencyManagement")).forEach { dependency ->
      val version = resolve(text(dependency, "version"))
      if (version != null && text(dependency, "scope") != "import") {
        own["${resolve(text(dependency, "groupId"))}:${resolve(text(dependency, "artifactId"))}"] =
          version
      }
    }
    own
  }

  /** BOMs that dependencyManagement imports, here and in the parents. */
  val boms: List<Dependency.Library> =
    parent?.boms.orEmpty() +
      dependencies(child(element, "dependencyManagement"))
        .filter { text(it, "scope") == "import" }
        .map { library("implementation", it) }

  fun resolve(value: String?): String? =
    value?.replace(Regex("""\$\{([^}]+)}""")) { properties[it.groupValues[1]] ?: it.value }

  fun library(configuration: String, dependency: Element): Dependency.Library {
    val group = resolve(text(dependency, "groupId")).orEmpty()
    val name = resolve(text(dependency, "artifactId")).orEmpty()
    return Dependency.Library(
      configuration,
      group,
      name,
      resolve(text(dependency, "version")) ?: managed["$group:$name"],
    )
  }
}

private fun children(element: Element?): List<Element> {
  val nodes = element?.childNodes ?: return emptyList()
  return (0 until nodes.length).map(nodes::item).filterIsInstance<Element>()
}

private fun child(element: Element?, name: String): Element? =
  children(element).firstOrNull { it.tagName == name }

private fun text(element: Element?, name: String): String? =
  child(element, name)?.textContent?.trim()

private fun dependencies(element: Element?): List<Element> =
  children(child(element, "dependencies")).filter { it.tagName == "dependency" }

internal fun descendants(element: Element, name: String): List<Element> {
  val nodes = element.getElementsByTagName(name)
  return (0 until nodes.length).map(nodes::item).filterIsInstance<Element>()
}

/** The root element of a Maven, Ant, or Ivy file. */
internal fun xml(file: File): Element {
  val factory = DocumentBuilderFactory.newInstance()
  // These files have no DTD, so refusing one keeps a hostile file from reading others.
  factory.setFeature("http://apache.org/xml/features/disallow-doctype-decl", true)
  factory.setAttribute(XMLConstants.ACCESS_EXTERNAL_DTD, "")
  return factory.newDocumentBuilder().parse(file).documentElement
}

/** Converts the Maven build rooted at [root]'s pom.xml, following its modules. */
internal fun maven(root: File): Migration {
  val migration = Migration("Maven")
  val poms = ArrayList<Pom>()
  fun read(directory: String, parent: Pom?) {
    val file = File(root, directory).resolve("pom.xml")
    val pom = Pom(directory, xml(file), parent)
    poms.add(pom)
    migration.steps.add(Step.Remove(if (directory.isEmpty()) "pom.xml" else "$directory/pom.xml"))
    for (module in children(child(pom.element, "modules"))) {
      val path =
        File(
            if (directory.isEmpty()) module.textContent.trim()
            else "$directory/${module.textContent.trim()}",
          )
          .normalize()
          .path
      read(path.removeSuffix("/pom.xml"), pom)
    }
  }
  read("", null)
  for (name in listOf("mvnw", "mvnw.cmd", ".mvn")) {
    if (File(root, name).exists()) migration.steps.add(Step.Remove(name))
  }
  migration.group = poms[0].groupId.ifEmpty { null }
  val reactor = poms.associate { "${it.groupId}:${it.artifactId}" to it.directory }
  for (pom in poms) {
    val file = if (pom.directory.isEmpty()) "pom.xml" else "${pom.directory}/pom.xml"
    if (pom.parent == null) {
      child(pom.element, "parent")?.let { parent ->
        val coordinates =
          "${text(parent, "groupId")}:${text(parent, "artifactId")}:${text(parent, "version")}"
        migration.notes.add(
          "$file: the parent $coordinates is outside the repository, so each module imports it as a platform for the versions it manages; move its plugins and settings by hand",
        )
      }
    }
    if (text(pom.element, "packaging") == "pom") continue
    val module = Module(pom.directory)
    val directory = File(root, pom.directory)
    module.kotlin =
      directory.resolve("src/main/kotlin").isDirectory ||
        descendants(pom.element, "artifactId").any {
          it.textContent.trim() == "kotlin-maven-plugin"
        }
    module.mainClass =
      (descendants(pom.element, "mainClass") + descendants(pom.element, "Main-Class"))
        .firstNotNullOfOrNull { pom.resolve(it.textContent.trim())?.ifEmpty { null } }
        ?: pom.properties["start-class"]
        ?: pom.properties["exec.mainClass"]
    val api = if (module.mainClass == null) "api" else "implementation"
    generateSequence(pom) { it.parent }
      .last()
      .let { top ->
        child(top.element, "parent")?.let {
          val parent =
            Dependency.Library(
              api,
              text(it, "groupId").orEmpty(),
              text(it, "artifactId").orEmpty(),
              text(it, "version"),
            )
          module.dependencies.add(Dependency.Platform(api, parent))
        }
      }
    pom.boms.forEach { module.dependencies.add(Dependency.Platform(api, it)) }
    for (dependency in dependencies(pom.element)) {
      val configuration =
        when (text(dependency, "scope")) {
          "provided" -> "compileOnly"
          "runtime" -> "runtimeOnly"
          "test" -> "testImplementation"
          "system" -> null
          else -> api
        }
      val library = pom.library(configuration ?: api, dependency)
      val local = reactor[library.module]
      when {
        configuration == null -> {
          val path = pom.resolve(text(dependency, "systemPath")).orEmpty()
          module.dependencies.add(Dependency.Jars("implementation", listOf(path)))
          migration.notes.add(
            "$file: ${library.module} has system scope; replace files(\"$path\") with its Maven coordinates",
          )
        }
        local != null ->
          module.dependencies.add(
            Dependency.Module(library.configuration, Module(local).gradlePath),
          )
        else -> {
          module.dependencies.add(library)
          if (
            text(dependency, "classifier") != null ||
              text(dependency, "type").let { it != null && it != "jar" }
          ) {
            migration.notes.add(
              "$file: ${library.module} has a classifier or type; add it to the catalog entry by hand",
            )
          }
        }
      }
    }
    for (key in
      listOf(
        "maven.compiler.release",
        "maven.compiler.target",
        "maven.compiler.source",
        "java.version",
      )) {
      pom.properties[key]?.let { javaLevelNote(file, it, migration.notes) }
    }
    descendants(pom.element, "release").firstOrNull()?.let {
      javaLevelNote(file, pom.resolve(it.textContent.trim()), migration.notes)
    }
    child(pom.element, "build")?.let { build ->
      for (tag in listOf("sourceDirectory", "testSourceDirectory")) {
        text(build, tag)?.let {
          migration.notes.add(
            "$file: $tag $it is not where Gradle looks; move the sources to src/main or src/test",
          )
        }
      }
    }
    migration.modules.add(module)
  }
  return migration
}
