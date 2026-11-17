package bedrock

import org.gradle.api.Project
import org.gradle.api.artifacts.MinimalExternalModuleDependency
import org.gradle.api.artifacts.VersionCatalog
import org.gradle.api.artifacts.VersionCatalogsExtension
import org.gradle.api.provider.Provider
import org.gradle.kotlin.dsl.getByType

/** The bedrock catalog, gradle/bedrock.versions.toml, which bedrock.settings registers. */
internal val Project.catalog: VersionCatalog
  get() = extensions.getByType<VersionCatalogsExtension>().named("bedrock")

internal fun Project.version(name: String): String =
  catalog.findVersion(name).orElseThrow { IllegalStateException("bedrock catalog has no $name") }
    .requiredVersion

internal fun Project.library(name: String): Provider<MinimalExternalModuleDependency> =
  catalog.findLibrary(name).orElseThrow { IllegalStateException("bedrock catalog has no $name") }
